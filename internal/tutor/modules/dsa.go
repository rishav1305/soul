package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rishav1305/soul/internal/tutor/eval"
	"github.com/rishav1305/soul/internal/tutor/store"
)

// DSAModule handles data structures and algorithms study tools.
type DSAModule struct {
	store      *store.Store
	contentDir string
	evaluator  *eval.Evaluator
}

// Learn retrieves a topic and its content for study.
// Input: {"topic_id": N} or {"topic": "name", "category": "cat"}.
func (m *DSAModule) Learn(input map[string]interface{}) (*ToolResult, error) {
	topic, err := m.resolveTopic(input)
	if err != nil {
		return nil, err
	}

	// Mark as learning.
	if err := m.store.UpdateTopicStatus(topic.ID, "learning"); err != nil {
		return nil, fmt.Errorf("dsa: update status: %w", err)
	}

	// Read content file if set.
	var content string
	if topic.ContentPath != "" {
		fullPath := topic.ContentPath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(m.contentDir, fullPath)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			content = fmt.Sprintf("[Content file not found: %s]", fullPath)
		} else {
			content = string(data)
		}
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Learning: %s (%s/%s) — %s difficulty", topic.Name, topic.Module, topic.Category, topic.Difficulty),
		Data: map[string]interface{}{
			"topic":   topic,
			"content": content,
		},
	}, nil
}

// Build returns a 5-step implementation guide for a topic.
// Input: {"topic": "name"}.
func (m *DSAModule) Build(input map[string]interface{}) (*ToolResult, error) {
	topicName, _ := input["topic"].(string)
	if topicName == "" {
		return nil, fmt.Errorf("dsa: build requires 'topic' field")
	}

	guide := fmt.Sprintf(`# Implementation Guide: %s

## Step 1: Interface Design
Define the public API — what operations does this data structure/algorithm expose?
Think about input types, return types, and error conditions.

## Step 2: Core Implementation
Implement the main logic. Start with the simplest correct version.
Focus on correctness before optimization.

## Step 3: Edge Cases
Handle empty inputs, single elements, duplicates, overflow, nil pointers.
List all edge cases before writing code.

## Step 4: Tests
Write test cases covering:
- Normal operation
- Edge cases from Step 3
- Performance characteristics (if applicable)

## Step 5: Complexity Analysis
- Time complexity: analyze each operation
- Space complexity: identify auxiliary storage
- Compare with alternative approaches`, topicName)

	return &ToolResult{
		Summary: fmt.Sprintf("Build guide for: %s", topicName),
		Data: map[string]interface{}{
			"topic": topicName,
			"guide": guide,
		},
	}, nil
}

// Drill handles quiz drilling — either starts a new question or evaluates an answer.
// Start mode: {"topic_id": N} — picks next question.
// Answer mode: {"question_id": N, "answer": "text"} — evaluates answer.
func (m *DSAModule) Drill(input map[string]interface{}) (*ToolResult, error) {
	// Answer mode.
	if qID, ok := getInt64(input, "question_id"); ok {
		return m.evaluateAnswer(context.Background(), qID, input)
	}

	// Start mode.
	topicID, ok := getInt64(input, "topic_id")
	if !ok {
		return nil, fmt.Errorf("dsa: drill requires 'topic_id' or 'question_id'")
	}

	question, err := m.store.PickNextQuestion(topicID)
	if err != nil {
		return nil, fmt.Errorf("dsa: pick question: %w", err)
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Question: %s", truncate(question.QuestionText, 80)),
		Data: map[string]interface{}{
			"question": question,
			"mode":     "question",
		},
	}, nil
}

// evaluateAnswer checks an answer against a quiz question, records the attempt, and updates SM2.
// Uses Claude evaluator if available, falls back to word-overlap scoring.
func (m *DSAModule) evaluateAnswer(ctx context.Context, questionID int64, input map[string]interface{}) (*ToolResult, error) {
	answer, _ := input["answer"].(string)
	if answer == "" {
		return nil, fmt.Errorf("dsa: drill answer requires 'answer' field")
	}

	question, err := m.store.GetQuizQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("dsa: get question: %w", err)
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
			// Evaluator error — fall back to word overlap.
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
		return nil, fmt.Errorf("dsa: record progress: %w", err)
	}

	// Record attempt.
	if _, err := m.store.RecordAttempt(questionID, prog.ID, correct, 0, answer); err != nil {
		return nil, fmt.Errorf("dsa: record attempt: %w", err)
	}

	// Update daily activity.
	today := time.Now().Format("2006-01-02")
	topic, _ := m.store.GetTopic(question.TopicID)
	moduleName := "dsa"
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

// Solve returns a 4-step walkthrough for a problem.
// Input: {"topic": "name", "problem": "description"}.
func (m *DSAModule) Solve(input map[string]interface{}) (*ToolResult, error) {
	topicName, _ := input["topic"].(string)
	problem, _ := input["problem"].(string)
	if topicName == "" || problem == "" {
		return nil, fmt.Errorf("dsa: solve requires 'topic' and 'problem' fields")
	}

	walkthrough := fmt.Sprintf(`# Problem Walkthrough: %s

**Problem:** %s

## Step 1: Understand
- Restate the problem in your own words
- Identify inputs and outputs
- Clarify constraints and assumptions
- Work through 1-2 examples by hand

## Step 2: Identify Pattern
- What category does this problem fall into?
- What data structures are applicable?
- Have you seen similar problems before?
- What is the brute force approach?

## Step 3: Solve
- Design the algorithm step by step
- Implement the solution
- Trace through with your examples

## Step 4: Analyze
- Time complexity
- Space complexity
- Can you optimize further?
- What are the trade-offs?`, topicName, problem)

	return &ToolResult{
		Summary: fmt.Sprintf("Walkthrough for: %s", topicName),
		Data: map[string]interface{}{
			"topic":       topicName,
			"problem":     problem,
			"walkthrough": walkthrough,
		},
	}, nil
}

// GenerateContent creates a topic and writes a markdown content file.
// Input: {"module": "dsa", "category": "cat", "name": "topic", "difficulty": "easy"}.
func (m *DSAModule) GenerateContent(input map[string]interface{}) (*ToolResult, error) {
	module, _ := input["module"].(string)
	if module == "" {
		module = "dsa"
	}
	category, _ := input["category"].(string)
	name, _ := input["name"].(string)
	difficulty, _ := input["difficulty"].(string)
	if category == "" || name == "" {
		return nil, fmt.Errorf("dsa: generate requires 'category' and 'name' fields")
	}
	if difficulty == "" {
		difficulty = "medium"
	}

	// Build content path.
	relPath := filepath.Join("dsa", category, name+".md")
	fullPath := filepath.Join(m.contentDir, relPath)

	// Ensure directory exists.
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("dsa: create content dir: %w", err)
	}

	// Write markdown template.
	content := fmt.Sprintf(`# %s

## Overview
[Description of %s]

## Key Concepts
- Concept 1
- Concept 2
- Concept 3

## Implementation
[Implementation details]

## Complexity
- Time: O(?)
- Space: O(?)

## Common Patterns
[Related patterns and variations]

## Interview Tips
[Key points to mention in interviews]
`, name, name)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("dsa: write content: %w", err)
	}

	// Create topic in store.
	topic, err := m.store.CreateTopic(module, category, name, difficulty, relPath)
	if err != nil {
		return nil, fmt.Errorf("dsa: create topic: %w", err)
	}

	return &ToolResult{
		Summary: fmt.Sprintf("Created topic: %s/%s/%s with content at %s", module, category, name, relPath),
		Data: map[string]interface{}{
			"topic":       topic,
			"contentPath": relPath,
		},
	}, nil
}

// resolveTopic resolves a topic from either topic_id or topic+category.
func (m *DSAModule) resolveTopic(input map[string]interface{}) (*store.Topic, error) {
	if topicID, ok := getInt64(input, "topic_id"); ok {
		return m.store.GetTopic(topicID)
	}

	topicName, _ := input["topic"].(string)
	category, _ := input["category"].(string)
	if topicName == "" {
		return nil, fmt.Errorf("dsa: requires 'topic_id' or 'topic' + 'category'")
	}

	// Search with module=dsa if category is provided.
	if category != "" {
		return m.store.GetTopicByName("dsa", category, topicName)
	}

	// Fallback: search all dsa topics.
	topics, err := m.store.ListTopics("dsa", "")
	if err != nil {
		return nil, err
	}
	for i := range topics {
		if strings.EqualFold(topics[i].Name, topicName) {
			return &topics[i], nil
		}
	}
	return nil, fmt.Errorf("dsa: topic not found: %s", topicName)
}

// --- helpers ---

func evaluateWordOverlap(userAnswer, correctAnswer string, threshold float64) bool {
	userWords := toWordSet(strings.ToLower(userAnswer))
	correctWords := toWordSet(strings.ToLower(correctAnswer))

	if len(correctWords) == 0 {
		return true
	}

	overlap := 0
	for w := range correctWords {
		if userWords[w] {
			overlap++
		}
	}

	return float64(overlap)/float64(len(correctWords)) >= threshold
}

func toWordSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range strings.Fields(s) {
		// Strip common punctuation.
		w = strings.Trim(w, ".,;:!?\"'()[]{}/-")
		if w != "" {
			set[w] = true
		}
	}
	return set
}

func getInt64(m map[string]interface{}, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
