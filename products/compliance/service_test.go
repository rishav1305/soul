package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func setupTestServer(t *testing.T) soulv1.ProductServiceClient {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	soulv1.RegisterProductServiceServer(srv, &ComplianceService{})

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Server was stopped; ignore.
		}
	}()
	t.Cleanup(func() {
		srv.Stop()
		lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return soulv1.NewProductServiceClient(conn)
}

func TestGetManifest(t *testing.T) {
	client := setupTestServer(t)

	resp, err := client.GetManifest(context.Background(), &soulv1.Empty{})
	if err != nil {
		t.Fatalf("GetManifest failed: %v", err)
	}

	if resp.Name != "compliance" {
		t.Errorf("expected name 'compliance', got %q", resp.Name)
	}
	if resp.Version != "0.2.0" {
		t.Errorf("expected version '0.2.0', got %q", resp.Version)
	}
	if len(resp.Tools) != 5 {
		t.Errorf("expected 5 tools, got %d", len(resp.Tools))
	}

	// Verify tool names
	expectedTools := map[string]bool{
		"scan":    false,
		"fix":     false,
		"badge":   false,
		"report":  false,
		"monitor": false,
	}
	for _, tool := range resp.Tools {
		if _, ok := expectedTools[tool.Name]; !ok {
			t.Errorf("unexpected tool name: %q", tool.Name)
		}
		expectedTools[tool.Name] = true
	}
	for name, found := range expectedTools {
		if !found {
			t.Errorf("missing expected tool: %q", name)
		}
	}
}

func TestExecuteToolScan(t *testing.T) {
	client := setupTestServer(t)

	// Create a temp directory with a file containing a known secret.
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "config.js")
	content := `const apiKey = "AKIAIOSFODNN7EXAMPLE";`
	if err := os.WriteFile(secretFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	inputJSON, _ := json.Marshal(scanInput{
		Directory: tmpDir,
	})

	resp, err := client.ExecuteTool(context.Background(), &soulv1.ToolRequest{
		Tool:      "scan",
		InputJson: string(inputJSON),
	})
	if err != nil {
		t.Fatalf("ExecuteTool(scan) failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Output)
	}

	if resp.StructuredJson == "" {
		t.Fatal("expected non-empty structured_json")
	}

	// Parse the structured JSON to verify it has findings.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp.StructuredJson), &result); err != nil {
		t.Fatalf("failed to parse structured_json: %v", err)
	}

	findings, ok := result["findings"]
	if !ok {
		t.Fatal("structured_json missing 'findings' key")
	}

	findingsSlice, ok := findings.([]interface{})
	if !ok {
		t.Fatal("findings is not an array")
	}

	if len(findingsSlice) == 0 {
		t.Error("expected at least one finding for the AWS key, got 0")
	}
}

func TestHealth(t *testing.T) {
	client := setupTestServer(t)

	resp, err := client.Health(context.Background(), &soulv1.Empty{})
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}

	if !resp.Healthy {
		t.Error("expected healthy=true, got false")
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

func TestExecuteToolUnknown(t *testing.T) {
	client := setupTestServer(t)

	resp, err := client.ExecuteTool(context.Background(), &soulv1.ToolRequest{
		Tool:      "nonexistent",
		InputJson: "{}",
	})
	if err != nil {
		t.Fatalf("ExecuteTool(nonexistent) failed with gRPC error: %v", err)
	}

	if resp.Success {
		t.Error("expected success=false for unknown tool, got true")
	}
}

func TestExecuteToolScanStream(t *testing.T) {
	client := setupTestServer(t)

	// Create a temp directory with a known secret.
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "app.py")
	content := `password = "AKIAIOSFODNN7EXAMPLE"`
	if err := os.WriteFile(secretFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	inputJSON, _ := json.Marshal(scanInput{
		Directory: tmpDir,
	})

	stream, err := client.ExecuteToolStream(context.Background(), &soulv1.ToolRequest{
		Tool:      "scan",
		InputJson: string(inputJSON),
	})
	if err != nil {
		t.Fatalf("ExecuteToolStream(scan) failed: %v", err)
	}

	var (
		progressCount int
		findingCount  int
		gotComplete   bool
	)

	for {
		event, err := stream.Recv()
		if err != nil {
			break
		}

		switch event.Event.(type) {
		case *soulv1.ToolEvent_Progress:
			progressCount++
		case *soulv1.ToolEvent_Finding:
			findingCount++
		case *soulv1.ToolEvent_Complete:
			gotComplete = true
			complete := event.GetComplete()
			if !complete.Success {
				t.Errorf("expected stream complete success=true, got false: %s", complete.Output)
			}
		case *soulv1.ToolEvent_Error:
			t.Errorf("received unexpected error event: %s", event.GetError().Message)
		}
	}

	if progressCount == 0 {
		t.Error("expected at least one progress event")
	}
	if !gotComplete {
		t.Error("expected a complete event")
	}
}

func TestExecuteToolReport(t *testing.T) {
	client := setupTestServer(t)

	tmpDir := t.TempDir()

	inputJSON, _ := json.Marshal(reportInput{
		Directory: tmpDir,
		Format:    "json",
	})

	resp, err := client.ExecuteTool(context.Background(), &soulv1.ToolRequest{
		Tool:      "report",
		InputJson: string(inputJSON),
	})
	if err != nil {
		t.Fatalf("ExecuteTool(report) failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Output)
	}

	// The output should be valid JSON.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Output), &result); err != nil {
		t.Errorf("report output is not valid JSON: %v", err)
	}
}

func TestExecuteToolBadge(t *testing.T) {
	client := setupTestServer(t)

	tmpDir := t.TempDir()

	inputJSON, _ := json.Marshal(badgeInput{
		Directory: tmpDir,
	})

	resp, err := client.ExecuteTool(context.Background(), &soulv1.ToolRequest{
		Tool:      "badge",
		InputJson: string(inputJSON),
	})
	if err != nil {
		t.Fatalf("ExecuteTool(badge) failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false: %s", resp.Output)
	}

	if len(resp.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(resp.Artifacts))
	}

	artifact := resp.Artifacts[0]
	if artifact.Type != "image/svg+xml" {
		t.Errorf("expected artifact type 'image/svg+xml', got %q", artifact.Type)
	}
	if len(artifact.Content) == 0 {
		t.Error("expected non-empty SVG content in artifact")
	}
}

func TestExecuteToolMonitor(t *testing.T) {
	client := setupTestServer(t)

	resp, err := client.ExecuteTool(context.Background(), &soulv1.ToolRequest{
		Tool:      "monitor",
		InputJson: `{"directory": "/tmp"}`,
	})
	if err != nil {
		t.Fatalf("ExecuteTool(monitor) failed: %v", err)
	}

	if resp.Success {
		t.Error("expected success=false for unimplemented monitor, got true")
	}
}
