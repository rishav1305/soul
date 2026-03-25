package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	prodctx "github.com/rishav1305/soul/internal/chat/context"
	"github.com/rishav1305/soul/internal/mcp/protocol"
)

// canonicalProducts lists one representative name per product context.
// Each name maps to a distinct *Context() function in the context package.
// For products with aliases (e.g., "devops"/"dba"/"migrate" all resolve to infraContext),
// we pick the first alias to avoid collecting duplicate tool sets.
var canonicalProducts = []string{
	"tasks",
	"tutor",
	"projects",
	"observe",
	"devops",      // infra (also dba, migrate)
	"compliance",  // quality (also qa, analytics)
	"dataeng",     // dataprod (also costops, viz)
	"docs",        // docsprod (also api)
	"sentinel",
	"bench",
	"mesh",
	"scout",
}

// builtinPrefixes are name prefixes that identify built-in tools
// (memory, custom tool management, subagent). These are excluded
// from the MCP registry because they are session-scoped, not product tools.
var builtinPrefixes = []string{"memory_", "tool_", "subagent"}

// Registry collects product tools from the context package and exposes
// them as MCP tools for tools/list and tools/call.
type Registry struct {
	tools      []protocol.MCPTool
	toolSet    map[string]bool
	dispatcher *prodctx.Dispatcher
}

// NewRegistry creates a registry populated with all product tools.
// The dispatcher may be nil for list-only mode; Call() will return
// an error if invoked without a dispatcher.
func NewRegistry(dispatcher *prodctx.Dispatcher) *Registry {
	r := &Registry{
		toolSet:    make(map[string]bool),
		dispatcher: dispatcher,
	}
	r.collect()
	return r
}

// collect iterates canonical products, strips built-in tools, deduplicates,
// and converts stream.Tool to protocol.MCPTool.
func (r *Registry) collect() {
	builtinCount := len(prodctx.Default().Tools)

	for _, product := range canonicalProducts {
		ctx := prodctx.ForProduct(product)

		// Skip the prepended built-in tools.
		productTools := ctx.Tools
		if len(productTools) > builtinCount {
			productTools = productTools[builtinCount:]
		}

		for _, t := range productTools {
			// Skip any tool that leaked through with a builtin prefix.
			if isBuiltin(t.Name) {
				continue
			}

			// Dedup by name across products.
			if r.toolSet[t.Name] {
				continue
			}
			r.toolSet[t.Name] = true

			r.tools = append(r.tools, protocol.MCPTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}
}

// isBuiltin returns true if the tool name matches a built-in prefix.
func isBuiltin(name string) bool {
	for _, prefix := range builtinPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return name == "subagent"
}

// List returns all registered MCP tools.
func (r *Registry) List() []protocol.MCPTool {
	return r.tools
}

// Has reports whether a tool with the given name is registered.
func (r *Registry) Has(name string) bool {
	return r.toolSet[name]
}

// Call executes a registered tool via the dispatcher.
// Returns an error if the tool is unknown or the dispatcher is nil.
func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (string, error) {
	if !r.Has(name) {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	if r.dispatcher == nil {
		return "", fmt.Errorf("dispatcher is nil: cannot execute tool %s", name)
	}
	return r.dispatcher.Execute(ctx, name, args)
}
