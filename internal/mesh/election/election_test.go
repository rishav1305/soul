package election

import (
	"testing"

	"github.com/rishav1305/soul-v2/internal/mesh/node"
)

func TestElectHub_HighestWins(t *testing.T) {
	nodes := []node.NodeInfo{
		{ID: "a", Name: "alpha", RAMTotalMB: 4096, StorageTotalGB: 100},
		{ID: "b", Name: "beta", RAMTotalMB: 16384, StorageTotalGB: 500},
	}
	got := ElectHub(nodes, "")
	if got != "b" {
		t.Fatalf("expected b (highest score), got %s", got)
	}
}

func TestElectHub_Hysteresis_IncumbentStays(t *testing.T) {
	// Incumbent "a" has score 30. Challenger "b" has score 35.
	// 35 is NOT > 30*1.2=36, so incumbent stays.
	nodes := []node.NodeInfo{
		{ID: "a", Name: "alpha", RAMTotalMB: 12288, StorageTotalGB: 250},  // RAM:30 + Storage:10 = 40
		{ID: "b", Name: "beta", RAMTotalMB: 14336, StorageTotalGB: 300},   // RAM:35 + Storage:12 = 47
	}
	got := ElectHub(nodes, "a")
	if got != "a" {
		t.Fatalf("expected incumbent a to stay (hysteresis), got %s", got)
	}
}

func TestElectHub_Hysteresis_ChallengerWins(t *testing.T) {
	// Incumbent "a" has low score. Challenger "b" exceeds by >20%.
	nodes := []node.NodeInfo{
		{ID: "a", Name: "alpha", RAMTotalMB: 2048, StorageTotalGB: 50},   // RAM:5 + Storage:2 = 7
		{ID: "b", Name: "beta", RAMTotalMB: 16384, StorageTotalGB: 500},  // RAM:40 + Storage:20 = 60
	}
	got := ElectHub(nodes, "a")
	if got != "b" {
		t.Fatalf("expected challenger b to win (exceeds 20%%), got %s", got)
	}
}

func TestElectHub_TiebreakByName(t *testing.T) {
	nodes := []node.NodeInfo{
		{ID: "z", Name: "zulu", RAMTotalMB: 8192, StorageTotalGB: 250},
		{ID: "a", Name: "alpha", RAMTotalMB: 8192, StorageTotalGB: 250},
	}
	got := ElectHub(nodes, "")
	if got != "a" {
		t.Fatalf("expected a (name alpha < zulu), got %s", got)
	}
}

func TestElectHub_EmptyList(t *testing.T) {
	got := ElectHub(nil, "x")
	if got != "" {
		t.Fatalf("expected empty string for empty list, got %s", got)
	}
}

func TestElectHub_IncumbentNotInList(t *testing.T) {
	nodes := []node.NodeInfo{
		{ID: "a", Name: "alpha", RAMTotalMB: 8192, StorageTotalGB: 250},
	}
	got := ElectHub(nodes, "gone")
	if got != "a" {
		t.Fatalf("expected a when incumbent missing, got %s", got)
	}
}
