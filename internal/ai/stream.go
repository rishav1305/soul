package ai

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// StreamEvent represents a parsed SSE event from the Claude streaming API.
type StreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// ContentBlockStart represents the start of a content block in a streaming response.
// For text blocks, Type is "text" and Text contains initial text (usually empty).
// For tool_use blocks, Type is "tool_use", ID contains the tool use ID, and Name
// contains the tool name.
type ContentBlockStart struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Text string `json:"text,omitempty"`
}

// ContentBlockDelta represents a delta update within a content block.
// For text deltas, Type is "text_delta" and Text contains the text chunk.
// For tool_use deltas, Type is "input_json_delta" and PartialJSON contains
// the partial JSON fragment.
// For thinking deltas, Type is "thinking_delta" and Thinking contains the
// thinking text chunk.
type ContentBlockDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
}

// ParseSSEStream reads Server-Sent Events from a reader and sends parsed
// StreamEvent values on the provided channel. It closes the channel when
// the reader reaches EOF or encounters an error.
//
// The caller should run this function in a goroutine:
//
//	events := make(chan ai.StreamEvent)
//	go ai.ParseSSEStream(reader, events)
//	for ev := range events { ... }
func ParseSSEStream(r io.Reader, events chan<- StreamEvent) {
	defer close(events)

	scanner := bufio.NewScanner(r)

	var currentType string
	var currentData string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			currentData = strings.TrimPrefix(line, "data: ")
			continue
		}

		// Empty line signals end of an SSE event block.
		if line == "" && currentData != "" {
			events <- StreamEvent{
				Type: currentType,
				Data: json.RawMessage(currentData),
			}
			currentType = ""
			currentData = ""
		}
	}

	// Flush any remaining event that wasn't terminated by a blank line.
	if currentData != "" {
		events <- StreamEvent{
			Type: currentType,
			Data: json.RawMessage(currentData),
		}
	}
}
