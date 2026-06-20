package dav

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
)

var _ Collection = (*CardDAVCollection)(nil)

// CardDAVCollection adapts a remote CardDAV address book to Collection.
type CardDAVCollection struct {
	client *carddav.Client
	path   string
}

// NewCardDAVCollection connects to the CardDAV address book at endpoint,
// optionally authenticating with HTTP Basic Auth.
func NewCardDAVCollection(endpoint, username, password string) (*CardDAVCollection, error) {
	client, err := carddav.NewClient(davHTTPClient(username, password), endpoint)
	if err != nil {
		return nil, fmt.Errorf("dav: carddav client: %w", err)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("dav: parse endpoint: %w", err)
	}
	return &CardDAVCollection{client: client, path: u.Path}, nil
}

// List returns the address object hrefs and ETags (without bodies).
func (c *CardDAVCollection) List(ctx context.Context) ([]ItemMeta, error) {
	infos, err := c.client.ReadDir(ctx, c.path, false)
	if err != nil {
		return nil, fmt.Errorf("dav: list %s: %w", c.path, err)
	}
	collection := strings.TrimRight(c.path, "/")
	out := make([]ItemMeta, 0, len(infos))
	for _, fi := range infos {
		if fi.IsDir || strings.TrimRight(fi.Path, "/") == collection {
			continue
		}
		out = append(out, ItemMeta{Href: fi.Path, ETag: fi.ETag})
	}
	return out, nil
}

// Get fetches an address object and serializes it to vCard bytes.
func (c *CardDAVCollection) Get(ctx context.Context, href string) (Item, error) {
	obj, err := c.client.GetAddressObject(ctx, href)
	if err != nil {
		return Item{}, fmt.Errorf("dav: get %s: %w", href, err)
	}
	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(obj.Card); err != nil {
		return Item{}, fmt.Errorf("dav: encode %s: %w", href, err)
	}
	return Item{Href: obj.Path, ETag: obj.ETag, Data: buf.Bytes()}, nil
}

// Put parses vCard bytes and stores them, returning the stored item.
func (c *CardDAVCollection) Put(ctx context.Context, item Item) (Item, error) {
	card, err := vcard.NewDecoder(bytes.NewReader(item.Data)).Decode()
	if err != nil {
		return Item{}, fmt.Errorf("dav: parse vcard: %w", err)
	}
	path := item.Href
	if !strings.HasPrefix(path, "/") {
		path = strings.TrimRight(c.path, "/") + "/" + path
	}
	obj, err := c.client.PutAddressObject(ctx, path, card)
	if err != nil {
		return Item{}, fmt.Errorf("dav: put %s: %w", path, err)
	}
	stored := Item{Href: obj.Path, ETag: obj.ETag, Data: item.Data}
	if stored.Href == "" {
		stored.Href = path
	}
	return stored, nil
}

// Delete removes an address object.
func (c *CardDAVCollection) Delete(ctx context.Context, href string) error {
	if err := c.client.RemoveAll(ctx, href); err != nil {
		return fmt.Errorf("dav: delete %s: %w", href, err)
	}
	return nil
}
