package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultCompactionTTL     = 24 * time.Hour
	defaultProxyReadTimeout  = 120 * time.Second
	defaultRetainedTurnCount = 4
)

type compactionShim struct {
	mu          sync.RWMutex
	store       map[string]compactionRecord
	ttl         time.Duration
	upstreamURL string
	client      *http.Client
}

type compactionRecord struct {
	Summary   string
	CreatedAt time.Time
}

type inputMessage struct {
	Role string
	Text string
}

func newCompactionShim(overrideUpstream string) *compactionShim {
	upstream := strings.TrimSpace(overrideUpstream)
	if upstream == "" {
		upstream = strings.TrimSpace(os.Getenv("SOUL_LITELLM_UPSTREAM_URL"))
	}
	return &compactionShim{
		store:       make(map[string]compactionRecord),
		ttl:         defaultCompactionTTL,
		upstreamURL: upstream,
		client:      &http.Client{Timeout: defaultProxyReadTimeout},
	}
}

// handleResponsesCompact implements a local Responses compaction hook:
// it summarizes the input, stores the summary by token, and returns a
// compaction marker object that can be replayed in future /responses calls.
func (s *Server) handleResponsesCompact(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	msgs := extractInputMessages(payload["input"])
	if len(msgs) == 0 {
		msgs = extractInputMessages(payload["messages"])
	}
	if len(msgs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "compact requires non-empty input"})
		return
	}

	summary := buildCompactionSummary(msgs)
	token := s.compaction.put(summary)
	retained := buildRetainedOutput(msgs, defaultRetainedTurnCount)

	output := make([]interface{}, 0, len(retained)+1)
	for _, msg := range retained {
		output = append(output, msg)
	}
	output = append(output, map[string]interface{}{
		"id":                newRandomID("cmp"),
		"type":              "compaction",
		"encrypted_content": token,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         newRandomID("resp_cmp"),
		"object":     "response",
		"created_at": time.Now().Unix(),
		"status":     "completed",
		"output":     output,
		"usage": map[string]interface{}{
			"input_tokens":  0,
			"output_tokens": 0,
			"total_tokens":  0,
		},
	})
}

// handleResponses proxies the request to the configured upstream and expands
// local compaction markers into a single developer message containing summary
// context before forwarding.
func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if input, ok := payload["input"]; ok {
		if expanded, changed := s.compaction.expandInput(input); changed {
			payload["input"] = expanded
		}
	}

	if s.compaction.upstreamURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "responses upstream not configured (set SOUL_LITELLM_UPSTREAM_URL)",
		})
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encode request"})
		return
	}

	upstreamURL := s.compaction.upstreamURL
	if r.URL.RawQuery != "" {
		if strings.Contains(upstreamURL, "?") {
			upstreamURL += "&" + r.URL.RawQuery
		} else {
			upstreamURL += "?" + r.URL.RawQuery
		}
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to create upstream request"})
		return
	}
	copyForwardHeaders(req.Header, r.Header)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.compaction.client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("upstream request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	for k, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func copyForwardHeaders(dst, src http.Header) {
	for k, values := range src {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range values {
			dst.Add(k, v)
		}
	}
}

func (c *compactionShim) put(summary string) string {
	if c == nil {
		return ""
	}
	token := newRandomID("cmp_tok")
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[token] = compactionRecord{
		Summary:   summary,
		CreatedAt: now,
	}
	c.cleanupLocked(now)
	return token
}

func (c *compactionShim) get(token string) (compactionRecord, bool) {
	if c == nil {
		return compactionRecord{}, false
	}
	c.mu.RLock()
	rec, ok := c.store[token]
	c.mu.RUnlock()
	if !ok {
		return compactionRecord{}, false
	}
	if time.Since(rec.CreatedAt) > c.ttl {
		c.mu.Lock()
		delete(c.store, token)
		c.mu.Unlock()
		return compactionRecord{}, false
	}
	return rec, true
}

func (c *compactionShim) cleanupLocked(now time.Time) {
	if c == nil {
		return
	}
	for token, rec := range c.store {
		if now.Sub(rec.CreatedAt) > c.ttl {
			delete(c.store, token)
		}
	}
}

func (c *compactionShim) expandInput(raw interface{}) (interface{}, bool) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return raw, false
	}

	expanded := make([]interface{}, 0, len(items))
	changed := false

	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			expanded = append(expanded, item)
			continue
		}

		if strings.EqualFold(stringField(obj, "type"), "compaction") {
			token := strings.TrimSpace(stringField(obj, "encrypted_content"))
			rec, found := c.get(token)
			if !found {
				continue
			}
			changed = true
			expanded = append(expanded, map[string]interface{}{
				"type": "message",
				"role": "developer",
				"content": []map[string]interface{}{
					{
						"type": "input_text",
						"text": "Compacted context from prior turns:\n" + rec.Summary,
					},
				},
			})
			continue
		}

		expanded = append(expanded, item)
	}

	return expanded, changed
}

func extractInputMessages(raw interface{}) []inputMessage {
	switch v := raw.(type) {
	case string:
		text := normalizeWhitespace(v)
		if text == "" {
			return nil
		}
		return []inputMessage{{Role: "user", Text: text}}
	case map[string]interface{}:
		if msg, ok := parseInputMessage(v); ok {
			return []inputMessage{msg}
		}
	case []interface{}:
		out := make([]inputMessage, 0, len(v))
		for _, item := range v {
			switch t := item.(type) {
			case string:
				text := normalizeWhitespace(t)
				if text != "" {
					out = append(out, inputMessage{Role: "user", Text: text})
				}
			case map[string]interface{}:
				if msg, ok := parseInputMessage(t); ok {
					out = append(out, msg)
				}
			}
		}
		return out
	}
	return nil
}

func parseInputMessage(obj map[string]interface{}) (inputMessage, bool) {
	if strings.EqualFold(stringField(obj, "type"), "compaction") {
		return inputMessage{}, false
	}

	role := canonicalRole(stringField(obj, "role"))
	if role == "" {
		role = "user"
	}

	text := extractText(obj["content"])
	if text == "" {
		text = normalizeWhitespace(stringField(obj, "text"))
	}
	if text == "" {
		return inputMessage{}, false
	}

	return inputMessage{
		Role: role,
		Text: text,
	}, true
}

func extractText(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return normalizeWhitespace(v)
	case map[string]interface{}:
		if txt := normalizeWhitespace(stringField(v, "text")); txt != "" {
			return txt
		}
		if txt := normalizeWhitespace(stringField(v, "content")); txt != "" {
			return txt
		}
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, it := range v {
			switch block := it.(type) {
			case string:
				if txt := normalizeWhitespace(block); txt != "" {
					parts = append(parts, txt)
				}
			case map[string]interface{}:
				if strings.EqualFold(stringField(block, "type"), "compaction") {
					continue
				}
				txt := normalizeWhitespace(stringField(block, "text"))
				if txt == "" {
					txt = normalizeWhitespace(stringField(block, "content"))
				}
				if txt != "" {
					parts = append(parts, txt)
				}
			}
		}
		return normalizeWhitespace(strings.Join(parts, "\n"))
	}
	return ""
}

func buildCompactionSummary(messages []inputMessage) string {
	if len(messages) == 0 {
		return "No prior context."
	}

	var users []string
	var assistants []string
	for _, m := range messages {
		switch canonicalRole(m.Role) {
		case "assistant":
			assistants = append(assistants, clipText(m.Text, 260))
		default:
			users = append(users, clipText(m.Text, 260))
		}
	}

	var b strings.Builder
	b.WriteString("Conversation Summary\n")

	if len(users) > 0 {
		b.WriteString("User intents:\n")
		for _, t := range tailStrings(users, 5) {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	if len(assistants) > 0 {
		b.WriteString("Assistant progress:\n")
		for _, t := range tailStrings(assistants, 4) {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	openQs := make([]string, 0, 3)
	for _, u := range tailStrings(users, 6) {
		if strings.Contains(u, "?") {
			openQs = append(openQs, clipText(u, 180))
		}
		if len(openQs) >= 3 {
			break
		}
	}
	if len(openQs) > 0 {
		b.WriteString("Open questions:\n")
		for _, q := range openQs {
			b.WriteString("- ")
			b.WriteString(q)
			b.WriteString("\n")
		}
	}

	b.WriteString("Carry-forward:\n- Preserve decisions, constraints, and pending tasks listed above.")
	return strings.TrimSpace(b.String())
}

func buildRetainedOutput(messages []inputMessage, n int) []map[string]interface{} {
	if n <= 0 || len(messages) == 0 {
		return nil
	}
	kept := messages
	if len(kept) > n {
		kept = kept[len(kept)-n:]
	}

	out := make([]map[string]interface{}, 0, len(kept))
	for _, m := range kept {
		contentType := "input_text"
		if canonicalRole(m.Role) == "assistant" {
			contentType = "output_text"
		}
		out = append(out, map[string]interface{}{
			"id":     newRandomID("msg_keep"),
			"type":   "message",
			"status": "completed",
			"role":   canonicalRole(m.Role),
			"content": []map[string]interface{}{
				{
					"type": contentType,
					"text": clipText(m.Text, 500),
				},
			},
		})
	}
	return out
}

func canonicalRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "assistant"
	case "developer":
		return "developer"
	case "system":
		return "system"
	default:
		return "user"
	}
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func clipText(s string, max int) string {
	s = normalizeWhitespace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func tailStrings(items []string, n int) []string {
	if n <= 0 || len(items) == 0 {
		return nil
	}
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func stringField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

func newRandomID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
