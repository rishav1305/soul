package stream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// SSEParser reads SSE events from an io.Reader.
type SSEParser struct {
	scanner *bufio.Scanner
}

// NewSSEParser creates a new SSE parser reading from the given reader.
func NewSSEParser(r io.Reader) *SSEParser {
	return &SSEParser{
		scanner: bufio.NewScanner(r),
	}
}

// Next returns the next SSE event. Returns io.EOF when the stream ends.
// Skips comment lines (starting with ':'), retry fields, and ping events.
// Returns a ParseError if the data field cannot be parsed as JSON.
func (p *SSEParser) Next() (*SSEEvent, error) {
	var eventType string
	var dataLines []string

	for p.scanner.Scan() {
		line := p.scanner.Text()

		// Empty line signals end of an event.
		if line == "" {
			if eventType == "" && len(dataLines) == 0 {
				// Empty line with no accumulated event — skip.
				continue
			}

			// We have a complete event. Parse it.
			if eventType == "" {
				// Data without event type — skip and reset.
				eventType = ""
				dataLines = nil
				continue
			}

			// Skip ping events.
			if eventType == "ping" {
				eventType = ""
				dataLines = nil
				continue
			}

			data := strings.Join(dataLines, "\n")
			evt, err := parseSSEData(eventType, data)
			if err != nil {
				return nil, err
			}
			return evt, nil
		}

		// Comment line — skip.
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Retry field — ignore per spec.
		if strings.HasPrefix(line, "retry:") {
			continue
		}

		// Parse field name and value.
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			// SSE spec: if value starts with a space, remove exactly one space.
			if strings.HasPrefix(value, " ") {
				value = value[1:]
			}
			dataLines = append(dataLines, value)
			continue
		}

		// Unknown field — skip per SSE spec.
	}

	// Check for scanner errors.
	if err := p.scanner.Err(); err != nil {
		return nil, fmt.Errorf("sse scanner error: %w", err)
	}

	// If we have accumulated data at EOF, try to parse it.
	if eventType != "" && len(dataLines) > 0 && eventType != "ping" {
		data := strings.Join(dataLines, "\n")
		evt, err := parseSSEData(eventType, data)
		if err != nil {
			return nil, err
		}
		return evt, nil
	}

	return nil, io.EOF
}

// ParseError is returned when SSE data cannot be parsed as JSON.
type ParseError struct {
	EventType string
	RawData   string
	Err       error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse SSE data for event %q: %s", e.EventType, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// parseSSEData parses the accumulated data for a given event type into an SSEEvent.
func parseSSEData(eventType, data string) (*SSEEvent, error) {
	evt := &SSEEvent{Type: eventType}

	if data == "" {
		return evt, nil
	}

	switch eventType {
	case "message_start":
		// data contains: {"type":"message_start","message":{...}}
		var envelope struct {
			Type    string    `json:"type"`
			Message *Response `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &envelope); err != nil {
			return nil, &ParseError{EventType: eventType, RawData: data, Err: err}
		}
		evt.Message = envelope.Message

	case "content_block_start":
		// data contains: {"type":"content_block_start","index":N,"content_block":{...}}
		var envelope struct {
			Type         string        `json:"type"`
			Index        int           `json:"index"`
			ContentBlock *ContentBlock `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &envelope); err != nil {
			return nil, &ParseError{EventType: eventType, RawData: data, Err: err}
		}
		evt.Index = envelope.Index
		evt.ContentBlock = envelope.ContentBlock

	case "content_block_delta":
		// data contains: {"type":"content_block_delta","index":N,"delta":{...}}
		var envelope struct {
			Type  string             `json:"type"`
			Index int                `json:"index"`
			Delta *ContentBlockDelta `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &envelope); err != nil {
			return nil, &ParseError{EventType: eventType, RawData: data, Err: err}
		}
		evt.Index = envelope.Index
		evt.Delta = envelope.Delta

	case "content_block_stop":
		// data contains: {"type":"content_block_stop","index":N}
		var envelope struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
		}
		if err := json.Unmarshal([]byte(data), &envelope); err != nil {
			return nil, &ParseError{EventType: eventType, RawData: data, Err: err}
		}
		evt.Index = envelope.Index

	case "message_delta":
		// data contains: {"type":"message_delta","delta":{"stop_reason":"..."},"usage":{...}}
		var envelope struct {
			Type  string `json:"type"`
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage *Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &envelope); err != nil {
			return nil, &ParseError{EventType: eventType, RawData: data, Err: err}
		}
		evt.Usage = envelope.Usage
		evt.StopReason = envelope.Delta.StopReason

	case "message_stop":
		// Terminal event, no meaningful data.

	case "error":
		// data contains: {"type":"error","error":{"type":"...","message":"..."}}
		var envelope struct {
			Type  string    `json:"type"`
			Error *APIError `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &envelope); err != nil {
			return nil, &ParseError{EventType: eventType, RawData: data, Err: err}
		}
		evt.Error = envelope.Error

	default:
		// Unknown event type — try generic JSON parse, don't fail.
		var generic map[string]interface{}
		if err := json.Unmarshal([]byte(data), &generic); err != nil {
			// Even if we can't parse it, return the event with type set.
			// Don't fail on unknown event types — forward as-is.
			return evt, nil
		}
	}

	return evt, nil
}
