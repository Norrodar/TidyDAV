package dav

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// body encodes "uid|unixModified|content" so tests can vary modified time and
// content independently while keeping a stable UID.
func body(uid string, mod int64, content string) string {
	return fmt.Sprintf("%s|%d|%s", uid, mod, content)
}

func modOf(d []byte) time.Time {
	parts := strings.SplitN(string(d), "|", 3)
	if len(parts) >= 2 {
		if n, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			return time.Unix(n, 0)
		}
	}
	return time.Time{}
}

func biOpts(conflict Conflict) Options {
	return Options{Direction: Bidirectional, Conflict: conflict, UID: uid, Modified: modOf}
}

// synced returns two collections sharing one item (U1) plus the established state.
func synced(t *testing.T) (*fakeColl, *fakeColl, *State) {
	t.Helper()
	a, b := newFake(), newFake()
	a.set("/a1", "ea1", body("U1", 1, "v1"))
	st := NewState()
	if _, err := Sync(context.Background(), a, b, st, biOpts(NewestWins)); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	return a, b, st
}

func TestBidiInitialMerge(t *testing.T) {
	ctx := context.Background()
	a, b := newFake(), newFake()
	a.set("/a1", "ea1", body("U1", 1, "fromA"))
	b.set("/b1", "eb1", body("U2", 1, "fromB"))

	res, err := Sync(ctx, a, b, NewState(), biOpts(NewestWins))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Created != 2 {
		t.Fatalf("created = %d, want 2", res.Created)
	}
	if len(a.items) != 2 || len(b.items) != 2 {
		t.Fatalf("after merge a=%d b=%d items, want 2/2", len(a.items), len(b.items))
	}
}

func TestBidiUpdateAToB(t *testing.T) {
	ctx := context.Background()
	a, b, st := synced(t)
	bHref := st.Items["U1"].DstHref

	a.set("/a1", "ea2", body("U1", 2, "v2")) // changed on A only
	res, err := Sync(ctx, a, b, st, biOpts(NewestWins))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Updated != 1 {
		t.Fatalf("updated = %d, want 1", res.Updated)
	}
	if got := string(b.items[bHref].Data); got != body("U1", 2, "v2") {
		t.Errorf("B data = %q, want v2", got)
	}
}

func TestBidiConflictNewestWins(t *testing.T) {
	ctx := context.Background()
	a, b, st := synced(t)
	bHref := st.Items["U1"].DstHref

	a.set("/a1", "ea2", body("U1", 5, "fromA"))
	b.set(bHref, "eb2", body("U1", 9, "fromB")) // B is newer

	res, err := Sync(ctx, a, b, st, biOpts(NewestWins))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Updated != 1 {
		t.Fatalf("updated = %d, want 1", res.Updated)
	}
	if got := string(a.items["/a1"].Data); got != body("U1", 9, "fromB") {
		t.Errorf("A data = %q, want B's newer content", got)
	}
}

func TestBidiConflictSourceWins(t *testing.T) {
	ctx := context.Background()
	a, b, st := synced(t)
	bHref := st.Items["U1"].DstHref

	a.set("/a1", "ea2", body("U1", 5, "fromA"))
	b.set(bHref, "eb2", body("U1", 9, "fromB")) // newer but source (A) wins

	if _, err := Sync(ctx, a, b, st, biOpts(SourceWins)); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got := string(b.items[bHref].Data); got != body("U1", 5, "fromA") {
		t.Errorf("B data = %q, want A's content (source-wins)", got)
	}
}

func TestBidiDeletePropagates(t *testing.T) {
	ctx := context.Background()
	a, b, st := synced(t)

	delete(a.items, "/a1") // deleted on A, untouched on B
	res, err := Sync(ctx, a, b, st, biOpts(NewestWins))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", res.Deleted)
	}
	if len(b.items) != 0 {
		t.Errorf("B still has %d items", len(b.items))
	}
}

func TestBidiChangeBeatsDelete(t *testing.T) {
	ctx := context.Background()
	a, b, st := synced(t)
	bHref := st.Items["U1"].DstHref

	delete(a.items, "/a1")                       // deleted on A
	b.set(bHref, "eb2", body("U1", 9, "edited")) // changed on B

	res, err := Sync(ctx, a, b, st, biOpts(NewestWins))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("created = %d, want 1 (resurrected)", res.Created)
	}
	if len(a.items) != 1 {
		t.Fatalf("A has %d items, want 1 (resurrected)", len(a.items))
	}
	if got := string(a.only(t).Data); got != body("U1", 9, "edited") {
		t.Errorf("resurrected A data = %q", got)
	}
}
