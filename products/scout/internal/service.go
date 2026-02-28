package internal

import (
	"context"
	"encoding/json"
	"fmt"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

// ScoutService implements the soulv1.ProductServiceServer interface,
// exposing the scout job-hunting automation tools over gRPC.
type ScoutService struct {
	soulv1.UnimplementedProductServiceServer
}

// GetManifest returns the product manifest describing the scout product
// and its available tools.
func (s *ScoutService) GetManifest(_ context.Context, _ *soulv1.Empty) (*soulv1.Manifest, error) {
	return &soulv1.Manifest{
		Name:    "scout",
		Version: "0.1.0",
		Tools: []*soulv1.Tool{
			{
				Name:        "setup",
				Description: "Open visible browser for platform login",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"platform": {"type": "string", "description": "Platform to log into (e.g. linkedin, indeed, glassdoor)"},
						"headless": {"type": "boolean", "description": "Run browser in headless mode (default false)"}
					},
					"required": ["platform"]
				}`,
			},
			{
				Name:        "sync",
				Description: "Compare Supabase profile vs live platforms",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"platforms": {"type": "array", "items": {"type": "string"}, "description": "Platforms to sync (e.g. linkedin, indeed)"},
						"profileId": {"type": "string", "description": "Supabase profile ID to compare against"}
					},
					"required": ["platforms"]
				}`,
			},
			{
				Name:        "sweep",
				Description: "Check job portals for opportunities",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"platforms": {"type": "array", "items": {"type": "string"}, "description": "Platforms to sweep"},
						"keywords": {"type": "array", "items": {"type": "string"}, "description": "Job search keywords"},
						"location": {"type": "string", "description": "Job location filter"},
						"limit": {"type": "integer", "description": "Maximum number of results per platform"}
					},
					"required": ["platforms", "keywords"]
				}`,
			},
			{
				Name:        "generate",
				Description: "Create tailored resume and cover letter PDF",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"jobUrl": {"type": "string", "description": "URL of the job posting to tailor for"},
						"jobDescription": {"type": "string", "description": "Raw job description text (alternative to jobUrl)"},
						"profileId": {"type": "string", "description": "Supabase profile ID for resume data"},
						"outputDir": {"type": "string", "description": "Directory to write generated PDFs"}
					},
					"required": []
				}`,
			},
			{
				Name:        "track",
				Description: "Application CRUD — create, read, update, delete tracked applications",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"action": {"type": "string", "enum": ["create", "read", "update", "delete", "list"], "description": "CRUD action to perform"},
						"applicationId": {"type": "string", "description": "Application ID (for read/update/delete)"},
						"data": {"type": "object", "description": "Application data (for create/update)", "properties": {
							"company": {"type": "string"},
							"role": {"type": "string"},
							"url": {"type": "string"},
							"status": {"type": "string", "enum": ["saved", "applied", "interviewing", "offered", "rejected", "withdrawn"]},
							"notes": {"type": "string"}
						}}
					},
					"required": ["action"]
				}`,
			},
			{
				Name:        "report",
				Description: "Generate structured dashboard JSON with application stats",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"format": {"type": "string", "enum": ["summary", "detailed", "weekly"], "description": "Report format"},
						"dateRange": {"type": "object", "properties": {
							"from": {"type": "string", "description": "Start date (ISO 8601)"},
							"to": {"type": "string", "description": "End date (ISO 8601)"}
						}, "description": "Optional date range filter"}
					},
					"required": []
				}`,
			},
		},
	}, nil
}

// setupInput holds the parsed JSON input for the setup tool.
type setupInput struct {
	Platform string `json:"platform"`
	Headless bool   `json:"headless"`
}

// syncInput holds the parsed JSON input for the sync tool.
type syncInput struct {
	Platforms []string `json:"platforms"`
	ProfileID string   `json:"profileId,omitempty"`
}

// sweepInput holds the parsed JSON input for the sweep tool.
type sweepInput struct {
	Platforms []string `json:"platforms"`
	Keywords  []string `json:"keywords"`
	Location  string   `json:"location,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

// generateInput holds the parsed JSON input for the generate tool.
type generateInput struct {
	JobURL         string `json:"jobUrl,omitempty"`
	JobDescription string `json:"jobDescription,omitempty"`
	ProfileID      string `json:"profileId,omitempty"`
	OutputDir      string `json:"outputDir,omitempty"`
}

// trackInput holds the parsed JSON input for the track tool.
type trackInput struct {
	Action        string          `json:"action"`
	ApplicationID string          `json:"applicationId,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`
}

// reportInput holds the parsed JSON input for the report tool.
type reportInput struct {
	Format    string          `json:"format,omitempty"`
	DateRange json.RawMessage `json:"dateRange,omitempty"`
}

// ExecuteTool routes incoming tool requests to the appropriate handler.
func (s *ScoutService) ExecuteTool(_ context.Context, req *soulv1.ToolRequest) (*soulv1.ToolResponse, error) {
	switch req.Tool {
	case "setup":
		return s.executeSetup(req.InputJson)
	case "sync":
		return s.executeSync(req.InputJson)
	case "sweep":
		return s.executeSweep(req.InputJson)
	case "generate":
		return s.executeGenerate(req.InputJson)
	case "track":
		return s.executeTrack(req.InputJson)
	case "report":
		return s.executeReport(req.InputJson)
	default:
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown tool: %s", req.Tool),
		}, nil
	}
}

func (s *ScoutService) executeSetup(inputJSON string) (*soulv1.ToolResponse, error) {
	var input setupInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}
	return &soulv1.ToolResponse{
		Success: false,
		Output:  fmt.Sprintf("setup tool not implemented (platform=%s)", input.Platform),
	}, nil
}

func (s *ScoutService) executeSync(inputJSON string) (*soulv1.ToolResponse, error) {
	var input syncInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}
	return &soulv1.ToolResponse{
		Success: false,
		Output:  fmt.Sprintf("sync tool not implemented (platforms=%v)", input.Platforms),
	}, nil
}

func (s *ScoutService) executeSweep(inputJSON string) (*soulv1.ToolResponse, error) {
	var input sweepInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}
	return &soulv1.ToolResponse{
		Success: false,
		Output:  fmt.Sprintf("sweep tool not implemented (platforms=%v, keywords=%v)", input.Platforms, input.Keywords),
	}, nil
}

func (s *ScoutService) executeGenerate(inputJSON string) (*soulv1.ToolResponse, error) {
	var input generateInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}
	return &soulv1.ToolResponse{
		Success: false,
		Output:  "generate tool not implemented",
	}, nil
}

func (s *ScoutService) executeTrack(inputJSON string) (*soulv1.ToolResponse, error) {
	var input trackInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}
	return &soulv1.ToolResponse{
		Success: false,
		Output:  fmt.Sprintf("track tool not implemented (action=%s)", input.Action),
	}, nil
}

func (s *ScoutService) executeReport(inputJSON string) (*soulv1.ToolResponse, error) {
	var input reportInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("invalid input: %v", err),
		}, nil
	}
	format := input.Format
	if format == "" {
		format = "summary"
	}
	return &soulv1.ToolResponse{
		Success: false,
		Output:  fmt.Sprintf("report tool not implemented (format=%s)", format),
	}, nil
}

// ExecuteToolStream streams progress for streaming tools (sync, sweep);
// for all other tools it wraps ExecuteTool in a single Complete event.
func (s *ScoutService) ExecuteToolStream(req *soulv1.ToolRequest, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	switch req.Tool {
	case "sync":
		return s.streamSync(req.InputJson, stream)
	case "sweep":
		return s.streamSweep(req.InputJson, stream)
	default:
		// For non-streaming tools, wrap ExecuteTool in a single Complete event.
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
}

func (s *ScoutService) streamSync(inputJSON string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	var input syncInput
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

	// Send progress for each platform.
	for i, platform := range input.Platforms {
		pct := float64(i) / float64(len(input.Platforms)) * 100.0
		if err := stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Progress{
				Progress: &soulv1.ProgressUpdate{
					Analyzer: platform,
					Percent:  pct,
					Message:  fmt.Sprintf("Syncing %s (not implemented)", platform),
				},
			},
		}); err != nil {
			return err
		}
	}

	// Send completion.
	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: &soulv1.ToolResponse{
				Success: false,
				Output:  fmt.Sprintf("sync tool not implemented (platforms=%v)", input.Platforms),
			},
		},
	})
}

func (s *ScoutService) streamSweep(inputJSON string, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	var input sweepInput
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

	// Send progress for each platform.
	for i, platform := range input.Platforms {
		pct := float64(i) / float64(len(input.Platforms)) * 100.0
		if err := stream.Send(&soulv1.ToolEvent{
			Event: &soulv1.ToolEvent_Progress{
				Progress: &soulv1.ProgressUpdate{
					Analyzer: platform,
					Percent:  pct,
					Message:  fmt.Sprintf("Sweeping %s (not implemented)", platform),
				},
			},
		}); err != nil {
			return err
		}
	}

	// Send completion.
	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: &soulv1.ToolResponse{
				Success: false,
				Output:  fmt.Sprintf("sweep tool not implemented (platforms=%v, keywords=%v)", input.Platforms, input.Keywords),
			},
		},
	})
}

// Health returns the health status of the scout service.
func (s *ScoutService) Health(_ context.Context, _ *soulv1.Empty) (*soulv1.HealthResponse, error) {
	return &soulv1.HealthResponse{
		Healthy: true,
		Status:  "ok",
	}, nil
}
