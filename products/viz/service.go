package main

import (
	"context"
	"encoding/json"
	"fmt"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

// VizService implements the soulv1.ProductServiceServer interface.
type VizService struct {
	soulv1.UnimplementedProductServiceServer
}

// GetManifest returns the product manifest.
func (s *VizService) GetManifest(_ context.Context, _ *soulv1.Empty) (*soulv1.Manifest, error) {
	return &soulv1.Manifest{
		Name:    "viz",
		Version: "0.1.0",
		Tools: []*soulv1.Tool{
			{
				Name:        "analyze",
				Description: "Analyze target for data visualization insights",
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
				Description: "Generate data visualization report",
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

type analyzeInputVizService struct {
	Target string `json:"target"`
}

type reportInputVizService struct {
	Target string `json:"target"`
	Format string `json:"format"`
}

// ExecuteTool routes incoming tool requests.
func (s *VizService) ExecuteTool(_ context.Context, req *soulv1.ToolRequest) (*soulv1.ToolResponse, error) {
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

func (s *VizService) executeAnalyze(inputJSON string) (*soulv1.ToolResponse, error) {
	var input analyzeInputVizService
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("viz analysis of %s: not yet implemented", input.Target),
	}, nil
}

func (s *VizService) executeReport(inputJSON string) (*soulv1.ToolResponse, error) {
	var input reportInputVizService
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return &soulv1.ToolResponse{Success: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	return &soulv1.ToolResponse{
		Success: true,
		Output:  fmt.Sprintf("viz report for %s (format: %s): not yet implemented", input.Target, input.Format),
	}, nil
}

// ExecuteToolStream handles streaming tool execution.
func (s *VizService) ExecuteToolStream(req *soulv1.ToolRequest, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
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
func (s *VizService) Health(_ context.Context, _ *soulv1.Empty) (*soulv1.HealthResponse, error) {
	return &soulv1.HealthResponse{Status: "ok"}, nil
}
