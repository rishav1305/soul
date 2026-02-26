package ai

import (
	"encoding/json"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
)

// ClaudeTool represents a tool definition in the Claude Messages API tool_use format.
type ClaudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// defaultSchema is used when a tool's InputSchemaJson is empty or invalid JSON.
var defaultSchema = json.RawMessage(`{"type":"object"}`)

// BuildClaudeTools converts a slice of product tools into Claude tool_use format.
// Tool names are prefixed with the product name: {productName}__{toolName}.
// If a tool's InputSchemaJson is empty or invalid JSON, it defaults to {"type":"object"}.
func BuildClaudeTools(productName string, tools []*soulv1.Tool) []ClaudeTool {
	if len(tools) == 0 {
		return nil
	}

	result := make([]ClaudeTool, len(tools))
	for i, tool := range tools {
		schema := json.RawMessage(tool.GetInputSchemaJson())
		if len(schema) == 0 || !json.Valid(schema) {
			schema = defaultSchema
		}

		result[i] = ClaudeTool{
			Name:        productName + "__" + tool.GetName(),
			Description: tool.GetDescription(),
			InputSchema: schema,
		}
	}
	return result
}
