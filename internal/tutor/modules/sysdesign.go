package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

// SystemDesignModule handles system design interview prep study tools.
type SystemDesignModule struct {
	store     *store.Store
	evaluator *eval.Evaluator
}

// Learn retrieves a system design topic and returns a framework template for study.
// Input: {"topic_id": N} or {"topic": "name", "category": "cat"}.
func (m *SystemDesignModule) Learn(input map[string]interface{}) (*ToolResult, error) {
	topic, err := m.resolveTopic(input)
	if err != nil {
		return nil, err
	}

	// Mark as learning.
	if err := m.store.UpdateTopicStatus(topic.ID, "learning"); err != nil {
		return nil, fmt.Errorf("sysdesign: update status: %w", err)
	}

	// Build framework template for system design.
	framework := generateSysDesignFramework(topic.Name)

	return &ToolResult{
		Summary: fmt.Sprintf("Learning: %s (%s/%s) — %s difficulty", topic.Name, topic.Module, topic.Category, topic.Difficulty),
		Data: map[string]interface{}{
			"topic":     topic,
			"framework": framework,
		},
	}, nil
}

// Drill handles quiz drilling — either starts a new question or evaluates an answer.
// Start mode: {"topic_id": N} — picks next question.
// Answer mode: {"question_id": N, "answer": "text"} — evaluates answer.
func (m *SystemDesignModule) Drill(input map[string]interface{}) (*ToolResult, error) {
	// Answer mode.
	if qID, ok := getInt64(input, "question_id"); ok {
		return m.evaluateAnswer(context.Background(), qID, input)
	}

	// Start mode.
	topicID, ok := getInt64(input, "topic_id")
	if !ok {
		return nil, fmt.Errorf("sysdesign: drill requires 'topic_id' or 'question_id'")
	}

	question, err := m.store.PickNextQuestion(topicID)
	if err != nil {
		return nil, fmt.Errorf("sysdesign: pick question: %w", err)
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Question: %s", truncate(question.QuestionText, 80)),
		Data: map[string]interface{}{
			"question": question,
			"mode":     "question",
		},
	}, nil
}

// GenerateContent creates a new system design topic with default difficulty "hard".
// Input: {"category": "cat", "name": "topic", "difficulty": "hard"}.
func (m *SystemDesignModule) GenerateContent(input map[string]interface{}) (*ToolResult, error) {
	category, _ := input["category"].(string)
	name, _ := input["name"].(string)
	difficulty, _ := input["difficulty"].(string)
	if category == "" || name == "" {
		return nil, fmt.Errorf("sysdesign: generate requires 'category' and 'name' fields")
	}
	if difficulty == "" {
		difficulty = "hard"
	}

	// Create topic in store.
	topic, err := m.store.CreateTopic("sysdesign", category, name, difficulty, "")
	if err != nil {
		return nil, fmt.Errorf("sysdesign: create topic: %w", err)
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Created topic: sysdesign/%s/%s (difficulty: %s)", category, name, difficulty),
		Data: map[string]interface{}{
			"topic": topic,
		},
	}, nil
}

// evaluateAnswer checks an answer against a quiz question, records the attempt, and updates SM2.
// Uses eval.Evaluator when available, falls back to evaluateWordOverlap.
func (m *SystemDesignModule) evaluateAnswer(ctx context.Context, questionID int64, input map[string]interface{}) (*ToolResult, error) {
	answer, _ := input["answer"].(string)
	if answer == "" {
		return nil, fmt.Errorf("sysdesign: drill answer requires 'answer' field")
	}

	question, err := m.store.GetQuizQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("sysdesign: get question: %w", err)
	}

	var (
		correct   bool
		score     float64
		quality   int
		feedback  string
		keyMissed []string
		keyHit    []string
	)

	// Use Claude evaluator if available, else fall back to word overlap.
	if m.evaluator != nil {
		result, err := m.evaluator.Evaluate(ctx, question.QuestionText, question.AnswerText, answer)
		if err == nil {
			correct = result.Correct
			score = result.Score
			quality = result.Quality
			feedback = result.Feedback
			keyMissed = result.KeyMissed
			keyHit = result.KeyHit
		} else {
			// Evaluator error — fall back.
			correct = evaluateWordOverlap(answer, question.AnswerText, 0.5)
			if correct {
				score = 100.0
				quality = 4
			} else {
				score = 0.0
				quality = 2
			}
		}
	} else {
		correct = evaluateWordOverlap(answer, question.AnswerText, 0.5)
		if correct {
			score = 100.0
			quality = 4
		} else {
			score = 0.0
			quality = 2
		}
	}

	// Record progress.
	prog, err := m.store.RecordProgress(question.TopicID, score, 1, boolToInt(correct), 0, "")
	if err != nil {
		return nil, fmt.Errorf("sysdesign: record progress: %w", err)
	}

	// Record attempt.
	if _, err := m.store.RecordAttempt(questionID, prog.ID, correct, 0, answer); err != nil {
		return nil, fmt.Errorf("sysdesign: record attempt: %w", err)
	}

	// Update daily activity.
	today := time.Now().Format("2006-01-02")
	topic, _ := m.store.GetTopic(question.TopicID)
	moduleName := "sysdesign"
	if topic != nil {
		moduleName = topic.Module
	}
	m.store.UpsertDailyActivity(today, moduleName, 0, 1, 1, score)

	// Update spaced repetition.
	sr, _ := m.store.GetSpacedRep(question.TopicID)
	currentInterval := 1.0
	currentEF := 2.5
	currentReps := 0
	if sr != nil {
		currentInterval = float64(sr.IntervalDays)
		currentEF = sr.EaseFactor
		currentReps = sr.RepetitionCount
	}

	sm2 := SM2Update(quality, currentInterval, currentEF, currentReps)
	m.store.UpsertSpacedRep(question.TopicID, sm2.NextReview, int(sm2.IntervalDays), sm2.EaseFactor, sm2.RepetitionCount)

	// Update topic status based on SR progress.
	topicStatus := "in_progress"
	if sm2.EaseFactor >= 2.5 && sm2.RepetitionCount >= 3 {
		topicStatus = "mastered"
	}
	m.store.UpdateTopicStatus(question.TopicID, topicStatus)

	status := "incorrect"
	if correct {
		status = "correct"
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Answer: %s — %s", status, truncate(question.AnswerText, 80)),
		Data: map[string]interface{}{
			"status":      status,
			"correct":     correct,
			"score":       score,
			"quality":     quality,
			"feedback":    feedback,
			"keyMissed":   keyMissed,
			"keyHit":      keyHit,
			"answer":      question.AnswerText,
			"explanation": question.Explanation,
			"nextReview":  sm2.NextReview.Format("2006-01-02"),
			"mode":        "result",
		},
	}, nil
}

// resolveTopic resolves a topic from either topic_id or topic+category, searching module="sysdesign".
func (m *SystemDesignModule) resolveTopic(input map[string]interface{}) (*store.Topic, error) {
	if topicID, ok := getInt64(input, "topic_id"); ok {
		return m.store.GetTopic(topicID)
	}

	topicName, _ := input["topic"].(string)
	category, _ := input["category"].(string)
	if topicName == "" {
		return nil, fmt.Errorf("sysdesign: requires 'topic_id' or 'topic' + 'category'")
	}

	// Search with module=sysdesign if category is provided.
	if category != "" {
		return m.store.GetTopicByName("sysdesign", category, topicName)
	}

	// Fallback: search all sysdesign topics.
	topics, err := m.store.ListTopics("sysdesign", "")
	if err != nil {
		return nil, err
	}
	for i := range topics {
		if strings.EqualFold(topics[i].Name, topicName) {
			return &topics[i], nil
		}
	}
	return nil, fmt.Errorf("sysdesign: topic not found: %s", topicName)
}

// generateSysDesignFramework returns a framework template for system design study.
func generateSysDesignFramework(topicName string) string {
	return fmt.Sprintf(`# System Design Framework: %s

## 1. Requirements Clarification
- Functional requirements: what the system must do
- Non-functional requirements: scale, latency, availability, consistency
- Constraints: read/write ratio, data size, QPS estimates

## 2. Capacity Estimation
- Daily/monthly active users
- Storage requirements (per record × records)
- Bandwidth (read QPS × avg response size)
- Memory (cache hit rate × working set)

## 3. High-Level Design
- Core components: clients, load balancers, app servers, databases, caches
- Data flow for primary use case
- API design (REST endpoints, request/response shapes)

## 4. Detailed Design
- Database schema: tables, indexes, sharding key
- Caching strategy: what to cache, eviction policy, TTL
- Message queues: async processing, fan-out patterns
- Service-to-service communication: sync vs async

## 5. Scalability
- Horizontal scaling of stateless services
- Database sharding / read replicas
- CDN for static assets
- Rate limiting and circuit breakers

## 6. Reliability & Fault Tolerance
- Single points of failure
- Replication and failover
- Data backup and recovery
- Graceful degradation

## 7. Trade-offs
- SQL vs NoSQL — consistency vs availability
- Strong vs eventual consistency
- Monolith vs microservices
- Push vs pull model

## Interview Tips for %s
- Start with clarifying questions (2-3 min)
- Draw the diagram while explaining
- Mention trade-offs explicitly
- Quantify with back-of-envelope math
- Propose a simple design first, then optimize
`, topicName, topicName)
}

// sysDesignContentTemplate returns a markdown content template for a new sysdesign topic.
func sysDesignContentTemplate(name string) string {
	return fmt.Sprintf(`# %s

## Overview
[High-level description of %s]

## Key Components
- Component 1
- Component 2
- Component 3

## Data Model
[Describe the main entities and relationships]

## Scalability Considerations
- Horizontal scaling approach
- Caching strategy
- Database sharding

## Common Interview Questions
- Question 1
- Question 2

## Reference Architectures
[Links or descriptions of real-world implementations]
`, name, name)
}

// writeSysDesignContent writes a markdown template to the content directory.
// Returns the relative path written, or empty string on failure (non-fatal).
func writeSysDesignContent(contentDir, category, name string) string {
	if contentDir == "" {
		return ""
	}
	relPath := filepath.Join("sysdesign", category, name+".md")
	fullPath := filepath.Join(contentDir, relPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return ""
	}
	content := sysDesignContentTemplate(name)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return ""
	}
	return relPath
}
