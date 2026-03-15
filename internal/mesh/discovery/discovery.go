package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

// DiscoveredPeer represents a peer found via network discovery.
type DiscoveredPeer struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
	IsHub bool  `json:"isHub"`
}

// tailscaleStatus is the minimal structure we need from "tailscale status --json".
type tailscaleStatus struct {
	Peer map[string]tailscalePeer `json:"Peer"`
}

type tailscalePeer struct {
	HostName     string   `json:"HostName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
}

// identityResponse is the response from /api/mesh/identity.
type identityResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	IsHub bool   `json:"isHub"`
}

// DiscoverTailscale runs "tailscale status --json", parses peers, and probes
// each online peer's /api/mesh/identity endpoint on the given port.
// Returns empty slice gracefully if tailscale is not available.
func DiscoverTailscale(meshPort int) ([]DiscoveredPeer, error) {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		// Tailscale not available — not an error, just no peers.
		return nil, nil
	}

	var status tailscaleStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("discovery: parse tailscale status: %w", err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	var peers []DiscoveredPeer

	for _, peer := range status.Peer {
		if !peer.Online || len(peer.TailscaleIPs) == 0 {
			continue
		}

		ip := peer.TailscaleIPs[0]
		// Plain HTTP is acceptable here — Tailscale provides WireGuard
		// encryption for all peer-to-peer traffic. No TLS needed on top.
		url := fmt.Sprintf("http://%s:%d/api/mesh/identity", ip, meshPort)

		resp, err := client.Get(url)
		if err != nil {
			continue
		}

		var ident identityResponse
		if err := json.NewDecoder(resp.Body).Decode(&ident); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		peers = append(peers, DiscoveredPeer{
			ID:   ident.ID,
			Host: ip,
			Port: meshPort,
			IsHub: ident.IsHub,
		})
	}

	return peers, nil
}

// DiscoverMDNS announces and browses for _soul-mesh._tcp.local. services.
// Stub implementation — returns empty slice. Full mDNS can be added later.
func DiscoverMDNS(serviceName string, timeout time.Duration) ([]DiscoveredPeer, error) {
	_ = serviceName
	_ = timeout
	return nil, nil
}
