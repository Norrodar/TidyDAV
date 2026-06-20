package dav

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// fakeColl is an in-memory Collection keyed by href.
type fakeColl struct {
	items map[string]Item
	seq   int
}

func newFake() *fakeColl { return &fakeColl{items: map[string]Item{}} }

func (f *fakeColl) set(href, etag, data string) {
	f.items[href] = Item{Href: href, ETag: etag, Data: []byte(data)}
}

func (f *fakeColl) List(_ context.Context) ([]ItemMeta, error) {
	out := make([]ItemMeta, 0, len(f.items))
	for h, it := range f.items {
		out = append(out, ItemMeta{Href: h, ETag: it.ETag})
	}
	return out, nil
}

func (f *fakeColl) Get(_ context.Context, href string) (Item, error) {
	it, ok := f.items[href]
	if !ok {
		return Item{}, fmt.Errorf("not found: %s", href)
	}
	return it, nil
}

func (f *fakeColl) Put(_ context.Context, item Item) (Item, error) {
	f.seq++
	stored := Item{Href: item.Href, ETag: fmt.Sprintf("etag-%d", f.seq), Data: item.Data}
	f.items[item.Href] = stored
	return stored, nil
}

func (f *fakeColl) Delete(_ context.Context, href string) error {
	delete(f.items, href)
	return nil
}

func (f *fakeColl) only(t *testing.T) Item {
	t.Helper()
	if len(f.items) != 1 {
		t.Fatalf("expected exactly one item, have %d", len(f.items))
	}
	for _, it := range f.items {
		return it
	}
	return Item{}
}

// uid extracts the part before '|' so a body can change while its UID stays put.
func uid(d []byte) string {
	s := string(d)
	if i := strings.IndexByte(s, '|'); i >= 0 {
		return s[:i]
	}
	return s
}

func TestSyncOneWayLifecycle(t *testing.T) {
	ctx := context.Background()
	src, dst := newFake(), newFake()
	st := NewState()
	opts := Options{Direction: AToB, UID: uid}

	// Create.
	src.set("/a", "e1", "uid-A|v1")
	res, err := Sync(ctx, src, dst, st, opts)
	if err != nil {
		t.Fatalf("create sync: %v", err)
	}
	if (res != Result{Created: 1}) {
		t.Fatalf("create result = %+v, want {Created:1}", res)
	}
	if got := string(dst.only(t).Data); got != "uid-A|v1" {
		t.Fatalf("dst data = %q", got)
	}

	// Unchanged -> no-op.
	if res, _ = Sync(ctx, src, dst, st, opts); res != (Result{}) {
		t.Errorf("unchanged result = %+v, want zero", res)
	}

	// Update (new etag + body, same UID).
	src.set("/a", "e2", "uid-A|v2")
	if res, err = Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("update sync: %v", err)
	}
	if (res != Result{Updated: 1}) {
		t.Fatalf("update result = %+v, want {Updated:1}", res)
	}
	if got := string(dst.only(t).Data); got != "uid-A|v2" {
		t.Errorf("dst data after update = %q", got)
	}

	// Delete on source -> delete on destination.
	delete(src.items, "/a")
	if res, err = Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("delete sync: %v", err)
	}
	if (res != Result{Deleted: 1}) {
		t.Fatalf("delete result = %+v, want {Deleted:1}", res)
	}
	if len(dst.items) != 0 {
		t.Errorf("dst still has %d items after delete", len(dst.items))
	}
	if len(st.Items) != 0 {
		t.Errorf("state still has %d items after delete", len(st.Items))
	}
}

func TestSyncBToA(t *testing.T) {
	ctx := context.Background()
	a, b := newFake(), newFake()
	b.set("/x", "e1", "uid-X|v1")

	res, err := Sync(ctx, a, b, NewState(), Options{Direction: BToA, UID: uid})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if (res != Result{Created: 1}) {
		t.Fatalf("result = %+v, want {Created:1}", res)
	}
	if string(a.only(t).Data) != "uid-X|v1" {
		t.Errorf("item not copied B->A")
	}
}

func TestSyncErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := Sync(ctx, newFake(), newFake(), NewState(), Options{Direction: AToB}); err == nil {
		t.Error("expected error when UID func is nil")
	}
	if _, err := Sync(ctx, newFake(), newFake(), NewState(), Options{Direction: "nope", UID: uid}); err == nil {
		t.Error("expected error for unknown direction")
	}
}
