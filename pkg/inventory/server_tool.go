package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"github.com/github/github-mcp-server/v2/pkg/octicons"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandlerFunc is a function that takes dependencies and returns an MCP tool handler.
// This allows tools to be defined statically while their handlers are generated
// on-demand with the appropriate dependencies.
// The deps parameter is typed as `any` to avoid circular dependencies - callers
// should define their own typed dependencies struct and type-assert as needed.
type HandlerFunc func(deps any) mcp.ToolHandler

// ScopeVisibility reports whether a token can use any form of a tool.
type ScopeVisibility func(activeScopes []string) bool

// ScopeChallenge returns the exact scopes to include in an OAuth challenge.
// An empty result means the call can continue.
type ScopeChallenge func(arguments map[string]any, activeScopes []string) []string

// ScopeAccess contains scope metadata and the two checks used by the server.
type ScopeAccess struct {
	// Scopes is the exhaustive upper bound of every scope this tool may request.
	// It is used for documentation, the list-scopes command, and bypassing
	// call-specific challenge evaluation when a token already grants them all.
	Scopes []string

	// Visible is used when filtering tools for a fixed-scope token.
	// Nil means the tool remains visible.
	Visible ScopeVisibility

	// Challenge evaluates one call before its handler runs.
	// Nil means the call does not use OAuth scope challenges.
	Challenge ScopeChallenge

	// Dynamic reports whether Challenge depends on tool arguments. Dynamic
	// policies must declare their exhaustive upper bound in Scopes.
	Dynamic bool

	// ArgumentNormalizer canonicalizes raw arguments before evaluating a
	// dynamic scope challenge. Inventory populates this from the tool's
	// InputNormalizer when building the HTTP scope map.
	ArgumentNormalizer InputNormalizer
}

func (r *typedToolRegistration) runtimeTool(era ProtocolEra) *mcp.Tool {
	if era == ProtocolEraLegacy {
		return r.legacyRuntimeTool
	}
	return r.modernRuntimeTool
}

// ToolHandlerMiddleware wraps an MCP tool handler. Middleware is applied from
// right to left, so the first middleware passed to RegisterFunc executes first.
type ToolHandlerMiddleware func(next mcp.ToolHandler) mcp.ToolHandler

// ToolHandlerMiddlewareProvider creates tool-specific middleware using the
// dependencies supplied when the tool is registered.
type ToolHandlerMiddlewareProvider func(deps any) ToolHandlerMiddleware

// InputNormalizer transforms raw tool arguments before the SDK validates and
// decodes them. Use it only for compatibility normalization; the registered
// input schema remains the contract clients see and the SDK validates.
type InputNormalizer func(json.RawMessage) (json.RawMessage, error)

// ToolCallPreflight inspects raw call arguments before SDK schema validation.
// Return nil, nil to continue; a result or error short-circuits the call.
// Returning a non-nil context passes request-scoped preparation to the handler.
type ToolCallPreflight func(context.Context, *mcp.CallToolRequest) (context.Context, *mcp.CallToolResult, error)

// TypedSchemaOptions customizes schemas inferred for a typed tool.
//
// Input and Output are passed to jsonschema.For when the corresponding schema
// is not provided on the tool. ValidationInputSchema, when set, is used by the
// SDK to validate calls while the tool's declared InputSchema remains visible
// to clients. InputEnums and OutputEnums are applied to inferred schemas at
// registration time.
type TypedSchemaOptions struct {
	Input                  *jsonschema.ForOptions
	Output                 *jsonschema.ForOptions
	ValidationInputSchema  *jsonschema.Schema
	InputEnums             []SchemaEnum
	OutputEnums            []SchemaEnum
	Preflight              ToolCallPreflight
	PreserveHandlerContent bool
}

// ToolsetID is a unique identifier for a toolset.
// Using a distinct type provides compile-time type safety.
type ToolsetID string

// ToolsetMetadata contains metadata about the toolset a tool belongs to.
type ToolsetMetadata struct {
	// ID is the unique identifier for the toolset (e.g., "repos", "issues")
	ID ToolsetID
	// Description provides a human-readable description of the toolset
	Description string
	// Default indicates this toolset should be enabled by default
	Default bool
	// Icon is the name of the Octicon to use for tools in this toolset.
	// Use the base name without size suffix, e.g., "repo" not "repo-16".
	// See https://primer.style/foundations/icons for available icons.
	Icon string
	// InstructionsFunc optionally returns instructions for this toolset.
	// It receives the inventory so it can check what other toolsets are enabled.
	InstructionsFunc func(inv *Inventory) string
}

// Icons returns MCP Icon objects for this toolset, or nil if no icon is set.
// Icons are provided in both 16x16 and 24x24 sizes.
func (tm ToolsetMetadata) Icons() []mcp.Icon {
	return octicons.Icons(tm.Icon)
}

// ServerTool represents an MCP tool with metadata and a handler generator function.
// The tool definition is static, while the handler is generated on-demand
// when the tool is registered with a server.
// Tools are now self-describing with their toolset membership and read-only status
// derived from the Tool.Annotations.ReadOnlyHint field.
type ServerTool struct {
	// Tool is the MCP tool definition containing name, description, schema, etc.
	Tool mcp.Tool

	// Toolset contains metadata about which toolset this tool belongs to.
	Toolset ToolsetMetadata

	// HandlerFunc generates the handler when given dependencies.
	// This allows tools to be passed around without handlers being set up,
	// and handlers are only created when needed.
	HandlerFunc HandlerFunc

	// FeatureRule declares and evaluates the feature flags that control whether
	// this tool is available. Its zero value leaves the tool available.
	FeatureRule FeatureRule

	// Enabled is an optional function called at build/filter time to determine
	// if this tool should be available. If nil, the tool is considered enabled
	// (subject to feature flag checks).
	// The context carries request-scoped information for the consumer to use.
	// Returns (enabled, error). On error, the tool should be treated as disabled.
	Enabled func(ctx context.Context) (bool, error)

	// MinimumProtocolVersion is the oldest MCP protocol version that may list or
	// call this tool. Empty means the tool is available on every version.
	MinimumProtocolVersion string

	// RequiredElicitationMode is the elicitation mode the client must support to
	// list or call this tool. Empty means the tool does not require elicitation.
	RequiredElicitationMode ElicitationMode

	// ScopeAccess controls fixed-token visibility and per-call OAuth challenges.
	ScopeAccess ScopeAccess

	registerTyped              func(*mcp.Server, *typedToolRegistration, ProtocolEra, ...ToolHandlerMiddleware)
	prepareTyped               func(*mcp.Tool) *typedToolRegistration
	handlerMiddlewareProviders []ToolHandlerMiddlewareProvider
	inputNormalizer            InputNormalizer
	schemaOptions              TypedSchemaOptions
}

// IsReadOnly returns true if this tool is marked as read-only via annotations.
func (st *ServerTool) IsReadOnly() bool {
	return st.Tool.Annotations != nil && st.Tool.Annotations.ReadOnlyHint
}

// HasHandler returns true if this tool has a handler function.
func (st *ServerTool) HasHandler() bool {
	return st.HandlerFunc != nil
}

// GetInputNormalizer returns the compatibility normalizer configured for this
// tool, if any.
func (st *ServerTool) GetInputNormalizer() InputNormalizer {
	return st.inputNormalizer
}

// Handler returns a tool handler by calling HandlerFunc with the given dependencies.
// Panics if HandlerFunc is nil - all tools should have handlers.
func (st *ServerTool) Handler(deps any) mcp.ToolHandler {
	if st.HandlerFunc == nil {
		panic("HandlerFunc is nil for tool: " + st.Tool.Name)
	}
	return applyToolHandlerMiddleware(st.HandlerFunc(deps), st.handlerMiddlewares(deps)...)
}

// AddHandlerMiddleware adds dependency-aware middleware used on both typed and
// raw registration paths.
func (st *ServerTool) AddHandlerMiddleware(provider ToolHandlerMiddlewareProvider) {
	if provider != nil {
		st.handlerMiddlewareProviders = append(st.handlerMiddlewareProviders, provider)
	}
}

func (st *ServerTool) handlerMiddlewares(deps any) []ToolHandlerMiddleware {
	middleware := make([]ToolHandlerMiddleware, 0, len(st.handlerMiddlewareProviders))
	for _, provider := range st.handlerMiddlewareProviders {
		middleware = append(middleware, provider(deps))
	}
	return middleware
}

// RegisterFunc registers the tool with the server using the provided dependencies.
// Icons are automatically applied from the toolset metadata if not already set.
// A shallow copy of the tool is made to avoid mutating the original ServerTool.
// Panics if the tool has no handler - all tools should have handlers.
func (st *ServerTool) RegisterFunc(s *mcp.Server, deps any, middleware ...ToolHandlerMiddleware) {
	st.RegisterFuncForProtocolEra(s, deps, ProtocolEraDynamic, middleware...)
}

// RegisterFuncForProtocolEra registers a preselected legacy or modern tool
// variant. Use this when the protocol era is known before server construction.
func (st *ServerTool) RegisterFuncForProtocolEra(s *mcp.Server, deps any, era ProtocolEra, middleware ...ToolHandlerMiddleware) {
	st.registerFunc(s, deps, true, era, nil, middleware...)
}

func (st *ServerTool) register(s *mcp.Server, deps any, era ProtocolEra, registration *typedToolRegistration, middleware ...ToolHandlerMiddleware) *mcp.Tool {
	return st.registerFunc(s, deps, false, era, registration, middleware...)
}

func (st *ServerTool) registerFunc(s *mcp.Server, deps any, addTypedOutputMiddleware bool, era ProtocolEra, registration *typedToolRegistration, middleware ...ToolHandlerMiddleware) *mcp.Tool {
	if st.HandlerFunc == nil {
		panic("HandlerFunc is nil for tool: " + st.Tool.Name)
	}
	// Make a shallow copy of the tool to avoid mutating the original
	toolCopy := st.Tool
	if registration == nil {
		// Apply icons from toolset metadata if tool doesn't have icons set
		if len(toolCopy.Icons) == 0 {
			toolCopy.Icons = st.Toolset.Icons()
		}
		// Project owner/repo routing params to standard MCP-Param-* headers (SEP-2243).
		AnnotateHeaderParams(&toolCopy)
		registration = st.typedRegistration(&toolCopy)
	}
	registration.fixedEra = era
	if addTypedOutputMiddleware {
		s.AddReceivingMiddleware(typedOutputMiddleware(&typedToolRegistrationSet{
			byName: map[string]*typedToolRegistration{st.Tool.Name: registration},
		}))
	}
	allMiddleware := make([]ToolHandlerMiddleware, 0, len(middleware)+len(st.handlerMiddlewareProviders)+1)
	allMiddleware = append(allMiddleware, func(next mcp.ToolHandler) mcp.ToolHandler {
		return st.wrapAvailabilityCheck(next)
	})
	allMiddleware = append(allMiddleware, middleware...)
	allMiddleware = append(allMiddleware, st.handlerMiddlewares(deps)...)
	if st.registerTyped != nil {
		registration.handlerMiddleware = allMiddleware
		st.registerTyped(s, registration, era)
		return registration.runtimeTool(era)
	}

	handler := applyToolHandlerMiddleware(st.HandlerFunc(deps), allMiddleware...)
	s.AddTool(registration.runtimeTool(era), handler)
	return registration.runtimeTool(era)
}

func applyToolHandlerMiddleware(handler mcp.ToolHandler, middleware ...ToolHandlerMiddleware) mcp.ToolHandler {
	for i := range slices.Backward(middleware) {
		handler = middleware[i](handler)
	}
	return handler
}

// HeaderParams maps owner/repo input properties to the MCP-Param-* headers a
// header-aware proxy reads for repository routing. The enforcement test in
// pkg/github guards full coverage.
var HeaderParams = map[string]string{"owner": "owner", "repo": "repo"}

// AnnotateHeaderParams returns a copy of tool whose owner/repo input properties
// carry an "x-mcp-header" annotation, which the
// SDK projects onto Mcp-Param-{name} request headers. It never mutates the
// input tool's schema or any map shared with the original tool definition:
// callers shallow-copy ServerTool.Tool, so the *jsonschema.Schema (and its
// per-property Extra maps) are shared, and per-request registration must not
// race on them. Only the schema, its Properties map, and the specific property
// schemas/Extra maps that gain an annotation are cloned.
func AnnotateHeaderParams(tool *mcp.Tool) {
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	if !ok || schema == nil {
		return
	}
	if _, owned := ownedSchemaPointers.Load(schema); owned {
		entryValue, found := annotatedSchemaCache.Load(schema)
		if !found {
			entryValue, _ = annotatedSchemaCache.LoadOrStore(schema, &inferredSchemaEntry{})
		}
		entry := entryValue.(*inferredSchemaEntry)
		entry.once.Do(func() {
			annotatedTool := mcp.Tool{InputSchema: schema}
			annotateHeaderParams(&annotatedTool)
			entry.schema = annotatedTool.InputSchema.(*jsonschema.Schema)
			ownedSchemaPointers.Store(entry.schema, struct{}{})
		})
		tool.InputSchema = entry.schema
		return
	}
	annotateHeaderParams(tool)
}

func annotateHeaderParams(tool *mcp.Tool) {
	schema := tool.InputSchema.(*jsonschema.Schema)

	// Collect params that actually need an annotation, so a tool without
	// owner/repo (or already annotated) is left untouched and unCloned.
	var toAnnotate []string
	for prop := range HeaderParams {
		if ps := schema.Properties[prop]; ps != nil {
			if _, exists := ps.Extra["x-mcp-header"]; !exists {
				toAnnotate = append(toAnnotate, prop)
			}
		}
	}
	if len(toAnnotate) == 0 {
		return
	}

	// Clone only what we mutate: a fresh schema value, a fresh Properties map,
	// and fresh property schemas with fresh Extra maps. The original schema and
	// its maps are never written to, so concurrent per-request registration is
	// race-free and deterministic.
	schemaCopy := *schema
	schemaCopy.Properties = maps.Clone(schema.Properties)
	for _, prop := range toAnnotate {
		propCopy := *schemaCopy.Properties[prop]
		extra := make(map[string]any, len(propCopy.Extra)+1)
		maps.Copy(extra, propCopy.Extra)
		extra["x-mcp-header"] = HeaderParams[prop]
		propCopy.Extra = extra
		schemaCopy.Properties[prop] = &propCopy
	}
	tool.InputSchema = &schemaCopy
}

// NewServerToolWithContextHandler creates a ServerTool with a handler that receives deps via context.
// This is the preferred approach for tools because it doesn't create closures at registration time,
// which is critical for performance in servers that create a new instance per request.
//
// When Out is concrete, registration uses mcp.AddTool so the SDK infers missing
// schemas and validates typed input and output. Out=any retains the raw handler
// registration path. Optional InputNormalizer callbacks can canonicalize raw
// JSON before SDK validation without weakening the advertised schema. Direct
// Handler calls apply the same normalization before decoding.
//
// The handler function is stored directly without wrapping in a deps closure.
// Dependencies should be injected into context before calling tool handlers.
func NewServerToolWithContextHandler[In any, Out any](tool mcp.Tool, toolset ToolsetMetadata, handler mcp.ToolHandlerFor[In, Out], inputNormalizers ...InputNormalizer) ServerTool {
	return NewServerToolWithContextHandlerAndSchemaOptions(
		tool,
		toolset,
		handler,
		TypedSchemaOptions{},
		inputNormalizers...,
	)
}

// NewServerToolWithContextHandlerAndSchemaOptions is like
// NewServerToolWithContextHandler, with additional schema inference options.
// Inferred input and output schemas are cached per ServerTool.
func NewServerToolWithContextHandlerAndSchemaOptions[In any, Out any](
	tool mcp.Tool,
	toolset ToolsetMetadata,
	handler mcp.ToolHandlerFor[In, Out],
	schemaOptions TypedSchemaOptions,
	inputNormalizers ...InputNormalizer,
) ServerTool {
	schemaOptions = cloneTypedSchemaOptions(schemaOptions)
	cacheExplicitToolSchemas(&tool)
	inputNormalizer := combineInputNormalizers(inputNormalizers)
	serverTool := ServerTool{
		Tool:            tool,
		Toolset:         toolset,
		inputNormalizer: inputNormalizer,
		schemaOptions:   schemaOptions,
		// HandlerFunc ignores deps - deps are retrieved from context at call time
		HandlerFunc: func(_ any) mcp.ToolHandler {
			return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				if schemaOptions.Preflight != nil && !preflightAlreadyRun(ctx, req.Params.Name) {
					preflightContext, result, err := schemaOptions.Preflight(ctx, req)
					if result != nil || err != nil {
						return result, err
					}
					if preflightContext != nil {
						ctx = preflightContext
					}
					ctx = markPreflightRun(ctx, req.Params.Name)
				}
				rawArguments := req.Params.Arguments
				if len(rawArguments) == 0 {
					rawArguments = json.RawMessage(`{}`)
				}
				if inputNormalizer != nil && !inputAlreadyNormalized(ctx, req.Params.Name) {
					normalized, err := inputNormalizer(rawArguments)
					if err != nil {
						return invalidArgumentsResult(fmt.Errorf("normalize tool arguments: %w", err)), nil
					}
					rawArguments = normalized
				}

				if bytes.Equal(bytes.TrimSpace(rawArguments), []byte("null")) {
					return invalidArgumentsResult(fmt.Errorf("arguments must be a JSON object")), nil
				}

				var arguments In
				if err := json.Unmarshal(rawArguments, &arguments); err != nil {
					return invalidArgumentsResult(err), nil
				}
				resp, _, err := handler(ctx, req, arguments)
				return resp, err
			}
		},
	}
	if reflect.TypeFor[Out]() != reflect.TypeFor[any]() {
		serverTool.prepareTyped = func(tool *mcp.Tool) *typedToolRegistration {
			modernTool := *tool
			cacheExplicitToolSchemas(&modernTool)
			if modernTool.InputSchema == nil && reflect.TypeFor[In]() != reflect.TypeFor[any]() {
				inferred, err := CachedInputSchemaFor[In](schemaOptions.Input, schemaOptions.InputEnums...)
				if err != nil {
					panic(fmt.Sprintf("failed to generate input schema for tool %q: %v", tool.Name, err))
				}
				modernTool.InputSchema = inferred
			} else if modernTool.InputSchema == nil {
				inferred, err := cachedObjectInputSchema()
				if err != nil {
					panic(fmt.Sprintf("failed to prepare default input schema for tool %q: %v", tool.Name, err))
				}
				modernTool.InputSchema = inferred
			}
			if modernTool.OutputSchema == nil {
				inferred, err := CachedSchemaFor[Out](schemaOptions.Output, schemaOptions.OutputEnums...)
				if err != nil {
					panic(fmt.Sprintf("failed to generate output schema for tool %q: %v", tool.Name, err))
				}
				modernTool.OutputSchema = inferred
			}
			legacyTool := modernTool
			legacyTool.OutputSchema = nil
			modernRuntimeTool := modernTool
			legacyRuntimeTool := legacyTool
			if schemaOptions.ValidationInputSchema != nil {
				modernRuntimeTool.InputSchema = schemaOptions.ValidationInputSchema
				legacyRuntimeTool.InputSchema = schemaOptions.ValidationInputSchema
			}
			return &typedToolRegistration{
				name:              tool.Name,
				modernTool:        &modernTool,
				legacyTool:        &legacyTool,
				modernRuntimeTool: &modernRuntimeTool,
				legacyRuntimeTool: &legacyRuntimeTool,
				hasTypedOutput:    true,
				inputNormalizer:   inputNormalizer,
				preflight:         schemaOptions.Preflight,
				preserveContent:   schemaOptions.PreserveHandlerContent,
			}
		}
		serverTool.registerTyped = func(server *mcp.Server, registration *typedToolRegistration, era ProtocolEra, _ ...ToolHandlerMiddleware) {
			tool := *registration.runtimeTool(era)
			inputSchema, err := cachedResolvedInputSchema(tool.InputSchema)
			if err != nil {
				panic(fmt.Sprintf("failed to resolve input schema for tool %q: %v", tool.Name, err))
			}
			// Input validation is deferred until guards have allowed the call.
			// Retain the advertised header bindings for checks at the original
			// HTTP boundary, even when the body validation schema is overridden.
			tool.InputSchema, err = cachedHeaderInputSchema(registration.modernTool.InputSchema)
			if err != nil {
				panic(fmt.Sprintf("failed to prepare input schema for tool %q: %v", tool.Name, err))
			}
			mcp.AddTool[any, any](server, &tool, wrapTypedHandler(handler, inputSchema))
		}
	}
	return serverTool
}

func (st *ServerTool) typedRegistration(tool *mcp.Tool) *typedToolRegistration {
	if st.prepareTyped != nil {
		return st.prepareTyped(tool)
	}
	modernTool := *tool
	cacheExplicitToolSchemas(&modernTool)
	legacyTool := modernTool
	return &typedToolRegistration{
		name:              tool.Name,
		modernTool:        &modernTool,
		legacyTool:        &legacyTool,
		modernRuntimeTool: &modernTool,
		legacyRuntimeTool: &legacyTool,
		inputNormalizer:   st.inputNormalizer,
	}
}

func cacheExplicitToolSchemas(tool *mcp.Tool) {
	if schema, ok := tool.InputSchema.(*jsonschema.Schema); ok {
		cached, err := CachedSchema(schema)
		if err != nil {
			panic(fmt.Sprintf("failed to cache input schema for tool %q: %v", tool.Name, err))
		}
		tool.InputSchema = cached
	}
	if schema, ok := tool.OutputSchema.(*jsonschema.Schema); ok {
		cached, err := CachedSchema(schema)
		if err != nil {
			panic(fmt.Sprintf("failed to cache output schema for tool %q: %v", tool.Name, err))
		}
		tool.OutputSchema = cached
	}
}

func cloneTypedSchemaOptions(options TypedSchemaOptions) TypedSchemaOptions {
	options.Input = cloneJSONSchemaForOptions(options.Input)
	options.Output = cloneJSONSchemaForOptions(options.Output)
	if options.ValidationInputSchema != nil {
		cached, err := CachedSchema(options.ValidationInputSchema)
		if err != nil {
			panic(fmt.Sprintf("failed to cache validation input schema: %v", err))
		}
		options.ValidationInputSchema = cached
	}
	options.InputEnums = slices.Clone(options.InputEnums)
	for i := range options.InputEnums {
		options.InputEnums[i].Values = slices.Clone(options.InputEnums[i].Values)
	}
	options.OutputEnums = slices.Clone(options.OutputEnums)
	for i := range options.OutputEnums {
		options.OutputEnums[i].Values = slices.Clone(options.OutputEnums[i].Values)
	}
	return options
}

func cloneJSONSchemaForOptions(options *jsonschema.ForOptions) *jsonschema.ForOptions {
	if options == nil {
		return nil
	}
	optionsCopy := *options
	if options.TypeSchemas != nil {
		optionsCopy.TypeSchemas = make(map[reflect.Type]*jsonschema.Schema, len(options.TypeSchemas))
		for typ, schema := range options.TypeSchemas {
			if schema != nil {
				optionsCopy.TypeSchemas[typ] = CloneSchema(schema)
			} else {
				optionsCopy.TypeSchemas[typ] = nil
			}
		}
	}
	return &optionsCopy
}

func combineInputNormalizers(normalizers []InputNormalizer) InputNormalizer {
	var active []InputNormalizer
	for _, normalizer := range normalizers {
		if normalizer != nil {
			active = append(active, normalizer)
		}
	}
	switch len(active) {
	case 0:
		return nil
	case 1:
		return active[0]
	default:
		return func(arguments json.RawMessage) (json.RawMessage, error) {
			for _, normalizer := range active {
				normalized, err := normalizer(arguments)
				if err != nil {
					return nil, err
				}
				arguments = normalized
			}
			return arguments, nil
		}
	}
}

func invalidArgumentsResult(err error) *mcp.CallToolResult {
	if inputError, ok := errors.AsType[*ToolInputError](err); ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: inputError.Message},
			},
			IsError: true,
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("invalid arguments: %s", err)},
		},
		IsError: true,
	}
}

// NewServerTool creates a ServerTool with a raw handler that receives deps via context.
// This is the preferred constructor for tools that use mcp.ToolHandler directly because
// it doesn't create closures at registration time, which is critical for performance in
// servers that create a new instance per request.
//
// The handler function is stored directly without wrapping in a deps closure.
// Dependencies should be injected into context before calling tool handlers.
func NewServerTool(tool mcp.Tool, toolset ToolsetMetadata, handler mcp.ToolHandler) ServerTool {
	cacheExplicitToolSchemas(&tool)
	return ServerTool{
		Tool:    tool,
		Toolset: toolset,
		// HandlerFunc ignores deps - deps are retrieved from context at call time
		HandlerFunc: func(_ any) mcp.ToolHandler {
			return handler
		},
	}
}
