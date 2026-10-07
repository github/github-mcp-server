package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	sdkjson "github.com/segmentio/encoding/json"
)

type inputNormalizationContextKey struct{}
type preflightCompleteKey struct{}
type typedCallStateKey struct{}
type typedRegistrationSetContextKey struct{}

type typedToolRegistrationSet struct {
	byName map[string]*typedToolRegistration
}

type typedCallState struct {
	toolName               string
	handlerCalled          bool
	hasOutput              bool
	contentLengthBeforeSDK int
	preserveContent        bool
	structuredOutput       any
	sdkCalled              bool
	shortCircuit           *mcp.CallToolResult
}

type typedToolRegistration struct {
	name              string
	modernTool        *mcp.Tool
	legacyTool        *mcp.Tool
	modernRuntimeTool *mcp.Tool
	legacyRuntimeTool *mcp.Tool
	hasTypedOutput    bool
	inputNormalizer   InputNormalizer
	preflight         ToolCallPreflight
	fixedEra          ProtocolEra
	preserveContent   bool
	handlerMiddleware []ToolHandlerMiddleware
}

func wrapTypedHandler[In, Out any](handler mcp.ToolHandlerFor[In, Out], inputSchema *jsonschema.Resolved) mcp.ToolHandlerFor[any, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, value any) (*mcp.CallToolResult, any, error) {
		if state, ok := ctx.Value(typedCallStateKey{}).(*typedCallState); ok && state.shortCircuit != nil {
			return state.shortCircuit, nil, nil
		}
		if err := inputSchema.ApplyDefaults(&value); err != nil {
			return typedInputErrorResult(fmt.Errorf("validating \"arguments\": applying schema defaults:\n%w", err)), nil, nil
		}
		if err := inputSchema.Validate(&value); err != nil {
			return typedInputErrorResult(fmt.Errorf("validating \"arguments\": %w", err)), nil, nil
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return typedInputErrorResult(fmt.Errorf("validating \"arguments\": marshalling with defaults: %w", err)), nil, nil
		}
		var input In
		decoder := sdkjson.NewDecoder(bytes.NewReader(encoded))
		decoder.DontMatchCaseInsensitiveStructFields()
		if err := decoder.Decode(&input); err != nil {
			return typedInputErrorResult(err), nil, nil
		}
		var output any
		handlerCalled := false
		next := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			handlerCalled = true
			result, typedOutput, err := handler(ctx, req, input)
			if err == nil && (result == nil || (!result.IsError && result.InputRequests == nil)) {
				output = typedOutput
			}
			return result, err
		}
		result, err := next(ctx, req)
		if err != nil {
			if rpcErr, ok := errors.AsType[*jsonrpc.Error](err); ok {
				return nil, output, rpcErr
			}
			return nil, output, err
		}
		if result == nil {
			result = &mcp.CallToolResult{}
		}
		if state, ok := ctx.Value(typedCallStateKey{}).(*typedCallState); ok {
			state.handlerCalled = handlerCalled
			state.hasOutput = handlerCalled && !result.IsError && result.InputRequests == nil
			state.contentLengthBeforeSDK = len(result.Content)
		}
		if result.IsError || result.InputRequests != nil || !handlerCalled {
			output = nil
		}
		return result, output, nil
	}
}

func typedInputErrorResult(err error) *mcp.CallToolResult {
	result := &mcp.CallToolResult{}
	result.SetError(err)
	return result
}

// typedOutputMiddleware selects immutable advertised schema variants and
// handles request-era compatibility without mutating registered tools.
func typedOutputMiddleware(registrationSet *typedToolRegistrationSet) mcp.Middleware {
	if registrationSet == nil {
		registrationSet = &typedToolRegistrationSet{byName: map[string]*typedToolRegistration{}}
	}
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			switch req := request.(type) {
			case *mcp.ListToolsRequest:
				result, err := next(ctx, method, request)
				if err != nil {
					return nil, err
				}
				list, ok := result.(*mcp.ListToolsResult)
				if !ok {
					return result, nil
				}
				var requestVersion string
				if req.Params != nil {
					requestVersion = requestMetaProtocolVersion(req.Params.Meta)
				}
				era := requestEra(ctx, requestVersion)
				for i, tool := range list.Tools {
					registration := registrationSet.byName[tool.Name]
					if registration == nil {
						continue
					}
					selectedEra := era
					if registration.fixedEra != ProtocolEraDynamic {
						selectedEra = registration.fixedEra
					}
					if selectedEra == ProtocolEraModern {
						list.Tools[i] = registration.modernTool
					} else {
						list.Tools[i] = registration.legacyTool
					}
				}
				return list, nil
			case *mcp.CallToolRequest:
				if req.Params == nil {
					return next(ctx, method, request)
				}
				registration := registrationSet.byName[req.Params.Name]
				if registration == nil {
					return next(ctx, method, request)
				}
				ctx, ownsRequest := claimTypedRegistrationSet(ctx, registrationSet)
				if !ownsRequest {
					return next(ctx, method, request)
				}
				state := &typedCallState{toolName: req.Params.Name}
				ctx = context.WithValue(ctx, typedCallStateKey{}, state)
				requestVersion := requestMetaProtocolVersion(req.Params.Meta)
				dispatch := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					request = req

					if registration != nil && registration.preflight != nil {
						preflightContext, preflightResult, preflightErr := registration.preflight(ctx, req)
						if preflightErr != nil || preflightResult != nil {
							if preflightErr != nil {
								if rpcErr, ok := errors.AsType[*jsonrpc.Error](preflightErr); ok {
									return nil, rpcErr
								}
								if preflightResult == nil {
									preflightResult = &mcp.CallToolResult{}
								}
								preflightResult.SetError(preflightErr)
							}
							era := requestEra(ctx, requestVersion)
							if registration.fixedEra != ProtocolEraDynamic {
								era = registration.fixedEra
							}
							if era == ProtocolEraModern {
								if err := validateExplicitStructuredOutput(outputSchema(registration), preflightResult); err != nil {
									return nil, err
								}
							}
							return preflightResult, nil
						}
						if preflightContext != nil {
							ctx = preflightContext
						}
						ctx = context.WithValue(ctx, typedCallStateKey{}, state)
						ctx = context.WithValue(ctx, preflightCompleteKey{}, req.Params.Name)
					}

					era := requestEra(ctx, requestVersion)
					if registration != nil && registration.fixedEra != ProtocolEraDynamic {
						era = registration.fixedEra
					}
					if registration != nil && registration.inputNormalizer != nil && !inputAlreadyNormalized(ctx, req.Params.Name) {
						arguments := req.Params.Arguments
						if len(arguments) == 0 {
							arguments = json.RawMessage(`{}`)
						}
						var normalized json.RawMessage
						if methodInfo, ok := ghcontext.MCPMethod(ctx); ok &&
							methodInfo.ArgumentsNormalized && methodInfo.ItemName == req.Params.Name {
							normalized = methodInfo.NormalizedArguments
						} else {
							var normalizeErr error
							normalized, normalizeErr = registration.inputNormalizer(arguments)
							if normalizeErr != nil {
								return invalidArgumentsResult(fmt.Errorf("normalize tool arguments: %w", normalizeErr)), nil
							}
						}
						requestCopy := *req
						paramsCopy := *req.Params
						paramsCopy.Arguments = normalized
						requestCopy.Params = &paramsCopy
						request = &requestCopy
						ctx = context.WithValue(ctx, inputNormalizationContextKey{}, req.Params.Name)
					}

					state.sdkCalled = true
					result, err := next(ctx, method, request)
					if err != nil {
						return nil, err
					}
					callResult, ok := result.(*mcp.CallToolResult)
					if !ok {
						return nil, fmt.Errorf("unexpected tools/call result type %T", result)
					}
					if registration != nil && registration.hasTypedOutput && state.hasOutput {
						state.structuredOutput = callResult.StructuredContent
						resultCopy := *callResult
						if err := removeSDKOutputFallback(&resultCopy, state); err != nil {
							return nil, err
						}
						callResult = &resultCopy
						if era != ProtocolEraModern {
							legacyResult := *callResult
							legacyResult.StructuredContent = nil
							callResult = &legacyResult
						}
					} else if registration != nil && registration.hasTypedOutput && !state.handlerCalled && era == ProtocolEraModern {
						if err := validateExplicitStructuredOutput(outputSchema(registration), callResult); err != nil {
							return nil, err
						}
					}
					if registration.hasTypedOutput && era != ProtocolEraModern && callResult.StructuredContent != nil {
						legacyResult := *callResult
						legacyResult.StructuredContent = nil
						callResult = &legacyResult
					}
					return callResult, nil
				}
				callResult, err := applyToolHandlerMiddleware(dispatch, registration.handlerMiddleware...)(ctx, req)
				if err != nil {
					return nil, err
				}
				if registration.hasTypedOutput && !state.sdkCalled && callResult != nil {
					// Guards and preflight run before decoding, but their results
					// must still pass through the SDK's tool-call finalization.
					state.shortCircuit = callResult
					requestCopy := *req
					paramsCopy := *req.Params
					paramsCopy.Arguments = json.RawMessage(`{}`)
					requestCopy.Params = &paramsCopy
					result, err := next(ctx, method, &requestCopy)
					if err != nil {
						return nil, err
					}
					var ok bool
					callResult, ok = result.(*mcp.CallToolResult)
					if !ok {
						return nil, fmt.Errorf("unexpected tools/call result type %T", result)
					}
				}
				era := requestEra(ctx, requestVersion)
				if registration.fixedEra != ProtocolEraDynamic {
					era = registration.fixedEra
				}
				if registration.hasTypedOutput && callResult != nil {
					if era != ProtocolEraModern {
						resultCopy := *callResult
						resultCopy.StructuredContent = nil
						callResult = &resultCopy
					} else {
						if state.hasOutput && state.preserveContent && !callResult.IsError {
							resultCopy := *callResult
							resultCopy.StructuredContent = state.structuredOutput
							callResult = &resultCopy
						}
						if !state.handlerCalled {
							if err := validateExplicitStructuredOutput(outputSchema(registration), callResult); err != nil {
								return nil, err
							}
						}
						if !registration.preserveContent && !state.preserveContent {
							if err := applyModernStructuredText(callResult); err != nil {
								return nil, err
							}
						}
					}
				}
				return callResult, nil
			default:
				return next(ctx, method, request)
			}
		}
	}
}

func claimTypedRegistrationSet(ctx context.Context, registrationSet *typedToolRegistrationSet) (context.Context, bool) {
	if active, ok := ctx.Value(typedRegistrationSetContextKey{}).(*typedToolRegistrationSet); ok {
		return ctx, active == registrationSet
	}
	return context.WithValue(ctx, typedRegistrationSetContextKey{}, registrationSet), true
}

func outputSchema(registration *typedToolRegistration) *jsonschema.Schema {
	if registration == nil || registration.modernTool == nil {
		return nil
	}
	if schema, ok := registration.modernTool.OutputSchema.(*jsonschema.Schema); ok {
		return schema
	}
	if raw, ok := registration.modernTool.OutputSchema.(json.RawMessage); ok {
		var schema jsonschema.Schema
		if json.Unmarshal(raw, &schema) == nil {
			return &schema
		}
	}
	return nil
}

func validateExplicitStructuredOutput(schema *jsonschema.Schema, result *mcp.CallToolResult) error {
	if schema == nil || result == nil || result.IsError || result.InputRequests != nil || result.StructuredContent == nil {
		return nil
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return fmt.Errorf("marshal explicit structured tool output: %w", err)
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return fmt.Errorf("decode explicit structured tool output: %w", err)
	}
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return fmt.Errorf("resolve tool output schema: %w", err)
	}
	if err := resolved.Validate(value); err != nil {
		return fmt.Errorf("validating tool output: %w", err)
	}
	return nil
}

func applyModernStructuredText(result *mcp.CallToolResult) error {
	if result == nil || result.IsError || result.InputRequests != nil || result.StructuredContent == nil {
		return nil
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return fmt.Errorf("marshal structured tool output for text content: %w", err)
	}
	if len(result.Content) > 0 {
		for _, content := range result.Content {
			if _, ok := content.(*mcp.TextContent); !ok {
				return nil
			}
		}
	}
	result.Content = []mcp.Content{&mcp.TextContent{Text: string(encoded)}}
	return nil
}

func removeSDKOutputFallback(result *mcp.CallToolResult, state *typedCallState) error {
	if result.StructuredContent == nil || len(result.Content) <= state.contentLengthBeforeSDK {
		return nil
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return fmt.Errorf("marshal typed tool output while removing fallback content: %w", err)
	}
	removeMatchingLastTextContent(result, encoded)
	return nil
}

func removeMatchingLastTextContent(result *mcp.CallToolResult, encoded []byte) {
	if len(result.Content) == 0 {
		return
	}
	last, ok := result.Content[len(result.Content)-1].(*mcp.TextContent)
	if !ok || !bytes.Equal(bytes.TrimSpace([]byte(last.Text)), bytes.TrimSpace(encoded)) {
		return
	}
	result.Content = result.Content[:len(result.Content)-1]
}

func requestEra(ctx context.Context, requestVersion string) ProtocolEra {
	if requestVersion != "" {
		return ProtocolEraForVersion(requestVersion)
	}
	if info, ok := ghcontext.MCPMethod(ctx); ok && info != nil {
		return ProtocolEraForVersion(info.ProtocolVersion)
	}
	return ProtocolEraLegacy
}

func requestMetaProtocolVersion(meta mcp.Meta) string {
	version, _ := meta[mcp.MetaKeyProtocolVersion].(string)
	return version
}

func preflightAlreadyRun(ctx context.Context, toolName string) bool {
	name, ok := ctx.Value(preflightCompleteKey{}).(string)
	return ok && name == toolName
}

func markPreflightRun(ctx context.Context, toolName string) context.Context {
	return context.WithValue(ctx, preflightCompleteKey{}, toolName)
}

func inputAlreadyNormalized(ctx context.Context, toolName string) bool {
	name, ok := ctx.Value(inputNormalizationContextKey{}).(string)
	return ok && name == toolName
}

// PreserveToolHandlerContent marks a typed call's handler content as
// intentionally non-JSON (for example CSV or resource blocks).
func PreserveToolHandlerContent(ctx context.Context) {
	if state, ok := ctx.Value(typedCallStateKey{}).(*typedCallState); ok {
		state.preserveContent = true
	}
}
