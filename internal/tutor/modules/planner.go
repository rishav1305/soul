package modules

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

// PlannerModule handles study plan creation and management.
type PlannerModule struct {
	store *store.Store
}

// planData represents the JSON structure stored in study_plans.plan_json.
type planData struct {
	TargetRole  string       `json:"targetRole"`
	TargetDate  string       `json:"targetDate"`
	DaysLeft    int          `json:"daysLeft"`
	Phases      []planPhase  `json:"phases"`
	ModuleStats []moduleStat `json:"moduleStats"`
	CreatedAt   string       `json:"createdAt"`
}

type planPhase struct {
	Name     string   `json:"name"`
	Days     int      `json:"days"`
	Focus    []string `json:"focus"`
	Priority string   `json:"priority"`
}

type moduleStat struct {
	Module        string  `json:"module"`
	TopicCount    int     `json:"topicCount"`
	CompletedPct  float64 `json:"completedPct"`
	AvgScore      float64 `json:"avgScore"`
}

// Plan routes based on the "action" field in input.
// - {"action": "create", "target_role": "SDE-2", "target_date": "2026-04-01"} — creates plan
// - {"action": "update"} — recalculates active plan
// - {"action": "get"} or no action — returns active plan
func (m *PlannerModule) Plan(input map[string]interface{}) (*ToolResult, error) {
	action, _ := input["action"].(string)
	if action == "" {
		action = "get"
	}

	switch action {
	case "create":
		return m.createPlan(input)
	case "update":
		return m.updatePlan()
	case "get":
		return m.getPlan()
	default:
		return nil, fmt.Errorf("planner: invalid action '%s', must be: create, update, get", action)
	}
}

func (m *PlannerModule) createPlan(input map[string]interface{}) (*ToolResult, error) {
	targetRole, _ := input["target_role"].(string)
	targetDate, _ := input["target_date"].(string)
	if targetRole == "" {
		return nil, fmt.Errorf("planner: create requires 'target_role' field")
	}
	if targetDate == "" {
		return nil, fmt.Errorf("planner: create requires 'target_date' field")
	}

	// Validate target date.
	target, err := time.Parse("2006-01-02", targetDate)
	if err != nil {
		return nil, fmt.Errorf("planner: invalid target_date format (use YYYY-MM-DD): %w", err)
	}

	daysLeft := int(time.Until(target).Hours() / 24)
	if daysLeft < 0 {
		return nil, fmt.Errorf("planner: target_date is in the past")
	}

	// Gather module stats.
	stats := m.gatherModuleStats()

	// Generate phases based on time.
	phases := generatePhases(daysLeft)

	pd := planData{
		TargetRole:  targetRole,
		TargetDate:  targetDate,
		DaysLeft:    daysLeft,
		Phases:      phases,
		ModuleStats: stats,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	planJSON, err := json.Marshal(pd)
	if err != nil {
		return nil, fmt.Errorf("planner: marshal plan: %w", err)
	}

	plan, err := m.store.CreatePlan(targetRole, targetDate, string(planJSON))
	if err != nil {
		return nil, fmt.Errorf("planner: create plan: %w", err)
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Study plan created: %s by %s (%d days)", targetRole, targetDate, daysLeft),
		Data: map[string]interface{}{
			"plan":   plan,
			"parsed": pd,
		},
	}, nil
}

func (m *PlannerModule) updatePlan() (*ToolResult, error) {
	plan, err := m.store.GetActivePlan()
	if err != nil {
		return nil, fmt.Errorf("planner: no active plan to update: %w", err)
	}

	// Parse current target date.
	target, err := time.Parse("2006-01-02", plan.TargetDate)
	if err != nil {
		return nil, fmt.Errorf("planner: parse target date: %w", err)
	}

	daysLeft := int(time.Until(target).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}

	stats := m.gatherModuleStats()
	phases := generatePhases(daysLeft)

	pd := planData{
		TargetRole:  plan.TargetRole,
		TargetDate:  plan.TargetDate,
		DaysLeft:    daysLeft,
		Phases:      phases,
		ModuleStats: stats,
		CreatedAt:   plan.CreatedAt.Format(time.RFC3339),
	}

	planJSON, err := json.Marshal(pd)
	if err != nil {
		return nil, fmt.Errorf("planner: marshal plan: %w", err)
	}

	if err := m.store.UpdatePlan(plan.ID, string(planJSON)); err != nil {
		return nil, fmt.Errorf("planner: update plan: %w", err)
	}

	plan.PlanJSON = string(planJSON)

	return &ToolResult{
		Summary: fmt.Sprintf("Study plan updated: %d days remaining", daysLeft),
		Data: map[string]interface{}{
			"plan":   plan,
			"parsed": pd,
		},
	}, nil
}

func (m *PlannerModule) getPlan() (*ToolResult, error) {
	plan, err := m.store.GetActivePlan()
	if err != nil {
		return &ToolResult{
			Summary: "No active study plan found",
			Data: map[string]interface{}{
				"exists": false,
			},
		}, nil
	}

	var pd planData
	json.Unmarshal([]byte(plan.PlanJSON), &pd)

	// Recalculate days left.
	target, err := time.Parse("2006-01-02", plan.TargetDate)
	if err == nil {
		pd.DaysLeft = int(time.Until(target).Hours() / 24)
		if pd.DaysLeft < 0 {
			pd.DaysLeft = 0
		}
	}

	// Overlay current readiness.
	stats := m.gatherModuleStats()

	return &ToolResult{
		Summary: fmt.Sprintf("Study plan: %s by %s (%d days left)", plan.TargetRole, plan.TargetDate, pd.DaysLeft),
		Data: map[string]interface{}{
			"plan":         plan,
			"parsed":       pd,
			"currentStats": stats,
			"exists":       true,
		},
	}, nil
}

func (m *PlannerModule) gatherModuleStats() []moduleStat {
	modules := []string{"dsa", "ai", "behavioral"}
	stats := make([]moduleStat, 0, len(modules))

	for _, mod := range modules {
		ms, err := m.store.GetModuleStats(mod)
		if err != nil {
			stats = append(stats, moduleStat{Module: mod})
			continue
		}
		stats = append(stats, moduleStat{
			Module:       mod,
			TopicCount:   ms.TopicCount,
			CompletedPct: ms.CompletionPct,
			AvgScore:     ms.AvgScore,
		})
	}

	return stats
}

func generatePhases(daysLeft int) []planPhase {
	if daysLeft <= 7 {
		// 1 sprint — focus on everything.
		return []planPhase{
			{
				Name:     "Sprint",
				Days:     daysLeft,
				Focus:    []string{"Review weak areas", "Mock interviews", "Behavioral prep", "Quick DSA drills"},
				Priority: "high",
			},
		}
	}

	if daysLeft <= 30 {
		// 2 phases — foundation + polish.
		foundationDays := daysLeft * 2 / 3
		polishDays := daysLeft - foundationDays
		return []planPhase{
			{
				Name:     "Foundation",
				Days:     foundationDays,
				Focus:    []string{"Core DSA topics", "AI/ML fundamentals", "Build STAR stories", "Daily drills"},
				Priority: "high",
			},
			{
				Name:     "Polish",
				Days:     polishDays,
				Focus:    []string{"Mock interviews", "Weak area review", "Behavioral practice", "Speed drills"},
				Priority: "medium",
			},
		}
	}

	// 31+ days — 3 phases: learn, drill, mock.
	learnDays := daysLeft * 2 / 5
	drillDays := daysLeft * 2 / 5
	mockDays := daysLeft - learnDays - drillDays
	return []planPhase{
		{
			Name:     "Learn",
			Days:     learnDays,
			Focus:    []string{"Study all DSA topics", "AI/ML theory", "Build narratives", "Content generation"},
			Priority: "high",
		},
		{
			Name:     "Drill",
			Days:     drillDays,
			Focus:    []string{"Daily quizzes", "Spaced repetition", "Problem solving", "HR question practice"},
			Priority: "high",
		},
		{
			Name:     "Mock",
			Days:     mockDays,
			Focus:    []string{"Full mock interviews", "JD analysis", "Weak area focus", "Final review"},
			Priority: "medium",
		},
	}
}
