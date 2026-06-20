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
func syncOneWay(ctx context.Context, src, dst Collection, state *State, opts Options) (Result, error) {
	var res Result
	uidFn := opts.UID

	stateBySrcHref := make(map[string]ItemState, len(state.Items))
	for _, st := range state.Items {
		if st.SrcHref != "" {
			stateBySrcHref[st.SrcHref] = st
		}
	}

	srcList, err := src.List(ctx)
	if err != nil {
		return res, fmt.Errorf("list source: %w", err)
	}

	seen := make(map[string]bool, len(srcList))
	for _, meta := range srcList {
		if st, ok := stateBySrcHref[meta.Href]; ok && st.SrcETag == meta.ETag {
			seen[st.UID] = true // unchanged, no fetch needed
			continue
		}

		item, err := src.Get(ctx, meta.Href)
		if err != nil {
			return res, fmt.Errorf("get %s: %w", meta.Href, err)
		}
		uid := uidFn(item.Data)
		if uid == "" {
			uid = meta.Href
		}
		seen[uid] = true

		cur := state.Items[uid]
		cur.UID = uid
		cur.SrcHref = meta.Href
		cur.SrcETag = meta.ETag

		if cur.DstHref == "" {
			stored, err := dst.Put(ctx, Item{Href: destHref(uid, opts.suffix()), Data: item.Data})
			if err != nil {
				return res, fmt.Errorf("create: %w", err)
			}
			cur.DstHref, cur.DstETag = stored.Href, stored.ETag
			res.Created++
		} else {
			stored, err := dst.Put(ctx, Item{Href: cur.DstHref, ETag: cur.DstETag, Data: item.Data})
			if err != nil {
				return res, fmt.Errorf("update %s: %w", cur.DstHref, err)
			}
			cur.DstHref, cur.DstETag = stored.Href, stored.ETag
			res.Updated++
		}
		state.Items[uid] = cur
	}

	// Propagate deletions: state entries whose source item disappeared.
	for uid, st := range state.Items {
		if seen[uid] {
			continue
		}
		if st.DstHref != "" {
			if err := dst.Delete(ctx, st.DstHref); err != nil {
				return res, fmt.Errorf("delete %s: %w", st.DstHref, err)
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
