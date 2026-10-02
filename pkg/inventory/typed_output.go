package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const typedOutputMetaKey = "github.com/github/github-mcp-server/typed-output"

type inputNormalizationContextKey struct{}

type typedOutputMetadata struct {
	hasOutput   bool
	arrayOutput bool
	contentSet  bool
}

func wrapTypedHandler[In, Out any](handler mcp.ToolHandlerFor[In, Out], middleware ...ToolHandlerMiddleware) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		var output Out
		handlerCalled := false
		next := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			handlerCalled = true
			result, typedOutput, err := handler(ctx, req, input)
			output = typedOutput
			return result, err
		}
		next = applyToolHandlerMiddleware(next, middleware...)

		result, err := next(ctx, req)
		if err != nil {
			var zero Out
			return nil, zero, err
		}
		if result == nil {
			result = &mcp.CallToolResult{}
		}
		if result.InputRequests != nil {
			return result, output, nil
		}

		result.Meta = maps.Clone(result.Meta)
		if result.Meta == nil {
			result.Meta = make(mcp.Meta)
		}
		result.Meta[typedOutputMetaKey] = typedOutputMetadata{
			hasOutput:   handlerCalled && !result.IsError,
			arrayOutput: isArrayOutput[Out](),
			contentSet:  result.Content != nil,
		}
		return result, output, nil
	}
}

func isArrayOutput[Out any]() bool {
	kind := reflect.TypeFor[Out]().Kind()
	return kind == reflect.Array || kind == reflect.Slice
}

// typedOutputMiddleware runs after SDK serialization so negotiated protocol
// gating can cover both inferred tool schemas and generated structured output.
func typedOutputMiddleware(normalizerByName map[string]InputNormalizer) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			call, isCall := request.(*mcp.CallToolRequest)
			if isCall && call.Params != nil {
				if normalizedName, ok := ctx.Value(inputNormalizationContextKey{}).(string); !ok || normalizedName != call.Params.Name {
					if normalizer, registered := normalizerByName[call.Params.Name]; registered {
						if normalizer != nil {
							arguments := call.Params.Arguments
							if len(arguments) == 0 {
								arguments = json.RawMessage(`{}`)
							}
							normalized, normalizeErr := normalizer(arguments)
							if normalizeErr != nil {
								return invalidArgumentsResult(fmt.Errorf("normalize tool arguments: %w", normalizeErr)), nil
							}
							callCopy := *call
							paramsCopy := *call.Params
							paramsCopy.Arguments = normalized
							callCopy.Params = &paramsCopy
							request = &callCopy
						}
						ctx = context.WithValue(ctx, inputNormalizationContextKey{}, call.Params.Name)
					}
				}
			}

			result, err := next(ctx, method, request)
			if err != nil {
				return nil, err
			}

			switch req := request.(type) {
			case *mcp.ListToolsRequest:
				list, ok := result.(*mcp.ListToolsResult)
				if !ok {
					return result, nil
				}
				protocolVersion := requestProtocolVersion(ctx, req.ProtocolVersion())
				tools := make([]*mcp.Tool, len(list.Tools))
				for i, tool := range list.Tools {
					_, typed := tool.Meta[typedOutputMetaKey]
					if !typed && tool.OutputSchema == nil {
						tools[i] = tool
						continue
					}
					toolCopy := *tool
					if typed {
						toolCopy.Meta = maps.Clone(tool.Meta)
						delete(toolCopy.Meta, typedOutputMetaKey)
					}
					if !protocolVersionAllowed(protocolVersion, ProtocolVersionMultiRoundTrip) {
						toolCopy.OutputSchema = nil
					}
					tools[i] = &toolCopy
				}
				listCopy := *list
				listCopy.Tools = tools
				return &listCopy, nil
			case *mcp.CallToolRequest:
				callResult, ok := result.(*mcp.CallToolResult)
				if !ok {
					return result, nil
				}
				metadata, typed := callResult.Meta[typedOutputMetaKey].(typedOutputMetadata)
				if !typed {
					return result, nil
				}
				resultCopy := *callResult
				resultCopy.Meta = maps.Clone(callResult.Meta)
				delete(resultCopy.Meta, typedOutputMetaKey)
				if err := removeTypedOutputFallback(&resultCopy, metadata); err != nil {
					return nil, err
				}
				if !metadata.hasOutput || !protocolVersionAllowed(requestProtocolVersion(ctx, req.ProtocolVersion()), ProtocolVersionMultiRoundTrip) {
					resultCopy.StructuredContent = nil
				}
				return &resultCopy, nil
			default:
				return result, nil
			}
		}
	}
}

func removeTypedOutputFallback(result *mcp.CallToolResult, metadata typedOutputMetadata) error {
	if result.StructuredContent == nil {
		return nil
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return fmt.Errorf("marshal typed tool output while removing fallback content: %w", err)
	}
	if metadata.hasOutput {
		if metadata.arrayOutput && metadata.contentSet {
			removeMatchingLastTextContent(result, encoded)
		}
		return nil
	}
	if !metadata.contentSet {
		result.Content = nil
		return nil
	}
	if metadata.arrayOutput {
		removeMatchingLastTextContent(result, encoded)
	}
	return nil
}

func removeMatchingLastTextContent(result *mcp.CallToolResult, encoded []byte) {
	if len(result.Content) < 2 {
		return
	}
	last, ok := result.Content[len(result.Content)-1].(*mcp.TextContent)
	if !ok || !bytes.Equal(bytes.TrimSpace([]byte(last.Text)), bytes.TrimSpace(encoded)) {
		return
	}
	result.Content = result.Content[:len(result.Content)-1]
}

func requestProtocolVersion(ctx context.Context, requestVersion string) string {
	if requestVersion != "" {
		return requestVersion
	}
	if info, ok := ghcontext.MCPMethod(ctx); ok && info != nil {
		return info.ProtocolVersion
	}
	return ""
}
