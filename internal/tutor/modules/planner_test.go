package modules

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/tutor/store"
)

func openTestStoreForPlanner(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "planner_test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newPlannerModule(t *testing.T) *PlannerModule {
	t.Helper()
	return &PlannerModule{store: openTestStoreForPlanner(t)}
}

// --- generatePhases (pure function, 3 branches) ---

func TestGeneratePhases_Sprint(t *testing.T) {
	// ≤7 days → single "Sprint" phase.
	phases := generatePhases(5)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if phases[0].Name != "Sprint" {
		t.Errorf("expected phase name 'Sprint', got %q", phases[0].Name)
	}
	if phases[0].Days != 5 {
		t.Errorf("expected 5 days, got %d", phases[0].Days)
	}
	if phases[0].Priority != "high" {
		t.Errorf("expected priority 'high', got %q", phases[0].Priority)
	}
	if len(phases[0].Focus) == 0 {
		t.Error("expected non-empty focus list")
	}
}

func TestGeneratePhases_SprintEdge(t *testing.T) {
	// Exactly 7 days → still single sprint.
	phases := generatePhases(7)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase for 7 days, got %d", len(phases))
	}
}

func TestGeneratePhases_TwoPhases(t *testing.T) {
	// 8-30 days → 2 phases (Foundation + Polish).
	phases := generatePhases(21)
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(phases))
	}
	if phases[0].Name != "Foundation" {
		t.Errorf("phase[0].Name = %q, want 'Foundation'", phases[0].Name)
	}
	if phases[1].Name != "Polish" {
		t.Errorf("phase[1].Name = %q, want 'Polish'", phases[1].Name)
	}
	// Days should sum to total.
	total := phases[0].Days + phases[1].Days
	if total != 21 {
		t.Errorf("days sum = %d, want 21", total)
	}
}

func TestGeneratePhases_ThreePhases(t *testing.T) {
	// 31+ days → 3 phases (Learn, Drill, Mock).
	phases := generatePhases(60)
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d", len(phases))
	}
	if phases[0].Name != "Learn" {
		t.Errorf("phase[0].Name = %q, want 'Learn'", phases[0].Name)
	}
	if phases[1].Name != "Drill" {
		t.Errorf("phase[1].Name = %q, want 'Drill'", phases[1].Name)
	}
	if phases[2].Name != "Mock" {
		t.Errorf("phase[2].Name = %q, want 'Mock'", phases[2].Name)
	}
	total := phases[0].Days + phases[1].Days + phases[2].Days
	if total != 60 {
		t.Errorf("days sum = %d, want 60", total)
	}
}

func TestGeneratePhases_ZeroDays(t *testing.T) {
	phases := generatePhases(0)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase for 0 days, got %d", len(phases))
	}
	if phases[0].Days != 0 {
		t.Errorf("expected 0 days, got %d", phases[0].Days)
	}
}

func TestGeneratePhases_ThirtyDays(t *testing.T) {
	// Exactly 30 → 2 phases.
	phases := generatePhases(30)
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases for 30 days, got %d", len(phases))
	}
}

func TestGeneratePhases_ThirtyOneDays(t *testing.T) {
	// 31 → 3 phases.
	phases := generatePhases(31)
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases for 31 days, got %d", len(phases))
	}
}

// --- PlannerModule.Plan routing ---

func TestPlannerPlan_CreateHappyPath(t *testing.T) {
	m := newPlannerModule(t)

	// Future date 30 days from now.
	target := time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02")
	result, err := m.Plan(map[string]interface{}{
		"action":      "create",
		"target_role": "SDE-2",
		"target_date": target,
	})
	if err != nil {
		t.Fatalf("Plan create: %v", err)
	}
	if result == nil {
		t.Fatal("Plan create returned nil")
	}
	if !strings.Contains(result.Summary, "SDE-2") {
		t.Errorf("summary missing target role: %s", result.Summary)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("result.Data is not map[string]interface{}")
	}
	if _, ok := data["plan"]; !ok {
		t.Error("data missing 'plan' key")
	}
	if _, ok := data["parsed"]; !ok {
		t.Error("data missing 'parsed' key")
	}
}

func TestPlannerPlan_CreateMissingRole(t *testing.T) {
	m := newPlannerModule(t)
	_, err := m.Plan(map[string]interface{}{
		"action":      "create",
		"target_date": "2026-12-01",
	})
	if err == nil {
		t.Fatal("expected error for missing target_role")
	}
}

func TestPlannerPlan_CreateMissingDate(t *testing.T) {
	m := newPlannerModule(t)
	_, err := m.Plan(map[string]interface{}{
		"action":      "create",
		"target_role": "SDE-2",
	})
	if err == nil {
		t.Fatal("expected error for missing target_date")
	}
}

func TestPlannerPlan_CreateInvalidDate(t *testing.T) {
	m := newPlannerModule(t)
	_, err := m.Plan(map[string]interface{}{
		"action":      "create",
		"target_role": "SDE-2",
		"target_date": "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
	if !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Errorf("expected date format hint in error, got: %v", err)
	}
}

func TestPlannerPlan_CreatePastDate(t *testing.T) {
	m := newPlannerModule(t)
	_, err := m.Plan(map[string]interface{}{
		"action":      "create",
		"target_role": "SDE-2",
		"target_date": "2020-01-01",
	})
	if err == nil {
		t.Fatal("expected error for past date")
	}
	if !strings.Contains(err.Error(), "past") {
		t.Errorf("expected 'past' in error, got: %v", err)
	}
}

func TestPlannerPlan_GetNoPlan(t *testing.T) {
	m := newPlannerModule(t)
	result, err := m.Plan(map[string]interface{}{
		"action": "get",
	})
	if err != nil {
		t.Fatalf("Plan get with no plan: %v", err)
	}
	data := result.Data.(map[string]interface{})
	if data["exists"] != false {
		t.Errorf("expected exists=false, got %v", data["exists"])
	}
}

func TestPlannerPlan_DefaultActionIsGet(t *testing.T) {
	m := newPlannerModule(t)
	result, err := m.Plan(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Plan default action: %v", err)
	}
	if result == nil {
		t.Fatal("Plan returned nil")
	}
	data := result.Data.(map[string]interface{})
	if data["exists"] != false {
		t.Errorf("expected exists=false for empty store, got %v", data["exists"])
	}
}

func TestPlannerPlan_InvalidAction(t *testing.T) {
	m := newPlannerModule(t)
	_, err := m.Plan(map[string]interface{}{"action": "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
	if !strings.Contains(err.Error(), "invalid action") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPlannerPlan_CreateThenGet(t *testing.T) {
	m := newPlannerModule(t)
	target := time.Now().Add(45 * 24 * time.Hour).Format("2006-01-02")

	// Create.
	_, err := m.Plan(map[string]interface{}{
		"action":      "create",
		"target_role": "Staff Eng",
		"target_date": target,
	})
	if err != nil {
		t.Fatalf("Plan create: %v", err)
	}

	// Get.
	result, err := m.Plan(map[string]interface{}{"action": "get"})
	if err != nil {
		t.Fatalf("Plan get: %v", err)
	}
	data := result.Data.(map[string]interface{})
	if data["exists"] != true {
		t.Error("expected exists=true after create")
	}
	if _, ok := data["currentStats"]; !ok {
		t.Error("get data missing 'currentStats'")
	}
}

func TestPlannerPlan_UpdateNoPlan(t *testing.T) {
	m := newPlannerModule(t)
	_, err := m.Plan(map[string]interface{}{"action": "update"})
	if err == nil {
		t.Fatal("expected error for update with no plan")
	}
}

func TestPlannerPlan_CreateThenUpdate(t *testing.T) {
	m := newPlannerModule(t)
	target := time.Now().Add(60 * 24 * time.Hour).Format("2006-01-02")

	_, err := m.Plan(map[string]interface{}{
		"action":      "create",
		"target_role": "SDE-3",
		"target_date": target,
	})
	if err != nil {
		t.Fatalf("Plan create: %v", err)
	}

	result, err := m.Plan(map[string]interface{}{"action": "update"})
	if err != nil {
		t.Fatalf("Plan update: %v", err)
	}
	if result == nil {
		t.Fatal("update returned nil")
	}
	if !strings.Contains(result.Summary, "updated") {
		t.Errorf("summary missing 'updated': %s", result.Summary)
	}
}

func TestPlannerPlan_PlanDataMarshals(t *testing.T) {
	// Verify the planData struct round-trips through JSON.
	pd := planData{
		TargetRole: "SDE-2",
		TargetDate: "2026-06-01",
		DaysLeft:   90,
		Phases:     generatePhases(90),
		ModuleStats: []moduleStat{
			{Module: "dsa", TopicCount: 10, CompletedPct: 50.0, AvgScore: 75.5},
		},
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(pd)
	if err != nil {
		t.Fatalf("marshal planData: %v", err)
	}

	var decoded planData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal planData: %v", err)
	}
	if decoded.TargetRole != "SDE-2" {
		t.Errorf("targetRole = %q, want 'SDE-2'", decoded.TargetRole)
	}
	if decoded.DaysLeft != 90 {
		t.Errorf("daysLeft = %d, want 90", decoded.DaysLeft)
	}
	if len(decoded.Phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(decoded.Phases))
	}
}

// --- gatherModuleStats ---

func TestGatherModuleStats(t *testing.T) {
	m := newPlannerModule(t)
	stats := m.gatherModuleStats()
	if len(stats) != 3 {
		t.Fatalf("expected 3 module stats, got %d", len(stats))
	}

	// Expected modules.
	expected := map[string]bool{"dsa": true, "ai": true, "behavioral": true}
	for _, s := range stats {
		if !expected[s.Module] {
			t.Errorf("unexpected module: %s", s.Module)
		}
	}
}
