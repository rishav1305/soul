package modules

import (
	"fmt"
	"strings"

	"github.com/rishav1305/soul/internal/tutor/store"
)

// BehavioralModule handles behavioral interview prep tools.
type BehavioralModule struct {
	store *store.Store
}

// Valid competencies for STAR stories.
var validCompetencies = map[string]bool{
	"leadership":  true,
	"conflict":    true,
	"failure":     true,
	"teamwork":    true,
	"innovation":  true,
	"ownership":   true,
}

// HR question bank — at least 2-3 questions per category.
var hrQuestionBank = map[string][]string{
	"salary": {
		"What are your salary expectations for this role?",
		"How do you determine your market value?",
		"Would you be willing to accept a lower salary for better growth opportunities?",
	},
	"gaps": {
		"I notice a gap in your employment history. Can you explain what happened during that time?",
		"What did you do to stay current with technology during your career break?",
		"How did your time away from work change your professional perspective?",
	},
	"weaknesses": {
		"What is your greatest weakness?",
		"Tell me about a skill you are currently working to improve.",
		"How do you handle tasks that fall outside your comfort zone?",
	},
	"conflict": {
		"Describe a time you had a disagreement with a colleague. How did you resolve it?",
		"Tell me about a time you received negative feedback. How did you respond?",
		"How do you handle working with someone whose work style differs from yours?",
	},
	"motivation": {
		"What motivates you in your work?",
		"What kind of work environment brings out your best performance?",
		"Describe a project that truly excited you and explain why.",
	},
	"why_leaving": {
		"Why are you looking to leave your current position?",
		"What would need to change at your current company for you to stay?",
		"How does this role compare to what you are doing now?",
	},
	"where_5_years": {
		"Where do you see yourself in 5 years?",
		"What are your long-term career goals?",
		"How does this role fit into your career plan?",
	},
}

// BuildNarrative returns a "Tell me about yourself" template.
// Input: {"focus": "optional focus area"}.
func (m *BehavioralModule) BuildNarrative(input map[string]interface{}) (*ToolResult, error) {
	focus, _ := input["focus"].(string)

	sections := []struct {
		Name   string
		Prompt string
	}{
		{"Opening Hook", "Start with a compelling one-liner that captures your professional identity. Example: 'I am a backend engineer who thrives on building systems that scale to millions of users.'"},
		{"Career Journey", "Walk through 2-3 key career transitions. Focus on growth, not just titles. What skills did you build at each stage?"},
		{"Current Role", "What are you doing now? What is the most impactful project you have worked on recently? Quantify results."},
		{"Why This Company", "Connect your background to this specific opportunity. What excites you about their mission, product, or technical challenges?"},
		{"Closing Statement", "End with a forward-looking statement. What do you want to accomplish next, and how does this role enable that?"},
	}

	var builder strings.Builder
	builder.WriteString("# Tell Me About Yourself\n\n")

	if focus != "" {
		builder.WriteString(fmt.Sprintf("**Focus area: %s** — Emphasize this section in your response.\n\n", focus))
	}

	for i, s := range sections {
		emphasized := ""
		if focus != "" && strings.Contains(strings.ToLower(s.Name), strings.ToLower(focus)) {
			emphasized = " **[EMPHASIZE THIS SECTION]**"
		}
		builder.WriteString(fmt.Sprintf("## %d. %s%s\n%s\n\n", i+1, s.Name, emphasized, s.Prompt))
	}

	builder.WriteString("## Tips\n")
	builder.WriteString("- Keep it under 2 minutes\n")
	builder.WriteString("- Practice transitions between sections\n")
	builder.WriteString("- Tailor the 'Why This Company' section for each interview\n")
	builder.WriteString("- End confidently — do not trail off\n")

	narrative := builder.String()

	return &ToolResult{
		Summary: "Tell Me About Yourself — narrative template",
		Data: map[string]interface{}{
			"narrative": narrative,
			"focus":     focus,
		},
	}, nil
}

// BuildStar returns a STAR story template or existing story for a competency.
// Input: {"competency": "leadership|conflict|failure|teamwork|innovation|ownership"}.
func (m *BehavioralModule) BuildStar(input map[string]interface{}) (*ToolResult, error) {
	competency, _ := input["competency"].(string)
	if competency == "" {
		return nil, fmt.Errorf("behavioral: build_star requires 'competency' field")
	}

	competency = strings.ToLower(competency)
	if !validCompetencies[competency] {
		valid := make([]string, 0, len(validCompetencies))
		for k := range validCompetencies {
			valid = append(valid, k)
		}
		return nil, fmt.Errorf("behavioral: invalid competency '%s', must be one of: %s", competency, strings.Join(valid, ", "))
	}

	// Check for existing story.
	story, err := m.store.GetStarStory(competency)
	if err == nil && story != nil {
		return &ToolResult{
			Summary: fmt.Sprintf("STAR story found for: %s (version %d)", competency, story.Version),
			Data: map[string]interface{}{
				"story":    story,
				"existing": true,
			},
		}, nil
	}

	// Return template with prompts.
	template := map[string]string{
		"competency": competency,
		"situation":  fmt.Sprintf("[Describe the context. What was the %s challenge you faced? Set the scene — team size, timeline, stakes.]", competency),
		"task":       "[What was your specific responsibility? What was expected of you? What constraints did you face?]",
		"action":     "[Detail the specific steps YOU took. Use 'I' not 'we'. Include your reasoning for key decisions.]",
		"result":     "[Quantify the outcome. Revenue impact, time saved, team growth, process improvement. What did you learn?]",
	}

	return &ToolResult{
		Summary: fmt.Sprintf("STAR template for: %s — fill in the sections", competency),
		Data: map[string]interface{}{
			"template": template,
			"existing": false,
		},
	}, nil
}

// DrillHR handles HR interview question drilling.
// Question mode: {"category": "salary|gaps|weaknesses|conflict|motivation|why_leaving|where_5_years"}.
// Evaluate mode: {"category": "...", "answer": "text"}.
func (m *BehavioralModule) DrillHR(input map[string]interface{}) (*ToolResult, error) {
	category, _ := input["category"].(string)
	if category == "" {
		return nil, fmt.Errorf("behavioral: drill_hr requires 'category' field")
	}

	questions, ok := hrQuestionBank[category]
	if !ok {
		cats := make([]string, 0, len(hrQuestionBank))
		for k := range hrQuestionBank {
			cats = append(cats, k)
		}
		return nil, fmt.Errorf("behavioral: invalid HR category '%s', must be one of: %s", category, strings.Join(cats, ", "))
	}

	// Evaluate mode.
	answer, hasAnswer := input["answer"].(string)
	if hasAnswer && answer != "" {
		return m.evaluateHRAnswer(category, answer)
	}

	// Question mode — pick a question (rotate based on simple hash).
	idx := len(category) % len(questions)
	question := questions[idx]

	return &ToolResult{
		Summary: fmt.Sprintf("HR Question (%s): %s", category, truncate(question, 60)),
		Data: map[string]interface{}{
			"category": category,
			"question": question,
			"mode":     "question",
		},
	}, nil
}

// evaluateHRAnswer scores an HR answer based on several criteria.
func (m *BehavioralModule) evaluateHRAnswer(category, answer string) (*ToolResult, error) {
	words := strings.Fields(answer)
	wordCount := len(words)

	score := 0.0
	feedback := make([]string, 0)
	maxScore := 4.0

	// Criterion 1: Length >= 10 words.
	if wordCount >= 10 {
		score++
		feedback = append(feedback, "Good length — sufficient detail provided.")
	} else {
		feedback = append(feedback, fmt.Sprintf("Too brief (%d words). Aim for at least 10 words with specific details.", wordCount))
	}

	// Criterion 2: Contains examples (signal words).
	lowerAnswer := strings.ToLower(answer)
	exampleSignals := []string{"for example", "for instance", "such as", "when i", "one time", "at my", "in my", "specifically", "i remember"}
	hasExample := false
	for _, signal := range exampleSignals {
		if strings.Contains(lowerAnswer, signal) {
			hasExample = true
			break
		}
	}
	if hasExample {
		score++
		feedback = append(feedback, "Good use of specific examples.")
	} else {
		feedback = append(feedback, "Add specific examples to make your answer more compelling.")
	}

	// Criterion 3: Positive framing.
	positiveSignals := []string{"learned", "grew", "improved", "achieved", "opportunity", "excited", "passionate", "motivated", "driven", "goal"}
	hasPositive := false
	for _, signal := range positiveSignals {
		if strings.Contains(lowerAnswer, signal) {
			hasPositive = true
			break
		}
	}
	if hasPositive {
		score++
		feedback = append(feedback, "Good positive framing — shows growth mindset.")
	} else {
		feedback = append(feedback, "Try to frame your answer more positively — focus on growth and learning.")
	}

	// Criterion 4: Quantification.
	quantSignals := []string{"%", "percent", "million", "thousand", "x", "doubled", "tripled", "reduced", "increased", "saved"}
	hasQuant := false
	for _, signal := range quantSignals {
		if strings.Contains(lowerAnswer, signal) {
			hasQuant = true
			break
		}
	}
	if hasQuant {
		score++
		feedback = append(feedback, "Good use of quantification — makes impact concrete.")
	} else {
		feedback = append(feedback, "Add numbers or metrics to quantify your impact.")
	}

	pct := (score / maxScore) * 100

	return &ToolResult{
		Summary: fmt.Sprintf("HR Answer Score: %.0f%% (%s)", pct, category),
		Data: map[string]interface{}{
			"category": category,
			"score":    pct,
			"feedback": feedback,
			"mode":     "result",
		},
	}, nil
}
