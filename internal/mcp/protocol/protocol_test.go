package protocol

import (
	"encoding/json"
	"testing"
)

func TestParseRequest_ToolsCall(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hello"}}}`)
	req, err := ParseRequest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", req.JSONRPC, "2.0")
	}
	if req.Method != "tools/call" {
		t.Errorf("method = %q, want %q", req.Method, "tools/call")
	}
	if req.IsNotification() {
		t.Error("expected non-notification (id is set)")
	}
	if string(req.ID) != "1" {
		t.Errorf("id = %s, want 1", string(req.ID))
	}
}

func TestParseRequest_Notification(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	req, err := ParseRequest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !req.IsNotification() {
		t.Error("expected notification (no id)")
	}
	if req.Method != "notifications/initialized" {
		t.Errorf("method = %q, want %q", req.Method, "notifications/initialized")
	}
}

func TestParseRequest_InvalidJSON(t *testing.T) {
	data := []byte(`{not valid json}`)
	_, err := ParseRequest(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseRequest_MissingVersion(t *testing.T) {
	data := []byte(`{"method":"tools/list"}`)
	_, err := ParseRequest(data)
	if err == nil {
		t.Fatal("expected error for missing jsonrpc version")
	}
}

func TestParseRequest_EmptyMethod(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"method":""}`)
	_, err := ParseRequest(data)
	if err == nil {
		t.Fatal("expected error for empty method")
	}
}

func TestNewResult(t *testing.T) {
	id := json.RawMessage(`42`)
	result := map[string]string{"status": "ok"}
	resp := NewResult(id, result)

	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", resp.JSONRPC, "2.0")
	}
	if string(resp.ID) != "42" {
		t.Errorf("id = %s, want 42", string(resp.ID))
	}
	if resp.Error != nil {
		t.Error("expected no error in result response")
	}
	if resp.Result == nil {
		t.Error("expected non-nil result")
	}
}

func TestNewError(t *testing.T) {
	id := json.RawMessage(`7`)
	resp := NewError(id, MethodNotFound, "method not found")

	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", resp.JSONRPC, "2.0")
	}
	if string(resp.ID) != "7" {
		t.Errorf("id = %s, want 7", string(resp.ID))
	}
	if resp.Error == nil {
		t.Fatal("expected error in error response")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, MethodNotFound)
	}
	if resp.Error.Message != "method not found" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "method not found")
	}
	if resp.Result != nil {
		t.Error("expected nil result in error response")
	}
}

func TestParseToolCallParams(t *testing.T) {
	params := json.RawMessage(`{"name":"file_read","arguments":{"path":"/tmp/test.txt"}}`)
	name, args, err := ParseToolCallParams(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "file_read" {
		t.Errorf("name = %q, want %q", name, "file_read")
	}

	var parsed map[string]string
	if err := json.Unmarshal(args, &parsed); err != nil {
		t.Fatalf("failed to unmarshal args: %v", err)
	}
	if parsed["path"] != "/tmp/test.txt" {
		t.Errorf("args.path = %q, want %q", parsed["path"], "/tmp/test.txt")
	}
}

func TestParseToolCallParams_MissingName(t *testing.T) {
	params := json.RawMessage(`{"arguments":{"path":"/tmp/test.txt"}}`)
	_, _, err := ParseToolCallParams(params)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseToolCallParams_InvalidJSON(t *testing.T) {
	params := json.RawMessage(`{garbage}`)
	_, _, err := ParseToolCallParams(params)
	if err == nil {
		t.Fatal("expected error for invalid params JSON")
	}
}

func TestResponse_Serialization(t *testing.T) {
	resp := NewResult(json.RawMessage(`1`), &ToolsListResult{
		Tools: []MCPTool{
			{
				Name:        "echo",
				Description: "Echoes input",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
			},
		},
	})

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if decoded.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", decoded.JSONRPC, "2.0")
	}
	if decoded.Error != nil {
		t.Error("expected no error after round-trip")
	}
}

func TestIsNotification_NullID(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":null,"method":"notifications/initialized"}`)
	req, err := ParseRequest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !req.IsNotification() {
		t.Error("expected null id to be treated as notification")
	}
}
