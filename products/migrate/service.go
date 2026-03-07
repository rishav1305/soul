package main

import (
	"context"
	"encoding/json"
	"fmt"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

// MigrateService implements the soulv1.ProductServiceServer interface.
type MigrateService struct {
	soulv1.UnimplementedProductServiceServer
}

// GetManifest returns the product manifest.
func (s *MigrateService) GetManifest(_ context.Context, _ *soulv1.Empty) (*soulv1.Manifest, error) {
	return &soulv1.Manifest{
		Name:    "migrate",
		Version: "0.1.0",
		Tools: []*soulv1.Tool{
			{
				Name:        "analyze",
				Description: "Analyze target for database and system migration insights",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"target": {"type": "string", "description": "Target path or connection string to analyze"}
					},
					"required": ["target"]
				}`,
			},
			{
				Name:        "report",
				Description: "Generate database and system migration report",
				InputSchemaJson: `{
					"type": "object",
					"properties": {
						"target": {"type": "string", "description": "Target to report on"},
						"format": {"type": "string", "enum": ["terminal", "json", "html"], "description": "Report format"}
					},
					"required": ["target"]
				}`,
			},
		},
	}, nil
}

type analyzeInputMigrateService struct {
	Target string `json:"target"`
}

type reportInputMigrateService struct {
	Target string `json:"target"`
	Format string `json:"format"`
}

// ExecuteTool routes incoming tool requests.
func (s *MigrateService) ExecuteTool(_ context.Context, req *soulv1.ToolRequest) (*soulv1.ToolResponse, error) {
	switch req.Tool {
	case "analyze":
		return s.executeAnalyze(req.InputJson)
	case "report":
		return s.executeReport(req.InputJson)
	default:
		return &soulv1.ToolResponse{
			Success: false,
			Output:  fmt.Sprintf("unknown tool: %s", req.Tool),
		}, nil
	}
}

func (s *MigrateService) executeAnalyze(inputJSON string) (*soulv1.ToolResponse, error) {
	var input analyzeInputMigrateService
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("migrate analysis of %s: not yet implemented", input.Target),
	}, nil
}

func (s *MigrateService) executeReport(inputJSON string) (*soulv1.ToolResponse, error) {
	var input reportInputMigrateService
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("migrate report for %s (format: %s): not yet implemented", input.Target, input.Format),
	}, nil
}

// ExecuteToolStream handles streaming tool execution.
func (s *MigrateService) ExecuteToolStream(req *soulv1.ToolRequest, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
	resp, err := s.ExecuteTool(stream.Context(), req)
	if err != nil {
		return err
	}
	return stream.Send(&soulv1.ToolEvent{
		Event: &soulv1.ToolEvent_Complete{
			Complete: resp,
		},
	})
}

// Health returns the health status.
func (s *MigrateService) Health(_ context.Context, _ *soulv1.Empty) (*soulv1.HealthResponse, error) {
	return &soulv1.HealthResponse{Status: "ok"}, nil
}
