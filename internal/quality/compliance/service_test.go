package compliance

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExecuteTool_UnknownTool(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("nonexistent", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown compliance tool") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteTool_ScanMissingDirectory(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("scan", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
	if !strings.Contains(err.Error(), "directory is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteTool_ScanInvalidJSON(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("scan", json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parsing scan input") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteTool_ScanValidDirectory(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	input, _ := json.Marshal(scanInput{Directory: dir})
	result, err := s.ExecuteTool("scan", input)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestExecuteTool_FixMissingDirectory(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("fix", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
	if !strings.Contains(err.Error(), "directory is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteTool_FixInvalidJSON(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("fix", json.RawMessage(`bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExecuteTool_FixDryRun(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	input, _ := json.Marshal(fixInput{Directory: dir, DryRun: true})
	result, err := s.ExecuteTool("fix", input)
	if err != nil {
		t.Fatalf("fix error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", m["dry_run"])
	}
}

func TestExecuteTool_BadgeMissingDirectory(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("badge", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
	if !strings.Contains(err.Error(), "directory is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteTool_BadgeInvalidJSON(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("badge", json.RawMessage(`bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExecuteTool_BadgeValid(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	input, _ := json.Marshal(badgeInput{Directory: dir})
	result, err := s.ExecuteTool("badge", input)
	if err != nil {
		t.Fatalf("badge error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, hasBadge := m["badge"]; !hasBadge {
		t.Error("expected badge field in result")
	}
	if _, hasScore := m["score"]; !hasScore {
		t.Error("expected score field in result")
	}
}

func TestExecuteTool_ReportMissingDirectory(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("report", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
	if !strings.Contains(err.Error(), "directory is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteTool_ReportInvalidJSON(t *testing.T) {
	s := &Service{}
	_, err := s.ExecuteTool("report", json.RawMessage(`bad`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExecuteTool_ReportJSON(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	input, _ := json.Marshal(reportInput{Directory: dir, Format: "json"})
	result, err := s.ExecuteTool("report", input)
	if err != nil {
		t.Fatalf("report error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["format"] != "json" {
		t.Errorf("format = %v, want json", m["format"])
	}
}

func TestExecuteTool_ReportDefaultFormat(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	// Omit format — should default to "json".
	input, _ := json.Marshal(reportInput{Directory: dir})
	result, err := s.ExecuteTool("report", input)
	if err != nil {
		t.Fatalf("report error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["format"] != "json" {
		t.Errorf("format = %v, want json", m["format"])
	}
}

func TestExecuteTool_ReportTerminal(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	input, _ := json.Marshal(reportInput{Directory: dir, Format: "terminal"})
	result, err := s.ExecuteTool("report", input)
	if err != nil {
		t.Fatalf("report error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["format"] != "terminal" {
		t.Errorf("format = %v, want terminal", m["format"])
	}
}

func TestExecuteTool_ScanWithFilters(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	input, _ := json.Marshal(scanInput{
		Directory:  dir,
		Frameworks: []string{"soc2"},
		Severity:   []string{"high"},
		Analyzers:  []string{"config"},
	})
	result, err := s.ExecuteTool("scan", input)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
