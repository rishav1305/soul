package team

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInboxBridge_SendMessage(t *testing.T) {
	dir := t.TempDir()
	ib := NewInboxBridge(dir, "test-team")

	msg, err := ib.SendMessage(SendMessageRequest{
		To:       "shuri",
		Body:     "Build the dashboard",
		Priority: "P1",
		From:     "team-lead",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if msg.To != "shuri" {
		t.Errorf("expected to=shuri, got %s", msg.To)
	}
	if msg.From != "team-lead" {
		t.Errorf("expected from=team-lead, got %s", msg.From)
	}
	if msg.Priority != "P1" {
		t.Errorf("expected priority P1, got %s", msg.Priority)
	}
	if msg.Status != "pending" {
		t.Errorf("expected status pending, got %s", msg.Status)
	}

	// Verify file was written. agentInboxDir falls back to "shuri" when
	// "shuri_shuri" doesn't pre-exist.
	inboxDir := filepath.Join(dir, "inboxes", "shuri", "new")
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		t.Fatalf("read inbox dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file in inbox, got %d", len(entries))
	}
}

func TestInboxBridge_SendMessage_Defaults(t *testing.T) {
	dir := t.TempDir()
	ib := NewInboxBridge(dir, "test-team")

	msg, err := ib.SendMessage(SendMessageRequest{
		To:   "happy",
		Body: "Hello",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if msg.From != "team-lead" {
		t.Errorf("expected default from=team-lead, got %s", msg.From)
	}
	if msg.Priority != "P2" {
		t.Errorf("expected default priority P2, got %s", msg.Priority)
	}
}

func TestInboxBridge_SendMessage_ValidationErrors(t *testing.T) {
	dir := t.TempDir()
	ib := NewInboxBridge(dir, "test-team")

	// Missing recipient.
	_, err := ib.SendMessage(SendMessageRequest{Body: "Hello"})
	if err == nil {
		t.Error("expected error for missing recipient")
	}

	// Missing body.
	_, err = ib.SendMessage(SendMessageRequest{To: "shuri"})
	if err == nil {
		t.Error("expected error for missing body")
	}

	// Invalid priority.
	_, err = ib.SendMessage(SendMessageRequest{To: "shuri", Body: "hi", Priority: "HIGH"})
	if err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestInboxBridge_GetHistory_Empty(t *testing.T) {
	dir := t.TempDir()
	ib := NewInboxBridge(dir, "test-team")

	messages, err := ib.GetHistory("shuri", 50)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestInboxBridge_GetHistory_WithMessages(t *testing.T) {
	dir := t.TempDir()
	ib := NewInboxBridge(dir, "test-team")

	// Send a message first.
	_, err := ib.SendMessage(SendMessageRequest{
		To:   "shuri",
		Body: "Test message",
		From: "team-lead",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Get history for shuri inbox.
	messages, err := ib.GetHistory("shuri", 50)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	// We get the message from the inbox dir.
	if len(messages) == 0 {
		t.Error("expected at least 1 message in history")
	}
}

func TestInboxBridge_ReadDeliveryLog(t *testing.T) {
	dir := t.TempDir()
	ib := NewInboxBridge(dir, "test-team")

	// Write a delivery log.
	logPath := filepath.Join(dir, "delivery-log-shuri.jsonl")
	logData := `{"ts": "2026-03-31T10:00:00Z", "from": "team-lead", "content": "Hello shuri", "priority": "P1", "file": "msg1"}
{"ts": "2026-03-31T11:00:00Z", "from": "shuri", "content": "Hi boss", "priority": "P2", "file": "msg2"}
`
	if err := os.WriteFile(logPath, []byte(logData), 0644); err != nil {
		t.Fatal(err)
	}

	messages := ib.readDeliveryLog(logPath, 50)
	if len(messages) != 2 {
		t.Errorf("expected 2 messages from log, got %d", len(messages))
	}
	if messages[0].From != "team-lead" {
		t.Errorf("expected first message from team-lead, got %s", messages[0].From)
	}
	if messages[1].Body != "Hi boss" {
		t.Errorf("expected second message body 'Hi boss', got %s", messages[1].Body)
	}
}

func TestInboxBridge_ReadMessageFile(t *testing.T) {
	dir := t.TempDir()
	ib := NewInboxBridge(dir, "test-team")

	msgJSON := `{
		"from": "friday",
		"to": "team-lead",
		"timestamp": "2026-03-25T15:57:00+05:30",
		"priority": "P1",
		"subject": "Test subject",
		"body": "Test body"
	}`

	path := filepath.Join(dir, "test_msg.json")
	if err := os.WriteFile(path, []byte(msgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	msg, err := ib.readMessageFile(path)
	if err != nil {
		t.Fatalf("readMessageFile: %v", err)
	}

	if msg.From != "friday" {
		t.Errorf("expected from=friday, got %s", msg.From)
	}
	if msg.Subject != "Test subject" {
		t.Errorf("expected subject 'Test subject', got %s", msg.Subject)
	}
	if msg.Priority != "P1" {
		t.Errorf("expected priority P1, got %s", msg.Priority)
	}
}
