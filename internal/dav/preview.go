package dav

import (
	"bytes"
	"context"
	"sort"
	"time"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/emersion/go-vcard"
)

// PreviewEntry is a compact, display-only view of one DAV item used by the
// merge preview (no bodies, no secrets).
type PreviewEntry struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
	When  string `json:"when"` // RFC3339 start for calendars, "" for contacts
}

// Summarize renders a compact preview entry from an item body. kind is "caldav"
// or "carddav".
func Summarize(kind string, data []byte) PreviewEntry {
	switch kind {
	case "carddav":
		e := PreviewEntry{UID: ContactUID(data)}
		if card, err := vcard.NewDecoder(bytes.NewReader(data)).Decode(); err == nil {
			e.Title = card.PreferredValue(vcard.FieldFormattedName)
		}
		return e
	default: // caldav
		e := PreviewEntry{UID: CalendarUID(data)}
		if cal, err := ics.Parse(bytes.NewReader(data)); err == nil {
			for _, ev := range cal.Events() {
				e.Title = ics.Text(ev, ics.FieldSummary)
				if t, err := ev.DateTimeStart(time.UTC); err == nil && !t.IsZero() {
					e.When = t.Format(time.RFC3339)
				}
				break
			}
		}
		return e
	}
}

// PreviewMerge lists both collections, fetches their bodies (within the date
// window for calendars) and returns the per-side entries plus the simulated
// result of a sync in opts.Direction:
//
//   - a-to-b: B after the sync = A's items (authoritative on UID) ∪ B-only items.
//   - b-to-a: mirror of the above.
//   - bidirectional: union by UID (A wins ties — a preview approximation).
//
// It works against the Collection interface so it is testable with fakes.
func PreviewMerge(ctx context.Context, a, b Collection, opts Options, kind string) (aOut, bOut, merged []PreviewEntry, err error) {
	aMap, aList, err := collect(ctx, a, opts, kind)
	if err != nil {
		return nil, nil, nil, err
	}
	bMap, bList, err := collect(ctx, b, opts, kind)
	if err != nil {
		return nil, nil, nil, err
	}

	var primary, secondary map[string]PreviewEntry
	switch opts.Direction {
	case BToA:
		primary, secondary = bMap, aMap
	default: // a-to-b and bidirectional: A is authoritative
		primary, secondary = aMap, bMap
	}
	mergedMap := make(map[string]PreviewEntry, len(primary)+len(secondary))
	for uid, e := range secondary {
		mergedMap[uid] = e
	}
	for uid, e := range primary {
		mergedMap[uid] = e // authoritative side overwrites
	}

	return sortEntries(aList), sortEntries(bList), sortEntries(mapValues(mergedMap)), nil
}

// collect lists a collection, fetches each item and returns both a UID-keyed map
// (for merge) and a flat slice (for display), filtering calendars by window.
func collect(ctx context.Context, coll Collection, opts Options, kind string) (map[string]PreviewEntry, []PreviewEntry, error) {
	list, err := coll.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	byUID := make(map[string]PreviewEntry, len(list))
	flat := make([]PreviewEntry, 0, len(list))
	for _, meta := range list {
		item, err := coll.Get(ctx, meta.Href)
		if err != nil {
			return nil, nil, err
		}
		if kind != "carddav" && !opts.inWindow(item.Data) {
			continue
		}
		e := Summarize(kind, item.Data)
		key := e.UID
		if key == "" {
			key = meta.Href
		}
		byUID[key] = e
		flat = append(flat, e)
	}
	return byUID, flat, nil
}

func mapValues(m map[string]PreviewEntry) []PreviewEntry {
	out := make([]PreviewEntry, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	return out
}

// sortEntries orders entries by start time then title for stable display.
func sortEntries(entries []PreviewEntry) []PreviewEntry {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].When != entries[j].When {
			return entries[i].When < entries[j].When
		}
		return entries[i].Title < entries[j].Title
	})
	return entries
}
