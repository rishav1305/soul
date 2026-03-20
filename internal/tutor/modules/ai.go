package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rishav1305/soul-v2/internal/tutor/eval"
	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

// AIModule handles AI/ML interview prep study tools.
type AIModule struct {
	store      *store.Store
	contentDir string
	evaluator  *eval.Evaluator
}

// LearnTheory retrieves or creates an AI theory topic and returns content at the given depth.
// Input: {"topic": "name", "category": "theory", "depth": "overview|detailed|deep"}.
func (m *AIModule) LearnTheory(input map[string]interface{}) (*ToolResult, error) {
	topicName, _ := input["topic"].(string)
	if topicName == "" {
		return nil, fmt.Errorf("ai: learn_theory requires 'topic' field")
	}
	category, _ := input["category"].(string)
	if category == "" {
		category = "theory"
	}
	depth, _ := input["depth"].(string)
	if depth == "" {
		depth = "overview"
	}

	// Auto-create topic if missing.
	topic, err := m.store.GetTopicByName("ai", category, topicName)
	if err != nil {
		topic, err = m.store.CreateTopic("ai", category, topicName, "medium", "")
		if err != nil {
			return nil, fmt.Errorf("ai: create topic: %w", err)
		}
	}

	// Mark as learning.
	m.store.UpdateTopicStatus(topic.ID, "learning")

	content := generateTheoryContent(topicName, depth)

	return &ToolResult{
		Summary: fmt.Sprintf("AI Theory: %s (%s depth)", topicName, depth),
		Data: map[string]interface{}{
			"topic":   topic,
			"depth":   depth,
			"content": content,
		},
	}, nil
}

// DrillTheory handles quiz drilling for AI theory topics.
// Start mode: {"topic_id": N} — picks next question.
// Answer mode: {"question_id": N, "answer": "text"} — evaluates answer.
func (m *AIModule) DrillTheory(input map[string]interface{}) (*ToolResult, error) {
	// Answer mode.
	if qID, ok := getInt64(input, "question_id"); ok {
		return m.evaluateAnswer(context.Background(), qID, input)
	}

	// Start mode.
	topicID, ok := getInt64(input, "topic_id")
	if !ok {
		return nil, fmt.Errorf("ai: drill requires 'topic_id' or 'question_id'")
	}

	question, err := m.store.PickNextQuestion(topicID)
	if err != nil {
		return nil, fmt.Errorf("ai: pick question: %w", err)
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
func (m *AIModule) evaluateAnswer(ctx context.Context, questionID int64, input map[string]interface{}) (*ToolResult, error) {
	answer, _ := input["answer"].(string)
	if answer == "" {
		return nil, fmt.Errorf("ai: drill answer requires 'answer' field")
	}

	question, err := m.store.GetQuizQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("ai: get question: %w", err)
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

	prog, err := m.store.RecordProgress(question.TopicID, score, 1, boolToInt(correct), 0, "")
	if err != nil {
		return nil, fmt.Errorf("ai: record progress: %w", err)
	}

	if _, err := m.store.RecordAttempt(questionID, prog.ID, correct, 0, answer); err != nil {
		return nil, fmt.Errorf("ai: record attempt: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	m.store.UpsertDailyActivity(today, "ai", 0, 1, 1, score)

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

// GenerateAIContent returns a 6-section content outline for an AI topic.
// Input: {"category": "cat", "name": "topic"}.
func (m *AIModule) GenerateAIContent(input map[string]interface{}) (*ToolResult, error) {
	category, _ := input["category"].(string)
	name, _ := input["name"].(string)
	if category == "" || name == "" {
		return nil, fmt.Errorf("ai: generate requires 'category' and 'name' fields")
	}

	outline := fmt.Sprintf(`# %s — Content Outline

## 1. Introduction
- What is %s?
- Why does it matter in AI/ML?
- Historical context and evolution

## 2. Core Concepts
- Fundamental principles
- Key terminology
- How it relates to other concepts

## 3. Mathematical Foundations
- Key equations and formulas
- Statistical underpinnings
- Proof sketches for important theorems

## 4. Implementation
- Algorithm pseudocode
- Common libraries and frameworks
- Step-by-step coding walkthrough

## 5. Applications
- Real-world use cases
- Industry applications
- Research frontiers

## 6. Interview Relevance
- Common interview questions
- Key talking points
- How to explain to non-technical audiences
`, name, name)

	return &ToolResult{
		Summary: fmt.Sprintf("AI content outline: %s/%s", category, name),
		Data: map[string]interface{}{
			"category": category,
			"name":     name,
			"outline":  outline,
		},
	}, nil
}

// generateTheoryContent returns theory explanation at the given depth level.
func generateTheoryContent(topic string, depth string) string {
	normalized := strings.ToLower(topic)
	_ = normalized

	switch depth {
	case "deep":
		return fmt.Sprintf(`# %s — Deep Dive

## Theoretical Foundations
In-depth exploration of the mathematical and theoretical underpinnings.
Covers proofs, derivations, and edge cases.

## Advanced Implementation Details
Production-level considerations, optimization techniques,
numerical stability, and scaling strategies.

## Research Frontiers
Recent papers, open problems, and emerging directions.

## Common Pitfalls
Subtle bugs, misconceptions, and implementation traps
that catch even experienced practitioners.

## Interview Deep Questions
Questions that test true understanding vs. surface knowledge.
`, topic)

	case "detailed":
		return fmt.Sprintf(`# %s — Detailed Overview

## Core Theory
Explanation of key principles with examples.
Covers the "why" behind the approach.

## Implementation Guide
Practical coding walkthrough with key decisions explained.
Includes common patterns and anti-patterns.

## Worked Examples
Step-by-step solutions to representative problems.

## Key Interview Points
What interviewers look for when asking about this topic.
`, topic)

	default: // "overview"
		return fmt.Sprintf(`# %s — Overview

## What Is It?
High-level description and intuition.

## Key Ideas
- Core concept 1
- Core concept 2
- Core concept 3

## When to Use
Common scenarios and selection criteria.

## Quick Reference
Essential formulas and complexity bounds.
`, topic)
	}
}
