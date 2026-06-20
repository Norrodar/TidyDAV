// Package dav synchronises CalDAV/CardDAV collections between two servers.
//
// The sync engine works against the Collection interface so it can be tested
// with an in-memory fake; real CalDAV/CardDAV clients implement the same
// interface. Items are matched across servers by their UID (extracted from the
// body), since hrefs and ETags are server-specific.
package dav

import (
	"context"
	"time"
)

// ItemMeta is the cheap listing info for a DAV resource.
type ItemMeta struct {
	Href string
	ETag string
}

// Item is a DAV resource together with its body.
type Item struct {
	Href string
	ETag string
	Data []byte
}

// Collection is one side of a sync: a CalDAV calendar or CardDAV address book.
type Collection interface {
	List(ctx context.Context) ([]ItemMeta, error)
	Get(ctx context.Context, href string) (Item, error)
	// Put creates or replaces an item and returns the stored item (its
	// server-canonical Href and new ETag).
	Put(ctx context.Context, item Item) (Item, error)
	Delete(ctx context.Context, href string) error
}

// Direction selects which way items flow.
type Direction string

const (
	AToB          Direction = "a-to-b"
	BToA          Direction = "b-to-a"
	Bidirectional Direction = "bidirectional"
)

// Conflict selects how a bidirectional conflict is resolved.
type Conflict string

const (
	NewestWins Conflict = "newest-wins"
	SourceWins Conflict = "source-wins"
)

// ItemState records, per UID, where an item lives and its last-seen ETags on
// each side, so changes and deletions can be detected between runs.
type ItemState struct {
	UID     string `json:"uid"`
	SrcHref string `json:"srcHref"`
	SrcETag string `json:"srcETag"`
	DstHref string `json:"dstHref"`
	DstETag string `json:"dstETag"`
}

// State is the persisted sync state for one job, keyed by UID.
type State struct {
	Items map[string]ItemState `json:"items"`
}

// NewState returns an empty state.
func NewState() *State { return &State{Items: map[string]ItemState{}} }

// Options configures a sync run.
type Options struct {
	Direction Direction
	Conflict  Conflict
	// UID extracts the stable cross-server identity from an item body.
	UID func([]byte) string
	// Modified extracts the last-modified time from an item body, used by the
	// newest-wins conflict policy. When nil, conflicts fall back to source-wins.
	Modified func([]byte) time.Time
}

// Result counts what a sync run changed on the destination side(s).
type Result struct {
	Created int
	Updated int
	Deleted int
}
