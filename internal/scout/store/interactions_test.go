package store

import (
	"testing"
)

func TestAddInteraction_GetInteractions(t *testing.T) {
	s := newTestStore(t)
	leadID, err := s.AddLead(Lead{JobTitle: "Engineer", Pipeline: "job"})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Insert interactions.
	id1, err := s.AddInteraction(leadID, "email", "gmail", "sent intro email")
	if err != nil {
		t.Fatalf("AddInteraction 1: %v", err)
	}
	if id1 == 0 {
		t.Error("expected non-zero ID")
	}

	id2, err := s.AddInteraction(leadID, "call", "phone", "follow-up call")
	if err != nil {
		t.Fatalf("AddInteraction 2: %v", err)
	}

	id3, err := s.AddInteraction(leadID, "message", "linkedin", "connected on LinkedIn")
	if err != nil {
		t.Fatalf("AddInteraction 3: %v", err)
	}

	if id1 == id2 || id2 == id3 {
		t.Error("expected unique IDs")
	}

	// List all.
	interactions, err := s.GetInteractions(leadID)
	if err != nil {
		t.Fatalf("GetInteractions: %v", err)
	}
	if len(interactions) != 3 {
		t.Fatalf("len = %d, want 3", len(interactions))
	}

	// Newest first (by id DESC).
	if interactions[0].Type != "message" {
		t.Errorf("first type = %q, want %q", interactions[0].Type, "message")
	}
	if interactions[0].Channel != "linkedin" {
		t.Errorf("first channel = %q, want %q", interactions[0].Channel, "linkedin")
	}
	if interactions[0].Description != "connected on LinkedIn" {
		t.Errorf("first description = %q, want %q", interactions[0].Description, "connected on LinkedIn")
	}
	if interactions[0].LeadID != leadID {
		t.Errorf("LeadID = %d, want %d", interactions[0].LeadID, leadID)
	}

	// Empty list for nonexistent lead.
	empty, err := s.GetInteractions(999)
	if err != nil {
		t.Fatalf("GetInteractions(999): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("len = %d, want 0", len(empty))
	}
}

func TestGetInteractionCount(t *testing.T) {
	s := newTestStore(t)
	leadID, _ := s.AddLead(Lead{JobTitle: "Engineer", Pipeline: "job"})

	// Zero initially.
	count, err := s.GetInteractionCount(leadID)
	if err != nil {
		t.Fatalf("GetInteractionCount: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Add 3 interactions.
	s.AddInteraction(leadID, "email", "gmail", "intro")
	s.AddInteraction(leadID, "call", "phone", "call")
	s.AddInteraction(leadID, "message", "slack", "message")

	count, err = s.GetInteractionCount(leadID)
	if err != nil {
		t.Fatalf("GetInteractionCount: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	// Different lead has zero.
	leadID2, _ := s.AddLead(Lead{JobTitle: "Designer", Pipeline: "freelance"})
	count2, err := s.GetInteractionCount(leadID2)
	if err != nil {
		t.Fatalf("GetInteractionCount(lead2): %v", err)
	}
	if count2 != 0 {
		t.Errorf("count = %d, want 0", count2)
	}
}
