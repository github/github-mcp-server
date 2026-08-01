// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"golang.org/x/sync/errgroup"
)

const maxMultiRoundTripRetries = 10
const maxLoadSheddingMultiRoundTripRetries = 3

// MultiRoundTripOptions configures the client-side multi round-trip request (SEP-2322)
// middleware. The middleware is enabled by default and automatically fulfills input
// requests from the server by invoking the appropriate client handlers and
// retrying the original call.
type MultiRoundTripOptions struct {
	// Disabled prevents the automatic multi-round-tirp middleware from being installed.
	// When true, the client returns input-required results directly and callers must
	// handle the retry loop themselves using [CallToolResult.NeedsInput],
	// [GetPromptResult.NeedsInput], or [ReadResourceResult.NeedsInput].
	Disabled bool
}

type multiRoundTripResponse interface {
	setResultType(resultType)
	inputRequests() map[string]InputRequest
	requestState() string
	hasContent() bool
}

func handleMultiRoundTripResult(ss *ServerSession, logger *slog.Logger, res multiRoundTripResponse) error {
	if res == nil {
		return nil
	}
	hasInputRequests := res.inputRequests() != nil

	if hasInputRequests && res.hasContent() {
		logger.Warn("handler returned both content and inputRequests")
		return &jsonrpc.Error{
			Code:    jsonrpc.CodeInternalError,
			Message: "server bug: result has both content and inputRequests",
		}
	}

	if clientSupportsMultiRoundTrip(ss) {
		// For older clients the resultType is left unset. Input requests will be handled
		// by serverMultiRoundTripMiddleware client calls and handler reinvocation.
		if hasInputRequests {
			res.setResultType(resultTypeInputRequired)
		} else {
			res.setResultType(resultTypeComplete)
		}
	}
	return nil
}

func clientSupportsMultiRoundTrip(ss *ServerSession) bool {
	protocolVersion := latestProtocolVersion
	if iparams := ss.InitializeParams(); iparams != nil {
		protocolVersion = iparams.ProtocolVersion
	}
	return protocolVersion >= protocolVersion20260728
}

func clientMultiRoundTripMiddleware() Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method != methodCallTool && method != methodGetPrompt && method != methodReadResource {
				return next(ctx, method, req)
			}

			loadSheddingFailures := 0
			for retries := 1; ; retries++ {
				res, err := next(ctx, method, req)
				if err != nil {
					return res, err
				}
				mrtrResult, ok := res.(multiRoundTripResponse)
				if !ok {
					return res, nil
				}
				reqMap := mrtrResult.inputRequests()
				if reqMap == nil {
					return res, nil
				}
				if len(reqMap) == 0 {
					loadSheddingFailures++
				}
				if loadSheddingFailures >= maxLoadSheddingMultiRoundTripRetries {
					return nil, fmt.Errorf("multi-round-trip: exceeded maximum load-shedding retries (%d)", maxLoadSheddingMultiRoundTripRetries)
				}
				if retries >= maxMultiRoundTripRetries {
					return nil, fmt.Errorf("multi-round-trip: exceeded maximum retries (%d)", maxMultiRoundTripRetries)
				}
				cs, ok := req.GetSession().(*ClientSession)
				if !ok {
					return res, nil
				}
				responses, err := fulfillInputRequests(ctx, cs, reqMap)
				if err != nil {
					return nil, err
				}
				setMultiRoundTripRetryParams(req, responses, mrtrResult.requestState())
			}
		}
	}
}

// serverMultiRoundTripMiddleware is a receiving middleware for servers that transparently
// handles multi-round-trip for clients on older protocol versions. When a handler returns
// InputRequests and the client does not support multi-round-trip, the middleware fulfills
// the requests by calling the client directly and reinvokes the handler once with the responses.
func serverMultiRoundTripMiddleware() Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			if method != methodCallTool && method != methodGetPrompt && method != methodReadResource {
				return next(ctx, method, req)
			}

			ss, ok := req.GetSession().(*ServerSession)
			if !ok {
				return next(ctx, method, req)
			}
			if clientSupportsMultiRoundTrip(ss) {
				return next(ctx, method, req)
			}

			res, err := next(ctx, method, req)
			if err != nil {
				return res, err
			}
			mrtrResult, ok := res.(multiRoundTripResponse)
			if !ok {
				return res, nil
			}
			reqMap := mrtrResult.inputRequests()
			if reqMap == nil {
				return res, nil
			}
			if len(reqMap) == 0 {
				return nil, fmt.Errorf("the server is busy, retry later")
			}
			responses, err := fulfillServerInputRequests(ctx, ss, reqMap)
			if err != nil {
				return nil, err
			}
			setMultiRoundTripRetryParams(req, responses, mrtrResult.requestState())
			return next(ctx, method, req)
		}
	}
}

func fulfillServerInputRequests(ctx context.Context, ss *ServerSession, requests InputRequestMap) (InputResponseMap, error) {
	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	responses := make(InputResponseMap, len(requests))
	for id, ir := range requests {
		g.Go(func() error {
			resp, err := fulfillServerInputRequest(ctx, ss, ir)
			if err != nil {
				return fmt.Errorf("fulfilling input request %q: %w", id, err)
			}
			mu.Lock()
			responses[id] = resp
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("multi-round-trip: %w", err)
	}
	return responses, nil
}

func fulfillServerInputRequest(ctx context.Context, ss *ServerSession, ir InputRequest) (InputResponse, error) {
	switch p := ir.(type) {
	case *ElicitParams:
		return ss.Elicit(ctx, p)
	case *CreateMessageParams:
		return ss.CreateMessageWithTools(ctx, createMessageParamsToWithTools(p))
	case *CreateMessageWithToolsParams:
		return ss.CreateMessageWithTools(ctx, p)
	case *ListRootsParams:
		return ss.ListRoots(ctx, p)
	default:
		return nil, fmt.Errorf("unknown input request type: %T", ir)
	}
}

func createMessageParamsToWithTools(p *CreateMessageParams) *CreateMessageWithToolsParams {
	var msgs []*SamplingMessageV2
	for _, m := range p.Messages {
		msgs = append(msgs, &SamplingMessageV2{Content: []Content{m.Content}, Role: m.Role})
	}
	return &CreateMessageWithToolsParams{
		Meta:             p.Meta,
		IncludeContext:   p.IncludeContext,
		MaxTokens:        p.MaxTokens,
		Messages:         msgs,
		Metadata:         p.Metadata,
		ModelPreferences: p.ModelPreferences,
		StopSequences:    p.StopSequences,
		SystemPrompt:     p.SystemPrompt,
		Temperature:      p.Temperature,
	}
}

func setMultiRoundTripRetryParams(req Request, responses InputResponseMap, state string) {
	switch p := req.GetParams().(type) {
	case *CallToolParams:
		p.InputResponses = responses
		p.RequestState = state
	case *CallToolParamsRaw:
		p.InputResponses = responses
		p.RequestState = state
	case *GetPromptParams:
		p.InputResponses = responses
		p.RequestState = state
	case *ReadResourceParams:
		p.InputResponses = responses
		p.RequestState = state
	}
}

func fulfillInputRequests(ctx context.Context, cs *ClientSession, requests InputRequestMap) (InputResponseMap, error) {
	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	responses := make(InputResponseMap, len(requests))
	for id, ir := range requests {
		g.Go(func() error {
			resp, err := fulfillInputRequest(ctx, cs, ir)
			if err != nil {
				return fmt.Errorf("fulfilling input request %q: %w", id, err)
			}
			mu.Lock()
			responses[id] = resp
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("multi round-trip: %w", err)
	}
	return responses, nil
}

func fulfillInputRequest(ctx context.Context, cs *ClientSession, ir InputRequest) (InputResponse, error) {
	switch p := ir.(type) {
	case *ElicitParams:
		return cs.client.elicit(ctx, newClientRequest(cs, p))
	case *CreateMessageParams:
		return cs.client.createMessage(ctx, &CreateMessageWithToolsRequest{Session: cs, Params: createMessageParamsToWithTools(p)})
	case *CreateMessageWithToolsParams:
		return cs.client.createMessage(ctx, &CreateMessageWithToolsRequest{Session: cs, Params: p})
	case *ListRootsParams:
		return cs.client.listRoots(ctx, newClientRequest(cs, p))
	default:
		return nil, fmt.Errorf("unknown input request type: %T", ir)
	}
}
