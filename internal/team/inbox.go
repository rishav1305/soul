package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// InboxBridge handles reading and writing messages in the clawteam inbox system.
type InboxBridge struct {
	clawteamDir string
	teamName    string
}

// NewInboxBridge creates an InboxBridge for the given clawteam directory.
func NewInboxBridge(clawteamDir, teamName string) *InboxBridge {
	return &InboxBridge{
		clawteamDir: clawteamDir,
		teamName:    teamName,
	}
}

// inboxMessage mirrors the JSON structure of inbox message files.
type inboxMessage struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Timestamp string `json:"timestamp"`
	Priority  string `json:"priority"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

// SendMessage writes a message file to the target agent's inbox.
func (ib *InboxBridge) SendMessage(req SendMessageRequest) (ChatMessage, error) {
	if req.To == "" {
		return ChatMessage{}, fmt.Errorf("recipient (to) is required")
	}
	if req.Body == "" {
		return ChatMessage{}, fmt.Errorf("message body is required")
	}
	if req.From == "" {
		req.From = "team-lead"
	}
	if req.Priority == "" {
		req.Priority = "P2"
	}

	// Validate priority.
	switch req.Priority {
	case "P1", "P2", "P3":
	default:
		return ChatMessage{}, fmt.Errorf("invalid priority %q: must be P1, P2, or P3", req.Priority)
	}

	now := time.Now()
	msgID := fmt.Sprintf("%s_%s_from_%s", now.Format("20060102_150405"), fmt.Sprintf("%d", now.UnixMilli()%10000), req.From)

	msg := inboxMessage{
		From:      req.From,
		To:        req.To,
		Timestamp: now.Format(time.RFC3339),
		Priority:  req.Priority,
		Body:      req.Body,
	}

	// Determine inbox path. Inbox dirs are named like "agent_agent".
	inboxDir := ib.agentInboxDir(req.To)
	newDir := filepath.Join(inboxDir, "new")

	// Ensure directories exist.
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return ChatMessage{}, fmt.Errorf("create inbox dir: %w", err)
	}

	filename := msgID + ".json"
	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return ChatMessage{}, fmt.Errorf("marshal message: %w", err)
	}

	if err := os.WriteFile(filepath.Join(newDir, filename), data, 0644); err != nil {
		return ChatMessage{}, fmt.Errorf("write message: %w", err)
	}

	return ChatMessage{
		ID:        msgID,
		From:      req.From,
		To:        req.To,
		Body:      req.Body,
		Priority:  req.Priority,
		Timestamp: now,
		Status:    "pending",
	}, nil
}

// GetHistory reads the delivery log and recent inbox messages for chat history.
func (ib *InboxBridge) GetHistory(agent string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}

	var messages []ChatMessage

	// Read from delivery log (contains all delivered messages).
	logPath := filepath.Join(ib.clawteamDir, fmt.Sprintf("delivery-log-%s.jsonl", agent))
	messages = append(messages, ib.readDeliveryLog(logPath, limit)...)

	// Also read from agent's inbox (pending messages).
	inboxDir := ib.agentInboxDir(agent)
	messages = append(messages, ib.readInboxDir(inboxDir)...)

	// Sort by timestamp, newest first.
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})

	// Deduplicate by ID.
	seen := make(map[string]bool)
	deduped := make([]ChatMessage, 0, len(messages))
	for _, m := range messages {
		key := m.From + m.Timestamp.Format(time.RFC3339)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, m)
	}

	if len(deduped) > limit {
		deduped = deduped[:limit]
	}

	return deduped, nil
}

// deliveryLogEntry mirrors the JSONL format of delivery logs.
type deliveryLogEntry struct {
	TS       string `json:"ts"`
	From     string `json:"from"`
	Content  string `json:"content"`
	Priority string `json:"priority"`
	File     string `json:"file"`
}

func (ib *InboxBridge) readDeliveryLog(path string, limit int) []ChatMessage {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var messages []ChatMessage

	// Read from the end (newest first).
	start := 0
	if len(lines) > limit {
		start = len(lines) - limit
	}

	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var entry deliveryLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		ts := parseTimeOr(entry.TS, time.Time{})
		messages = append(messages, ChatMessage{
			ID:        entry.File,
			From:      entry.From,
			Body:      entry.Content,
			Priority:  entry.Priority,
			Timestamp: ts,
			Status:    "delivered",
		})
	}

	return messages
}

func (ib *InboxBridge) readInboxDir(dir string) []ChatMessage {
	var messages []ChatMessage

	// Read from new/ subdirectory.
	newDir := filepath.Join(dir, "new")
	entries, err := os.ReadDir(newDir)
	if err != nil {
		// Try reading top-level JSON files.
		entries, err = os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			if msg, err := ib.readMessageFile(filepath.Join(dir, entry.Name())); err == nil {
				messages = append(messages, msg)
			}
		}
		return messages
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if msg, err := ib.readMessageFile(filepath.Join(newDir, entry.Name())); err == nil {
			messages = append(messages, msg)
		}
	}

	// Also read archive/ for recent history.
	archiveDir := filepath.Join(dir, "archive")
	archiveEntries, err := os.ReadDir(archiveDir)
	if err == nil {
		for _, entry := range archiveEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			if msg, err := ib.readMessageFile(filepath.Join(archiveDir, entry.Name())); err == nil {
				msg.Status = "read"
				messages = append(messages, msg)
			}
		}
	}

	return messages
}

func (ib *InboxBridge) readMessageFile(path string) (ChatMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ChatMessage{}, err
	}

	var msg inboxMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return ChatMessage{}, err
	}

	ts := parseTimeOr(msg.Timestamp, time.Time{})
	return ChatMessage{
		ID:        filepath.Base(path),
		From:      msg.From,
		To:        msg.To,
		Subject:   msg.Subject,
		Body:      msg.Body,
		Priority:  msg.Priority,
		Timestamp: ts,
		Status:    "pending",
	}, nil
}

func (ib *InboxBridge) agentInboxDir(agent string) string {
	// Convention: inbox dirs named "agent_agent" or just "agent".
	dirName := agent + "_" + agent
	dir := filepath.Join(ib.clawteamDir, "inboxes", dirName)
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	// Fallback to plain agent name.
	return filepath.Join(ib.clawteamDir, "inboxes", agent)
}
