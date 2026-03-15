package scoring

import (
	"math"
	"testing"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestScoreJSONSchema(t *testing.T) {
	config := map[string]interface{}{
		"required_keys": []interface{}{"name", "age", "role"},
		"field_checks": map[string]interface{}{
			"role": []interface{}{"admin", "user", "guest"},
		},
	}

	// Valid JSON with all keys and valid enum
	resp := `{"name": "Alice", "age": 30, "role": "admin"}`
	score := scoreJSONSchema(resp, config)
	// 1 (valid json) + 3 (required keys) + 1 (field check) = 5/5
	if !approxEqual(score, 1.0) {
		t.Errorf("expected 1.0, got %f", score)
	}

	// Missing one key
	resp = `{"name": "Alice", "role": "user"}`
	score = scoreJSONSchema(resp, config)
	// 1 (valid) + 2 (name, role present) + 1 (role valid) = 4/5
	if !approxEqual(score, 0.8) {
		t.Errorf("expected 0.8, got %f", score)
	}

	// JSON in markdown code block
	resp = "```json\n{\"name\": \"Bob\", \"age\": 25, \"role\": \"guest\"}\n```"
	score = scoreJSONSchema(resp, config)
	if !approxEqual(score, 1.0) {
		t.Errorf("expected 1.0 for markdown block, got %f", score)
	}

	// Invalid JSON
	resp = "not json at all"
	score = scoreJSONSchema(resp, config)
	if score != 0.0 {
		t.Errorf("expected 0.0 for invalid json, got %f", score)
	}

	// Invalid enum value
	resp = `{"name": "Alice", "age": 30, "role": "superadmin"}`
	score = scoreJSONSchema(resp, config)
	// 1 (valid) + 3 (keys) + 0 (bad enum) = 4/5
	if !approxEqual(score, 0.8) {
		t.Errorf("expected 0.8 for bad enum, got %f", score)
	}
}

func TestScoreContainsKeywords(t *testing.T) {
	config := map[string]interface{}{
		"keywords": []interface{}{"goroutine", "channel", "mutex", "waitgroup"},
	}

	resp := "Use a goroutine with a channel for concurrency. A Mutex helps with shared state."
	score := scoreContainsKeywords(resp, config)
	// goroutine, channel, mutex match (case insensitive), waitgroup missing = 3/4
	if !approxEqual(score, 0.75) {
		t.Errorf("expected 0.75, got %f", score)
	}

	resp = "goroutine channel mutex waitgroup"
	score = scoreContainsKeywords(resp, config)
	if !approxEqual(score, 1.0) {
		t.Errorf("expected 1.0, got %f", score)
	}

	resp = "nothing relevant here"
	score = scoreContainsKeywords(resp, config)
	if score != 0.0 {
		t.Errorf("expected 0.0, got %f", score)
	}
}

func TestScoreCodeExecutes(t *testing.T) {
	config := map[string]interface{}{}

	// Valid Go function
	resp := "```go\nfunc Add(a, b int) int {\n\treturn a + b\n}\n```"
	score := scoreCodeExecutes(resp, config)
	if score != 1.0 {
		t.Errorf("expected 1.0 for valid code, got %f", score)
	}

	// Invalid code
	resp = "```go\nfunc broken( {\n```"
	score = scoreCodeExecutes(resp, config)
	if score != 0.0 {
		t.Errorf("expected 0.0 for invalid code, got %f", score)
	}

	// Raw valid function (no code block)
	resp = "func Multiply(x, y int) int { return x * y }"
	score = scoreCodeExecutes(resp, config)
	if score != 1.0 {
		t.Errorf("expected 1.0 for raw valid code, got %f", score)
	}
}

func TestScoreOrderedSteps(t *testing.T) {
	config := map[string]interface{}{
		"required_order": []interface{}{"define", "implement", "test", "deploy"},
	}

	// Correct order
	resp := "First define the API, then implement the logic, then test it, and finally deploy."
	score := scoreOrderedSteps(resp, config)
	if !approxEqual(score, 1.0) {
		t.Errorf("expected 1.0, got %f", score)
	}

	// Swap last two: test after deploy
	resp = "First define the API, then implement, then deploy, and finally test."
	score = scoreOrderedSteps(resp, config)
	// define<implement OK, implement<deploy OK, deploy<test FAIL = 2/3
	if !approxEqual(score, 2.0/3.0) {
		t.Errorf("expected 0.667, got %f", score)
	}

	// All reversed
	resp = "deploy test implement define"
	score = scoreOrderedSteps(resp, config)
	if score != 0.0 {
		t.Errorf("expected 0.0 for reversed, got %f", score)
	}
}

func TestScoreExactMatchLabel(t *testing.T) {
	if scoreExactMatchLabel("Binary Search", "  binary search  ") != 1.0 {
		t.Error("expected case-insensitive match")
	}
	if scoreExactMatchLabel("Binary Search", "linear search") != 0.0 {
		t.Error("expected no match")
	}
}

func TestScoreExactMatchNumber(t *testing.T) {
	if scoreExactMatchNumber("42", "The answer is 42") != 1.0 {
		t.Error("expected match for last number")
	}
	if scoreExactMatchNumber("42", "I got 10 then 42") != 1.0 {
		t.Error("expected match for last number 42")
	}
	if scoreExactMatchNumber("42", "The result is 43") != 0.0 {
		t.Error("expected no match")
	}
	if scoreExactMatchNumber("42", "no numbers here") != 0.0 {
		t.Error("expected 0 for no numbers")
	}
}

func TestScoreContainsFunction(t *testing.T) {
	if scoreContainsFunction("func Add", "Here is func Add(a, b int) int") != 1.0 {
		t.Error("expected substring match")
	}
	if scoreContainsFunction("func Add", "Here is func Subtract(a, b int)") != 0.0 {
		t.Error("expected no match")
	}
}

func TestScoreResult_Dispatch(t *testing.T) {
	tests := []struct {
		name     string
		prompt   PromptData
		response string
		want     float64
	}{
		{
			name: "dispatch json_schema",
			prompt: PromptData{
				Scoring: "json_schema",
				ScoringConfig: map[string]interface{}{
					"required_keys": []interface{}{"name"},
				},
			},
			response: `{"name": "test"}`,
			want:     1.0,
		},
		{
			name: "dispatch contains_keywords",
			prompt: PromptData{
				Scoring: "contains_keywords",
				ScoringConfig: map[string]interface{}{
					"keywords": []interface{}{"hello", "world"},
				},
			},
			response: "hello world",
			want:     1.0,
		},
		{
			name: "dispatch code_executes",
			prompt: PromptData{
				Scoring:       "code_executes",
				ScoringConfig: map[string]interface{}{},
			},
			response: "func F() {}",
			want:     1.0,
		},
		{
			name: "dispatch ordered_steps",
			prompt: PromptData{
				Scoring: "ordered_steps",
				ScoringConfig: map[string]interface{}{
					"required_order": []interface{}{"a", "b", "c"},
				},
			},
			response: "a then b then c",
			want:     1.0,
		},
		{
			name: "dispatch exact_match_label",
			prompt: PromptData{
				Scoring:        "exact_match_label",
				ExpectedAnswer: "yes",
			},
			response: "YES",
			want:     1.0,
		},
		{
			name: "dispatch exact_match_number",
			prompt: PromptData{
				Scoring:        "exact_match_number",
				ExpectedAnswer: "7",
			},
			response: "The answer is 7",
			want:     1.0,
		},
		{
			name: "dispatch contains_function",
			prompt: PromptData{
				Scoring:        "contains_function",
				ExpectedAnswer: "func Main",
			},
			response: "func Main() {}",
			want:     1.0,
		},
		{
			name: "dispatch unknown",
			prompt: PromptData{
				Scoring: "unknown_method",
			},
			response: "anything",
			want:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreResult(tt.response, tt.prompt)
			if !approxEqual(got, tt.want) {
				t.Errorf("ScoreResult() = %f, want %f", got, tt.want)
			}
		})
	}
}
