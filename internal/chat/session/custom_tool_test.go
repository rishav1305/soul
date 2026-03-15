package session

import (
	"strings"
	"testing"
	"time"
)

func TestCreateCustomTool(t *testing.T) {
	s := openTestStore(t)

	ct, err := s.CreateCustomTool("deploy", "Deploy the app", `{"type":"object"}`, "make deploy {{.env}}")
	if err != nil {
		t.Fatalf("CreateCustomTool: %v", err)
	}
	if ct.Name != "deploy" {
		t.Errorf("Name = %q, want %q", ct.Name, "deploy")
	}
	if ct.Description != "Deploy the app" {
		t.Errorf("Description = %q, want %q", ct.Description, "Deploy the app")
	}
	if ct.InputSchema != `{"type":"object"}` {
		t.Errorf("InputSchema = %q, want %q", ct.InputSchema, `{"type":"object"}`)
	}
	if ct.CommandTemplate != "make deploy {{.env}}" {
		t.Errorf("CommandTemplate = %q, want %q", ct.CommandTemplate, "make deploy {{.env}}")
	}
	if ct.ID == 0 {
		t.Error("ID should be non-zero")
	}
	if ct.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
	if ct.UpdatedAt == "" {
		t.Error("UpdatedAt should not be empty")
	}
	if _, err := time.Parse(time.RFC3339, ct.CreatedAt); err != nil {
		t.Errorf("CreatedAt is not valid RFC3339: %v", err)
	}
}

func TestCreateCustomTool_Duplicate(t *testing.T) {
	s := openTestStore(t)

	_, err := s.CreateCustomTool("deploy", "Deploy the app", `{"type":"object"}`, "make deploy")
	if err != nil {
		t.Fatalf("CreateCustomTool: %v", err)
	}

	_, err = s.CreateCustomTool("deploy", "Deploy again", `{"type":"object"}`, "make deploy2")
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err)
	}
}

func TestListCustomTools(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.CreateCustomTool("zebra", "Z tool", `{}`, "z"); err != nil {
		t.Fatalf("CreateCustomTool: %v", err)
	}
	if _, err := s.CreateCustomTool("alpha", "A tool", `{}`, "a"); err != nil {
		t.Fatalf("CreateCustomTool: %v", err)
	}

	tools, err := s.ListCustomTools()
	if err != nil {
		t.Fatalf("ListCustomTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}
	if tools[0].Name != "alpha" {
		t.Errorf("first tool = %q, want %q", tools[0].Name, "alpha")
	}
	if tools[1].Name != "zebra" {
		t.Errorf("second tool = %q, want %q", tools[1].Name, "zebra")
	}
}

func TestGetCustomTool(t *testing.T) {
	s := openTestStore(t)

	_, err := s.CreateCustomTool("builder", "Build stuff", `{"type":"object"}`, "make build --target={{.target}}")
	if err != nil {
		t.Fatalf("CreateCustomTool: %v", err)
	}

	ct, err := s.GetCustomTool("builder")
	if err != nil {
		t.Fatalf("GetCustomTool: %v", err)
	}
	if ct.CommandTemplate != "make build --target={{.target}}" {
		t.Errorf("CommandTemplate = %q, want %q", ct.CommandTemplate, "make build --target={{.target}}")
	}
}

func TestGetCustomTool_NotFound(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetCustomTool("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestDeleteCustomTool(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.CreateCustomTool("temp", "Temporary", `{}`, "echo temp"); err != nil {
		t.Fatalf("CreateCustomTool: %v", err)
	}

	if err := s.DeleteCustomTool("temp"); err != nil {
		t.Fatalf("DeleteCustomTool: %v", err)
	}

	_, err := s.GetCustomTool("temp")
	if err == nil {
		t.Fatal("expected error after delete")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}

	// Deleting again should error.
	err = s.DeleteCustomTool("temp")
	if err == nil {
		t.Fatal("expected error deleting nonexistent tool")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}
