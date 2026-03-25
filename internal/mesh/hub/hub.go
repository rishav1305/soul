package hub

import (
	"encoding/json"
	"fmt"

	"github.com/rishav1305/soul/internal/mesh/store"
)

// Hub manages node registration, heartbeat processing, and resource aggregation.
type Hub struct {
	store *store.Store
}

// New creates a Hub backed by the given store.
func New(s *store.Store) *Hub {
	return &Hub{store: s}
}

// HeartbeatPayload is the JSON structure inside a heartbeat message payload.
type HeartbeatPayload struct {
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
	CPULoad1m       float64 `json:"cpuLoad1m"`
	RAMAvailableMB  int     `json:"ramAvailableMB"`
	RAMUsedPercent  float64 `json:"ramUsedPercent"`
	StorageFreeGB   int     `json:"storageFreeGB"`
}

// HandleHeartbeat parses a heartbeat payload and updates the store.
func (h *Hub) HandleHeartbeat(nodeID string, data json.RawMessage) error {
	var p HeartbeatPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("hub: parse heartbeat: %w", err)
	}

	hb := store.Heartbeat{
		NodeID:          nodeID,
		CPUUsagePercent: p.CPUUsagePercent,
		CPULoad1m:       p.CPULoad1m,
		RAMAvailableMB:  p.RAMAvailableMB,
		RAMUsedPercent:  p.RAMUsedPercent,
		StorageFreeGB:   p.StorageFreeGB,
	}

	return h.store.UpdateHeartbeat(nodeID, hb)
}

// ClusterResources is the aggregated resource summary across all nodes.
type ClusterResources struct {
	NodeCount      int `json:"nodeCount"`
	TotalCPUCores  int `json:"totalCpuCores"`
	TotalRAMMB     int `json:"totalRamMB"`
	TotalStorageGB int `json:"totalStorageGB"`
	OnlineCount    int `json:"onlineCount"`
}

// AggregateResources sums CPU cores, RAM, and storage across all registered nodes.
func (h *Hub) AggregateResources() (*ClusterResources, error) {
	nodes, err := h.store.ListNodes()
	if err != nil {
		return nil, fmt.Errorf("hub: list nodes: %w", err)
	}

	res := &ClusterResources{NodeCount: len(nodes)}
	for _, n := range nodes {
		res.TotalCPUCores += n.CPUCores
		res.TotalRAMMB += n.RAMTotalMB
		res.TotalStorageGB += n.StorageTotalGB
		if n.Status == "online" {
			res.OnlineCount++
		}
	}
	return res, nil
}
