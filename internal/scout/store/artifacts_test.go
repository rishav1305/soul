package store

import (
	"testing"
)

func TestAddArtifact_GetArtifacts(t *testing.T) {
	s := newTestStore(t)
	leadID, err := s.AddLead(Lead{JobTitle: "Engineer", Pipeline: "job"})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Insert 3 artifacts.
	ids := make([]int64, 3)
	types := []string{"cover_letter", "proposal", "resume"}
	for i, typ := range types {
		id, err := s.AddArtifact(leadID, typ, "content for "+typ)
		if err != nil {
			t.Fatalf("AddArtifact(%s): %v", typ, err)
		}
		if id == 0 {
			t.Errorf("expected non-zero ID for %s", typ)
		}
		ids[i] = id
	}

	// List all.
	artifacts, err := s.GetArtifacts(leadID)
	if err != nil {
		t.Fatalf("GetArtifacts: %v", err)
	}
	if len(artifacts) != 3 {
		t.Fatalf("len = %d, want 3", len(artifacts))
	}

	// Newest first (by id DESC) — last inserted should be first.
	if artifacts[0].Type != "resume" {
		t.Errorf("first artifact type = %q, want %q", artifacts[0].Type, "resume")
	}
	if artifacts[0].Content != "content for resume" {
		t.Errorf("first artifact content = %q, want %q", artifacts[0].Content, "content for resume")
	}
	if artifacts[0].LeadID != leadID {
		t.Errorf("LeadID = %d, want %d", artifacts[0].LeadID, leadID)
	}
}

func TestGetArtifactsByType(t *testing.T) {
	s := newTestStore(t)
	leadID, _ := s.AddLead(Lead{JobTitle: "Engineer", Pipeline: "job"})

	// Insert multiple types.
	s.AddArtifact(leadID, "cover_letter", "cover 1")
	s.AddArtifact(leadID, "proposal", "proposal 1")
	s.AddArtifact(leadID, "cover_letter", "cover 2")
	s.AddArtifact(leadID, "resume", "resume 1")

	// Filter by cover_letter.
	covers, err := s.GetArtifactsByType(leadID, "cover_letter")
	if err != nil {
		t.Fatalf("GetArtifactsByType(cover_letter): %v", err)
	}
	if len(covers) != 2 {
		t.Fatalf("len = %d, want 2", len(covers))
	}
	for _, a := range covers {
		if a.Type != "cover_letter" {
			t.Errorf("type = %q, want %q", a.Type, "cover_letter")
		}
	}

	// Filter by proposal.
	proposals, err := s.GetArtifactsByType(leadID, "proposal")
	if err != nil {
		t.Fatalf("GetArtifactsByType(proposal): %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("len = %d, want 1", len(proposals))
	}

	// Filter by nonexistent type.
	empty, err := s.GetArtifactsByType(leadID, "pitch")
	if err != nil {
		t.Fatalf("GetArtifactsByType(pitch): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("len = %d, want 0", len(empty))
	}
}

func TestGetLatestArtifact(t *testing.T) {
	s := newTestStore(t)
	leadID, _ := s.AddLead(Lead{JobTitle: "Engineer", Pipeline: "job"})

	// Insert 3 of same type.
	s.AddArtifact(leadID, "cover_letter", "version 1")
	s.AddArtifact(leadID, "cover_letter", "version 2")
	s.AddArtifact(leadID, "cover_letter", "version 3")

	latest, err := s.GetLatestArtifact(leadID, "cover_letter")
	if err != nil {
		t.Fatalf("GetLatestArtifact: %v", err)
	}
	if latest == nil {
		t.Fatal("expected non-nil artifact")
	}
	if latest.Content != "version 3" {
		t.Errorf("Content = %q, want %q", latest.Content, "version 3")
	}
	if latest.Type != "cover_letter" {
		t.Errorf("Type = %q, want %q", latest.Type, "cover_letter")
	}

	// Nonexistent type returns nil, no error.
	missing, err := s.GetLatestArtifact(leadID, "pitch")
	if err != nil {
		t.Fatalf("GetLatestArtifact(pitch): %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for nonexistent type, got %+v", missing)
	}
}
