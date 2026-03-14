package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rishav1305/soul-v2/internal/tutor/store"
)

// Importer handles importing content from external sources.
type Importer struct {
	store      *store.Store
	contentDir string
}

// ImportStats records import results.
type ImportStats struct {
	TopicsCreated     int `json:"topics_created"`
	QuestionsImported int `json:"questions_imported"`
	FilesCopied       int `json:"files_copied"`
}

// NewImporter creates a new Importer.
func NewImporter(s *store.Store, contentDir string) *Importer {
	return &Importer{store: s, contentDir: contentDir}
}

// Regex patterns for parsing EVALUATION_QUESTIONS from Python files.
var (
	evalBlockRe   = regexp.MustCompile(`(?s)EVALUATION_QUESTIONS\s*=\s*\[(.*?)\]`)
	dictEntryRe   = regexp.MustCompile(`(?s)\{[^}]+\}`)
	questionRe    = regexp.MustCompile(`"question"\s*:\s*"([^"]+)"`)
	answerRe      = regexp.MustCompile(`"answer"\s*:\s*"([^"]+)"`)
	difficultyRe  = regexp.MustCompile(`"difficulty"\s*:\s*"([^"]+)"`)
	numberPrefixRe = regexp.MustCompile(`^\d+[_\-.]?`)
)

// ImportStdlib imports Python stdlib cheatsheet content from ~/interview-prep/week1/stdlib_cheatsheet/.
func (m *Importer) ImportStdlib() (*ImportStats, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("importer: get home dir: %w", err)
	}

	srcDir := filepath.Join(homeDir, "interview-prep", "week1", "stdlib_cheatsheet")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("importer: source directory not found: %s", srcDir)
	}

	// Ensure destination directory exists.
	dstDir := filepath.Join(m.contentDir, "dsa", "stdlib")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return nil, fmt.Errorf("importer: create destination dir: %w", err)
	}

	// Find .py files.
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("importer: read source dir: %w", err)
	}

	stats := &ImportStats{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".py") {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		// Copy file.
		data, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			continue
		}
		stats.FilesCopied++

		// Derive topic name from filename (strip number prefix and extension).
		baseName := strings.TrimSuffix(entry.Name(), ".py")
		topicName := numberPrefixRe.ReplaceAllString(baseName, "")
		if topicName == "" {
			topicName = baseName
		}
		topicName = strings.ReplaceAll(topicName, "_", " ")

		// Create topic.
		relPath := filepath.Join("dsa", "stdlib", entry.Name())
		topic, err := m.store.CreateTopic("dsa", "stdlib", topicName, "medium", relPath)
		if err != nil {
			continue
		}
		stats.TopicsCreated++

		// Parse EVALUATION_QUESTIONS.
		content := string(data)
		blockMatch := evalBlockRe.FindStringSubmatch(content)
		if len(blockMatch) < 2 {
			continue
		}

		dictEntries := dictEntryRe.FindAllString(blockMatch[1], -1)
		for _, dictStr := range dictEntries {
			qMatch := questionRe.FindStringSubmatch(dictStr)
			aMatch := answerRe.FindStringSubmatch(dictStr)
			dMatch := difficultyRe.FindStringSubmatch(dictStr)

			if len(qMatch) < 2 || len(aMatch) < 2 {
				continue
			}

			difficulty := "medium"
			if len(dMatch) >= 2 {
				difficulty = dMatch[1]
			}

			_, err := m.store.CreateQuizQuestion(topic.ID, difficulty, qMatch[1], aMatch[1], "", entry.Name())
			if err != nil {
				continue
			}
			stats.QuestionsImported++
		}
	}

	return stats, nil
}
