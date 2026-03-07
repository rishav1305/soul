package skills

import (
	"os"
	"path/filepath"
	"strings"
)

// Skill represents a loaded skill file.
type Skill struct {
	Name    string
	Content string
}

// Store holds all loaded skills indexed by name (lowercase).
type Store struct {
	skills map[string]Skill
}

// Load scans ~/.claude/plugins/cache and ~/.claude/skills for SKILL.md files and indexes them.
func Load() *Store {
	store := &Store{skills: make(map[string]Skill)}

	home, err := os.UserHomeDir()
	if err != nil {
		return store
	}

	cacheDir := filepath.Join(home, ".claude", "plugins", "cache")

	// Support three layout depths:
	//   cache/<plugin>/<version>/skills/<name>/SKILL.md  (original spec)
	//   cache/<marketplace>/<plugin>/<version>/skills/<name>/SKILL.md  (actual layout)
	//   ~/.claude/skills/<name>/SKILL.md  (personal skills)
	patterns := []string{
		filepath.Join(cacheDir, "*", "*", "skills", "*", "SKILL.md"),
		filepath.Join(cacheDir, "*", "*", "*", "skills", "*", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "*", "SKILL.md"),
	}

	var matches []string
	for _, p := range patterns {
		m, _ := filepath.Glob(p)
		matches = append(matches, m...)
	}

	seen := make(map[string]bool)
	for _, path := range matches {
		// Skill name is the directory containing SKILL.md.
		name := strings.ToLower(filepath.Base(filepath.Dir(path)))
		if seen[name] {
			continue // take first match (avoid duplicates)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		store.skills[name] = Skill{Name: name, Content: string(data)}
		seen[name] = true
	}

	return store
}

// Get returns skill content by name. Returns ("", false) if not found.
func (s *Store) Get(name string) (string, bool) {
	skill, ok := s.skills[strings.ToLower(name)]
	return skill.Content, ok
}

// Names returns all loaded skill names.
func (s *Store) Names() []string {
	names := make([]string, 0, len(s.skills))
	for n := range s.skills {
		names = append(names, n)
	}
	return names
}
