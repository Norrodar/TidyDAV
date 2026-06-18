// Package audit records admin-visible configuration changes.
package audit

import (
	"context"
	"log/slog"

	"github.com/Norrodar/TidyDAV/internal/store"
)

// Logger writes audit entries best-effort.
type Logger struct {
	store *store.Store
	log   *slog.Logger
}

// New creates an audit logger.
func New(st *store.Store, log *slog.Logger) *Logger {
	return &Logger{store: st, log: log}
}

// Record writes an audit entry. Failures are logged, not returned, so auditing
// never blocks the action being audited.
func (l *Logger) Record(ctx context.Context, user *store.User, action, target, detail string) {
	if user == nil {
		return
	}
	e := &store.AuditEntry{UserID: user.ID, Action: action, Target: target, Detail: detail}
	if user.Email.Valid {
		e.UserEmail = user.Email.String
	}
	if err := l.store.AddAuditEntry(ctx, e); err != nil {
		l.log.Warn("audit write failed", "action", action, "error", err)
	}
}
