// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//lint:file-ignore SA1019 tests exercise deprecated SEP-2577 APIs (roots, sampling, logging).

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

type hiParams struct {
	Name string
}

// TODO(jba): after schemas are stateless (WIP), this can be a variable.
func greetTool() *Tool { return &Tool{Name: "greet", Description: "say hi"} }

func sayHi(ctx context.Context, req *CallToolRequest, args hiParams) (*CallToolResult, any, error) {
	if err := req.Session.Ping(ctx, nil); err != nil {
		return nil, nil, fmt.Errorf("ping failed: %v", err)
	}
	return &CallToolResult{Content: []Content{&TextContent{Text: "hi " + args.Name}}}, nil, nil
}

var codeReviewPrompt = &Prompt{
	Name:        "code_review",
	Description: "do a code review",
	Arguments:   []*PromptArgument{{Name: "Code", Required: true}},
}

func codReviewPromptHandler(_ context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
	return &GetPromptResult{
		Description: "Code review prompt",
		Messages: []*PromptMessage{
			{Role: "user", Content: &TextContent{Text: "Please review the following code: " + req.Params.Arguments["Code"]}},
		},
	}, nil
}

// TODO(maciej-kisiel): split this test into multiple test cases that target specific functionality.
func TestEndToEnd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		var ct, st Transport = NewInMemoryTransports()

		// Channels to check if notification callbacks happened.
		// These test asynchronous sending of notifications after a small delay (see
		// Server.sendNotification).
		notificationChans := map[string]chan int{}
		for _, name := range []string{"initialized", "roots", "tools", "prompts", "resources", "progress_server", "progress_client", "resource_updated", "subscribe", "unsubscribe", "elicitation_complete"} {
			notificationChans[name] = make(chan int, 1)
		}

		waitForNotification := func(t *testing.T, name string) {
			t.Helper()
			time.Sleep(notificationDelay * 2)
			<-notificationChans[name]
		}

		sopts := &ServerOptions{
			InitializedHandler: func(context.Context, *InitializedRequest) {
				notificationChans["initialized"] <- 0
			},
			RootsListChangedHandler: func(context.Context, *RootsListChangedRequest) {
				notificationChans["roots"] <- 0
			},
			ProgressNotificationHandler: func(context.Context, *ProgressNotificationServerRequest) {
				notificationChans["progress_server"] <- 0
			},
			SubscribeHandler: func(context.Context, *SubscribeRequest) error {
				notificationChans["subscribe"] <- 0
				return nil
			},
			UnsubscribeHandler: func(context.Context, *UnsubscribeRequest) error {
				notificationChans["unsubscribe"] <- 0
				return nil
			},
		}
		s := NewServer(testImpl, sopts)
		AddTool(s, &Tool{
			Name:        "greet",
			Description: "say hi",
		}, sayHi)
		AddTool(s, &Tool{Name: "fail", InputSchema: &jsonschema.Schema{Type: "object"}},
			func(context.Context, *CallToolRequest, map[string]any) (*CallToolResult, any, error) {
				return nil, nil, errTestFailure
			})
		s.AddPrompt(codeReviewPrompt, codReviewPromptHandler)
		s.AddPrompt(&Prompt{Name: "fail"}, func(_ context.Context, _ *GetPromptRequest) (*GetPromptResult, error) {
			return nil, errTestFailure
		})
		s.AddResource(resource1, readHandler)
		s.AddResource(resource2, readHandler)

		// Connect the server.
		ss, err := s.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Collect(s.Sessions()); len(got) != 1 {
			t.Errorf("after connection, Clients() has length %d, want 1", len(got))
		}

		loggingMessages := make(chan *LoggingMessageParams, 100) // big enough for all logging
		opts := &ClientOptions{
			CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
				return &CreateMessageResult{Model: "aModel", Content: &TextContent{}}, nil
			},
			ElicitationHandler: func(ctx context.Context, req *ElicitRequest) (*ElicitResult, error) {
				return &ElicitResult{
					Action: "accept",
					Content: map[string]any{
						"name":  "Test User",
						"email": "test@example.com",
					},
				}, nil
			},
			ToolListChangedHandler: func(context.Context, *ToolListChangedRequest) {
				notificationChans["tools"] <- 0
			},
			PromptListChangedHandler: func(context.Context, *PromptListChangedRequest) {
				notificationChans["prompts"] <- 0
			},
			ResourceListChangedHandler: func(context.Context, *ResourceListChangedRequest) {
				notificationChans["resources"] <- 0
			},
			LoggingMessageHandler: func(_ context.Context, req *LoggingMessageRequest) {
				loggingMessages <- req.Params
			},
			ProgressNotificationHandler: func(context.Context, *ProgressNotificationClientRequest) {
				notificationChans["progress_client"] <- 0
			},
			ResourceUpdatedHandler: func(context.Context, *ResourceUpdatedNotificationRequest) {
				notificationChans["resource_updated"] <- 0
			},
			ElicitationCompleteHandler: func(_ context.Context, req *ElicitationCompleteNotificationRequest) {
				notificationChans["elicitation_complete"] <- 0
			},
		}
		c := NewClient(testImpl, opts)
		rootAbs, err := filepath.Abs(filepath.FromSlash("testdata/files"))
		if err != nil {
			t.Fatal(err)
		}
		c.AddRoots(&Root{URI: "file://" + rootAbs})

		// Connect the client.
		//
		// Pin the session to 2025-11-25 so the legacy
		// semantics apply.
		cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
		if err != nil {
			t.Fatal(err)
		}

		waitForNotification(t, "initialized")
		if err := cs.Ping(ctx, nil); err != nil {
			t.Fatalf("ping failed: %v", err)
		}

		// ===== prompts =====
		t.Log("Testing prompts")
		{
			res, err := cs.ListPrompts(ctx, nil)
			if err != nil {
				t.Fatalf("prompts/list failed: %v", err)
			}
			wantPrompts := []*Prompt{
				{
					Name:        "code_review",
					Description: "do a code review",
					Arguments:   []*PromptArgument{{Name: "Code", Required: true}},
				},
				{Name: "fail"},
			}
			if diff := cmp.Diff(wantPrompts, res.Prompts); diff != "" {
				t.Fatalf("prompts/list mismatch (-want +got):\n%s", diff)
			}

			gotReview, err := cs.GetPrompt(ctx, &GetPromptParams{Name: "code_review", Arguments: map[string]string{"Code": "1+1"}})
			if err != nil {
				t.Fatal(err)
			}
			wantReview := &GetPromptResult{
				Description: "Code review prompt",
				Messages: []*PromptMessage{{
					Content: &TextContent{Text: "Please review the following code: 1+1"},
					Role:    "user",
				}},
			}
			if diff := cmp.Diff(wantReview, gotReview, ctrCmpOpts...); diff != "" {
				t.Errorf("prompts/get 'code_review' mismatch (-want +got):\n%s", diff)
			}

			if _, err := cs.GetPrompt(ctx, &GetPromptParams{Name: "fail"}); err == nil || !strings.Contains(err.Error(), errTestFailure.Error()) {
				t.Errorf("fail returned unexpected error: got %v, want containing %v", err, errTestFailure)
			}

			s.AddPrompt(&Prompt{Name: "T"}, nil)
			waitForNotification(t, "prompts")
			s.RemovePrompts("T")
			waitForNotification(t, "prompts")
		}

		// ===== tools =====
		t.Log("Testing tools")
		{
			// ListTools is tested in client_list_test.go.
			gotHi, err := cs.CallTool(ctx, &CallToolParams{
				Name:      "greet",
				Arguments: map[string]any{"Name": "user"},
			})
			if err != nil {
				t.Fatal(err)
			}
			wantHi := &CallToolResult{
				Content: []Content{
					&TextContent{Text: "hi user"},
				},
			}
			if diff := cmp.Diff(wantHi, gotHi, ctrCmpOpts...); diff != "" {
				t.Errorf("tools/call 'greet' mismatch (-want +got):\n%s", diff)
			}

			gotFail, err := cs.CallTool(ctx, &CallToolParams{
				Name:      "fail",
				Arguments: map[string]any{},
			})
			// Counter-intuitively, when a tool fails, we don't expect an RPC error for
			// call tool: instead, the failure is embedded in the result.
			if err != nil {
				t.Fatal(err)
			}
			wantFail := &CallToolResult{
				IsError: true,
				Content: []Content{
					&TextContent{Text: errTestFailure.Error()},
				},
			}
			if diff := cmp.Diff(wantFail, gotFail, ctrCmpOpts...); diff != "" {
				t.Errorf("tools/call 'fail' mismatch (-want +got):\n%s", diff)
			}

			// Check output schema validation.
			badout := &Tool{
				Name: "badout",
				OutputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"x": {Type: "string"},
					},
				},
			}
			AddTool(s, badout, func(_ context.Context, _ *CallToolRequest, arg map[string]any) (*CallToolResult, map[string]any, error) {
				return nil, map[string]any{"x": 1}, nil
			})
			_, err = cs.CallTool(ctx, &CallToolParams{Name: "badout"})
			wantMsg := `has type "integer", want "string"`
			if err == nil || !strings.Contains(err.Error(), wantMsg) {
				t.Errorf("\ngot  %q\nwant error message containing %q", err, wantMsg)
			}

			// Check tools-changed notifications.
			s.AddTool(&Tool{Name: "T", InputSchema: &jsonschema.Schema{Type: "object"}}, nopHandler)
			waitForNotification(t, "tools")
			s.RemoveTools("T")
			waitForNotification(t, "tools")
		}

		// ===== resources =====
		t.Log("Testing resources")
		//TODO: fix for Windows
		if runtime.GOOS != "windows" {
			wantResources := []*Resource{resource2, resource1}
			lrres, err := cs.ListResources(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(wantResources, lrres.Resources); diff != "" {
				t.Errorf("resources/list mismatch (-want, +got):\n%s", diff)
			}

			template := &ResourceTemplate{
				Name:        "rt",
				MIMEType:    "text/template",
				URITemplate: "file:///{+filename}", // the '+' means that filename can contain '/'
			}
			s.AddResourceTemplate(template, readHandler)
			tres, err := cs.ListResourceTemplates(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff([]*ResourceTemplate{template}, tres.ResourceTemplates); diff != "" {
				t.Errorf("resources/list mismatch (-want, +got):\n%s", diff)
			}

			for _, tt := range []struct {
				uri      string
				mimeType string // "": not found; "text/plain": resource; "text/template": template
				fail     bool   // non-nil error returned
			}{
				{"file:///info.txt", "text/plain", false},
				{"file:///fail.txt", "", false},
				{"file:///template.txt", "text/template", false},
				{"file:///../private.txt", "", true}, // not found: escaping disallowed
			} {
				rres, err := cs.ReadResource(ctx, &ReadResourceParams{URI: tt.uri})
				if err != nil {
					if code := errorCode(err); code == CodeResourceNotFound {
						if tt.mimeType != "" {
							t.Errorf("%s: not found but expected it to be", tt.uri)
						}
					} else if !tt.fail {
						t.Errorf("%s: unexpected error %v", tt.uri, err)
					}
				} else {
					if tt.fail {
						t.Errorf("%s: unexpected success", tt.uri)
					} else if g, w := len(rres.Contents), 1; g != w {
						t.Errorf("got %d contents, wanted %d", g, w)
					} else {
						c := rres.Contents[0]
						if got := c.URI; got != tt.uri {
							t.Errorf("got uri %q, want %q", got, tt.uri)
						}
						if got := c.MIMEType; got != tt.mimeType {
							t.Errorf("%s: got MIME type %q, want %q", tt.uri, got, tt.mimeType)
						}
					}
				}
			}

			s.AddResource(&Resource{URI: "http://U"}, nil)
			waitForNotification(t, "resources")
			s.RemoveResources("http://U")
			waitForNotification(t, "resources")
		}

		// ===== roots =====
		t.Log("Testing roots")
		{
			rootRes, err := ss.ListRoots(ctx, &ListRootsParams{})
			if err != nil {
				t.Fatal(err)
			}
			gotRoots := rootRes.Roots
			wantRoots := slices.Collect(c.roots.all())
			if diff := cmp.Diff(wantRoots, gotRoots); diff != "" {
				t.Errorf("roots/list mismatch (-want +got):\n%s", diff)
			}

			c.AddRoots(&Root{URI: "U"})
			waitForNotification(t, "roots")
			c.RemoveRoots("U")
			waitForNotification(t, "roots")
		}

		// ===== sampling =====
		t.Log("Testing sampling")
		{
			// TODO: test that a client that doesn't have the handler returns CodeUnsupportedMethod.
			res, err := ss.CreateMessage(ctx, &CreateMessageParams{})
			if err != nil {
				t.Fatal(err)
			}
			if g, w := res.Model, "aModel"; g != w {
				t.Errorf("got %q, want %q", g, w)
			}
		}

		// ===== logging =====
		t.Log("Testing logging")
		{
			want := []*LoggingMessageParams{
				{
					Logger: "test",
					Level:  "warning",
					Data: map[string]any{
						"msg":     "first",
						"name":    "Pat",
						"logtest": true,
					},
				},
				{
					Logger: "test",
					Level:  "alert",
					Data: map[string]any{
						"msg":     "second",
						"count":   2.0,
						"logtest": true,
					},
				},
			}

			check := func(t *testing.T) {
				t.Helper()
				var got []*LoggingMessageParams
				// Read messages from this test until we've seen all we expect.
				for len(got) < len(want) {
					p := <-loggingMessages
					// Ignore logging from other tests.
					if m, ok := p.Data.(map[string]any); ok && m["logtest"] != nil {
						delete(m, "time")
						got = append(got, p)
					}
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("mismatch (-want, +got):\n%s", diff)
				}
			}

			// Use the LoggingMessage method directly.
			t.Log("Testing logging (direct)")
			{
				mustLog := func(level LoggingLevel, data any) {
					t.Helper()
					if err := ss.Log(ctx, &LoggingMessageParams{
						Logger: "test",
						Level:  level,
						Data:   data,
					}); err != nil {
						t.Fatal(err)
					}
				}

				// Nothing should be logged until the client sets a level.
				mustLog("info", "before")
				if err := cs.SetLoggingLevel(ctx, &SetLoggingLevelParams{Level: "warning"}); err != nil {
					t.Fatal(err)
				}
				mustLog("warning", want[0].Data)
				mustLog("debug", "nope")    // below the level
				mustLog("info", "negative") // below the level
				mustLog("alert", want[1].Data)
				check(t)
			}

			// Use the slog handler.
			t.Log("Testing logging (handler)")
			{
				// We can't check the "before SetLevel" behavior because it's already been set.
				// Not a big deal: that check is in LoggingMessage anyway.
				logger := slog.New(NewLoggingHandler(ss, &LoggingHandlerOptions{LoggerName: "test"}))
				logger.Warn("first", "name", "Pat", "logtest", true)
				logger.Debug("nope")    // below the level
				logger.Info("negative") // below the level
				logger.Log(ctx, LevelAlert, "second", "count", 2, "logtest", true)
				check(t)
			}
		}

		// ===== progress =====
		t.Log("Testing progress")
		{
			ss.NotifyProgress(ctx, &ProgressNotificationParams{
				ProgressToken: "token-xyz",
				Message:       "progress update",
				Progress:      0.5,
				Total:         2,
			})
			waitForNotification(t, "progress_client")

			cs.NotifyProgress(ctx, &ProgressNotificationParams{
				ProgressToken: "token-abc",
				Message:       "progress update",
				Progress:      1,
				Total:         2,
			})
			waitForNotification(t, "progress_server")
		}

		// ===== resource_subscriptions =====
		t.Log("Testing resource_subscriptions")
		{
			err := cs.Subscribe(ctx, &SubscribeParams{
				URI: "test",
			})
			if err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			<-notificationChans["subscribe"]

			s.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{
				URI: "test",
			})
			waitForNotification(t, "resource_updated")

			err = cs.Unsubscribe(ctx, &UnsubscribeParams{
				URI: "test",
			})
			if err != nil {
				t.Fatal(err)
			}
			waitForNotification(t, "unsubscribe")

			// Verify the client does not receive the update after unsubscribing.
			s.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{
				URI: "test",
			})
			synctest.Wait()
			select {
			case <-notificationChans["resource_updated"]:
				t.Fatalf("resource updated after unsubscription")
			default:
				// Expected: no notification received
			}
		}

		// ===== elicitation =====
		t.Log("Testing elicitation")
		{
			result, err := ss.Elicit(ctx, &ElicitParams{
				Message: "Please provide information",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Action != "accept" {
				t.Errorf("got action %q, want %q", result.Action, "accept")
			}
		}

		// Disconnect.
		cs.Close()
		if err := ss.Wait(); err != nil {
			t.Errorf("server failed: %v", err)
		}

		// After disconnecting, neither client nor server should have any
		// connections.
		for range s.Sessions() {
			t.Errorf("unexpected client after disconnection")
		}
	})
}

// Registry of values to be referenced in tests.
var (
	errTestFailure = errors.New("mcp failure")

	resource1 = &Resource{
		Name:     "public",
		MIMEType: "text/plain",
		URI:      "file:///info.txt",
	}
	resource2 = &Resource{
		Name:     "public", // names are not unique IDs
		MIMEType: "text/plain",
		URI:      "file:///fail.txt",
	}
	resource3 = &Resource{
		Name:     "info",
		MIMEType: "text/plain",
		URI:      "embedded:info",
	}
	readHandler = fileResourceHandler("testdata/files")
)

var embeddedResources = map[string]string{
	"info": "This is the MCP test server.",
}

func handleEmbeddedResource(_ context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "embedded" {
		return nil, fmt.Errorf("wrong scheme: %q", u.Scheme)
	}
	key := u.Opaque
	text, ok := embeddedResources[key]
	if !ok {
		return nil, fmt.Errorf("no embedded resource named %q", key)
	}
	return &ReadResourceResult{
		Contents: []*ResourceContents{
			{URI: req.Params.URI, MIMEType: "text/plain", Text: text},
		},
	}, nil
}

// errorCode returns the code associated with err.
// If err is nil, it returns 0.
// If there is no code, it returns -1.
func errorCode(err error) int64 {
	if err == nil {
		return 0
	}
	var werr *jsonrpc.Error
	if errors.As(err, &werr) {
		return werr.Code
	}
	return -1
}

// basicConnection returns a new basic client-server connection, with the server
// configured via the provided function.
//
// The caller should cancel either the client connection or server connection
// when the connections are no longer needed.
//
// The returned func cleans up by closing the client and waiting for the server
// to shut down.
func basicConnection(t *testing.T, config func(*Server)) (*ClientSession, *ServerSession, func()) {
	return basicClientServerConnection(t, nil, nil, config)
}

// basicClientServerConnection creates a basic connection between client and
// server. If either client or server is nil, empty implementations are used.
//
// The provided function may be used to configure features on the resulting
// server, prior to connection.
//
// The caller should cancel either the client connection or server connection
// when the connections are no longer needed.
//
// The returned func cleans up by closing the client and waiting for the server
// to shut down.
func basicClientServerConnection(t *testing.T, client *Client, server *Server, config func(*Server)) (*ClientSession, *ServerSession, func()) {
	t.Helper()

	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	if server == nil {
		server = NewServer(testImpl, nil)
	}
	if config != nil {
		config(server)
	}
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	if client == nil {
		client = NewClient(testImpl, nil)
	}
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return cs, ss, func() {
		cs.Close()
		ss.Wait()
	}
}

func TestServerClosing(t *testing.T) {
	cs, ss, cleanup := basicConnection(t, func(s *Server) {
		AddTool(s, greetTool(), sayHi)
	})
	defer cleanup()

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Go(func() {
		if err := cs.Wait(); err != nil {
			t.Errorf("server connection failed: %v", err)
		}
	})
	if _, err := cs.CallTool(ctx, &CallToolParams{
		Name:      "greet",
		Arguments: map[string]any{"Name": "user"},
	}); err != nil {
		t.Fatalf("after connecting: %v", err)
	}
	ss.Close()
	wg.Wait()
	if _, err := cs.CallTool(ctx, &CallToolParams{
		Name:      "greet",
		Arguments: map[string]any{"name": "user"},
	}); !errors.Is(err, ErrConnectionClosed) {
		t.Errorf("after disconnection, got error %v, want EOF", err)
	}
}

func TestCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			start     = make(chan struct{})
			cancelled = make(chan struct{}, 1) // don't block the request
		)
		slowTool := func(ctx context.Context, req *CallToolRequest, args any) (*CallToolResult, any, error) {
			start <- struct{}{}
			select {
			case <-ctx.Done():
				cancelled <- struct{}{}
			case <-time.After(5 * time.Second):
				return nil, nil, nil
			}
			return nil, nil, nil
		}
		cs, _, cleanup := basicConnection(t, func(s *Server) {
			AddTool(s, &Tool{Name: "slow", InputSchema: &jsonschema.Schema{Type: "object"}}, slowTool)
		})
		defer cleanup()

		ctx, cancel := context.WithCancel(context.Background())
		go cs.CallTool(ctx, &CallToolParams{Name: "slow"})
		<-start
		cancel()

		<-cancelled
	})
}

func TestMiddleware(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer(testImpl, nil)
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Wait for the server to exit after the client closes its connection.
	t.Cleanup(func() { _ = ss.Close() })

	var sbuf, cbuf bytes.Buffer
	sbuf.WriteByte('\n')
	cbuf.WriteByte('\n')

	// "1" is the outer middleware layer, called first; then "2" is called, and finally
	// the default dispatcher.
	s.AddSendingMiddleware(traceCalls[*ServerSession](&sbuf, "S1"), traceCalls[*ServerSession](&sbuf, "S2"))
	s.AddReceivingMiddleware(traceCalls[*ServerSession](&sbuf, "R1"), traceCalls[*ServerSession](&sbuf, "R2"))

	c := NewClient(testImpl, nil)
	c.AddSendingMiddleware(traceCalls[*ClientSession](&cbuf, "S1"), traceCalls[*ClientSession](&cbuf, "S2"))
	c.AddReceivingMiddleware(traceCalls[*ClientSession](&cbuf, "R1"), traceCalls[*ClientSession](&cbuf, "R2"))

	// Pin to 2025-11-25 because the test's expected wire sequence asserts
	// the legacy initialize / notifications/initialized handshake, which
	// 2026-07-28 replaces with server/discover.
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.ListRoots(ctx, nil); err != nil {
		t.Fatal(err)
	}

	wantServer := `
R1 >initialize
R2 >initialize
R2 <initialize
R1 <initialize
R1 >notifications/initialized
R2 >notifications/initialized
R2 <notifications/initialized
R1 <notifications/initialized
R1 >tools/list
R2 >tools/list
R2 <tools/list
R1 <tools/list
S1 >roots/list
S2 >roots/list
S2 <roots/list
S1 <roots/list
`
	if diff := cmp.Diff(wantServer, sbuf.String()); diff != "" {
		t.Errorf("server mismatch (-want, +got):\n%s", diff)
	}

	wantClient := `
S1 >initialize
S2 >initialize
S2 <initialize
S1 <initialize
S1 >notifications/initialized
S2 >notifications/initialized
S2 <notifications/initialized
S1 <notifications/initialized
S1 >tools/list
S2 >tools/list
S2 <tools/list
S1 <tools/list
R1 >roots/list
R2 >roots/list
R2 <roots/list
R1 <roots/list
`
	if diff := cmp.Diff(wantClient, cbuf.String()); diff != "" {
		t.Errorf("client mismatch (-want, +got):\n%s", diff)
	}
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *safeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}

func TestNoJSONNull(t *testing.T) {
	ctx := context.Background()
	var ct, st Transport = NewInMemoryTransports()

	// Collect logs, to sanity check that we don't write JSON null anywhere.
	var logbuf safeBuffer
	ct = &LoggingTransport{Transport: ct, Writer: &logbuf}

	s := NewServer(testImpl, nil)
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}

	c := NewClient(testImpl, nil)
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cs.ListTools(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.ListPrompts(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.ListResources(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.ListResourceTemplates(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.ListRoots(ctx, nil); err != nil {
		t.Fatal(err)
	}

	cs.Close()
	ss.Wait()

	logs := logbuf.Bytes()
	if i := bytes.Index(logs, []byte("null")); i >= 0 {
		start := max(i-20, 0)
		end := min(i+20, len(logs))
		t.Errorf("conformance violation: MCP logs contain JSON null: %s", "..."+string(logs[start:end])+"...")
	}
}

// traceCalls creates a middleware function that prints the method before and after each call
// with the given prefix.
func traceCalls[S Session](w io.Writer, prefix string) Middleware {
	return func(h MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			fmt.Fprintf(w, "%s >%s\n", prefix, method)
			defer fmt.Fprintf(w, "%s <%s\n", prefix, method)
			return h(ctx, method, req)
		}
	}
}

func nopHandler(context.Context, *CallToolRequest) (*CallToolResult, error) {
	return nil, nil
}

func TestKeepAlive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()

		ct, st := NewInMemoryTransports()

		serverOpts := &ServerOptions{
			KeepAlive: 100 * time.Millisecond,
		}
		s := NewServer(testImpl, serverOpts)
		AddTool(s, greetTool(), sayHi)

		ss, err := s.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ss.Close()

		clientOpts := &ClientOptions{
			KeepAlive: 100 * time.Millisecond,
		}
		c := NewClient(testImpl, clientOpts)
		cs, err := c.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		// Wait for a few keepalive cycles to ensure pings are working.
		// With synctest, this advances simulated time instantly.
		time.Sleep(300 * time.Millisecond)

		// Test that the connection is still alive by making a call
		result, err := cs.CallTool(ctx, &CallToolParams{
			Name:      "greet",
			Arguments: map[string]any{"Name": "user"},
		})
		if err != nil {
			t.Fatalf("call failed after keepalive: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}
		if textContent, ok := result.Content[0].(*TextContent); !ok || textContent.Text != "hi user" {
			t.Fatalf("unexpected result: %v", result.Content[0])
		}
	})
}

func TestKeepAliveMethodNotFound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()

		ct, st := NewInMemoryTransports()

		// Server that rejects ping with method-not-found, simulating a
		// server that does not implement the optional ping method.
		s := NewServer(testImpl, nil)
		AddTool(s, greetTool(), sayHi)
		s.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, req Request) (Result, error) {
				if method == "ping" {
					return nil, jsonrpc2.ErrMethodNotFound
				}
				return next(ctx, method, req)
			}
		})
		ss, err := s.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ss.Close()

		clientOpts := &ClientOptions{
			KeepAlive: 50 * time.Millisecond,
		}
		c := NewClient(testImpl, clientOpts)
		cs, err := c.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		// Advance past several keepalive cycles.
		time.Sleep(200 * time.Millisecond)

		// The session should still be alive despite the server not
		// supporting ping.
		result, err := cs.CallTool(ctx, &CallToolParams{
			Name:      "greet",
			Arguments: map[string]any{"Name": "user"},
		})
		if err != nil {
			t.Fatalf("call failed after keepalive with method-not-found: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}
		if textContent, ok := result.Content[0].(*TextContent); !ok || textContent.Text != "hi user" {
			t.Fatalf("unexpected result: %v", result.Content[0])
		}
	})
}

func TestElicitationUnsupportedMethod(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	// Server
	s := NewServer(testImpl, nil)
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	// Client without ElicitationHandler
	c := NewClient(testImpl, &ClientOptions{
		CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
			return &CreateMessageResult{Model: "aModel", Content: &TextContent{}}, nil
		},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Test that elicitation fails when no handler is provided
	_, err = ss.Elicit(ctx, &ElicitParams{
		Message: "This should fail",
		RequestedSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"test": {Type: "string"},
			},
		},
	})

	if err == nil {
		t.Error("expected error when ElicitationHandler is not provided, got nil")
	}
	if code := errorCode(err); code != -1 {
		t.Errorf("got error code %d, want -1", code)
	}
	if !strings.Contains(err.Error(), "does not support elicitation") {
		t.Errorf("error should mention unsupported elicitation, got: %v", err)
	}
}

func anyPtr[T any](v T) *any {
	var a any = v
	return &a
}

func TestElicitationSchemaValidation(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer(testImpl, nil)
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	c := NewClient(testImpl, &ClientOptions{
		ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
			return &ElicitResult{Action: "accept", Content: map[string]any{"test": "value"}}, nil
		},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Test valid schemas - these should not return errors
	validSchemas := []struct {
		name   string
		schema *jsonschema.Schema
	}{
		{
			name:   "nil schema",
			schema: nil,
		},
		{
			name: "empty object schema",
			schema: &jsonschema.Schema{
				Type: "object",
			},
		},
		{
			name: "simple string property",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {Type: "string"},
				},
			},
		},
		{
			name: "string with valid formats",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"email":    {Type: "string", Format: "email"},
					"website":  {Type: "string", Format: "uri"},
					"birthday": {Type: "string", Format: "date"},
					"created":  {Type: "string", Format: "date-time"},
				},
			},
		},
		{
			name: "string with constraints",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {Type: "string", MinLength: ptr(1), MaxLength: ptr(100)},
				},
			},
		},
		{
			name: "number with constraints",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"age":   {Type: "integer", Minimum: ptr(0.0), Maximum: ptr(150.0)},
					"score": {Type: "number", Minimum: ptr(0.0), Maximum: ptr(100.0)},
				},
			},
		},
		{
			name: "boolean with default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"enabled": {Type: "boolean", Default: json.RawMessage("true")},
				},
			},
		},
		{
			name: "single select untitled enum",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"status": {
						Type: "string",
						Enum: []any{
							"active",
							"inactive",
							"pending",
						},
					},
				},
			},
		},
		{
			name: "legacy titled enum",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"priority": {
						Type: "string",
						Enum: []any{
							"high",
							"medium",
							"low",
						},
						Extra: map[string]any{
							"enumNames": []any{"High Priority", "Medium Priority", "Low Priority"},
						},
					},
				},
			},
		},
		{
			name: "single select titled enum",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"priority": {
						Type: "string",
						OneOf: []*jsonschema.Schema{
							{
								Const: anyPtr("high"),
								Title: "High Priority",
							},
							{
								Const: anyPtr("medium"),
								Title: "Medium Priority",
							},
							{
								Const: anyPtr("low"),
								Title: "Low Priority",
							},
						},
					},
				},
			},
		},
		{
			name: "multi select untitled enum",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"status": {
						Type: "array",
						Items: &jsonschema.Schema{
							Type: "string",
							Enum: []any{
								"active",
								"inactive",
								"pending",
							},
						},
					},
				},
			},
		},
		{
			name: "multi select titled enum",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"priority": {
						Type: "array",
						Items: &jsonschema.Schema{
							AnyOf: []*jsonschema.Schema{
								{
									Const: anyPtr("high"),
									Title: "High Priority",
								},
								{
									Const: anyPtr("medium"),
									Title: "Medium Priority",
								},
								{
									Const: anyPtr("low"),
									Title: "Low Priority",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range validSchemas {
		t.Run("valid_"+tc.name, func(t *testing.T) {
			_, err := ss.Elicit(ctx, &ElicitParams{
				Message:         "Test valid schema: " + tc.name,
				RequestedSchema: tc.schema,
			})
			if err != nil {
				t.Errorf("expected no error for valid schema %q, got: %v", tc.name, err)
			}
		})
	}

	// Test invalid schemas - these should return errors
	invalidSchemas := []struct {
		name          string
		schema        *jsonschema.Schema
		expectedError string
	}{
		{
			name: "root schema non-object type",
			schema: &jsonschema.Schema{
				Type: "string",
			},
			expectedError: "elicit schema must be of type 'object', got \"string\"",
		},
		{
			name: "nested object property",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"user": {
						Type: "object",
						Properties: map[string]*jsonschema.Schema{
							"name": {Type: "string"},
						},
					},
				},
			},
			expectedError: "elicit schema property \"user\" contains nested properties, only primitive properties are allowed",
		},
		{
			name: "property with explicit object type",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"config": {Type: "object"},
				},
			},
			expectedError: "elicit schema property \"config\" has unsupported type \"object\", only string, number, integer, boolean, and array are allowed",
		},
		{
			name: "array without items",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"items": {Type: "array"},
				},
			},
			expectedError: "elicit schema property \"items\" is array but missing 'items' definition",
		},
		{
			name: "array with integer items",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"items": {Type: "array", Items: &jsonschema.Schema{Type: "integer"}},
				},
			},
			expectedError: "elicit schema property \"items\" items have unsupported type \"integer\"",
		},
		{
			name: "array of generic strings",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"items": {Type: "array", Items: &jsonschema.Schema{Type: "string"}},
				},
			},
			expectedError: "elicit schema property \"items\" items must specify enum for untitled enums",
		},
		{
			name: "unsupported string format",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"phone": {Type: "string", Format: "phone"},
				},
			},
			expectedError: "elicit schema property \"phone\" has unsupported format \"phone\", only email, uri, date, and date-time are allowed",
		},
		{
			name: "unsupported type",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"data": {Type: "null"},
				},
			},
			expectedError: "elicit schema property \"data\" has unsupported type \"null\"",
		},
		{
			name: "string with invalid minLength",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {Type: "string", MinLength: ptr(-1)},
				},
			},
			expectedError: "elicit schema property \"name\" has invalid minLength -1, must be non-negative",
		},
		{
			name: "string with invalid maxLength",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {Type: "string", MaxLength: ptr(-5)},
				},
			},
			expectedError: "elicit schema property \"name\" has invalid maxLength -5, must be non-negative",
		},
		{
			name: "string with maxLength less than minLength",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {Type: "string", MinLength: ptr(10), MaxLength: ptr(5)},
				},
			},
			expectedError: "elicit schema property \"name\" has maxLength 5 less than minLength 10",
		},
		{
			name: "number with maximum less than minimum",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"score": {Type: "number", Minimum: ptr(100.0), Maximum: ptr(50.0)},
				},
			},
			expectedError: "elicit schema property \"score\" has maximum 50 less than minimum 100",
		},
		{
			name: "boolean with invalid default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"enabled": {Type: "boolean", Default: json.RawMessage(`"not-a-boolean"`)},
				},
			},
			expectedError: "elicit schema property \"enabled\" has invalid default value, must be a bool",
		},
		{
			name: "string with invalid default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"enabled": {Type: "string", Default: json.RawMessage("true")},
				},
			},
			expectedError: "elicit schema property \"enabled\" has invalid default value, must be a string",
		},
		{
			name: "integer with invalid default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"enabled": {Type: "integer", Default: json.RawMessage("true")},
				},
			},
			expectedError: "elicit schema property \"enabled\" has default value that cannot be interpreted as an int or float",
		},
		{
			name: "number with invalid default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"enabled": {Type: "number", Default: json.RawMessage("true")},
				},
			},
			expectedError: "elicit schema property \"enabled\" has default value that cannot be interpreted as an int or float",
		},
		{
			name: "enum with mismatched enumNames length",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"priority": {
						Type: "string",
						Enum: []any{
							"high",
							"medium",
							"low",
						},
						Extra: map[string]any{
							"enumNames": []any{"High Priority", "Medium Priority"}, // Only 2 names for 3 values
						},
					},
				},
			},
			expectedError: "elicit schema property \"priority\" has 3 enum values but 2 enumNames, they must match",
		},
		{
			name: "enum with invalid enumNames type",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"status": {
						Type: "string",
						Enum: []any{
							"active",
							"inactive",
						},
						Extra: map[string]any{
							"enumNames": "not an array", // Should be array
						},
					},
				},
			},
			expectedError: "elicit schema property \"status\" has invalid enumNames type, must be an array",
		},
		{
			name: "enum without explicit type",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"priority": {
						Enum: []any{
							"high",
							"medium",
							"low",
						},
					},
				},
			},
			expectedError: "elicit schema property \"priority\" has unsupported type \"\", only string, number, integer, boolean, and array are allowed",
		},
		{
			name: "titled enum without const",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"priority": {
						Type: "string",
						OneOf: []*jsonschema.Schema{
							{
								Const: anyPtr("high"),
								Title: "High Priority",
							},
							{
								Type:  "string",
								Title: "Other Priority",
							},
						},
					},
				},
			},
			expectedError: "elicit schema property \"priority\" oneOf has invalid entry: const is required for titled enum entries",
		},
		{
			name: "untyped property",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"data": {},
				},
			},
			expectedError: "elicit schema property \"data\" has unsupported type \"\", only string, number, integer, boolean, and array are allowed",
		},
	}

	for _, tc := range invalidSchemas {
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			_, err := ss.Elicit(ctx, &ElicitParams{
				Message:         "Test invalid schema: " + tc.name,
				RequestedSchema: tc.schema,
			})
			if err == nil {
				t.Errorf("expected error for invalid schema %q, got nil", tc.name)
				return
			}
			if code := errorCode(err); code != jsonrpc.CodeInvalidParams {
				t.Errorf("got error code %d, want %d (CodeInvalidParams)", code, jsonrpc.CodeInvalidParams)
			}
			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("error message %q does not contain expected text %q", err.Error(), tc.expectedError)
			}
		})
	}
}

func TestElicitContentValidation(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer(testImpl, nil)
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	// Set up a client that exercises valid/invalid elicitation: the returned
	// Content from the handler ("potato") is validated against the schemas
	// defined in the testcases below.
	c := NewClient(testImpl, &ClientOptions{
		ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
			return &ElicitResult{Action: "accept", Content: map[string]any{"test": "potato"}}, nil
		},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	testcases := []struct {
		name          string
		schema        *jsonschema.Schema
		expectedError string
	}{
		{
			name: "string enum with schema not matching content",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"test": {
						Type: "string",
						OneOf: []*jsonschema.Schema{
							{
								Const: anyPtr("high"),
								Title: "High Priority",
							},
						},
					},
				},
			},
			expectedError: "oneOf: did not validate against any of",
		},
		{
			name: "string enum with schema matching content",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"test": {
						Type: "string",
						OneOf: []*jsonschema.Schema{
							{
								Const: anyPtr("potato"),
								Title: "Potato Priority",
							},
						},
					},
				},
			},
			expectedError: "",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ss.Elicit(ctx, &ElicitParams{
				Message:         "Test schema: " + tc.name,
				RequestedSchema: tc.schema,
			})
			if tc.expectedError != "" {
				if err == nil {
					t.Errorf("expected error but got no error: %s", tc.expectedError)
					return
				}
				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("error message %q does not contain expected text %q", err.Error(), tc.expectedError)
				}
			}
		})
	}
}

func TestElicitationProgressToken(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer(testImpl, nil)
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	c := NewClient(testImpl, &ClientOptions{
		ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
			return &ElicitResult{Action: "accept"}, nil
		},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	params := &ElicitParams{
		Message: "Test progress token",
		Meta:    Meta{},
	}
	params.SetProgressToken("test-token")

	if token := params.GetProgressToken(); token != "test-token" {
		t.Errorf("got progress token %v, want %q", token, "test-token")
	}

	_, err = ss.Elicit(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
}

func TestElicitationCapabilityDeclaration(t *testing.T) {
	ctx := context.Background()

	t.Run("with handler", func(t *testing.T) {
		ct, st := NewInMemoryTransports()

		// Client with ElicitationHandler should declare capability
		c := NewClient(testImpl, &ClientOptions{
			ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
				return &ElicitResult{Action: "cancel"}, nil
			},
		})

		s := NewServer(testImpl, nil)
		ss, err := s.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ss.Close()

		cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		// The client should have declared elicitation capability during initialization
		// We can verify this worked by successfully making an elicitation call
		result, err := ss.Elicit(ctx, &ElicitParams{
			Message:         "Test capability",
			RequestedSchema: &jsonschema.Schema{Type: "object"},
		})
		if err != nil {
			t.Fatalf("elicitation should work when capability is declared, got error: %v", err)
		}
		if result.Action != "cancel" {
			t.Errorf("got action %q, want %q", result.Action, "cancel")
		}
	})

	t.Run("without handler", func(t *testing.T) {
		ct, st := NewInMemoryTransports()

		// Client without ElicitationHandler should not declare capability
		c := NewClient(testImpl, &ClientOptions{
			CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
				return &CreateMessageResult{Model: "aModel", Content: &TextContent{}}, nil
			},
		})

		s := NewServer(testImpl, nil)
		ss, err := s.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ss.Close()

		cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		// Elicitation should fail with UnsupportedMethod
		_, err = ss.Elicit(ctx, &ElicitParams{
			Message:         "This should fail",
			RequestedSchema: &jsonschema.Schema{Type: "object"},
		})

		if err == nil {
			t.Error("expected UnsupportedMethod error when no capability declared")
		}
		if code := errorCode(err); code != -1 {
			t.Errorf("got error code %d, want -1", code)
		}
	})
}

func TestElicitationDefaultValues(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer(testImpl, nil)
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	c := NewClient(testImpl, &ClientOptions{
		ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
			return &ElicitResult{Action: "accept", Content: map[string]any{"default": "response"}}, nil
		},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	testcases := []struct {
		name     string
		schema   *jsonschema.Schema
		expected map[string]any
	}{
		{
			name: "boolean with default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"key": {Type: "boolean", Default: json.RawMessage("true")},
				},
			},
			expected: map[string]any{"key": true, "default": "response"},
		},
		{
			name: "string with default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"key": {Type: "string", Default: json.RawMessage("\"potato\"")},
				},
			},
			expected: map[string]any{"key": "potato", "default": "response"},
		},
		{
			name: "integer with default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"key": {Type: "integer", Default: json.RawMessage("123")},
				},
			},
			expected: map[string]any{"key": float64(123), "default": "response"},
		},
		{
			name: "number with default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"key": {Type: "number", Default: json.RawMessage("89.7")},
				},
			},
			expected: map[string]any{"key": float64(89.7), "default": "response"},
		},
		{
			name: "enum with default",
			schema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"key": {Type: "string", Enum: []any{"one", "two"}, Default: json.RawMessage("\"one\"")},
				},
			},
			expected: map[string]any{"key": "one", "default": "response"},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ss.Elicit(ctx, &ElicitParams{
				Message:         "Test schema with defaults: " + tc.name,
				RequestedSchema: tc.schema,
			})
			if err != nil {
				t.Fatalf("expected no error for default schema %q, got: %v", tc.name, err)
			}
			if diff := cmp.Diff(tc.expected, res.Content); diff != "" {
				t.Errorf("%s: did not get expected value, -want +got:\n%s", tc.name, diff)
			}
		})
	}
}

func TestKeepAliveFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()

		// This test doesn't pinpoint keepalive detected failures well due to the fact that in-memory transports
		// propagate `io.EOF` synchronously, causing the transport level connection to be closed immediately.
		// TODO: propose better transports that would allow testing this scenario precisely.
		ct, st := NewInMemoryTransports()

		// Server without keepalive (to test one-sided keepalive)
		s := NewServer(testImpl, nil)
		AddTool(s, greetTool(), sayHi)
		ss, err := s.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Client with short keepalive
		clientOpts := &ClientOptions{
			KeepAlive: 50 * time.Millisecond,
		}
		c := NewClient(testImpl, clientOpts)
		cs, err := c.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		// Let the connection establish properly first
		synctest.Wait()

		// simulate ping failure
		ss.Close()

		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		_, err = cs.CallTool(ctx, &CallToolParams{
			Name:      "greet",
			Arguments: map[string]any{"Name": "user"},
		})
		if err != nil && (errors.Is(err, ErrConnectionClosed) || strings.Contains(err.Error(), "connection closed")) {
			return // Test passed
		}

		t.Errorf("expected connection to be closed by keepalive, but it wasn't. Last error: %v", err)
	})
}

// TestKeepAliveFailure_Logged verifies that a keepalive ping failure is
// reported via the configured slog.Logger instead of being silently dropped.
// Regression test for #218.
func TestKeepAliveFailure_Logged(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()

		ct, st := NewInMemoryTransports()

		// Server without keepalive.
		s := NewServer(testImpl, nil)
		AddTool(s, greetTool(), sayHi)
		ss, err := s.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Client with a short keepalive and a logger that writes to a
		// buffer so we can assert on its output.
		var buf bytes.Buffer
		clientOpts := &ClientOptions{
			KeepAlive: 50 * time.Millisecond,
			Logger:    slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})),
		}
		c := NewClient(testImpl, clientOpts)
		// Pin to 2025-11-25: KeepAlive uses the ping RPC, which is removed
		// in 2026-07-28, so keepalive is only meaningful on legacy protocol
		// versions.
		cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		synctest.Wait()

		// Trigger a ping failure by closing the server side.
		ss.Close()

		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		got := buf.String() // slog serializes Write calls internally
		if !strings.Contains(got, "keepalive ping failed") {
			t.Errorf("expected keepalive failure to be logged, got log output:\n%s", got)
		}
	})
}

// scriptedKeepaliveSession is a keepaliveSession test double whose Ping
// returns errors from a script (one entry consumed per call; the last entry
// repeats once exhausted), and records how many times Close was called. Ping
// returns immediately so the keepalive loop's pace is driven purely by the
// ticker, making the test deterministic under synctest.
type scriptedKeepaliveSession struct {
	pingErrs   []error
	pingCalls  atomic.Int64
	closeCalls atomic.Int64
}

func (s *scriptedKeepaliveSession) Ping(context.Context, *PingParams) error {
	n := int(s.pingCalls.Add(1)) - 1
	if n >= len(s.pingErrs) {
		n = len(s.pingErrs) - 1
	}
	return s.pingErrs[n]
}

func (s *scriptedKeepaliveSession) Close() error {
	s.closeCalls.Add(1)
	return nil
}

// TestStartKeepalive_FailureThreshold verifies that the session is kept alive
// across consecutive ping failures below the threshold and only closed once the
// threshold is reached.
func TestStartKeepalive_FailureThreshold(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const interval = 100 * time.Millisecond
		sess := &scriptedKeepaliveSession{pingErrs: []error{errors.New("boom")}}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		var cancel context.CancelFunc
		startKeepalive(sess, interval, 3, &cancel, logger)
		defer cancel()

		// After two ticks → two failures, still below threshold 3: not closed.
		time.Sleep(2*interval + interval/2)
		synctest.Wait()
		if got := sess.closeCalls.Load(); got != 0 {
			t.Fatalf("session closed below threshold: closeCalls=%d (pingCalls=%d)", got, sess.pingCalls.Load())
		}

		// Third tick → third failure reaches threshold: session closed.
		time.Sleep(interval)
		synctest.Wait()
		if got := sess.closeCalls.Load(); got != 1 {
			t.Fatalf("expected one Close at threshold, got closeCalls=%d (pingCalls=%d)", got, sess.pingCalls.Load())
		}
	})
}

// TestStartKeepalive_SuccessResetsFailures verifies that a successful ping
// resets the consecutive-failure counter, so an isolated failure between
// successes never accumulates toward the threshold.
func TestStartKeepalive_SuccessResetsFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const interval = 100 * time.Millisecond
		// fail, success, fail, fail, then success (the tail repeats): the run
		// never has 3 consecutive failures, so the session is never closed.
		sess := &scriptedKeepaliveSession{pingErrs: []error{
			errors.New("boom"), nil, errors.New("boom"), errors.New("boom"), nil,
		}}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		var cancel context.CancelFunc
		startKeepalive(sess, interval, 3, &cancel, logger)
		defer cancel()

		time.Sleep(6 * interval)
		synctest.Wait()
		if got := sess.closeCalls.Load(); got != 0 {
			t.Fatalf("session closed despite a success resetting the counter: closeCalls=%d (pingCalls=%d)", got, sess.pingCalls.Load())
		}
	})
}

func TestAddTool_DuplicateNoPanicAndNoDuplicate(t *testing.T) {
	// Adding the same tool pointer twice should not panic and should not
	// produce duplicates in the server's tool list.
	cs, _, cleanup := basicConnection(t, func(s *Server) {
		// Use two distinct Tool instances with the same name but different
		// descriptions to ensure the second replaces the first
		// This case was written specifically to reproduce a bug where duplicate tools where causing jsonschema errors
		t1 := &Tool{Name: "dup", Description: "first", InputSchema: &jsonschema.Schema{Type: "object"}}
		t2 := &Tool{Name: "dup", Description: "second", InputSchema: &jsonschema.Schema{Type: "object"}}
		s.AddTool(t1, nopHandler)
		s.AddTool(t2, nopHandler)
	})
	defer cleanup()

	ctx := context.Background()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	var gotDesc string
	for _, tt := range res.Tools {
		if tt.Name == "dup" {
			count++
			gotDesc = tt.Description
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one 'dup' tool, got %d", count)
	}
	if gotDesc != "second" {
		t.Fatalf("expected replaced tool to have description %q, got %q", "second", gotDesc)
	}
}

func TestSynchronousNotifications(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var toolsChanged atomic.Int32
		clientOpts := &ClientOptions{
			ToolListChangedHandler: func(ctx context.Context, req *ToolListChangedRequest) {
				toolsChanged.Add(1)
			},
			CreateMessageHandler: func(ctx context.Context, req *CreateMessageRequest) (*CreateMessageResult, error) {
				// See the comment after "from server" below.
				if n := toolsChanged.Load(); n != 1 {
					return nil, fmt.Errorf("got %d tools-changed notification, wanted 1", n)
				}
				// TODO(rfindley): investigate the error returned from this test if
				// CreateMessageResult is new(CreateMessageResult): it's a mysterious
				// unmarshalling error that we should improve.
				return &CreateMessageResult{Content: &TextContent{}}, nil
			},
		}
		client := NewClient(testImpl, clientOpts)

		var rootsChanged atomic.Bool
		serverOpts := &ServerOptions{
			RootsListChangedHandler: func(_ context.Context, req *RootsListChangedRequest) {
				rootsChanged.Store(true)
			},
		}
		server := NewServer(testImpl, serverOpts)
		addTool := func(s *Server) {
			AddTool(s, &Tool{Name: "tool"}, func(ctx context.Context, req *CallToolRequest, args any) (*CallToolResult, any, error) {
				if !rootsChanged.Load() {
					return nil, nil, fmt.Errorf("didn't get root change notification")
				}
				return new(CallToolResult), nil, nil
			})
		}
		ctx := context.Background()
		ct, st := NewInMemoryTransports()
		addTool(server)
		ss, err := server.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = ss.Close() })
		cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = cs.Close()
			ss.Wait()
		})

		t.Log("from client")
		{
			client.AddRoots(&Root{Name: "myroot", URI: "file://foo"})
			res, err := cs.CallTool(context.Background(), &CallToolParams{Name: "tool"})
			if err != nil {
				t.Fatalf("CallTool failed: %v", err)
			}
			if res.IsError {
				t.Errorf("tool error: %v", res.Content[0].(*TextContent).Text)
			}
		}

		t.Log("from server")
		{
			// Despite all this tool-changed activity, we expect only one notification.
			for range 10 {
				server.RemoveTools("tool")
				addTool(server)
			}

			time.Sleep(notificationDelay * 2) // Wait for delayed notification.
			if _, err := ss.CreateMessage(context.Background(), new(CreateMessageParams)); err != nil {
				t.Errorf("CreateMessage failed: %v", err)
			}
		}

	})
}

func TestNoDistributedDeadlock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// This test verifies that calls are asynchronous, and so it's not possible
		// to have a distributed deadlock.
		//
		// The setup creates potential deadlock for both the client and server: the
		// client sends a call to tool1, which itself calls createMessage, which in
		// turn calls tool2, which calls ping.
		//
		// If the server were not asynchronous, the call to tool2 would hang. If the
		// client were not asynchronous, the call to ping would hang.
		//
		// Such a scenario is unlikely in practice, but is still theoretically
		// possible, and in any case making tool calls asynchronous by default
		// delegates synchronization to the user.
		clientOpts := &ClientOptions{
			CreateMessageHandler: func(ctx context.Context, req *CreateMessageRequest) (*CreateMessageResult, error) {
				req.Session.CallTool(ctx, &CallToolParams{Name: "tool2"})
				return &CreateMessageResult{Content: &TextContent{}}, nil
			},
		}
		client := NewClient(testImpl, clientOpts)
		cs, _, cleanup := basicClientServerConnection(t, client, nil, func(s *Server) {
			AddTool(s, &Tool{Name: "tool1"}, func(ctx context.Context, req *CallToolRequest, args any) (*CallToolResult, any, error) {
				req.Session.CreateMessage(ctx, new(CreateMessageParams))
				return new(CallToolResult), nil, nil
			})
			AddTool(s, &Tool{Name: "tool2"}, func(ctx context.Context, req *CallToolRequest, args any) (*CallToolResult, any, error) {
				req.Session.Ping(ctx, nil)
				return new(CallToolResult), nil, nil
			})
		})
		defer cleanup()

		if _, err := cs.CallTool(context.Background(), &CallToolParams{Name: "tool1"}); err != nil {
			// should not deadlock
			t.Fatalf("CallTool failed: %v", err)
		}
	})
}

var testImpl = &Implementation{Name: "test", Version: "v1.0.0"}

// This test checks that when we use pointer types for tools, we get the same
// schema as when using the non-pointer types. It is too much of a footgun for
// there to be a difference (see #199 and #200).
//
// If anyone asks, we can add an option that controls how pointers are treated.
func TestPointerArgEquivalence(t *testing.T) {
	type input struct {
		In string `json:",omitempty"`
	}
	type output struct {
		Out string
	}
	cs, _, cleanup := basicConnection(t, func(s *Server) {
		// Add two equivalent tools, one of which operates in the 'pointer' realm,
		// the other of which does not.
		//
		// We handle a few different types of results, to assert they behave the
		// same in all cases.
		AddTool(s, &Tool{Name: "pointer"}, func(_ context.Context, req *CallToolRequest, in *input) (*CallToolResult, *output, error) {
			switch in.In {
			case "":
				return nil, nil, fmt.Errorf("must provide input")
			case "nil":
				return nil, nil, nil
			case "empty":
				return &CallToolResult{}, nil, nil
			case "ok":
				return &CallToolResult{}, &output{Out: "foo"}, nil
			default:
				panic("unreachable")
			}
		})
		AddTool(s, &Tool{Name: "nonpointer"}, func(_ context.Context, req *CallToolRequest, in input) (*CallToolResult, output, error) {
			switch in.In {
			case "":
				return nil, output{}, fmt.Errorf("must provide input")
			case "nil":
				return nil, output{}, nil
			case "empty":
				return &CallToolResult{}, output{}, nil
			case "ok":
				return &CallToolResult{}, output{Out: "foo"}, nil
			default:
				panic("unreachable")
			}
		})
	})
	defer cleanup()

	ctx := context.Background()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(tools.Tools), 2; got != want {
		t.Fatalf("got %d tools, want %d", got, want)
	}
	t0 := tools.Tools[0]
	t1 := tools.Tools[1]

	// First, check that the tool schemas don't differ.
	if diff := cmp.Diff(t0.InputSchema, t1.InputSchema); diff != "" {
		t.Errorf("input schemas do not match (-%s +%s):\n%s", t0.Name, t1.Name, diff)
	}
	if diff := cmp.Diff(t0.OutputSchema, t1.OutputSchema); diff != "" {
		t.Errorf("output schemas do not match (-%s +%s):\n%s", t0.Name, t1.Name, diff)
	}

	// Then, check that we handle empty input equivalently.
	for _, args := range []any{nil, struct{}{}} {
		r0, err := cs.CallTool(ctx, &CallToolParams{Name: t0.Name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		r1, err := cs.CallTool(ctx, &CallToolParams{Name: t1.Name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(r0, r1, ctrCmpOpts...); diff != "" {
			t.Errorf("CallTool(%v) with no arguments mismatch (-%s +%s):\n%s", args, t0.Name, t1.Name, diff)
		}
	}

	// Then, check that we handle different types of output equivalently.
	for _, in := range []string{"nil", "empty", "ok"} {
		t.Run(in, func(t *testing.T) {
			r0, err := cs.CallTool(ctx, &CallToolParams{Name: t0.Name, Arguments: input{In: in}})
			if err != nil {
				t.Fatal(err)
			}
			r1, err := cs.CallTool(ctx, &CallToolParams{Name: t1.Name, Arguments: input{In: in}})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(r0, r1, ctrCmpOpts...); diff != "" {
				t.Errorf("CallTool({\"In\": %q}) mismatch (-%s +%s):\n%s", in, t0.Name, t1.Name, diff)
			}
		})
	}
}

// ptr is a helper function to create pointers for schema constraints
func ptr[T any](v T) *T {
	return &v
}

func TestComplete(t *testing.T) {
	completionValues := []string{"python", "pytorch", "pyside"}

	serverOpts := &ServerOptions{
		CompletionHandler: func(_ context.Context, request *CompleteRequest) (*CompleteResult, error) {
			return &CompleteResult{
				Completion: CompletionResultDetails{
					Values: completionValues,
				},
			}, nil
		},
	}
	server := NewServer(testImpl, serverOpts)
	cs, _, cleanup := basicClientServerConnection(t, nil, server, func(s *Server) {})
	defer cleanup()

	result, err := cs.Complete(context.Background(), &CompleteParams{
		Argument: CompleteParamsArgument{
			Name:  "language",
			Value: "py",
		},
		Ref: &CompleteReference{
			Type: "ref/prompt",
			Name: "code_review",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(completionValues, result.Completion.Values); diff != "" {
		t.Errorf("Complete() mismatch (-want +got):\n%s", diff)
	}
}

// TestEmbeddedStructResponse performs a tool call to verify that a struct with
// an embedded pointer generates a correct, flattened JSON schema and that its
// response is validated successfully.
func TestEmbeddedStructResponse(t *testing.T) {
	type foo struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	// bar embeds foo
	type bar struct {
		*foo         // Embedded - should flatten in JSON
		Extra string `json:"extra"`
	}

	type response struct {
		Data bar `json:"data"`
	}

	// testTool demonstrates an embedded struct in its response.
	testTool := func(ctx context.Context, req *CallToolRequest, args any) (*CallToolResult, response, error) {
		response := response{
			Data: bar{
				foo: &foo{
					ID:   "foo",
					Name: "Test Foo",
				},
				Extra: "additional data",
			},
		}
		return nil, response, nil
	}
	ctx := context.Background()
	clientTransport, serverTransport := NewInMemoryTransports()
	server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
	AddTool(server, &Tool{
		Name: "test_embedded_struct",
	}, testTool)

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()

	client := NewClient(&Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	_, err = clientSession.CallTool(ctx, &CallToolParams{
		Name: "test_embedded_struct",
	})
	if err != nil {
		t.Errorf("CallTool() failed: %v", err)
	}
}

func TestToolErrorMiddleware(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	s := NewServer(testImpl, nil)
	AddTool(s, &Tool{
		Name:        "greet",
		Description: "say hi",
	}, sayHi)
	AddTool(s, &Tool{Name: "fail", InputSchema: &jsonschema.Schema{Type: "object"}},
		func(context.Context, *CallToolRequest, map[string]any) (*CallToolResult, any, error) {
			return nil, nil, errTestFailure
		})

	var middleErr error
	s.AddReceivingMiddleware(func(h MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			res, err := h(ctx, method, req)
			if err == nil {
				if ctr, ok := res.(*CallToolResult); ok {
					middleErr = ctr.GetError()
				}
			}
			return res, err
		}
	})
	_, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(&Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	_, err = clientSession.CallTool(ctx, &CallToolParams{
		Name:      "greet",
		Arguments: map[string]any{"Name": "al"},
	})
	if err != nil {
		t.Errorf("CallTool() failed: %v", err)
	}
	if middleErr != nil {
		t.Errorf("middleware got error %v, want nil", middleErr)
	}
	res, err := clientSession.CallTool(ctx, &CallToolParams{
		Name: "fail",
	})
	if err != nil {
		t.Errorf("CallTool() failed: %v", err)
	}
	if !res.IsError {
		t.Fatal("want error, got none")
	}
	// Clients can't see the error, because it isn't marshaled.
	if err := res.GetError(); err != nil {
		t.Fatalf("got %v, want nil", err)
	}
	if middleErr != errTestFailure {
		t.Errorf("middleware got err %v, want errTestFailure", middleErr)
	}
}

func TestSetErrorPreservesContent(t *testing.T) {
	for _, tt := range []struct {
		name        string
		content     []Content
		err         error
		wantContent string
	}{
		{
			name:        "nil content",
			err:         errors.New("internal failure"),
			wantContent: "internal failure",
		},
		{
			name:        "empty slice content",
			content:     []Content{},
			err:         errors.New("internal failure"),
			wantContent: "internal failure",
		},
		{
			name:        "existing content preserved",
			content:     []Content{&TextContent{Text: "user-friendly msg"}},
			err:         errors.New("db timeout"),
			wantContent: "user-friendly msg",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := CallToolResult{Content: tt.content}
			res.SetError(tt.err)
			if !res.IsError {
				t.Fatal("want IsError=true")
			}
			if got := res.Content[0].(*TextContent).Text; got != tt.wantContent {
				t.Errorf("Content text = %q, want %q", got, tt.wantContent)
			}
			if got := res.GetError(); got != tt.err {
				t.Errorf("GetError() = %v, want %v", got, tt.err)
			}
		})
	}
}

var ctrCmpOpts = []cmp.Option{
	cmpopts.IgnoreUnexported(CallToolResult{}, GetPromptResult{}, ReadResourceResult{}),
	// Server responses under the >= 2026-07-28 protocol carry an auto-populated
	// [MetaKeyServerInfo] entry; tests that compare result bodies against
	// hand-crafted expected values should ignore it.
	cmpopts.IgnoreFields(CallToolResult{}, "Meta"),
	cmpopts.IgnoreFields(GetPromptResult{}, "Meta"),
	cmpopts.IgnoreFields(ReadResourceResult{}, "Meta"),
}

// runSubscriptionsListenTest exercises the SEP-2575 auto-listen flow end-to-end
// against the supplied transport pair. It captures every notification and the
// acknowledgment the client sees, then asserts:
//
//   - the auto-listen issued by Client.Connect is acknowledged with a tagged
//     subscription ID;
//   - tool and prompt list-changed notifications are delivered to the matching
//     handlers, each carrying the same subscription ID as the ack;
//   - the subscription persists across multiple unrelated changes;
//   - cs.Close() ends the subscription and further changes don't deliver.
func runSubscriptionsListenTest(t *testing.T, client *Client, server *Server, ct Transport, events chan subListenEvent) {
	t.Helper()

	ctx, topCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer topCancel()

	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	waitFor := func(kind string) subListenEvent {
		t.Helper()
		select {
		case e := <-events:
			if e.kind != kind {
				t.Fatalf("got event %q (id=%s), want kind %q", e.kind, e.id, kind)
			}
			return e
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %q event", kind)
			return subListenEvent{}
		}
	}
	expectNoEvent := func(d time.Duration) {
		t.Helper()
		select {
		case e := <-events:
			t.Fatalf("unexpected event %q (id=%s)", e.kind, e.id)
		case <-time.After(d):
		}
	}

	ack := waitFor("ack")
	if ack.id == "" {
		t.Fatalf("acknowledgment missing subscription ID")
	}

	server.AddTool(&Tool{Name: "t2", InputSchema: &jsonschema.Schema{Type: "object"}}, nil)
	if e := waitFor("tool"); e.id != ack.id {
		t.Errorf("first tool notif id = %s, want %s", e.id, ack.id)
	}

	server.AddPrompt(&Prompt{Name: "p2"}, nil)
	if e := waitFor("prompt"); e.id != ack.id {
		t.Errorf("first prompt notif id = %s, want %s", e.id, ack.id)
	}

	server.AddTool(&Tool{Name: "t3", InputSchema: &jsonschema.Schema{Type: "object"}}, nil)
	if e := waitFor("tool"); e.id != ack.id {
		t.Errorf("second tool notif id = %s, want %s", e.id, ack.id)
	}
	server.AddPrompt(&Prompt{Name: "p3"}, nil)
	if e := waitFor("prompt"); e.id != ack.id {
		t.Errorf("second prompt notif id = %s, want %s", e.id, ack.id)
	}
	expectNoEvent(notificationDelay * 5)

	cs.Close()
	time.Sleep(50 * time.Millisecond)

	server.AddTool(&Tool{Name: "t4", InputSchema: &jsonschema.Schema{Type: "object"}}, nil)
	server.AddPrompt(&Prompt{Name: "p4"}, nil)
	expectNoEvent(notificationDelay * 20)
}

type subListenEvent struct {
	kind string // "ack", "tool", "prompt"
	id   string // subscription ID from _meta, stringified for cross-encoding equality
}

// newSubListenClient returns a client wired to push every ack and every
// list-changed notification it receives into events, tagged with the kind
// and the subscription ID extracted from _meta.
func newSubListenClient(events chan subListenEvent) *Client {
	asEvent := func(kind string, raw any) subListenEvent {
		return subListenEvent{kind, fmt.Sprint(raw)}
	}
	c := NewClient(testImpl, &ClientOptions{
		ToolListChangedHandler: func(_ context.Context, req *ToolListChangedRequest) {
			events <- asEvent("tool", req.Params.Meta[MetaKeySubscriptionID])
		},
		PromptListChangedHandler: func(_ context.Context, req *PromptListChangedRequest) {
			events <- asEvent("prompt", req.Params.Meta[MetaKeySubscriptionID])
		},
	})
	c.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method == notificationSubscriptionsAck {
				if cr, ok := req.(*ClientRequest[*SubscriptionsAcknowledgedParams]); ok && cr.Params != nil {
					events <- asEvent("ack", cr.Params.Meta[MetaKeySubscriptionID])
				}
			}
			return next(ctx, method, req)
		}
	})
	return c
}

func newSubListenServer() *Server {
	s := NewServer(testImpl, nil)
	AddTool(s, &Tool{Name: "t1"}, sayHi)
	s.AddPrompt(&Prompt{Name: "p1"}, nil)
	return s
}

// TestSubscriptionsListen_InMemory exercises the listen flow over the
// session-shared in-memory transport (semantically equivalent to STDIO).
// Cancellation here propagates via notifications/cancelled.
func TestSubscriptionsListen_InMemory(t *testing.T) {
	events := make(chan subListenEvent, 64)
	server := newSubListenServer()
	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()
	runSubscriptionsListenTest(t, newSubListenClient(events), server, ct, events)
}

// TestSubscriptionsListen_Streamable exercises the listen flow over a
// stateless HTTP server (SEP-2575). Each listen rides its own SSE response
// stream; cs.Close() tears it down.
func TestSubscriptionsListen_Streamable(t *testing.T) {
	events := make(chan subListenEvent, 64)
	server := newSubListenServer()
	handler := NewStreamableHTTPHandler(
		func(*http.Request) *Server { return server },
		&StreamableHTTPOptions{Stateless: true},
	)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()
	runSubscriptionsListenTest(t, newSubListenClient(events), server,
		&StreamableClientTransport{Endpoint: httpServer.URL}, events)
}

// TestSubscriptionsListen_NoHandlersNoListen verifies that a new-protocol
// client without any list-changed handlers registered does not open an
// auto-listen on connect, and therefore does not receive any acknowledgment
// or downstream notifications.
func TestSubscriptionsListen_NoHandlersNoListen(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events := make(chan subListenEvent, 8)
	server := newSubListenServer()
	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	c := NewClient(testImpl, nil)
	c.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method == notificationSubscriptionsAck {
				events <- subListenEvent{"ack", ""}
			}
			return next(ctx, method, req)
		}
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	server.AddTool(&Tool{Name: "t2", InputSchema: &jsonschema.Schema{Type: "object"}}, nil)

	select {
	case e := <-events:
		t.Fatalf("unexpected event %q on no-handler client", e.kind)
	case <-time.After(notificationDelay * 10):
	}
}

// TestSubscriptionsListen_MissingNotifications verifies that a
// subscriptions/listen request without the required "notifications" field
// is rejected with an invalid params error.
func TestSubscriptionsListen_MissingNotifications(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := newSubListenServer()
	_, st := NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	// Invoke the server handler directly with a nil Notifications field,
	// simulating a request whose params omit the required field on the wire.
	// The client-side subscriptionsListen does not await the RPC response
	// (the call's lifetime is the notification stream), so we assert the
	// server-side behavior at the handler level.
	id, err := jsonrpc.MakeID("test-1")
	if err != nil {
		t.Fatalf("MakeID: %v", err)
	}
	reqCtx := context.WithValue(ctx, idContextKey{}, id)
	req := &SubscriptionsListenRequest{
		Session: ss,
		Params:  &SubscriptionsListenParams{}, // Notifications is nil
	}
	_, err = server.subscriptionsListen(reqCtx, req)
	if err == nil {
		t.Fatal("expected error for missing notifications field, got nil")
	}
	var jerr *jsonrpc.Error
	if !errors.As(err, &jerr) {
		t.Fatalf("expected *jsonrpc.Error, got %T: %v", err, err)
	}
	if jerr.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("error code = %d, want %d", jerr.Code, jsonrpc.CodeInvalidParams)
	}
}

// resourceSubServer builds a server that advertises resource subscriptions
// and records every Subscribe/Unsubscribe handler invocation through chans.
func resourceSubServer(t *testing.T, subCh, unsubCh chan string) *Server {
	t.Helper()
	s := NewServer(testImpl, &ServerOptions{
		SubscribeHandler: func(_ context.Context, r *SubscribeRequest) error {
			subCh <- r.Params.URI
			return nil
		},
		UnsubscribeHandler: func(_ context.Context, r *UnsubscribeRequest) error {
			unsubCh <- r.Params.URI
			return nil
		},
	})
	s.AddResource(&Resource{Name: "r1", URI: "file:///r1"}, nil)
	return s
}

// resourceSubEvent is one delivered notifications/resources/updated.
type resourceSubEvent struct {
	uri string
	id  string // _meta subscription ID, stringified
}

// TestResourceSubscriptionsSEP2575_Streamable verifies the Subscribe ->
// ResourceUpdated path on a stateless Streamable HTTP server.
//
// Caveat: per-subscription Unsubscribe is intentionally NOT verified here.
// In stateless Streamable HTTP mode the subscriptions/listen handler blocks
// on its request context, and neither the HTTP POST disconnect nor the
// separate notifications/cancelled POST currently propagates to that
// handler's context. The handler only unwinds when the server next attempts
// a write to the (now-dead) SSE stream and the writeErr branch in the
// jsonrpc2 layer cancels the in-flight request. To keep the test
// hermetic we therefore trigger a write at the end by adding a resource,
// which fires notifications/resources/list_changed on the auto-listen path
// (if any) and on the per-URI listen, causing the listen handler to unwind.
// The spec-correct fix is to plumb the POST's request context down to the
// subscriptionsListen handler so HTTP disconnect is observed directly; this
// is tracked separately.
func TestResourceSubscriptions_Streamable(t *testing.T) {

	subCh := make(chan string, 8)
	unsubCh := make(chan string, 8)
	events := make(chan resourceSubEvent, 16)

	server := resourceSubServer(t, subCh, unsubCh)
	handler := NewStreamableHTTPHandler(
		func(*http.Request) *Server { return server },
		&StreamableHTTPOptions{Stateless: true},
	)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c := NewClient(testImpl, &ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, req *ResourceUpdatedNotificationRequest) {
			id := ""
			if req.Params != nil && req.Params.Meta != nil {
				id = fmt.Sprint(req.Params.Meta[MetaKeySubscriptionID])
			}
			events <- resourceSubEvent{uri: req.Params.URI, id: id}
		},
	})
	cs, err := c.Connect(ctx, &StreamableClientTransport{Endpoint: httpServer.URL},
		&ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	if err := cs.Subscribe(ctx, &SubscribeParams{URI: "file:///r1"}); err != nil {
		t.Fatalf("subscribe r1: %v", err)
	}
	select {
	case got := <-subCh:
		if got != "file:///r1" {
			t.Fatalf("got URI %q, want %q", got, "file:///r1")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SubscribeHandler")
	}

	server.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{URI: "file:///r1"})
	select {
	case e := <-events:
		if e.uri != "file:///r1" {
			t.Fatalf("got URI %q, want %q", e.uri, "file:///r1")
		}
		if e.id == "" || e.id == "<nil>" {
			t.Fatalf("missing subscription ID on update (got %q)", e.id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for resource update")
	}

	// See test header comment for the explanation of this teardown ritual:
	// close the client, then drop server-side TCP, then drive a write that
	// will fail (any extra ResourceUpdated for our URI), to unblock the
	// in-flight listen handler so httpServer.Close can return.
	_ = cs.Close()
	httpServer.CloseClientConnections()
	server.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{URI: "file:///r1"})
	httpServer.Close()
}

// TestResourceSubscriptions_InMemory mirrors TestResourceSubscriptions_Streamable
// over an in-memory (stdio-equivalent) transport: the per-URI Subscribe path
// uses notifications/cancelled rather than HTTP disconnect for teardown.
func TestResourceSubscriptions_InMemory(t *testing.T) {

	subCh := make(chan string, 8)
	unsubCh := make(chan string, 8)
	events := make(chan resourceSubEvent, 16)

	server := resourceSubServer(t, subCh, unsubCh)
	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient(testImpl, &ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, req *ResourceUpdatedNotificationRequest) {
			id := ""
			if req.Params != nil && req.Params.Meta != nil {
				id = fmt.Sprint(req.Params.Meta[MetaKeySubscriptionID])
			}
			events <- resourceSubEvent{uri: req.Params.URI, id: id}
		},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	if err := cs.Subscribe(ctx, &SubscribeParams{URI: "file:///r1"}); err != nil {
		t.Fatalf("subscribe r1: %v", err)
	}
	waitURI := func(ch chan string, want string) {
		t.Helper()
		select {
		case got := <-ch:
			if got != want {
				t.Fatalf("got URI %q, want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for URI %q", want)
		}
	}
	waitURI(subCh, "file:///r1")

	server.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{URI: "file:///r1"})
	select {
	case e := <-events:
		if e.uri != "file:///r1" {
			t.Fatalf("got URI %q, want %q", e.uri, "file:///r1")
		}
		if e.id == "" || e.id == "<nil>" {
			t.Fatalf("missing subscription ID on update (got %q)", e.id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for resource update")
	}

	// On stdio/in-memory, Unsubscribe sends notifications/cancelled which
	// reaches the server preempter, cancels the listen handler ctx, and
	// triggers the UnsubscribeHandler.
	if err := cs.Unsubscribe(ctx, &UnsubscribeParams{URI: "file:///r1"}); err != nil {
		t.Fatalf("unsubscribe r1: %v", err)
	}
	waitURI(unsubCh, "file:///r1")

	// Updates for an unsubscribed URI MUST NOT be delivered. Give the server
	// a moment to process the cancellation first.
	time.Sleep(50 * time.Millisecond)
	server.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{URI: "file:///r1"})
	select {
	case e := <-events:
		t.Fatalf("unexpected resource update after unsubscribe: %q (id=%s)", e.uri, e.id)
	case <-time.After(notificationDelay * 10):
	}
}

// TestResourceSubscriptions_Subscribe_Idempotent verifies that calling
// Subscribe twice for the same URI in the same session is a no-op for the
// second call: it returns nil without invoking SubscribeHandler again and
// without opening a second listen stream.
func TestResourceSubscriptions_Subscribe_Idempotent(t *testing.T) {

	subCh := make(chan string, 8)
	unsubCh := make(chan string, 8)

	server := resourceSubServer(t, subCh, unsubCh)
	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient(testImpl, &ClientOptions{
		ResourceUpdatedHandler: func(context.Context, *ResourceUpdatedNotificationRequest) {},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	if err := cs.Subscribe(ctx, &SubscribeParams{URI: "file:///r1"}); err != nil {
		t.Fatalf("first subscribe: %v", err)
	}
	select {
	case got := <-subCh:
		if got != "file:///r1" {
			t.Fatalf("got URI %q, want %q", got, "file:///r1")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first SubscribeHandler")
	}

	// Second Subscribe for the same URI returns nil and does NOT fire
	// SubscribeHandler again.
	if err := cs.Subscribe(ctx, &SubscribeParams{URI: "file:///r1"}); err != nil {
		t.Fatalf("second subscribe should be no-op, got error: %v", err)
	}
	select {
	case got := <-subCh:
		t.Fatalf("duplicate Subscribe should not re-invoke SubscribeHandler (got %q)", got)
	case <-time.After(notificationDelay * 10):
	}

	// Subsequent Unsubscribe still works (verifies the URI is tracked
	// correctly even though the second Subscribe was a no-op).
	if err := cs.Unsubscribe(ctx, &UnsubscribeParams{URI: "file:///r1"}); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	select {
	case <-unsubCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for UnsubscribeHandler")
	}
}

// TestResourceSubscriptions_MultipleURIs verifies that two concurrent
// Subscribe calls on the same session each open their own independent listen
// stream with a distinct subscription ID. Unsubscribing one does not affect
// the other.
func TestResourceSubscriptions_MultipleURIs(t *testing.T) {

	subCh := make(chan string, 8)
	unsubCh := make(chan string, 8)
	events := make(chan resourceSubEvent, 16)

	server := resourceSubServer(t, subCh, unsubCh)
	server.AddResource(&Resource{Name: "r2", URI: "file:///r2"}, nil)
	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient(testImpl, &ClientOptions{
		ResourceUpdatedHandler: func(_ context.Context, req *ResourceUpdatedNotificationRequest) {
			id := ""
			if req.Params != nil && req.Params.Meta != nil {
				id = fmt.Sprint(req.Params.Meta[MetaKeySubscriptionID])
			}
			events <- resourceSubEvent{uri: req.Params.URI, id: id}
		},
	})
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	if err := cs.Subscribe(ctx, &SubscribeParams{URI: "file:///r1"}); err != nil {
		t.Fatalf("subscribe r1: %v", err)
	}
	if err := cs.Subscribe(ctx, &SubscribeParams{URI: "file:///r2"}); err != nil {
		t.Fatalf("subscribe r2: %v", err)
	}

	// Each Subscribe MUST fire its own SubscribeHandler invocation.
	gotURIs := map[string]bool{}
	for range 2 {
		select {
		case got := <-subCh:
			gotURIs[got] = true
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for SubscribeHandler")
		}
	}
	if !gotURIs["file:///r1"] || !gotURIs["file:///r2"] {
		t.Fatalf("missing SubscribeHandler invocation; got %v", gotURIs)
	}

	// Each update delivers exactly one event tagged with that URI's distinct
	// subscription ID.
	server.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{URI: "file:///r1"})
	ev1 := <-events
	server.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{URI: "file:///r2"})
	ev2 := <-events
	if ev1.uri != "file:///r1" || ev2.uri != "file:///r2" {
		t.Fatalf("got URIs %q and %q, want r1 and r2", ev1.uri, ev2.uri)
	}
	if ev1.id == "" || ev2.id == "" {
		t.Fatalf("missing subscription IDs: r1=%q r2=%q", ev1.id, ev2.id)
	}
	if ev1.id == ev2.id {
		t.Fatalf("r1 and r2 should have distinct subscription IDs, both = %q", ev1.id)
	}

	// Unsubscribe r1 only. r2 keeps working.
	if err := cs.Unsubscribe(ctx, &UnsubscribeParams{URI: "file:///r1"}); err != nil {
		t.Fatalf("unsubscribe r1: %v", err)
	}
	select {
	case got := <-unsubCh:
		if got != "file:///r1" {
			t.Fatalf("got unsubscribe URI %q, want %q", got, "file:///r1")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for UnsubscribeHandler")
	}
	time.Sleep(50 * time.Millisecond) // let server-side cancellation settle

	server.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{URI: "file:///r1"})
	server.ResourceUpdated(ctx, &ResourceUpdatedNotificationParams{URI: "file:///r2"})

	// Only the r2 update should arrive.
	select {
	case e := <-events:
		if e.uri != "file:///r2" || e.id != ev2.id {
			t.Fatalf("post-unsubscribe got %q (id=%s), want r2 (id=%s)", e.uri, e.id, ev2.id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for r2 update after r1 unsubscribe")
	}
	select {
	case e := <-events:
		t.Fatalf("unexpected event after r1 unsubscribe: %q (id=%s)", e.uri, e.id)
	case <-time.After(notificationDelay * 10):
	}

	// Explicit Unsubscribe r2 to verify it still works independently.
	if err := cs.Unsubscribe(ctx, &UnsubscribeParams{URI: "file:///r2"}); err != nil {
		t.Fatalf("unsubscribe r2: %v", err)
	}
	select {
	case <-unsubCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for r2 UnsubscribeHandler")
	}
}

// TestSubscriptionsListen_MultipleSessions verifies that two concurrent
// client sessions on the same server are isolated: a list-changed
// notification is fanned out to BOTH sessions, each with its own subscription
// ID; closing one session does not affect deliveries to the other.
func TestSubscriptionsListen_MultipleSessions(t *testing.T) {

	server := newSubListenServer()

	// Open two clients on the same server.
	open := func(t *testing.T) (*ClientSession, chan subListenEvent, *ServerSession) {
		t.Helper()
		events := make(chan subListenEvent, 16)
		ct, st := NewInMemoryTransports()
		ss, err := server.Connect(context.Background(), st, nil)
		if err != nil {
			t.Fatalf("server connect: %v", err)
		}
		c := newSubListenClient(events)
		cs, err := c.Connect(context.Background(), ct,
			&ClientSessionOptions{protocolVersion: protocolVersion20260728})
		if err != nil {
			t.Fatalf("client connect: %v", err)
		}
		return cs, events, ss
	}
	csA, evA, ssA := open(t)
	defer ssA.Close()
	csB, evB, ssB := open(t)
	defer ssB.Close()

	waitFor := func(ch chan subListenEvent, kind string) subListenEvent {
		t.Helper()
		select {
		case e := <-ch:
			if e.kind != kind {
				t.Fatalf("got event %q, want %q", e.kind, kind)
			}
			return e
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %q event", kind)
			return subListenEvent{}
		}
	}
	ackA := waitFor(evA, "ack")
	ackB := waitFor(evB, "ack")
	// Request IDs are per-connection, so both sessions may legitimately use
	// the same ID number; we only require that each session's own
	// notifications carry its own ack ID.

	// A single change fans out to BOTH sessions, each tagged with that
	// session's own ack ID.
	server.AddTool(&Tool{Name: "t2", InputSchema: &jsonschema.Schema{Type: "object"}}, nil)
	gotA := waitFor(evA, "tool")
	gotB := waitFor(evB, "tool")
	if gotA.id != ackA.id {
		t.Errorf("session A: tool notif id=%s, want %s", gotA.id, ackA.id)
	}
	if gotB.id != ackB.id {
		t.Errorf("session B: tool notif id=%s, want %s", gotB.id, ackB.id)
	}

	// Close A; B's subscription must keep delivering.
	csA.Close()
	time.Sleep(50 * time.Millisecond)
	server.AddTool(&Tool{Name: "t3", InputSchema: &jsonschema.Schema{Type: "object"}}, nil)
	gotB2 := waitFor(evB, "tool")
	if gotB2.id != ackB.id {
		t.Errorf("session B after A closed: tool notif id=%s, want %s", gotB2.id, ackB.id)
	}
	select {
	case e := <-evA:
		t.Fatalf("session A unexpected event after Close: %q", e.kind)
	case <-time.After(notificationDelay * 5):
	}

	csB.Close()
}

// TestSubscriptionsListen_ResourceListChanged covers the resources/list_changed
// auto-listen branch (the other two list-changed types are covered by
// runSubscriptionsListenTest, but resources is not).
func TestSubscriptionsListen_ResourceListChanged(t *testing.T) {
	events := make(chan subListenEvent, 16)

	server := NewServer(testImpl, nil)
	server.AddResource(&Resource{Name: "r1", URI: "file:///r1"}, nil)

	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	asEvent := func(kind string, raw any) subListenEvent {
		return subListenEvent{kind, fmt.Sprint(raw)}
	}
	c := NewClient(testImpl, &ClientOptions{
		ResourceListChangedHandler: func(_ context.Context, req *ResourceListChangedRequest) {
			id := any(nil)
			if req.Params != nil && req.Params.Meta != nil {
				id = req.Params.Meta[MetaKeySubscriptionID]
			}
			events <- asEvent("resource", id)
		},
	})
	c.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method == notificationSubscriptionsAck {
				if cr, ok := req.(*ClientRequest[*SubscriptionsAcknowledgedParams]); ok && cr.Params != nil {
					events <- asEvent("ack", cr.Params.Meta[MetaKeySubscriptionID])
				}
			}
			return next(ctx, method, req)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	waitFor := func(kind string) subListenEvent {
		t.Helper()
		select {
		case e := <-events:
			if e.kind != kind {
				t.Fatalf("got event %q, want %q", e.kind, kind)
			}
			return e
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %q", kind)
			return subListenEvent{}
		}
	}
	ack := waitFor("ack")
	if ack.id == "" {
		t.Fatal("acknowledgment missing subscription ID")
	}
	server.AddResource(&Resource{Name: "r2", URI: "file:///r2"}, nil)
	got := waitFor("resource")
	if got.id != ack.id {
		t.Errorf("resource notif id=%s, want %s", got.id, ack.id)
	}
	cs.Close()
}

// TestSubscriptionsListen_DisconnectScrubsMaps verifies that closing a
// session removes its entries from the server's three per-type subscription
// maps via Server.disconnect.
func TestSubscriptionsListen_DisconnectScrubsMaps(t *testing.T) {
	events := make(chan subListenEvent, 16)
	server := newSubListenServer()

	ct, st := NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	c := newSubListenClient(events)
	cs, err := c.Connect(context.Background(), ct,
		&ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	// Wait for the auto-listen to actually register on the server side.
	select {
	case e := <-events:
		if e.kind != "ack" {
			t.Fatalf("got %q, want ack", e.kind)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ack")
	}

	// Server should now have this session in tools+prompts maps (the fixture
	// client opts in to both via newSubListenClient).
	server.mu.Lock()
	if _, ok := server.toolChangeSubscriptions[ss]; !ok {
		server.mu.Unlock()
		t.Fatal("session missing from toolChangeSubscriptions before close")
	}
	if _, ok := server.promptChangeSubscriptions[ss]; !ok {
		server.mu.Unlock()
		t.Fatal("session missing from promptChangeSubscriptions before close")
	}
	server.mu.Unlock()

	cs.Close()
	ss.Close()
	// Closures complete asynchronously on the server side.
	deadline := time.Now().Add(5 * time.Second)
	for {
		server.mu.Lock()
		_, inTool := server.toolChangeSubscriptions[ss]
		_, inPrompt := server.promptChangeSubscriptions[ss]
		_, inResource := server.resourceChangeSubscriptions[ss]
		server.mu.Unlock()
		if !inTool && !inPrompt && !inResource {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscription maps not scrubbed after Close: tool=%v prompt=%v resource=%v",
				inTool, inPrompt, inResource)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCustomMethods(t *testing.T) {
	type searchParams struct {
		ParamsBase
		Query string `json:"query"`
		Limit int    `json:"limit,omitempty"`
	}

	type searchResult struct {
		ResultBase
		Hits  []string `json:"hits"`
		Total int      `json:"total"`
	}

	callCustom := func(ctx context.Context, conn *jsonrpc2.Connection, method string, params, result any) error {
		return conn.Call(ctx, method, params).Await(ctx, result)
	}

	ctx := context.Background()
	s := NewServer(testImpl, nil)

	if err := AddReceivingCustomMethod(s, "acme/search", func(ctx context.Context, ss *ServerSession, params *searchParams) (*searchResult, error) {
		hits := []string{"result for " + params.Query}
		return &searchResult{
			Hits:  hits,
			Total: len(hits),
		}, nil
	}); err != nil {
		t.Fatal(err)
	}

	ct, st := NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	c := NewClient(testImpl, nil)
	if err := AddSendingCustomMethod[*searchParams, *searchResult](c, "acme/search"); err != nil {
		t.Fatal(err)
	}

	cs, err := c.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	// Test raw JSON-RPC call.
	var result1 searchResult
	if err := callCustom(ctx, cs.getConn(), "acme/search", &searchParams{Query: "hello", Limit: 10}, &result1); err != nil {
		t.Fatal(err)
	}
	if len(result1.Hits) != 1 || result1.Hits[0] != "result for hello" {
		t.Errorf("raw call: unexpected hits: %v", result1.Hits)
	}
	if result1.Total != 1 {
		t.Errorf("raw call: unexpected total: %d", result1.Total)
	}

	// Test the typed CallCustomMethod helper.
	result2, err := CallCustomMethod[*searchParams, *searchResult](
		ctx, cs, "acme/search", &searchParams{Query: "world"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result2.Hits) != 1 || result2.Hits[0] != "result for world" {
		t.Errorf("CallCustomMethod: unexpected hits: %v", result2.Hits)
	}
	if result2.Total != 1 {
		t.Errorf("CallCustomMethod: unexpected total: %d", result2.Total)
	}

	// CallCustomMethod must reject methods that were never registered.
	if _, err := CallCustomMethod[*searchParams, *searchResult](
		ctx, cs, "acme/unregistered", &searchParams{Query: "x"}); err == nil {
		t.Error("CallCustomMethod: expected error for unregistered method, got nil")
	}
}

func TestAddCustomMethodRejectsStandardMethods(t *testing.T) {
	type params struct{ ParamsBase }
	type result struct{ ResultBase }

	t.Run("server", func(t *testing.T) {
		s := NewServer(testImpl, nil)
		err := AddReceivingCustomMethod(s, "tools/call",
			func(ctx context.Context, ss *ServerSession, p *params) (*result, error) {
				return &result{}, nil
			})
		if err == nil {
			t.Fatal("AddReceivingCustomMethod: expected error when shadowing a standard method, got nil")
		}
	})

	t.Run("client", func(t *testing.T) {
		c := NewClient(testImpl, nil)
		if err := AddSendingCustomMethod[*params, *result](c, "tools/call"); err == nil {
			t.Fatal("AddSendingCustomMethod: expected error when shadowing a standard method, got nil")
		}
	})
}

// TestCallCustomMethodTypedNilParams exercises the typed-nil params path.
// User param types embed ParamsBase, so the inherited isNil forwarder would
// dereference a typed-nil outer if injectRequestMeta were called with it.
// CallCustomMethod must allocate a fresh value before the meta-injection step.
func TestCallCustomMethodTypedNilParams(t *testing.T) {
	type pingParams struct{ ParamsBase }
	type pingResult struct{ ResultBase }

	ctx := context.Background()
	s := NewServer(testImpl, nil)
	if err := AddReceivingCustomMethod(s, "acme/ping",
		func(ctx context.Context, ss *ServerSession, p *pingParams) (*pingResult, error) {
			return &pingResult{}, nil
		}); err != nil {
		t.Fatal(err)
	}
	ct, st := NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	c := NewClient(testImpl, nil)
	if err := AddSendingCustomMethod[*pingParams, *pingResult](c, "acme/ping"); err != nil {
		t.Fatal(err)
	}
	cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	var typedNil *pingParams
	if _, err := CallCustomMethod[*pingParams, *pingResult](ctx, cs, "acme/ping", typedNil); err != nil {
		t.Fatalf("CallCustomMethod with typed-nil params: %v", err)
	}
}
