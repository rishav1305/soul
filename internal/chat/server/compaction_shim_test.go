package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResponsesCompact_ReturnsCompactionMarker(t *testing.T) {
	srv := newTestServer(t)

	body := `{
		"model": "test-model",
		"input": [
			{"type":"message","role":"user","content":[{"type":"input_text","text":"How many projects do we have?"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"We currently have 8 projects."}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"Give me a grouped breakdown."}]}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses/compact", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	output, ok := resp["output"].([]interface{})
	if !ok || len(output) == 0 {
		t.Fatalf("expected non-empty output array, got %T", resp["output"])
	}

	last, ok := output[len(output)-1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected output tail to be object, got %T", output[len(output)-1])
	}
	if last["type"] != "compaction" {
		t.Fatalf("expected last output type=compaction, got %v", last["type"])
	}
	token, _ := last["encrypted_content"].(string)
	if strings.TrimSpace(token) == "" {
		t.Fatal("expected non-empty encrypted_content token")
	}
}

func TestResponses_ExpandsCompactionBeforeForwarding(t *testing.T) {
	var upstreamBody map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("upstream method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"resp_upstream","object":"response","status":"completed","output":[]}`))
	}))
	defer upstream.Close()

	t.Setenv("SOUL_LITELLM_UPSTREAM_URL", upstream.URL)
	srv := newTestServer(t)

	compactReq := `{
		"input": [
			{"type":"message","role":"user","content":[{"type":"input_text","text":"Track all project counts by status."}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Sure, I will group by status."}]}
		]
	}`
	req1 := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses/compact", strings.NewReader(compactReq))
	rec1 := httptest.NewRecorder()
	srv.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("compact expected 200, got %d", rec1.Code)
	}

	var compactResp map[string]interface{}
	if err := json.NewDecoder(rec1.Body).Decode(&compactResp); err != nil {
		t.Fatalf("decode compact response: %v", err)
	}
	output, ok := compactResp["output"].([]interface{})
	if !ok || len(output) == 0 {
		t.Fatalf("compact output invalid: %T", compactResp["output"])
	}
	tail := output[len(output)-1].(map[string]interface{})
	token := tail["encrypted_content"].(string)

	responsesReq := `{
		"model": "test-model",
		"input": [
			{"type":"compaction","encrypted_content":"` + token + `"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"Now answer in one paragraph."}]}
		]
	}`
	req2 := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses", strings.NewReader(responsesReq))
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("responses expected 200, got %d", rec2.Code)
	}

	input, ok := upstreamBody["input"].([]interface{})
	if !ok || len(input) == 0 {
		t.Fatalf("upstream input missing/invalid: %T", upstreamBody["input"])
	}

	first, ok := input[0].(map[string]interface{})
	if !ok {
		t.Fatalf("upstream first input type = %T", input[0])
	}
	if first["type"] != "message" {
		t.Fatalf("expected first expanded input type=message, got %v", first["type"])
	}
	if first["role"] != "developer" {
		t.Fatalf("expected first expanded role=developer, got %v", first["role"])
	}

	content, ok := first["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("expanded developer content invalid: %T", first["content"])
	}
	block := content[0].(map[string]interface{})
	text := block["text"].(string)
	if !strings.Contains(text, "Compacted context from prior turns:") {
		t.Fatalf("expanded summary text missing prefix, got %q", text)
	}
}

// --- Compaction shim internal function tests ---

func TestCanonicalRole(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"assistant", "assistant"},
		{"ASSISTANT", "assistant"},
		{" assistant ", "assistant"},
		{"developer", "developer"},
		{"system", "system"},
		{"user", "user"},
		{"anything_else", "user"},
		{"", "user"},
	}
	for _, tt := range tests {
		got := canonicalRole(tt.input)
		if got != tt.want {
			t.Errorf("canonicalRole(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  hello   world  ", "hello world"},
		{"no extra spaces", "no extra spaces"},
		{"\n\ttabs and\nnewlines\n", "tabs and newlines"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		got := normalizeWhitespace(tt.input)
		if got != tt.want {
			t.Errorf("normalizeWhitespace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClipText(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"", 10, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"hello", 0, "hello"}, // max <= 0 returns as-is
		{"hello world this is long", 15, "hello world ..."},
	}
	for _, tt := range tests {
		got := clipText(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("clipText(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestTailStrings(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		n     int
		want  int
	}{
		{"nil items", nil, 3, 0},
		{"empty items", []string{}, 3, 0},
		{"fewer than n", []string{"a", "b"}, 5, 2},
		{"exact n", []string{"a", "b", "c"}, 3, 3},
		{"more than n", []string{"a", "b", "c", "d", "e"}, 3, 3},
		{"n=0", []string{"a"}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tailStrings(tt.items, tt.n)
			if len(got) != tt.want {
				t.Errorf("tailStrings(..., %d) returned %d items, want %d", tt.n, len(got), tt.want)
			}
		})
	}

	// Verify tail semantics: returns last n items
	result := tailStrings([]string{"a", "b", "c", "d"}, 2)
	if result[0] != "c" || result[1] != "d" {
		t.Errorf("tailStrings should return last items, got %v", result)
	}
}

func TestStringField(t *testing.T) {
	tests := []struct {
		name  string
		m     map[string]interface{}
		key   string
		want  string
	}{
		{"string value", map[string]interface{}{"k": "v"}, "k", "v"},
		{"missing key", map[string]interface{}{"k": "v"}, "missing", ""},
		{"nil map", nil, "k", ""},
		{"nil value", map[string]interface{}{"k": nil}, "k", ""},
		{"int value", map[string]interface{}{"k": 42}, "k", "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringField(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("stringField(..., %q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"string", "hello world", "hello world"},
		{"string with whitespace", "  hello   world  ", "hello world"},
		{"map with text", map[string]interface{}{"text": "from map"}, "from map"},
		{"map with content", map[string]interface{}{"content": "from content"}, "from content"},
		{"empty map", map[string]interface{}{}, ""},
		{"nil", nil, ""},
		{"array of strings", []interface{}{"part1", "part2"}, "part1 part2"},
		{"array of maps", []interface{}{
			map[string]interface{}{"type": "input_text", "text": "first"},
			map[string]interface{}{"type": "input_text", "text": "second"},
		}, "first second"},
		{"array skips compaction", []interface{}{
			map[string]interface{}{"type": "compaction", "text": "skip"},
			map[string]interface{}{"type": "input_text", "text": "keep"},
		}, "keep"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.input)
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildCompactionSummary_Empty(t *testing.T) {
	result := buildCompactionSummary(nil)
	if result != "No prior context." {
		t.Errorf("expected 'No prior context.', got %q", result)
	}
}

func TestBuildCompactionSummary_WithMessages(t *testing.T) {
	msgs := []inputMessage{
		{Role: "user", Text: "How do I deploy?"},
		{Role: "assistant", Text: "You can deploy using make deploy."},
		{Role: "user", Text: "What about rollback?"},
	}

	result := buildCompactionSummary(msgs)
	if !strings.Contains(result, "Conversation Summary") {
		t.Error("expected summary header")
	}
	if !strings.Contains(result, "User intents:") {
		t.Error("expected User intents section")
	}
	if !strings.Contains(result, "Assistant progress:") {
		t.Error("expected Assistant progress section")
	}
	if !strings.Contains(result, "deploy") {
		t.Error("expected deploy keyword in summary")
	}
	if !strings.Contains(result, "Open questions:") {
		t.Error("expected open questions for messages with '?'")
	}
}

func TestBuildRetainedOutput_Empty(t *testing.T) {
	result := buildRetainedOutput(nil, 3)
	if result != nil {
		t.Errorf("expected nil for empty messages, got %v", result)
	}

	result = buildRetainedOutput([]inputMessage{{Role: "user", Text: "hi"}}, 0)
	if result != nil {
		t.Errorf("expected nil for n=0, got %v", result)
	}
}

func TestBuildRetainedOutput_KeepsLastN(t *testing.T) {
	msgs := []inputMessage{
		{Role: "user", Text: "first"},
		{Role: "assistant", Text: "second"},
		{Role: "user", Text: "third"},
	}

	result := buildRetainedOutput(msgs, 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 retained messages, got %d", len(result))
	}

	// Should be the last 2 messages
	if result[0]["role"] != "assistant" {
		t.Errorf("expected first retained role=assistant, got %v", result[0]["role"])
	}
	if result[1]["role"] != "user" {
		t.Errorf("expected second retained role=user, got %v", result[1]["role"])
	}
}

func TestNewRandomID(t *testing.T) {
	id1 := newRandomID("test")
	id2 := newRandomID("test")
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if !strings.HasPrefix(id1, "test_") {
		t.Errorf("expected prefix 'test_', got %q", id1)
	}
}

func TestCompactionShim_PutGet(t *testing.T) {
	shim := newCompactionShim("")
	token := shim.put("test summary content")
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	record, ok := shim.get(token)
	if !ok {
		t.Fatal("expected to find record")
	}
	if record.Summary != "test summary content" {
		t.Errorf("expected summary 'test summary content', got %q", record.Summary)
	}
}

func TestCompactionShim_GetNotFound(t *testing.T) {
	shim := newCompactionShim("")
	_, ok := shim.get("nonexistent-token")
	if ok {
		t.Error("expected false for nonexistent token")
	}
}

// --- Additional coverage tests ---

func TestExtractInputMessages_StringInput(t *testing.T) {
	msgs := extractInputMessages("hello world")
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("role = %q, want user", msgs[0].Role)
	}
	if msgs[0].Text != "hello world" {
		t.Errorf("text = %q, want 'hello world'", msgs[0].Text)
	}
}

func TestExtractInputMessages_EmptyString(t *testing.T) {
	msgs := extractInputMessages("   ")
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for whitespace-only string, got %d", len(msgs))
	}
}

func TestExtractInputMessages_SingleMap(t *testing.T) {
	msgs := extractInputMessages(map[string]interface{}{
		"role":    "assistant",
		"content": "I can help.",
	})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "assistant" {
		t.Errorf("role = %q, want assistant", msgs[0].Role)
	}
	if msgs[0].Text != "I can help." {
		t.Errorf("text = %q", msgs[0].Text)
	}
}

func TestExtractInputMessages_NilInput(t *testing.T) {
	msgs := extractInputMessages(nil)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for nil, got %d", len(msgs))
	}
}

func TestExtractInputMessages_IntInput(t *testing.T) {
	msgs := extractInputMessages(42)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for int, got %d", len(msgs))
	}
}

func TestExtractInputMessages_ArrayOfStrings(t *testing.T) {
	msgs := extractInputMessages([]interface{}{"hello", "world"})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Text != "hello" || msgs[1].Text != "world" {
		t.Errorf("texts = %q, %q", msgs[0].Text, msgs[1].Text)
	}
}

func TestExtractInputMessages_ArrayWithEmptyStrings(t *testing.T) {
	msgs := extractInputMessages([]interface{}{"hello", "   ", "world"})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (skip empty), got %d", len(msgs))
	}
}

func TestExtractInputMessages_ArrayMixed(t *testing.T) {
	msgs := extractInputMessages([]interface{}{
		"direct string",
		map[string]interface{}{"role": "assistant", "content": "map message"},
		42, // non-string/non-map items are silently skipped
	})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestExtractInputMessages_MapCompactionSkipped(t *testing.T) {
	msgs := extractInputMessages(map[string]interface{}{
		"type":              "compaction",
		"encrypted_content": "tok_123",
	})
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for compaction map, got %d", len(msgs))
	}
}

func TestParseInputMessage_CompactionType(t *testing.T) {
	_, ok := parseInputMessage(map[string]interface{}{
		"type":              "compaction",
		"encrypted_content": "tok_123",
	})
	if ok {
		t.Error("expected false for compaction type")
	}
}

func TestParseInputMessage_EmptyContent(t *testing.T) {
	_, ok := parseInputMessage(map[string]interface{}{
		"role":    "user",
		"content": "",
	})
	if ok {
		t.Error("expected false for empty content")
	}
}

func TestParseInputMessage_TextFallback(t *testing.T) {
	msg, ok := parseInputMessage(map[string]interface{}{
		"role": "developer",
		"text": "fallback text",
	})
	if !ok {
		t.Fatal("expected true")
	}
	if msg.Role != "developer" {
		t.Errorf("role = %q, want developer", msg.Role)
	}
	if msg.Text != "fallback text" {
		t.Errorf("text = %q", msg.Text)
	}
}

func TestParseInputMessage_NoRole(t *testing.T) {
	msg, ok := parseInputMessage(map[string]interface{}{
		"content": "no role specified",
	})
	if !ok {
		t.Fatal("expected true")
	}
	if msg.Role != "user" {
		t.Errorf("role = %q, want user (default)", msg.Role)
	}
}

func TestCopyForwardHeaders(t *testing.T) {
	src := http.Header{}
	src.Set("Authorization", "Bearer tok123")
	src.Set("Content-Type", "application/json")
	src.Set("Content-Length", "42")
	src.Set("Host", "example.com")
	src.Set("X-Custom", "keep-me")

	dst := http.Header{}
	copyForwardHeaders(dst, src)

	if dst.Get("Authorization") != "Bearer tok123" {
		t.Error("expected Authorization header to be forwarded")
	}
	if dst.Get("X-Custom") != "keep-me" {
		t.Error("expected X-Custom header to be forwarded")
	}
	if dst.Get("Content-Length") != "" {
		t.Error("expected Content-Length to be skipped")
	}
	if dst.Get("Host") != "" {
		t.Error("expected Host to be skipped")
	}
}

func TestCopyForwardHeaders_Empty(t *testing.T) {
	dst := http.Header{}
	copyForwardHeaders(dst, http.Header{})
	if len(dst) != 0 {
		t.Errorf("expected empty dst, got %d headers", len(dst))
	}
}

func TestCompactionShim_GetExpired(t *testing.T) {
	shim := newCompactionShim("")
	shim.ttl = 1 * time.Millisecond

	token := shim.put("expiring summary")

	// Wait for TTL to expire.
	time.Sleep(5 * time.Millisecond)

	_, ok := shim.get(token)
	if ok {
		t.Error("expected false for expired record")
	}
}

func TestCompactionShim_CleanupRemovesExpired(t *testing.T) {
	shim := newCompactionShim("")
	shim.ttl = 1 * time.Millisecond

	shim.put("old1")
	shim.put("old2")

	// Wait for TTL to expire, then put a new record which triggers cleanup.
	time.Sleep(5 * time.Millisecond)
	shim.put("new")

	shim.mu.RLock()
	count := len(shim.store)
	shim.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 record after cleanup, got %d", count)
	}
}

func TestCompactionShim_NilPutGet(t *testing.T) {
	var shim *compactionShim
	token := shim.put("test")
	if token != "" {
		t.Errorf("expected empty token from nil shim, got %q", token)
	}
	_, ok := shim.get("any")
	if ok {
		t.Error("expected false from nil shim get")
	}
}

func TestCompactionShim_NilCleanup(t *testing.T) {
	var shim *compactionShim
	// Should not panic.
	shim.cleanupLocked(time.Now())
}

func TestExpandInput_NonArray(t *testing.T) {
	shim := newCompactionShim("")
	result, changed := shim.expandInput("not an array")
	if changed {
		t.Error("expected changed=false for non-array")
	}
	if result != "not an array" {
		t.Errorf("expected original value returned")
	}
}

func TestExpandInput_EmptyArray(t *testing.T) {
	shim := newCompactionShim("")
	result, changed := shim.expandInput([]interface{}{})
	if changed {
		t.Error("expected changed=false for empty array")
	}
	_ = result
}

func TestExpandInput_CompactionNotFound(t *testing.T) {
	shim := newCompactionShim("")
	input := []interface{}{
		map[string]interface{}{
			"type":              "compaction",
			"encrypted_content": "nonexistent_token",
		},
		map[string]interface{}{
			"type": "message",
			"role": "user",
		},
	}
	result, changed := shim.expandInput(input)
	// When compaction token is not found, item is silently dropped but changed stays false.
	if changed {
		t.Error("expected changed=false when compaction not found")
	}
	expanded := result.([]interface{})
	// Compaction item was dropped, only message remains.
	if len(expanded) != 1 {
		t.Errorf("expected 1 item after dropping unfound compaction, got %d", len(expanded))
	}
}

func TestExpandInput_NonObjectItems(t *testing.T) {
	shim := newCompactionShim("")
	input := []interface{}{"string item", 42, true}
	result, changed := shim.expandInput(input)
	if changed {
		t.Error("expected changed=false for non-object items")
	}
	expanded := result.([]interface{})
	if len(expanded) != 3 {
		t.Errorf("expected 3 items, got %d", len(expanded))
	}
}

func TestHandleResponsesCompact_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses/compact", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleResponsesCompact_EmptyInput(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses/compact",
		strings.NewReader(`{"model":"test","input":[]}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty input, got %d", rec.Code)
	}
}

func TestHandleResponses_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleResponses_NoUpstream(t *testing.T) {
	// newTestServer creates a server without upstream configured.
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses",
		strings.NewReader(`{"model":"test","input":"hello"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for no upstream, got %d", rec.Code)
	}
}

func TestHandleResponses_QueryParams(t *testing.T) {
	var capturedURL string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	t.Setenv("SOUL_LITELLM_UPSTREAM_URL", upstream.URL)
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses?beta=v2",
		strings.NewReader(`{"model":"test","input":"hello"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(capturedURL, "beta=v2") {
		t.Errorf("expected query param forwarded, captured URL: %q", capturedURL)
	}
}

func TestExtractText_MapWithContentKey(t *testing.T) {
	got := extractText(map[string]interface{}{
		"content": "content value",
	})
	if got != "content value" {
		t.Errorf("extractText map content = %q, want 'content value'", got)
	}
}

func TestExtractText_ArrayWithContentFallback(t *testing.T) {
	got := extractText([]interface{}{
		map[string]interface{}{"content": "fallback content"},
	})
	if got != "fallback content" {
		t.Errorf("extractText = %q, want 'fallback content'", got)
	}
}

func TestHandleResponsesCompact_WithStringInput(t *testing.T) {
	srv := newTestServer(t)

	body := `{"model":"test","input":"What is the meaning of life?"}`
	req := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses/compact", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	output, _ := resp["output"].([]interface{})
	if len(output) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func TestHandleResponsesCompact_FallbackToMessages(t *testing.T) {
	srv := newTestServer(t)

	// Uses "messages" key instead of "input" — compact should fall back.
	body := `{"model":"test","messages":[{"role":"user","content":"test message"}]}`
	req := httptest.NewRequest(http.MethodPost, "/litellm/v1/responses/compact", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware_ProtectsLiteLLMRoutes(t *testing.T) {
	srv := New(WithAuthToken("secret123"))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/litellm/v1/responses/compact", strings.NewReader(`{"input":[{"type":"message","role":"user","content":"x"}]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth on litellm route, got %d", resp.StatusCode)
	}
}
