package ws

import "testing"

func TestSubagentConfig_Defaults(t *testing.T) {
	sc := &SubagentConfig{Task: "test task"}
	sc.applyDefaults()

	if sc.MaxIterations != 5 {
		t.Errorf("expected default MaxIterations=5, got %d", sc.MaxIterations)
	}
}

func TestSubagentConfig_Cap(t *testing.T) {
	sc := &SubagentConfig{Task: "test task", MaxIterations: 20}
	sc.applyDefaults()

	if sc.MaxIterations != 10 {
		t.Errorf("expected capped MaxIterations=10, got %d", sc.MaxIterations)
	}
}

func TestSubagentConfig_ZeroCapped(t *testing.T) {
	sc := &SubagentConfig{Task: "test task", MaxIterations: 0}
	sc.applyDefaults()

	if sc.MaxIterations != 5 {
		t.Errorf("expected default MaxIterations=5 for zero value, got %d", sc.MaxIterations)
	}
}

func TestSubagentConfig_NegativeCapped(t *testing.T) {
	sc := &SubagentConfig{Task: "test task", MaxIterations: -3}
	sc.applyDefaults()

	if sc.MaxIterations != 5 {
		t.Errorf("expected default MaxIterations=5 for negative value, got %d", sc.MaxIterations)
	}
}

func TestSubagentConfig_ValidValue(t *testing.T) {
	sc := &SubagentConfig{Task: "test task", MaxIterations: 7}
	sc.applyDefaults()

	if sc.MaxIterations != 7 {
		t.Errorf("expected MaxIterations=7 (within range), got %d", sc.MaxIterations)
	}
}

func TestReadOnlyTools(t *testing.T) {
	tools := readOnlyTools()
	if len(tools) != 4 {
		t.Fatalf("expected 4 read-only tools, got %d", len(tools))
	}

	expected := []string{"file_read", "file_search", "file_grep", "file_glob"}
	for i, name := range expected {
		if tools[i].Name != name {
			t.Errorf("tool %d: expected name %q, got %q", i, name, tools[i].Name)
		}
	}
}

func TestExecuteReadOnlyTool_Stub(t *testing.T) {
	result := executeReadOnlyTool("/tmp", "file_read", `{"path":"test.go"}`)
	expected := "Tool file_read not yet implemented"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
