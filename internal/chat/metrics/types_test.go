package metrics

import "testing"

func TestEventTypeConstants(t *testing.T) {
	events := []string{
		EventSystemExit,
		EventAuthFail,
		EventAuthOK,
		EventWSClose,
		EventWSReconnectAttempt,
		EventWSReconnectSuccess,
		EventWSReconnectFail,
		EventWSStreamResume,
	}
	for _, e := range events {
		if e == "" {
			t.Error("event type constant must not be empty")
		}
	}
}
