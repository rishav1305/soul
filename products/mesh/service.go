package main

import (
	"context"
	"fmt"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

type MeshService struct {
	soulv1.UnimplementedProductServiceServer
}

func (s *MeshService) GetManifest(_ context.Context, _ *soulv1.Empty) (*soulv1.Manifest, error) {
	return &soulv1.Manifest{
		Name:    "mesh",
		Version: "0.1.0",
		Tools:   []*soulv1.Tool{},
	}, nil
}

func (s *MeshService) ExecuteTool(_ context.Context, req *soulv1.ToolRequest) (*soulv1.ToolResponse, error) {
	return &soulv1.ToolResponse{
		Success: false,
		Output:  fmt.Sprintf("unknown tool: %s", req.Tool),
	}, nil
}

func (s *MeshService) ExecuteToolStream(req *soulv1.ToolRequest, stream grpc.ServerStreamingServer[soulv1.ToolEvent]) error {
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

func (s *MeshService) Health(_ context.Context, _ *soulv1.Empty) (*soulv1.HealthResponse, error) {
	return &soulv1.HealthResponse{Status: "ok"}, nil
}
