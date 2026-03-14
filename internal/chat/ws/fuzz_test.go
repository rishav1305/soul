package ws

import (
	"testing"
)

func FuzzInboundMessage(f *testing.F) {
	// Seed with valid messages.
	f.Add([]byte(`{"type":"chat.send","sessionId":"abc","content":"hello"}`))
	f.Add([]byte(`{"type":"session.switch","sessionId":"abc"}`))
	f.Add([]byte(`{"type":"session.create"}`))
	f.Add([]byte(`{"type":"session.delete","sessionId":"abc"}`))

	// Seed with edge cases.
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte{})
	f.Add([]byte(`{"type":"unknown.type"}`))
	f.Add([]byte(`{"type":123}`))
	f.Add([]byte(`{"type":null}`))
	f.Add([]byte(`{"type":"chat.send","sessionId":"","content":""}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`"string"`))
	f.Add([]byte(`{"type":"` + string(make([]byte, 1000)) + `"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic — errors are fine.
		ParseInboundMessage(data)
	})
}
