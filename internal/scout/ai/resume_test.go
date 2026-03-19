package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// errSender is defined in content_test.go — reuse it here.

func TestResumeTailor(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "resume-baseline.md"), []byte("# Rishav\nSenior Go Engineer"), 0644); err != nil {
		t.Fatal(err)
	}

	sender := &mockSender{response: "# Tailored Resume\nSenior Go Engineer at Stripe"}
	svc := New(st, nil, sender, dataDir)

	text, err := svc.ResumeTailor(context.Background(), id)
	if err != nil {
		t.Fatalf("resume tailor: %v", err)
	}
	if text != "# Tailored Resume\nSenior Go Engineer at Stripe" {
		t.Errorf("text = %q, want tailored resume", text)
	}

	// Verify artifact was stored.
	var content string
	err = st.DB().QueryRow("SELECT content FROM lead_artifacts WHERE lead_id = ? AND type = 'resume'", id).Scan(&content)
	if err != nil {
		t.Fatalf("query artifact: %v", err)
	}
	if content != text {
		t.Errorf("stored content = %q, want %q", content, text)
	}
}

func TestResumeTailor_LeadNotFound(t *testing.T) {
	st := newTestStore(t)
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ResumeTailor(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for missing lead")
	}
}

func TestResumeTailor_NoBaseline(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	// dataDir with no resume-baseline.md
	sender := &mockSender{response: "anything"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.ResumeTailor(context.Background(), id)
	if err == nil {
		t.Fatal("expected error when baseline file missing")
	}
}

func TestResumeTailor_SenderError(t *testing.T) {
	st := newTestStore(t)
	lead := makeTestLead()
	id, _ := st.AddLead(lead)

	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "resume-baseline.md"), []byte("# Resume"), 0644); err != nil {
		t.Fatal(err)
	}

	sender := &errSender{err: errors.New("claude unavailable")}
	svc := New(st, nil, sender, dataDir)

	_, err := svc.ResumeTailor(context.Background(), id)
	if err == nil {
		t.Fatal("expected error from sender")
	}
}
