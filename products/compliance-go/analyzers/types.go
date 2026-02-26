package analyzers

import "github.com/rishav1305/soul/products/compliance-go/rules"

// Finding represents a single compliance violation detected by an analyzer.
type Finding struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Framework   []string `json:"framework"`
	ControlIDs  []string `json:"control_ids"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Column      int      `json:"column,omitempty"`
	Evidence    string   `json:"evidence,omitempty"`
	Analyzer    string   `json:"analyzer"`
	Fixable     bool     `json:"fixable"`
}

// ScannedFile represents a file discovered during directory scanning.
type ScannedFile struct {
	Path         string
	RelativePath string
	Extension    string
	Size         int64
}

// Analyzer is the interface that all compliance analyzers must implement.
type Analyzer interface {
	Name() string
	Analyze(files []ScannedFile, rules []rules.Rule) ([]Finding, error)
}
