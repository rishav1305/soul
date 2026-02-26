package products_test

import (
	"testing"

	"github.com/rishav1305/soul/internal/products"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

func TestRegistryAddAndGet(t *testing.T) {
	reg := products.NewRegistry()

	manifest := &soulv1.Manifest{
		Name:    "compliance",
		Version: "1.0.0",
		Tools: []*soulv1.Tool{
			{Name: "scan", Description: "Run compliance scan"},
			{Name: "report", Description: "Generate report"},
		},
	}

	reg.Register("compliance", manifest)

	// Verify Get returns the manifest.
	got, ok := reg.Get("compliance")
	if !ok {
		t.Fatal("expected Get to return true for registered product")
	}
	if got.GetName() != "compliance" {
		t.Fatalf("expected manifest name 'compliance', got %q", got.GetName())
	}
	if len(got.GetTools()) != 2 {
		t.Fatalf("expected 2 tools in manifest, got %d", len(got.GetTools()))
	}

	// Verify Get returns false for unknown product.
	_, ok = reg.Get("unknown")
	if ok {
		t.Fatal("expected Get to return false for unregistered product")
	}

	// Verify AllTools returns 2 entries with correct ProductName.
	allTools := reg.AllTools()
	if len(allTools) != 2 {
		t.Fatalf("expected AllTools to return 2 entries, got %d", len(allTools))
	}

	for _, entry := range allTools {
		if entry.ProductName != "compliance" {
			t.Fatalf("expected ProductName 'compliance', got %q", entry.ProductName)
		}
		if entry.Tool == nil {
			t.Fatal("expected Tool to be non-nil")
		}
	}

	// Verify tool names are present.
	toolNames := make(map[string]bool)
	for _, entry := range allTools {
		toolNames[entry.Tool.GetName()] = true
	}
	if !toolNames["scan"] {
		t.Fatal("expected AllTools to include 'scan' tool")
	}
	if !toolNames["report"] {
		t.Fatal("expected AllTools to include 'report' tool")
	}
}

func TestRegistryFindTool(t *testing.T) {
	reg := products.NewRegistry()

	manifest := &soulv1.Manifest{
		Name:    "compliance",
		Version: "1.0.0",
		Tools: []*soulv1.Tool{
			{Name: "scan", Description: "Run compliance scan"},
			{Name: "report", Description: "Generate report"},
		},
	}

	reg.Register("compliance", manifest)

	// FindTool with valid qualified name succeeds.
	entry, ok := reg.FindTool("compliance__scan")
	if !ok {
		t.Fatal("expected FindTool to return true for 'compliance__scan'")
	}
	if entry.ProductName != "compliance" {
		t.Fatalf("expected ProductName 'compliance', got %q", entry.ProductName)
	}
	if entry.Tool.GetName() != "scan" {
		t.Fatalf("expected tool name 'scan', got %q", entry.Tool.GetName())
	}

	// FindTool with valid product but nonexistent tool fails.
	_, ok = reg.FindTool("compliance__nonexistent")
	if ok {
		t.Fatal("expected FindTool to return false for 'compliance__nonexistent'")
	}

	// FindTool with unknown product fails.
	_, ok = reg.FindTool("unknown__scan")
	if ok {
		t.Fatal("expected FindTool to return false for 'unknown__scan'")
	}

	// FindTool with invalid format (no double underscore) fails.
	_, ok = reg.FindTool("invalidformat")
	if ok {
		t.Fatal("expected FindTool to return false for invalid qualified name")
	}
}
