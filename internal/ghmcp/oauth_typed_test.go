package ghmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/oauth"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type oauthProbeOutput struct {
	Status string `json:"status"`
}

func oauthRegisteredSession(
	t *testing.T,
	typed bool,
	fake oauthAuthenticator,
	protocol string,
	options *mcp.ClientOptions,
	toolCalls *int,
	captures ...chan json.RawMessage,
) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "oauth-test", Version: "test"}, nil)
	middleware := createOAuthToolMiddleware(fake, discardLogger())
	if typed {
		tool := inventory.NewServerToolWithContextHandler(
			mcp.Tool{Name: probeToolName},
			inventory.ToolsetMetadata{ID: "test"},
			func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, oauthProbeOutput, error) {
				(*toolCalls)++
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "tool-ran"}}},
					oauthProbeOutput{Status: "tool-ran"}, nil
			},
		)
		tool.RegisterFunc(server, nil, middleware)
	} else {
		server.AddTool(&mcp.Tool{Name: probeToolName, InputSchema: &jsonschema.Schema{Type: "object"}},
			middleware(func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				(*toolCalls)++
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "tool-ran"}}}, nil
			}))
	}
	return connectOAuthRegisteredServer(t, server, protocol, options, captures...)
}

func connectOAuthRegisteredServer(t *testing.T, server *mcp.Server, protocol string, options *mcp.ClientOptions, captures ...chan json.RawMessage) *mcp.ClientSession {
	t.Helper()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "oauth-client", Version: "test"}, options)
	var transport mcp.Transport = ct
	if len(captures) > 0 {
		transport = oauthCaptureTransport{Transport: ct, responses: captures[0]}
	}
	cs, err := client.Connect(context.Background(), transport, &mcp.ClientSessionOptions{ProtocolVersion: protocol})
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

type oauthCaptureTransport struct {
	mcp.Transport
	responses chan json.RawMessage
}

func (t oauthCaptureTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	connection, err := t.Transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return oauthCaptureConnection{Connection: connection, responses: t.responses}, nil
}

type oauthCaptureConnection struct {
	mcp.Connection
	responses chan json.RawMessage
}

func (c oauthCaptureConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	message, err := c.Connection.Read(ctx)
	if err == nil {
		if response, ok := message.(*jsonrpc.Response); ok && response.Result != nil {
			c.responses <- append(json.RawMessage(nil), response.Result...)
		}
	}
	return message, err
}

func oauthPendingAuthenticator() *fakeAuthenticator {
	return &fakeAuthenticator{
		outcome: &oauth.Outcome{
			UserAction: &oauth.UserAction{URL: "https://example.com/auth", Message: "Authorize, then retry."},
			FlowID:     "flow-1",
		},
		tokenAfterAwait: true,
		cancelResult:    true,
	}
}

func TestOAuthTypedRegistration(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "untyped-control"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			urlCaps := &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}}}
			t.Run("wire-input-required", func(t *testing.T) {
				fake := oauthPendingAuthenticator()
				calls := 0
				captures := make(chan json.RawMessage, 8)
				session := oauthRegisteredSession(t, typed, fake, "", &mcp.ClientOptions{
					Capabilities: urlCaps, MultiRoundTrip: &mcp.MultiRoundTripOptions{Disabled: true},
				}, &calls, captures)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: probeToolName})
				require.NoError(t, err)
				<-captures // initialize
				wire := <-captures
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(wire, &fields))
				assert.JSONEq(t, `"input_required"`, string(fields["resultType"]), "wire: %s", wire)
				assert.True(t, result.NeedsInput())
				assert.Contains(t, result.InputRequests, oauthElicitIDPrefix+"flow-1")
				assert.Nil(t, result.StructuredContent)
				assert.Zero(t, calls)
				t.Logf("raw tools/call response: %s", wire)
			})
			for _, action := range []string{"accept", "decline"} {
				t.Run(action, func(t *testing.T) {
					fake := oauthPendingAuthenticator()
					calls, prompts := 0, 0
					session := oauthRegisteredSession(t, typed, fake, "", &mcp.ClientOptions{
						Capabilities: urlCaps,
						ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
							prompts++
							assert.Equal(t, "url", req.Params.Mode)
							assert.Equal(t, "https://example.com/auth", req.Params.URL)
							return &mcp.ElicitResult{Action: action}, nil
						},
					}, &calls)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: probeToolName})
					require.NoError(t, err)
					assert.Equal(t, 1, prompts)
					require.Len(t, result.Content, 1)
					assert.False(t, result.NeedsInput())
					wire, err := json.Marshal(result)
					require.NoError(t, err)
					assert.Contains(t, string(wire), `"resultType":"complete"`)
					if action == "accept" {
						assert.Equal(t, 1, calls)
						assert.Equal(t, 1, fake.awaitCalls)
						assert.Equal(t, "flow-1", fake.lastAwaitFlowID)
						assert.Zero(t, fake.cancelCalls)
						if typed {
							output, err := json.Marshal(result.StructuredContent)
							require.NoError(t, err)
							assert.JSONEq(t, `{"status":"tool-ran"}`, string(output))
						}
					} else {
						assert.Zero(t, calls)
						assert.Zero(t, fake.awaitCalls)
						assert.Equal(t, 1, fake.cancelCalls)
						assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "declined")
						assert.Nil(t, result.StructuredContent)
					}
				})
			}
			for _, protocol := range []string{"", "2025-11-25"} {
				t.Run("manual-fallback/"+protocol, func(t *testing.T) {
					fake := oauthPendingAuthenticator()
					calls := 0
					session := oauthRegisteredSession(t, typed, fake, protocol, &mcp.ClientOptions{
						Capabilities: &mcp.ClientCapabilities{},
					}, &calls)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: probeToolName})
					require.NoError(t, err)
					require.Len(t, result.Content, 1)
					assert.Equal(t, "Authorize, then retry.", result.Content[0].(*mcp.TextContent).Text)
					assert.False(t, result.NeedsInput())
					assert.Nil(t, result.StructuredContent)
					assert.Zero(t, calls)
					assert.Zero(t, fake.awaitCalls)
					assert.Zero(t, fake.cancelCalls)
					assert.Equal(t, protocol != "", fake.lastPrompter != nil)
				})
			}
		})
	}
}

type legacyPromptAuthenticator struct {
	fakeAuthenticator
}

func (f *legacyPromptAuthenticator) Authenticate(ctx context.Context, prompter oauth.Prompter) (*oauth.Outcome, error) {
	f.authCalls++
	f.lastPrompter = prompter
	if err := prompter.PromptURL(ctx, oauth.Prompt{URL: "https://example.com/auth", Message: "Authorize"}); err != nil {
		return nil, err
	}
	f.hasToken = true
	return nil, nil
}

func TestOAuthTypedRegistrationLegacyElicitation(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "untyped-control", true: "typed"}[typed], func(t *testing.T) {
			fake := &legacyPromptAuthenticator{}
			calls, prompts := 0, 0
			session := oauthRegisteredSession(t, typed, fake, "2025-11-25", &mcp.ClientOptions{
				Capabilities: &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}}},
				ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
					prompts++
					return &mcp.ElicitResult{Action: "accept"}, nil
				},
			}, &calls)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: probeToolName})
			require.NoError(t, err)
			assert.Equal(t, 1, prompts)
			assert.Equal(t, 1, calls)
			assert.Equal(t, 1, fake.authCalls)
			assert.False(t, result.NeedsInput())
			assert.Nil(t, result.StructuredContent)
			require.Len(t, result.Content, 1)
			assert.Equal(t, "tool-ran", result.Content[0].(*mcp.TextContent).Text)
		})
	}
}

func TestOAuthTypedHTTPHeaderBoundary(t *testing.T) {
	for _, tc := range []struct {
		name      string
		arguments string
		owner     string
		repo      string
		mismatch  bool
		valid     bool
	}{
		{"owner-mismatch", `{"owner":"octo","repo":"hello","mode":"valid"}`, "other", "hello", true, true},
		{"repo-mismatch", `{"owner":"octo","repo":"hello","mode":"valid"}`, "octo", "other", true, true},
		{"missing-header", `{"owner":"octo","repo":"hello","mode":"valid"}`, "", "hello", true, true},
		{"matching", `{"owner":"octo","repo":"hello","mode":"valid"}`, "octo", "hello", false, true},
		{"missing-required", `{}`, "", "", false, false},
		{"invalid-enum", `{"owner":"octo","repo":"hello","mode":"invalid"}`, "octo", "hello", false, false},
		{"invalid-owner-type", `{"owner":42,"repo":"hello","mode":"valid"}`, "42", "hello", false, false},
		{"null-owner", `{"owner":null,"repo":"hello","mode":"valid"}`, "", "hello", false, false},
		{"non-object", `[]`, "", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := oauthPendingAuthenticator()
			handlerCalls, receivingCalls, preflightCalls, normalizerCalls := 0, 0, 0, 0
			type input struct {
				Owner string `json:"owner"`
				Repo  string `json:"repo"`
				Mode  string `json:"mode"`
			}
			tool := inventory.NewServerToolWithContextHandlerAndSchemaOptions(
				mcp.Tool{
					Name: probeToolName,
					InputSchema: &jsonschema.Schema{
						Type: "object",
						Properties: map[string]*jsonschema.Schema{
							"owner": {Type: "string"},
							"repo":  {Type: "string"},
							"mode":  {Type: "string", Enum: []any{"valid"}},
						},
						Required: []string{"owner", "repo", "mode"},
					},
				},
				inventory.ToolsetMetadata{ID: "test"},
				func(context.Context, *mcp.CallToolRequest, input) (*mcp.CallToolResult, oauthProbeOutput, error) {
					handlerCalls++
					return nil, oauthProbeOutput{Status: "tool-ran"}, nil
				},
				inventory.TypedSchemaOptions{
					Preflight: func(ctx context.Context, _ *mcp.CallToolRequest) (context.Context, *mcp.CallToolResult, error) {
						preflightCalls++
						return ctx, nil, nil
					},
				},
				func(raw json.RawMessage) (json.RawMessage, error) {
					normalizerCalls++
					return raw, nil
				},
			)
			server := mcp.NewServer(&mcp.Implementation{Name: "oauth-http", Version: "test"}, nil)
			tool.RegisterFunc(server, nil, createOAuthToolMiddleware(fake, discardLogger()))
			server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method == inventory.MCPMethodToolsCall {
						receivingCalls++
					}
					return next(ctx, method, req)
				}
			})
			handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
				return server
			}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
			call := func(accept bool) *httptest.ResponseRecorder {
				t.Helper()
				params := map[string]any{
					"name": probeToolName, "arguments": json.RawMessage(tc.arguments),
					"_meta": mcp.Meta{
						mcp.MetaKeyProtocolVersion:    inventory.ProtocolVersionMultiRoundTrip,
						mcp.MetaKeyClientInfo:         &mcp.Implementation{Name: "test", Version: "test"},
						mcp.MetaKeyClientCapabilities: &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}}},
					},
				}
				if accept {
					params["inputResponses"] = mcp.InputResponseMap{
						oauthElicitIDPrefix + "flow-1": &mcp.ElicitResult{Action: "accept"},
					}
				}
				body, err := json.Marshal(params)
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(fmt.Sprintf(
					`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":%s}`, body)))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				req.Header.Set("Mcp-Protocol-Version", inventory.ProtocolVersionMultiRoundTrip)
				req.Header.Set("Mcp-Method", "tools/call")
				req.Header.Set("Mcp-Name", probeToolName)
				req.Header.Set("Mcp-Param-owner", tc.owner)
				req.Header.Set("Mcp-Param-repo", tc.repo)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				return rec
			}
			rec := call(false)
			if tc.mismatch {
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				assert.Contains(t, rec.Body.String(), "header mismatch")
				assert.Zero(t, receivingCalls)
				assert.Zero(t, fake.authCalls)
			} else {
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				assert.Contains(t, rec.Body.String(), `"resultType":"input_required"`)
				assert.NotContains(t, rec.Body.String(), "structuredContent")
				assert.Equal(t, 1, receivingCalls)
				assert.Equal(t, 1, fake.authCalls)
			}
			assert.Zero(t, handlerCalls)
			assert.Zero(t, preflightCalls)
			assert.Zero(t, normalizerCalls)
			if tc.mismatch {
				return
			}
			rec = call(true)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), `"resultType":"complete"`)
			assert.NotContains(t, rec.Body.String(), `"resultType":"input_required"`)
			assert.Equal(t, 1, fake.awaitCalls)
			assert.Equal(t, 1, preflightCalls)
			assert.Equal(t, 1, normalizerCalls)
			if tc.valid {
				assert.Equal(t, 1, handlerCalls)
				assert.Contains(t, rec.Body.String(), `"structuredContent":{"status":"tool-ran"}`)
			} else {
				assert.Zero(t, handlerCalls)
				assert.Contains(t, rec.Body.String(), `"isError":true`)
				assert.NotContains(t, rec.Body.String(), "structuredContent")
			}
		})
	}
}

func TestOAuthTypedRegistrationGuardsInvalidArguments(t *testing.T) {
	type input struct {
		Mode string `json:"mode"`
	}
	for _, raw := range []string{
		`{}`, `{"mode":"invalid"}`, `{"mode":42}`, `[]`, `null`, `"not an object"`,
	} {
		t.Run(raw, func(t *testing.T) {
			arguments := json.RawMessage(raw)
			fake := oauthPendingAuthenticator()
			handlerCalls, preflightCalls, normalizerCalls := 0, 0, 0
			tool := inventory.NewServerToolWithContextHandlerAndSchemaOptions(
				mcp.Tool{
					Name: probeToolName,
					InputSchema: &jsonschema.Schema{
						Type:       "object",
						Properties: map[string]*jsonschema.Schema{"mode": {Type: "string", Enum: []any{"valid"}}},
						Required:   []string{"mode"},
					},
				},
				inventory.ToolsetMetadata{ID: "test"},
				func(context.Context, *mcp.CallToolRequest, input) (*mcp.CallToolResult, oauthProbeOutput, error) {
					handlerCalls++
					return nil, oauthProbeOutput{Status: "unexpected"}, nil
				},
				inventory.TypedSchemaOptions{
					Preflight: func(ctx context.Context, _ *mcp.CallToolRequest) (context.Context, *mcp.CallToolResult, error) {
						preflightCalls++
						return ctx, nil, nil
					},
				},
				func(raw json.RawMessage) (json.RawMessage, error) {
					normalizerCalls++
					return raw, nil
				},
			)
			server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
			tool.RegisterFunc(server, nil, createOAuthToolMiddleware(fake, discardLogger()))
			session := connectOAuthRegisteredServer(t, server, "", &mcp.ClientOptions{
				Capabilities:   &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}}},
				MultiRoundTrip: &mcp.MultiRoundTripOptions{Disabled: true},
			})
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: probeToolName, Arguments: arguments})
			require.NoError(t, err)
			assert.True(t, result.NeedsInput(), "auth must win over invalid arguments")
			assert.Zero(t, handlerCalls)
			assert.Zero(t, preflightCalls)
			assert.Zero(t, normalizerCalls)
			result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: probeToolName, Arguments: arguments,
				InputResponses: mcp.InputResponseMap{oauthElicitIDPrefix + "flow-1": &mcp.ElicitResult{Action: "accept"}},
			})
			require.NoError(t, err)
			assert.True(t, result.IsError, "authorized retries must enforce original input schema")
			assert.False(t, result.NeedsInput())
			assert.Zero(t, handlerCalls)
			assert.Equal(t, 1, preflightCalls)
			assert.Equal(t, 1, normalizerCalls)
			assert.Equal(t, 1, fake.awaitCalls)
			list, err := session.ListTools(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, list.Tools, 1)
			encoded, err := json.Marshal(list.Tools[0].InputSchema)
			require.NoError(t, err)
			assert.Contains(t, string(encoded), `"required":["mode"]`)
			assert.Contains(t, string(encoded), `"enum":["valid"]`)
		})
	}
}
