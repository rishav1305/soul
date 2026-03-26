package modules

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rishav1305/soul/internal/tutor/store"
)

// MockModule handles mock interview session creation and job description analysis.
type MockModule struct {
	store *store.Store
}

// skillCategories maps keywords to their module for JD analysis.
var skillCategories = map[string]string{
	// DSA / Programming
	"python":              "dsa",
	"go":                  "dsa",
	"golang":              "dsa",
	"java":                "dsa",
	"c++":                 "dsa",
	"javascript":          "dsa",
	"typescript":          "dsa",
	"rust":                "dsa",
	"algorithms":          "dsa",
	"data structures":     "dsa",
	"leetcode":            "dsa",
	"coding":              "dsa",
	"sql":                 "dsa",
	"nosql":               "dsa",
	"react":               "dsa",
	"node":                "dsa",
	"kubernetes":          "dsa",
	"docker":              "dsa",
	"aws":                 "dsa",
	"gcp":                 "dsa",
	"azure":               "dsa",
	"microservices":       "dsa",
	"distributed systems": "dsa",
	"system design":       "dsa",
	"api":                 "dsa",
	"rest":                "dsa",
	"grpc":                "dsa",
	"graphql":             "dsa",
	"database":            "dsa",
	"redis":               "dsa",
	"kafka":               "dsa",
	"ci/cd":               "dsa",
	// AI/ML
	"machine learning":        "ai",
	"deep learning":           "ai",
	"neural network":          "ai",
	"nlp":                     "ai",
	"natural language":        "ai",
	"computer vision":         "ai",
	"tensorflow":              "ai",
	"pytorch":                 "ai",
	"llm":                     "ai",
	"large language model":    "ai",
	"generative ai":          "ai",
	"reinforcement learning":  "ai",
	"recommendation":          "ai",
	"ml ops":                  "ai",
	"mlops":                   "ai",
	"data science":            "ai",
	// Behavioral
	"leadership":    "behavioral",
	"communication": "behavioral",
	"teamwork":      "behavioral",
	"mentoring":     "behavioral",
	"collaboration": "behavioral",
	"stakeholder":   "behavioral",
	"cross-functional": "behavioral",
	"agile":         "behavioral",
	"scrum":         "behavioral",
	"project management": "behavioral",
}

// MockInterview creates a mock interview session with generated questions.
// Input: {"type": "technical|behavioral|full_loop", "job_description": "optional"}.
func (m *MockModule) MockInterview(input map[string]interface{}) (*ToolResult, error) {
	sessionType, _ := input["type"].(string)
	if sessionType == "" {
		sessionType = "technical"
	}
	jobDesc, _ := input["job_description"].(string)

	// Validate type.
	validTypes := map[string]bool{"technical": true, "behavioral": true, "full_loop": true}
	if !validTypes[sessionType] {
		return nil, fmt.Errorf("mock: invalid type '%s', must be one of: technical, behavioral, full_loop", sessionType)
	}

	// Create session in store.
	session, err := m.store.CreateMockSession(sessionType, jobDesc)
	if err != nil {
		return nil, fmt.Errorf("mock: create session: %w", err)
	}

	// Generate questions based on type.
	questions := generateQuestions(sessionType)

	// Store questions as feedback JSON for retrieval.
	qJSON, _ := json.Marshal(questions)
	m.store.CompleteMockSession(session.ID, 0, string(qJSON))

	// Re-fetch to get updated feedback.
	session, _ = m.store.GetMockSession(session.ID)

	return &ToolResult{
		Summary: fmt.Sprintf("Mock interview created: %s (%d questions)", sessionType, len(questions)),
		Data: map[string]interface{}{
			"session":   session,
			"questions": questions,
		},
	}, nil
}

// AnalyzeJD analyzes a job description to identify skill gaps and suggest a study plan.
// Input: {"text": "job description text"}.
func (m *MockModule) AnalyzeJD(input map[string]interface{}) (*ToolResult, error) {
	text, _ := input["text"].(string)
	if text == "" {
		return nil, fmt.Errorf("mock: analyze_jd requires 'text' field")
	}

	lowerText := strings.ToLower(text)

	// Extract skills by keyword matching.
	type SkillMatch struct {
		Skill  string `json:"skill"`
		Module string `json:"module"`
	}

	var matched []SkillMatch
	seen := make(map[string]bool)
	for keyword, mod := range skillCategories {
		if strings.Contains(lowerText, keyword) && !seen[keyword] {
			seen[keyword] = true
			matched = append(matched, SkillMatch{Skill: keyword, Module: mod})
		}
	}

	// Check which topics are mastered vs. not.
	type GapItem struct {
		Skill    string `json:"skill"`
		Module   string `json:"module"`
		Status   string `json:"status"`
		Mastered bool   `json:"mastered"`
	}

	var gaps []GapItem
	masteredCount := 0

	for _, skill := range matched {
		// Look up topic — try exact match first.
		topics, _ := m.store.ListTopics(skill.Module, "")
		mastered := false
		status := "not_found"

		for _, t := range topics {
			if strings.Contains(strings.ToLower(t.Name), strings.ToLower(skill.Skill)) {
				status = t.Status
				if t.Status == "completed" || t.Status == "mastered" {
					mastered = true
				}
				break
			}
		}

		if mastered {
			masteredCount++
		}
		gaps = append(gaps, GapItem{
			Skill:    skill.Skill,
			Module:   skill.Module,
			Status:   status,
			Mastered: mastered,
		})
	}

	// Build suggested study plan.
	var studyPlan []string
	for _, g := range gaps {
		if !g.Mastered {
			studyPlan = append(studyPlan, fmt.Sprintf("- [%s] %s (current: %s)", g.Module, g.Skill, g.Status))
		}
	}

	readiness := 0.0
	if len(gaps) > 0 {
		readiness = float64(masteredCount) / float64(len(gaps)) * 100
	}

	return &ToolResult{
		Summary: fmt.Sprintf("JD Analysis: %d skills found, %.0f%% readiness", len(matched), readiness),
		Data: map[string]interface{}{
			"skillsFound":    len(matched),
			"masteredCount":  masteredCount,
			"readinessPct":   readiness,
			"gaps":           gaps,
			"suggestedPlan":  studyPlan,
		},
	}, nil
}

// generateQuestions creates interview questions based on session type.
func generateQuestions(sessionType string) []map[string]string {
	var questions []map[string]string

	switch sessionType {
	case "technical":
		questions = []map[string]string{
			{"type": "coding", "question": "Design and implement an LRU cache with O(1) get and put operations. What data structures would you use?"},
			{"type": "coding", "question": "Given a stream of integers, implement a class that finds the median at any point. Explain the time complexity."},
			{"type": "system_design", "question": "Design a URL shortener service that can handle 100M new URLs per day. Discuss storage, routing, and analytics."},
		}

	case "behavioral":
		questions = []map[string]string{
			{"type": "star", "competency": "leadership", "question": "Tell me about a time you led a project that was at risk of failing. What did you do?"},
			{"type": "star", "competency": "conflict", "question": "Describe a situation where you disagreed with a technical decision. How did you handle it?"},
			{"type": "star", "competency": "innovation", "question": "Tell me about a time you introduced a new technology or process to your team."},
			{"type": "star", "competency": "failure", "question": "Tell me about your biggest professional failure. What did you learn?"},
			{"type": "star", "competency": "teamwork", "question": "Describe how you helped a struggling teammate improve their performance."},
		}

	case "full_loop":
		questions = []map[string]string{
			// 2 technical
			{"type": "coding", "question": "Implement a function to detect a cycle in a linked list and return the start of the cycle."},
			{"type": "system_design", "question": "Design a real-time notification system that supports web, mobile, and email channels."},
			// 2 behavioral
			{"type": "star", "competency": "ownership", "question": "Tell me about a time you took ownership of something outside your job description."},
			{"type": "star", "competency": "teamwork", "question": "Describe a time you had to coordinate with multiple teams to deliver a project."},
			// 1 HR
			{"type": "hr", "question": "Where do you see yourself in 5 years, and how does this role fit into that vision?"},
		}
	}

	return questions
}
