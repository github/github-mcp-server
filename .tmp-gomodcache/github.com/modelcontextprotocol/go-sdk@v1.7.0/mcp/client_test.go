// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//lint:file-ignore SA1019 tests exercise deprecated SEP-2577 APIs (roots, sampling, logging).

package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

type Item struct {
	Name  string
	Value string
}

type ListTestParams struct {
	Cursor string
}

func (p *ListTestParams) cursorPtr() *string {
	return &p.Cursor
}

type ListTestResult struct {
	Items      []*Item
	NextCursor string
}

func (r *ListTestResult) nextCursorPtr() *string {
	return &r.NextCursor
}

var allItems = []*Item{
	{"alpha", "val-A"},
	{"bravo", "val-B"},
	{"charlie", "val-C"},
	{"delta", "val-D"},
	{"echo", "val-E"},
	{"foxtrot", "val-F"},
	{"golf", "val-G"},
	{"hotel", "val-H"},
	{"india", "val-I"},
	{"juliet", "val-J"},
	{"kilo", "val-K"},
}

// generatePaginatedResults is a helper to create a sequence of mock responses for pagination.
// It simulates a server returning items in pages based on a given page size.
func generatePaginatedResults(all []*Item, pageSize int) []*ListTestResult {
	if len(all) == 0 {
		return []*ListTestResult{{Items: []*Item{}, NextCursor: ""}}
	}
	if pageSize <= 0 {
		panic("pageSize must be greater than 0")
	}
	numPages := (len(all) + pageSize - 1) / pageSize // Ceiling division
	var results []*ListTestResult
	for i := range numPages {
		startIndex := i * pageSize
		endIndex := min(startIndex+pageSize, len(all)) // Use min to prevent out of bounds
		nextCursor := ""
		if endIndex < len(all) { // If there are more items after this page
			nextCursor = fmt.Sprintf("cursor_%d", endIndex)
		}
		results = append(results, &ListTestResult{Items: all[startIndex:endIndex], NextCursor: nextCursor})
	}
	return results
}

func TestClientPaginateBasic(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name          string
		results       []*ListTestResult
		mockError     error
		initialParams *ListTestParams
		expected      []*Item
		expectError   bool
	}{
		{
			name:     "SinglePageAllItems",
			results:  generatePaginatedResults(allItems, len(allItems)),
			expected: allItems,
		},
		{
			name:     "MultiplePages",
			results:  generatePaginatedResults(allItems, 3),
			expected: allItems,
		},
		{
			name:     "EmptyResults",
			results:  generatePaginatedResults([]*Item{}, 10),
			expected: nil,
		},
		{
			name:        "ListFuncReturnsErrorImmediately",
			results:     []*ListTestResult{{}},
			mockError:   fmt.Errorf("API error on first call"),
			expected:    nil,
			expectError: true,
		},
		{
			name:          "InitialCursorProvided",
			initialParams: &ListTestParams{Cursor: "cursor_2"},
			results:       generatePaginatedResults(allItems[2:], 3),
			expected:      allItems[2:],
		},
		{
			name:          "CursorBeyondAllItems",
			initialParams: &ListTestParams{Cursor: "cursor_999"},
			results:       []*ListTestResult{{Items: []*Item{}, NextCursor: ""}},
			expected:      nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			listFunc := func(ctx context.Context, params *ListTestParams) (*ListTestResult, error) {
				if len(tc.results) == 0 {
					t.Fatalf("listFunc called but no more results defined for test case %q", tc.name)
				}
				res := tc.results[0]
				tc.results = tc.results[1:]
				var err error
				if tc.mockError != nil {
					err = tc.mockError
				}
				return res, err
			}

			params := tc.initialParams
			if tc.initialParams == nil {
				params = &ListTestParams{}
			}

			var gotItems []*Item
			var iterationErr error
			seq := paginate(ctx, params, listFunc, func(r *ListTestResult) []*Item { return r.Items })
			for item, err := range seq {
				if err != nil {
					iterationErr = err
					break
				}
				gotItems = append(gotItems, item)
			}
			if tc.expectError {
				if iterationErr == nil {
					t.Errorf("paginate() expected an error during iteration, but got none")
				}
			} else {
				if iterationErr != nil {
					t.Errorf("paginate() got: %v, want: nil", iterationErr)
				}
			}
			if diff := cmp.Diff(tc.expected, gotItems, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
				t.Fatalf("paginate() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestClientLogger(t *testing.T) {
	// Case 1: No logger provided
	c1 := NewClient(&Implementation{Name: "test", Version: "1.0"}, nil)
	if c1.opts.Logger == nil {
		t.Error("expected default logger, got nil")
	}

	// Case 2: Logger provided
	logger := slog.Default()
	c2 := NewClient(&Implementation{Name: "test", Version: "1.0"}, &ClientOptions{
		Logger: logger,
	})
	if c2.opts.Logger != logger {
		t.Error("expected provided logger, got different one")
	}
}

func TestClientPaginateVariousPageSizes(t *testing.T) {
	ctx := context.Background()
	for i := 1; i < len(allItems)+1; i++ {
		testname := fmt.Sprintf("PageSize=%d", i)
		t.Run(testname, func(t *testing.T) {
			results := generatePaginatedResults(allItems, i)
			listFunc := func(ctx context.Context, params *ListTestParams) (*ListTestResult, error) {
				res := results[0]
				results = results[1:]
				return res, nil
			}
			var gotItems []*Item
			seq := paginate(ctx, &ListTestParams{}, listFunc, func(r *ListTestResult) []*Item { return r.Items })
			for item, err := range seq {
				if err != nil {
					t.Fatalf("paginate() unexpected error during iteration: %v", err)
				}
				gotItems = append(gotItems, item)
			}
			if diff := cmp.Diff(allItems, gotItems, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
				t.Fatalf("paginate() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestClientCapabilities(t *testing.T) {
	testCases := []struct {
		name             string
		configureClient  func(s *Client)
		clientOpts       ClientOptions
		protocolVersion  string // defaults to latestProtocolVersion if empty
		wantCapabilities *ClientCapabilities
	}{
		{
			name:            "default",
			configureClient: func(s *Client) {},
			wantCapabilities: &ClientCapabilities{
				Roots:   RootCapabilities{ListChanged: true},
				RootsV2: &RootCapabilities{ListChanged: true},
			},
		},
		{
			name:            "with sampling",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
					return nil, nil
				},
			},
			wantCapabilities: &ClientCapabilities{
				Roots:    RootCapabilities{ListChanged: true},
				RootsV2:  &RootCapabilities{ListChanged: true},
				Sampling: &SamplingCapabilities{},
			},
		},
		{
			name:            "with elicitation",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
					return nil, nil
				},
			},
			protocolVersion: protocolVersion20251125,
			wantCapabilities: &ClientCapabilities{
				Roots:   RootCapabilities{ListChanged: true},
				RootsV2: &RootCapabilities{ListChanged: true},
				Elicitation: &ElicitationCapabilities{
					Form: &FormElicitationCapabilities{},
				},
			},
		},
		{
			name:            "with elicitation (old protocol)",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
					return nil, nil
				},
			},
			protocolVersion: protocolVersion20250618,
			wantCapabilities: &ClientCapabilities{
				Roots:       RootCapabilities{ListChanged: true},
				RootsV2:     &RootCapabilities{ListChanged: true},
				Elicitation: &ElicitationCapabilities{},
			},
		},
		{
			name:            "with URL elicitation",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				Capabilities: &ClientCapabilities{
					Roots:   RootCapabilities{ListChanged: true},
					RootsV2: &RootCapabilities{ListChanged: true},
					Elicitation: &ElicitationCapabilities{
						URL: &URLElicitationCapabilities{},
					},
				},
				ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
					return nil, nil
				},
			},
			wantCapabilities: &ClientCapabilities{
				Roots:   RootCapabilities{ListChanged: true},
				RootsV2: &RootCapabilities{ListChanged: true},
				Elicitation: &ElicitationCapabilities{
					URL: &URLElicitationCapabilities{},
				},
			},
		},
		{
			name:            "with form and URL elicitation",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				Capabilities: &ClientCapabilities{
					Roots:   RootCapabilities{ListChanged: true},
					RootsV2: &RootCapabilities{ListChanged: true},
					Elicitation: &ElicitationCapabilities{
						Form: &FormElicitationCapabilities{},
						URL:  &URLElicitationCapabilities{},
					},
				},
				ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
					return nil, nil
				},
			},
			wantCapabilities: &ClientCapabilities{
				Roots:   RootCapabilities{ListChanged: true},
				RootsV2: &RootCapabilities{ListChanged: true},
				Elicitation: &ElicitationCapabilities{
					Form: &FormElicitationCapabilities{},
					URL:  &URLElicitationCapabilities{},
				},
			},
		},
		{
			name:            "no capabilities",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				Capabilities: &ClientCapabilities{},
			},
			wantCapabilities: &ClientCapabilities{},
		},
		{
			name:            "no roots",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				Capabilities: &ClientCapabilities{
					Sampling: &SamplingCapabilities{},
				},
			},
			wantCapabilities: &ClientCapabilities{
				Sampling: &SamplingCapabilities{},
			},
		},
		{
			name:            "roots-no list",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				Capabilities: &ClientCapabilities{
					RootsV2: &RootCapabilities{ListChanged: false},
				},
			},
			wantCapabilities: &ClientCapabilities{
				RootsV2: &RootCapabilities{ListChanged: false},
			},
		},
		{
			name:            "custom capabilities with sampling",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				Capabilities: &ClientCapabilities{
					RootsV2: &RootCapabilities{ListChanged: true},
				},
				CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
					return nil, nil
				},
			},
			wantCapabilities: &ClientCapabilities{
				Roots:    RootCapabilities{ListChanged: true},
				RootsV2:  &RootCapabilities{ListChanged: true},
				Sampling: &SamplingCapabilities{},
			},
		},
		{
			name:            "elicitation override",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				Capabilities: &ClientCapabilities{
					Elicitation: &ElicitationCapabilities{
						URL: &URLElicitationCapabilities{},
					},
				},
				ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
					return nil, nil
				},
			},
			wantCapabilities: &ClientCapabilities{
				Elicitation: &ElicitationCapabilities{
					URL: &URLElicitationCapabilities{},
				},
			},
		},
		{
			name:            "custom capabilities with experimental",
			configureClient: func(s *Client) {},
			clientOpts: ClientOptions{
				Capabilities: &ClientCapabilities{
					Experimental: map[string]any{"custom": "value"},
					RootsV2:      &RootCapabilities{ListChanged: true},
				},
			},
			wantCapabilities: &ClientCapabilities{
				Experimental: map[string]any{"custom": "value"},
				Roots:        RootCapabilities{ListChanged: true},
				RootsV2:      &RootCapabilities{ListChanged: true},
			},
		},
		{
			name:            "extensions preserved",
			configureClient: func(s *Client) {},
			clientOpts: func() ClientOptions {
				caps := &ClientCapabilities{
					RootsV2: &RootCapabilities{ListChanged: true},
				}
				caps.AddExtension("io.example/ext1", map[string]any{"key": "value"})
				caps.AddExtension("io.example/ext2", nil)
				return ClientOptions{Capabilities: caps}
			}(),
			wantCapabilities: &ClientCapabilities{
				Extensions: map[string]any{
					"io.example/ext1": map[string]any{"key": "value"},
					"io.example/ext2": map[string]any{},
				},
				Roots:   RootCapabilities{ListChanged: true},
				RootsV2: &RootCapabilities{ListChanged: true},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(testImpl, &tc.clientOpts)
			tc.configureClient(client)
			protocolVersion := tc.protocolVersion
			if protocolVersion == "" {
				protocolVersion = latestProtocolVersion
			}
			gotCapabilities := client.capabilities(protocolVersion)
			if diff := cmp.Diff(tc.wantCapabilities, gotCapabilities); diff != "" {
				t.Errorf("capabilities() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLookupTool(t *testing.T) {
	tool1 := &Tool{Name: "tool1", Description: "first"}
	tool2 := &Tool{Name: "tool2", Description: "second"}
	tool1Updated := &Tool{Name: "tool1", Description: "updated"}

	// page represents a single cached ListToolsResult, keyed by cursor.
	type page struct {
		cursor string
		tools  []*Tool
	}

	testCases := []struct {
		name   string
		pages  []page
		lookup string
		want   *Tool
	}{
		{
			name:   "empty cache",
			lookup: "tool1",
			want:   nil,
		},
		{
			name:   "single tool found",
			pages:  []page{{cursor: "", tools: []*Tool{tool1}}},
			lookup: "tool1",
			want:   tool1,
		},
		{
			name:   "unknown tool",
			pages:  []page{{cursor: "", tools: []*Tool{tool1}}},
			lookup: "nonexistent",
			want:   nil,
		},
		{
			name:   "multiple tools single page",
			pages:  []page{{cursor: "", tools: []*Tool{tool1, tool2}}},
			lookup: "tool2",
			want:   tool2,
		},
		{
			name: "tool found across paginated pages",
			pages: []page{
				{cursor: "", tools: []*Tool{tool1}},
				{cursor: "page2", tools: []*Tool{tool2}},
			},
			lookup: "tool2",
			want:   tool2,
		},
		{
			name: "re-list same cursor overwrites entry",
			pages: []page{
				{cursor: "", tools: []*Tool{tool1}},
				{cursor: "", tools: []*Tool{tool1Updated}},
			},
			lookup: "tool1",
			want:   tool1Updated,
		},
		{
			name:   "empty page no-op",
			pages:  []page{{cursor: "", tools: []*Tool{}}},
			lookup: "tool1",
			want:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cs := &ClientSession{}
			for _, p := range tc.pages {
				cs.toolsCache.put(p.cursor, &ListToolsResult{
					Cacheable: Cacheable{TTLMs: 60_000},
					Tools:     p.tools,
				})
			}
			got := cs.lookupTool(tc.lookup)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("lookupTool(%q) mismatch (-want +got):\n%s", tc.lookup, diff)
			}
		})
	}
}

func TestClientCapabilitiesOverWire(t *testing.T) {
	testCases := []struct {
		name             string
		clientOpts       *ClientOptions
		wantCapabilities *ClientCapabilities
	}{
		{
			name:       "Default capabilities",
			clientOpts: nil,
			wantCapabilities: &ClientCapabilities{
				Roots:   RootCapabilities{ListChanged: true},
				RootsV2: &RootCapabilities{ListChanged: true},
			},
		},
		{
			name: "Custom Capabilities with roots listChanged false",
			clientOpts: &ClientOptions{
				Capabilities: &ClientCapabilities{
					RootsV2: &RootCapabilities{ListChanged: false},
				},
			},
			wantCapabilities: &ClientCapabilities{
				Roots:   RootCapabilities{ListChanged: false},
				RootsV2: &RootCapabilities{ListChanged: false},
			},
		},
		{
			name: "Dynamic sampling capability",
			clientOpts: &ClientOptions{
				Capabilities: &ClientCapabilities{
					RootsV2: &RootCapabilities{ListChanged: true},
				},
				CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
					return nil, nil
				},
			},
			wantCapabilities: &ClientCapabilities{
				Roots:    RootCapabilities{ListChanged: true},
				RootsV2:  &RootCapabilities{ListChanged: true},
				Sampling: &SamplingCapabilities{},
			},
		},
		{
			name: "Empty capabilities disables defaults",
			clientOpts: &ClientOptions{
				Capabilities: &ClientCapabilities{},
			},
			wantCapabilities: &ClientCapabilities{},
		},
		{
			name: "Extensions over wire",
			clientOpts: func() *ClientOptions {
				caps := &ClientCapabilities{
					RootsV2: &RootCapabilities{ListChanged: true},
				}
				caps.AddExtension("io.example/ext", map[string]any{"key": "value"})
				return &ClientOptions{Capabilities: caps}
			}(),
			wantCapabilities: &ClientCapabilities{
				Extensions: map[string]any{
					"io.example/ext": map[string]any{"key": "value"},
				},
				Roots:   RootCapabilities{ListChanged: true},
				RootsV2: &RootCapabilities{ListChanged: true},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Create client.
			impl := &Implementation{Name: "testClient", Version: "v1.0.0"}
			client := NewClient(impl, tc.clientOpts)

			// Connect client and server.
			cTransport, sTransport := NewInMemoryTransports()
			server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
			ss, err := server.Connect(ctx, sTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()

			cs, err := client.Connect(ctx, cTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()

			// Check that the server received the expected capabilities.
			initParams := ss.InitializeParams()
			if initParams == nil {
				t.Fatal("InitializeParams is nil")
			}

			if diff := cmp.Diff(tc.wantCapabilities, initParams.Capabilities); diff != "" {
				t.Errorf("Capabilities mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestClientConnectDiscover exercises the SEP-2575 server/discover probe that
// Client.Connect now sends before falling back to the legacy initialize
// handshake.
//
// Each subtest installs a server-side receiving middleware that intercepts the
// "server/discover" method and returns a canned response: a successful
// DiscoverResult, a "Method not found" error, an UnsupportedProtocolVersion
// error, an unrelated failure, or a DiscoverResult whose supportedVersions
// don't overlap with the SDK. The test then asserts the resulting session
// state and whether the legacy initialize handshake ran.
func TestClientConnectDiscover(t *testing.T) {
	const otherVersionsOnly = "1999-01-01"

	tests := []struct {
		name string
		// discoverHandler decides how the server replies to server/discover.
		// Returning (nil, nil) means "let the default stub handle it" (which
		// returns ErrMethodNotFound).
		discoverHandler func() (Result, error)
		// wantInitialize is true if the legacy initialize handshake should
		// have run (i.e. discover signaled "not supported").
		wantInitialize bool
		// wantVersion is the protocol version expected to end up on
		// ClientSession.state.InitializeResult after Connect returns.
		wantVersion string
	}{
		{
			name: "discover success skips initialize",
			discoverHandler: func() (Result, error) {
				return &DiscoverResult{
					Meta: Meta{
						MetaKeyServerInfo: &Implementation{Name: "discoverServer", Version: "v1.0.0"},
					},
					SupportedVersions: []string{protocolVersion20260728},
					Capabilities: &ServerCapabilities{
						Tools: &ToolCapabilities{ListChanged: true},
					},
				}, nil
			},
			wantInitialize: false,
			wantVersion:    protocolVersion20260728,
		},
		{
			name: "method not found falls back to initialize",
			discoverHandler: func() (Result, error) {
				return nil, jsonrpc2.ErrMethodNotFound
			},
			wantInitialize: true,
			wantVersion:    protocolVersion20251125,
		},
		{
			name: "unsupported protocol version falls back to initialize",
			discoverHandler: func() (Result, error) {
				return nil, &jsonrpc.Error{
					Code:    CodeUnsupportedProtocolVersion,
					Message: "unsupported protocol version",
				}
			},
			wantInitialize: true,
			wantVersion:    protocolVersion20251125,
		},
		{
			name: "no overlapping supported version falls back to initialize",
			discoverHandler: func() (Result, error) {
				return &DiscoverResult{
					Meta: Meta{
						MetaKeyServerInfo: &Implementation{Name: "discoverServer", Version: "v1.0.0"},
					},
					SupportedVersions: []string{otherVersionsOnly},
					Capabilities:      &ServerCapabilities{},
				}, nil
			},
			wantInitialize: true,
			wantVersion:    protocolVersion20251125,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			var (
				gotDiscover   atomic.Bool
				gotInitialize atomic.Bool
			)

			s := NewServer(testImpl, nil)
			s.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
				return func(ctx context.Context, method string, req Request) (Result, error) {
					switch method {
					case methodDiscover:
						gotDiscover.Store(true)
						return tc.discoverHandler()
					case methodInitialize:
						gotInitialize.Store(true)
					}
					return next(ctx, method, req)
				}
			})

			ct, st := NewInMemoryTransports()
			ss, err := s.Connect(ctx, st, nil)
			if err != nil {
				t.Fatalf("server Connect: %v", err)
			}
			defer ss.Close()

			c := NewClient(testImpl, nil)
			cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			defer cs.Close()

			if !gotDiscover.Load() {
				t.Error("server did not receive server/discover")
			}
			if got, want := gotInitialize.Load(), tc.wantInitialize; got != want {
				t.Errorf("initialize invoked = %v, want %v", got, want)
			}
			ir := cs.InitializeResult()
			if ir == nil {
				t.Fatal("InitializeResult is nil after Connect")
			}
			if got, want := ir.ProtocolVersion, tc.wantVersion; got != want {
				t.Errorf("InitializeResult.ProtocolVersion = %q, want %q", got, want)
			}
		})
	}
}

// TestClientConnectDiscover_RequestContents verifies that the server/discover
// request sent by Client.Connect carries the SEP-2575 per-request _meta triple:
// protocolVersion, clientInfo, and clientCapabilities.
func TestClientConnectDiscover_RequestContents(t *testing.T) {
	ctx := context.Background()

	type captured struct {
		params *DiscoverParams
	}
	var got captured

	s := NewServer(testImpl, nil)
	s.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method == methodDiscover {
				sr, ok := req.(*ServerRequest[*DiscoverParams])
				if !ok {
					t.Errorf("discover req has unexpected type %T", req)
					return nil, jsonrpc2.ErrMethodNotFound
				}
				got.params = sr.Params
				// Make discover "not supported" so Connect proceeds (we only
				// care about the discover request payload here).
				return nil, jsonrpc2.ErrMethodNotFound
			}
			return next(ctx, method, req)
		}
	})

	ct, st := NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	defer ss.Close()

	clientImpl := &Implementation{Name: "discover-probe-client", Version: "v9.9.9"}
	c := NewClient(clientImpl, &ClientOptions{
		CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
			return nil, nil
		},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer cs.Close()

	if got.params == nil {
		t.Fatal("server did not receive server/discover")
	}

	meta := got.params.GetMeta()
	if v, _ := meta[MetaKeyProtocolVersion].(string); v != protocolVersion20260728 {
		t.Errorf("_meta[%s] = %q, want %q", MetaKeyProtocolVersion, v, protocolVersion20260728)
	}
	// _meta values round-trip through JSON, so on the server side they
	// arrive as map[string]any rather than the typed Go pointers we sent.
	info, ok := meta[MetaKeyClientInfo].(map[string]any)
	if !ok {
		t.Fatalf("_meta[%s] = %T, want map[string]any", MetaKeyClientInfo, meta[MetaKeyClientInfo])
	}
	if got, want := info["name"], any(clientImpl.Name); got != want {
		t.Errorf("clientInfo.name = %v, want %v", got, want)
	}
	caps, ok := meta[MetaKeyClientCapabilities].(map[string]any)
	if !ok {
		t.Fatalf("_meta[%s] = %T, want map[string]any", MetaKeyClientCapabilities, meta[MetaKeyClientCapabilities])
	}
	if _, ok := caps["sampling"]; !ok {
		t.Errorf("clientCapabilities.sampling missing (CreateMessageHandler was set); got %v", caps)
	}
}

// TestInMemory_E2E_DiscoverSuccess is a full end-to-end smoke test for
// SEP-2575 over a non-HTTP transport.
func TestInMemory_E2E_DiscoverSuccess(t *testing.T) {
	ctx := context.Background()

	server := NewServer(&Implementation{Name: "stdio-like-server", Version: "v1"}, nil)
	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()

	client := NewClient(&Implementation{Name: "stdio-like-client", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	ir := cs.InitializeResult()
	if ir == nil {
		t.Fatal("InitializeResult is nil; discover should have populated it")
	}
	if ir.ProtocolVersion != protocolVersion20260728 {
		t.Errorf("InitializeResult.ProtocolVersion = %q, want %q (negotiated via discover, no initialize)",
			ir.ProtocolVersion, protocolVersion20260728)
	}
	if ir.ServerInfo == nil || ir.ServerInfo.Name != "stdio-like-server" {
		t.Errorf("InitializeResult.ServerInfo = %+v, want name=stdio-like-server", ir.ServerInfo)
	}

	// Prove the session is usable.
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Errorf("ListTools after discover: %v", err)
	}
}

// TestInMemory_E2E_DiscoverFallback_NoOverlap verifies the fallback path
// over an InMemory (STDIO-equivalent) transport: the client probes with
// _meta.protocolVersion = 2026-07-28, but the server overrides discover via
// middleware to advertise only legacy versions, so the client must fall
// back to the legacy initialize handshake.
func TestInMemory_E2E_DiscoverFallback_NoOverlap(t *testing.T) {
	ctx := context.Background()
	server := NewServer(&Implementation{Name: "vpre-like-server", Version: "v1"}, nil)
	// Intercept discover and reply as if we were a server that only
	// supports legacy versions.
	server.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method == methodDiscover {
				return &DiscoverResult{
					Meta: Meta{
						MetaKeyServerInfo: &Implementation{Name: "vpre-like-server", Version: "v1"},
					},
					SupportedVersions: []string{protocolVersion20251125},
					Capabilities:      &ServerCapabilities{},
				}, nil
			}
			return next(ctx, method, req)
		}
	})

	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()

	client := NewClient(&Implementation{Name: "new-client", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	ir := cs.InitializeResult()
	if ir == nil {
		t.Fatal("InitializeResult is nil after fallback initialize")
	}
	// The fallback initialize explicitly requests protocolVersion20251125
	// (see client.go), so the server negotiates that version.
	if ir.ProtocolVersion != protocolVersion20251125 {
		t.Errorf("InitializeResult.ProtocolVersion = %q, want %q (legacy fallback after no-overlap discover)",
			ir.ProtocolVersion, protocolVersion20251125)
	}

	// Prove the session is usable after fallback.
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Errorf("ListTools after fallback initialize: %v", err)
	}
}

// TestInMemory_E2E_DiscoverFallback_MethodNotFound verifies the fallback
// path over InMemory when the server doesn't know about server/discover at
// all (simulating a true pre-SEP-2575 server).
func TestInMemory_E2E_DiscoverFallback_MethodNotFound(t *testing.T) {
	ctx := context.Background()

	server := NewServer(&Implementation{Name: "vpre-server", Version: "v1"}, nil)
	server.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method == methodDiscover {
				return nil, jsonrpc2.ErrMethodNotFound
			}
			return next(ctx, method, req)
		}
	})

	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()

	client := NewClient(&Implementation{Name: "new-client", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	ir := cs.InitializeResult()
	if ir == nil {
		t.Fatal("InitializeResult is nil after fallback initialize")
	}
	// The fallback initialize explicitly requests protocolVersion20251125
	// (see client.go), so the server negotiates that version.
	if ir.ProtocolVersion != protocolVersion20251125 {
		t.Errorf("InitializeResult.ProtocolVersion = %q, want %q (legacy fallback after MethodNotFound)",
			ir.ProtocolVersion, protocolVersion20251125)
	}

	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Errorf("ListTools after fallback initialize: %v", err)
	}
}

// TestInMemory_E2E_DiscoverFallback_UnsupportedProtocolVersion verifies the
// fallback path when the server explicitly rejects the discover probe with
// CodeUnsupportedProtocolVersion (the structured SEP-2575 signal). This
// exercises Path A of the fallback logic in client.go.
func TestInMemory_E2E_DiscoverFallback_UnsupportedProtocolVersion(t *testing.T) {
	ctx := context.Background()

	server := NewServer(&Implementation{Name: "strict-server", Version: "v1"}, nil)
	server.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method == methodDiscover {
				return nil, &jsonrpc.Error{
					Code:    CodeUnsupportedProtocolVersion,
					Message: "unsupported protocol version",
				}
			}
			return next(ctx, method, req)
		}
	})

	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer ss.Close()

	client := NewClient(&Implementation{Name: "new-client", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	ir := cs.InitializeResult()
	if ir == nil {
		t.Fatal("InitializeResult is nil after fallback initialize")
	}
	// The fallback initialize explicitly requests protocolVersion20251125
	// (see client.go), so the server negotiates that version.
	if ir.ProtocolVersion != protocolVersion20251125 {
		t.Errorf("InitializeResult.ProtocolVersion = %q, want %q (legacy fallback after UnsupportedProtocolVersion)",
			ir.ProtocolVersion, protocolVersion20251125)
	}
}

// TestClientConnectDiscover_UnsupportedVersionNegotiation verifies the
// per SEP-2575 Version Negotiation Flow: when the client probes server/discover
// with a protocol version the server doesn't implement, the server's
// UnsupportedProtocolVersionError carries Data.Supported, and the client
// selects a mutually supported version from that list and retries.
func TestClientConnectDiscover_UnsupportedVersionNegotiation(t *testing.T) {
	ctx := context.Background()

	const unsupportedClientVersion = "2099-12-31"

	var (
		discoverCalls           atomic.Int32
		gotInitialize           atomic.Bool
		observedDiscoverVersion atomic.Value // string
	)

	s := NewServer(testImpl, nil)
	s.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			switch method {
			case methodDiscover:
				sr, ok := req.(*ServerRequest[*DiscoverParams])
				if !ok {
					t.Errorf("discover req has unexpected type %T", req)
					return nil, jsonrpc2.ErrMethodNotFound
				}
				if v, _ := sr.Params.GetMeta()[MetaKeyProtocolVersion].(string); v != "" {
					observedDiscoverVersion.Store(v)
				}
				discoverCalls.Add(1)
			case methodInitialize:
				gotInitialize.Store(true)
			}
			return next(ctx, method, req)
		}
	})

	ct, st := NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	defer ss.Close()

	c := NewClient(testImpl, nil)
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: unsupportedClientVersion})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer cs.Close()

	if got, want := discoverCalls.Load(), int32(1); got != want {
		t.Errorf("server/discover handler call count = %d, want %d (first probe is rejected by the dispatcher; only the retry reaches the handler)", got, want)
	}
	if got, _ := observedDiscoverVersion.Load().(string); got != protocolVersion20260728 {
		t.Errorf("retry discover requested version = %q, want %q (highest mutually supported version)", got, protocolVersion20260728)
	}
	if gotInitialize.Load() {
		t.Error("legacy initialize handshake ran, but negotiated discover should have succeeded")
	}

	ir := cs.InitializeResult()
	if ir == nil {
		t.Fatal("InitializeResult is nil after Connect")
	}
	if got, want := ir.ProtocolVersion, protocolVersion20260728; got != want {
		t.Errorf("InitializeResult.ProtocolVersion = %q, want %q", got, want)
	}
}
