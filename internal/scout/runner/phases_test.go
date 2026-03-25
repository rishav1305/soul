package runner

import (
	"testing"
	"time"

	"github.com/rishav1305/soul/internal/scout/store"
)

// --- Networking Phases ---

func TestNetworkingEngagePhase(t *testing.T) {
	s := newTestStore(t)

	// Add a networking lead at "connected".
	id, err := s.AddLead(store.Lead{
		JobTitle: "CTO at StartupCo",
		Company:  "StartupCo",
		Pipeline: "networking",
		Stage:    "connected",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Add 3 interactions (threshold).
	for i := 0; i < 3; i++ {
		if _, err := s.AddInteraction(id, "message", "linkedin", "conversation"); err != nil {
			t.Fatalf("AddInteraction: %v", err)
		}
	}

	n, err := NetworkingEngagePhase(s)
	if err != nil {
		t.Fatalf("NetworkingEngagePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "engaging" {
		t.Errorf("Stage = %q, want engaging", lead.Stage)
	}
}

func TestNetworkingEngagePhase_InsufficientInteractions(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle: "VP Eng",
		Company:  "BigCo",
		Pipeline: "networking",
		Stage:    "connected",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Only 2 interactions (below threshold of 3).
	for i := 0; i < 2; i++ {
		if _, err := s.AddInteraction(id, "message", "email", "hello"); err != nil {
			t.Fatalf("AddInteraction: %v", err)
		}
	}

	n, err := NetworkingEngagePhase(s)
	if err != nil {
		t.Fatalf("NetworkingEngagePhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "connected" {
		t.Errorf("Stage = %q, want connected", lead.Stage)
	}
}

func TestNetworkingWarmPhase(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle: "Engineering Manager",
		Company:  "TechCo",
		Pipeline: "networking",
		Stage:    "engaging",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Add 5 interactions (threshold).
	for i := 0; i < 5; i++ {
		if _, err := s.AddInteraction(id, "call", "phone", "discussion"); err != nil {
			t.Fatalf("AddInteraction: %v", err)
		}
	}

	n, err := NetworkingWarmPhase(s)
	if err != nil {
		t.Fatalf("NetworkingWarmPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "warm" {
		t.Errorf("Stage = %q, want warm", lead.Stage)
	}
}

func TestNetworkingStalePhase(t *testing.T) {
	s := newTestStore(t)

	staleTime := time.Now().UTC().Add(-35 * 24 * time.Hour).Format(time.RFC3339)
	_, err := s.AddLead(store.Lead{
		JobTitle:          "Director of Eng",
		Company:           "OldCo",
		Pipeline:          "networking",
		Stage:             "connected",
		LastInteractionAt: staleTime,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := NetworkingStalePhase(s)
	if err != nil {
		t.Fatalf("NetworkingStalePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}
}

func TestNetworkingStalePhase_RecentInteraction(t *testing.T) {
	s := newTestStore(t)

	recentTime := time.Now().UTC().Add(-5 * 24 * time.Hour).Format(time.RFC3339)
	_, err := s.AddLead(store.Lead{
		JobTitle:          "Staff Engineer",
		Company:           "NewCo",
		Pipeline:          "networking",
		Stage:             "connected",
		LastInteractionAt: recentTime,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := NetworkingStalePhase(s)
	if err != nil {
		t.Fatalf("NetworkingStalePhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (recent interaction)", n)
	}
}

// --- Freelance Phases ---

func TestFreelanceQualifyPhase(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle:   "React Native Developer",
		Company:    "MobileApp Inc",
		Pipeline:   "freelance",
		Stage:      "found",
		MatchScore: 85,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := FreelanceQualifyPhase(s)
	if err != nil {
		t.Fatalf("FreelanceQualifyPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "proposal-ready" {
		t.Errorf("Stage = %q, want proposal-ready", lead.Stage)
	}
}

func TestFreelanceQualifyPhase_LowScore(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle:   "Data Entry Clerk",
		Company:    "BoringCo",
		Pipeline:   "freelance",
		Stage:      "found",
		MatchScore: 40,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := FreelanceQualifyPhase(s)
	if err != nil {
		t.Fatalf("FreelanceQualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (low score)", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "found" {
		t.Errorf("Stage = %q, want found", lead.Stage)
	}
}

func TestFreelanceStalePhase(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddLead(store.Lead{
		JobTitle: "Go Backend Developer",
		Company:  "FreelanceCo",
		Pipeline: "freelance",
		Stage:    "proposal-ready",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Force updated_at to 10 days ago.
	staleTime := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.DB().Exec("UPDATE leads SET updated_at = ? WHERE pipeline = 'freelance'", staleTime); err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	n, err := FreelanceStalePhase(s)
	if err != nil {
		t.Fatalf("FreelanceStalePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}
}

// --- Contract Phases ---

func TestContractQualifyPhase(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle:   "Go Contractor",
		Company:    "ContractCo",
		Pipeline:   "contract",
		Stage:      "discovered",
		MatchScore: 80,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := ContractQualifyPhase(s)
	if err != nil {
		t.Fatalf("ContractQualifyPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "applied" {
		t.Errorf("Stage = %q, want applied", lead.Stage)
	}

	// Verify stage history.
	history, err := s.GetStageHistory(id)
	if err != nil {
		t.Fatalf("GetStageHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].FromStage != "discovered" || history[0].ToStage != "applied" {
		t.Errorf("history = %s -> %s, want discovered -> applied",
			history[0].FromStage, history[0].ToStage)
	}
}

func TestContractQualifyPhase_LowScore(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle:   "PHP Contractor",
		Company:    "LegacyCo",
		Pipeline:   "contract",
		Stage:      "discovered",
		MatchScore: 50,
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := ContractQualifyPhase(s)
	if err != nil {
		t.Fatalf("ContractQualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "discovered" {
		t.Errorf("Stage = %q, want discovered", lead.Stage)
	}
}

func TestContractEngagedPhase(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddLead(store.Lead{
		JobTitle: "ML Contractor",
		Company:  "AICo",
		Pipeline: "contract",
		Stage:    "offer",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Force updated_at to 5 days ago (> 3 day threshold).
	oldTime := time.Now().UTC().Add(-5 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.DB().Exec("UPDATE leads SET updated_at = ? WHERE pipeline = 'contract' AND stage = 'offer'", oldTime); err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	n, err := ContractEngagedPhase(s)
	if err != nil {
		t.Fatalf("ContractEngagedPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}
}

func TestContractEngagedPhase_Recent(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddLead(store.Lead{
		JobTitle: "DevOps Contractor",
		Company:  "CloudCo",
		Pipeline: "contract",
		Stage:    "offer",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Lead was just added — updated_at is now, so no follow-up needed.
	n, err := ContractEngagedPhase(s)
	if err != nil {
		t.Fatalf("ContractEngagedPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (recent lead)", n)
	}
}

// --- Consulting Phases ---

func TestConsultingQualifyPhase(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle: "AI Strategy Consultant",
		Company:  "ConsultCo",
		Pipeline: "consulting",
		Stage:    "lead",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Add an artifact to qualify.
	if _, err := s.AddArtifact(id, "call_prep", "Prepared notes for discovery call"); err != nil {
		t.Fatalf("AddArtifact: %v", err)
	}

	n, err := ConsultingQualifyPhase(s)
	if err != nil {
		t.Fatalf("ConsultingQualifyPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "discovery-call" {
		t.Errorf("Stage = %q, want discovery-call", lead.Stage)
	}
}

func TestConsultingQualifyPhase_NoArtifacts(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle: "ML Advisor",
		Company:  "SmallCo",
		Pipeline: "consulting",
		Stage:    "lead",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	n, err := ConsultingQualifyPhase(s)
	if err != nil {
		t.Fatalf("ConsultingQualifyPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (no artifacts)", n)
	}

	lead, err := s.GetLead(id)
	if err != nil {
		t.Fatalf("GetLead: %v", err)
	}
	if lead.Stage != "lead" {
		t.Errorf("Stage = %q, want lead", lead.Stage)
	}
}

func TestConsultingStalePhase(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddLead(store.Lead{
		JobTitle: "Data Consultant",
		Company:  "OldConsult",
		Pipeline: "consulting",
		Stage:    "proposal-sent",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Force updated_at to 20 days ago.
	staleTime := time.Now().UTC().Add(-20 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.DB().Exec("UPDATE leads SET updated_at = ? WHERE pipeline = 'consulting'", staleTime); err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	n, err := ConsultingStalePhase(s)
	if err != nil {
		t.Fatalf("ConsultingStalePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}
}

func TestConsultingStalePhase_TerminalExcluded(t *testing.T) {
	s := newTestStore(t)

	_, err := s.AddLead(store.Lead{
		JobTitle: "Past Consultant",
		Company:  "DoneCo",
		Pipeline: "consulting",
		Stage:    "delivered",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Force old updated_at — terminal stages should still be excluded.
	staleTime := time.Now().UTC().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.DB().Exec("UPDATE leads SET updated_at = ? WHERE pipeline = 'consulting'", staleTime); err != nil {
		t.Fatalf("set updated_at: %v", err)
	}

	n, err := ConsultingStalePhase(s)
	if err != nil {
		t.Fatalf("ConsultingStalePhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (terminal stage excluded)", n)
	}
}

// --- Content Phases ---

func TestContentPublishPhase(t *testing.T) {
	s := newTestStore(t)

	// Add a scheduled post with a past date.
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	_, err := s.AddContentPost(store.ContentPost{
		Platform:      "linkedin",
		Pillar:        "builder_insights",
		Topic:         "Go concurrency patterns",
		Status:        "scheduled",
		ScheduledDate: yesterday,
	})
	if err != nil {
		t.Fatalf("AddContentPost: %v", err)
	}

	n, err := ContentPublishPhase(s)
	if err != nil {
		t.Fatalf("ContentPublishPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}
}

func TestContentPublishPhase_FutureDate(t *testing.T) {
	s := newTestStore(t)

	// Add a scheduled post with a future date.
	tomorrow := time.Now().UTC().Add(48 * time.Hour).Format("2006-01-02")
	_, err := s.AddContentPost(store.ContentPost{
		Platform:      "x",
		Pillar:        "technical_takes",
		Topic:         "Future post",
		Status:        "scheduled",
		ScheduledDate: tomorrow,
	})
	if err != nil {
		t.Fatalf("AddContentPost: %v", err)
	}

	n, err := ContentPublishPhase(s)
	if err != nil {
		t.Fatalf("ContentPublishPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (future date)", n)
	}
}

func TestContentStalePhase(t *testing.T) {
	s := newTestStore(t)

	// Add a draft post with created_at 20 days ago.
	oldTime := time.Now().UTC().Add(-20 * 24 * time.Hour).Format(time.RFC3339)
	_, err := s.AddContentPost(store.ContentPost{
		Platform:  "linkedin",
		Pillar:    "builder_insights",
		Topic:     "Old draft",
		Status:    "draft",
		CreatedAt: oldTime,
	})
	if err != nil {
		t.Fatalf("AddContentPost: %v", err)
	}

	n, err := ContentStalePhase(s)
	if err != nil {
		t.Fatalf("ContentStalePhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1", n)
	}
}

func TestContentStalePhase_RecentDraft(t *testing.T) {
	s := newTestStore(t)

	// Add a draft post just created (defaults to now).
	_, err := s.AddContentPost(store.ContentPost{
		Platform: "x",
		Pillar:   "technical_takes",
		Topic:    "Fresh draft",
		Status:   "draft",
	})
	if err != nil {
		t.Fatalf("AddContentPost: %v", err)
	}

	n, err := ContentStalePhase(s)
	if err != nil {
		t.Fatalf("ContentStalePhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (recent draft)", n)
	}
}

// --- Profile Phase ---

func TestProfileAuditPhase_NoAudit(t *testing.T) {
	s := newTestStore(t)

	// No profile_audit artifact exists — should recommend audit.
	n, err := ProfileAuditPhase(s)
	if err != nil {
		t.Fatalf("ProfileAuditPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1 (no previous audit)", n)
	}
}

func TestProfileAuditPhase_RecentAudit(t *testing.T) {
	s := newTestStore(t)

	// Need a lead to attach the artifact to.
	id, err := s.AddLead(store.Lead{
		JobTitle: "Profile Lead",
		Pipeline: "networking",
		Stage:    "identified",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Add a recent profile_audit artifact.
	if _, err := s.AddArtifact(id, "profile_audit", "All sections up to date"); err != nil {
		t.Fatalf("AddArtifact: %v", err)
	}

	n, err := ProfileAuditPhase(s)
	if err != nil {
		t.Fatalf("ProfileAuditPhase: %v", err)
	}
	if n != 0 {
		t.Errorf("processed = %d, want 0 (recent audit)", n)
	}
}

func TestProfileAuditPhase_OldAudit(t *testing.T) {
	s := newTestStore(t)

	id, err := s.AddLead(store.Lead{
		JobTitle: "Profile Lead",
		Pipeline: "networking",
		Stage:    "identified",
	})
	if err != nil {
		t.Fatalf("AddLead: %v", err)
	}

	// Add an old profile_audit artifact.
	if _, err := s.AddArtifact(id, "profile_audit", "Old audit"); err != nil {
		t.Fatalf("AddArtifact: %v", err)
	}

	// Force created_at to 100 days ago.
	oldTime := time.Now().UTC().Add(-100 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.DB().Exec("UPDATE lead_artifacts SET created_at = ? WHERE type = 'profile_audit'", oldTime); err != nil {
		t.Fatalf("set created_at: %v", err)
	}

	n, err := ProfileAuditPhase(s)
	if err != nil {
		t.Fatalf("ProfileAuditPhase: %v", err)
	}
	if n != 1 {
		t.Errorf("processed = %d, want 1 (old audit)", n)
	}
}
