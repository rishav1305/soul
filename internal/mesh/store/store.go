package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Node represents a mesh network node.
type Node struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Role           string `json:"role"`
	Platform       string `json:"platform"`
	Arch           string `json:"arch"`
	CPUCores       int    `json:"cpuCores"`
	RAMTotalMB     int    `json:"ramTotalMB"`
	StorageTotalGB int    `json:"storageTotalGB"`
	Status         string `json:"status"`
	LastHeartbeat  string `json:"lastHeartbeat"`
	AccountID      string `json:"accountId"`
}

// Heartbeat represents a periodic health report from a node.
type Heartbeat struct {
	ID              int64   `json:"id"`
	NodeID          string  `json:"nodeId"`
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
	CPULoad1m       float64 `json:"cpuLoad1m"`
	RAMAvailableMB  int     `json:"ramAvailableMB"`
	RAMUsedPercent  float64 `json:"ramUsedPercent"`
	StorageFreeGB   int     `json:"storageFreeGB"`
	Timestamp       string  `json:"timestamp"`
}

// Peer represents a known peer in the mesh network.
type Peer struct {
	PeerID   string `json:"peerId"`
	LastSeen string `json:"lastSeen"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	IsHub    bool   `json:"isHub"`
}

// LinkingCode represents a short-lived code for linking a node to an account.
type LinkingCode struct {
	Code      string `json:"code"`
	NodeID    string `json:"nodeId"`
	AccountID string `json:"accountId"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
}

// Store provides SQLite-backed mesh data CRUD.
type Store struct {
	db *sql.DB
}

// Open creates a new Store with the given database path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("mesh: open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("mesh: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("mesh: enable foreign keys: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("mesh: set busy timeout: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		name TEXT,
		host TEXT,
		port INTEGER,
		role TEXT,
		platform TEXT,
		arch TEXT,
		cpu_cores INTEGER,
		ram_total_mb INTEGER,
		storage_total_gb INTEGER,
		status TEXT DEFAULT 'unknown',
		last_heartbeat TEXT,
		account_id TEXT
	);

	CREATE TABLE IF NOT EXISTS heartbeats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id TEXT REFERENCES nodes(id),
		cpu_usage_percent REAL,
		cpu_load_1m REAL,
		ram_available_mb INTEGER,
		ram_used_percent REAL,
		storage_free_gb INTEGER,
		timestamp TEXT
	);

	CREATE TABLE IF NOT EXISTS peers (
		peer_id TEXT PRIMARY KEY,
		last_seen TEXT,
		host TEXT,
		port INTEGER,
		is_hub BOOLEAN DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS linking_codes (
		code TEXT PRIMARY KEY,
		node_id TEXT,
		account_id TEXT,
		created_at TEXT,
		expires_at TEXT
	);`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("mesh: migrate: %w", err)
	}
	return nil
}

// RegisterNode inserts or replaces a node record.
func (s *Store) RegisterNode(n Node) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO nodes (id, name, host, port, role, platform, arch, cpu_cores, ram_total_mb, storage_total_gb, status, last_heartbeat, account_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Host, n.Port, n.Role, n.Platform, n.Arch, n.CPUCores, n.RAMTotalMB, n.StorageTotalGB, n.Status, n.LastHeartbeat, n.AccountID,
	)
	if err != nil {
		return fmt.Errorf("mesh: register node: %w", err)
	}
	return nil
}

// GetNode retrieves a node by ID.
func (s *Store) GetNode(id string) (*Node, error) {
	var n Node
	err := s.db.QueryRow(`SELECT id, name, host, port, role, platform, arch, cpu_cores, ram_total_mb, storage_total_gb, status, COALESCE(last_heartbeat, ''), COALESCE(account_id, '') FROM nodes WHERE id = ?`, id).Scan(
		&n.ID, &n.Name, &n.Host, &n.Port, &n.Role, &n.Platform, &n.Arch, &n.CPUCores, &n.RAMTotalMB, &n.StorageTotalGB, &n.Status, &n.LastHeartbeat, &n.AccountID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mesh: get node: %w", err)
	}
	return &n, nil
}

// ListNodes returns all registered nodes.
func (s *Store) ListNodes() ([]Node, error) {
	rows, err := s.db.Query(`SELECT id, name, host, port, role, platform, arch, cpu_cores, ram_total_mb, storage_total_gb, status, COALESCE(last_heartbeat, ''), COALESCE(account_id, '') FROM nodes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("mesh: list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Host, &n.Port, &n.Role, &n.Platform, &n.Arch, &n.CPUCores, &n.RAMTotalMB, &n.StorageTotalGB, &n.Status, &n.LastHeartbeat, &n.AccountID); err != nil {
			return nil, fmt.Errorf("mesh: scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// UpdateHeartbeat inserts a heartbeat record and updates the node's last_heartbeat and status.
func (s *Store) UpdateHeartbeat(nodeID string, hb Heartbeat) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if hb.Timestamp == "" {
		hb.Timestamp = now
	}

	_, err := s.db.Exec(`
		INSERT INTO heartbeats (node_id, cpu_usage_percent, cpu_load_1m, ram_available_mb, ram_used_percent, storage_free_gb, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		nodeID, hb.CPUUsagePercent, hb.CPULoad1m, hb.RAMAvailableMB, hb.RAMUsedPercent, hb.StorageFreeGB, hb.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("mesh: insert heartbeat: %w", err)
	}

	_, err = s.db.Exec(`UPDATE nodes SET last_heartbeat = ?, status = 'online' WHERE id = ?`, hb.Timestamp, nodeID)
	if err != nil {
		return fmt.Errorf("mesh: update node heartbeat: %w", err)
	}
	return nil
}

// GetRecentHeartbeats returns the most recent heartbeats for a node.
func (s *Store) GetRecentHeartbeats(nodeID string, limit int) ([]Heartbeat, error) {
	rows, err := s.db.Query(`
		SELECT id, node_id, cpu_usage_percent, cpu_load_1m, ram_available_mb, ram_used_percent, storage_free_gb, timestamp
		FROM heartbeats WHERE node_id = ? ORDER BY timestamp DESC LIMIT ?`, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("mesh: get heartbeats: %w", err)
	}
	defer rows.Close()

	var hbs []Heartbeat
	for rows.Next() {
		var h Heartbeat
		if err := rows.Scan(&h.ID, &h.NodeID, &h.CPUUsagePercent, &h.CPULoad1m, &h.RAMAvailableMB, &h.RAMUsedPercent, &h.StorageFreeGB, &h.Timestamp); err != nil {
			return nil, fmt.Errorf("mesh: scan heartbeat: %w", err)
		}
		hbs = append(hbs, h)
	}
	return hbs, rows.Err()
}

// UpsertPeer inserts or updates a peer record.
func (s *Store) UpsertPeer(p Peer) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO peers (peer_id, last_seen, host, port, is_hub)
		VALUES (?, ?, ?, ?, ?)`,
		p.PeerID, p.LastSeen, p.Host, p.Port, p.IsHub,
	)
	if err != nil {
		return fmt.Errorf("mesh: upsert peer: %w", err)
	}
	return nil
}

// ListPeers returns all known peers.
func (s *Store) ListPeers() ([]Peer, error) {
	rows, err := s.db.Query(`SELECT peer_id, COALESCE(last_seen, ''), host, port, is_hub FROM peers ORDER BY peer_id`)
	if err != nil {
		return nil, fmt.Errorf("mesh: list peers: %w", err)
	}
	defer rows.Close()

	var peers []Peer
	for rows.Next() {
		var p Peer
		if err := rows.Scan(&p.PeerID, &p.LastSeen, &p.Host, &p.Port, &p.IsHub); err != nil {
			return nil, fmt.Errorf("mesh: scan peer: %w", err)
		}
		peers = append(peers, p)
	}
	return peers, rows.Err()
}

// CreateLinkingCode stores a linking code with an expiration time.
func (s *Store) CreateLinkingCode(code, nodeID, accountID, expiresAt string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO linking_codes (code, node_id, account_id, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		code, nodeID, accountID, now, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("mesh: create linking code: %w", err)
	}
	return nil
}

// ValidateLinkingCode checks if a code exists and is not expired. Returns the code record or nil.
func (s *Store) ValidateLinkingCode(code string) (*LinkingCode, error) {
	var lc LinkingCode
	err := s.db.QueryRow(`
		SELECT code, node_id, account_id, created_at, expires_at
		FROM linking_codes WHERE code = ?`, code).Scan(
		&lc.Code, &lc.NodeID, &lc.AccountID, &lc.CreatedAt, &lc.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mesh: validate linking code: %w", err)
	}

	expires, err := time.Parse(time.RFC3339, lc.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("mesh: parse expiry: %w", err)
	}
	if time.Now().UTC().After(expires) {
		return nil, nil
	}
	return &lc, nil
}
