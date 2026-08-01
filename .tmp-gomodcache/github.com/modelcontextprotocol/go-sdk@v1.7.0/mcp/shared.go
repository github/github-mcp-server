// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file contains code shared between client and server, including
// method handler and middleware definitions.
//
// Much of this is here so that we can factor out commonalities using
// generics. If this becomes unwieldy, it can perhaps be simplified with
// reflection.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	internaljson "github.com/modelcontextprotocol/go-sdk/internal/json"
	"github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2"
	"github.com/modelcontextprotocol/go-sdk/internal/mcpgodebug"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

// nowrapinvalidparams is a compatibility parameter that restores the previous
// behavior of [methodInfo.unmarshalParams]. When unset (the default), a
// params-decoding failure is wrapped with [jsonrpc2.ErrInvalidParams] so the
// wire response carries error code -32602 ("invalid params") rather than the
// zero-value code 0. See:
// https://github.com/modelcontextprotocol/go-sdk/issues/976#issuecomment-4829124838.
//
// See the documentation for the mcpgodebug package for instructions how to enable it.
// The option will be removed in a future version of the SDK.
var nowrapinvalidparams = mcpgodebug.Value("nowrapinvalidparams")

const (
	// latestProtocolVersion is the latest protocol version that this version of
	// the SDK supports.
	//
	// It is the version that the client sends in the initialization request, and
	// the default version used by the server.
	latestProtocolVersion   = protocolVersion20260728
	protocolVersion20260728 = "2026-07-28"
	protocolVersion20251125 = "2025-11-25"
	protocolVersion20250618 = "2025-06-18"
	protocolVersion20250326 = "2025-03-26"
	protocolVersion20241105 = "2024-11-05"
)

var supportedProtocolVersions = []string{
	protocolVersion20260728,
	protocolVersion20251125,
	protocolVersion20250618,
	protocolVersion20250326,
	protocolVersion20241105,
}

// negotiatedVersion returns the effective protocol version to use, given a
// client version.
func negotiatedVersion(clientVersion string) string {
	// In general, prefer to use the clientVersion, but if we don't support the
	// client's version, use the latest version.
	//
	// Cap the supported versions at the legacy protocolVersion20251125, as this
	// method is used by the initialize method which is deprecated in
	// version protocolVersion20260728.
	if slices.Contains(supportedProtocolVersions, clientVersion) && clientVersion < protocolVersion20260728 {
		return clientVersion
	}
	return protocolVersion20251125
}

// negotiateMutuallySupportedVersion returns a protocol version that is supported
// by both the client and the server.
func negotiateMutuallySupportedVersion(supported []string) string {
	for _, ver := range supportedProtocolVersions {
		if slices.Contains(supported, ver) {
			return ver
		}
	}
	return ""
}

// A MethodHandler handles MCP messages.
// For methods, exactly one of the return values must be nil.
// For notifications, both must be nil.
type MethodHandler func(ctx context.Context, method string, req Request) (result Result, err error)

// A Session is either a [ClientSession] or a [ServerSession].
type Session interface {
	// ID returns the session ID, or the empty string if there is none.
	ID() string

	sendingMethodInfos() map[string]methodInfo
	receivingMethodInfos() map[string]methodInfo
	sendingMethodHandler() MethodHandler
	receivingMethodHandler() MethodHandler
	getConn() *jsonrpc2.Connection
}

// Middleware is a function from [MethodHandler] to [MethodHandler].
type Middleware func(MethodHandler) MethodHandler

// addMiddleware wraps the handler in the middleware functions.
func addMiddleware(handlerp *MethodHandler, middleware []Middleware) {
	for _, m := range slices.Backward(middleware) {
		*handlerp = m(*handlerp)
	}
}

func defaultSendingMethodHandler(ctx context.Context, method string, req Request) (Result, error) {
	info, ok := req.GetSession().sendingMethodInfos()[method]
	if !ok {
		// This can be called from user code, with an arbitrary value for method.
		return nil, jsonrpc2.ErrNotHandled
	}
	params := req.GetParams()
	if initParams, ok := params.(*InitializeParams); ok {
		// Fix the marshaling of initialize params, to work around #607.
		//
		// The initialize params we produce should never be nil, nor have nil
		// capabilities, so any panic here is a bug.
		params = initParams.toV2()
	}
	// Notifications don't have results.
	if strings.HasPrefix(method, "notifications/") {
		return nil, req.GetSession().getConn().Notify(ctx, method, params)
	}
	// Create the result to unmarshal into.
	// The concrete type of the result is the return type of the receiving function.
	res := info.newResult()
	if method == methodSubscriptionsListen {
		callSubscriptionsListen(ctx, req.GetSession().getConn(), method, params)
	} else {
		if err := call(ctx, req.GetSession().getConn(), method, params, res); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// Helper method to avoid typed nil.
func orZero[T any, P *U, U any](p P) T {
	if p == nil {
		var zero T
		return zero
	}
	return any(p).(T)
}

func handleNotify(ctx context.Context, method string, req Request) error {
	mh := req.GetSession().sendingMethodHandler()
	_, err := mh(ctx, method, req)
	return err
}

func handleSend[R Result](ctx context.Context, method string, req Request) (R, error) {
	mh := req.GetSession().sendingMethodHandler()
	// mh might be user code, so ensure that it returns the right values for the jsonrpc2 protocol.
	res, err := mh(ctx, method, req)
	if err != nil {
		var z R
		return z, err
	}
	return res.(R), nil
}

// defaultReceivingMethodHandler is the initial MethodHandler for servers and clients, before being wrapped by middleware.
func defaultReceivingMethodHandler[S Session](ctx context.Context, method string, req Request) (Result, error) {
	info, ok := req.GetSession().receivingMethodInfos()[method]
	if !ok {
		// This can be called from user code, with an arbitrary value for method.
		return nil, jsonrpc2.ErrNotHandled
	}
	return info.handleMethod(ctx, method, req)
}

func handleReceive[S Session](ctx context.Context, session S, jreq *jsonrpc.Request) (Result, error) {
	info, err := checkRequest(jreq, session.receivingMethodInfos())
	if err != nil {
		return nil, err
	}
	params, err := info.unmarshalParams(jreq.Params)
	if err != nil {
		return nil, fmt.Errorf("handling '%s': %w", jreq.Method, err)
	}

	mh := session.receivingMethodHandler()
	re, _ := jreq.Extra.(*RequestExtra)
	req := info.newRequest(session, params, re)
	// mh might be user code, so ensure that it returns the right values for the jsonrpc2 protocol.
	res, err := mh(ctx, jreq.Method, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// checkRequest checks the given request against the provided method info, to
// ensure it is a valid MCP request.
//
// If valid, the relevant method info is returned. Otherwise, a non-nil error
// is returned describing why the request is invalid.
//
// This is extracted from request handling so that it can be called in the
// transport layer to preemptively reject bad requests.
func checkRequest(req *jsonrpc.Request, infos map[string]methodInfo) (methodInfo, error) {
	info, ok := infos[req.Method]
	if !ok {
		return methodInfo{}, fmt.Errorf("%w: %q unsupported", jsonrpc2.ErrNotHandled, req.Method)
	}
	if info.flags&notification != 0 && req.IsCall() {
		return methodInfo{}, fmt.Errorf("%w: unexpected id for %q", jsonrpc2.ErrInvalidRequest, req.Method)
	}
	if info.flags&notification == 0 && !req.IsCall() {
		return methodInfo{}, fmt.Errorf("%w: missing id for %q", jsonrpc2.ErrInvalidRequest, req.Method)
	}
	// missingParamsOK is checked here to catch the common case where "params" is
	// missing entirely.
	//
	// However, it's checked again after unmarshalling to catch the rare but
	// possible case where "params" is JSON null (see https://go.dev/issue/33835).
	if info.flags&missingParamsOK == 0 && len(req.Params) == 0 {
		return methodInfo{}, fmt.Errorf("%w: missing required \"params\"", jsonrpc2.ErrInvalidRequest)
	}
	return info, nil
}

// methodInfo is information about sending and receiving a method.
type methodInfo struct {
	// flags is a collection of flags controlling how the JSONRPC method is
	// handled. See individual flag values for documentation.
	flags methodFlags
	// Unmarshal params from the wire into a Params struct.
	// Used on the receive side.
	unmarshalParams func(json.RawMessage) (Params, error)
	newRequest      func(Session, Params, *RequestExtra) Request
	// Run the code when a call to the method is received.
	// Used on the receive side.
	handleMethod MethodHandler
	// Create a pointer to a Result struct.
	// Used on the send side.
	newResult func() Result
}

// The following definitions support converting from typed to untyped method handlers.
// Type parameter meanings:
// - S: sessions
// - P: params
// - R: results

// A typedMethodHandler is like a MethodHandler, but with type information.
type (
	typedClientMethodHandler[P Params, R Result] func(context.Context, *ClientRequest[P]) (R, error)
	typedServerMethodHandler[P Params, R Result] func(context.Context, *ServerRequest[P]) (R, error)
)

type paramsPtr[T any] interface {
	*T
	Params
}

type methodFlags int

const (
	notification    methodFlags = 1 << iota // method is a notification, not request
	missingParamsOK                         // params may be missing or null
)

func newClientMethodInfo[P paramsPtr[T], R Result, T any](d typedClientMethodHandler[P, R], flags methodFlags) methodInfo {
	mi := newMethodInfo[P, R](flags)
	mi.newRequest = func(s Session, p Params, _ *RequestExtra) Request {
		r := &ClientRequest[P]{Session: s.(*ClientSession)}
		if p != nil {
			r.Params = p.(P)
		}
		return r
	}
	mi.handleMethod = MethodHandler(func(ctx context.Context, _ string, req Request) (Result, error) {
		return d(ctx, req.(*ClientRequest[P]))
	})
	return mi
}

func newServerMethodInfo[P paramsPtr[T], R Result, T any](d typedServerMethodHandler[P, R], flags methodFlags) methodInfo {
	mi := newMethodInfo[P, R](flags)
	mi.newRequest = func(s Session, p Params, re *RequestExtra) Request {
		r := &ServerRequest[P]{Session: s.(*ServerSession), Extra: re}
		if p != nil {
			r.Params = p.(P)
		}
		return r
	}
	mi.handleMethod = MethodHandler(func(ctx context.Context, _ string, req Request) (Result, error) {
		return d(ctx, req.(*ServerRequest[P]))
	})
	return mi
}

// newMethodInfo creates a methodInfo from a typedMethodHandler.
//
// If isRequest is set, the method is treated as a request rather than a
// notification.
func newMethodInfo[P paramsPtr[T], R Result, T any](flags methodFlags) methodInfo {
	return methodInfo{
		flags: flags,
		unmarshalParams: func(m json.RawMessage) (Params, error) {
			var p P
			if m != nil {
				if err := internaljson.Unmarshal(m, &p); err != nil {
					// Legacy behavior: pre-fix versions surfaced this as a
					// plain wrapped error, which caused the wire response to
					// carry code 0 instead of -32602. Restore via
					// MCPGODEBUG=nowrapinvalidparams=1.
					if nowrapinvalidparams == "1" {
						return nil, fmt.Errorf("unmarshaling %q into a %T: %w", m, p, err)
					}
					// Wrap jsonrpc2.ErrInvalidParams so toWireError surfaces
					// code -32602 ("invalid params") while preserving the
					// descriptive message.
					return nil, fmt.Errorf("%w: unmarshaling %q into a %T: %w", jsonrpc2.ErrInvalidParams, m, p, err)
				}
			}
			// We must check missingParamsOK here, in addition to checkRequest, to
			// catch the edge cases where "params" is set to JSON null.
			// See also https://go.dev/issue/33835.
			//
			// We need to ensure that p is non-null to guard against crashes, as our
			// internal code or externally provided handlers may assume that params
			// is non-null.
			if flags&missingParamsOK == 0 && p == nil {
				return nil, fmt.Errorf("%w: missing required \"params\"", jsonrpc2.ErrInvalidRequest)
			}
			return orZero[Params](p), nil
		},
		// newResult is used on the send side, to construct the value to unmarshal the result into.
		// R is a pointer to a result struct. There is no way to "unpointer" it without reflection.
		// TODO(jba): explore generic approaches to this, perhaps by treating R in
		// the signature as the unpointered type.
		newResult: func() Result { return reflect.New(reflect.TypeFor[R]().Elem()).Interface().(R) },
	}
}

// serverMethod is glue for creating a typedMethodHandler from a method on Server.
func serverMethod[P Params, R Result](
	f func(*Server, context.Context, *ServerRequest[P]) (R, error),
) typedServerMethodHandler[P, R] {
	return func(ctx context.Context, req *ServerRequest[P]) (R, error) {
		return f(req.Session.server, ctx, req)
	}
}

// clientMethod is glue for creating a typedMethodHandler from a method on Client.
func clientMethod[P Params, R Result](
	f func(*Client, context.Context, *ClientRequest[P]) (R, error),
) typedClientMethodHandler[P, R] {
	return func(ctx context.Context, req *ClientRequest[P]) (R, error) {
		return f(req.Session.client, ctx, req)
	}
}

// serverSessionMethod is glue for creating a typedServerMethodHandler from a method on ServerSession.
func serverSessionMethod[P Params, R Result](f func(*ServerSession, context.Context, P) (R, error)) typedServerMethodHandler[P, R] {
	return func(ctx context.Context, req *ServerRequest[P]) (R, error) {
		return f(req.GetSession().(*ServerSession), ctx, req.Params)
	}
}

// clientSessionMethod is glue for creating a typedMethodHandler from a method on ServerSession.
func clientSessionMethod[P Params, R Result](f func(*ClientSession, context.Context, P) (R, error)) typedClientMethodHandler[P, R] {
	return func(ctx context.Context, req *ClientRequest[P]) (R, error) {
		return f(req.GetSession().(*ClientSession), ctx, req.Params)
	}
}

// MCP-specific error codes.
const (
	// CodeHeaderMismatch indicates that HTTP headers do not match the corresponding values
	// in the request body, or that required headers are missing or malformed.
	CodeHeaderMismatch = -32020
	// CodeMissingRequiredClientCapabilities is the JSON-RPC error code defined by
	// SEP-2575 for MissingRequiredClientCapabilitiesError.
	CodeMissingRequiredClientCapabilities = -32021
	// CodeUnsupportedProtocolVersion is the JSON-RPC error code defined by
	// SEP-2575 for UnsupportedProtocolVersionError.
	CodeUnsupportedProtocolVersion = -32022
	// CodeURLElicitationRequired indicates that the server requires URL elicitation
	// before processing the request. The client should execute the elicitation handler
	// with the elicitations provided in the error data.
	CodeURLElicitationRequired = -32042
)

// CodeResourceNotFound indicates that a requested resource could not be found.
//
// By default, the value is -32602 (Invalid Params), as specified in the
// MCP specification (SEP-2164). To restore the pre-1.7.0 release behavior where the
// error code was -32002, set MCPGODEBUG=customresnotfounderrcode=1.
//
// Deprecated: Use [jsonrpc.CodeInvalidParams] directly. This variable will be
// removed in a future version.
var CodeResourceNotFound int64 = jsonrpc.CodeInvalidParams

// URLElicitationRequiredError returns an error indicating that URL elicitation is required
// before the request can be processed. The elicitations parameter should contain the
// elicitation requests that must be completed.
func URLElicitationRequiredError(elicitations []*ElicitParams) error {
	// Validate that all elicitations are URL mode
	for _, elicit := range elicitations {
		mode := elicit.Mode
		if mode == "" {
			mode = "form" // default mode
		}
		if mode != "url" {
			panic(fmt.Sprintf("URLElicitationRequiredError requires all elicitations to be URL mode, got %q", mode))
		}
	}

	data, err := json.Marshal(map[string]any{
		"elicitations": elicitations,
	})
	if err != nil {
		// This should never happen with valid ElicitParams
		panic(fmt.Sprintf("failed to marshal elicitations: %v", err))
	}
	return &jsonrpc.Error{
		Code:    CodeURLElicitationRequired,
		Message: "URL elicitation required",
		Data:    json.RawMessage(data),
	}
}

// Internal error codes
const (
	// The error code if the method exists and was called properly, but the peer does not support it.
	//
	// TODO(rfindley): this code is wrong, and we should fix it to be
	// consistent with other SDKs.
	codeUnsupportedMethod = -31001
)

// notifySessions calls Notify on all the sessions.
// Should be called on a copy of the peer sessions.
// The logger must be non-nil.
func notifySessions[S Session, P Params](sessions []S, method string, params P, logger *slog.Logger) {
	if sessions == nil {
		return
	}
	// Notify with the background context, so the messages are sent on the
	// standalone stream.
	// TODO: make this timeout configurable, or call handleNotify asynchronously.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// TODO: there's a potential spec violation here, when the feature list
	// changes before the session (client or server) is initialized.
	for _, s := range sessions {
		req := newRequest(s, params)
		if err := handleNotify(ctx, method, req); err != nil {
			logger.Warn(fmt.Sprintf("calling %s: %v", method, err))
		}
	}
}

func newRequest[S Session, P Params](s S, p P) Request {
	switch s := any(s).(type) {
	case *ClientSession:
		return &ClientRequest[P]{Session: s, Params: p}
	case *ServerSession:
		return &ServerRequest[P]{Session: s, Params: p}
	default:
		panic("bad session")
	}
}

// Meta is additional metadata for requests, responses and other types.
type Meta map[string]any

// GetMeta returns metadata from a value.
func (m Meta) GetMeta() map[string]any { return m }

// SetMeta sets the metadata on a value.
func (m *Meta) SetMeta(x map[string]any) { *m = x }

const progressTokenKey = "progressToken"

func getProgressToken(p Params) any {
	return p.GetMeta()[progressTokenKey]
}

func setProgressToken(p Params, pt any) {
	switch pt.(type) {
	// Support int32 and int64 for atomic.IntNN.
	case int, int32, int64, string:
	default:
		panic(fmt.Sprintf("progress token %v is of type %[1]T, not int or string", pt))
	}
	m := p.GetMeta()
	if m == nil {
		m = map[string]any{}
		p.SetMeta(m)
	}
	m[progressTokenKey] = pt
}

// extractRequestMeta performs a lightweight partial unmarshal of the `_meta`
// field from a JSON-RPC request's raw params.
func extractRequestMeta(rawParams json.RawMessage) Meta {
	if len(rawParams) == 0 {
		return nil
	}
	var meta struct {
		Meta Meta `json:"_meta"`
	}
	if err := internaljson.Unmarshal(rawParams, &meta); err != nil {
		return nil
	}
	return meta.Meta
}

type validatedMeta struct {
	usesNewProtocol  bool
	initializeParams *InitializeParams
	logLevel         LoggingLevel
}

// validateRequestMeta inspects a JSON-RPC request to detect whether it follows
// the >= 2026-07-28 protocol via the `_meta` field.
// If the request has no _meta, or no protocolVersion in _meta, it returns a non-nil
// validatedMeta with usesNewProtocol set to false, and a nil error.
// If the request has a protocolVersion in _meta it validates the presence of
// clientCapabilities in _meta. If it is missing or invalid, it returns nil and
// a non-nil error. clientInfo is optional; if present but invalid, an error is
// returned. Otherwise, it returns usesNewProtocol set to true and the populated
// initializeParams.
func validateRequestMeta(req *jsonrpc.Request) (*validatedMeta, error) {
	meta := extractRequestMeta(req.Params)
	if meta == nil {
		return &validatedMeta{usesNewProtocol: false, initializeParams: nil}, nil
	}
	protocolVersion, ok := meta[MetaKeyProtocolVersion].(string)
	if !ok || protocolVersion < protocolVersion20260728 {
		return &validatedMeta{usesNewProtocol: false, initializeParams: nil}, nil
	}
	var clientInfo *Implementation
	if _, present := meta[MetaKeyClientInfo]; present {
		var ok bool
		clientInfo, ok = decodeMetaValue[*Implementation](meta, MetaKeyClientInfo)
		if !ok {
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeInvalidParams,
				Message: fmt.Sprintf("invalid _meta field %q", MetaKeyClientInfo),
			}
		}
	}
	capabilities, ok := decodeMetaValue[*clientCapabilitiesV2](meta, MetaKeyClientCapabilities)
	if !ok {
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: fmt.Sprintf("missing or invalid _meta field %q", MetaKeyClientCapabilities),
		}
	}
	logLevel, _ := decodeMetaValue[LoggingLevel](meta, MetaKeyLogLevel)
	return &validatedMeta{usesNewProtocol: true, initializeParams: &InitializeParams{
		ProtocolVersion: protocolVersion,
		Capabilities:    capabilities.toV1(),
		ClientInfo:      clientInfo,
	}, logLevel: logLevel}, nil
}

// A Request is a method request with parameters and additional information, such as the session.
// Request is implemented by [*ClientRequest] and [*ServerRequest].
type Request interface {
	isRequest()
	GetSession() Session
	GetParams() Params
	// GetExtra returns the Extra field for ServerRequests, and nil for ClientRequests.
	GetExtra() *RequestExtra
}

// A ClientRequest is a request to a client.
type ClientRequest[P Params] struct {
	Session *ClientSession
	Params  P
}

// A ServerRequest is a request to a server.
type ServerRequest[P Params] struct {
	Session *ServerSession
	Params  P
	Extra   *RequestExtra
}

// RequestExtra is extra information included in requests, typically from
// the transport layer.
type RequestExtra struct {
	TokenInfo *auth.TokenInfo // bearer token info (e.g. from OAuth) if any
	Header    http.Header     // header from HTTP request, if any

	// If set, CloseSSEStream explicitly closes the current SSE request stream.
	//
	// [SEP-1699] introduced server-side SSE stream disconnection: for
	// long-running requests, servers may opt to close the SSE stream and
	// ask the client to retry at a later time. CloseSSEStream implements this
	// feature; if RetryAfter is set, an event is sent with a `retry:` field
	// to configure the reconnection delay.
	//
	// [SEP-1699]: https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1699
	// This mechanism is deprecated in protocol version 2026-07-28 as the resumability
	// feature is removed.
	CloseSSEStream func(CloseSSEStreamArgs)
}

// CloseSSEStreamArgs are arguments for [RequestExtra.CloseSSEStream].
type CloseSSEStreamArgs struct {
	// RetryAfter configures the reconnection delay sent to the client via the
	// SSE retry field. If zero, no retry field is sent.
	RetryAfter time.Duration
}

func (*ClientRequest[P]) isRequest() {}
func (*ServerRequest[P]) isRequest() {}

func (r *ClientRequest[P]) GetSession() Session { return r.Session }
func (r *ServerRequest[P]) GetSession() Session { return r.Session }

func (r *ClientRequest[P]) GetParams() Params { return r.Params }
func (r *ServerRequest[P]) GetParams() Params { return r.Params }

func (r *ClientRequest[P]) GetExtra() *RequestExtra { return nil }
func (r *ServerRequest[P]) GetExtra() *RequestExtra { return r.Extra }

// ProtocolVersion returns the protocol version negotiated for this request.
//
// For requests following the >= 2026-07-28 protocol, the value is read from
// the per-request `_meta` field. For older protocol requests, the value falls
// back to the session-level [InitializeParams] established during the
// initialize handshake.
func (r *ServerRequest[P]) ProtocolVersion() string {
	if m := getRequestMeta(r); m != nil {
		if v, ok := m[MetaKeyProtocolVersion].(string); ok {
			return v
		}
	}
	if r.Session != nil {
		if p := r.Session.InitializeParams(); p != nil {
			return p.ProtocolVersion
		}
	}
	return ""
}

// ClientInfo returns the [Implementation] identifying the calling client.
//
// For requests following the >= 2026-07-28 protocol, the value is read from
// the per-request `_meta` field. For older protocol requests, the value falls
// back to the session-level [InitializeParams].
func (r *ServerRequest[P]) ClientInfo() *Implementation {
	if m := getRequestMeta(r); m != nil {
		if v, ok := decodeMetaValue[*Implementation](m, MetaKeyClientInfo); ok {
			return v
		}
	}
	if r.Session != nil {
		if p := r.Session.InitializeParams(); p != nil {
			return p.ClientInfo
		}
	}
	return nil
}

// ClientCapabilities returns the [ClientCapabilities] of the calling client.
//
// For requests following the >= 2026-07-28 protocol, the value is read from
// the per-request `_meta` field. For older protocol requests, the value falls
// back to the session-level [InitializeParams].
func (r *ServerRequest[P]) ClientCapabilities() *ClientCapabilities {
	if m := getRequestMeta(r); m != nil {
		if v, ok := decodeMetaValue[*clientCapabilitiesV2](m, MetaKeyClientCapabilities); ok {
			return v.toV1()
		}
	}
	if r.Session != nil {
		if p := r.Session.InitializeParams(); p != nil {
			return p.Capabilities
		}
	}
	return nil
}

// getRequestMeta returns the raw `_meta` map from the request's params, or
// nil if the params are absent.
func getRequestMeta[P Params](r *ServerRequest[P]) map[string]any {
	// In practice P is a pointer type implementing Params.
	if any(r.Params) == nil || r.Params.isNil() {
		return nil
	}
	return r.Params.GetMeta()
}

// decodeMetaValue decodes a typed value out of a `_meta` map. Values may
// arrive either as the typed Go value (when constructed in-process) or as
// the generic JSON map produced by encoding/json after wire transit. In the
// latter case, the value is re-encoded and decoded into the target type.
func decodeMetaValue[T any](m map[string]any, key string) (T, bool) {
	var zero T
	raw, ok := m[key]
	if !ok || raw == nil {
		return zero, false
	}
	if v, ok := raw.(T); ok {
		return v, true
	}
	var v T
	if err := remarshal(raw, &v); err != nil {
		return zero, false
	}
	return v, true
}

func serverRequestFor[P Params](s *ServerSession, p P) *ServerRequest[P] {
	return &ServerRequest[P]{Session: s, Params: p}
}

func clientRequestFor[P Params](s *ClientSession, p P) *ClientRequest[P] {
	return &ClientRequest[P]{Session: s, Params: p}
}

// Params is a parameter (input) type for an MCP call or notification.
type Params interface {
	// GetMeta returns metadata from a value.
	GetMeta() map[string]any
	// SetMeta sets the metadata on a value.
	SetMeta(map[string]any)

	// isParams discourages implementation of Params outside of this package.
	isParams()

	// isNil returns true if the underlying value is nil.
	isNil() bool
}

// ParamsBase can be embedded in custom parameter structs to satisfy the
// [Params] interface. It provides the required [Meta] field and the unexported
// isParams marker method.
//
//	type SearchParams struct {
//	    mcp.ParamsBase
//	    Query string `json:"query"`
//	}
type ParamsBase struct {
	Meta `json:"_meta,omitempty"`
}

func (*ParamsBase) isParams() {}

func (p *ParamsBase) isNil() bool { return p == nil }

// RequestParams is a parameter (input) type for an MCP request.
type RequestParams interface {
	Params

	// GetProgressToken returns the progress token from the params' Meta field, or nil
	// if there is none.
	GetProgressToken() any

	// SetProgressToken sets the given progress token into the params' Meta field.
	// It panics if its argument is not an int or a string.
	SetProgressToken(any)
}

// Result is a result of an MCP call.
type Result interface {
	// isResult discourages implementation of Result outside of this package.
	isResult()

	// GetMeta returns metadata from a value.
	GetMeta() map[string]any
	// SetMeta sets the metadata on a value.
	SetMeta(map[string]any)
}

// ResultBase can be embedded in custom result structs to satisfy the
// [Result] interface. It provides the required [Meta] field and the unexported
// isResult marker method.
//
//	type SearchResult struct {
//	    mcp.ResultBase
//	    Hits []string `json:"hits"`
//	}
type ResultBase struct {
	Meta `json:"_meta,omitempty"`
}

func (*ResultBase) isResult() {}

// emptyResult is returned by methods that have no result, like ping.
// Those methods cannot return nil, because jsonrpc2 cannot handle nils.
type emptyResult struct{}

func (*emptyResult) isResult()               {}
func (*emptyResult) GetMeta() map[string]any { panic("should never be called") }
func (*emptyResult) SetMeta(map[string]any)  { panic("should never be called") }

type listParams interface {
	// Returns a pointer to the param's Cursor field.
	cursorPtr() *string
}

type listResult[T any] interface {
	// Returns a pointer to the param's NextCursor field.
	nextCursorPtr() *string
}

// keepaliveSession represents a session that supports keepalive functionality.
type keepaliveSession interface {
	Ping(ctx context.Context, params *PingParams) error
	Close() error
}

// startKeepalive starts the keepalive mechanism for a session.
// It assigns the cancel function to the provided cancelPtr and starts a goroutine
// that sends ping messages at the specified interval.
//
// failureThreshold is the number of consecutive ping failures tolerated before
// the session is closed; a value below 1 is treated as 1 (close on the first
// failure). A successful ping resets the counter. This mirrors the spec's
// "multiple failed pings MAY trigger a connection reset" language, letting a
// transient miss pass without tearing down an otherwise live session.
//
// logger must be non-nil; ping failures (both the tolerated ones and the final
// one that closes the session) are reported via logger so they are not silently
// dropped.
func startKeepalive(session keepaliveSession, interval time.Duration, failureThreshold int, cancelPtr *context.CancelFunc, logger *slog.Logger) {
	if failureThreshold < 1 {
		failureThreshold = 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Assign cancel function before starting goroutine to avoid race condition.
	// We cannot return it because the caller may need to cancel during the
	// window between goroutine scheduling and function return.
	*cancelPtr = cancel

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		consecutiveFailures := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(context.Background(), interval/2)
				err := session.Ping(pingCtx, nil)
				pingCancel()
				if err == nil {
					consecutiveFailures = 0
					continue
				}
				if errors.Is(err, jsonrpc2.ErrMethodNotFound) {
					// Peer doesn't support ping, stop the keepalive process.
					return
				}
				consecutiveFailures++
				if consecutiveFailures < failureThreshold {
					// Tolerate transient failures below the threshold; log so
					// the misses are still observable to operators. See #218.
					logger.Warn("keepalive ping failed; tolerating below threshold",
						"error", err,
						"consecutiveFailures", consecutiveFailures,
						"failureThreshold", failureThreshold)
					continue
				}
				// Threshold reached; log before closing the session so the
				// failure is observable to operators. See #218.
				logger.Error("keepalive ping failed; closing session",
					"error", err,
					"consecutiveFailures", consecutiveFailures,
					"failureThreshold", failureThreshold)
				_ = session.Close()
				return
			}
		}
	}()
}
