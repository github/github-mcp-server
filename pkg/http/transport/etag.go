package transport

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/github/github-mcp-server/pkg/http/headers"
)

// defaultETagCacheSize bounds the number of cached conditional responses held
// in memory by an ETagTransport.
const defaultETagCacheSize = 512

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
// are never shared across tokens. The cache is bounded (LRU) and safe for
// concurrent use.
type ETagTransport struct {
	Transport http.RoundTripper

	// MaxEntries bounds the number of cached responses. When zero,
	// defaultETagCacheSize is used.
	MaxEntries int

	mu    sync.Mutex
	ll    *list.List
	items map[string]*list.Element
}

func (t *ETagTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}

	// Only cache GET requests, and never override a caller-supplied conditional
	// header.
	if req.Method != http.MethodGet || req.Header.Get(headers.IfNoneMatchHeader) != "" {
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
		if etag := resp.Header.Get(headers.ETagHeader); etag != "" {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			t.add(key, etagEntry{
				etag:   etag,
				status: resp.StatusCode,
				header: resp.Header.Clone(),
				body:   body,
			})
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
		}
	}

	return resp, nil
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

func (t *ETagTransport) add(key string, entry etagEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.items == nil {
		t.items = make(map[string]*list.Element)
		t.ll = list.New()
	}
	if el, ok := t.items[key]; ok {
		el.Value.(*lruItem).entry = entry
		t.ll.MoveToFront(el)
		return
	}
	el := t.ll.PushFront(&lruItem{key: key, entry: entry})
	t.items[key] = el

	max := t.MaxEntries
	if max <= 0 {
		max = defaultETagCacheSize
	}
	for t.ll.Len() > max {
		oldest := t.ll.Back()
		if oldest == nil {
			break
		}
		t.ll.Remove(oldest)
		delete(t.items, oldest.Value.(*lruItem).key)
	}
}
