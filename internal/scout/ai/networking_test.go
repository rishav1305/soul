package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/chat/stream"
	"github.com/rishav1305/soul/internal/scout/store"
)

// captureSender records the system prompt and user message for verification.
type captureSender struct {
	response string
	system   string
	userMsg  string
}

func (c *captureSender) Send(ctx context.Context, req *stream.Request) (*stream.Response, error) {
	c.system = req.System
	if len(req.Messages) > 0 && len(req.Messages[0].Content) > 0 {
		c.userMsg = req.Messages[0].Content[0].Text
	}
	return &stream.Response{
		Content: []stream.ContentBlock{{Type: "text", Text: c.response}},
	}, nil
}

func makeNetworkingLead() store.Lead {
	return store.Lead{
		Source:        "manual",
		Pipeline:      "networking",
		Stage:         "discovered",
		Company:       "Anthropic",
		HiringManager: "Jane Smith",
		ContactType:   "engineering_lead",
		Intent:        "collaboration",
		Warmth:        "warm",
	}
}

func TestNetworkingDraft_LinkedIn(t *testing.T) {
	st := newTestStore(t)
	lead := makeNetworkingLead()
	id, _ := st.AddLead(lead)

	sender := &captureSender{response: "Hi Jane, loved your recent post about LLM scaling..."}
	svc := New(st, nil, sender, t.TempDir())

	text, err := svc.NetworkingDraft(context.Background(), id, "linkedin", "Posted about LLM scaling laws")
	if err != nil {
		t.Fatalf("networking draft: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty draft")
	}
	if !strings.Contains(sender.system, "LinkedIn networking expert") {
		t.Errorf("expected LinkedIn system prompt, got: %s", sender.system)
	}
	if !strings.Contains(sender.userMsg, "Jane Smith") {
		t.Errorf("expected contact name in user msg, got: %s", sender.userMsg)
	}
	if !strings.Contains(sender.userMsg, "Anthropic") {
		t.Errorf("expected company in user msg, got: %s", sender.userMsg)
	}
	if !strings.Contains(sender.userMsg, "linkedin") {
		t.Errorf("expected channel in user msg, got: %s", sender.userMsg)
	}
}

func TestNetworkingDraft_X(t *testing.T) {
	st := newTestStore(t)
	lead := makeNetworkingLead()
	id, _ := st.AddLead(lead)

	sender := &captureSender{response: "Great thread on scaling..."}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.NetworkingDraft(context.Background(), id, "x", "Tweeted about Go concurrency")
	if err != nil {
		t.Fatalf("networking draft x: %v", err)
	}
	if !strings.Contains(sender.system, "X/Twitter reply") {
		t.Errorf("expected X system prompt, got: %s", sender.system)
	}
}

func TestNetworkingDraft_Email(t *testing.T) {
	st := newTestStore(t)
	lead := makeNetworkingLead()
	id, _ := st.AddLead(lead)

	sender := &captureSender{response: "Subject: Interesting approach to..."}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.NetworkingDraft(context.Background(), id, "email", "Published blog on distributed systems")
	if err != nil {
		t.Fatalf("networking draft email: %v", err)
	}
	if !strings.Contains(sender.system, "cold outreach email") {
		t.Errorf("expected email system prompt, got: %s", sender.system)
	}
}

func TestNetworkingDraft_DefaultChannel(t *testing.T) {
	st := newTestStore(t)
	lead := makeNetworkingLead()
	id, _ := st.AddLead(lead)

	sender := &captureSender{response: "draft text"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.NetworkingDraft(context.Background(), id, "unknown_channel", "some activity")
	if err != nil {
		t.Fatalf("networking draft default: %v", err)
	}
	if !strings.Contains(sender.system, "LinkedIn networking expert") {
		t.Errorf("expected LinkedIn (default) system prompt for unknown channel, got: %s", sender.system)
	}
}

func TestNetworkingDraft_InvalidLead(t *testing.T) {
	st := newTestStore(t)

	sender := &captureSender{response: "draft"}
	svc := New(st, nil, sender, t.TempDir())

	_, err := svc.NetworkingDraft(context.Background(), 99999, "linkedin", "activity")
	if err == nil {
		t.Fatal("expected error for non-existent lead")
	}
}

func TestWeeklyNetworkingBrief_Groups(t *testing.T) {
	st := newTestStore(t)
	now := time.Now().UTC()
	recent := now.Add(-5 * 24 * time.Hour).Format(time.RFC3339)
	old := now.Add(-60 * 24 * time.Hour).Format(time.RFC3339)

	// Warm contact: warmth=warm, recent interaction, 5 interactions.
	warm := makeNetworkingLead()
	warm.HiringManager = "Alice Warm"
	warm.Warmth = "warm"
	warm.InteractionCount = 5
	warm.LastInteractionAt = recent
	warm.CreatedAt = recent
	st.AddLead(warm)

	// Ready contact: warmth=ready, recent interaction, 8 interactions.
	ready := makeNetworkingLead()
	ready.HiringManager = "Bob Ready"
	ready.Warmth = "ready"
	ready.InteractionCount = 8
	ready.LastInteractionAt = recent
	ready.CreatedAt = recent
	st.AddLead(ready)

	// Dormant contact: last interaction 60 days ago.
	dormant := makeNetworkingLead()
	dormant.HiringManager = "Carol Dormant"
	dormant.Warmth = "warm"
	dormant.InteractionCount = 4
	dormant.LastInteractionAt = old
	dormant.CreatedAt = old
	st.AddLead(dormant)

	// Dormant by creation: no interaction, created 60 days ago.
	dormant2 := makeNetworkingLead()
	dormant2.HiringManager = "Dave Old"
	dormant2.Warmth = "new"
	dormant2.InteractionCount = 0
	dormant2.LastInteractionAt = ""
	dormant2.CreatedAt = old
	st.AddLead(dormant2)

	sender := &mockSender{response: "unused"}
	svc := New(st, nil, sender, t.TempDir())

	brief, err := svc.WeeklyNetworkingBrief(context.Background())
	if err != nil {
		t.Fatalf("weekly brief: %v", err)
	}

	if len(brief.WarmContacts) != 1 {
		t.Errorf("warm contacts = %d, want 1", len(brief.WarmContacts))
	}
	if len(brief.ReadyContacts) != 1 {
		t.Errorf("ready contacts = %d, want 1", len(brief.ReadyContacts))
	}
	if len(brief.DormantContacts) != 2 {
		t.Errorf("dormant contacts = %d, want 2", len(brief.DormantContacts))
	}

	if brief.WarmContacts[0].Name != "Alice Warm" {
		t.Errorf("warm contact name = %q, want Alice Warm", brief.WarmContacts[0].Name)
	}
	if brief.ReadyContacts[0].Name != "Bob Ready" {
		t.Errorf("ready contact name = %q, want Bob Ready", brief.ReadyContacts[0].Name)
	}

	if !strings.Contains(brief.Summary, "1 contacts warm") {
		t.Errorf("summary missing warm count: %s", brief.Summary)
	}
	if !strings.Contains(brief.Summary, "2 dormant") {
		t.Errorf("summary missing dormant count: %s", brief.Summary)
	}
	if !strings.Contains(brief.Summary, "1 ready") {
		t.Errorf("summary missing ready count: %s", brief.Summary)
	}
}

func TestWeeklyNetworkingBrief_NoContacts(t *testing.T) {
	st := newTestStore(t)

	sender := &mockSender{response: "unused"}
	svc := New(st, nil, sender, t.TempDir())

	brief, err := svc.WeeklyNetworkingBrief(context.Background())
	if err != nil {
		t.Fatalf("weekly brief empty: %v", err)
	}
	if len(brief.WarmContacts) != 0 {
		t.Errorf("warm = %d, want 0", len(brief.WarmContacts))
	}
	if len(brief.DormantContacts) != 0 {
		t.Errorf("dormant = %d, want 0", len(brief.DormantContacts))
	}
	if len(brief.ReadyContacts) != 0 {
		t.Errorf("ready = %d, want 0", len(brief.ReadyContacts))
	}
	if !strings.Contains(brief.Summary, "0 contacts warm") {
		t.Errorf("summary = %q, expected zeros", brief.Summary)
	}
}

func TestWeeklyNetworkingBrief_AllDormant(t *testing.T) {
	st := newTestStore(t)
	old := time.Now().UTC().Add(-90 * 24 * time.Hour).Format(time.RFC3339)

	for i := 0; i < 3; i++ {
		lead := makeNetworkingLead()
		lead.HiringManager = "Dormant Person"
		lead.Warmth = "warm"
		lead.InteractionCount = 3
		lead.LastInteractionAt = old
		lead.CreatedAt = old
		st.AddLead(lead)
	}

	sender := &mockSender{response: "unused"}
	svc := New(st, nil, sender, t.TempDir())

	brief, err := svc.WeeklyNetworkingBrief(context.Background())
	if err != nil {
		t.Fatalf("all dormant brief: %v", err)
	}
	if len(brief.DormantContacts) != 3 {
		t.Errorf("dormant = %d, want 3", len(brief.DormantContacts))
	}
	if len(brief.WarmContacts) != 0 {
		t.Errorf("warm = %d, want 0", len(brief.WarmContacts))
	}
	if len(brief.ReadyContacts) != 0 {
		t.Errorf("ready = %d, want 0", len(brief.ReadyContacts))
	}
}

func TestWeeklyNetworkingBrief_ByInteractionCount(t *testing.T) {
	st := newTestStore(t)
	recent := time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339)

	// Lead with interaction_count 7+ but warmth not explicitly "ready".
	high := makeNetworkingLead()
	high.HiringManager = "High Interaction"
	high.Warmth = "warm"
	high.InteractionCount = 9
	high.LastInteractionAt = recent
	high.CreatedAt = recent
	st.AddLead(high)

	// Lead with interaction_count 4-6 but warmth not explicitly "warm".
	mid := makeNetworkingLead()
	mid.HiringManager = "Mid Interaction"
	mid.Warmth = "new"
	mid.InteractionCount = 5
	mid.LastInteractionAt = recent
	mid.CreatedAt = recent
	st.AddLead(mid)

	sender := &mockSender{response: "unused"}
	svc := New(st, nil, sender, t.TempDir())

	brief, err := svc.WeeklyNetworkingBrief(context.Background())
	if err != nil {
		t.Fatalf("interaction count brief: %v", err)
	}

	if len(brief.ReadyContacts) != 1 {
		t.Errorf("ready = %d, want 1 (high interaction count)", len(brief.ReadyContacts))
	}
	if len(brief.WarmContacts) != 1 {
		t.Errorf("warm = %d, want 1 (mid interaction count)", len(brief.WarmContacts))
	}
}

func TestWeeklyNetworkingBrief_ExcludesNonNetworking(t *testing.T) {
	st := newTestStore(t)
	recent := time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339)

	// Job pipeline lead -- should NOT appear in networking brief.
	job := makeTestLead()
	job.Pipeline = "job"
	job.CreatedAt = recent
	st.AddLead(job)

	// Networking pipeline lead -- should appear.
	net := makeNetworkingLead()
	net.HiringManager = "Network Person"
	net.LastInteractionAt = recent
	net.CreatedAt = recent
	st.AddLead(net)

	sender := &mockSender{response: "unused"}
	svc := New(st, nil, sender, t.TempDir())

	brief, err := svc.WeeklyNetworkingBrief(context.Background())
	if err != nil {
		t.Fatalf("exclude non-networking: %v", err)
	}

	total := len(brief.WarmContacts) + len(brief.DormantContacts) + len(brief.ReadyContacts)
	if total != 1 {
		t.Errorf("total contacts = %d, want 1 (only networking pipeline)", total)
	}
}

func TestIsDormant(t *testing.T) {
	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)

	tests := []struct {
		name              string
		lastInteractionAt string
		createdAt         string
		want              bool
	}{
		{
			name:              "recent interaction",
			lastInteractionAt: time.Now().UTC().Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			createdAt:         time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339),
			want:              false,
		},
		{
			name:              "old interaction",
			lastInteractionAt: time.Now().UTC().Add(-45 * 24 * time.Hour).Format(time.RFC3339),
			createdAt:         time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339),
			want:              true,
		},
		{
			name:              "no interaction recent creation",
			lastInteractionAt: "",
			createdAt:         time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339),
			want:              false,
		},
		{
			name:              "no interaction old creation",
			lastInteractionAt: "",
			createdAt:         time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339),
			want:              true,
		},
		{
			name:              "empty both",
			lastInteractionAt: "",
			createdAt:         "",
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDormant(tt.lastInteractionAt, tt.createdAt, cutoff)
			if got != tt.want {
				t.Errorf("isDormant(%q, %q) = %v, want %v", tt.lastInteractionAt, tt.createdAt, got, tt.want)
			}
		})
	}
}
