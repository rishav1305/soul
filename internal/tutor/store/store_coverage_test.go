package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openCovStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cov.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Not-found / ErrNoRows paths ---

func TestGetTopic_NotFound(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetTopic(99999)
	if err == nil {
		t.Error("expected error for nonexistent topic")
	}
}

func TestGetTopicByName_NotFound(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetTopicByName("dsa", "sorting", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent topic")
	}
}

func TestUpdateTopicStatus_NotFound(t *testing.T) {
	s := openCovStore(t)
	err := s.UpdateTopicStatus(99999, "mastered")
	if err == nil {
		t.Error("expected error for nonexistent topic")
	}
}

func TestGetQuizQuestion_NotFound(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetQuizQuestion(99999)
	if err == nil {
		t.Error("expected error for nonexistent question")
	}
}

func TestPickNextQuestion_NoQuestions(t *testing.T) {
	s := openCovStore(t)
	topic, _ := s.CreateTopic("dsa", "sorting", "empty-topic", "easy", "")
	_, err := s.PickNextQuestion(topic.ID)
	if err == nil {
		t.Error("expected error when no questions exist")
	}
}

func TestGetSpacedRep_NotFound(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetSpacedRep(99999)
	if err == nil {
		t.Error("expected error for nonexistent spaced rep")
	}
}

func TestGetActivity_NotFound(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetActivity("2025-01-01", "dsa")
	if err == nil {
		t.Error("expected error for nonexistent activity")
	}
}

func TestGetMockSession_NotFound(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetMockSession(99999)
	if err == nil {
		t.Error("expected error for nonexistent mock session")
	}
}

func TestCompleteMockSession_NotFound(t *testing.T) {
	s := openCovStore(t)
	err := s.CompleteMockSession(99999, 85.0, `{"summary":"good"}`)
	if err == nil {
		t.Error("expected error for nonexistent mock session")
	}
}


func TestGetActivePlan_NoPlan(t *testing.T) {
	s := openCovStore(t)
	_, err := s.GetActivePlan()
	if err == nil {
		t.Error("expected error when no plan exists")
	}
}

// --- CRUD roundtrips for less-tested entities ---

func TestUpsertSpacedRep_CreateAndUpdate(t *testing.T) {
	s := openCovStore(t)
	topic, _ := s.CreateTopic("dsa", "sorting", "sr-test", "medium", "")

	next := time.Now().UTC().Add(24 * time.Hour)
	sr, err := s.UpsertSpacedRep(topic.ID, next, 1, 2.5, 1)
	if err != nil {
		t.Fatalf("UpsertSpacedRep (create): %v", err)
	}
	if sr.TopicID != topic.ID {
		t.Errorf("TopicID = %d, want %d", sr.TopicID, topic.ID)
	}

	// Update
	next2 := time.Now().UTC().Add(48 * time.Hour)
	sr2, err := s.UpsertSpacedRep(topic.ID, next2, 2, 2.6, 2)
	if err != nil {
		t.Fatalf("UpsertSpacedRep (update): %v", err)
	}
	if sr2.IntervalDays != 2 {
		t.Errorf("IntervalDays = %d, want 2", sr2.IntervalDays)
	}
}

func TestGetDueReviews_Empty(t *testing.T) {
	s := openCovStore(t)
	reviews, err := s.GetDueReviews(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 0 {
		t.Errorf("expected 0 due reviews, got %d", len(reviews))
	}
}

func TestGetDueReviews_ReturnsDue(t *testing.T) {
	s := openCovStore(t)
	topic, _ := s.CreateTopic("dsa", "sorting", "due-test", "easy", "")

	// Set next review to the past.
	past := time.Now().UTC().Add(-1 * time.Hour)
	s.UpsertSpacedRep(topic.ID, past, 1, 2.5, 1)

	reviews, err := s.GetDueReviews(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 {
		t.Errorf("expected 1 due review, got %d", len(reviews))
	}
}

func TestUpsertDailyActivity_CreateAndUpdate(t *testing.T) {
	s := openCovStore(t)
	date := "2025-01-15"

	act, err := s.UpsertDailyActivity(date, "dsa", 300, 1, 5, 80.0)
	if err != nil {
		t.Fatalf("UpsertDailyActivity (create): %v", err)
	}
	if act.Date != date {
		t.Errorf("Date = %q, want %q", act.Date, date)
	}

	// Update same date/module.
	act2, err := s.UpsertDailyActivity(date, "dsa", 600, 2, 10, 85.0)
	if err != nil {
		t.Fatalf("UpsertDailyActivity (update): %v", err)
	}
	if act2.TimeSpentSeconds != 900 {
		t.Errorf("TimeSpentSeconds = %d, want 900 (300+600)", act2.TimeSpentSeconds)
	}
}

func TestGetTodayActivity_EmptyDB(t *testing.T) {
	s := openCovStore(t)
	activities, err := s.GetTodayActivity()
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 0 {
		t.Errorf("expected 0, got %d", len(activities))
	}
}

func TestGetStreak_NoActivity(t *testing.T) {
	s := openCovStore(t)
	streak, err := s.GetStreak()
	if err != nil {
		t.Fatal(err)
	}
	if streak != 0 {
		t.Errorf("expected 0 streak, got %d", streak)
	}
}

func TestCreateMockSession_Roundtrip(t *testing.T) {
	s := openCovStore(t)
	ms, err := s.CreateMockSession("technical", "Build a REST API")
	if err != nil {
		t.Fatal(err)
	}
	if ms.Type != "technical" {
		t.Errorf("SessionType = %q", ms.Type)
	}

	got, err := s.GetMockSession(ms.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != ms.ID {
		t.Errorf("ID mismatch")
	}
}

func TestListMockSessions_FilterByType(t *testing.T) {
	s := openCovStore(t)
	s.CreateMockSession("technical", "desc")
	s.CreateMockSession("behavioral", "desc")
	s.CreateMockSession("technical", "desc2")

	tech, err := s.ListMockSessions("technical")
	if err != nil {
		t.Fatal(err)
	}
	if len(tech) != 2 {
		t.Errorf("expected 2 technical sessions, got %d", len(tech))
	}

	all, err := s.ListMockSessions("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 total sessions, got %d", len(all))
	}
}

func TestCompleteMockSession_Success(t *testing.T) {
	s := openCovStore(t)
	ms, _ := s.CreateMockSession("behavioral", "desc")

	err := s.CompleteMockSession(ms.ID, 90.0, `{"feedback":"great"}`)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := s.GetMockSession(ms.ID)
	if got.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestAddMockScore_Roundtrip(t *testing.T) {
	s := openCovStore(t)
	ms, _ := s.CreateMockSession("technical", "desc")

	err := s.AddMockScore(ms.ID, "communication", 85.0)
	if err != nil {
		t.Fatal(err)
	}

	scores, err := s.GetMockScores(ms.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}
	if scores[0].Dimension != "communication" {
		t.Errorf("Dimension = %q", scores[0].Dimension)
	}
}

func TestUpsertStarStory_CreateAndGet(t *testing.T) {
	s := openCovStore(t)
	star, err := s.UpsertStarStory("leadership", "Led a team", "Deliver feature", "Organized sprints", "Shipped on time", "project-a")
	if err != nil {
		t.Fatal(err)
	}
	if star.Competency != "leadership" {
		t.Errorf("Competency = %q", star.Competency)
	}

	got, err := s.GetStarStory("leadership")
	if err != nil {
		t.Fatal(err)
	}
	if got.Situation != "Led a team" {
		t.Errorf("Situation = %q", got.Situation)
	}
}

func TestCreatePlan_Roundtrip(t *testing.T) {
	s := openCovStore(t)
	plan, err := s.CreatePlan("SDE-2", "2025-06-01", `{"weeks":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetRole != "SDE-2" {
		t.Errorf("TargetRole = %q", plan.TargetRole)
	}

	active, err := s.GetActivePlan()
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != plan.ID {
		t.Error("GetActivePlan returned different plan")
	}
}

func TestUpdatePlan(t *testing.T) {
	s := openCovStore(t)
	plan, _ := s.CreatePlan("SDE-2", "2025-06-01", `{"weeks":[]}`)

	err := s.UpdatePlan(plan.ID, `{"weeks":["week1"]}`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdatePlan_NotFound(t *testing.T) {
	s := openCovStore(t)
	err := s.UpdatePlan(99999, `{}`)
	if err == nil {
		t.Error("expected error for nonexistent plan")
	}
}

func TestAddConfidenceRating(t *testing.T) {
	s := openCovStore(t)
	topic, _ := s.CreateTopic("dsa", "sorting", "confidence-test", "easy", "")

	err := s.AddConfidenceRating(topic.ID, 90.0, 75.0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetConfidenceGaps(t *testing.T) {
	s := openCovStore(t)
	topic, _ := s.CreateTopic("dsa", "sorting", "gap-test", "easy", "")

	// Create a gap: self-rated 90, actual 60 — gap of 30.
	s.AddConfidenceRating(topic.ID, 90.0, 60.0)

	gaps, err := s.GetConfidenceGaps(20.0)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if gaps[0].TopicID != topic.ID {
		t.Errorf("TopicID = %d, want %d", gaps[0].TopicID, topic.ID)
	}
}

func TestGetModuleStats(t *testing.T) {
	s := openCovStore(t)
	// Create some topics.
	s.CreateTopic("dsa", "sorting", "stat-a", "easy", "")
	s.CreateTopic("dsa", "sorting", "stat-b", "medium", "")

	stats, err := s.GetModuleStats("dsa")
	if err != nil {
		t.Fatal(err)
	}
	if stats.TopicCount < 2 {
		t.Errorf("TotalTopics = %d, want >= 2", stats.TopicCount)
	}
}

// --- Closed store ---

func TestClosedStore_Operations(t *testing.T) {
	s := openCovStore(t)
	s.Close()

	_, err := s.CreateTopic("dsa", "sorting", "x", "easy", "")
	if err == nil {
		t.Error("expected error from CreateTopic on closed store")
	}
	_, err = s.GetTopic(1)
	if err == nil {
		t.Error("expected error from GetTopic on closed store")
	}
	_, err = s.GetTopicByName("dsa", "sorting", "x")
	if err == nil {
		t.Error("expected error from GetTopicByName on closed store")
	}
	_, err = s.ListTopics("", "")
	if err == nil {
		t.Error("expected error from ListTopics on closed store")
	}
	err = s.UpdateTopicStatus(1, "mastered")
	if err == nil {
		t.Error("expected error from UpdateTopicStatus on closed store")
	}
	_, err = s.GetQuizQuestion(1)
	if err == nil {
		t.Error("expected error from GetQuizQuestion on closed store")
	}
	_, err = s.ListQuestions(1)
	if err == nil {
		t.Error("expected error from ListQuestions on closed store")
	}
	_, err = s.PickNextQuestion(1)
	if err == nil {
		t.Error("expected error from PickNextQuestion on closed store")
	}
	_, err = s.GetSpacedRep(1)
	if err == nil {
		t.Error("expected error from GetSpacedRep on closed store")
	}
	_, err = s.GetDueReviews(time.Now())
	if err == nil {
		t.Error("expected error from GetDueReviews on closed store")
	}
	_, err = s.GetActivity("2025-01-01", "dsa")
	if err == nil {
		t.Error("expected error from GetActivity on closed store")
	}
	_, err = s.GetTodayActivity()
	if err == nil {
		t.Error("expected error from GetTodayActivity on closed store")
	}
	_, err = s.GetStreak()
	if err == nil {
		t.Error("expected error from GetStreak on closed store")
	}
	_, err = s.GetMockSession(1)
	if err == nil {
		t.Error("expected error from GetMockSession on closed store")
	}
	_, err = s.ListMockSessions("")
	if err == nil {
		t.Error("expected error from ListMockSessions on closed store")
	}
	_, err = s.GetStarStory("x")
	if err == nil {
		t.Error("expected error from GetStarStory on closed store")
	}
	_, err = s.GetActivePlan()
	if err == nil {
		t.Error("expected error from GetActivePlan on closed store")
	}
	_, err = s.GetMockScores(1)
	if err == nil {
		t.Error("expected error from GetMockScores on closed store")
	}
	_, err = s.GetConfidenceGaps(10.0)
	if err == nil {
		t.Error("expected error from GetConfidenceGaps on closed store")
	}
	_, err = s.GetModuleStats("dsa")
	if err == nil {
		t.Error("expected error from GetModuleStats on closed store")
	}
}

// --- Open error ---

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/deep/path/db.sqlite")
	if err == nil {
		t.Error("expected error for invalid path")
	}
	if !strings.Contains(err.Error(), "tutor") {
		t.Errorf("error should mention tutor: %q", err)
	}
}
