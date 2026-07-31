package dav

import (
	"context"
	"fmt"
	"strings"
)

// Sync reconciles collections a and b according to opts, updating state in place.
func Sync(ctx context.Context, a, b Collection, state *State, opts Options) (Result, error) {
	if state.Items == nil {
		state.Items = map[string]ItemState{}
	}
	if opts.UID == nil {
		return Result{}, fmt.Errorf("dav: Options.UID is required")
	}

	switch opts.Direction {
	case AToB:
		return syncOneWay(ctx, a, b, state, opts)
	case BToA:
		return syncOneWay(ctx, b, a, state, opts)
	case Bidirectional:
		return syncBidirectional(ctx, a, b, state, opts)
	default:
		return Result{}, fmt.Errorf("dav: unknown direction %q", opts.Direction)
	}
}

// syncOneWay mirrors src onto dst: creates/updates changed items and deletes
// destination items whose source counterpart is gone. State.Items[*].Src* refer
// to src and Dst* to dst.
//
// It is a true mirror, not a change feed: the destination is listed and every
// known item is verified against it, so a copy deleted or edited on the
// destination is restored on the next run even though the source never changed.
// Verification compares body fingerprints rather than ETags, so a destination
// that reissues ETags on every listing does not trigger an endless rewrite.
// Destination items TidyDAV does not know about are left alone.
func syncOneWay(ctx context.Context, src, dst Collection, state *State, opts Options) (Result, error) {
	var res Result
	uidFn := opts.UID

	stateBySrcHref := make(map[string]ItemState, len(state.Items))
	knownDst := 0
	for _, st := range state.Items {
		if st.SrcHref != "" {
			stateBySrcHref[st.SrcHref] = st
		}
		if st.DstHref != "" {
			knownDst++
		}
	}

	srcList, err := src.List(ctx)
	if err != nil {
		return res, fmt.Errorf("list source: %w", err)
	}
	if err := guardVanished("source", len(srcList), len(state.Items)); err != nil {
		return res, err
	}

	dstList, err := dst.List(ctx)
	if err != nil {
		return res, fmt.Errorf("list destination: %w", err)
	}
	// A destination that suddenly lists nothing is as suspicious as a source
	// that does: without this guard the mirror would happily re-upload the whole
	// collection into a wrong or broken endpoint.
	if err := guardVanished("destination", len(dstList), knownDst); err != nil {
		return res, err
	}
	dstIdx := indexByHref(dstList)

	seen := make(map[string]bool, len(srcList))
	filtered := make(map[string]bool) // out-of-window UIDs: skipped, protected from deletion
	for _, meta := range srcList {
		var (
			checkedUID string // UID whose destination copy was already inspected
			status     dstStatus
			dstETag    string
			dstHash    string
		)

		// Source unchanged: still verify the destination copy. Skipping that
		// check is what used to make destination-side deletions and edits
		// permanent.
		if st, ok := stateBySrcHref[meta.Href]; ok && st.SrcETag == meta.ETag {
			cur, ok := state.Items[st.UID]
			if !ok {
				cur = st
			}
			if status, dstETag, dstHash, err = inspectDst(ctx, dst, cur, dstIdx); err != nil {
				return res, err
			}
			checkedUID = st.UID
			if status == dstOK {
				seen[st.UID] = true
				cur.DstETag, cur.DstHash = dstETag, dstHash
				state.Items[st.UID] = cur
				continue // nothing to write
			}
			// Destination copy is missing or drifted: fall through, the repair
			// needs the source body.
		}

		item, err := src.Get(ctx, meta.Href)
		if err != nil {
			return res, fmt.Errorf("get %s: %w", meta.Href, err)
		}
		uid := uidFn(item.Data)
		if uid == "" {
			uid = meta.Href
		}
		if !opts.inWindow(item.Data) {
			filtered[uid] = true // outside the date window: don't sync, don't delete
			continue
		}
		seen[uid] = true

		cur := state.Items[uid]
		cur.UID = uid
		cur.SrcHref = meta.Href
		cur.SrcETag = meta.ETag
		srcHash := bodyHash(item.Data)
		// Compare bodies, not ETags: sources that reissue ETags without any
		// content change (a plain re-save on Nextcloud/SOGo) must not cause a
		// write. A changed body always changes the hash, so nothing is missed.
		srcChanged := cur.SrcHash == "" || cur.SrcHash != srcHash
		cur.SrcHash = srcHash

		if checkedUID != uid {
			if status, dstETag, dstHash, err = inspectDst(ctx, dst, cur, dstIdx); err != nil {
				return res, err
			}
		}

		switch {
		case status == dstMissing:
			// Restore onto the remembered href when there is one: the worst case
			// of a href format mismatch is then a redundant overwrite, never a
			// duplicate.
			href := cur.DstHref
			if href == "" {
				href = destHref(uid, opts.suffix())
			}
			newHref, newETag, newHash, err := putMirror(ctx, dst, href, "", item.Data)
			if err != nil {
				return res, fmt.Errorf("create: %w", err)
			}
			cur.DstHref, cur.DstETag, cur.DstHash = newHref, newETag, newHash
			res.Created++
		case status == dstDrifted || srcChanged:
			newHref, newETag, newHash, err := putMirror(ctx, dst, cur.DstHref, cur.DstETag, item.Data)
			if err != nil {
				return res, fmt.Errorf("update %s: %w", cur.DstHref, err)
			}
			cur.DstHref, cur.DstETag, cur.DstHash = newHref, newETag, newHash
			res.Updated++
		default:
			// In sync on both sides; only refresh the recorded fingerprints.
			cur.DstETag, cur.DstHash = dstETag, dstHash
		}
		state.Items[uid] = cur
	}

	// Propagate deletions: state entries whose source item disappeared.
	for uid, st := range state.Items {
		if seen[uid] || filtered[uid] {
			continue
		}
		if st.DstHref != "" {
			// Skip the DELETE when the destination copy is already gone.
			if _, ok := lookupMeta(dstIdx, st.DstHref); ok {
				if err := dst.Delete(ctx, st.DstHref); err != nil {
					return res, fmt.Errorf("delete %s: %w", st.DstHref, err)
				}
			}
		}
		delete(state.Items, uid)
		res.Deleted++
	}

	return res, nil
}

// destHref derives a safe destination href from a UID plus a suffix (e.g. .ics).
func destHref(uid, suffix string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, uid)
	if safe == "" {
		safe = "item"
	}
	return safe + suffix
}
