// Package notifier evaluates feed notification triggers on a schedule and
// dispatches a notification the first time each matched event is seen.
package notifier

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Norrodar/TidyDAV/internal/feed"
	"github.com/Norrodar/TidyDAV/internal/notify"
	"github.com/Norrodar/TidyDAV/internal/pipeline"
	"github.com/Norrodar/TidyDAV/internal/store"
)

// retention bounds how long the de-duplication ledger keeps entries.
const retention = 30 * 24 * time.Hour

// Notifier dispatches feed notifications.
type Notifier struct {
	store        *store.Store
	feeds        *feed.Service
	log          *slog.Logger
	allowPrivate bool
}

// New creates a Notifier. allowPrivate mirrors TIDYDAV_ALLOW_PRIVATE_TARGETS
// and gates whether notification targets may resolve to non-public addresses.
func New(st *store.Store, feeds *feed.Service, log *slog.Logger, allowPrivate bool) *Notifier {
	return &Notifier{store: st, feeds: feeds, log: log, allowPrivate: allowPrivate}
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
	if _, err := n.store.DeleteNotifiedBefore(ctx, time.Now().Add(-retention)); err != nil {
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
	if !cfg.Enabled() {
		return
	}

	matches, err := n.feeds.Matches(ctx, f)
	if err != nil {
		n.log.Warn("notification match evaluation failed", "feed", f.ID, "error", err)
		return
	}

	disp := cfg.Dispatcher(n.log, n.allowPrivate)
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
				Time:    time.Now(),
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

// eventKey is a stable identity for a matched event: its UID (falling back to
// summary) plus start, so the same occurrence notifies at most once.
func eventKey(ev pipeline.MatchedEvent) string {
	id := ev.UID
	if id == "" {
		id = ev.Summary
	}
	return id + "|" + ev.Start
}
