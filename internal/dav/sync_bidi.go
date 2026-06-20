package dav

import (
	"context"
	"fmt"
)

type hrefState struct {
	etag string
	uid  string
}

type sideItem struct {
	href    string
	etag    string
	uid     string
	changed bool
	data    []byte // populated when fetched (new or changed)
}

// resolveSide lists a collection and resolves each item to a UID, fetching the
// body only for items that are new or whose ETag changed since the last sync.
func resolveSide(ctx context.Context, coll Collection, list []ItemMeta, known map[string]hrefState, uidFn func([]byte) string) (map[string]sideItem, error) {
	out := make(map[string]sideItem, len(list))
	for _, meta := range list {
		if ks, ok := known[meta.Href]; ok && ks.etag == meta.ETag {
			out[ks.uid] = sideItem{href: meta.Href, etag: meta.ETag, uid: ks.uid}
			continue
		}
		item, err := coll.Get(ctx, meta.Href)
		if err != nil {
			return nil, fmt.Errorf("get %s: %w", meta.Href, err)
		}
		uid := uidFn(item.Data)
		if uid == "" {
			uid = meta.Href
		}
		out[uid] = sideItem{href: meta.Href, etag: meta.ETag, uid: uid, changed: true, data: item.Data}
	}
	return out, nil
}

func dataOf(ctx context.Context, coll Collection, si sideItem) ([]byte, error) {
	if si.data != nil {
		return si.data, nil
	}
	item, err := coll.Get(ctx, si.href)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", si.href, err)
	}
	return item.Data, nil
}

// copyItem copies si from one collection to another at toHref, returning the new ETag.
func copyItem(ctx context.Context, from, to Collection, si sideItem, toHref string) (string, error) {
	data, err := dataOf(ctx, from, si)
	if err != nil {
		return "", err
	}
	etag, err := to.Put(ctx, Item{Href: toHref, Data: data})
	if err != nil {
		return "", fmt.Errorf("put %s: %w", toHref, err)
	}
	return etag, nil
}

// winnerIsA decides a both-changed conflict. SourceWins (and the newest-wins
// fallback when no Modified func is set) make A the winner.
func winnerIsA(ctx context.Context, a, b Collection, ai, bi sideItem, opts Options) (bool, error) {
	if opts.Conflict == SourceWins || opts.Modified == nil {
		return true, nil
	}
	aData, err := dataOf(ctx, a, ai)
	if err != nil {
		return false, err
	}
	bData, err := dataOf(ctx, b, bi)
	if err != nil {
		return false, err
	}
	return !opts.Modified(aData).Before(opts.Modified(bData)), nil // A wins on tie
}

// syncBidirectional reconciles both collections, propagating creates, updates and
// deletes in both directions. A change always beats a delete (no data loss); a
// both-changed conflict is resolved by opts.Conflict.
func syncBidirectional(ctx context.Context, a, b Collection, state *State, opts Options) (Result, error) {
	var res Result

	aKnown := make(map[string]hrefState)
	bKnown := make(map[string]hrefState)
	for _, st := range state.Items {
		if st.SrcHref != "" {
			aKnown[st.SrcHref] = hrefState{st.SrcETag, st.UID}
		}
		if st.DstHref != "" {
			bKnown[st.DstHref] = hrefState{st.DstETag, st.UID}
		}
	}

	aList, err := a.List(ctx)
	if err != nil {
		return res, fmt.Errorf("list A: %w", err)
	}
	bList, err := b.List(ctx)
	if err != nil {
		return res, fmt.Errorf("list B: %w", err)
	}
	aSide, err := resolveSide(ctx, a, aList, aKnown, opts.UID)
	if err != nil {
		return res, err
	}
	bSide, err := resolveSide(ctx, b, bList, bKnown, opts.UID)
	if err != nil {
		return res, err
	}

	uids := make(map[string]struct{})
	for u := range aSide {
		uids[u] = struct{}{}
	}
	for u := range bSide {
		uids[u] = struct{}{}
	}
	for u := range state.Items {
		uids[u] = struct{}{}
	}

	for uid := range uids {
		ai, aOK := aSide[uid]
		bi, bOK := bSide[uid]
		st, stOK := state.Items[uid]

		switch {
		case aOK && bOK:
			switch {
			case !ai.changed && !bi.changed:
				// in sync, nothing to do
			case ai.changed && !bi.changed:
				etag, err := copyItem(ctx, a, b, ai, bi.href)
				if err != nil {
					return res, err
				}
				state.Items[uid] = ItemState{UID: uid, SrcHref: ai.href, SrcETag: ai.etag, DstHref: bi.href, DstETag: etag}
				res.Updated++
			case bi.changed && !ai.changed:
				etag, err := copyItem(ctx, b, a, bi, ai.href)
				if err != nil {
					return res, err
				}
				state.Items[uid] = ItemState{UID: uid, SrcHref: ai.href, SrcETag: etag, DstHref: bi.href, DstETag: bi.etag}
				res.Updated++
			default:
				aWins, err := winnerIsA(ctx, a, b, ai, bi, opts)
				if err != nil {
					return res, err
				}
				if aWins {
					etag, err := copyItem(ctx, a, b, ai, bi.href)
					if err != nil {
						return res, err
					}
					state.Items[uid] = ItemState{UID: uid, SrcHref: ai.href, SrcETag: ai.etag, DstHref: bi.href, DstETag: etag}
				} else {
					etag, err := copyItem(ctx, b, a, bi, ai.href)
					if err != nil {
						return res, err
					}
					state.Items[uid] = ItemState{UID: uid, SrcHref: ai.href, SrcETag: etag, DstHref: bi.href, DstETag: bi.etag}
				}
				res.Updated++
			}

		case aOK && !bOK:
			if stOK && st.DstHref != "" && !ai.changed {
				if err := a.Delete(ctx, ai.href); err != nil {
					return res, fmt.Errorf("delete A %s: %w", ai.href, err)
				}
				delete(state.Items, uid)
				res.Deleted++
			} else {
				href := destHref(uid)
				etag, err := copyItem(ctx, a, b, ai, href)
				if err != nil {
					return res, err
				}
				state.Items[uid] = ItemState{UID: uid, SrcHref: ai.href, SrcETag: ai.etag, DstHref: href, DstETag: etag}
				res.Created++
			}

		case !aOK && bOK:
			if stOK && st.SrcHref != "" && !bi.changed {
				if err := b.Delete(ctx, bi.href); err != nil {
					return res, fmt.Errorf("delete B %s: %w", bi.href, err)
				}
				delete(state.Items, uid)
				res.Deleted++
			} else {
				href := destHref(uid)
				etag, err := copyItem(ctx, b, a, bi, href)
				if err != nil {
					return res, err
				}
				state.Items[uid] = ItemState{UID: uid, SrcHref: href, SrcETag: etag, DstHref: bi.href, DstETag: bi.etag}
				res.Created++
			}

		default:
			// gone from both sides since last sync
			delete(state.Items, uid)
		}
	}

	return res, nil
}
