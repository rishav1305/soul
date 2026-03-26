package modules

import (
	"fmt"
	"time"

	"github.com/rishav1305/soul/internal/tutor/store"
)

// ProgressModule handles progress tracking and analytics.
type ProgressModule struct {
	store *store.Store
}

// Progress routes based on the "view" field in input.
// - {"view": "dashboard"} — readiness, module stats, streak, due reviews, today activity
// - {"view": "analytics"} — last 30 days activity, confidence gaps
// - {"view": "topics", "module": "dsa"} — topic list with optional module filter
// - {"view": "mocks"} — mock session list
func (m *ProgressModule) Progress(input map[string]interface{}) (*ToolResult, error) {
	view, _ := input["view"].(string)
	if view == "" {
		view = "dashboard"
	}

	switch view {
	case "dashboard":
		return m.dashboard()
	case "analytics":
		return m.analytics()
	case "topics":
		module, _ := input["module"].(string)
		return m.topics(module)
	case "mocks":
		return m.mocks()
	default:
		return nil, fmt.Errorf("progress: invalid view '%s', must be: dashboard, analytics, topics, mocks", view)
	}
}

func (m *ProgressModule) dashboard() (*ToolResult, error) {
	// Gather module stats.
	modules := []string{"dsa", "ai", "behavioral"}
	type modStat struct {
		Module       string  `json:"module"`
		TopicCount   int     `json:"topicCount"`
		Completed    int     `json:"completed"`
		InProgress   int     `json:"inProgress"`
		CompletionPct float64 `json:"completionPct"`
		AvgScore     float64 `json:"avgScore"`
	}

	var stats []modStat
	totalCompletion := 0.0
	activeModules := 0

	for _, mod := range modules {
		ms, err := m.store.GetModuleStats(mod)
		if err != nil {
			stats = append(stats, modStat{Module: mod})
			continue
		}
		stats = append(stats, modStat{
			Module:        mod,
			TopicCount:    ms.TopicCount,
			Completed:     ms.CompletedCount,
			InProgress:    ms.InProgressCount,
			CompletionPct: ms.CompletionPct,
			AvgScore:      ms.AvgScore,
		})
		if ms.TopicCount > 0 {
			totalCompletion += ms.CompletionPct
			activeModules++
		}
	}

	readiness := 0.0
	if activeModules > 0 {
		readiness = totalCompletion / float64(activeModules)
	}

	// Streak.
	streak, _ := m.store.GetStreak()

	// Due reviews.
	dueReviews, _ := m.store.GetDueReviews(time.Now())
	dueCount := len(dueReviews)

	// Today's activity.
	todayActivity, _ := m.store.GetTodayActivity()

	return &ToolResult{
		Summary: fmt.Sprintf("Dashboard: %.0f%% readiness, %d day streak, %d reviews due", readiness, streak, dueCount),
		Data: map[string]interface{}{
			"readinessPct":  readiness,
			"moduleStats":   stats,
			"streak":        streak,
			"dueReviewCount": dueCount,
			"todayActivity": todayActivity,
		},
	}, nil
}

func (m *ProgressModule) analytics() (*ToolResult, error) {
	// Last 30 days of daily activity — collect by iterating dates.
	type dayEntry struct {
		Date     string                `json:"date"`
		Activity []store.DailyActivity `json:"activity"`
	}

	var days []dayEntry
	for i := 29; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		var dayActivities []store.DailyActivity

		for _, mod := range []string{"dsa", "ai", "behavioral"} {
			activity, err := m.store.GetActivity(date, mod)
			if err == nil && activity != nil {
				dayActivities = append(dayActivities, *activity)
			}
		}

		days = append(days, dayEntry{
			Date:     date,
			Activity: dayActivities,
		})
	}

	// Confidence gaps (>= 20% gap, i.e., 0.2 on a 0-1 scale or 20 on a 0-100 scale).
	gaps, _ := m.store.GetConfidenceGaps(20.0)

	return &ToolResult{
		Summary: fmt.Sprintf("Analytics: 30-day history, %d confidence gaps", len(gaps)),
		Data: map[string]interface{}{
			"last30Days":     days,
			"confidenceGaps": gaps,
		},
	}, nil
}

func (m *ProgressModule) topics(module string) (*ToolResult, error) {
	topics, err := m.store.ListTopics(module, "")
	if err != nil {
		return nil, fmt.Errorf("progress: list topics: %w", err)
	}

	filter := "all modules"
	if module != "" {
		filter = module
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Topics (%s): %d found", filter, len(topics)),
		Data: map[string]interface{}{
			"topics": topics,
			"module": module,
			"count":  len(topics),
		},
	}, nil
}

func (m *ProgressModule) mocks() (*ToolResult, error) {
	sessions, err := m.store.ListMockSessions("")
	if err != nil {
		return nil, fmt.Errorf("progress: list mock sessions: %w", err)
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Mock sessions: %d found", len(sessions)),
		Data: map[string]interface{}{
			"sessions": sessions,
			"count":    len(sessions),
		},
	}, nil
}
