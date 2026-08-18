package transport

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/github/github-mcp-server/pkg/http/headers"
)

const (
	// defaultETagCacheSize bounds the number of cached conditional responses held
	// in memory by an ETagTransport.
	defaultETagCacheSize = 512

	// defaultMaxEntryBytes bounds the size of a single cached body. Responses
	// larger than this are still revalidated normally but are never retained, so
	// a few large reads cannot pin large buffers in memory.
	defaultMaxEntryBytes = 1 << 20 // 1 MiB

	// defaultMaxTotalBytes bounds the combined size of all cached bodies. The
	// least-recently-used entries are evicted until the cache is within budget.
	defaultMaxTotalBytes = 32 << 20 // 32 MiB
)

// rateLimitHeaders are copied from the live 304 response onto a cache-served
// response so downstream rate-limit accounting observes the current state.
var rateLimitHeaders = []string{
	"X-RateLimit-Limit",
	"X-RateLimit-Remaining",
	"X-RateLimit-Used",
	"X-RateLimit-Reset",
	"X-RateLimit-Resource",
	"Retry-After",
	"Date",
}

// etagEntry is a cached response body and headers keyed by an ETag.
type etagEntry struct {
	etag   string
	status int
	header http.Header
	body   []byte
}

// response reconstructs an *http.Response from a cached entry, layering the
// live 304 response's rate-limit and timing headers on top so the caller sees
// the current rate-limit state while receiving the cached body.
func (e etagEntry) response(live *http.Response) *http.Response {
	h := e.header.Clone()
	for _, name := range rateLimitHeaders {
		if values := live.Header.Values(name); len(values) > 0 {
			h.Del(name)
			for _, v := range values {
				h.Add(name, v)
			}
		}
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", e.status, http.StatusText(e.status)),
		StatusCode:    e.status,
		Proto:         live.Proto,
		ProtoMajor:    live.ProtoMajor,
		ProtoMinor:    live.ProtoMinor,
		Header:        h,
		Body:          io.NopCloser(bytes.NewReader(e.body)),
		ContentLength: int64(len(e.body)),
		Request:       live.Request,
	}
}

type lruItem struct {
	key   string
	entry etagEntry
}

// ETagTransport is an http.RoundTripper that adds HTTP conditional-request
// support (ETag / If-None-Match) to GET requests. For each cacheable GET it
// stores the response ETag and body; on a subsequent identical request it sends
// If-None-Match and, when the server answers 304 Not Modified, serves the
// cached body instead of re-downloading it.
//
// Every request is still sent to the server, so responses are always
// revalidated and never served stale. A 304 Not Modified does not count against
// the GitHub REST API primary rate limit, so revalidated requests conserve
// rate-limit budget and bandwidth.
//
// Cached entries are scoped by the request's Authorization header so responses
// are never shared across tokens. Responses marked non-storable by HTTP cache
// directives (request or response Cache-Control: no-store, or Vary: *) are never
// retained. The cache is bounded both by entry count and by a total-byte budget
// (LRU) and is safe for concurrent use.
//
// This transport is intended for the long-lived local (stdio) server only. The
// hosted, horizontally-scaled server constructs a fresh REST client per request
// and does not use it, so an in-process cache adds nothing there.
type ETagTransport struct {
	Transport http.RoundTripper

	// MaxEntries bounds the number of cached responses. When zero,
	// defaultETagCacheSize is used.
	MaxEntries int

	// MaxEntryBytes bounds the size of a single cached body. Responses larger
	// than this are revalidated but not retained. When zero, defaultMaxEntryBytes
	// is used.
	MaxEntryBytes int

	// MaxTotalBytes bounds the combined size of all cached bodies. When zero,
	// defaultMaxTotalBytes is used.
	MaxTotalBytes int

	mu       sync.Mutex
	ll       *list.List
	items    map[string]*list.Element
	curBytes int
}

func (t *ETagTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}

	// Only cache GET requests, never override a caller-supplied conditional
	// header, and honor a client request to bypass the cache entirely.
	if req.Method != http.MethodGet || req.Header.Get(headers.IfNoneMatchHeader) != "" || hasNoStore(req.Header) {
		return rt.RoundTrip(req)
	}

	key := cacheKey(req)
	cached, ok := t.get(key)

	req = req.Clone(req.Context())
	if ok {
		req.Header.Set(headers.IfNoneMatchHeader, cached.etag)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode == http.StatusNotModified && ok {
		// Discard the empty 304 body and serve the cached response instead.
		if resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		return cached.response(resp), nil
	}

	if resp.StatusCode == http.StatusOK {
		etag := resp.Header.Get(headers.ETagHeader)

		// Drop any prior entry and skip caching when the response is missing an
		// ETag or is marked non-storable by HTTP cache directives.
		if etag == "" || !storable(resp) {
			t.remove(key)
			return resp, nil
		}

		maxEntry := t.maxEntryBytes()
		limited := io.LimitReader(resp.Body, int64(maxEntry)+1)
		body, readErr := io.ReadAll(limited)
		if readErr != nil {
			resp.Body.Close()
			return nil, readErr
		}

		if len(body) > maxEntry {
			t.remove(key)
			resp.Body = struct {
				io.Reader
				io.Closer
			}{io.MultiReader(bytes.NewReader(body), resp.Body), resp.Body}
			return resp, nil
		}

		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))

		t.add(key, etagEntry{
			etag:   etag,
			status: resp.StatusCode,
			header: resp.Header.Clone(),
			body:   body,
		})
	}

	return resp, nil
}

// hasNoStore reports whether a Cache-Control header carries the no-store
// directive.
func hasNoStore(h http.Header) bool {
	for _, cc := range h.Values(headers.CacheControlHeader) {
		for directive := range strings.SplitSeq(cc, ",") {
			if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
				return true
			}
		}
	}
	return false
}

// storable reports whether a response may be retained. Responses that request
// no-store or that vary on every request (Vary: *) must not be cached.
func storable(resp *http.Response) bool {
	if hasNoStore(resp.Header) {
		return false
	}
	for _, vary := range resp.Header.Values(headers.VaryHeader) {
		for field := range strings.SplitSeq(vary, ",") {
			if strings.TrimSpace(field) == "*" {
				return false
			}
		}
	}
	return true
}

func cacheKey(req *http.Request) string {
	sum := sha256.Sum256([]byte(req.Header.Get(headers.AuthorizationHeader)))
	return req.Method + " " + req.URL.String() + " " + hex.EncodeToString(sum[:8])
}

func (t *ETagTransport) get(key string) (etagEntry, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.items == nil {
		return etagEntry{}, false
	}
	el, ok := t.items[key]
	if !ok {
		return etagEntry{}, false
	}
	t.ll.MoveToFront(el)
	return el.Value.(*lruItem).entry, true
}

func (t *ETagTransport) maxEntryBytes() int {
	if t.MaxEntryBytes > 0 {
		return t.MaxEntryBytes
	}
	return defaultMaxEntryBytes
}

func (t *ETagTransport) maxTotalBytes() int {
	if t.MaxTotalBytes > 0 {
		return t.MaxTotalBytes
	}
	return defaultMaxTotalBytes
}

// remove drops a cached entry if present, keeping the byte accounting in sync.
func (t *ETagTransport) remove(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.items == nil {
		return
	}
	el, ok := t.items[key]
	if !ok {
		return
	}
	t.curBytes -= len(el.Value.(*lruItem).entry.body)
	t.ll.Remove(el)
	delete(t.items, key)
}

func (t *ETagTransport) add(key string, entry etagEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.items == nil {
		t.items = make(map[string]*list.Element)
		t.ll = list.New()
	}
	if el, ok := t.items[key]; ok {
		item := el.Value.(*lruItem)
		t.curBytes += len(entry.body) - len(item.entry.body)
		item.entry = entry
		t.ll.MoveToFront(el)
	} else {
		el := t.ll.PushFront(&lruItem{key: key, entry: entry})
		t.items[key] = el
		t.curBytes += len(entry.body)
	}

	limit := t.MaxEntries
	if limit <= 0 {
		limit = defaultETagCacheSize
	}
	maxBytes := t.maxTotalBytes()
	// Evict least-recently-used entries until within both the entry-count and
	// total-byte budgets. Always keep at least the entry just inserted.
	for t.ll.Len() > 1 && (t.ll.Len() > limit || t.curBytes > maxBytes) {
		oldest := t.ll.Back()
		if oldest == nil {
			break
		}
		item := oldest.Value.(*lruItem)
		t.curBytes -= len(item.entry.body)
		t.ll.Remove(oldest)
		delete(t.items, item.key)
	}
}
