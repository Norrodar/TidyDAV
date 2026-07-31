package dav

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// recordColl counts the writes a sync performs, so "this run changed nothing"
// can be asserted on the wire and not just on the Result counters.
type recordColl struct {
	Collection
	puts    int
	deletes int
}

func (r *recordColl) Put(ctx context.Context, item Item) (Item, error) {
	r.puts++
	return r.Collection.Put(ctx, item)
}

func (r *recordColl) Delete(ctx context.Context, href string) error {
	r.deletes++
	return r.Collection.Delete(ctx, href)
}

func (r *recordColl) reset() { r.puts, r.deletes = 0, 0 }

// churnColl hands out a fresh ETag on every listing while the bodies stay put —
// the behaviour of servers that derive ETags from a modification timestamp or a
// request id. A mirror that trusts ETags rewrites everything against such a
// server on every single run.
type churnColl struct {
	*fakeColl
	round int
}

func (c *churnColl) List(ctx context.Context) ([]ItemMeta, error) {
	c.round++
	list, err := c.fakeColl.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].ETag = fmt.Sprintf("%s-r%d", list[i].ETag, c.round)
	}
	return list, nil
}

// mirrored performs an initial one-way sync of a single item and returns the
// source, the raw destination store, the counting wrapper and the state.
func mirrored(t *testing.T, dstBase Collection) (*fakeColl, *recordColl, *State, Options) {
	t.Helper()
	src := newFake()
	dst := &recordColl{Collection: dstBase}
	st := NewState()
	opts := Options{Direction: AToB, UID: uid}

	src.set("/a", "e1", "uid-A|v1")
	res, err := Sync(context.Background(), src, dst, st, opts)
	if err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	if (res != Result{Created: 1}) {
		t.Fatalf("initial result = %+v, want {Created:1}", res)
	}
	if st.Items["uid-A"].DstHref == "" {
		t.Fatalf("no destination href recorded: %+v", st.Items["uid-A"])
	}
	dst.reset()
	return src, dst, st, opts
}

// An item deleted on the destination must come back, even though the source is
// untouched and its ETag still matches the state — exactly the fast path that
// used to hide destination-side changes forever.
func TestSyncOneWayRestoresDeletedDestinationItem(t *testing.T) {
	ctx := context.Background()
	base := newFake()
	src, dst, st, opts := mirrored(t, base)

	href := st.Items["uid-A"].DstHref
	delete(base.items, href)

	res, err := Sync(ctx, src, dst, st, opts)
	if err != nil {
		t.Fatalf("repair sync: %v", err)
	}
	if (res != Result{Created: 1}) {
		t.Fatalf("repair result = %+v, want {Created:1}", res)
	}
	if got := string(base.only(t).Data); got != "uid-A|v1" {
		t.Errorf("restored data = %q, want the source body", got)
	}
	if st.Items["uid-A"].DstHash == "" {
		t.Error("no destination fingerprint recorded after the restore")
	}
	if dst.deletes != 0 {
		t.Errorf("repair issued %d deletes, want 0", dst.deletes)
	}
}

// An item edited on the destination must be overwritten again: a-to-b means the
// source wins.
func TestSyncOneWayOverwritesDriftedDestinationItem(t *testing.T) {
	ctx := context.Background()
	base := newFake()
	src, dst, st, opts := mirrored(t, base)

	href := st.Items["uid-A"].DstHref
	base.set(href, "tampered", "uid-A|edited-on-destination")

	res, err := Sync(ctx, src, dst, st, opts)
	if err != nil {
		t.Fatalf("repair sync: %v", err)
	}
	if (res != Result{Updated: 1}) {
		t.Fatalf("repair result = %+v, want {Updated:1}", res)
	}
	if got := string(base.items[href].Data); got != "uid-A|v1" {
		t.Errorf("destination data = %q, want the source body restored", got)
	}
	if dst.puts != 1 {
		t.Errorf("puts = %d, want exactly 1", dst.puts)
	}

	// And the repaired item settles down: no further writes.
	dst.reset()
	if res, err = Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("settle sync: %v", err)
	}
	if res != (Result{}) || dst.puts != 0 {
		t.Errorf("settle run wrote: result=%+v puts=%d", res, dst.puts)
	}
}

// A run with nothing to do must not write — not even against a destination that
// invents a new ETag on every listing. Comparing ETags instead of bodies would
// turn this into an endless rewrite loop, which is worse than the bug being
// fixed.
func TestSyncOneWayWritesNothingWhenDestinationETagsChurn(t *testing.T) {
	ctx := context.Background()
	base := newFake()
	src, dst, st, opts := mirrored(t, &churnColl{fakeColl: base})

	for run := 1; run <= 3; run++ {
		dst.reset()
		res, err := Sync(ctx, src, dst, st, opts)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if res != (Result{}) {
			t.Errorf("run %d result = %+v, want zero", run, res)
		}
		if dst.puts != 0 || dst.deletes != 0 {
			t.Errorf("run %d wrote: %d puts, %d deletes", run, dst.puts, dst.deletes)
		}
	}
}

// The same guard that protects against a source endpoint answering with an
// empty listing must protect against a destination endpoint doing it.
func TestSyncOneWayGuardsVanishedDestination(t *testing.T) {
	ctx := context.Background()
	src, dst := newFake(), newFake()
	st := NewState()
	opts := Options{Direction: AToB, UID: uid}

	for i := 0; i < vanishGuardThreshold; i++ {
		u := fmt.Sprintf("uid-%d", i)
		src.set("/"+u, "e1", u+"|v1")
	}
	if _, err := Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	// The destination endpoint now lists nothing at all (moved collection URL,
	// broken PROPFIND). Re-uploading everything there would be wrong.
	dst.items = map[string]Item{}
	if _, err := Sync(ctx, src, dst, st, opts); !errors.Is(err, ErrCollectionVanished) {
		t.Fatalf("sync error = %v, want ErrCollectionVanished", err)
	}
	if len(dst.items) != 0 {
		t.Errorf("destination was written to despite the guard: %d items", len(dst.items))
	}
	if len(st.Items) != vanishGuardThreshold {
		t.Errorf("state was modified despite the guard: %d items", len(st.Items))
	}
}

// State written before fingerprints existed has no baseline. Such an item must
// adopt whatever the destination currently holds instead of being rewritten —
// otherwise the first run after an upgrade re-uploads every collection.
func TestSyncOneWayAdoptsMissingBaselineWithoutWriting(t *testing.T) {
	ctx := context.Background()
	base := newFake()
	src, dst, st, opts := mirrored(t, base)

	cur := st.Items["uid-A"]
	cur.SrcHash, cur.DstHash = "", "" // as an old state would deserialise
	st.Items["uid-A"] = cur
	base.set(cur.DstHref, "tampered", "uid-A|drift-from-before-the-upgrade")

	res, err := Sync(ctx, src, dst, st, opts)
	if err != nil {
		t.Fatalf("adoption sync: %v", err)
	}
	if res != (Result{}) || dst.puts != 0 {
		t.Fatalf("adoption run wrote: result=%+v puts=%d", res, dst.puts)
	}
	if st.Items["uid-A"].DstHash == "" {
		t.Fatal("baseline was not adopted")
	}

	// From now on drift is healed.
	base.set(cur.DstHref, "tampered-again", "uid-A|drift-after-the-upgrade")
	dst.reset()
	if res, err = Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("healing sync: %v", err)
	}
	if (res != Result{Updated: 1}) {
		t.Fatalf("healing result = %+v, want {Updated:1}", res)
	}
}

// Listing the destination must not turn the one-way sync into a destination
// cleaner: items TidyDAV never wrote stay untouched.
func TestSyncOneWayLeavesUnknownDestinationItemsAlone(t *testing.T) {
	ctx := context.Background()
	base := newFake()
	src, dst, st, opts := mirrored(t, base)
	base.set("/foreign.ics", "f1", "uid-F|not ours")

	res, err := Sync(ctx, src, dst, st, opts)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res != (Result{}) {
		t.Errorf("result = %+v, want zero", res)
	}
	if _, ok := base.items["/foreign.ics"]; !ok {
		t.Error("a destination item TidyDAV does not manage was removed")
	}
	if dst.deletes != 0 {
		t.Errorf("deletes = %d, want 0", dst.deletes)
	}
}

// The date window keeps its meaning for repairs: an out-of-window item is
// neither restored on the destination nor deleted from the state.
func TestSyncOneWayRepairRespectsWindow(t *testing.T) {
	ctx := context.Background()
	src, dst := newFake(), newFake()
	st := NewState()
	src.set("/in", "1", vevent("in", "In", "20260115"))

	jan := Options{Direction: AToB, UID: CalendarUID}
	jan.WindowStart, jan.WindowEnd, _ = ParseWindow("2026-01-01", "2026-01-31")
	if _, err := Sync(ctx, src, dst, st, jan); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	href := st.Items["in"].DstHref
	delete(dst.items, href) // destroyed on the destination

	// The window has since moved past the event.
	mar := Options{Direction: AToB, UID: CalendarUID}
	mar.WindowStart, mar.WindowEnd, _ = ParseWindow("2026-03-01", "2026-03-31")
	res, err := Sync(ctx, src, dst, st, mar)
	if err != nil {
		t.Fatalf("windowed sync: %v", err)
	}
	if res != (Result{}) {
		t.Fatalf("result = %+v, want zero (out of window: no restore, no delete)", res)
	}
	if len(dst.items) != 0 {
		t.Error("out-of-window item was restored")
	}
	if _, ok := st.Items["in"]; !ok {
		t.Error("out-of-window state entry was dropped")
	}
}

// failGetColl makes one destination object unreadable, the way a server does
// for a single corrupt or permission-protected resource.
type failGetColl struct {
	*fakeColl
	failHref string
}

func (c *failGetColl) Get(ctx context.Context, href string) (Item, error) {
	if href == c.failHref {
		return Item{}, errors.New("403 forbidden")
	}
	return c.fakeColl.Get(ctx, href)
}

// One unreadable object on the destination must not stop the run: every other
// item is still repaired and the deletion pass still runs. Aborting would mean a
// single broken resource freezes the whole job forever.
func TestSyncOneWayContinuesWhenADestinationItemIsUnreadable(t *testing.T) {
	ctx := context.Background()
	src, base := newFake(), newFake()
	st := NewState()
	opts := Options{Direction: AToB, UID: uid}

	for _, u := range []string{"uid-1", "uid-2", "uid-3"} {
		src.set("/"+u, "e1", u+"|v1")
	}
	broken := &failGetColl{fakeColl: base}
	dst := &recordColl{Collection: broken}
	if _, err := Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	dst.reset()

	// uid-1: its destination copy becomes unreadable (and its ETag moved, so the
	// cheap path cannot skip the read). uid-2: edited on the destination, must
	// still be repaired. uid-3: gone from the source, must still be deleted.
	broken.failHref = st.Items["uid-1"].DstHref
	base.set(broken.failHref, "moved", "uid-1|v1")
	base.set(st.Items["uid-2"].DstHref, "tampered", "uid-2|edited-on-destination")
	delete(src.items, "/uid-3")

	res, err := Sync(ctx, src, dst, st, opts)
	if err == nil {
		t.Fatal("run reported success although an item could not be verified")
	}
	if !strings.Contains(err.Error(), "could not be verified") {
		t.Errorf("error = %v, want it to name the unverified items", err)
	}
	if (res != Result{Updated: 1, Deleted: 1}) {
		t.Errorf("result = %+v, want {Updated:1 Deleted:1} — later items must still be processed", res)
	}
	if got := string(base.items[st.Items["uid-2"].DstHref].Data); got != "uid-2|v1" {
		t.Errorf("uid-2 = %q, want the source body restored", got)
	}
	if _, ok := st.Items["uid-3"]; ok {
		t.Error("deletion pass did not run")
	}
	if _, ok := st.Items["uid-1"]; !ok {
		t.Error("the unverifiable item was dropped from the state")
	}
	if _, ok := base.items[broken.failHref]; !ok {
		t.Error("the unverifiable item was deleted on the destination")
	}
}

// The deletion counter reports what was removed from the destination. An item
// the user already deleted there is not counted a second time by TidyDAV.
func TestSyncOneWayCountsOnlyDeletesItIssued(t *testing.T) {
	ctx := context.Background()
	base := newFake()
	src, dst, st, opts := mirrored(t, base)

	delete(base.items, st.Items["uid-A"].DstHref) // deleted by hand on the destination
	delete(src.items, "/a")                       // and gone from the source in the same run

	res, err := Sync(ctx, src, dst, st, opts)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res != (Result{}) {
		t.Errorf("result = %+v, want zero: TidyDAV deleted nothing", res)
	}
	if dst.deletes != 0 {
		t.Errorf("deletes = %d, want 0", dst.deletes)
	}
	if _, ok := st.Items["uid-A"]; ok {
		t.Error("state entry survived although source and destination copies are gone")
	}
}

// State from a release without fingerprints must not be rewritten wholesale —
// not even when the source hands out a fresh ETag on every listing, which is
// what makes the fast path miss.
func TestSyncOneWayAdoptsLegacyStateWhenSourceETagsChurn(t *testing.T) {
	ctx := context.Background()
	srcBase, dstBase := newFake(), newFake()
	src := &churnColl{fakeColl: srcBase}
	dst := &recordColl{Collection: dstBase}
	st := NewState()
	opts := Options{Direction: AToB, UID: uid}

	for i := 0; i < 10; i++ {
		u := fmt.Sprintf("uid-%d", i)
		srcBase.set("/"+u, "e1", u+"|v1")
	}
	if _, err := Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	for k, v := range st.Items { // as a state written before fingerprints existed
		v.SrcHash, v.DstHash = "", ""
		st.Items[k] = v
	}
	dst.reset()

	res, err := Sync(ctx, src, dst, st, opts)
	if err != nil {
		t.Fatalf("upgrade run: %v", err)
	}
	if res != (Result{}) || dst.puts != 0 {
		t.Fatalf("upgrade run rewrote the collection: result=%+v puts=%d", res, dst.puts)
	}

	// A real source change is still propagated afterwards.
	srcBase.set("/uid-3", "e1", "uid-3|v2")
	dst.reset()
	if res, err = Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("change run: %v", err)
	}
	if (res != Result{Updated: 1}) || dst.puts != 1 {
		t.Errorf("change run: result=%+v puts=%d, want one update", res, dst.puts)
	}
}

func TestNormHrefAndLookup(t *testing.T) {
	idx := indexByHref([]ItemMeta{
		{Href: "https://dav.example.com/cal/user/a%20b.ics", ETag: "e1"},
		{Href: "/cal/user/plain.ics", ETag: "e2"},
	})
	cases := []struct {
		href string
		want string
	}{
		{"https://dav.example.com/cal/user/a%20b.ics", "e1"}, // exact
		{"/cal/user/a b.ics", "e1"},                          // decoded path
		{"/cal/user/a%20b.ics", "e1"},                        // encoded path
		{"/cal//user/plain.ics", "e2"},                       // doubled separator
		{"/cal/user/plain.ics/", "e2"},                       // trailing slash
	}
	for _, tc := range cases {
		meta, ok := lookupMeta(idx, tc.href)
		if !ok || meta.ETag != tc.want {
			t.Errorf("lookupMeta(%q) = %+v/%v, want etag %q", tc.href, meta, ok, tc.want)
		}
	}
	if _, ok := lookupMeta(idx, "/cal/user/other.ics"); ok {
		t.Error("lookupMeta matched an href that is not listed")
	}
	if _, ok := lookupMeta(idx, ""); ok {
		t.Error("lookupMeta matched an empty href")
	}
}

// The persisted state is a plain JSON blob shared with older releases: the new
// fields must be additive both ways.
func TestItemStateJSONStaysCompatible(t *testing.T) {
	var st ItemState
	old := `{"uid":"u1","srcHref":"/a","srcETag":"e1","dstHref":"/b","dstETag":"e2"}`
	if err := json.Unmarshal([]byte(old), &st); err != nil {
		t.Fatalf("decode legacy state: %v", err)
	}
	if st.UID != "u1" || st.DstHref != "/b" || st.SrcHash != "" || st.DstHash != "" {
		t.Fatalf("legacy state decoded as %+v", st)
	}

	// Without fingerprints (bidirectional jobs) the encoding is unchanged.
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if strings.Contains(string(raw), "Hash") || strings.Contains(string(raw), "hash") {
		t.Errorf("hash keys leaked into a state without fingerprints: %s", raw)
	}
}

func TestBodyHashDistinguishesBodies(t *testing.T) {
	if bodyHash([]byte("a")) == bodyHash([]byte("b")) {
		t.Error("different bodies share a fingerprint")
	}
	if bodyHash([]byte("a")) != bodyHash([]byte("a")) {
		t.Error("fingerprint is not stable")
	}
	if len(bodyHash(nil)) != 32 {
		t.Errorf("fingerprint length = %d, want 32", len(bodyHash(nil)))
	}
}
