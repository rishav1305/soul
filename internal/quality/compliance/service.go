package compliance

import (
	"encoding/json"
	"fmt"

	"github.com/rishav1305/soul/internal/quality/compliance/fix"
	"github.com/rishav1305/soul/internal/quality/compliance/reporters"
	"github.com/rishav1305/soul/internal/quality/compliance/scan"
)

// Service exposes the compliance engine as tool operations.
type Service struct{}

// scanInput is the JSON schema for the scan tool.
type scanInput struct {
	Directory  string   `json:"directory"`
	Frameworks []string `json:"frameworks,omitempty"`
	Severity   []string `json:"severity,omitempty"`
	Analyzers  []string `json:"analyzers,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
}

// fixInput is the JSON schema for the fix tool.
type fixInput struct {
	Directory  string   `json:"directory"`
	DryRun     bool     `json:"dry_run"`
	Frameworks []string `json:"frameworks,omitempty"`
	Severity   []string `json:"severity,omitempty"`
	Analyzers  []string `json:"analyzers,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
}

// badgeInput is the JSON schema for the badge tool.
type badgeInput struct {
	Directory  string   `json:"directory"`
	Frameworks []string `json:"frameworks,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
}

// reportInput is the JSON schema for the report tool.
type reportInput struct {
	Directory  string   `json:"directory"`
	Format     string   `json:"format"`
	Frameworks []string `json:"frameworks,omitempty"`
	Severity   []string `json:"severity,omitempty"`
	Analyzers  []string `json:"analyzers,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
}

// ExecuteTool dispatches a compliance tool by name.
func (s *Service) ExecuteTool(name string, input json.RawMessage) (interface{}, error) {
	switch name {
	case "scan":
		return s.executeScan(input)
	case "fix":
		return s.executeFix(input)
	case "badge":
		return s.executeBadge(input)
	case "report":
		return s.executeReport(input)
	default:
		return nil, fmt.Errorf("unknown compliance tool: %q", name)
	}
}

func (s *Service) executeScan(input json.RawMessage) (interface{}, error) {
	var in scanInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("parsing scan input: %w", err)
	}
	if in.Directory == "" {
		return nil, fmt.Errorf("directory is required")
	}
	return scan.ScanDirectory(scan.ScanOptions{
		Directory:  in.Directory,
		Frameworks: in.Frameworks,
		Severity:   in.Severity,
		Analyzers:  in.Analyzers,
		Exclude:    in.Exclude,
	})
}

func (s *Service) executeFix(input json.RawMessage) (interface{}, error) {
	var in fixInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("parsing fix input: %w", err)
	}
	if in.Directory == "" {
		return nil, fmt.Errorf("directory is required")
	}

	result, err := scan.ScanDirectory(scan.ScanOptions{
		Directory:  in.Directory,
		Frameworks: in.Frameworks,
		Severity:   in.Severity,
		Analyzers:  in.Analyzers,
		Exclude:    in.Exclude,
	})
	if err != nil {
		return nil, fmt.Errorf("scanning before fix: %w", err)
	}

	fixes, err := fix.ApplyFixes(result.Findings, in.DryRun)
	if err != nil {
		return nil, fmt.Errorf("applying fixes: %w", err)
	}

	return map[string]interface{}{
		"fixes":   fixes,
		"dry_run": in.DryRun,
		"scan":    result.Summary,
	}, nil
}

func (s *Service) executeBadge(input json.RawMessage) (interface{}, error) {
	var in badgeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("parsing badge input: %w", err)
	}
	if in.Directory == "" {
		return nil, fmt.Errorf("directory is required")
	}

	result, err := scan.ScanDirectory(scan.ScanOptions{
		Directory:  in.Directory,
		Frameworks: in.Frameworks,
		Exclude:    in.Exclude,
	})
	if err != nil {
		return nil, fmt.Errorf("scanning for badge: %w", err)
	}

	badge := reporters.GenerateBadge(result)
	score := reporters.CalculateScore(result)
	return map[string]interface{}{
		"badge": badge,
		"score": score,
	}, nil
}

func (s *Service) executeReport(input json.RawMessage) (interface{}, error) {
	var in reportInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("parsing report input: %w", err)
	}
	if in.Directory == "" {
		return nil, fmt.Errorf("directory is required")
	}

	format := in.Format
	if format == "" {
		format = "json"
	}

	result, err := scan.ScanDirectory(scan.ScanOptions{
		Directory:  in.Directory,
		Frameworks: in.Frameworks,
		Severity:   in.Severity,
		Analyzers:  in.Analyzers,
		Exclude:    in.Exclude,
	})
	if err != nil {
		return nil, fmt.Errorf("scanning for report: %w", err)
	}

	report, err := reporters.Generate(result, format)
	if err != nil {
		return nil, fmt.Errorf("generating report: %w", err)
	}

	return map[string]interface{}{
		"report": report,
		"format": format,
	}, nil
}
