package stream

import (
	"bytes"
	"testing"
)

func FuzzSSEParser(f *testing.F) {
	// Seeds with real SSE data.
	f.Add([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-sonnet-4-20250514\",\"stop_reason\":null}}\n\n"))
	f.Add([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n"))
	f.Add([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
	f.Add([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
	f.Add([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":12}}\n\n"))
	f.Add([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	f.Add([]byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n"))

	// Edge cases.
	f.Add([]byte(":comment\n\n"))
	f.Add([]byte("retry: 1000\n\n"))
	f.Add([]byte{})
	f.Add([]byte("event: unknown\ndata: {}\n\n"))
	f.Add([]byte("data: not json\n\n"))
	f.Add([]byte("\n\n\n"))
	f.Add([]byte("event: ping\ndata: {\"type\":\"ping\"}\n\n"))
	f.Add([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\ndata: \"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"multi-line\"}}\n\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		p := NewSSEParser(bytes.NewReader(data))
		for {
			_, err := p.Next()
			if err != nil {
				break
			}
		}
	})
}
