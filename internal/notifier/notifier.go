// Package notifier evaluates feed notification triggers on a schedule and
// dispatches a notification the first time each matched event is seen.
package notifier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Norrodar/TidyDAV/internal/feed"
	"github.com/Norrodar/TidyDAV/internal/notify"
	"github.com/Norrodar/TidyDAV/internal/pipeline"
	"github.com/Norrodar/TidyDAV/internal/store"
)

// retention bounds how long the de-duplication ledger keeps entries.
const retention = 30 * 24 * time.Hour

// Rule names carried by outage notifications. They are not pipeline rules but
// share the Event.Rule field so webhook consumers can tell them apart.
const (
	ruleSourceStale     = "source_stale"
	ruleSourceRecovered = "source_recovered"
)

// Notifier dispatches feed notifications.
type Notifier struct {
	store        *store.Store
	feeds        *feed.Service
	log          *slog.Logger
	allowPrivate bool
	now          func() time.Time
}

// New creates a Notifier. allowPrivate mirrors TIDYDAV_ALLOW_PRIVATE_TARGETS
// and gates whether notification targets may resolve to non-public addresses.
func New(st *store.Store, feeds *feed.Service, log *slog.Logger, allowPrivate bool) *Notifier {
	return &Notifier{store: st, feeds: feeds, log: log, allowPrivate: allowPrivate, now: time.Now}
}

// Run evaluates every feed's notification triggers and dispatches notifications
// for newly matched events. It is meant to be called on an interval (it never
// fires on /ics polls, avoiding notification spam).
func (n *Notifier) Run(ctx context.Context) error {
	feeds, err := n.store.AllFeeds(ctx)
	if err != nil {
		return err
	}
	for _, f := range feeds {
		n.runFeed(ctx, f)
	}
	if _, err := n.store.DeleteNotifiedBefore(ctx, n.now().Add(-retention)); err != nil {
		n.log.Warn("prune notified ledger failed", "error", err)
	}
	return nil
}

func (n *Notifier) runFeed(ctx context.Context, f *store.Feed) {
	var cfg notify.FeedNotifications
	if len(f.Notifications) > 0 {
		if err := json.Unmarshal(f.Notifications, &cfg); err != nil {
			n.log.Warn("decode feed notifications failed", "feed", f.ID, "error", err)
			return
		}
	}
	// A feed that only arms the outage alert has no rule triggers, so the gate
	// here is "can we reach anyone at all" — not "is a rule trigger set".
	if !cfg.HasTarget() {
		return
	}

	disp := cfg.Dispatcher(n.log, n.allowPrivate)
	if cfg.Enabled() {
		n.runMatches(ctx, f, cfg, disp)
	}
	// After the matches: evaluating them refreshes the cache, so a source that
	// just came back counts as healthy in the very same run.
	if cfg.StaleEnabled() {
		n.runStale(ctx, f, cfg, disp)
	}
}

func (n *Notifier) runMatches(ctx context.Context, f *store.Feed, cfg notify.FeedNotifications, disp *notify.Dispatcher) {
	matches, err := n.feeds.Matches(ctx, f)
	if err != nil {
		n.log.Warn("notification match evaluation failed", "feed", f.ID, "error", err)
		return
	}

	for _, m := range matches {
		if !cfg.Triggered(m.Rule) {
			continue
		}
		for _, ev := range m.Events {
			key := m.Rule + "|" + eventKey(ev)
			isNew, err := n.store.MarkNotified(ctx, f.ID, key)
			if err != nil {
				n.log.Warn("mark notified failed", "feed", f.ID, "error", err)
				continue
			}
			if !isNew {
				continue
			}
			if err := disp.Dispatch(ctx, notify.Event{
				Feed:    f.Name,
				Rule:    m.Rule,
				Summary: ev.Summary,
				Message: m.Rule + " matched: " + ev.Summary,
				Time:    n.now(),
			}); err != nil {
				// Nothing got through: drop the ledger entry so the next run
				// retries instead of treating the event as announced.
				n.log.Warn("notification undeliverable; will retry", "feed", f.ID, "error", err)
				if err := n.store.UnmarkNotified(ctx, f.ID, key); err != nil {
					n.log.Warn("roll back notified ledger failed", "feed", f.ID, "error", err)
				}
			}
		}
	}
}

// runStale warns once per source that stopped updating, and once more when it
// recovers. Without this a dead upstream is invisible: the proxy keeps serving
// the last good copy forever, so the calendar silently shows stale dates.
func (n *Notifier) runStale(ctx context.Context, f *store.Feed, cfg notify.FeedNotifications, disp *notify.Dispatcher) {
	threshold := time.Duration(cfg.SourceStaleHours) * time.Hour
	now := n.now()
	for _, src := range f.Sources {
		fetchedAt, err := n.store.CachedFeedFetchedAt(ctx, src.URL)
		if err != nil {
			n.log.Warn("source freshness lookup failed", "feed", f.ID, "error", err)
			continue
		}
		key := staleKey(src.URL)
		// The source URL may carry a ?token= or user:pass@ — the message goes to
		// third-party servers, so only the redacted form may ever leave here.
		safeURL := notify.RedactURL(src.URL)

		if fetchedAt.IsZero() || now.Sub(fetchedAt) > threshold {
			// MarkNotified also refreshes the timestamp of an existing entry, so
			// an outage lasting longer than the ledger retention is not pruned
			// and then announced a second time.
			isNew, err := n.store.MarkNotified(ctx, f.ID, key)
			if err != nil {
				n.log.Warn("mark notified failed", "feed", f.ID, "error", err)
				continue
			}
			if !isNew {
				continue
			}
			msg := fmt.Sprintf("Calendar %q: source %s has never been fetched successfully.", f.Name, safeURL)
			if !fetchedAt.IsZero() {
				msg = fmt.Sprintf("Calendar %q: source %s has not updated for %dh.",
					f.Name, safeURL, int(now.Sub(fetchedAt).Hours()))
			}
			if err := disp.Dispatch(ctx, n.staleEvent(f, ruleSourceStale, msg)); err != nil {
				n.log.Warn("outage notification undeliverable; will retry", "feed", f.ID, "error", err)
				if err := n.store.UnmarkNotified(ctx, f.ID, key); err != nil {
					n.log.Warn("roll back notified ledger failed", "feed", f.ID, "error", err)
				}
			}
			continue
		}

		// Healthy again: only announce recovery if an outage was announced.
		announced, err := n.store.IsNotified(ctx, f.ID, key)
		if err != nil {
			n.log.Warn("read notified ledger failed", "feed", f.ID, "error", err)
			continue
		}
		if !announced {
			continue
		}
		msg := fmt.Sprintf("Calendar %q: source %s is updating again.", f.Name, safeURL)
		if err := disp.Dispatch(ctx, n.staleEvent(f, ruleSourceRecovered, msg)); err != nil {
			// Keep the ledger entry so the next run retries the all-clear.
			n.log.Warn("recovery notification undeliverable; will retry", "feed", f.ID, "error", err)
			continue
		}
		if err := n.store.UnmarkNotified(ctx, f.ID, key); err != nil {
			n.log.Warn("clear notified ledger failed", "feed", f.ID, "error", err)
		}
	}
}

func (n *Notifier) staleEvent(f *store.Feed, rule, msg string) notify.Event {
	return notify.Event{Feed: f.Name, Rule: rule, Summary: f.Name, Message: msg, Time: n.now()}
}

// staleKey identifies a source in the de-duplication ledger. It hashes the URL
// rather than storing it: a raw source URL can contain a token or a password,
// which must not be persisted in clear text in the notified table. The key is
// stable across the "stale" → "never fetched" transition, so a long outage
// still notifies exactly once.
func staleKey(url string) string {
	sum := sha256.Sum256([]byte(url))
	return "source_stale|" + hex.EncodeToString(sum[:])[:16]
}

// eventKey is a stable identity for a matched event: its UID (falling back to
// summary) plus start, so the same occurrence notifies at most once.
func eventKey(ev pipeline.MatchedEvent) string {
	id := ev.UID
	if id == "" {
		id = ev.Summary
	}
	return id + "|" + ev.Start
}
