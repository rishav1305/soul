package products

import (
	"context"
	"fmt"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

// Proxy routes tool execution requests to the appropriate product's gRPC service.
type Proxy struct {
	manager *Manager
}

// NewProxy creates a new tool execution proxy.
func NewProxy(manager *Manager) *Proxy {
	return &Proxy{manager: manager}
}

// ExecuteTool finds the tool by qualified name, locates the owning product's
// gRPC client, and executes the tool via a unary RPC call.
func (p *Proxy) ExecuteTool(ctx context.Context, qualifiedName, inputJSON, sessionID string) (*soulv1.ToolResponse, error) {
	entry, ok := p.manager.Registry().FindTool(qualifiedName)
	if !ok {
		return nil, fmt.Errorf("tool %q not found", qualifiedName)
	}

	client, ok := p.manager.GetClient(entry.ProductName)
	if !ok {
		return nil, fmt.Errorf("product %q not running", entry.ProductName)
	}

	req := &soulv1.ToolRequest{
		Tool:      entry.Tool.GetName(),
		InputJson: inputJSON,
		SessionId: sessionID,
	}

	return client.ExecuteTool(ctx, req)
}

// ExecuteToolStream finds the tool by qualified name, locates the owning
// product's gRPC client, and executes the tool via a server-streaming RPC call.
func (p *Proxy) ExecuteToolStream(ctx context.Context, qualifiedName, inputJSON, sessionID string) (grpc.ServerStreamingClient[soulv1.ToolEvent], error) {
	entry, ok := p.manager.Registry().FindTool(qualifiedName)
	if !ok {
		return nil, fmt.Errorf("tool %q not found", qualifiedName)
	}

	client, ok := p.manager.GetClient(entry.ProductName)
	if !ok {
		return nil, fmt.Errorf("product %q not running", entry.ProductName)
	}

	req := &soulv1.ToolRequest{
		Tool:      entry.Tool.GetName(),
		InputJson: inputJSON,
		SessionId: sessionID,
	}

	return client.ExecuteToolStream(ctx, req)
}
