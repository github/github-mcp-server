// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"net/url"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	internaljson "github.com/modelcontextprotocol/go-sdk/internal/json"
	"github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2"
	"github.com/modelcontextprotocol/go-sdk/internal/mcpgodebug"
	"github.com/modelcontextprotocol/go-sdk/internal/util"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/yosida95/uritemplate/v3"
)

// DefaultPageSize is the default for [ServerOptions.PageSize].
const DefaultPageSize = 1000

// A Server is an instance of an MCP server.
//
// Servers expose server-side MCP features, which can serve one or more MCP
// sessions by using [Server.Run].
type Server struct {
	// fixed at creation
	impl *Implementation
	opts ServerOptions

	mu                          sync.Mutex
	prompts                     *featureSet[*serverPrompt]
	tools                       *featureSet[*serverTool]
	resources                   *featureSet[*serverResource]
	resourceTemplates           *featureSet[*serverResourceTemplate]
	sessions                    []*ServerSession
	sendingMethodHandler_       MethodHandler
	receivingMethodHandler_     MethodHandler
	toolChangeSubscriptions     map[*ServerSession]jsonrpc.ID            // session -> requestID for "tools/changed"
	promptChangeSubscriptions   map[*ServerSession]jsonrpc.ID            // session -> requestID for "prompts/changed"
	resourceChangeSubscriptions map[*ServerSession]jsonrpc.ID            // session -> requestID for "resources/changed"
	resourceSubscriptions       map[string]map[*ServerSession]jsonrpc.ID // uri -> session -> requestID
	pendingNotifications        map[string]*time.Timer                   // notification name -> timer for pending notification send
	// receiveMethods is the merged map of methods this server may receive
	// from a client: it always contains the standard server methods (from
	// serverMethodInfos) plus any custom methods registered via
	// [AddReceivingCustomMethod].
	receiveMethods map[string]methodInfo
}

// ServerOptions is used to configure behavior of the server.
type ServerOptions struct {
	// Optional instructions for connected clients.
	Instructions string
	// Logger may be set to a non-nil value to enable logging of server activity.
	Logger *slog.Logger
	// If non-nil, called when "notifications/initialized" is received.
	InitializedHandler func(context.Context, *InitializedRequest)
	// PageSize is the maximum number of items to return in a single page for
	// list methods (e.g. ListTools).
	//
	// If zero, defaults to [DefaultPageSize].
	PageSize int
	// If non-nil, called when "notifications/roots/list_changed" is received.
	//
	// Deprecated: the roots feature is deprecated as of protocol version
	// 2026-07-28 (SEP-2577). It remains functional during the deprecation
	// window (at least twelve months). Migrate to passing paths via tool
	// parameters, resource URIs, or configuration. See
	// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
	RootsListChangedHandler func(context.Context, *RootsListChangedRequest)
	// If non-nil, called when "notifications/progress" is received.
	ProgressNotificationHandler func(context.Context, *ProgressNotificationServerRequest)
	// If non-nil, called when "completion/complete" is received.
	CompletionHandler func(context.Context, *CompleteRequest) (*CompleteResult, error)
	// If non-zero, defines an interval for regular "ping" requests.
	// If the peer fails to respond to pings originating from the keepalive check,
	// the session is automatically closed.
	KeepAlive time.Duration
	// KeepAliveFailureThreshold is the number of consecutive keepalive ping
	// failures tolerated before the session is closed. A value of 0 or 1
	// closes the session on the first failure (the default). Higher values
	// align with the spec's "multiple failed pings MAY trigger a connection
	// reset" guidance, letting a transient miss pass without tearing down an
	// otherwise live session. Has no effect unless KeepAlive is non-zero.
	KeepAliveFailureThreshold int
	// Function called when a client session subscribes to a resource.
	SubscribeHandler func(context.Context, *SubscribeRequest) error
	// Function called when a client session unsubscribes from a resource.
	UnsubscribeHandler func(context.Context, *UnsubscribeRequest) error

	// Capabilities optionally configures the server's default capabilities,
	// before any capabilities are inferred from other configuration or server
	// features.
	//
	// If Capabilities is nil, the default server capabilities are {"logging":{}},
	// for historical reasons. Setting Capabilities to a non-nil value overrides
	// this default. For example, setting Capabilities to `&ServerCapabilities{}`
	// disables the logging capability.
	//
	// # Interaction with capability inference
	//
	// "tools", "prompts", and "resources" capabilities are automatically added when
	// tools, prompts, or resources are added to the server (for example, via
	// [Server.AddPrompt]), with default value `{"listChanged":true}`. Similarly,
	// if the [ClientOptions.SubscribeHandler] or
	// [ClientOptions.CompletionHandler] are set, the inferred capabilities are
	// adjusted accordingly.
	//
	// Any non-nil field in Capabilities overrides the inferred value.
	// For example:
	//
	//  - To advertise the "tools" capability, even if no tools are added, set
	//    Capabilities.Tools to &ToolCapabilities{ListChanged:true}.
	//  - To disable tool list notifications, set Capabilities.Tools to
	//    &ToolCapabilities{}.
	//
	// Conversely, if Capabilities does not set a field (for example, if the
	// Prompts field is nil), the inferred capability will be used.
	Capabilities *ServerCapabilities

	// If true, advertises the prompts capability during initialization,
	// even if no prompts have been registered.
	//
	// Deprecated: Use Capabilities instead.
	HasPrompts bool
	// If true, advertises the resources capability during initialization,
	// even if no resources have been registered.
	//
	// Deprecated: Use Capabilities instead.
	HasResources bool
	// If true, advertises the tools capability during initialization,
	// even if no tools have been registered.
	//
	// Deprecated: Use Capabilities instead.
	HasTools bool
	// SchemaCache, if non-nil, caches JSON schemas to avoid repeated
	// reflection. This is useful for stateless server deployments where
	// a new [Server] is created for each request. See [SchemaCache] for
	// trade-offs and usage guidance.
	SchemaCache *SchemaCache

	// GetSessionID provides the next session ID to use for an incoming request.
	// If nil, a default randomly generated ID will be used.
	//
	// Session IDs should be globally unique across the scope of the server,
	// which may span multiple processes in the case of distributed servers.
	//
	// As a special case, if GetSessionID returns the empty string, the
	// Mcp-Session-Id header will not be set.
	//
	// GetSessionID is not consulted when [StreamableHTTPOptions.Stateless] is
	// true, since stateless servers do not maintain sessions.
	GetSessionID func() string
}

// NewServer creates a new MCP server. The resulting server has no features:
// add features using the various Server.AddXXX methods, and the [AddTool] function.
//
// The server can be connected to one or more MCP clients using [Server.Run].
//
// The first argument must not be nil.
//
// If non-nil, the provided options are used to configure the server.
func NewServer(impl *Implementation, options *ServerOptions) *Server {
	if impl == nil {
		panic("nil Implementation")
	}
	var opts ServerOptions
	if options != nil {
		opts = *options
	}
	options = nil // prevent reuse
	if opts.PageSize < 0 {
		panic(fmt.Errorf("invalid page size %d", opts.PageSize))
	}
	if opts.PageSize == 0 {
		opts.PageSize = DefaultPageSize
	}
	if opts.SubscribeHandler != nil && opts.UnsubscribeHandler == nil {
		panic("SubscribeHandler requires UnsubscribeHandler")
	}
	if opts.UnsubscribeHandler != nil && opts.SubscribeHandler == nil {
		panic("UnsubscribeHandler requires SubscribeHandler")
	}

	if opts.GetSessionID == nil {
		opts.GetSessionID = rand.Text
	}

	if opts.Logger == nil { // ensure we have a logger
		opts.Logger = ensureLogger(nil)
	}

	receiveMethods := make(map[string]methodInfo, len(serverMethodInfos))
	maps.Copy(receiveMethods, serverMethodInfos)

	s := &Server{
		impl:                        impl,
		opts:                        opts,
		prompts:                     newFeatureSet(func(p *serverPrompt) string { return p.prompt.Name }),
		tools:                       newFeatureSet(func(t *serverTool) string { return t.tool.Name }),
		resources:                   newFeatureSet(func(r *serverResource) string { return r.resource.URI }),
		resourceTemplates:           newFeatureSet(func(t *serverResourceTemplate) string { return t.resourceTemplate.URITemplate }),
		sendingMethodHandler_:       defaultSendingMethodHandler,
		receivingMethodHandler_:     defaultReceivingMethodHandler[*ServerSession],
		toolChangeSubscriptions:     make(map[*ServerSession]jsonrpc.ID),
		promptChangeSubscriptions:   make(map[*ServerSession]jsonrpc.ID),
		resourceChangeSubscriptions: make(map[*ServerSession]jsonrpc.ID),
		resourceSubscriptions:       make(map[string]map[*ServerSession]jsonrpc.ID),
		pendingNotifications:        make(map[string]*time.Timer),
		receiveMethods:              receiveMethods,
	}
	s.AddReceivingMiddleware(serverMultiRoundTripMiddleware())
	return s
}

// AddPrompt adds a [Prompt] to the server, or replaces one with the same name.
func (s *Server) AddPrompt(p *Prompt, h PromptHandler) {
	// Assume there was a change, since add replaces existing items.
	// (It's possible an item was replaced with an identical one, but not worth checking.)
	s.changeAndNotify(
		notificationPromptListChanged,
		func() bool { s.prompts.add(&serverPrompt{p, h}); return true })
}

// RemovePrompts removes the prompts with the given names.
// It is not an error to remove a nonexistent prompt.
func (s *Server) RemovePrompts(names ...string) {
	s.changeAndNotify(notificationPromptListChanged, func() bool { return s.prompts.remove(names...) })
}

// AddTool adds a [Tool] to the server, or replaces one with the same name.
// The Tool argument must not be modified after this call.
//
// The tool's input schema must be non-nil and have the type "object". For a tool
// that takes no input, or one where any input is valid, set [Tool.InputSchema] to
// `{"type": "object"}`, using your preferred library or `json.RawMessage`.
//
// If present, [Tool.OutputSchema] may be any valid JSON Schema (object, array,
// primitive, or composition).
//
// When the handler is invoked as part of a CallTool request, req.Params.Arguments
// will be a json.RawMessage.
//
// Unmarshaling the arguments and validating them against the input schema are the
// caller's responsibility.
//
// Validating the result against the output schema, if any, is the caller's responsibility.
//
// Setting the result's Content, StructuredContent and IsError fields are the caller's
// responsibility.
//
// Most users should use the top-level function [AddTool], which handles all these
// responsibilities.
func (s *Server) AddTool(t *Tool, h ToolHandler) {
	if err := validateToolName(t.Name); err != nil {
		s.opts.Logger.Error(fmt.Sprintf("AddTool: invalid tool name %q: %v", t.Name, err))
	}
	if t.InputSchema == nil {
		// This prevents the tool author from forgetting to write a schema where
		// one should be provided. If we papered over this by supplying the empty
		// schema, then every input would be validated and the problem wouldn't be
		// discovered until runtime, when the LLM sent bad data.
		panic(fmt.Errorf("AddTool %q: missing input schema", t.Name))
	}
	if s, ok := t.InputSchema.(*jsonschema.Schema); ok {
		if s == nil {
			panic(fmt.Errorf("AddTool %q: input schema is nil", t.Name))
		}
		if s.Type != "object" {
			panic(fmt.Errorf(`AddTool %q: input schema must have type "object"`, t.Name))
		}
	} else {
		var m map[string]any
		if err := remarshal(t.InputSchema, &m); err != nil {
			panic(fmt.Errorf("AddTool %q: can't marshal input schema to a JSON object: %v", t.Name, err))
		}
		if typ := m["type"]; typ != "object" {
			panic(fmt.Errorf(`AddTool %q: input schema must have type "object" (got %v)`, t.Name, typ))
		}
	}
	if t.OutputSchema != nil {
		if s, ok := t.OutputSchema.(*jsonschema.Schema); ok {
			if s == nil {
				panic(fmt.Errorf("AddTool %q: output schema is nil", t.Name))
			}
		} else {
			var m any
			if err := remarshal(t.OutputSchema, &m); err != nil {
				panic(fmt.Errorf("AddTool %q: can't marshal output schema to JSON: %v", t.Name, err))
			}
		}
	}
	if err := validateParamHeaderAnnotations(t); err != nil {
		panic(fmt.Errorf("AddTool %q: invalid parameter header annotations: %v", t.Name, err))
	}
	st := &serverTool{tool: t, handler: h}
	// Assume there was a change, since add replaces existing tools.
	// (It's possible a tool was replaced with an identical one, but not worth checking.)
	// TODO: Batch these changes by size and time? The typescript SDK doesn't.
	// TODO: Surface notify error here? best not, in case we need to batch.
	s.changeAndNotify(notificationToolListChanged, func() bool { s.tools.add(st); return true })
}

func toolForErr[In, Out any](t *Tool, h ToolHandlerFor[In, Out], cache *SchemaCache) (*Tool, ToolHandler, error) {
	tt := *t

	// Special handling for an "any" input: treat as an empty object.
	if reflect.TypeFor[In]() == reflect.TypeFor[any]() && t.InputSchema == nil {
		tt.InputSchema = &jsonschema.Schema{Type: "object"}
	}

	var inputResolved *jsonschema.Resolved
	if _, err := setSchema[In](&tt.InputSchema, &inputResolved, cache); err != nil {
		return nil, nil, fmt.Errorf("input schema: %w", err)
	}

	// Handling for zero values:
	//
	// If Out is a pointer type and we've derived the output schema from its
	// element type, use the zero value of its element type in place of a typed
	// nil.
	var (
		elemZero       any // only non-nil if Out is a pointer type
		outputResolved *jsonschema.Resolved
	)
	if t.OutputSchema != nil || reflect.TypeFor[Out]() != reflect.TypeFor[any]() {
		var err error
		elemZero, err = setSchema[Out](&tt.OutputSchema, &outputResolved, cache)
		if err != nil {
			return nil, nil, fmt.Errorf("output schema: %v", err)
		}
	}

	th := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		var input json.RawMessage
		if req.Params.Arguments != nil {
			input = req.Params.Arguments
		}
		// Validate input and apply defaults.
		var err error
		input, err = applySchema(input, inputResolved, false)
		if err != nil {
			var errRes CallToolResult
			errRes.SetError(fmt.Errorf("validating \"arguments\": %v", err))
			return &errRes, nil
		}

		// Unmarshal and validate args.
		var in In
		if input != nil {
			if err := internaljson.Unmarshal(input, &in); err != nil {
				var errRes CallToolResult
				errRes.SetError(err)
				return &errRes, nil
			}
		}

		// Call typed handler.
		res, out, err := h(ctx, req, in)
		// Handle server errors appropriately:
		// - If the handler returns a structured error (like jsonrpc.Error), return it directly
		// - If the handler returns a regular error, wrap it in a CallToolResult with IsError=true
		// - This allows tools to distinguish between protocol errors and tool execution errors
		if err != nil {
			// Check if this is already a structured JSON-RPC error
			if wireErr, ok := err.(*jsonrpc.Error); ok {
				return nil, wireErr
			}
			// For regular errors, embed them in the tool result as per MCP spec
			var errRes CallToolResult
			errRes.SetError(err)
			return &errRes, nil
		}

		if res == nil {
			res = &CallToolResult{}
		}

		// Marshal the output and put the RawMessage in the StructuredContent field.
		// Skip when the handler returned input requests (multi round-trip): content and
		// inputRequests are mutually exclusive on the wire.
		var outval any = out
		if res.InputRequests != nil {
			outval = nil
		} else if elemZero != nil {
			// Avoid typed nil, which will serialize as JSON null.
			// Instead, use the zero value of the unpointered type.
			var z Out
			if any(out) == any(z) { // zero is only non-nil if Out is a pointer type
				outval = elemZero
			}
		}
		if outval != nil {
			outbytes, err := json.Marshal(outval)
			if err != nil {
				return nil, fmt.Errorf("marshaling output: %w", err)
			}
			outJSON := json.RawMessage(outbytes)
			// Validate the output JSON, and apply defaults.
			//
			// We validate against the JSON, rather than the output value, as
			// some types may have custom JSON marshalling (issue #447).
			outJSON, err = applySchema(outJSON, outputResolved, true)
			if err != nil {
				return nil, fmt.Errorf("validating tool output: %w", err)
			}
			res.StructuredContent = outJSON // avoid a second marshal over the wire

			// If the Content field isn't being used, return the serialized JSON in a
			// TextContent block, as the spec suggests:
			// https://modelcontextprotocol.io/specification/2025-06-18/server/tools#structured-content.
			//
			// Ensure a serialized-JSON TextContent fallback is present in case of servers using array
			// or primitive structuredContent, so that pre-SEP-2106 clients can recover the structured
			// payload from unstructured content.
			if res.Content == nil {
				res.Content = []Content{&TextContent{
					Text: string(outJSON),
				}}
			} else if !isObjectJSON(outJSON) {
				res.Content = append(res.Content, &TextContent{
					Text: string(outJSON),
				})
			}
		}
		return res, nil
	} // end of handler

	return &tt, th, nil
}

// setSchema sets the schema and resolved schema corresponding to the type T.
//
// If sfield is nil, the schema is derived from T.
//
// Pointers are treated equivalently to non-pointers when deriving the schema.
// If an indirection occurred to derive the schema, a non-nil zero value is
// returned to be used in place of the typed nil zero value.
//
// Note that if sfield already holds a schema, zero will be nil even if T is a
// pointer: if the user provided the schema, they may have intentionally
// derived it from the pointer type, and handling of zero values is up to them.
//
// If cache is non-nil, schemas are cached to avoid repeated reflection.
//
// TODO(rfindley): we really shouldn't ever return 'null' results. Maybe we
// should have a jsonschema.Zero(schema) helper?
func setSchema[T any](sfield *any, rfield **jsonschema.Resolved, cache *SchemaCache) (zero any, err error) {
	rt := reflect.TypeFor[T]()
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
		zero = reflect.Zero(rt).Interface()
	}

	var internalSchema *jsonschema.Schema

	if *sfield == nil {
		// No schema provided: check cache, or generate via reflection.
		if cache != nil {
			if schema, resolved, ok := cache.getByType(rt); ok {
				*sfield = schema
				*rfield = resolved
				return zero, nil
			}
		}

		internalSchema, err = jsonschema.ForType(rt, &jsonschema.ForOptions{})
		if err != nil {
			return zero, err
		}
		if internalSchema == nil {
			return zero, fmt.Errorf("schema is nil for type %v", rt)
		}
		*sfield = internalSchema

		resolved, err := internalSchema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
		if err != nil {
			return zero, err
		}
		*rfield = resolved
		if cache != nil {
			cache.setByType(rt, internalSchema, resolved)
		}
		return zero, nil
	}

	// Schema was provided: check cache by pointer, or resolve it.
	if providedSchema, ok := (*sfield).(*jsonschema.Schema); ok {
		if cache != nil {
			if resolved, ok := cache.getBySchema(providedSchema); ok {
				*rfield = resolved
				return zero, nil
			}
		}
		internalSchema = providedSchema
	} else {
		// Schema provided as different type (e.g., map): remarshal to *Schema.
		if err := remarshal(*sfield, &internalSchema); err != nil {
			return zero, err
		}
	}

	if internalSchema == nil {
		return zero, fmt.Errorf("schema is nil for type %v", rt)
	}

	resolved, err := internalSchema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return zero, err
	}
	*rfield = resolved

	if cache != nil {
		if providedSchema, ok := (*sfield).(*jsonschema.Schema); ok {
			cache.setBySchema(providedSchema, resolved)
		}
	}

	return zero, nil
}

// AddTool adds a tool and typed tool handler to the server.
//
// If the tool's input schema is nil, it is set to the schema inferred from the
// In type parameter. Types are inferred from Go types, and property
// descriptions are read from the 'jsonschema' struct tag. Internally, the SDK
// uses the github.com/google/jsonschema-go package for inference and
// validation. The In type argument must be a map or a struct, so that its
// inferred JSON Schema has type "object", as required by the spec. As a
// special case, if the In type is 'any', the tool's input schema is set to an
// empty object schema value.
//
// If the tool's output schema is nil, and the Out type is not 'any', the
// output schema is set to the schema inferred from the Out type argument.
// Per SEP-2106, the Out type may be any Go type whose inferred schema is a
// valid JSON Schema (struct, map, slice, primitive, etc.). If the Out type is
// 'any', the output schema is omitted.
//
// Unlike [Server.AddTool], AddTool does a lot automatically, and forces
// tools to conform to the MCP spec. See [ToolHandlerFor] for a detailed
// description of this automatic behavior.
func AddTool[In, Out any](s *Server, t *Tool, h ToolHandlerFor[In, Out]) {
	tt, hh, err := toolForErr(t, h, s.opts.SchemaCache)
	if err != nil {
		panic(fmt.Sprintf("AddTool: tool %q: %v", t.Name, err))
	}
	s.AddTool(tt, hh)
}

// RemoveTools removes the tools with the given names.
// It is not an error to remove a nonexistent tool.
func (s *Server) RemoveTools(names ...string) {
	s.changeAndNotify(notificationToolListChanged, func() bool { return s.tools.remove(names...) })
}

// AddResource adds a [Resource] to the server, or replaces one with the same URI.
// AddResource panics if the resource URI is invalid or not absolute (has an empty scheme).
func (s *Server) AddResource(r *Resource, h ResourceHandler) {
	s.changeAndNotify(notificationResourceListChanged,
		func() bool {
			if _, err := url.Parse(r.URI); err != nil {
				panic(err) // url.Parse includes the URI in the error
			}
			s.resources.add(&serverResource{r, h})
			return true
		})
}

// RemoveResources removes the resources with the given URIs.
// It is not an error to remove a nonexistent resource.
func (s *Server) RemoveResources(uris ...string) {
	s.changeAndNotify(notificationResourceListChanged, func() bool { return s.resources.remove(uris...) })
}

// AddResourceTemplate adds a [ResourceTemplate] to the server, or replaces one with the same URI.
// AddResourceTemplate panics if a URI template is invalid or not absolute (has an empty scheme).
func (s *Server) AddResourceTemplate(t *ResourceTemplate, h ResourceHandler) {
	s.changeAndNotify(notificationResourceListChanged,
		func() bool {
			// Validate the URI template syntax
			_, err := uritemplate.New(t.URITemplate)
			if err != nil {
				panic(fmt.Errorf("URI template %q is invalid: %w", t.URITemplate, err))
			}
			s.resourceTemplates.add(&serverResourceTemplate{t, h})
			return true
		})
}

// RemoveResourceTemplates removes the resource templates with the given URI templates.
// It is not an error to remove a nonexistent resource.
func (s *Server) RemoveResourceTemplates(uriTemplates ...string) {
	s.changeAndNotify(notificationResourceListChanged, func() bool { return s.resourceTemplates.remove(uriTemplates...) })
}

func (s *Server) capabilities() *ServerCapabilities {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Start with user-provided capabilities as defaults, or use SDK defaults.
	var caps *ServerCapabilities
	if s.opts.Capabilities != nil {
		// Deep copy the user-provided capabilities to avoid mutation.
		caps = s.opts.Capabilities.clone()
	} else {
		// SDK defaults: only logging capability.
		caps = &ServerCapabilities{
			Logging: &LoggingCapabilities{},
		}
	}

	// Augment with tools capability if tools exist or legacy HasTools is set.
	if s.opts.HasTools || s.tools.len() > 0 {
		if caps.Tools == nil {
			caps.Tools = &ToolCapabilities{ListChanged: true}
		}
	}

	// Augment with prompts capability if prompts exist or legacy HasPrompts is set.
	if s.opts.HasPrompts || s.prompts.len() > 0 {
		if caps.Prompts == nil {
			caps.Prompts = &PromptCapabilities{ListChanged: true}
		}
	}

	// Augment with resources capability if resources/templates exist or legacy HasResources is set.
	if s.opts.HasResources || s.resources.len() > 0 || s.resourceTemplates.len() > 0 {
		if caps.Resources == nil {
			caps.Resources = &ResourceCapabilities{ListChanged: true}
		}
		if s.opts.SubscribeHandler != nil {
			caps.Resources.Subscribe = true
		}
	}

	// Augment with completions capability if handler is set.
	if s.opts.CompletionHandler != nil {
		if caps.Completions == nil {
			caps.Completions = &CompletionCapabilities{}
		}
	}

	return caps
}

// disablecompleteparamsvalidation is a compatibility parameter that restores
// the previous behavior of [Server.complete], where required fields on
// [CompleteParams] were not validated before dispatching to the completion
// handler. See the documentation for the mcpgodebug package for instructions
// how to enable it.
// The option will be removed in a future version of the SDK.
var disablecompleteparamsvalidation = mcpgodebug.Value("disablecompleteparamsvalidation")

func (s *Server) complete(ctx context.Context, req *CompleteRequest) (*CompleteResult, error) {
	if disablecompleteparamsvalidation != "1" {
		if req.Params.Ref == nil {
			return nil, fmt.Errorf("%w: missing required 'ref' field", jsonrpc2.ErrInvalidParams)
		}
		if req.Params.Argument.Name == "" {
			return nil, fmt.Errorf("%w: missing required 'argument.name' field", jsonrpc2.ErrInvalidParams)
		}
	}
	if s.opts.CompletionHandler == nil {
		return nil, jsonrpc2.ErrMethodNotFound
	}
	return s.opts.CompletionHandler(ctx, req)
}

// Map from notification name to a function creating its corresponding Params.
// We need to create a fresh one each time to add the jsonrpc ID. See
// [injectMetaSubscriptionID].
var changeNotificationParams = map[string]func() Params{
	notificationToolListChanged:     func() Params { return &ToolListChangedParams{} },
	notificationPromptListChanged:   func() Params { return &PromptListChangedParams{} },
	notificationResourceListChanged: func() Params { return &ResourceListChangedParams{} },
}

// How long to wait before sending a change notification.
const notificationDelay = 10 * time.Millisecond

// changeAndNotify is called when a feature is added or removed.
// It calls change, which should do the work and report whether a change actually occurred.
// If there was a change, it sets a timer to send a notification.
// This debounces change notifications: a single notification is sent after
// multiple changes occur in close proximity.
func (s *Server) changeAndNotify(notification string, change func() bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if change() && s.shouldSendListChangedNotification(notification) {
		if len(s.sessions) == 0 {
			if t := s.pendingNotifications[notification]; t != nil {
				t.Stop()
				s.pendingNotifications[notification] = nil
			}
			return
		}

		// Reset the outstanding delayed call, if any.
		if t := s.pendingNotifications[notification]; t == nil {
			s.pendingNotifications[notification] = time.AfterFunc(notificationDelay, func() { s.notifySessions(notification) })
		} else {
			t.Reset(notificationDelay)
		}
	}
}

// notifySessions sends the notification n to all existing sessions.
// It is called asynchronously by changeAndNotify.
//
// Legacy (pre-SEP-2575) sessions receive the notification on the shared
// session channel. Sessions speaking the new protocol receive it only if they
// have an active subscriptions/listen stream that opted in to this
// notification type.
func (s *Server) notifySessions(n string) {
	s.mu.Lock()
	s.pendingNotifications[n] = nil
	// Legacy (pre-SEP-2575) sessions receive list-changed notifications on the
	// shared session channel without opt-in; collect them while we hold the lock.
	var legacySessions []*ServerSession
	for _, sess := range s.sessions {
		if sess.InitializeParams().isNil() || sess.InitializeParams().ProtocolVersion < protocolVersion20260728 {
			legacySessions = append(legacySessions, sess)
		}
	}
	var subscribers map[*ServerSession]jsonrpc.ID
	switch n {
	case notificationToolListChanged:
		subscribers = maps.Clone(s.toolChangeSubscriptions)
	case notificationPromptListChanged:
		subscribers = maps.Clone(s.promptChangeSubscriptions)
	case notificationResourceListChanged:
		subscribers = maps.Clone(s.resourceChangeSubscriptions)
	}
	s.mu.Unlock() // Don't hold the lock during notification: it causes deadlock.

	// Notify legacy sessions on the shared session channel regardless of
	// their subscription status.
	notifySessions(legacySessions, n, changeNotificationParams[n](), s.opts.Logger)

	// Notify modern sessions only if they have an active subscription for
	// this notification type.
	s.notifySubscribedSessions(subscribers, n, changeNotificationParams[n])
}

// notifySubscribedSessions delivers a list-changed or resource-updated
// notification to each session in subscribers, stamping the session's
// listen-request ID into the params' _meta so the receiving client can demultiplex
// notifications belonging to different concurrent listens.
func (s *Server) notifySubscribedSessions(subscribers map[*ServerSession]jsonrpc.ID, method string, makeParams func() Params) {
	if len(subscribers) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for sess, reqID := range subscribers {
		params := makeParams()
		injectMetaSubscriptionID(params, reqID)
		req := newRequest(sess, params)
		if err := handleNotify(ctx, method, req); err != nil {
			s.opts.Logger.Warn(fmt.Sprintf("calling %s: %v", method, err))
		}
	}
}

// injectMetaSubscriptionID stamps the listen request's JSON-RPC ID into the
// params' _meta under [MetaKeySubscriptionID], so the receiving client can
// correlate the notification with the [subscriptions/listen] stream it
// originated.
//
// [subscriptions/listen]: https://modelcontextprotocol.io/seps/2575-stateless-mcp#multiple-concurrent-subscriptions
func injectMetaSubscriptionID(params Params, reqID jsonrpc.ID) {
	m := params.GetMeta()
	if m == nil {
		m = map[string]any{}
	}
	m[MetaKeySubscriptionID] = reqID.Raw()
	params.SetMeta(m)
}

// shouldSendListChangedNotification checks if the server's capabilities allow
// sending the given list-changed notification.
func (s *Server) shouldSendListChangedNotification(notification string) bool {
	// Get effective capabilities (considering user-provided defaults).
	caps := s.opts.Capabilities

	switch notification {
	case notificationToolListChanged:
		// If user didn't specify capabilities, default behavior sends notifications.
		if caps == nil || caps.Tools == nil {
			return true
		}
		return caps.Tools.ListChanged
	case notificationPromptListChanged:
		if caps == nil || caps.Prompts == nil {
			return true
		}
		return caps.Prompts.ListChanged
	case notificationResourceListChanged:
		if caps == nil || caps.Resources == nil {
			return true
		}
		return caps.Resources.ListChanged
	default:
		// Unknown notification, allow by default.
		return true
	}
}

// Sessions returns an iterator that yields the current set of server sessions.
//
// There is no guarantee that the iterator observes sessions that are added or
// removed during iteration.
func (s *Server) Sessions() iter.Seq[*ServerSession] {
	s.mu.Lock()
	clients := slices.Clone(s.sessions)
	s.mu.Unlock()
	return slices.Values(clients)
}

func (s *Server) listPrompts(_ context.Context, req *ListPromptsRequest) (*ListPromptsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Params == nil {
		req.Params = &ListPromptsParams{}
	}
	res, err := paginateList(s.prompts, s.opts.PageSize, req.Params, &ListPromptsResult{}, func(res *ListPromptsResult, prompts []*serverPrompt) {
		res.Prompts = []*Prompt{} // avoid JSON null
		for _, p := range prompts {
			res.Prompts = append(res.Prompts, p.prompt)
		}
	})
	if err != nil {
		return nil, err
	}
	res.setDefaultCacheableValues()
	return res, nil
}

func (s *Server) getPrompt(ctx context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
	s.mu.Lock()
	prompt, ok := s.prompts.get(req.Params.Name)
	s.mu.Unlock()
	if !ok {
		// Return a proper JSON-RPC error with the correct error code
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: fmt.Sprintf("unknown prompt %q", req.Params.Name),
		}
	}
	res, err := prompt.handler(ctx, req)
	if err == nil && res != nil {
		if err := handleMultiRoundTripResult(req.Session, s.opts.Logger, res); err != nil {
			return nil, err
		}
	}
	return res, err
}

// discover is the server-side handler for the SEP-2575 "server/discover" RPC.
//
// It returns the protocol versions supported by the underlying transport,
// the server's capabilities, the server's identity, and the server's
// instructions, allowing clients to negotiate without performing the legacy
// initialize handshake.
func (s *Server) discover(_ context.Context, req *ServerRequest[*DiscoverParams]) (*DiscoverResult, error) {
	req.Session.mu.Lock()
	versions := req.Session.supportedVersions
	req.Session.mu.Unlock()
	if versions == nil {
		versions = slices.Clone(supportedProtocolVersions)
	}
	// Read the request-scoped identity/capabilities before acquiring the
	// session lock: these accessors may fall back to Session.InitializeParams
	// (which also locks Session.mu), so calling them from inside updateState
	// would self-deadlock.
	init := &InitializeParams{
		ProtocolVersion: req.ProtocolVersion(),
		Capabilities:    req.ClientCapabilities(),
		ClientInfo:      req.ClientInfo(),
	}
	req.Session.updateState(func(state *ServerSessionState) {
		state.InitializeParams = init
	})
	res := &DiscoverResult{
		SupportedVersions: versions,
		Capabilities:      s.capabilities(),
		Instructions:      s.opts.Instructions,
	}
	res.setDefaultCacheableValues()
	return res, nil
}

// filterSupportedVersions returns the subset of [supportedProtocolVersions]
// that the Transport can serve. If t does not implement [ProtocolVersionSupporter], every
// SDK-supported version is included.
func filterSupportedVersions(t Transport) []string {
	pvs, ok := t.(ProtocolVersionSupporter)
	if !ok {
		return slices.Clone(supportedProtocolVersions)
	}
	out := make([]string, 0, len(supportedProtocolVersions))
	for _, v := range supportedProtocolVersions {
		if pvs.SupportsProtocolVersion(v) {
			out = append(out, v)
		}
	}
	return out
}

func (s *Server) listTools(_ context.Context, req *ListToolsRequest) (*ListToolsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Params == nil {
		req.Params = &ListToolsParams{}
	}
	res, err := paginateList(s.tools, s.opts.PageSize, req.Params, &ListToolsResult{}, func(res *ListToolsResult, tools []*serverTool) {
		res.Tools = []*Tool{} // avoid JSON null
		for _, t := range tools {
			res.Tools = append(res.Tools, t.tool)
		}
	})
	if err != nil {
		return nil, err
	}
	res.setDefaultCacheableValues()
	return res, nil
}

// getServerTool looks up a server tool by name.
func (s *Server) getServerTool(name string) (*serverTool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tools.get(name)
}

func (s *Server) callTool(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
	st, ok := s.getServerTool(req.Params.Name)
	if !ok {
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: fmt.Sprintf("unknown tool %q", req.Params.Name),
		}
	}
	res, err := st.handler(ctx, req)
	if err == nil && res != nil {
		if err := handleMultiRoundTripResult(req.Session, s.opts.Logger, res); err != nil {
			return nil, err
		}
		if res.Content == nil && res.resultType != resultTypeInputRequired {
			res2 := *res
			res2.Content = []Content{} // avoid "null"
			res = &res2
		}
	}
	return res, err
}

func (s *Server) listResources(_ context.Context, req *ListResourcesRequest) (*ListResourcesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Params == nil {
		req.Params = &ListResourcesParams{}
	}
	res, err := paginateList(s.resources, s.opts.PageSize, req.Params, &ListResourcesResult{}, func(res *ListResourcesResult, resources []*serverResource) {
		res.Resources = []*Resource{} // avoid JSON null
		for _, r := range resources {
			res.Resources = append(res.Resources, r.resource)
		}
	})
	if err != nil {
		return nil, err
	}
	res.setDefaultCacheableValues()
	return res, nil
}

func (s *Server) listResourceTemplates(_ context.Context, req *ListResourceTemplatesRequest) (*ListResourceTemplatesResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Params == nil {
		req.Params = &ListResourceTemplatesParams{}
	}
	res, err := paginateList(s.resourceTemplates, s.opts.PageSize, req.Params, &ListResourceTemplatesResult{},
		func(res *ListResourceTemplatesResult, rts []*serverResourceTemplate) {
			res.ResourceTemplates = []*ResourceTemplate{} // avoid JSON null
			for _, rt := range rts {
				res.ResourceTemplates = append(res.ResourceTemplates, rt.resourceTemplate)
			}
		})
	if err != nil {
		return nil, err
	}
	res.setDefaultCacheableValues()
	return res, nil
}

func (s *Server) readResource(ctx context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
	uri := req.Params.URI
	// Look up the resource URI in the lists of resources and resource templates.
	// This is a security check as well as an information lookup.
	handler, mimeType, ok := s.lookupResourceHandler(uri)
	if !ok {
		// Don't expose the server configuration to the client.
		// Treat an unregistered resource the same as a registered one that couldn't be found.
		return nil, ResourceNotFoundError(uri)
	}
	res, err := handler(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := handleMultiRoundTripResult(req.Session, s.opts.Logger, res); err != nil {
		return nil, err
	}
	res.setDefaultCacheableValues()
	if res.resultType == resultTypeInputRequired {
		return res, nil
	}
	if res == nil || res.Contents == nil {
		return nil, fmt.Errorf("reading resource %s: read handler returned nil information", uri)
	}
	// As a convenience, populate some fields.
	for _, c := range res.Contents {
		if c.URI == "" {
			c.URI = uri
		}
		if c.MIMEType == "" {
			c.MIMEType = mimeType
		}
	}
	return res, nil
}

// lookupResourceHandler returns the resource handler and MIME type for the resource or
// resource template matching uri. If none, the last return value is false.
func (s *Server) lookupResourceHandler(uri string) (ResourceHandler, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Try resources first.
	if r, ok := s.resources.get(uri); ok {
		return r.handler, r.resource.MIMEType, true
	}
	// Look for matching template.
	for rt := range s.resourceTemplates.all() {
		if rt.Matches(uri) {
			return rt.handler, rt.resourceTemplate.MIMEType, true
		}
	}
	return nil, "", false
}

// fileResourceHandler returns a ReadResourceHandler that reads paths using dir as
// a base directory.
// It honors client roots and protects against path traversal attacks.
//
// The dir argument should be a filesystem path. It need not be absolute, but
// that is recommended to avoid a dependency on the current working directory (the
// check against client roots is done with an absolute path). If dir is not absolute
// and the current working directory is unavailable, fileResourceHandler panics.
//
// Lexical path traversal attacks, where the path has ".." elements that escape dir,
// are always caught. The SDK also protects against symlink-based attacks,
// where symlinks under dir lead out of the tree.
func fileResourceHandler(dir string) ResourceHandler {
	// Convert dir to an absolute path.
	dirFilepath, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	return func(ctx context.Context, req *ReadResourceRequest) (_ *ReadResourceResult, err error) {
		defer util.Wrapf(&err, "reading resource %s", req.Params.URI)

		// TODO(#25): use a memoizing API here.
		rootRes, err := req.Session.ListRoots(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("listing roots: %w", err)
		}
		roots, err := fileRoots(rootRes.Roots)
		if err != nil {
			return nil, err
		}
		data, err := readFileResource(req.Params.URI, dirFilepath, roots)
		if err != nil {
			return nil, err
		}
		// TODO(jba): figure out mime type. Omit for now: Server.readResource will fill it in.
		return &ReadResourceResult{Contents: []*ResourceContents{
			{URI: req.Params.URI, Blob: data},
		}}, nil
	}
}

// ResourceUpdated sends a notification to all clients that have subscribed to the
// resource specified in params. This method is the primary way for a
// server author to signal that a resource has changed.
func (s *Server) ResourceUpdated(ctx context.Context, params *ResourceUpdatedNotificationParams) error {
	s.mu.Lock()
	subscribedSessions := s.resourceSubscriptions[params.URI]
	// Only add legacy sessions for the notification, new ones use the new notification mechanism.
	var legacySessions []*ServerSession
	newSessions := make(map[*ServerSession]jsonrpc.ID)
	for sess, reqID := range subscribedSessions {
		if sess.InitializeParams().isNil() || sess.InitializeParams().ProtocolVersion < protocolVersion20260728 {
			legacySessions = append(legacySessions, sess)
		} else {
			newSessions[sess] = reqID
		}
	}
	s.mu.Unlock()

	notifySessions(legacySessions, notificationResourceUpdated, params, s.opts.Logger)

	// Notify modern sessions, injecting the per-session subscription ID into the notification's metadata.
	s.notifySubscribedSessions(newSessions, notificationResourceUpdated, func() Params {
		p := *params
		return &p
	})
	s.opts.Logger.Info("resource updated notification sent",
		"uri", params.URI,
		"subscriber_count", len(legacySessions)+len(newSessions))
	return nil
}

func (s *Server) subscribe(ctx context.Context, req *SubscribeRequest) (*emptyResult, error) {
	requestID, ok := ctx.Value(idContextKey{}).(jsonrpc.ID)
	if !ok || !requestID.IsValid() {
		return nil, fmt.Errorf("%w: subscribe requires a request ID", jsonrpc2.ErrInvalidRequest)
	}
	if s.opts.SubscribeHandler == nil {
		return nil, fmt.Errorf("%w: server does not support resource subscriptions", jsonrpc2.ErrMethodNotFound)
	}
	if err := s.opts.SubscribeHandler(ctx, req); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resourceSubscriptions[req.Params.URI] == nil {
		s.resourceSubscriptions[req.Params.URI] = make(map[*ServerSession]jsonrpc.ID)
	}
	s.resourceSubscriptions[req.Params.URI][req.Session] = requestID
	s.opts.Logger.Info("resource subscribed", "uri", req.Params.URI, "session_id", req.Session.ID(), "request_id", requestID)

	return &emptyResult{}, nil
}

func (s *Server) unsubscribe(ctx context.Context, req *UnsubscribeRequest) (*emptyResult, error) {
	if s.opts.UnsubscribeHandler == nil {
		return nil, jsonrpc2.ErrMethodNotFound
	}

	if err := s.opts.UnsubscribeHandler(ctx, req); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if subscribedSessions, ok := s.resourceSubscriptions[req.Params.URI]; ok {
		delete(subscribedSessions, req.Session)
		if len(subscribedSessions) == 0 {
			delete(s.resourceSubscriptions, req.Params.URI)
		}
	}
	s.opts.Logger.Info("resource unsubscribed", "uri", req.Params.URI, "session_id", req.Session.ID())

	return &emptyResult{}, nil
}

func (s *Server) subscriptionsListen(ctx context.Context, req *SubscriptionsListenRequest) (*SubscriptionsListenResult, error) {
	requestID, ok := ctx.Value(idContextKey{}).(jsonrpc.ID)
	if !ok || !requestID.IsValid() {
		return nil, fmt.Errorf("%w: subscriptions/listen requires a request ID", jsonrpc2.ErrInvalidRequest)
	}

	if req.Params.Notifications == nil {
		return nil, fmt.Errorf("%w: missing required 'notifications' field", jsonrpc2.ErrInvalidParams)
	}

	allowed := s.allowedSubscriptions(req.Params.Notifications)
	s.mu.Lock()
	if allowed.ToolsListChanged {
		s.toolChangeSubscriptions[req.Session] = requestID
	}
	if allowed.PromptsListChanged {
		s.promptChangeSubscriptions[req.Session] = requestID
	}
	if allowed.ResourcesListChanged {
		s.resourceChangeSubscriptions[req.Session] = requestID
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.toolChangeSubscriptions, req.Session)
		delete(s.promptChangeSubscriptions, req.Session)
		delete(s.resourceChangeSubscriptions, req.Session)
		s.mu.Unlock()
	}()

	for _, uri := range allowed.ResourceSubscriptions {
		_, err := s.subscribe(ctx, &SubscribeRequest{
			Session: req.Session,
			Params: &SubscribeParams{
				URI:  uri,
				Meta: req.Params.GetMeta(),
			},
		})
		if err != nil {
			return nil, err
		}
		defer s.unsubscribe(ctx, &UnsubscribeRequest{
			Session: req.Session,
			Params: &UnsubscribeParams{
				URI:  uri,
				Meta: req.Params.GetMeta(),
			},
		})
	}

	ackParams := &SubscriptionsAcknowledgedParams{
		Notifications: allowed,
		Meta:          Meta{MetaKeySubscriptionID: requestID.Raw()},
	}
	if err := req.Session.notifySubscriptionAcked(ctx, ackParams); err != nil {
		return nil, fmt.Errorf("sending subscriptions/acknowledged: %w", err)
	}

	// If there are any active subscriptions, we block until the context is cancelled. Otherwise, we return immediately.
	if len(allowed.ResourceSubscriptions) > 0 || allowed.ToolsListChanged || allowed.PromptsListChanged || allowed.ResourcesListChanged {
		<-ctx.Done()
	}
	return &SubscriptionsListenResult{
		Meta: Meta{MetaKeySubscriptionID: requestID.Raw()},
	}, nil
}

func (s *Server) allowedSubscriptions(want *NotificationSubscriptions) NotificationSubscriptions {
	caps := s.capabilities()
	agreed := NotificationSubscriptions{}
	if want.ToolsListChanged && caps.Tools != nil && caps.Tools.ListChanged {
		agreed.ToolsListChanged = true
	}
	if want.PromptsListChanged && caps.Prompts != nil && caps.Prompts.ListChanged {
		agreed.PromptsListChanged = true
	}
	if want.ResourcesListChanged && caps.Resources != nil && caps.Resources.ListChanged {
		agreed.ResourcesListChanged = true
	}
	if len(want.ResourceSubscriptions) > 0 && caps.Resources != nil && caps.Resources.Subscribe {
		agreed.ResourceSubscriptions = slices.Clone(want.ResourceSubscriptions)
	}
	return agreed
}

// Run runs the server over the given transport, which must be persistent.
//
// Run blocks until the client terminates the connection or the provided
// context is cancelled. If the context is cancelled, Run closes the connection.
//
// If tools have been added to the server before this call, then the server will
// advertise the capability for tools, including the ability to send list-changed notifications.
// If no tools have been added, the server will not have the tool capability.
// The same goes for other features like prompts and resources.
//
// Run is a convenience for servers that handle a single session (or one session at a time).
// It need not be called on servers that are used for multiple concurrent connections,
// as with [StreamableHTTPHandler].
func (s *Server) Run(ctx context.Context, t Transport) error {
	s.opts.Logger.Info("server run start")
	ss, err := s.Connect(ctx, t, nil)
	if err != nil {
		s.opts.Logger.Error("server connect failed", "error", err)
		return err
	}

	ssClosed := make(chan error)
	go func() {
		ssClosed <- ss.Wait()
	}()

	select {
	case <-ctx.Done():
		ss.Close()
		<-ssClosed // wait until waiting go routine above actually completes
		s.opts.Logger.Error("server run cancelled", "error", ctx.Err())
		return ctx.Err()
	case err := <-ssClosed:
		if err != nil {
			s.opts.Logger.Error("server session ended with error", "error", err)
		} else {
			s.opts.Logger.Info("server session ended")
		}
		return err
	}
}

// bind implements the binder[*ServerSession] interface, so that Servers can
// be connected using [connect].
func (s *Server) bind(mcpConn Connection, conn *jsonrpc2.Connection, state *ServerSessionState, onClose func()) *ServerSession {
	assert(mcpConn != nil && conn != nil, "nil connection")
	ss := &ServerSession{conn: conn, mcpConn: mcpConn, server: s, onClose: onClose}
	if state != nil {
		ss.state = *state
	}
	s.mu.Lock()
	s.sessions = append(s.sessions, ss)
	s.mu.Unlock()
	s.opts.Logger.Info("server session connected", "session_id", ss.ID())
	return ss
}

// disconnect implements the binder[*ServerSession] interface, so that
// Servers can be connected using [connect].
func (s *Server) disconnect(cc *ServerSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = slices.DeleteFunc(s.sessions, func(cc2 *ServerSession) bool {
		return cc2 == cc
	})

	for _, subscribedSessions := range s.resourceSubscriptions {
		delete(subscribedSessions, cc)
	}
	delete(s.toolChangeSubscriptions, cc)
	delete(s.promptChangeSubscriptions, cc)
	delete(s.resourceChangeSubscriptions, cc)

	s.opts.Logger.Info("server session disconnected", "session_id", cc.ID())
}

// ServerSessionOptions configures the server session.
type ServerSessionOptions struct {
	State *ServerSessionState

	onClose func() // used to clean up associated resources
}

// Connect connects the MCP server over the given transport and starts handling
// messages.
//
// It returns a connection object that may be used to terminate the connection
// (with [Connection.Close]), or await client termination (with
// [Connection.Wait]).
//
// If opts.State is non-nil, it is the initial state for the server.
func (s *Server) Connect(ctx context.Context, t Transport, opts *ServerSessionOptions) (*ServerSession, error) {
	var state *ServerSessionState
	var onClose func()
	if opts != nil {
		state = opts.State
		onClose = opts.onClose
	}

	s.opts.Logger.Info("server connecting")
	ss, err := connect(ctx, t, s, state, onClose, s.opts.Logger)
	if err != nil {
		s.opts.Logger.Error("server connect error", "error", err)
		return nil, err
	}

	// Compute the protocol versions this session can serve, filtered by the
	// transport's capabilities (if it implements [ProtocolVersionSupporter]).
	// The list is consumed by the SEP-2575 server/discover handler.
	//
	// The write is guarded by ss.mu to establish a happens-before edge with
	// the matching read in Server.discover, which runs on the jsonrpc2 read
	// goroutine spawned inside connect(). The two are not concurrent in
	// wall-clock terms (no incoming message is dispatched until the caller
	// has fed the transport, which happens after Server.Connect returns),
	// but without the lock the Go memory model gives the read goroutine no
	// guarantee of seeing this write, and -race flags it.
	ss.mu.Lock()
	ss.supportedVersions = filterSupportedVersions(t)
	ss.mu.Unlock()

	// Start keepalive before returning the session to avoid race conditions with Close.
	// This is safe because the spec allows sending pings before initialization (see ServerSession.handle for details).
	if s.opts.KeepAlive > 0 {
		ss.startKeepalive(ss.server.opts.KeepAlive)
	}

	return ss, nil
}

// TODO: (nit) move all ServerSession methods below the ServerSession declaration.
func (ss *ServerSession) initialized(ctx context.Context, params *InitializedParams) (Result, error) {
	if params == nil {
		// Since we use nilness to signal 'initialized' state, we must ensure that
		// params are non-nil.
		params = new(InitializedParams)
	}
	var wasInit, wasInitd bool
	ss.updateState(func(state *ServerSessionState) {
		wasInit = state.InitializeParams != nil
		wasInitd = state.InitializedParams != nil
		if wasInit && !wasInitd {
			state.InitializedParams = params
		}
	})

	if !wasInit {
		ss.server.opts.Logger.Error("initialized before initialize")
		return nil, fmt.Errorf("%q before %q", notificationInitialized, methodInitialize)
	}
	if wasInitd {
		ss.server.opts.Logger.Error("duplicate initialized notification")
		return nil, fmt.Errorf("duplicate %q received", notificationInitialized)
	}
	if h := ss.server.opts.InitializedHandler; h != nil {
		h(ctx, serverRequestFor(ss, params))
	}
	ss.server.opts.Logger.Info("session initialized")
	return nil, nil
}

func (s *Server) callRootsListChangedHandler(ctx context.Context, req *RootsListChangedRequest) (Result, error) {
	if h := s.opts.RootsListChangedHandler; h != nil {
		h(ctx, req)
	}
	return nil, nil
}

func (ss *ServerSession) callProgressNotificationHandler(ctx context.Context, p *ProgressNotificationParams) (Result, error) {
	if h := ss.server.opts.ProgressNotificationHandler; h != nil {
		h(ctx, serverRequestFor(ss, p))
	}
	return nil, nil
}

// NotifyProgress sends a progress notification from the server to the client
// associated with this session.
// This is typically used to report on the status of a long-running request
// that was initiated by the client.
func (ss *ServerSession) NotifyProgress(ctx context.Context, params *ProgressNotificationParams) error {
	return handleNotify(ctx, notificationProgress, newServerRequest(ss, orZero[Params](params)))
}

// notifySubscriptionAcked sends a "notifications/subscriptions/acknowledged"
// notification on the listen stream represented by this session, indicating
// the subscription filter the server accepted (SEP-2575).
func (ss *ServerSession) notifySubscriptionAcked(ctx context.Context, params *SubscriptionsAcknowledgedParams) error {
	return handleNotify(ctx, notificationSubscriptionsAck, newServerRequest(ss, orZero[Params](params)))
}

func newServerRequest[P Params](ss *ServerSession, params P) *ServerRequest[P] {
	return &ServerRequest[P]{Session: ss, Params: params}
}

// A ServerSession is a logical connection from a single MCP client. Its
// methods can be used to send requests or notifications to the client. Create
// a session by calling [Server.Connect].
//
// Call [ServerSession.Close] to close the connection, or await client
// termination with [ServerSession.Wait].
type ServerSession struct {
	// Ensure that onClose is called at most once.
	// We defensively use an atomic CompareAndSwap rather than a sync.Once, in case the
	// onClose callback triggers a re-entrant call to Close.
	calledOnClose atomic.Bool
	onClose       func()

	server          *Server
	conn            *jsonrpc2.Connection
	mcpConn         Connection
	keepaliveCancel context.CancelFunc

	// supportedVersions is the subset of [supportedProtocolVersions] that the
	// transport can actually serve, computed once at connection time from
	// [ProtocolVersionSupporter] (if implemented by the transport) and used by
	// the SEP-2575 server/discover handler.
	supportedVersions []string

	mu    sync.Mutex
	state ServerSessionState
}

func (ss *ServerSession) updateState(mut func(*ServerSessionState)) {
	ss.mu.Lock()
	mut(&ss.state)
	copy := ss.state
	ss.mu.Unlock()
	if c, ok := ss.mcpConn.(serverConnection); ok {
		c.sessionUpdated(copy)
	}
}

// hasInitialized reports whether the server has received the initialized
// notification.
//
// TODO(findleyr): use this to prevent change notifications.
func (ss *ServerSession) hasInitialized() bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.state.InitializedParams != nil
}

// checkInitialized returns a formatted error if the server has not yet
// received the initialized notification.
func (ss *ServerSession) checkInitialized(method string) error {
	if !ss.hasInitialized() {
		// TODO(rfindley): enable this check.
		// Right now is is flaky, because server tests don't await the initialized notification.
		// Perhaps requests should simply block until they have received the initialized notification

		// if strings.HasPrefix(method, "notifications/") {
		// 	return fmt.Errorf("must not send %q before %q is received", method, notificationInitialized)
		// } else {
		// 	return fmt.Errorf("cannot call %q before %q is received", method, notificationInitialized)
		// }
	}
	return nil
}

func (ss *ServerSession) ID() string {
	if c, ok := ss.mcpConn.(hasSessionID); ok {
		return c.SessionID()
	}
	return ""
}

// assertServerInitiatedRequestAllowed returns an error when the session is
// negotiated at protocol version >= 2026-07-28, where the spec (SEP-2322 /
// SEP-2575) forbids server-initiated JSON-RPC requests for elicitation,
// sampling, and roots: those interactions MUST be embedded as [InputRequests]
// in an [InputRequiredResult] returned from a handler for one of the multi
// round-trip methods (`tools/call`, `prompts/get`, `resources/read`).
func (ss *ServerSession) assertServerInitiatedRequestAllowed(method string) error {
	if iparams := ss.InitializeParams(); iparams != nil &&
		iparams.ProtocolVersion >= protocolVersion20260728 {
		return fmt.Errorf(
			"%q cannot be sent while serving a request on protocol version %s: "+
				"return an InputRequests map instead (multi round-trip requests, SEP-2322)",
			method, iparams.ProtocolVersion)
	}
	return nil
}

// Ping pings the client.
func (ss *ServerSession) Ping(ctx context.Context, params *PingParams) error {
	_, err := handleSend[*emptyResult](ctx, methodPing, newServerRequest(ss, orZero[Params](params)))
	return err
}

// ListRoots lists the client roots.
//
// Deprecated: the roots feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). Migrate to passing paths via tool parameters,
// resource URIs, or configuration. See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
func (ss *ServerSession) ListRoots(ctx context.Context, params *ListRootsParams) (*ListRootsResult, error) {
	if err := ss.checkInitialized(methodListRoots); err != nil {
		return nil, err
	}
	if err := ss.assertServerInitiatedRequestAllowed(methodListRoots); err != nil {
		return nil, err
	}
	return handleSend[*ListRootsResult](ctx, methodListRoots, newServerRequest(ss, orZero[Params](params)))
}

// CreateMessage sends a sampling request to the client.
//
// If the client returns multiple content blocks (e.g. parallel tool calls),
// CreateMessage returns an error. Use [ServerSession.CreateMessageWithTools]
// for tool-enabled sampling.
//
// Deprecated: the sampling feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). Migrate to calling LLM provider APIs directly
// from your server. See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
func (ss *ServerSession) CreateMessage(ctx context.Context, params *CreateMessageParams) (*CreateMessageResult, error) {
	if err := ss.checkInitialized(methodCreateMessage); err != nil {
		return nil, err
	}
	if err := ss.assertServerInitiatedRequestAllowed(methodCreateMessage); err != nil {
		return nil, err
	}
	if params == nil {
		params = &CreateMessageParams{Messages: []*SamplingMessage{}}
	}
	if params.Messages == nil {
		p2 := *params
		p2.Messages = []*SamplingMessage{} // avoid JSON "null"
		params = &p2
	}
	res, err := handleSend[*CreateMessageWithToolsResult](ctx, methodCreateMessage, newServerRequest(ss, orZero[Params](params)))
	if err != nil {
		return nil, err
	}
	// Downconvert to singular content.
	if len(res.Content) > 1 {
		return nil, fmt.Errorf("CreateMessage result has %d content blocks; use CreateMessageWithTools for multiple content", len(res.Content))
	}
	var content Content
	if len(res.Content) > 0 {
		content = res.Content[0]
	}
	return &CreateMessageResult{
		Meta:       res.Meta,
		Content:    content,
		Model:      res.Model,
		Role:       res.Role,
		StopReason: res.StopReason,
	}, nil
}

// CreateMessageWithTools sends a sampling request with tools to the client,
// returning a [CreateMessageWithToolsResult] that supports array content
// (for parallel tool calls). Use this instead of [ServerSession.CreateMessage]
// when the request includes tools.
//
// Deprecated: the sampling feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). Migrate to calling LLM provider APIs directly
// from your server. See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
func (ss *ServerSession) CreateMessageWithTools(ctx context.Context, params *CreateMessageWithToolsParams) (*CreateMessageWithToolsResult, error) {
	if err := ss.checkInitialized(methodCreateMessage); err != nil {
		return nil, err
	}
	if err := ss.assertServerInitiatedRequestAllowed(methodCreateMessage); err != nil {
		return nil, err
	}
	if params == nil {
		params = &CreateMessageWithToolsParams{Messages: []*SamplingMessageV2{}}
	}
	if params.Messages == nil {
		p2 := *params
		p2.Messages = []*SamplingMessageV2{} // avoid JSON "null"
		params = &p2
	}
	return handleSend[*CreateMessageWithToolsResult](ctx, methodCreateMessage, newServerRequest(ss, orZero[Params](params)))
}

// Elicit sends an elicitation request to the client asking for user input.
func (ss *ServerSession) Elicit(ctx context.Context, params *ElicitParams) (*ElicitResult, error) {
	if err := ss.checkInitialized(methodElicit); err != nil {
		return nil, err
	}
	if err := ss.assertServerInitiatedRequestAllowed(methodElicit); err != nil {
		return nil, err
	}
	if params == nil {
		return nil, fmt.Errorf("%w: params cannot be nil", jsonrpc2.ErrInvalidParams)
	}

	params = params.inferElicitMode()

	if iparams := ss.InitializeParams(); iparams == nil || iparams.Capabilities == nil || iparams.Capabilities.Elicitation == nil {
		return nil, fmt.Errorf("client does not support elicitation")
	}
	caps := ss.InitializeParams().Capabilities.Elicitation
	switch params.Mode {
	case "form":
		if caps.Form == nil && caps.URL != nil {
			// Note: if both 'Form' and 'URL' are nil, we assume the client supports
			// form elicitation for backward compatibility.
			return nil, errors.New(`client does not support "form" elicitation`)
		}
	case "url":
		if caps.URL == nil {
			return nil, errors.New(`client does not support "url" elicitation`)
		}
	}

	res, err := handleSend[*ElicitResult](ctx, methodElicit, newServerRequest(ss, orZero[Params](params)))
	if err != nil {
		return nil, err
	}

	if res.Action != "accept" || res.Content == nil {
		return res, nil
	}

	if params.RequestedSchema == nil {
		return res, nil
	}
	schema, err := validateElicitSchema(params.RequestedSchema)
	if err != nil {
		return nil, err
	}
	if schema == nil {
		return res, nil
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, err
	}
	if err := resolved.Validate(res.Content); err != nil {
		return nil, fmt.Errorf("elicitation result content does not match requested schema: %v", err)
	}
	err = resolved.ApplyDefaults(&res.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to apply schema defaults to elicitation result: %v", err)
	}

	return res, nil
}

// Log sends a log message to the client.
//
// For new-protocol (>= 2026-07-28) requests, the level is taken from the
// originating request's `_meta` field (SEP-2575); an absent or empty value
// suppresses the message per spec. For old-protocol requests, the level is
// taken from the session state set via `logging/setLevel`.
//
// Deprecated: the logging feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
func (ss *ServerSession) Log(ctx context.Context, params *LoggingMessageParams) error {
	ss.mu.Lock()
	logLevel := ss.state.LogLevel
	ss.mu.Unlock()
	if logLevel == "" {
		// The spec is unclear, but seems to imply that no log messages are sent until the client
		// sets the level.
		// TODO(jba): read other SDKs, possibly file an issue.
		return nil
	}
	if compareLevels(params.Level, logLevel) < 0 {
		return nil
	}
	return handleNotify(ctx, notificationLoggingMessage, newServerRequest(ss, orZero[Params](params)))
}

// AddSendingMiddleware wraps the current sending method handler using the provided
// middleware. Middleware is applied from right to left, so that the first one is
// executed first.
//
// For example, AddSendingMiddleware(m1, m2, m3) augments the method handler as
// m1(m2(m3(handler))).
//
// Sending middleware is called when a request is sent. It is useful for tasks
// such as tracing, metrics, and adding progress tokens.
func (s *Server) AddSendingMiddleware(middleware ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	addMiddleware(&s.sendingMethodHandler_, middleware)
}

// AddReceivingMiddleware wraps the current receiving method handler using
// the provided middleware. Middleware is applied from right to left, so that the
// first one is executed first.
//
// For example, AddReceivingMiddleware(m1, m2, m3) augments the method handler as
// m1(m2(m3(handler))).
//
// Receiving middleware is called when a request is received. It is useful for tasks
// such as authentication, request logging and metrics.
func (s *Server) AddReceivingMiddleware(middleware ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	addMiddleware(&s.receivingMethodHandler_, middleware)
}

// serverMethodInfos maps from the RPC method name to serverMethodInfos.
//
// The 'allowMissingParams' values are extracted from the protocol schema.
// TODO(rfindley): actually load and validate the protocol schema, rather than
// curating these method flags.
var serverMethodInfos = map[string]methodInfo{
	methodComplete:               newServerMethodInfo(serverMethod((*Server).complete), 0),
	methodDiscover:               newServerMethodInfo(serverMethod((*Server).discover), missingParamsOK),
	methodInitialize:             initializeMethodInfo(),
	methodPing:                   newServerMethodInfo(serverSessionMethod((*ServerSession).ping), missingParamsOK),
	methodListPrompts:            newServerMethodInfo(serverMethod((*Server).listPrompts), missingParamsOK),
	methodGetPrompt:              newServerMethodInfo(serverMethod((*Server).getPrompt), 0),
	methodListTools:              newServerMethodInfo(serverMethod((*Server).listTools), missingParamsOK),
	methodCallTool:               newServerMethodInfo(serverMethod((*Server).callTool), 0),
	methodListResources:          newServerMethodInfo(serverMethod((*Server).listResources), missingParamsOK),
	methodListResourceTemplates:  newServerMethodInfo(serverMethod((*Server).listResourceTemplates), missingParamsOK),
	methodReadResource:           newServerMethodInfo(serverMethod((*Server).readResource), 0),
	methodSetLevel:               newServerMethodInfo(serverSessionMethod((*ServerSession).setLevel), 0),
	methodSubscribe:              newServerMethodInfo(serverMethod((*Server).subscribe), 0),
	methodSubscriptionsListen:    newServerMethodInfo(serverMethod((*Server).subscriptionsListen), 0),
	methodUnsubscribe:            newServerMethodInfo(serverMethod((*Server).unsubscribe), 0),
	notificationCancelled:        newServerMethodInfo(serverSessionMethod((*ServerSession).cancel), notification|missingParamsOK),
	notificationInitialized:      newServerMethodInfo(serverSessionMethod((*ServerSession).initialized), notification|missingParamsOK),
	notificationRootsListChanged: newServerMethodInfo(serverMethod((*Server).callRootsListChangedHandler), notification|missingParamsOK),
	notificationProgress:         newServerMethodInfo(serverSessionMethod((*ServerSession).callProgressNotificationHandler), notification),
}

// initializeMethodInfo handles the workaround for #607: we must set
// params.Capabilities.RootsV2.
func initializeMethodInfo() methodInfo {
	info := newServerMethodInfo(serverSessionMethod((*ServerSession).initialize), 0)
	info.unmarshalParams = func(m json.RawMessage) (Params, error) {
		var params *initializeParamsV2
		if m != nil {
			if err := internaljson.Unmarshal(m, &params); err != nil {
				return nil, fmt.Errorf("unmarshaling %q into a %T: %w", m, params, err)
			}
		}
		if params == nil {
			return nil, fmt.Errorf(`missing required "params"`)
		}
		return params.toV1(), nil
	}
	return info
}

func (ss *ServerSession) sendingMethodInfos() map[string]methodInfo { return clientMethodInfos }

func (s *Server) receivingMethodInfos() map[string]methodInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.receiveMethods
}

func (ss *ServerSession) receivingMethodInfos() map[string]methodInfo {
	return ss.server.receivingMethodInfos()
}

func (ss *ServerSession) sendingMethodHandler() MethodHandler {
	s := ss.server
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sendingMethodHandler_
}

func (ss *ServerSession) receivingMethodHandler() MethodHandler {
	s := ss.server
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.receivingMethodHandler_
}

// getConn implements [session.getConn].
func (ss *ServerSession) getConn() *jsonrpc2.Connection { return ss.conn }

// handle invokes the method described by the given JSON RPC request.
func (ss *ServerSession) handle(ctx context.Context, req *jsonrpc.Request) (any, error) {
	ss.mu.Lock()
	initialized := ss.state.InitializeParams != nil
	ss.mu.Unlock()

	// Per-request protocol detection (SEP-2575): if the request carries
	// `io.modelcontextprotocol/protocolVersion` in its `_meta` field, it
	// follows the new sessionless protocol. The initialization gate is
	// skipped for such requests.
	validatedMeta, perRequestErr := validateRequestMeta(req)
	if perRequestErr != nil {
		return nil, perRequestErr
	}

	if validatedMeta.usesNewProtocol &&
		!slices.Contains(supportedProtocolVersions, validatedMeta.initializeParams.ProtocolVersion) {
		data, _ := json.Marshal(UnsupportedProtocolVersionData{
			Supported: supportedProtocolVersions,
			Requested: validatedMeta.initializeParams.ProtocolVersion,
		})
		return nil, &jsonrpc.Error{
			Code:    CodeUnsupportedProtocolVersion,
			Message: "unsupported protocol version",
			Data:    data,
		}
	}

	switch req.Method {
	case methodInitialize, methodPing, notificationInitialized, notificationRootsListChanged, methodSetLevel, methodSubscribe, methodUnsubscribe:
		if validatedMeta.usesNewProtocol {
			ss.server.opts.Logger.Error("method removed in the new protocol", "method", req.Method)
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeMethodNotFound,
				Message: fmt.Sprintf("%q is not supported in the new protocol", req.Method),
			}
		}
	case methodDiscover:
		// In case of methodDiscover call the state.initializeParams is populated
		// within the discover handle function to make sure the method is supported
		// when the user is probing a pre-2026-07-28 server.
		if !validatedMeta.usesNewProtocol {
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeMethodNotFound,
				Message: fmt.Sprintf("%q is only supported in protocol version >= %s", req.Method, protocolVersion20260728),
			}
		}
	default:
		if !initialized && !validatedMeta.usesNewProtocol {
			ss.server.opts.Logger.Error("method invalid during initialization", "method", req.Method)
			return nil, fmt.Errorf("method %q is invalid during session initialization", req.Method)
		}
		if !initialized && validatedMeta.usesNewProtocol && validatedMeta.initializeParams != nil {
			ss.updateState(func(state *ServerSessionState) {
				state.InitializeParams = validatedMeta.initializeParams
			})
		}
	}

	// modelcontextprotocol/go-sdk#26: handle calls asynchronously, and
	// notifications synchronously, except for 'initialize' which shouldn't be
	// asynchronous to other
	if req.IsCall() && req.Method != methodInitialize {
		jsonrpc2.Async(ctx)
	}

	// For the streamable transport, we need the request ID to correlate
	// server->client calls and notifications to the incoming request from which
	// they originated. See [idContextKey] for details.
	ctx = context.WithValue(ctx, idContextKey{}, req.ID)
	// For new-protocol requests, propagate the per-request log level.
	if validatedMeta.usesNewProtocol {
		ss.setLevel(ctx, &SetLoggingLevelParams{Level: validatedMeta.logLevel})
	}
	res, err := handleReceive(ctx, ss, req)
	if err != nil {
		return nil, err
	}
	if validatedMeta.usesNewProtocol {
		setCompleteResultType(res)
		annotateServerInfo(res, ss.server.impl)
	}
	return res, nil
}

// annotateServerInfo sets [MetaKeyServerInfo] on the result's `_meta` unless
// it is already present. Per SEP-2575, servers should identify themselves in
// each result's `_meta`.
func annotateServerInfo(res Result, impl *Implementation) {
	if res == nil || impl == nil {
		return
	}
	if _, isEmpty := res.(*emptyResult); isEmpty {
		return
	}
	m := res.GetMeta()
	if m == nil {
		m = map[string]any{}
	}
	if _, ok := m[MetaKeyServerInfo]; ok {
		return
	}
	m[MetaKeyServerInfo] = impl
	res.SetMeta(m)
}

// InitializeParams returns the InitializeParams provided during the client's
// initial connection.
func (ss *ServerSession) InitializeParams() *InitializeParams {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.state.InitializeParams
}

func (ss *ServerSession) initialize(ctx context.Context, params *InitializeParams) (*InitializeResult, error) {
	if params == nil {
		return nil, fmt.Errorf("%w: \"params\" must be be provided", jsonrpc2.ErrInvalidParams)
	}
	var wasInit bool
	ss.updateState(func(state *ServerSessionState) {
		wasInit = state.InitializeParams != nil
		if !wasInit {
			state.InitializeParams = params
		}
	})
	if wasInit {
		ss.server.opts.Logger.Error("duplicate initialize request")
		return nil, fmt.Errorf("duplicate %q received", methodInitialize)
	}

	s := ss.server
	return &InitializeResult{
		// TODO(rfindley): alter behavior when falling back to an older version:
		// reject unsupported features.
		ProtocolVersion: negotiatedVersion(params.ProtocolVersion),
		Capabilities:    s.capabilities(),
		Instructions:    s.opts.Instructions,
		ServerInfo:      s.impl,
	}, nil
}

func (ss *ServerSession) ping(context.Context, *PingParams) (*emptyResult, error) {
	return &emptyResult{}, nil
}

// cancel is a placeholder: cancellation is handled the jsonrpc2 package.
//
// It should never be invoked in practice because cancellation is preempted,
// but having its signature here facilitates the construction of methodInfo
// that can be used to validate incoming cancellation notifications.
func (ss *ServerSession) cancel(context.Context, *CancelledParams) (Result, error) {
	return nil, nil
}

func (ss *ServerSession) setLevel(_ context.Context, params *SetLoggingLevelParams) (*emptyResult, error) {
	ss.updateState(func(state *ServerSessionState) {
		state.LogLevel = params.Level
	})
	ss.server.opts.Logger.Info("client log level set", "level", params.Level)
	return &emptyResult{}, nil
}

// Close performs a graceful shutdown of the connection, preventing new
// requests from being handled, and waiting for ongoing requests to return.
// Close then terminates the connection.
//
// Close is idempotent and concurrency safe.
func (ss *ServerSession) Close() error {
	if ss.keepaliveCancel != nil {
		// Note: keepaliveCancel access is safe without a mutex because:
		// 1. keepaliveCancel is only written once during Server.Connect (through startKeepalive),
		//    which happens before any code that may call Close from another goroutine
		// 2. context.CancelFunc is safe to call multiple times and from multiple goroutines
		// 3. The keepalive goroutine calls Close on ping failure, but this is safe since
		//    Close is idempotent and conn.Close() handles concurrent calls correctly
		ss.keepaliveCancel()
	}
	err := ss.conn.Close()

	if ss.onClose != nil && ss.calledOnClose.CompareAndSwap(false, true) {
		ss.onClose()
	}

	return err
}

// Wait waits for the connection to be closed by the client.
func (ss *ServerSession) Wait() error {
	return ss.conn.Wait()
}

// startKeepalive starts the keepalive mechanism for this server session.
func (ss *ServerSession) startKeepalive(interval time.Duration) {
	startKeepalive(ss, interval, ss.server.opts.KeepAliveFailureThreshold, &ss.keepaliveCancel, ss.server.opts.Logger)
}

// pageToken is the internal structure for the opaque pagination cursor.
// It will be Gob-encoded and then Base64-encoded for use as a string token.
type pageToken struct {
	LastUID string // The unique ID of the last resource seen.
}

// encodeCursor encodes a unique identifier (UID) into a opaque pagination cursor
// by serializing a pageToken struct.
func encodeCursor(uid string) (string, error) {
	var buf bytes.Buffer
	token := pageToken{LastUID: uid}
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(token); err != nil {
		return "", fmt.Errorf("failed to encode page token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

// decodeCursor decodes an opaque pagination cursor into the original pageToken struct.
func decodeCursor(cursor string) (*pageToken, error) {
	decodedBytes, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var token pageToken
	buf := bytes.NewBuffer(decodedBytes)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode page token: %w, cursor: %v", err, cursor)
	}
	return &token, nil
}

// paginateList is a generic helper that returns a paginated slice of items
// from a featureSet. It populates the provided result res with the items
// and sets its next cursor for subsequent pages.
// If there are no more pages, the next cursor within the result will be an empty string.
func paginateList[P listParams, R listResult[T], T any](fs *featureSet[T], pageSize int, params P, res R, setFunc func(R, []T)) (R, error) {
	var seq iter.Seq[T]
	if params.cursorPtr() == nil || *params.cursorPtr() == "" {
		seq = fs.all()
	} else {
		pageToken, err := decodeCursor(*params.cursorPtr())
		// According to the spec, invalid cursors should return Invalid params.
		if err != nil {
			var zero R
			return zero, jsonrpc2.ErrInvalidParams
		}
		seq = fs.above(pageToken.LastUID)
	}
	var count int
	var features []T
	for f := range seq {
		count++
		// If we've seen pageSize + 1 elements, we've gathered enough info to determine
		// if there's a next page. Stop processing the sequence.
		if count == pageSize+1 {
			break
		}
		features = append(features, f)
	}
	setFunc(res, features)
	// No remaining pages.
	if count < pageSize+1 {
		return res, nil
	}
	nextCursor, err := encodeCursor(fs.uniqueID(features[len(features)-1]))
	if err != nil {
		var zero R
		return zero, err
	}
	*res.nextCursorPtr() = nextCursor
	return res, nil
}

// AddReceivingCustomMethod registers a handler for a custom (non-standard)
// JSON-RPC method on the server.
//
// When a client sends a request with the given method name, the params will be
// unmarshaled into P, the handler will be called, and the returned R will be
// marshaled as the JSON-RPC result.
//
// Custom methods go through the server's middleware chain just like standard
// MCP methods (tools/call, prompts/list, etc.).
//
// P and R must implement [Params] and [Result] respectively, which is most
// easily done by embedding [ParamsBase] and [ResultBase]:
//
//	type SearchParams struct {
//	    mcp.ParamsBase
//	    Query string `json:"query"`
//	}
//
//	type SearchResult struct {
//	    mcp.ResultBase
//	    Hits []string `json:"hits"`
//	}
//
//	if err := mcp.AddReceivingCustomMethod(server, "acme/search",
//	    func(ctx context.Context, ss *mcp.ServerSession, params *SearchParams) (*SearchResult, error) {
//	        return &SearchResult{Hits: []string{"result"}}, nil
//	    }); err != nil {
//	    return err
//	}
//
// AddReceivingCustomMethod returns an error if method is the name of a
// standard MCP method. Registering the same custom method twice replaces the
// previous handler.
func AddReceivingCustomMethod[P paramsPtr[T], R Result, T any](
	s *Server,
	method string,
	handler func(ctx context.Context, ss *ServerSession, params P) (R, error),
) error {
	if _, ok := serverMethodInfos[method]; ok {
		return fmt.Errorf("mcp: AddReceivingCustomMethod: %q shadows a standard MCP method", method)
	}

	typed := typedServerMethodHandler[P, R](func(ctx context.Context, req *ServerRequest[P]) (R, error) {
		return handler(ctx, req.Session, req.Params)
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	s.receiveMethods[method] = newServerMethodInfo(typed, missingParamsOK)
	return nil
}
