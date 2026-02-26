package products

import (
	"strings"
	"sync"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

// ToolEntry pairs a tool with its owning product name.
type ToolEntry struct {
	ProductName string
	Tool        *soulv1.Tool
}

// Registry stores manifests for all registered products and provides
// lookup methods for tools across products.
type Registry struct {
	mu        sync.RWMutex
	manifests map[string]*soulv1.Manifest
}

// NewRegistry creates an empty product registry.
func NewRegistry() *Registry {
	return &Registry{
		manifests: make(map[string]*soulv1.Manifest),
	}
}

// Register stores a manifest under the given product name.
func (r *Registry) Register(name string, manifest *soulv1.Manifest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifests[name] = manifest
}

// Get returns the manifest for the named product and whether it was found.
func (r *Registry) Get(name string) (*soulv1.Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.manifests[name]
	return m, ok
}

// AllTools returns a flattened slice of all tools from all registered products,
// each annotated with its owning product name.
func (r *Registry) AllTools() []ToolEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []ToolEntry
	for name, manifest := range r.manifests {
		for _, tool := range manifest.GetTools() {
			entries = append(entries, ToolEntry{
				ProductName: name,
				Tool:        tool,
			})
		}
	}
	return entries
}

// FindTool looks up a tool by its qualified name (product__tool, using double
// underscore as separator). Returns the ToolEntry and whether the tool was found.
func (r *Registry) FindTool(qualifiedName string) (ToolEntry, bool) {
	parts := strings.SplitN(qualifiedName, "__", 2)
	if len(parts) != 2 {
		return ToolEntry{}, false
	}

	productName := parts[0]
	toolName := parts[1]

	r.mu.RLock()
	defer r.mu.RUnlock()

	manifest, ok := r.manifests[productName]
	if !ok {
		return ToolEntry{}, false
	}

	for _, tool := range manifest.GetTools() {
		if tool.GetName() == toolName {
			return ToolEntry{ProductName: productName, Tool: tool}, true
		}
	}

	return ToolEntry{}, false
}
