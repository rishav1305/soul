package analyzers

// Finding represents a single compliance issue detected by an analyzer.
type Finding struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"` // critical, high, medium, low, info
	Framework   []string `json:"framework"`
	ControlIDs  []string `json:"control_ids"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Column      int      `json:"column"`
	Evidence    string   `json:"evidence"`
	Analyzer    string   `json:"analyzer"`
	Fixable     bool     `json:"fixable"`
}

// ScannedFile represents a file discovered during a compliance scan.
type ScannedFile struct {
	Path         string
	RelativePath string
	Extension    string
	Size         int64
}

// Analyzer defines the interface for compliance analyzers.
type Analyzer interface {
	Name() string
	Analyze(files []ScannedFile, rules []Rule) ([]Finding, error)
}

// Rule defines a compliance rule loaded from YAML configuration.
type Rule struct {
	ID          string   `yaml:"id" json:"id"`
	Title       string   `yaml:"title" json:"title"`
	Severity    string   `yaml:"severity" json:"severity"`
	Analyzer    string   `yaml:"analyzer" json:"analyzer"`
	Pattern     string   `yaml:"pattern" json:"pattern"`
	Controls    []string `yaml:"controls" json:"controls"`
	Framework   []string `yaml:"framework" json:"framework"`
	Description string   `yaml:"description" json:"description"`
	Fixable     bool     `yaml:"fixable" json:"fixable"`
}
