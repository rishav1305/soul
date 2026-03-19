package store

import (
	"fmt"
	"strings"
)

// ContentPost represents a social media content post.
type ContentPost struct {
	ID            int64  `json:"id"`
	Platform      string `json:"platform"`
	Pillar        string `json:"pillar"`
	Topic         string `json:"topic"`
	Status        string `json:"status"`
	Content       string `json:"content"`
	Hook          string `json:"hook"`
	ScheduledDate string `json:"scheduledDate"`
	PublishedAt   string `json:"publishedAt"`
	Impressions   int    `json:"impressions"`
	Likes         int    `json:"likes"`
	Comments      int    `json:"comments"`
	Shares        int    `json:"shares"`
	Saves         int    `json:"saves"`
	ProfileViews  int    `json:"profileViews"`
	InboundLeads  int    `json:"inboundLeads"`
	PostURL       string `json:"postUrl"`
	CreatedAt     string `json:"createdAt"`
}

// BacklogItem represents a content idea in the backlog.
type BacklogItem struct {
	ID         int64  `json:"id"`
	Topic      string `json:"topic"`
	Pillar     string `json:"pillar"`
	Source     string `json:"source"`
	Angle      string `json:"angle"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	ArchivedAt string `json:"archivedAt"`
}

// allowedContentPostFields defines which fields can be updated dynamically.
// Excludes: id, created_at.
var allowedContentPostFields = map[string]bool{
	"platform":       true,
	"pillar":         true,
	"topic":          true,
	"status":         true,
	"content":        true,
	"hook":           true,
	"scheduled_date": true,
	"published_at":   true,
	"impressions":    true,
	"likes":          true,
	"comments":       true,
	"shares":         true,
	"saves":          true,
	"profile_views":  true,
	"inbound_leads":  true,
	"post_url":       true,
}

// allowedBacklogFields defines which fields can be updated dynamically.
// Excludes: id, created_at.
var allowedBacklogFields = map[string]bool{
	"topic":       true,
	"pillar":      true,
	"source":      true,
	"angle":       true,
	"status":      true,
	"archived_at": true,
}

// AddContentPost inserts a new content post and returns its ID.
func (s *Store) AddContentPost(post ContentPost) (int64, error) {
	ts := now()
	if post.CreatedAt == "" {
		post.CreatedAt = ts
	}
	if post.Status == "" {
		post.Status = "draft"
	}

	res, err := s.db.Exec(`
		INSERT INTO content_posts (
			platform, pillar, topic, status, content, hook,
			scheduled_date, published_at,
			impressions, likes, comments, shares, saves,
			profile_views, inbound_leads, post_url, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		post.Platform, post.Pillar, post.Topic, post.Status, post.Content, post.Hook,
		post.ScheduledDate, post.PublishedAt,
		post.Impressions, post.Likes, post.Comments, post.Shares, post.Saves,
		post.ProfileViews, post.InboundLeads, post.PostURL, post.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("scout: add content post: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// ListContentPosts returns content posts, optionally filtered by platform and/or status.
// Pass empty strings to skip a filter.
func (s *Store) ListContentPosts(platform string, status string) ([]ContentPost, error) {
	query := "SELECT id, platform, pillar, topic, status, content, hook, scheduled_date, published_at, impressions, likes, comments, shares, saves, profile_views, inbound_leads, post_url, created_at FROM content_posts"
	var conditions []string
	var args []interface{}

	if platform != "" {
		conditions = append(conditions, "platform = ?")
		args = append(args, platform)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("scout: list content posts: %w", err)
	}
	defer rows.Close()

	var posts []ContentPost
	for rows.Next() {
		var p ContentPost
		if err := rows.Scan(
			&p.ID, &p.Platform, &p.Pillar, &p.Topic, &p.Status,
			&p.Content, &p.Hook, &p.ScheduledDate, &p.PublishedAt,
			&p.Impressions, &p.Likes, &p.Comments, &p.Shares, &p.Saves,
			&p.ProfileViews, &p.InboundLeads, &p.PostURL, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scout: scan content post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

// UpdateContentPost modifies content post fields dynamically. Only allowed fields are applied.
func (s *Store) UpdateContentPost(id int64, fields map[string]interface{}) error {
	var setClauses []string
	var args []interface{}

	for k, v := range fields {
		if !allowedContentPostFields[k] {
			continue
		}
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	if len(setClauses) == 0 {
		return nil
	}

	args = append(args, id)

	result, err := s.db.Exec(
		"UPDATE content_posts SET "+strings.Join(setClauses, ", ")+" WHERE id = ?",
		args...,
	)
	if err != nil {
		return fmt.Errorf("scout: update content post: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("scout: content post not found: %d", id)
	}
	return nil
}

// AddBacklogItem inserts a new backlog item and returns its ID.
func (s *Store) AddBacklogItem(item BacklogItem) (int64, error) {
	ts := now()
	if item.CreatedAt == "" {
		item.CreatedAt = ts
	}
	if item.Status == "" {
		item.Status = "pending"
	}

	res, err := s.db.Exec(
		"INSERT INTO content_backlog (topic, pillar, source, angle, status, created_at, archived_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		item.Topic, item.Pillar, item.Source, item.Angle, item.Status, item.CreatedAt, item.ArchivedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("scout: add backlog item: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// ListBacklog returns backlog items, optionally filtered by status.
// Pass empty string to return all items.
func (s *Store) ListBacklog(status string) ([]BacklogItem, error) {
	query := "SELECT id, topic, pillar, source, angle, status, created_at, archived_at FROM content_backlog"
	var args []interface{}

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("scout: list backlog: %w", err)
	}
	defer rows.Close()

	var items []BacklogItem
	for rows.Next() {
		var b BacklogItem
		if err := rows.Scan(&b.ID, &b.Topic, &b.Pillar, &b.Source, &b.Angle, &b.Status, &b.CreatedAt, &b.ArchivedAt); err != nil {
			return nil, fmt.Errorf("scout: scan backlog item: %w", err)
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

// UpdateBacklogItem modifies backlog item fields dynamically. Only allowed fields are applied.
func (s *Store) UpdateBacklogItem(id int64, fields map[string]interface{}) error {
	var setClauses []string
	var args []interface{}

	for k, v := range fields {
		if !allowedBacklogFields[k] {
			continue
		}
		setClauses = append(setClauses, k+" = ?")
		args = append(args, v)
	}
	if len(setClauses) == 0 {
		return nil
	}

	args = append(args, id)

	result, err := s.db.Exec(
		"UPDATE content_backlog SET "+strings.Join(setClauses, ", ")+" WHERE id = ?",
		args...,
	)
	if err != nil {
		return fmt.Errorf("scout: update backlog item: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("scout: backlog item not found: %d", id)
	}
	return nil
}
