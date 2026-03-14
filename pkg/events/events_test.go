package events

import "testing"

func TestNopLogger(t *testing.T) {
	var l Logger = NopLogger{}
	if err := l.Log("test", nil); err != nil {
		t.Errorf("NopLogger.Log() = %v, want nil", err)
	}
}

func TestConstants(t *testing.T) {
	if EventOAuthRefresh != "oauth.refresh" {
		t.Errorf("EventOAuthRefresh = %q", EventOAuthRefresh)
	}
	if EventOAuthReload != "oauth.reload" {
		t.Errorf("EventOAuthReload = %q", EventOAuthReload)
	}
}
