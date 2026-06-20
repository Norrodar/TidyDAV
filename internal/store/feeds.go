package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// FeedSource is one upstream ICS URL with optional HTTP Basic Auth.
type FeedSource struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Feed is an output feed definition served at /ics/<secret>.
type Feed struct {
	ID            string
	UserID        string
	Name          string
	Secret        string
	Sources       []FeedSource
	Rules         json.RawMessage // JSON array of pipeline rule configs
	TTLSeconds    int
	BasicAuthUser string
	BasicAuthHash string          // bcrypt; empty means the endpoint is not basic-auth protected
	Notifications json.RawMessage // notify.FeedNotifications as JSON
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const feedColumns = "id, user_id, name, secret, sources, rules, ttl_seconds, " +
	"basic_auth_user, basic_auth_hash, notifications, created_at, updated_at"

// CreateFeed inserts a new feed.
func (s *Store) CreateFeed(ctx context.Context, f *Feed) error {
	now := time.Now().UTC()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	f.UpdatedAt = now

	sources, err := marshalSources(f.Sources)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO feeds (`+feedColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.UserID, f.Name, f.Secret, sources, rulesOrEmpty(f.Rules), f.TTLSeconds,
		f.BasicAuthUser, f.BasicAuthHash, notifOrEmpty(f.Notifications),
		f.CreatedAt.Format(time.RFC3339), f.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create feed: %w", err)
	}
	return nil
}

// UpdateFeed updates a feed owned by f.UserID. Returns ErrNotFound if no such feed.
func (s *Store) UpdateFeed(ctx context.Context, f *Feed) error {
	f.UpdatedAt = time.Now().UTC()
	sources, err := marshalSources(f.Sources)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE feeds SET name = ?, secret = ?, sources = ?, rules = ?, ttl_seconds = ?,
		     basic_auth_user = ?, basic_auth_hash = ?, notifications = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		f.Name, f.Secret, sources, rulesOrEmpty(f.Rules), f.TTLSeconds,
		f.BasicAuthUser, f.BasicAuthHash, notifOrEmpty(f.Notifications), f.UpdatedAt.Format(time.RFC3339),
		f.ID, f.UserID,
	)
	if err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	return checkAffected(res)
}

// DeleteFeed removes a feed owned by userID. Returns ErrNotFound if no such feed.
func (s *Store) DeleteFeed(ctx context.Context, id, userID string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM feeds WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete feed: %w", err)
	}
	return checkAffected(res)
}

// FeedByID returns a feed by id, or ErrNotFound.
func (s *Store) FeedByID(ctx context.Context, id string) (*Feed, error) {
	return s.feedBy(ctx, "id", id)
}

// FeedBySecret returns a feed by its access secret, or ErrNotFound.
func (s *Store) FeedBySecret(ctx context.Context, secret string) (*Feed, error) {
	return s.feedBy(ctx, "secret", secret)
}

func (s *Store) feedBy(ctx context.Context, column, value string) (*Feed, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+feedColumns+" FROM feeds WHERE "+column+" = ?", value)
	f, err := scanFeed(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query feed by %s: %w", column, err)
	}
	return f, nil
}

// FeedsByUser lists a user's feeds, oldest first.
func (s *Store) FeedsByUser(ctx context.Context, userID string) ([]*Feed, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+feedColumns+" FROM feeds WHERE user_id = ? ORDER BY created_at", userID)
	if err != nil {
		return nil, fmt.Errorf("query feeds: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanFeeds(rows)
}

// AllFeeds returns every feed across all users, oldest first.
func (s *Store) AllFeeds(ctx context.Context) ([]*Feed, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+feedColumns+" FROM feeds ORDER BY created_at")
	if err != nil {
		return nil, fmt.Errorf("query feeds: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanFeeds(rows)
}

func scanFeeds(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]*Feed, error) {
	var feeds []*Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, f)
	}
	return feeds, rows.Err()
}

func scanFeed(sc rowScanner) (*Feed, error) {
	var (
		f                     Feed
		sources, rules, notif string
		created, updated      string
	)
	if err := sc.Scan(&f.ID, &f.UserID, &f.Name, &f.Secret, &sources, &rules, &f.TTLSeconds,
		&f.BasicAuthUser, &f.BasicAuthHash, &notif, &created, &updated); err != nil {
		return nil, err
	}
	if sources != "" {
		if err := json.Unmarshal([]byte(sources), &f.Sources); err != nil {
			return nil, fmt.Errorf("decode feed sources: %w", err)
		}
	}
	f.Rules = json.RawMessage(rules)
	f.Notifications = json.RawMessage(notif)
	f.CreatedAt = parseTime(created)
	f.UpdatedAt = parseTime(updated)
	return &f, nil
}

func marshalSources(sources []FeedSource) (string, error) {
	if len(sources) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(sources)
	if err != nil {
		return "", fmt.Errorf("encode feed sources: %w", err)
	}
	return string(b), nil
}

func rulesOrEmpty(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "[]"
	}
	return string(raw)
}

func notifOrEmpty(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

func checkAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
