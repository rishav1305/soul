package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
