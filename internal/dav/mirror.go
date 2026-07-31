package dav

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// bodyHash fingerprints an item body. Truncated to 16 bytes (32 hex chars):
// 128 bit is plenty to detect a changed body and keeps the persisted sync state
// small — the hashes are stored per item, twice.
func bodyHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

// normHref reduces a href to a comparable form: URL-decoded path, cleaned, no
// trailing slash. Servers are free to echo a href back differently from how they
// list it (absolute URL vs. path, percent-encoding, "//"), and a mismatch would
// make every known item look missing — i.e. rewrite the whole collection on
// every run.
func normHref(h string) string {
	if h == "" {
		return ""
	}
	s := h
	if u, err := url.Parse(s); err == nil && u.Path != "" {
		s = u.Path // url.Parse already unescapes into Path
	} else if dec, err := url.PathUnescape(s); err == nil {
		s = dec
	}
	s = path.Clean(s)
	if len(s) > 1 {
		s = strings.TrimRight(s, "/")
	}
	return s
}

// indexByHref indexes a listing under both the raw and the normalised href so a
// lookup can fall back from an exact to a tolerant match. Raw keys win.
func indexByHref(list []ItemMeta) map[string]ItemMeta {
	idx := make(map[string]ItemMeta, len(list)*2)
	for _, m := range list {
		if m.Href != "" {
			idx[m.Href] = m
		}
	}
	for _, m := range list {
		n := normHref(m.Href)
		if n == "" {
			continue
		}
		if _, ok := idx[n]; !ok {
			idx[n] = m
		}
	}
	return idx
}

// lookupMeta finds the listing entry for href, exact match first.
func lookupMeta(idx map[string]ItemMeta, href string) (ItemMeta, bool) {
	if href == "" {
		return ItemMeta{}, false
	}
	if m, ok := idx[href]; ok {
		return m, true
	}
	if n := normHref(href); n != "" {
		if m, ok := idx[n]; ok {
			return m, true
		}
	}
	return ItemMeta{}, false
}

// dstStatus is the verdict of inspecting the destination copy of a known item.
type dstStatus int

const (
	dstOK      dstStatus = iota // present and matching the recorded fingerprint
	dstMissing                  // not listed on the destination any more
	dstDrifted                  // present but its body no longer matches
)

// inspectDst checks the destination copy described by st without writing
// anything. It returns the verdict plus the ETag and body fingerprint to record
// in the state.
//
// A missing fingerprint (st.DstHash == "") means "no baseline yet" — state
// written before this check existed, or a Put whose read-back failed. Such an
// item adopts whatever the destination currently holds instead of being
// rewritten: adopting keeps drift that predates the baseline, rewriting would
// re-upload every item of every collection once, which is far worse.
func inspectDst(ctx context.Context, dst Collection, st ItemState, idx map[string]ItemMeta) (dstStatus, string, string, error) {
	if st.DstHref == "" {
		return dstMissing, "", "", nil
	}
	meta, ok := lookupMeta(idx, st.DstHref)
	if !ok {
		return dstMissing, "", "", nil
	}
	if meta.ETag != "" && meta.ETag == st.DstETag && st.DstHash != "" {
		return dstOK, meta.ETag, st.DstHash, nil // cheap path: nothing changed
	}

	// The ETag moved (or we have none to compare): only the body can tell a real
	// edit from a server that reissues ETags on every listing.
	item, err := dst.Get(ctx, meta.Href)
	if err != nil {
		return dstOK, "", "", fmt.Errorf("inspect destination %s: %w", meta.Href, err)
	}
	etag := meta.ETag
	if etag == "" {
		etag = item.ETag
	}
	hash := bodyHash(item.Data)
	if st.DstHash == "" || hash == st.DstHash {
		return dstOK, etag, hash, nil
	}
	return dstDrifted, etag, hash, nil
}

// putMirror writes data to the destination and learns the server's own
// rendering of it via a read-back.
//
// The baseline must be the destination's view, not ours: DAV servers
// canonicalise what they store (SEQUENCE, PRODID, property order, VTIMEZONE),
// so hashing the bytes we sent would flag the item as drifted on the very next
// run and rewrite it forever. The read-back costs one GET per write, not per
// item and not per run.
func putMirror(ctx context.Context, dst Collection, href, etag string, data []byte) (string, string, string, error) {
	stored, err := dst.Put(ctx, Item{Href: href, ETag: etag, Data: data})
	if err != nil {
		return "", "", "", err
	}
	newHref := stored.Href
	if newHref == "" {
		newHref = href
	}
	newETag, newHash := stored.ETag, bodyHash(data)
	// A failed read-back is not fatal (eventual consistency, missing read
	// rights): we keep the optimistic fingerprint and at worst rewrite the item
	// once more on the next run.
	if back, err := dst.Get(ctx, newHref); err == nil {
		newHash = bodyHash(back.Data)
		if back.ETag != "" {
			newETag = back.ETag
		}
	}
	return newHref, newETag, newHash, nil
}
