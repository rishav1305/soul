package scoring

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// PromptData represents a benchmark prompt with scoring config.
type PromptData struct {
	ID             string                 `json:"id"`
	Task           string                 `json:"task"`
	Prompt         string                 `json:"prompt"`
	ExpectedAnswer string                 `json:"expected_answer"`
	Scoring        string                 `json:"scoring"`
	ScoringConfig  map[string]interface{} `json:"scoring_config"`
}

// ScoreResult dispatches to the correct scoring method.
func ScoreResult(response string, prompt PromptData) float64 {
	switch prompt.Scoring {
	case "json_schema":
		return scoreJSONSchema(response, prompt.ScoringConfig)
	case "contains_keywords":
		return scoreContainsKeywords(response, prompt.ScoringConfig)
	case "code_executes":
		return scoreCodeExecutes(response, prompt.ScoringConfig)
	case "ordered_steps":
		return scoreOrderedSteps(response, prompt.ScoringConfig)
	case "exact_match_label":
		return scoreExactMatchLabel(prompt.ExpectedAnswer, response)
	case "exact_match_number":
		return scoreExactMatchNumber(prompt.ExpectedAnswer, response)
	case "contains_function":
		return scoreContainsFunction(prompt.ExpectedAnswer, response)
	default:
		return 0.0
	}
}

// scoreJSONSchema checks that the response contains valid JSON with required keys and field values.
func scoreJSONSchema(response string, config map[string]interface{}) float64 {
	jsonStr := extractJSON(response)

	totalChecks := 0
	passedChecks := 0

	// Check 1: valid JSON
	totalChecks++
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return 0.0
	}
	passedChecks++

	// Check required keys
	if keys, ok := config["required_keys"]; ok {
		if keyList, ok := toStringSlice(keys); ok {
			for _, key := range keyList {
				totalChecks++
				if _, exists := parsed[key]; exists {
					passedChecks++
				}
			}
		}
	}

	// Check field_checks (enum validation)
	if checks, ok := config["field_checks"]; ok {
		if checkMap, ok := checks.(map[string]interface{}); ok {
			for field, constraint := range checkMap {
				totalChecks++
				val, exists := parsed[field]
				if !exists {
					continue
				}
				valStr := fmt.Sprintf("%v", val)
				if enumList, ok := toStringSlice(constraint); ok {
					for _, allowed := range enumList {
						if valStr == allowed {
							passedChecks++
							break
						}
					}
				}
			}
		}
	}

	if totalChecks == 0 {
		return 0.0
	}
	return float64(passedChecks) / float64(totalChecks)
}

// scoreContainsKeywords checks for case-insensitive keyword presence.
func scoreContainsKeywords(response string, config map[string]interface{}) float64 {
	keywords, ok := config["keywords"]
	if !ok {
		return 0.0
	}
	keywordList, ok := toStringSlice(keywords)
	if !ok || len(keywordList) == 0 {
		return 0.0
	}

	lower := strings.ToLower(response)
	matches := 0
	for _, kw := range keywordList {
		if strings.Contains(lower, strings.ToLower(kw)) {
			matches++
		}
	}
	return float64(matches) / float64(len(keywordList))
}

// scoreCodeExecutes checks that Go code in the response parses successfully.
func scoreCodeExecutes(response string, config map[string]interface{}) float64 {
	code := extractCodeBlock(response)

	// Wrap in package if needed for parser
	src := code
	if !strings.Contains(src, "package ") {
		src = "package main\n" + src
	}

	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "", src, parser.AllErrors)
	if err != nil {
		return 0.0
	}
	return 1.0
}

// scoreOrderedSteps checks that steps appear in the required order.
func scoreOrderedSteps(response string, config map[string]interface{}) float64 {
	order, ok := config["required_order"]
	if !ok {
		return 0.0
	}
	steps, ok := toStringSlice(order)
	if !ok || len(steps) < 2 {
		return 0.0
	}

	lower := strings.ToLower(response)
	constraints := len(steps) - 1
	met := 0

	for i := 0; i < constraints; i++ {
		posA := strings.Index(lower, strings.ToLower(steps[i]))
		posB := strings.Index(lower, strings.ToLower(steps[i+1]))
		if posA >= 0 && posB >= 0 && posA < posB {
			met++
		}
	}

	return float64(met) / float64(constraints)
}

// scoreExactMatchLabel compares trimmed lowercase strings.
func scoreExactMatchLabel(expected, response string) float64 {
	if strings.TrimSpace(strings.ToLower(expected)) == strings.TrimSpace(strings.ToLower(response)) {
		return 1.0
	}
	return 0.0
}

// scoreExactMatchNumber extracts the last standalone number from the response and compares.
func scoreExactMatchNumber(expected, response string) float64 {
	re := regexp.MustCompile(`\b(\d+(?:\.\d+)?)\b`)
	matches := re.FindAllString(response, -1)
	if len(matches) == 0 {
		return 0.0
	}
	last := matches[len(matches)-1]
	if strings.TrimSpace(last) == strings.TrimSpace(expected) {
		return 1.0
	}
	return 0.0
}

// scoreContainsFunction checks if the expected string is a substring of the response.
func scoreContainsFunction(expected, response string) float64 {
	if strings.Contains(response, expected) {
		return 1.0
	}
	return 0.0
}

// extractJSON tries to pull JSON from a markdown code block or returns the raw string.
func extractJSON(s string) string {
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\n?(.*?)```")
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(s)
}

// extractCodeBlock pulls code from a markdown code block or returns raw input.
func extractCodeBlock(s string) string {
	re := regexp.MustCompile("(?s)```(?:\\w+)?\\s*\n?(.*?)```")
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(s)
}

// toStringSlice converts an interface{} (expected []interface{} of strings) to []string.
func toStringSlice(v interface{}) ([]string, bool) {
	switch val := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result, len(result) > 0
	case []string:
		return val, len(val) > 0
	default:
		return nil, false
	}
}
