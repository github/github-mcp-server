// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file is for SSE events.
// See https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events.

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
)

// If true, MemoryEventStore will do frequent validation to check invariants, slowing it down.
// Enable for debugging.
const validateMemoryEventStore = false

// An Event is a server-sent event.
// See https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#fields.
type Event struct {
	Name  string // the "event" field
	ID    string // the "id" field
	Data  []byte // the "data" field
	Retry string // the "retry" field
}

// Empty reports whether the Event is empty.
func (e Event) Empty() bool {
	return e.Name == "" && e.ID == "" && len(e.Data) == 0 && e.Retry == ""
}

// writeEvent writes the event to w, and flushes.
func writeEvent(w http.ResponseWriter, evt Event) (int, error) {
	var b bytes.Buffer
	if evt.Name != "" {
		fmt.Fprintf(&b, "event: %s\n", evt.Name)
	}
	if evt.ID != "" {
		fmt.Fprintf(&b, "id: %s\n", evt.ID)
	}
	if evt.Retry != "" {
		fmt.Fprintf(&b, "retry: %s\n", evt.Retry)
	}
	// Write the payload directly into a pre-grown buffer to avoid the extra
	// copy from string(evt.Data) and repeated buffer regrowths.
	b.Grow(len("data: \n\n") + len(evt.Data))
	b.WriteString("data: ")
	b.Write(evt.Data)
	b.WriteString("\n\n")
	n, err := w.Write(b.Bytes())
	rc := http.NewResponseController(w)
	// Ignore returned error as flushing is best-effort.
	_ = rc.Flush()
	return n, err
}

// scanEvents iterates SSE events in the given scanner. The iterated error is
// terminal: if encountered, the stream is corrupt or broken and should no
// longer be used.
//
// TODO(rfindley): consider a different API here that makes failure modes more
// apparent.
func scanEvents(r io.Reader) iter.Seq2[Event, error] {
	reader := bufio.NewReader(r)

	// TODO: investigate proper behavior when events are out of order, or have
	// non-standard names.
	var (
		eventKey = []byte("event")
		idKey    = []byte("id")
		dataKey  = []byte("data")
		retryKey = []byte("retry")
	)

	return func(yield func(Event, error) bool) {
		// iterate event from the wire.
		// https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#examples
		//
		//  - `key: value` line records.
		//  - Consecutive `data: ...` fields are joined with newlines.
		//  - Unrecognized fields are ignored. Since we only care about 'event', 'id', and
		//   'data', these are the only three we consider.
		//  - Lines starting with ":" are ignored.
		//  - Records are terminated with two consecutive newlines.
		var (
			evt     Event
			dataBuf *bytes.Buffer // if non-nil, preceding field was also data
		)
		yieldEvent := func() bool {
			if dataBuf != nil {
				evt.Data = dataBuf.Bytes()
				dataBuf = nil
			}
			if evt.Empty() {
				return true
			}
			if !yield(evt, nil) {
				return false
			}
			evt = Event{}
			return true
		}
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				yield(Event{}, fmt.Errorf("error reading event: %v", err))
				return
			}
			line = bytes.TrimRight(line, "\r\n")
			isEOF := errors.Is(err, io.EOF)

			if len(line) == 0 {
				if !yieldEvent() {
					return
				}
				if isEOF {
					return
				}
				continue
			}
			before, after, found := bytes.Cut(line, []byte{':'})
			if !found {
				yield(Event{}, fmt.Errorf("%w: malformed line in SSE stream: %q", errMalformedEvent, string(line)))
				return
			}
			switch {
			case bytes.Equal(before, eventKey):
				evt.Name = strings.TrimSpace(string(after))
			case bytes.Equal(before, idKey):
				evt.ID = strings.TrimSpace(string(after))
			case bytes.Equal(before, retryKey):
				evt.Retry = strings.TrimSpace(string(after))
			case bytes.Equal(before, dataKey):
				data := bytes.TrimSpace(after)
				if dataBuf == nil {
					dataBuf = new(bytes.Buffer)
				} else {
					dataBuf.WriteByte('\n')
				}
				dataBuf.Write(data)
			}

			if isEOF {
				yieldEvent()
				return
			}
		}
	}
}

// An EventStore tracks data for SSE streams.
// A single EventStore suffices for all sessions, since session IDs are
// globally unique. So one EventStore can be created per process, for
// all Servers in the process.
// Such a store is able to bound resource usage for the entire process.
//
// All of an EventStore's methods must be safe for use by multiple goroutines.
type EventStore interface {
	// Open is called when a new stream is created. It may be used to ensure that
	// the underlying data structure for the stream is initialized, making it
	// ready to store and replay event streams.
	Open(_ context.Context, sessionID, streamID string) error

	// Append appends data for an outgoing event to given stream, which is part of the
	// given session.
	Append(_ context.Context, sessionID, streamID string, data []byte) error

	// After returns an iterator over the data for the given session and stream, beginning
	// just after the given index.
	//
	// Once the iterator yields a non-nil error, it will stop.
	// After's iterator must return an error immediately if any data after index was
	// dropped; it must not return partial results.
	// The stream must have been opened previously (see [EventStore.Open]).
	After(_ context.Context, sessionID, streamID string, index int) iter.Seq2[[]byte, error]

	// SessionClosed informs the store that the given session is finished, along
	// with all of its streams.
	//
	// A store cannot rely on this method being called for cleanup. It should institute
	// additional mechanisms, such as timeouts, to reclaim storage.
	SessionClosed(_ context.Context, sessionID string) error

	// There is no StreamClosed method. A server doesn't know when a stream is finished, because
	// the client can always send a GET with a Last-Event-ID referring to the stream.
}

// A dataList is a list of []byte.
// The zero dataList is ready to use.
type dataList struct {
	size  int // total size of data bytes
	first int // the stream index of the first element in data
	data  [][]byte
}

func (dl *dataList) appendData(d []byte) {
	// Empty data consumes memory but doesn't increment size. However, it should
	// be rare.
	dl.data = append(dl.data, d)
	dl.size += len(d)
}

// removeFirst removes the first data item in dl, returning the size of the item.
// It panics if dl is empty.
func (dl *dataList) removeFirst() int {
	if len(dl.data) == 0 {
		panic("empty dataList")
	}
	r := len(dl.data[0])
	dl.size -= r
	dl.data[0] = nil // help GC
	dl.data = dl.data[1:]
	dl.first++
	return r
}

// A MemoryEventStore is an [EventStore] backed by memory.
type MemoryEventStore struct {
	mu       sync.Mutex
	maxBytes int                             // max total size of all data
	nBytes   int                             // current total size of all data
	store    map[string]map[string]*dataList // session ID -> stream ID -> *dataList
}

// MemoryEventStoreOptions are options for a [MemoryEventStore].
type MemoryEventStoreOptions struct{}

// MaxBytes returns the maximum number of bytes that the store will retain before
// purging data.
func (s *MemoryEventStore) MaxBytes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxBytes
}

// SetMaxBytes sets the maximum number of bytes the store will retain before purging
// data. The argument must not be negative. If it is zero, a suitable default will be used.
// SetMaxBytes can be called at any time. The size of the store will be adjusted
// immediately.
func (s *MemoryEventStore) SetMaxBytes(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case n < 0:
		panic("negative argument")
	case n == 0:
		s.maxBytes = defaultMaxBytes
	default:
		s.maxBytes = n
	}
	s.purge()
}

const defaultMaxBytes = 10 << 20 // 10 MiB

// NewMemoryEventStore creates a [MemoryEventStore] with the default value
// for MaxBytes.
func NewMemoryEventStore(opts *MemoryEventStoreOptions) *MemoryEventStore {
	return &MemoryEventStore{
		maxBytes: defaultMaxBytes,
		store:    make(map[string]map[string]*dataList),
	}
}

// Open implements [EventStore.Open]. It ensures that the underlying data
// structures for the given session are initialized and ready for use.
func (s *MemoryEventStore) Open(_ context.Context, sessionID, streamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.init(sessionID, streamID)
	return nil
}

// init is an internal helper function that ensures the nested map structure for a
// given sessionID and streamID exists, creating it if necessary. It returns the
// dataList associated with the specified IDs.
// Requires s.mu.
func (s *MemoryEventStore) init(sessionID, streamID string) *dataList {
	streamMap, ok := s.store[sessionID]
	if !ok {
		streamMap = make(map[string]*dataList)
		s.store[sessionID] = streamMap
	}
	dl, ok := streamMap[streamID]
	if !ok {
		dl = &dataList{}
		streamMap[streamID] = dl
	}
	return dl
}

// Append implements [EventStore.Append] by recording data in memory.
func (s *MemoryEventStore) Append(_ context.Context, sessionID, streamID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dl := s.init(sessionID, streamID)
	// Purge before adding, so at least the current data item will be present.
	// (That could result in nBytes > maxBytes, but we'll live with that.)
	s.purge()
	dl.appendData(data)
	s.nBytes += len(data)
	return nil
}

// ErrEventsPurged is the error that [EventStore.After] should return if the event just after the
// index is no longer available.
var ErrEventsPurged = errors.New("data purged")

// errMalformedEvent is returned when an SSE event cannot be parsed due to format violations.
// This is a hard error indicating corrupted data or protocol violations, as opposed to
// transient I/O errors which may be retryable.
var errMalformedEvent = errors.New("malformed event")

// After implements [EventStore.After].
func (s *MemoryEventStore) After(_ context.Context, sessionID, streamID string, index int) iter.Seq2[[]byte, error] {
	// Return the data items to yield.
	// We must copy, because dataList.removeFirst nils out slice elements.
	copyData := func() ([][]byte, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		streamMap, ok := s.store[sessionID]
		if !ok {
			return nil, fmt.Errorf("MemoryEventStore.After: unknown session ID %q", sessionID)
		}
		dl, ok := streamMap[streamID]
		if !ok {
			return nil, fmt.Errorf("MemoryEventStore.After: unknown stream ID %v in session %q", streamID, sessionID)
		}
		start := index + 1
		if dl.first > start {
			return nil, fmt.Errorf("MemoryEventStore.After: index %d, stream ID %v, session %q: %w",
				index, streamID, sessionID, ErrEventsPurged)
		}
		return slices.Clone(dl.data[start-dl.first:]), nil
	}

	return func(yield func([]byte, error) bool) {
		ds, err := copyData()
		if err != nil {
			yield(nil, err)
			return
		}
		for _, d := range ds {
			if !yield(d, nil) {
				return
			}
		}
	}
}

// SessionClosed implements [EventStore.SessionClosed].
func (s *MemoryEventStore) SessionClosed(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, dl := range s.store[sessionID] {
		s.nBytes -= dl.size
	}
	delete(s.store, sessionID)
	s.validate()
	return nil
}

// purge removes data until no more than s.maxBytes bytes are in use.
// It must be called with s.mu held.
func (s *MemoryEventStore) purge() {
	// Remove the first element of every dataList until below the max.
	for s.nBytes > s.maxBytes {
		changed := false
		for _, sm := range s.store {
			for _, dl := range sm {
				if dl.size > 0 {
					r := dl.removeFirst()
					// Even if we remove an empty chunk, that's
					// still progress. There may be non-empty
					// chunks after it.
					changed = true
					s.nBytes -= r
				}
			}
		}
		if !changed {
			panic("no progress during purge")
		}
	}
	s.validate()
}

// validate checks that the store's data structures are valid.
// It must be called with s.mu held.
func (s *MemoryEventStore) validate() {
	if !validateMemoryEventStore {
		return
	}
	// Check that we're accounting for the size correctly.
	n := 0
	for _, sm := range s.store {
		for _, dl := range sm {
			for _, d := range dl.data {
				n += len(d)
			}
		}
	}
	if n != s.nBytes {
		panic("sizes don't add up")
	}
}

// debugString returns a string containing the state of s.
// Used in tests.
func (s *MemoryEventStore) debugString() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for i, sess := range slices.Sorted(maps.Keys(s.store)) {
		if i > 0 {
			fmt.Fprintf(&b, "; ")
		}
		sm := s.store[sess]
		for i, sid := range slices.Sorted(maps.Keys(sm)) {
			if i > 0 {
				fmt.Fprintf(&b, "; ")
			}
			dl := sm[sid]
			fmt.Fprintf(&b, "%s %s first=%d", sess, sid, dl.first)
			for _, d := range dl.data {
				fmt.Fprintf(&b, " %s", d)
			}
		}
	}
	return b.String()
}
