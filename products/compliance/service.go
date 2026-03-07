package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rishav1305/soul/products/compliance/analyzers"
	"github.com/rishav1305/soul/products/compliance/fix"
	"github.com/rishav1305/soul/products/compliance/reporters"
	"github.com/rishav1305/soul/products/compliance/scan"
	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	return path
}

// ComplianceService implements the soulv1.ProductServiceServer interface,
// exposing the compliance scanner, fixer, badge generator, and reporter
// functionality over gRPC.
type ComplianceService struct {
	soulv1.UnimplementedProductServiceServer
}

// GetManifest returns the product manifest describing the compliance product
// and its available tools.
func (s *ComplianceService) GetManifest(_ context.Context, _ *soulv1.Empty) (*soulv1.Manifest, error) {
	return &soulv1.Manifest{
		Name:    "compliance",
		Version: "0.2.0",
		Tools: []*soulv1.Tool{
			{
				Name:        "scan",
				Description: "Scan directory for compliance issues",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"directory": {"type": "string", "description": "Directory to scan"},
						"frameworks": {"type": "array", "items": {"type": "string"}, "description": "Frameworks to check against"},
						"severity": {"type": "array", "items": {"type": "string"}, "description": "Severity levels to include"}
					},
					"required": ["directory"]
				}`,
			},
			{
				Name:        "fix",
				Description: "Auto-fix compliance issues",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"directory": {"type": "string", "description": "Directory to fix"},
						"dryRun": {"type": "boolean", "description": "If true, only generate patches without applying"}
					},
					"required": ["directory"]
				}`,
			},
			{
				Name:        "badge",
				Description: "Generate compliance badge",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"directory": {"type": "string", "description": "Directory to scan for badge generation"}
					},
					"required": ["directory"]
				}`,
			},
			{
				Name:        "report",
				Description: "Generate compliance report",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"directory": {"type": "string", "description": "Directory to scan for report"},
						"format": {"type": "string", "enum": ["terminal", "json", "html"], "description": "Report output format"}
					},
					"required": ["directory"]
				}`,
			},
			{
				Name:        "monitor",
				Description: "Monitor directory for changes",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"directory": {"type": "string", "description": "Directory to monitor"}
					},
					"required": ["directory"]
				}`,
			},
		},
	}, nil
}

// scanInput holds the parsed JSON input for the scan tool.
type scanInput struct {
	Directory  string   `json:"directory"`
	Frameworks []string `json:"frameworks,omitempty"`
	Severity   []string `json:"severity,omitempty"`
}

// fixInput holds the parsed JSON input for the fix tool.
type fixInput struct {
	Directory string `json:"directory"`
	DryRun    bool   `json:"dryRun"`
}

// badgeInput holds the parsed JSON input for the badge tool.
type badgeInput struct {
	Directory string `json:"directory"`
}

// reportInput holds the parsed JSON input for the report tool.
type reportInput struct {
	Directory string `json:"directory"`
	Format    string `json:"format"`
}

// monitorInput holds the parsed JSON input for the monitor tool.
type monitorInput struct {
	Directory string `json:"directory"`
}

// ExecuteTool routes incoming tool requests to the appropriate handler.
func (s *ComplianceService) ExecuteTool(_ context.Context, req *soulv1.ToolRequest) (*soulv1.ToolResponse, error) {
	switch req.Tool {
	case "scan":
		return s.executeScan(req.InputJson)
	case "fix":
		return s.executeFix(req.InputJson)
	case "badge":
		return s.executeBadge(req.InputJson)
	case "report":
		return s.executeReport(req.InputJson)
	case "monitor":
		return s.executeMonitor(req.InputJson)
	default:
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown tool: %s", req.Tool),
		}, nil
	}
}

func (s *ComplianceService) executeScan(inputJSON string) (*soulv1.ToolResponse, error) {
	var input scanInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}
	input.Directory = expandPath(input.Directory)

	result, err := scan.RunScan(scan.ScanOptions{
		Directory:  input.Directory,
		Frameworks: input.Frameworks,
		Severity:   input.Severity,
	})
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("scan failed: %v", err),
		}, nil
	}

	structured, err := json.Marshal(result)
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("failed to marshal result: %v", err),
		}, nil
	}

	return &soulv1.ToolResponse{
		Success:        true,
		Output:         fmt.Sprintf("Scan complete: %d findings", result.Summary.Total),
		StructuredJson: string(structured),
	}, nil
}

func (s *ComplianceService) executeFix(inputJSON string) (*soulv1.ToolResponse, error) {
	var input fixInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	// First run a scan to get findings.
	result, err := scan.RunScan(scan.ScanOptions{
		Directory: input.Directory,
	})
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("scan failed: %v", err),
		}, nil
	}

	// Apply fixes.
	fixResults, err := fix.ApplyFixes(result.Findings, input.DryRun)
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("fix failed: %v", err),
		}, nil
	}

	structured, err := json.Marshal(fixResults)
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("failed to marshal result: %v", err),
		}, nil
	}

	fixedCount := 0
	for _, r := range fixResults {
		if r.Fixed {
			fixedCount++
		}
	}

	return &soulv1.ToolResponse{
		Success:        true,
		Output:         fmt.Sprintf("Fix complete: %d/%d issues fixed", fixedCount, len(fixResults)),
		StructuredJson: string(structured),
	}, nil
}

func (s *ComplianceService) executeBadge(inputJSON string) (*soulv1.ToolResponse, error) {
	var input badgeInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	result, err := scan.RunScan(scan.ScanOptions{
		Directory: input.Directory,
	})
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("scan failed: %v", err),
		}, nil
	}

	svg := reporters.GenerateBadge(result)

	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("Badge generated (score: %d%%)", reporters.CalculateScore(result)),
		Artifacts: []*soulv1.Artifact{
			{
				Type:    "image/svg+xml",
				Path:    "compliance-badge.svg",
				Content: []byte(svg),
			},
		},
	}, nil
}

func (s *ComplianceService) executeReport(inputJSON string) (*soulv1.ToolResponse, error) {
	var input reportInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}

	result, err := scan.RunScan(scan.ScanOptions{
		Directory: input.Directory,
	})
	if err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("scan failed: %v", err),
		}, nil
	}

	format := input.Format
	if format == "" {
		format = "terminal"
	}

	var output string
	switch format {
	case "terminal":
		output = reporters.FormatTerminal(result)
	case "json":
		jsonOutput, jsonErr := reporters.FormatJSON(result)
		if jsonErr != nil {
			return &soulv1.ToolResponse{
				Success: false,
				Output:  fmt.Sprintf("JSON formatting failed: %v", jsonErr),
			}, nil
		}
		output = jsonOutput
	case "html":
		output = reporters.GenerateHTML(result)
	default:
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown format: %s", format),
		}, nil
	}

	return &soulv1.ToolResponse{
		Success: true,
		Output:  output,
	}, nil
}

func (s *ComplianceService) executeMonitor(_ string) (*soulv1.ToolResponse, error) {
	return &soulv1.ToolResponse{
		Success: false,
		Output:  "monitor tool is not implemented",
	}, nil
}

// ExecuteToolStream streams progress and findings for the scan tool;
// for all other tools it wraps ExecuteTool in a single Complete event.
func (s *ComplianceService) ExecuteToolStream(req *soulv1.ToolRequest, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	if req.Tool == "scan" {
		return s.streamScan(req.InputJson, stream)
	}

	// For non-scan tools, wrap ExecuteTool in a single Complete event.
	resp, err := s.ExecuteTool(context.Background(), req)
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "EXECUTE_ERROR",
					Message: err.Error(),
				},
			},
		})
	}

	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: resp,
		},
	})
}

func (s *ComplianceService) streamScan(inputJSON string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	var input scanInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "INVALID_INPUT",
					Message: fmt.Sprintf("invalid input: %v", err),
				},
			},
		})
	}
	input.Directory = expandPath(input.Directory)

	// Send progress for scan start.
	analyzerNames := []string{"secret-scanner", "config-checker", "git-analyzer", "dep-auditor", "ast-analyzer"}
	for i, name := range analyzerNames {
		pct := float64(i) / float64(len(analyzerNames)) * 100.0
		if err := stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Progress{
				Progress: &soulv1.ProgressUpdate{
					Analyzer: name,
					Percent:  pct,
					Message:  fmt.Sprintf("Starting %s", name),
				},
			},
		}); err != nil {
			return err
		}
	}

	// Run the actual scan.
	result, err := scan.RunScan(scan.ScanOptions{
		Directory:  input.Directory,
		Frameworks: input.Frameworks,
		Severity:   input.Severity,
	})
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "SCAN_FAILED",
					Message: fmt.Sprintf("scan failed: %v", err),
				},
			},
		})
	}

	// Send each finding as a FindingEvent.
	for _, f := range result.Findings {
		if err := stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Finding{
				Finding: findingToEvent(f),
			},
		}); err != nil {
			return err
		}
	}

	// Send completion progress.
	for _, name := range analyzerNames {
		if err := stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Progress{
				Progress: &soulv1.ProgressUpdate{
					Analyzer: name,
					Percent:  100.0,
					Message:  fmt.Sprintf("Completed %s", name),
				},
			},
		}); err != nil {
			return err
		}
	}

	// Send the complete result.
	structured, err := json.Marshal(result)
	if err != nil {
		return stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Error{
				Error: &soulv1.ErrorEvent{
					Code:    "MARSHAL_ERROR",
					Message: fmt.Sprintf("failed to marshal result: %v", err),
				},
			},
		})
	}

	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: &soulv1.ToolResponse{
				Success:        true,
				Output:         fmt.Sprintf("Scan complete: %d findings", result.Summary.Total),
				StructuredJson: string(structured),
			},
		},
	})
}

// findingToEvent converts an analyzers.Finding to a soulv1.FindingEvent.
func findingToEvent(f analyzers.Finding) *soulv1.FindingEvent {
	return &soulv1.FindingEvent{
		Id:       f.ID,
		Title:    f.Title,
		Severity: f.Severity,
		File:     f.File,
		Line:     int32(f.Line),
		Evidence: f.Evidence,
	}
}

// Health returns the health status of the compliance service.
func (s *ComplianceService) Health(_ context.Context, _ *soulv1.Empty) (*soulv1.HealthResponse, error) {
	return &soulv1.HealthResponse{
		Healthy: true,
		Status:  "ok",
	}, nil
}
