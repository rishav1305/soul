package store

import "fmt"

// AddOptimization inserts a new optimization record and returns its ID.
func (s *Store) AddOptimization(opt Optimization) (int64, error) {
	if opt.OptimizedAt == "" {
		opt.OptimizedAt = now()
	}
	if opt.Status == "" {
		opt.Status = "pending"
	}
	res, err := s.db.Exec(`
		INSERT INTO optimizations (platform, section, field, previous, updated, status, optimized_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		opt.Platform, opt.Section, opt.Field, opt.Previous, opt.Updated, opt.Status, opt.OptimizedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("scout: add optimization: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// ListOptimizations returns all optimization records, newest first.
func (s *Store) ListOptimizations() ([]Optimization, error) {
	rows, err := s.db.Query(
		"SELECT id, platform, section, field, previous, updated, status, optimized_at FROM optimizations ORDER BY optimized_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("scout: list optimizations: %w", err)
	}
	defer rows.Close()

	var opts []Optimization
	for rows.Next() {
		var o Optimization
		if err := rows.Scan(&o.ID, &o.Platform, &o.Section, &o.Field, &o.Previous, &o.Updated, &o.Status, &o.OptimizedAt); err != nil {
			return nil, fmt.Errorf("scout: scan optimization: %w", err)
		}
		opts = append(opts, o)
	}
	return opts, rows.Err()
}
