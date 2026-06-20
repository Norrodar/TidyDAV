package dav

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
)

var _ Collection = (*CalDAVCollection)(nil)

// CalDAVCollection adapts a remote CalDAV calendar collection to Collection.
type CalDAVCollection struct {
	client *caldav.Client
	path   string // absolute server path of the calendar collection
}

// NewCalDAVCollection connects to the CalDAV calendar at endpoint (the calendar
// collection URL), optionally authenticating with HTTP Basic Auth.
func NewCalDAVCollection(endpoint, username, password string) (*CalDAVCollection, error) {
	var httpClient webdav.HTTPClient = http.DefaultClient
	if username != "" {
		httpClient = webdav.HTTPClientWithBasicAuth(http.DefaultClient, username, password)
	}
	client, err := caldav.NewClient(httpClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("dav: caldav client: %w", err)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("dav: parse endpoint: %w", err)
	}
	return &CalDAVCollection{client: client, path: u.Path}, nil
}

// List returns the calendar object hrefs and ETags (without bodies).
func (c *CalDAVCollection) List(ctx context.Context) ([]ItemMeta, error) {
	infos, err := c.client.ReadDir(ctx, c.path, false)
	if err != nil {
		return nil, fmt.Errorf("dav: list %s: %w", c.path, err)
	}
	collection := strings.TrimRight(c.path, "/")
	out := make([]ItemMeta, 0, len(infos))
	for _, fi := range infos {
		if fi.IsDir || strings.TrimRight(fi.Path, "/") == collection {
			continue // skip subcollections and the collection itself
		}
		out = append(out, ItemMeta{Href: fi.Path, ETag: fi.ETag})
	}
	return out, nil
}

// Get fetches a calendar object and serializes it to ICS bytes.
func (c *CalDAVCollection) Get(ctx context.Context, href string) (Item, error) {
	obj, err := c.client.GetCalendarObject(ctx, href)
	if err != nil {
		return Item{}, fmt.Errorf("dav: get %s: %w", href, err)
	}
	var buf bytes.Buffer
	if err := ics.Serialize(&buf, obj.Data); err != nil {
		return Item{}, fmt.Errorf("dav: serialize %s: %w", href, err)
	}
	return Item{Href: obj.Path, ETag: obj.ETag, Data: buf.Bytes()}, nil
}

// Put parses ICS bytes and stores them, returning the stored item.
func (c *CalDAVCollection) Put(ctx context.Context, item Item) (Item, error) {
	cal, err := ics.Parse(bytes.NewReader(item.Data))
	if err != nil {
		return Item{}, fmt.Errorf("dav: parse item: %w", err)
	}
	path := item.Href
	if !strings.HasPrefix(path, "/") {
		path = strings.TrimRight(c.path, "/") + "/" + path
	}
	obj, err := c.client.PutCalendarObject(ctx, path, cal)
	if err != nil {
		return Item{}, fmt.Errorf("dav: put %s: %w", path, err)
	}
	stored := Item{Href: obj.Path, ETag: obj.ETag, Data: item.Data}
	if stored.Href == "" {
		stored.Href = path // some servers don't echo the path
	}
	return stored, nil
}

// Delete removes a calendar object.
func (c *CalDAVCollection) Delete(ctx context.Context, href string) error {
	if err := c.client.RemoveAll(ctx, href); err != nil {
		return fmt.Errorf("dav: delete %s: %w", href, err)
	}
	return nil
}
