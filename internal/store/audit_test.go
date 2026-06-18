package store

import (
	"context"
	"testing"
)

func TestAuditLog(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.AddAuditEntry(ctx, &AuditEntry{
		UserID: "u", UserEmail: "a@example.com", Action: "feed.create", Target: "f1", Detail: "Müll",
	}); err != nil {
		t.Fatalf("AddAuditEntry: %v", err)
	}
	if err := st.AddAuditEntry(ctx, &AuditEntry{UserID: "u", Action: "feed.delete", Target: "f1"}); err != nil {
		t.Fatalf("AddAuditEntry: %v", err)
	}

	entries, err := st.ListAuditEntries(ctx, 10)
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Action != "feed.delete" {
		t.Errorf("newest action = %q, want feed.delete", entries[0].Action)
	}
	if entries[1].Detail != "Müll" {
		t.Errorf("oldest detail = %q, want Müll", entries[1].Detail)
	}
}
