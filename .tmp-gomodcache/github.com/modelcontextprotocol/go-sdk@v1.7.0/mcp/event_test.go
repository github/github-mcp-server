// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"context"
	"crypto/rand"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestScanEvents(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []Event
		wantErr string
	}{
		{
			name:  "simple event",
			input: "event: message\nid: 1\ndata: hello\n\n",
			want: []Event{
				{Name: "message", ID: "1", Data: []byte("hello")},
			},
		},
		{
			name:  "multiple data lines",
			input: "data: line 1\ndata: line 2\n\n",
			want: []Event{
				{Data: []byte("line 1\nline 2")},
			},
		},
		{
			name:  "multiple events",
			input: "data: first\n\nevent: second\ndata: second\n\n",
			want: []Event{
				{Data: []byte("first")},
				{Name: "second", Data: []byte("second")},
			},
		},
		{
			name:  "no trailing newline",
			input: "data: hello",
			want: []Event{
				{Data: []byte("hello")},
			},
		},
		{
			name:    "malformed line",
			input:   "invalid line\n\n",
			wantErr: "malformed line",
		},
		{
			name:  "message with 2 data lines and another event",
			input: "event: message\ndata: hello\ndata: hello\ndata: hello\n\nevent:keepalive",
			want: []Event{
				{Name: "message", Data: []byte("hello\nhello\nhello")},
				{Name: "keepalive"},
			},
		},
		{
			name:  "event with multiple lines",
			input: "event: message\ndata: hello\ndata: hello\ndata: hello\nid:1",
			want: []Event{
				{Name: "message", ID: "1", Data: []byte("hello\nhello\nhello")},
			},
		},
		{
			name: "multiple events, out of order keys",
			input: strings.Join([]string{
				"event:message",
				"data: hello0",
				"\n",
				"data: hello1",
				"data: hello1",
				"id:1",
				"event:message",
				"\n",
				"event:message",
				"data: hello3",
				"data: hello3",
				"id:3",
				"\n",
				"data: hello4",
				"data: hello4",
				"id:4",
				"event:message",
			}, "\n"),
			want: []Event{
				{Name: "message", Data: []byte("hello0")},
				{Name: "message", ID: "1", Data: []byte("hello1\nhello1")},
				{Name: "message", ID: "3", Data: []byte("hello3\nhello3")},
				{Name: "message", ID: "4", Data: []byte("hello4\nhello4")},
			},
		},
		{
			name:  "non-continuous data items in the event",
			input: "event: foo\ndata: 123\nretry: 5\ndata: 456",
			want: []Event{
				{Name: "foo", Data: []byte("123\n456"), Retry: "5"},
			},
		},
		{
			name:  "no-data events",
			input: "event: foo\n\nevent: bar",
			want: []Event{
				{Name: "foo"},
				{Name: "bar"},
			},
		},
		{
			name:  "empty data event",
			input: "event: foo\ndata:\n\nevent: bar",
			want: []Event{
				{Name: "foo"},
				{Name: "bar"},
			},
		},
		{

			name:    "malformed data event",
			input:   "someline",
			wantErr: "malformed event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			var got []Event
			var err error
			for e, err2 := range scanEvents(r) {
				if err2 != nil {
					err = err2
					break
				}
				got = append(got, e)
			}

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("scanEvents() got nil error, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("scanEvents() error = %q, want containing %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("scanEvents() returned unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("scanEvents() got %d events, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if g, w := got[i].Name, tt.want[i].Name; g != w {
					t.Errorf("event %d: name = %q, want %q", i, g, w)
				}
				if g, w := got[i].ID, tt.want[i].ID; g != w {
					t.Errorf("event %d: id = %q, want %q", i, g, w)
				}
				if g, w := string(got[i].Data), string(tt.want[i].Data); g != w {
					t.Errorf("event %d: data = %q, want %q", i, g, w)
				}
			}
		})
	}
}

func TestMemoryEventStoreState(t *testing.T) {
	ctx := context.Background()

	appendEvent := func(s *MemoryEventStore, sess, stream string, data string) {
		if err := s.Append(ctx, sess, stream, []byte(data)); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range []struct {
		name     string
		actions  func(*MemoryEventStore)
		want     string // output of debugString
		wantSize int    // value of nBytes
	}{
		{
			"appends",
			func(s *MemoryEventStore) {
				appendEvent(s, "S1", "1", "d1")
				appendEvent(s, "S1", "2", "d2")
				appendEvent(s, "S1", "1", "d3")
				appendEvent(s, "S2", "8", "d4")
			},
			"S1 1 first=0 d1 d3; S1 2 first=0 d2; S2 8 first=0 d4",
			8,
		},
		{
			"session close",
			func(s *MemoryEventStore) {
				appendEvent(s, "S1", "1", "d1")
				appendEvent(s, "S1", "2", "d2")
				appendEvent(s, "S1", "1", "d3")
				appendEvent(s, "S2", "8", "d4")
				s.SessionClosed(ctx, "S1")
			},
			"S2 8 first=0 d4",
			2,
		},
		{
			"purge",
			func(s *MemoryEventStore) {
				appendEvent(s, "S1", "1", "d1")
				appendEvent(s, "S1", "2", "d2")
				appendEvent(s, "S1", "1", "d3")
				appendEvent(s, "S2", "8", "d4")
				// We are using 8 bytes (d1,d2, d3, d4).
				// To purge 6, we remove the first of each stream, leaving only d3.
				s.SetMaxBytes(2)
			},
			// The other streams remain, because we may add to them.
			"S1 1 first=1 d3; S1 2 first=1; S2 8 first=1",
			2,
		},
		{
			"purge append",
			func(s *MemoryEventStore) {
				appendEvent(s, "S1", "1", "d1")
				appendEvent(s, "S1", "2", "d2")
				appendEvent(s, "S1", "1", "d3")
				appendEvent(s, "S2", "8", "d4")
				s.SetMaxBytes(2)
				// Up to here, identical to the "purge" case.
				// Each of these additions will result in a purge.
				appendEvent(s, "S1", "2", "d5") // remove d3
				appendEvent(s, "S1", "2", "d6") // remove d5
			},
			"S1 1 first=2; S1 2 first=2 d6; S2 8 first=1",
			2,
		},
		{
			"purge resize append",
			func(s *MemoryEventStore) {
				appendEvent(s, "S1", "1", "d1")
				appendEvent(s, "S1", "2", "d2")
				appendEvent(s, "S1", "1", "d3")
				appendEvent(s, "S2", "8", "d4")
				s.SetMaxBytes(2)
				// Up to here, identical to the "purge" case.
				s.SetMaxBytes(6) // make room
				appendEvent(s, "S1", "2", "d5")
				appendEvent(s, "S1", "2", "d6")
			},
			// The other streams remain, because we may add to them.
			"S1 1 first=1 d3; S1 2 first=1 d5 d6; S2 8 first=1",
			6,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := NewMemoryEventStore(nil)
			tt.actions(s)
			got := s.debugString()
			if got != tt.want {
				t.Errorf("\ngot  %s\nwant %s", got, tt.want)
			}
			if g, w := s.nBytes, tt.wantSize; g != w {
				t.Errorf("got size %d, want %d", g, w)
			}
		})
	}
}

func TestMemoryEventStoreAfter(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEventStore(nil)
	s.SetMaxBytes(4)
	s.Append(ctx, "S1", "1", []byte("d1"))
	s.Append(ctx, "S1", "1", []byte("d2"))
	s.Append(ctx, "S1", "1", []byte("d3"))
	s.Append(ctx, "S1", "2", []byte("d4")) // will purge d1
	want := "S1 1 first=1 d2 d3; S1 2 first=0 d4"
	if got := s.debugString(); got != want {
		t.Fatalf("got state %q, want %q", got, want)
	}

	for _, tt := range []struct {
		sessionID string
		streamID  string
		index     int
		want      []string
		wantErr   string // if non-empty, error should contain this string
	}{
		{"S1", "1", 0, []string{"d2", "d3"}, ""},
		{"S1", "1", 1, []string{"d3"}, ""},
		{"S1", "1", 2, nil, ""},
		{"S1", "2", 0, nil, ""},
		{"S1", "3", 0, nil, "unknown stream ID"},
		{"S2", "0", 0, nil, "unknown session ID"},
	} {
		t.Run(fmt.Sprintf("%s-%s-%d", tt.sessionID, tt.streamID, tt.index), func(t *testing.T) {
			var got []string
			for d, err := range s.After(ctx, tt.sessionID, tt.streamID, tt.index) {
				if err != nil {
					if tt.wantErr == "" {
						t.Fatalf("unexpected error %q", err)
					} else if g := err.Error(); !strings.Contains(g, tt.wantErr) {
						t.Fatalf("got error %q, want it to contain %q", g, tt.wantErr)
					} else {
						return
					}
				}
				got = append(got, string(d))
			}
			if tt.wantErr != "" {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryEventStorePurgeEmpty(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryEventStore(nil)
	s.SetMaxBytes(100)

	// Append an empty chunk first.
	if err := s.Append(ctx, "S1", "1", []byte("")); err != nil {
		t.Fatal(err)
	}
	// Append a non-empty chunk.
	if err := s.Append(ctx, "S1", "1", []byte("1234567890")); err != nil {
		t.Fatal(err)
	}
	// Now nBytes = 10, but the first chunk is empty.

	// This should not panic.
	s.SetMaxBytes(5)

	if s.nBytes > 5 {
		t.Errorf("got nBytes %d, want <= 5", s.nBytes)
	}
}

func BenchmarkMemoryEventStore(b *testing.B) {
	// Benchmark with various settings for event store size, number of session,
	// and payload size.
	//
	// Assume a small number of streams per session, which is probably realistic.
	tests := []struct {
		name     string
		limit    int
		sessions int
		datasize int
	}{
		{"1KB", 1024, 1, 16},
		{"1MB", 1024 * 1024, 10, 16},
		{"10MB", 10 * 1024 * 1024, 100, 16},
		{"10MB_big", 10 * 1024 * 1024, 1000, 128},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			store := NewMemoryEventStore(nil)
			store.SetMaxBytes(test.limit)
			ctx := context.Background()
			sessionIDs := make([]string, test.sessions)
			streamIDs := make([][3]string, test.sessions)
			for i := range sessionIDs {
				sessionIDs[i] = fmt.Sprint(i)
				for j := range 3 {
					streamIDs[i][j] = rand.Text()
				}
			}
			payload := make([]byte, test.datasize)
			start := time.Now()
			b.ResetTimer()
			for i := range b.N {
				sessionID := sessionIDs[i%len(sessionIDs)]
				streamID := streamIDs[i%len(sessionIDs)][i%3]
				store.Append(ctx, sessionID, streamID, payload)
			}
			b.ReportMetric(float64(test.datasize)*float64(b.N)/time.Since(start).Seconds(), "bytes/s")
		})
	}
}
