package ai

import (
	"context"
	"fmt"
	"time"
)

// NetworkingBrief is the structured output from a weekly networking review.
type NetworkingBrief struct {
	WarmContacts    []BriefContact `json:"warm_contacts"`
	DormantContacts []BriefContact `json:"dormant_contacts"`
	ReadyContacts   []BriefContact `json:"ready_contacts"`
	Summary         string         `json:"summary"`
}

// BriefContact represents a single contact in a networking brief.
type BriefContact struct {
	LeadID  int64  `json:"lead_id"`
	Name    string `json:"name"`
	Company string `json:"company"`
	Warmth  string `json:"warmth"`
	Action  string `json:"suggested_action"`
}

// channelPrompts maps networking channels to their system prompts.
var channelPrompts = map[string]string{
	"linkedin": "You are a LinkedIn networking expert. Write a professional but warm connection message or follow-up. Reference something specific about the person's work. Never mention job seeking directly. Keep under 300 characters for connection requests, under 150 words for follow-ups.",
	"x":        "You are writing an X/Twitter reply. Be casual, technical, insightful. Add value to the conversation. Under 280 characters.",
	"email":    "You are writing a cold outreach email. Lead with a specific observation about their company. Offer value first, no pitch. Under 100 words.",
}

// NetworkingDraft generates a channel-appropriate networking message for a lead.
func (s *Service) NetworkingDraft(ctx context.Context, leadID int64, channel string, activityContext string) (string, error) {
	lead, err := s.store.GetLead(leadID)
	if err != nil {
		return "", fmt.Errorf("get lead: %w", err)
	}

	system, ok := channelPrompts[channel]
	if !ok {
		system = channelPrompts["linkedin"]
	}

	userMsg := fmt.Sprintf("Contact: %s at %s. Role: %s. Intent: %s. Warmth: %s. Activity: %s. Channel: %s",
		lead.HiringManager, lead.Company, lead.ContactType, lead.Intent, lead.Warmth, activityContext, channel)

	return s.sendAndExtractText(ctx, system, userMsg)
}

// WeeklyNetworkingBrief aggregates networking contacts by warmth level.
// This is data aggregation only -- no Claude API call.
func (s *Service) WeeklyNetworkingBrief(ctx context.Context) (*NetworkingBrief, error) {
	leads, err := s.store.ListLeads("networking", false)
	if err != nil {
		return nil, fmt.Errorf("list networking leads: %w", err)
	}

	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)

	brief := &NetworkingBrief{}

	for _, lead := range leads {
		bc := BriefContact{
			LeadID:  lead.ID,
			Name:    lead.HiringManager,
			Company: lead.Company,
			Warmth:  lead.Warmth,
		}

		// Check if dormant: last interaction > 30 days ago, or no interaction and created > 30 days ago.
		if isDormant(lead.LastInteractionAt, lead.CreatedAt, cutoff) {
			bc.Action = "Re-engage with a value-add message"
			brief.DormantContacts = append(brief.DormantContacts, bc)
			continue
		}

		switch {
		case lead.Warmth == "ready" || lead.InteractionCount >= 7:
			bc.Action = "Ask window is open -- propose coffee chat or collaboration"
			brief.ReadyContacts = append(brief.ReadyContacts, bc)
		case lead.Warmth == "warm" || (lead.InteractionCount >= 4 && lead.InteractionCount <= 6):
			bc.Action = "Continue engaging -- coffee chat candidate"
			brief.WarmContacts = append(brief.WarmContacts, bc)
		default:
			// New or cold contacts with recent activity -- treat as warm-in-progress.
			bc.Action = "Build rapport with consistent engagement"
			brief.WarmContacts = append(brief.WarmContacts, bc)
		}
	}

	brief.Summary = fmt.Sprintf("%d contacts warm (coffee chat candidates), %d dormant (re-engage), %d ready (ask window open)",
		len(brief.WarmContacts), len(brief.DormantContacts), len(brief.ReadyContacts))

	return brief, nil
}

// isDormant returns true if the contact hasn't been interacted with recently.
func isDormant(lastInteractionAt string, createdAt string, cutoff time.Time) bool {
	if lastInteractionAt != "" {
		t, err := time.Parse(time.RFC3339, lastInteractionAt)
		if err == nil {
			return t.Before(cutoff)
		}
	}
	// No last interaction -- check if created more than 30 days ago.
	if createdAt != "" {
		t, err := time.Parse(time.RFC3339, createdAt)
		if err == nil {
			return t.Before(cutoff)
		}
	}
	return false
}
