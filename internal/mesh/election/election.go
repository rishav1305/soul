package election

import "github.com/rishav1305/soul/internal/mesh/node"

// ElectHub selects the hub from a list of nodes.
// 20% hysteresis: incumbent keeps hub unless challenger exceeds by 20%.
// Tiebreak: capability score → name → id.
func ElectHub(nodes []node.NodeInfo, currentHubID string) string {
	if len(nodes) == 0 {
		return ""
	}

	type scored struct {
		id    string
		name  string
		score int
	}

	candidates := make([]scored, len(nodes))
	for i, n := range nodes {
		candidates[i] = scored{
			id:    n.ID,
			name:  n.Name,
			score: node.CapabilityScore(n),
		}
	}

	// Find the best candidate (highest score, tiebreak by name then id).
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		} else if c.score == best.score {
			if c.name < best.name || (c.name == best.name && c.id < best.id) {
				best = c
			}
		}
	}

	// Hysteresis: if there is an incumbent, it stays unless the best
	// challenger exceeds its score by more than 20%.
	if currentHubID == "" {
		return best.id
	}

	// If the incumbent is the best, keep it.
	if best.id == currentHubID {
		return currentHubID
	}

	// Find incumbent's score.
	incumbentScore := -1
	for _, c := range candidates {
		if c.id == currentHubID {
			incumbentScore = c.score
			break
		}
	}

	// Incumbent not in the list — elect new hub.
	if incumbentScore < 0 {
		return best.id
	}

	// Challenger must exceed incumbent by >20% to unseat.
	threshold := float64(incumbentScore) * 1.20
	if float64(best.score) > threshold {
		return best.id
	}

	return currentHubID
}
