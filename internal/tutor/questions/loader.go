package questions

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"

	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

//go:embed dsa_python.json ai_llm.json system_design.json behavioral.json
var questionFS embed.FS

// Question represents a seed question loaded from an embedded JSON file.
type Question struct {
	Module      string `json:"module"`
	Category    string `json:"category"`
	Topic       string `json:"topic"`
	Difficulty  string `json:"difficulty"`
	QuestionTxt string `json:"question"`
	Answer      string `json:"answer"`
	Explanation string `json:"explanation"`
	Source      string `json:"source"`
}

// LoadStats reports the outcome of a Load call.
type LoadStats struct {
	TopicsCreated    int `json:"topicsCreated"`
	QuestionsCreated int `json:"questionsCreated"`
	QuestionsSkipped int `json:"questionsSkipped"`
}

// Load reads all embedded question JSON files and seeds them into the store.
// It is idempotent: topics and questions with an existing source key are skipped.
func Load(s *store.Store) (*LoadStats, error) {
	files := []string{"dsa_python.json", "ai_llm.json", "system_design.json", "behavioral.json"}
	stats := &LoadStats{}

	for _, file := range files {
		data, err := questionFS.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("questions: read %s: %w", file, err)
		}

		var questions []Question
		if err := json.Unmarshal(data, &questions); err != nil {
			return nil, fmt.Errorf("questions: parse %s: %w", file, err)
		}

		for _, q := range questions {
			topic, err := s.CreateTopic(q.Module, q.Category, q.Topic, q.Difficulty, "")
			if err != nil {
				log.Printf("questions: skip topic %s/%s/%s: %v", q.Module, q.Category, q.Topic, err)
				stats.QuestionsSkipped++
				continue
			}
			stats.TopicsCreated++

			_, err = s.CreateQuizQuestion(topic.ID, q.Difficulty, q.QuestionTxt, q.Answer, q.Explanation, q.Source)
			if err != nil {
				log.Printf("questions: skip question %s: %v", q.Source, err)
				stats.QuestionsSkipped++
				continue
			}
			stats.QuestionsCreated++
		}
	}

	return stats, nil
}
