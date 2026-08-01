// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// NOTE: see streamable_server.go and streamable_client.go for detailed
// documentation of the streamable server design.
// TODO: move the client and server logic into those files.

package mcp

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"math/rand/v2"
	"mime"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	internaljson "github.com/modelcontextprotocol/go-sdk/internal/json"
	"github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2"
	"github.com/modelcontextprotocol/go-sdk/internal/mcpgodebug"
	"github.com/modelcontextprotocol/go-sdk/internal/util"
	"github.com/modelcontextprotocol/go-sdk/internal/xcontext"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"golang.org/x/oauth2"
)

// A StreamableHTTPHandler is an http.Handler that serves streamable MCP
// sessions, as defined by the [MCP spec].
//
// [MCP spec]: https://modelcontextprotocol.io/2025/03/26/streamable-http-transport.html
type StreamableHTTPHandler struct {
	getServer func(*http.Request) *Server
	opts      StreamableHTTPOptions

	onTransportDeletion func(sessionID string) // for testing

	mu       sync.Mutex
	sessions map[string]*sessionInfo // keyed by session ID
}

type sessionInfo struct {
	session   *ServerSession
	transport *StreamableServerTransport
	// userID is the user ID from the TokenInfo when the session was created.
	// If non-empty, subsequent requests must have the same user ID to prevent
	// session hijacking.
	userID string

	// If timeout is set, automatically close the session after an idle period.
	timeout time.Duration
	timerMu sync.Mutex
	refs    int // reference count
	timer   *time.Timer
}

// startPOST signals that a POST request for this session is starting (which
// carries a client->server message), pausing the session timeout if it was
// running.
//
// TODO: we may want to also pause the timer when resuming non-standalone SSE
// streams, but that is tricy to implement. Clients should generally make
// keepalive pings if they want to keep the session live.
func (i *sessionInfo) startPOST() {
	if i.timeout <= 0 {
		return
	}

	i.timerMu.Lock()
	defer i.timerMu.Unlock()

	if i.timer == nil {
		return // timer stopped permanently
	}
	if i.refs == 0 {
		i.timer.Stop()
	}
	i.refs++
}

// endPOST signals that a request for this session is ending, starting the
// timeout if there are no other requests running.
func (i *sessionInfo) endPOST() {
	if i.timeout <= 0 {
		return
	}

	i.timerMu.Lock()
	defer i.timerMu.Unlock()

	if i.timer == nil {
		return // timer stopped permanently
	}

	i.refs--
	assert(i.refs >= 0, "negative ref count")
	if i.refs == 0 {
		i.timer.Reset(i.timeout)
	}
}

// stopTimer stops the inactivity timer permanently.
func (i *sessionInfo) stopTimer() {
	i.timerMu.Lock()
	defer i.timerMu.Unlock()
	if i.timer != nil {
		i.timer.Stop()
		i.timer = nil
	}
}

// StreamableHTTPOptions configures the StreamableHTTPHandler.
type StreamableHTTPOptions struct {
	// Stateless controls whether the session is 'stateless'.
	//
	// A stateless server does not read or set the Mcp-Session-Id header, and
	// uses a temporary session with default initialization parameters for each
	// request. [ServerOptions.GetSessionID] is not consulted. Any
	// server->client request is rejected immediately as there's no way for the
	// client to respond. Server->Client notifications may reach the client if
	// they are made in the context of an incoming request, as described in the
	// documentation for [StreamableServerTransport].
	// In Stateless mode, GET and DELETE requests return 405 Method Not Allowed.
	//
	// This mode aligns with the sessionless direction of the MCP spec; see
	// [SEP-2567]. The previous behavior, in which stateless servers still
	// honored Mcp-Session-Id, can be restored temporarily via the
	// MCPGODEBUG compatibility parameter "allowsessionsinstateless=1".
	//
	// [SEP-2567]: https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2567
	Stateless bool

	// TODO(#148): support session retention (?)

	// JSONResponse causes streamable responses to return application/json rather
	// than text/event-stream ([§2.1.5] of the spec).
	//
	// [§2.1.5]: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#sending-messages-to-the-server
	JSONResponse bool

	// Logger specifies the logger to use.
	// If nil, do not log.
	Logger *slog.Logger

	// EventStore enables stream resumption.
	//
	// If set, EventStore will be used to persist stream events and replay them
	// upon stream resumption.
	EventStore EventStore

	// SessionTimeout configures a timeout for idle sessions.
	//
	// When sessions receive no new HTTP requests from the client for this
	// duration, they are automatically closed.
	//
	// If SessionTimeout is the zero value, idle sessions are never closed.
	SessionTimeout time.Duration

	// DisableLocalhostProtection disables automatic DNS rebinding protection.
	// By default, requests arriving via a localhost address (127.0.0.1, [::1])
	// that have a non-localhost Host header are rejected with 403 Forbidden.
	// This protects against DNS rebinding attacks regardless of whether the
	// server is listening on localhost specifically or on 0.0.0.0.
	//
	// Only disable this if you understand the security implications.
	// See: https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices#local-mcp-server-compromise
	DisableLocalhostProtection bool

	// CrossOriginProtection allows to customize cross-origin protection.
	// The deny handler set in the CrossOriginProtection through SetDenyHandler
	// is ignored.
	// If nil, no cross-origin protection is applied. Use the `enableoriginverification`
	// MCPGODEBUG compatibility parameter to enable the default protection until v1.8.0.
	//
	// Deprecated: wrap the handler with cross-origin protection middleware
	// instead. For example:
	//
	//   handler := mcp.NewStreamableHTTPHandler(...)
	//   protection := http.NewCrossOriginProtection()
	//   protectedHandler := protection.Handler(handler)
	CrossOriginProtection *http.CrossOriginProtection

	// MaxRequestBodyBytes limits the number of bytes read from any incoming
	// HTTP request body. Requests that exceed this limit are rejected with
	// 413 Request Entity Too Large.
	//
	// The limit is enforced during the read, so it applies uniformly to
	// requests using Content-Length, Transfer-Encoding: chunked, or HTTP/2
	// (which has no Content-Length).
	//
	// If zero, [DefaultMaxRequestBodyBytes] is used.
	// A negative value disables the limit entirely; do not use this on
	// servers exposed to untrusted clients.
	MaxRequestBodyBytes int64

	// PropagateRequestCancellation, when true, ties the in-flight handler
	// context to the originating HTTP request's context. Only applies to
	// requests using the >= 2026-07-28 protocol, where the POST is the whole
	// request lifecycle.
	// The handler context cancels whenever the HTTP request context does as the
	// response can no longer be delivered, so cancelling the handler is safe.
	//
	// Requests using older protocol versions (including those routed through
	// the allowsessionsinstateless compatibility path) are unaffected.
	PropagateRequestCancellation bool
}

// DefaultMaxRequestBodyBytes is the default value used for
// [StreamableHTTPOptions.MaxRequestBodyBytes] when it is left at zero.
const DefaultMaxRequestBodyBytes = 4 << 20 // 4 MiB

// NewStreamableHTTPHandler returns a new [StreamableHTTPHandler].
//
// The getServer function is used to create or look up servers for new
// sessions. It is OK for getServer to return the same server multiple times.
// If getServer returns nil, a 400 Bad Request will be served.
func NewStreamableHTTPHandler(getServer func(*http.Request) *Server, opts *StreamableHTTPOptions) *StreamableHTTPHandler {
	h := &StreamableHTTPHandler{
		getServer: getServer,
		sessions:  make(map[string]*sessionInfo),
	}
	if opts != nil {
		h.opts = *opts
	}

	h.opts.Logger = ensureLogger(h.opts.Logger)

	if h.opts.CrossOriginProtection == nil && enableoriginverification == "1" {
		h.opts.CrossOriginProtection = &http.CrossOriginProtection{}
	}

	if h.opts.MaxRequestBodyBytes == 0 {
		h.opts.MaxRequestBodyBytes = DefaultMaxRequestBodyBytes
	}

	return h
}

// closeAll closes all ongoing sessions, for tests.
//
// TODO(rfindley): investigate the best API for callers to configure their
// session lifecycle. (?)
//
// Should we allow passing in a session store? That would allow the handler to
// be stateless.
func (h *StreamableHTTPHandler) closeAll() {
	// TODO: if we ever expose this outside of tests, we'll need to do better
	// than simply collecting sessions while holding the lock: we need to prevent
	// new sessions from being added.
	//
	// Currently, sessions remove themselves from h.sessions when closed, so we
	// can't call Close while holding the lock.
	h.mu.Lock()
	sessionInfos := slices.Collect(maps.Values(h.sessions))
	h.sessions = nil
	h.mu.Unlock()
	for _, s := range sessionInfos {
		s.session.Close()
	}
}

// disablelocalhostprotection is a compatibility parameter that allows to disable
// DNS rebinding protection, which was added in the 1.4.0 version of the SDK.
// See the documentation for the mcpgodebug package for instructions how to enable it.
// The option will be removed in the 1.8.0 version of the SDK.
var disablelocalhostprotection = mcpgodebug.Value("disablelocalhostprotection")

// enableoriginverification is a compatibility parameter that restores the
// default cross-origin protection behavior from v1.4.1-v1.5.0. When set to
// "1", a zero-value CrossOriginProtection will be applied if none is
// explicitly provided in StreamableHTTPOptions.
// See the documentation for the mcpgodebug package for instructions how to enable it.
// The option will be removed in the 1.8.0 version of the SDK.
var enableoriginverification = mcpgodebug.Value("enableoriginverification")

// allowsessionsinstateless is a compatibility parameter that restores the old
// behavior of reading and using Mcp-Session-Id headers in stateless mode. When
// set to "1", stateless servers will read the session ID from the request
// header (or generate one via GetSessionID), set it on response headers, and
// accept DELETE requests. When unset (the default), stateless servers ignore
// session IDs entirely and reject DELETE with 405.
// See the documentation for the mcpgodebug package for instructions how to enable it.
// The option will be removed in the 1.9.0 version of the SDK.
var allowsessionsinstateless = mcpgodebug.Value("allowsessionsinstateless")

// noprotocolerrorbody is a compatibility parameter that restores the previous
// behavior of [streamableClientConn.checkResponse]. When unset (the default),
// the client always attempts to surface the underlying JSON-RPC error.
var noprotocolerrorbody = mcpgodebug.Value("noprotocolerrorbody")

// disablecontenttypecheck is a compatibility parameter that allows to disable
// Content-Type validation on POST requests.
// See the documentation for the mcpgodebug package for instructions how to enable it.
// The option will be removed in the 1.8.0 version of the SDK.
var disablecontenttypecheck = mcpgodebug.Value("disablecontenttypecheck")

// writeJSONRPCError writes a JSON-RPC error response with the given HTTP
// status code, request ID (may be a zero ID for errors that occur before the
// request body has been parsed), and JSON-RPC error.
func writeJSONRPCError(w http.ResponseWriter, status int, id jsonrpc.ID, jerr *jsonrpc.Error) {
	resp := &jsonrpc.Response{ID: id, Error: jerr}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data, err := jsonrpc2.EncodeMessage(resp); err == nil {
		w.Write(data)
	}
}

func (h *StreamableHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// DNS rebinding protection: auto-enabled for localhost servers.
	// See: https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices#local-mcp-server-compromise
	if !h.opts.DisableLocalhostProtection && disablelocalhostprotection != "1" {
		if localAddr, ok := req.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && localAddr != nil {
			if util.IsLoopback(localAddr.String()) && !util.IsLoopback(req.Host) {
				http.Error(w, fmt.Sprintf("Forbidden: invalid Host header %q", req.Host), http.StatusForbidden)
				return
			}
		}
	}

	if h.opts.CrossOriginProtection != nil {
		if err := h.opts.CrossOriginProtection.Check(req); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	}

	// Bound the request body to protect against OOM attacks.
	if req.Body != nil && h.opts.MaxRequestBodyBytes > 0 {
		req.Body = http.MaxBytesReader(w, req.Body, h.opts.MaxRequestBodyBytes)
	}

	// [§2.7] of the spec (2025-06-18): validate the MCP-Protocol-Version
	// header. If provided, it must be a supported version. If absent, the
	// version is unknown (the request may be an initialize for any version).
	//
	// [§2.7]: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#protocol-version-header
	protocolVersion := req.Header.Get(protocolVersionHeader)
	if protocolVersion != "" && !slices.Contains(supportedProtocolVersions, protocolVersion) && protocolVersion < protocolVersion20260728 {
		http.Error(w, fmt.Sprintf("Bad Request: Unsupported protocol version (supported versions: %s)", strings.Join(supportedProtocolVersions, ",")), http.StatusBadRequest)
		return
	}
	req = req.WithContext(context.WithValue(req.Context(), protocolVersionContextKey{}, protocolVersion))

	if h.opts.Stateless {
		h.serveStateless(w, req)
	} else {
		h.serveStateful(w, req)
	}
}

// serveStateless handles requests for stateless servers.
// Stateless servers only support POST. Each request creates a temporary
// session that is closed when the request completes.
//
// When the allowsessionsinstateless compatibility flag is set, DELETE is also
// accepted (as a no-op) and session IDs are read from the request header.
func (h *StreamableHTTPHandler) serveStateless(w http.ResponseWriter, req *http.Request) {
	legacySessions := allowsessionsinstateless == "1"

	if req.Method == http.MethodDelete && legacySessions {
		h.serveStatelessLegacyDELETE(w, req)
		return
	}

	if req.Method != http.MethodPost {
		// RFC 9110 §15.5.6: 405 responses MUST include Allow header.
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if disablecontenttypecheck != "1" && baseMediaType(req.Header.Get("Content-Type")) != "application/json" {
		http.Error(w, "Content-Type must be 'application/json'", http.StatusUnsupportedMediaType)
		return
	}

	// Accept must contain both 'application/json' and 'text/event-stream'.
	jsonOK, streamOK := streamableAccepts(req.Header.Values("Accept"))
	if !jsonOK || !streamOK {
		http.Error(w, "Accept must contain both 'application/json' and 'text/event-stream'", http.StatusBadRequest)
		return
	}

	server := h.getServer(req)
	if server == nil {
		http.Error(w, "no server available", http.StatusBadRequest)
		return
	}

	info, err := h.ephemeralConnectOpts(req)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, fmt.Sprintf("request body exceeds %d bytes", mbe.Limit), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var sessionID string
	if legacySessions && !info.usesNewProtocol {
		sessionID = req.Header.Get(sessionIDHeader)
		if sessionID == "" {
			sessionID = server.opts.GetSessionID()
		}
	}

	transport := &StreamableServerTransport{
		SessionID:                   sessionID,
		Stateless:                   true,
		EventStore:                  h.opts.EventStore,
		jsonResponse:                h.opts.JSONResponse,
		logger:                      h.opts.Logger,
		shouldPropagateCancellation: info.usesNewProtocol && (info.isSubscriptionsListen || h.opts.PropagateRequestCancellation),
	}

	session, err := connectStreamable(req.Context(), server, transport, info.opts)
	if err != nil {
		h.opts.Logger.Error(fmt.Sprintf("failed to connect: %v", err))
		http.Error(w, "failed connection", http.StatusInternalServerError)
		return
	}
	defer session.Close()

	transport.ServeHTTP(w, req)
}

// serveStatelessLegacyDELETE handles DELETE requests in stateless mode when the
// allowsessionsinstateless compatibility flag is set. DELETE requires a
// Mcp-Session-Id header but is otherwise a no-op since stateless servers don't
// persist sessions.
func (h *StreamableHTTPHandler) serveStatelessLegacyDELETE(w http.ResponseWriter, req *http.Request) {
	sessionID := req.Header.Get(sessionIDHeader)
	if sessionID == "" {
		http.Error(w, "Bad Request: DELETE requires an Mcp-Session-Id header", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ephemeralConnectInfo struct {
	opts                  *ServerSessionOptions
	usesNewProtocol       bool
	isSubscriptionsListen bool
}

// ephemeralConnectOpts peeks at the request body to determine connection
// parameters and whether protocol version >= 2026-06-30 (SEP-2575).
//
// For old-protocol requests, default session state is synthesized so that
// the session's init gate doesn't reject the request.
//
// It is used for both stateless servers and stateful servers with no session ID.
func (h *StreamableHTTPHandler) ephemeralConnectOpts(req *http.Request) (*ephemeralConnectInfo, error) {
	protocolVersion := protocolVersionFromContext(req.Context())
	if protocolVersion == "" {
		protocolVersion = protocolVersion20250326
	}

	var hasInitialize, hasInitialized, usesNewProtocol, isSubscriptionsListen bool
	body, err := io.ReadAll(req.Body)
	if err != nil {
		// Preserve *http.MaxBytesError so serveStateless can respond with 413.
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewBuffer(body))
	msgs, _, err := readBatch(body)
	if err == nil {
		for _, msg := range msgs {
			if r, ok := msg.(*jsonrpc.Request); ok {
				switch r.Method {
				case methodInitialize:
					hasInitialize = true
				case notificationInitialized:
					hasInitialized = true
				case methodSubscriptionsListen:
					isSubscriptionsListen = true
				}
				if protocolVersion >= protocolVersion20260728 {
					usesNewProtocol = true
				}
			}
		}
	}

	state := new(ServerSessionState)
	// Only synthesize fake InitializeParams/InitializedParams for old-protocol
	// requests.
	if !hasInitialize && !usesNewProtocol {
		state.InitializeParams = &InitializeParams{
			ProtocolVersion: protocolVersion,
		}
	}
	if !hasInitialized && !usesNewProtocol {
		state.InitializedParams = new(InitializedParams)
	}
	if !usesNewProtocol {
		state.LogLevel = "info"
	}
	return &ephemeralConnectInfo{
		opts: &ServerSessionOptions{
			State: state,
		},
		usesNewProtocol:       usesNewProtocol,
		isSubscriptionsListen: isSubscriptionsListen,
	}, nil
}

func connectStreamable(ctx context.Context, server *Server, transport *StreamableServerTransport, opts *ServerSessionOptions) (*ServerSession, error) {
	s, err := server.Connect(ctx, transport, opts)
	if err != nil {
		return nil, err
	}
	transport.connection.server = server
	transport.connection.toolLookup = server.getServerTool
	return s, nil
}

// serveStateful handles requests for stateful servers.
// Stateful servers support GET, POST, and DELETE, and maintain persistent
// sessions keyed by session ID.
func (h *StreamableHTTPHandler) serveStateful(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		h.serveStatefulGET(w, req)
	case http.MethodPost:
		h.serveStatefulPOST(w, req)
	case http.MethodDelete:
		h.serveStatefulDELETE(w, req)
	default:
		// RFC 9110 §15.5.6: 405 responses MUST include Allow header.
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// lookupSession looks up a session by the Mcp-Session-Id header value.
// It returns the session info and whether the caller should proceed. If ok is
// false, an error response has been written. The sessionID must be non-empty;
// callers are responsible for checking this before calling lookupSession.
func (h *StreamableHTTPHandler) lookupSession(w http.ResponseWriter, req *http.Request, sessionID string) (info *sessionInfo, ok bool) {
	h.mu.Lock()
	info = h.sessions[sessionID]
	h.mu.Unlock()
	if info == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return nil, false
	}
	if info.userID != "" {
		tokenInfo := auth.TokenInfoFromContext(req.Context())
		if tokenInfo == nil || tokenInfo.UserID != info.userID {
			http.Error(w, "session user mismatch", http.StatusForbidden)
			return nil, false
		}
	}
	return info, true
}

// serveStatefulGET handles GET requests for standalone SSE streams.
// GET requires a valid Mcp-Session-Id header.
func (h *StreamableHTTPHandler) serveStatefulGET(w http.ResponseWriter, req *http.Request) {
	if _, streamOK := streamableAccepts(req.Header.Values("Accept")); !streamOK {
		http.Error(w, "Accept must contain 'text/event-stream' for GET requests", http.StatusBadRequest)
		return
	}

	sessionID := req.Header.Get(sessionIDHeader)
	if sessionID == "" {
		http.Error(w, "Bad Request: GET requires an Mcp-Session-Id header", http.StatusBadRequest)
		return
	}

	sessInfo, ok := h.lookupSession(w, req, sessionID)
	if !ok {
		return
	}

	sessInfo.transport.ServeHTTP(w, req)
}

// serveStatefulDELETE handles DELETE requests for session termination.
// DELETE requires a valid Mcp-Session-Id header.
func (h *StreamableHTTPHandler) serveStatefulDELETE(w http.ResponseWriter, req *http.Request) {
	sessionID := req.Header.Get(sessionIDHeader)
	if sessionID == "" {
		http.Error(w, "Bad Request: DELETE requires an Mcp-Session-Id header", http.StatusBadRequest)
		return
	}

	sessInfo, ok := h.lookupSession(w, req, sessionID)
	if !ok {
		return
	}

	sessInfo.session.Close()
	w.WriteHeader(http.StatusNoContent)
}

// serveStatefulPOST handles POST requests for stateful servers.
// POST may arrive with or without a Mcp-Session-Id header. Without a session
// ID, a new session is created (this is the normal path for the first
// initialize request).
func (h *StreamableHTTPHandler) serveStatefulPOST(w http.ResponseWriter, req *http.Request) {
	if disablecontenttypecheck != "1" && baseMediaType(req.Header.Get("Content-Type")) != "application/json" {
		http.Error(w, "Content-Type must be 'application/json'", http.StatusUnsupportedMediaType)
		return
	}

	jsonOK, streamOK := streamableAccepts(req.Header.Values("Accept"))
	if !jsonOK || !streamOK {
		http.Error(w, "Accept must contain both 'application/json' and 'text/event-stream'", http.StatusBadRequest)
		return
	}

	sessionID := req.Header.Get(sessionIDHeader)

	// Look up existing session if a session ID was provided.
	if sessionID != "" {
		sessInfo, ok := h.lookupSession(w, req, sessionID)
		if !ok {
			return
		}
		sessInfo.startPOST()
		defer sessInfo.endPOST()
		sessInfo.transport.ServeHTTP(w, req)
		return
	}

	// No session ID: create a new session.
	server := h.getServer(req)
	if server == nil {
		http.Error(w, "no server available", http.StatusBadRequest)
		return
	}
	sessionID = server.opts.GetSessionID()

	transport := &StreamableServerTransport{
		SessionID:    sessionID,
		Stateless:    false,
		EventStore:   h.opts.EventStore,
		jsonResponse: h.opts.JSONResponse,
		logger:       h.opts.Logger,
	}

	// Sessions without a session ID (GetSessionID returned "") are ephemeral:
	// there's no way to address them, so they are closed after the request.
	// This can happen when ServerOptions.GetSessionID is explicitly set to
	// return "" to suppress session IDs entirely. It also covers any request
	// that arrives before a session exists (e.g. initialize or ping) on a
	// server configured this way.
	if sessionID == "" {
		info, err := h.ephemeralConnectOpts(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		session, err := connectStreamable(req.Context(), server, transport, info.opts)
		if err != nil {
			h.opts.Logger.Error(fmt.Sprintf("failed to connect: %v", err))
			http.Error(w, "failed connection", http.StatusInternalServerError)
			return
		}
		defer session.Close()
		transport.ServeHTTP(w, req)
		return
	}

	connectOpts := &ServerSessionOptions{
		onClose: func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if info, ok := h.sessions[transport.SessionID]; ok {
				info.stopTimer()
				delete(h.sessions, transport.SessionID)
				if h.onTransportDeletion != nil {
					h.onTransportDeletion(transport.SessionID)
				}
			}
		},
	}

	// Pass req.Context() here, to allow middleware to add context values.
	// The context is detached in the jsonrpc2 library when handling the
	// long-running stream.
	session, err := connectStreamable(req.Context(), server, transport, connectOpts)
	if err != nil {
		h.opts.Logger.Error(fmt.Sprintf("failed to connect: %v", err))
		http.Error(w, "failed connection", http.StatusInternalServerError)
		return
	}
	// Capture the user ID from the token info to enable session hijacking
	// prevention on subsequent requests.
	var userID string
	if tokenInfo := auth.TokenInfoFromContext(req.Context()); tokenInfo != nil {
		userID = tokenInfo.UserID
	}
	sessInfo := &sessionInfo{
		session:   session,
		transport: transport,
		userID:    userID,
	}

	if h.opts.SessionTimeout > 0 {
		sessInfo.timeout = h.opts.SessionTimeout
		sessInfo.timer = time.AfterFunc(sessInfo.timeout, func() {
			sessInfo.session.Close()
		})
	}
	h.mu.Lock()
	h.sessions[transport.SessionID] = sessInfo
	h.mu.Unlock()
	defer func() {
		// If initialization failed, clean up the session (#578).
		if session.InitializeParams() == nil {
			session.Close()
		}
	}()

	sessInfo.startPOST()
	defer sessInfo.endPOST()
	sessInfo.transport.ServeHTTP(w, req)
}

func streamableAccepts(values []string) (jsonOK, streamOK bool) {
	for _, value := range values {
		for _, raw := range strings.Split(value, ",") {
			token := strings.TrimSpace(raw)
			// Ignore Accept parameters like ";charset=utf-8"; match the base media type.
			base, _, _ := strings.Cut(token, ";")
			switch strings.ToLower(strings.TrimSpace(base)) {
			case "application/json", "application/*":
				jsonOK = true
			case "text/event-stream", "text/*":
				streamOK = true
			case "*/*":
				jsonOK = true
				streamOK = true
			}
		}
	}
	return jsonOK, streamOK
}

func baseMediaType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return mediaType
}

// A StreamableServerTransport implements the server side of the MCP streamable
// transport.
//
// Each StreamableServerTransport must be connected (via [Server.Connect]) at
// most once, since [StreamableServerTransport.ServeHTTP] serves messages to
// the connected session.
//
// Reads from the streamable server connection receive messages from http POST
// requests from the client. Writes to the streamable server connection are
// sent either to the related stream, or to the standalone SSE stream,
// according to the following rules:
//   - JSON-RPC responses to incoming requests are always routed to the
//     appropriate HTTP response.
//   - Requests or notifications made with a context.Context value derived from
//     an incoming request handler, are routed to the HTTP response
//     corresponding to that request, unless it has already terminated, in
//     which case they are routed to the standalone SSE stream.
//   - Requests or notifications made with a detached context.Context value are
//     routed to the standalone SSE stream.
type StreamableServerTransport struct {
	// SessionID is the ID of this session.
	//
	// If SessionID is the empty string, this is a 'stateless' session, which has
	// limited ability to communicate with the client. Otherwise, the session ID
	// must be globally unique, that is, different from any other session ID
	// anywhere, past and future. (We recommend using a crypto random number
	// generator to produce one, as with [crypto/rand.Text].)
	SessionID string

	// Stateless controls whether the eventstore is 'Stateless'. Server sessions
	// connected to a stateless transport are disallowed from making outgoing
	// requests.
	//
	// See also [StreamableHTTPOptions.Stateless].
	Stateless bool

	// EventStore enables stream resumption.
	//
	// If set, EventStore will be used to persist stream events and replay them
	// upon stream resumption.
	EventStore EventStore

	// jsonResponse, if set, tells the server to prefer to respond to requests
	// using application/json responses rather than text/event-stream.
	//
	// Specifically, responses will be application/json whenever incoming POST
	// request contain only a single message. In this case, notifications or
	// requests made within the context of a server request will be sent to the
	// standalone SSE stream, if any.
	//
	// TODO(rfindley): jsonResponse should be exported, since
	// StreamableHTTPOptions.JSONResponse is exported, and we want to allow users
	// to write their own streamable HTTP handler.
	jsonResponse bool

	// optional logger provided through the [StreamableHTTPOptions.Logger].
	//
	// TODO(rfindley): logger should be exported, since we want to allow users
	// to write their own streamable HTTP handler.
	logger *slog.Logger

	// shouldPropagateCancellation is forwarded to the underlying
	// [streamableServerConn]. See its docstring.
	shouldPropagateCancellation bool

	// connection is non-nil if and only if the transport has been connected.
	connection *streamableServerConn
}

// Connect implements the [Transport] interface.
func (t *StreamableServerTransport) Connect(ctx context.Context) (Connection, error) {
	if t.connection != nil {
		return nil, fmt.Errorf("transport already connected")
	}
	t.connection = &streamableServerConn{
		sessionID:                   t.SessionID,
		stateless:                   t.Stateless,
		eventStore:                  t.EventStore,
		jsonResponse:                t.jsonResponse,
		logger:                      ensureLogger(t.logger), // see #556: must be non-nil
		shouldPropagateCancellation: t.shouldPropagateCancellation,
		incoming:                    make(chan jsonrpc.Message, 10),
		done:                        make(chan struct{}),
		streams:                     make(map[string]*stream),
		requestStreams:              make(map[jsonrpc.ID]string),
	}
	// Stream 0 corresponds to the standalone SSE stream.
	//
	// It is always text/event-stream, since it must carry arbitrarily many
	// messages.
	var err error
	t.connection.streams[""], err = t.connection.newStream(ctx, nil, "")
	if err != nil {
		return nil, err
	}
	return t.connection, nil
}

// The streamable HTTP transport supports every legacy SDK protocol version,
// but the SEP-2575 >= 2026-07-28 protocol is only supported when the
// transport is configured as stateless.
func (t *StreamableServerTransport) SupportsProtocolVersion(version string) bool {
	if version >= protocolVersion20260728 {
		return t.Stateless && slices.Contains(supportedProtocolVersions, version)
	}
	return slices.Contains(supportedProtocolVersions, version)
}

type streamableServerConn struct {
	sessionID    string
	stateless    bool
	jsonResponse bool
	eventStore   EventStore

	// shouldPropagateCancellation is true when the underlying HTTP request's
	// lifetime IS the connection's cancellation signal (e.g., a stateless
	// POST that owns a long-lived subscriptions/listen stream). It is read
	// by the [cancellationPropagator] interface so the jsonrpc2 layer wires
	// handler contexts to observe the carrier's cancellation.
	shouldPropagateCancellation bool

	logger *slog.Logger

	server     *Server
	toolLookup func(name string) (*serverTool, bool)

	incoming chan jsonrpc.Message // messages from the client to the server

	mu sync.Mutex // guards all fields below

	// Sessions are closed exactly once.
	isDone bool
	done   chan struct{}

	// Sessions can have multiple logical connections (which we call streams),
	// corresponding to HTTP requests. Additionally, streams may be resumed by
	// subsequent HTTP requests, when the HTTP connection is terminated
	// unexpectedly.
	//
	// Therefore, we use a logical stream ID to key the stream state, and
	// perform the accounting described below when incoming HTTP requests are
	// handled.

	// streams holds the logical streams for this session, keyed by their ID.
	//
	// Lifecycle: streams persist until all of their responses are received from
	// the server.
	streams map[string]*stream

	// requestStreams maps incoming requests to their logical stream ID.
	//
	// Lifecycle: requestStreams persist until their response is received.
	requestStreams map[jsonrpc.ID]string
}

func (c *streamableServerConn) SessionID() string {
	return c.sessionID
}

// propagateCancellation implements [cancellationPropagator]. It returns true
// when this connection is bound to a single HTTP request whose lifetime
// should drive request-handler cancellation — for example, a stateless POST
// carrying a long-lived subscriptions/listen stream that must unwind when
// the client TCP-disconnects.
func (c *streamableServerConn) propagateCancellation() bool {
	return c.shouldPropagateCancellation
}

// A stream is a single logical stream of SSE events within a server session.
// A stream begins with a client request, or with a client GET that has
// no Last-Event-ID header.
//
// A stream ends only when its session ends; we cannot determine its end otherwise,
// since a client may send a GET with a Last-Event-ID that references the stream
// at any time.
type stream struct {
	// id is the logical ID for the stream, unique within a session.
	//
	// The standalone SSE stream has id "".
	id string

	// logger is used for logging errors during stream operations.
	logger *slog.Logger

	// mu guards the fields below, as well as storage of new messages in the
	// connection's event store (if any).
	mu sync.Mutex

	// If pendingJSONMessages is non-nil, this is a JSON stream and messages are
	// collected here until the stream is complete, at which point they are
	// flushed as a single JSON response. Note that the non-nilness of this field
	// is significant, as it signals the expected content type.
	//
	// Note: if we remove support for batching, this could just be a bool.
	pendingJSONMessages []json.RawMessage

	// w is the HTTP response writer for this stream. A non-nil w indicates
	// that the stream is claimed by an HTTP request (the hanging POST or GET);
	// it is set to nil when the request completes.
	w http.ResponseWriter

	// done is closed to release the hanging HTTP request.
	//
	// Invariant: a non-nil done implies w is also non-nil, though the converse
	// is not necessarily true: done is set to nil when it is closed, to avoid
	// duplicate closure.
	done chan struct{}

	// lastIdx is the index of the last written SSE event, for event ID generation.
	// It starts at -1 since indices start at 0.
	lastIdx int

	// protocolVersion is the protocol version for this stream.
	protocolVersion string

	// requests is the set of unanswered incoming requests for the stream.
	//
	// Requests are removed when their response has been received.
	// In practice, there is only one request, but in the 2025-03-26 version of
	// the spec and earlier there was a concept of batching, in which POST
	// payloads could hold multiple requests or responses.
	requests map[jsonrpc.ID]struct{}

	// isListen reports whether this stream was opened by a
	// subscriptions/listen request. Listen streams are always SSE, live for
	// the duration of the subscription, and act as the target for
	// out-of-band notifications routed through this connection.
	isListen bool
}

// close sends a 'close' event to the client (if protocolVersion >= 2025-11-25
// and reconnectAfter > 0) and closes the done channel.
//
// The done channel is set to nil after closing, so that done != nil implies
// the stream is active and done is open. This simplifies checks elsewhere.
func (s *stream) close(reconnectAfter time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done == nil {
		return // stream not connected or already closed
	}
	if s.protocolVersion >= protocolVersion20251125 && reconnectAfter > 0 {
		reconnectStr := strconv.FormatInt(reconnectAfter.Milliseconds(), 10)
		if _, err := writeEvent(s.w, Event{
			Name:  "close",
			Retry: reconnectStr,
		}); err != nil {
			s.logger.Warn(fmt.Sprintf("Writing close event: %v", err))
		}
	}
	close(s.done)
	s.done = nil
}

// release releases the stream from its HTTP request, allowing it to be
// claimed by another request (e.g., for resumption).
func (s *stream) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.w = nil
	s.done = nil // may already be nil, if the stream is done or closed
}

// extractErrorStatus reports the HTTP status to send when the given
// outgoing message is a JSON-RPC error response under the SEP-2575 protocol
// (>= 2026-07-28).
//
// Per SEP-2575:
//   - MethodNotFound (-32601) MUST return HTTP 404.
//   - InvalidParams (-32602), UnsupportedProtocolVersion (-32022) and
//     CodeMissingRequiredClientCapabilities (-32021) MUST
//     return HTTP 400.
func extractErrorStatus(ctx context.Context, msg jsonrpc.Message) int {
	if protocolVersionFromContext(ctx) < protocolVersion20260728 {
		return 0
	}
	resp, ok := msg.(*jsonrpc.Response)
	if !ok || resp.Error == nil {
		return 0
	}
	var jerr *jsonrpc.Error
	if !errors.As(resp.Error, &jerr) {
		return 0
	}
	switch jerr.Code {
	case jsonrpc.CodeMethodNotFound:
		return http.StatusNotFound
	case jsonrpc.CodeInvalidParams, CodeUnsupportedProtocolVersion, CodeMissingRequiredClientCapabilities:
		return http.StatusBadRequest
	}
	return 0
}

// deliverLocked writes data to the stream (for SSE) or stores it in
// pendingJSONMessages (for JSON mode). The eventID is used for SSE event ID;
// pass "" to omit.
//
// If responseTo is valid, it is removed from the requests map. When all
// requests have been responded to, the done channel is closed and set to nil.
//
// If overrideStatus is non-zero, data is treated as a SEP-2575 protocol-level
// error response (>= 2026-07-28): it is written as a single raw JSON-RPC
// response body with Content-Type: application/json and HTTP status
// overrideStatus.
//
// Returns true if the stream is now done (all requests have been responded to).
// The done value is always accurate, even if an error is returned.
//
// s.mu must be held when calling this method.
func (s *stream) deliverLocked(data []byte, eventID string, responseTo jsonrpc.ID, overrideStatus int) (done bool, err error) {
	// First, record the response. We must do this *before* returning an error
	// below, as even if the stream is disconnected we want to update our
	// accounting.
	if responseTo.IsValid() {
		delete(s.requests, responseTo)
	}
	// Now, try to deliver the message to the client.
	done = len(s.requests) == 0 && s.id != ""
	if s.done == nil {
		return done, fmt.Errorf("stream not connected or already closed")
	}
	if done {
		defer func() { close(s.done); s.done = nil }()
	}
	// SEP-2575 protocol-level error override: write the error as a raw
	// JSON-RPC response with the spec-mandated HTTP status, bypassing any
	// SSE framing.
	if overrideStatus != 0 {
		s.w.Header().Set("Content-Type", "application/json")
		s.w.WriteHeader(overrideStatus)
		if _, err := s.w.Write(data); err != nil {
			return done, err
		}
		return done, nil
	}
	// Try to write to the response.
	//
	// If we get here, the request is still hanging (because s.done != nil
	// implies s.w != nil), but may have been cancelled by the client/http layer:
	// there's a brief race between request cancellation and releasing the
	// stream.
	if s.pendingJSONMessages != nil {
		s.pendingJSONMessages = append(s.pendingJSONMessages, data)
		if done {
			// Flush all pending messages as JSON response.
			var toWrite []byte
			if len(s.pendingJSONMessages) == 1 {
				toWrite = s.pendingJSONMessages[0]
			} else {
				toWrite, err = json.Marshal(s.pendingJSONMessages)
				if err != nil {
					return done, err
				}
			}
			if _, err := s.w.Write(toWrite); err != nil {
				return done, err
			}
		}
	} else {
		// SSE mode: write event to response writer.
		s.lastIdx++
		if _, err := writeEvent(s.w, Event{Name: "message", Data: data, ID: eventID}); err != nil {
			return done, err
		}
	}
	return done, nil
}

// doneLocked reports whether the stream is logically complete.
//
// s.requests was populated when reading the POST body, requests are deleted as
// they are responded to. Once all requests have been responded to, the stream
// is done.
//
// s.mu must be held while calling this function.
func (s *stream) doneLocked() bool {
	return len(s.requests) == 0 && s.id != ""
}

func (c *streamableServerConn) newStream(ctx context.Context, requests map[jsonrpc.ID]struct{}, id string) (*stream, error) {
	if c.eventStore != nil && protocolVersionFromContext(ctx) < protocolVersion20260728 {
		if err := c.eventStore.Open(ctx, c.sessionID, id); err != nil {
			return nil, err
		}
	}
	return &stream{
		id:       id,
		requests: requests,
		lastIdx:  -1, // indices start at 0, incremented before each write
		logger:   c.logger,
	}, nil
}

// We track the incoming request ID inside the handler context using
// idContextValue, so that notifications and server->client calls that occur in
// the course of handling incoming requests are correlated with the incoming
// request that caused them, and can be dispatched as server-sent events to the
// correct HTTP request.
//
// Currently, this is implemented in [ServerSession.handle]. This is not ideal,
// because it means that a user of the MCP package couldn't implement the
// streamable transport, as they'd lack this privileged access.
//
// If we ever wanted to expose this mechanism, we have a few options:
//  1. Make ServerSession an interface, and provide an implementation of
//     ServerSession to handlers that closes over the incoming request ID.
//  2. Expose a 'HandlerTransport' interface that allows transports to provide
//     a handler middleware, so that we don't hard-code this behavior in
//     ServerSession.handle.
//  3. Add a `func ForRequest(context.Context) jsonrpc.ID` accessor that lets
//     any transport access the incoming request ID.
//
// For now, by giving only the StreamableServerTransport access to the request
// ID, we avoid having to make this API decision.
type idContextKey struct{}

// protocolVersionContextKey stores the protocol version extracted from the
// MCP-Protocol-Version HTTP header for use by lower layers.
type protocolVersionContextKey struct{}

// protocolVersionFromContext returns the protocol version from the context, or
// the empty string if not set. An empty string means the version is unknown
// (e.g. the header was absent).
func protocolVersionFromContext(ctx context.Context) string {
	v, _ := ctx.Value(protocolVersionContextKey{}).(string)
	return v
}

// ServeHTTP handles a single HTTP request for the session.
func (t *StreamableServerTransport) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if t.connection == nil {
		http.Error(w, "transport not connected", http.StatusInternalServerError)
		return
	}
	switch req.Method {
	case http.MethodGet:
		t.connection.serveGET(w, req)
	case http.MethodPost:
		t.connection.servePOST(w, req)
	default:
		// Should not be reached, as this is checked in StreamableHTTPHandler.ServeHTTP.
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		return
	}
}

// serveGET streams messages to a hanging http GET, with stream ID and last
// message parsed from the Last-Event-ID header.
//
// It returns an HTTP status code and error message.
func (c *streamableServerConn) serveGET(w http.ResponseWriter, req *http.Request) {
	// streamID "" corresponds to the default GET request.
	streamID := ""
	// By default, we haven't seen a last index. Since indices start at 0, we represent
	// that by -1. This is incremented just before each event is written.
	lastIdx := -1
	if len(req.Header.Values(lastEventIDHeader)) > 0 {
		eid := req.Header.Get(lastEventIDHeader)
		var ok bool
		streamID, lastIdx, ok = parseEventID(eid)
		if !ok {
			http.Error(w, fmt.Sprintf("malformed Last-Event-ID %q", eid), http.StatusBadRequest)
			return
		}
		if c.eventStore == nil {
			http.Error(w, "stream replay unsupported", http.StatusBadRequest)
			return
		}
	}

	ctx := req.Context()

	protocolVersion := protocolVersionFromContext(ctx)
	if protocolVersion == "" {
		protocolVersion = protocolVersion20250326
	}

	stream, done := c.acquireStream(ctx, w, streamID, lastIdx, protocolVersion)
	if stream == nil {
		return
	}
	defer stream.release()
	c.hangResponse(ctx, done)
}

// hangResponse blocks the HTTP response until one of three conditions is met:
//   - ctx is cancelled (the client disconnected or the request timed out)
//   - done is closed (all responses have been sent, or the stream was explicitly closed)
//   - the session is closed
//
// This keeps the HTTP connection open so that server-sent events can be
// written to the response.
func (c *streamableServerConn) hangResponse(ctx context.Context, done <-chan struct{}) {
	select {
	case <-ctx.Done():
	case <-done:
	case <-c.done:
	}
}

// acquireStream replays all events since lastIdx, and acquires the ongoing
// stream, if any. If non-nil, the resulting stream will be registered for
// receiving new messages, and the stream's done channel will be closed when
// all related messages have been delivered.
//
// If any errors occur, they will be written to w and the resulting stream will
// be nil. The resulting stream may also be nil if the stream is complete.
//
// Importantly, this function must hold the stream mutex until done replaying
// all messages, so that no delivery or storage of new messages occurs while
// the stream is still replaying.
//
// protocolVersion is the protocol version for this stream, used to determine
// feature support (e.g. prime and close events were added in 2025-11-25).
func (c *streamableServerConn) acquireStream(ctx context.Context, w http.ResponseWriter, streamID string, lastIdx int, protocolVersion string) (*stream, chan struct{}) {
	// if tempStream is set, the stream is done and we're just replaying messages.
	//
	// We record a temporary stream to claim exclusive replay rights. The spec
	// (https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#resumability-and-redelivery)
	// does not explicitly require exclusive replay, but we enforce it defensively.
	tempStream := false
	c.mu.Lock()
	s, ok := c.streams[streamID]
	if !ok {
		// The stream is logically done, but claim exclusive rights to replay it by
		// adding a temporary entry in the streams map.
		//
		// We create this entry with a non-nil w, to ensure it isn't claimed by
		// another request before we lock it below.
		tempStream = true
		s = &stream{
			id: streamID,
			w:  w,
		}
		c.streams[streamID] = s

		// Since this stream is transient, we must clean up after replaying.
		defer func() {
			c.mu.Lock()
			delete(c.streams, streamID)
			c.mu.Unlock()
		}()
	}
	c.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check that this stream wasn't claimed by another request.
	if !tempStream && s.w != nil {
		http.Error(w, "stream ID conflicts with ongoing stream", http.StatusConflict)
		return nil, nil
	}

	// Collect events to replay. Collect them all before writing, so that we
	// have an opportunity to set the HTTP status code on an error.
	//
	// As indicated above, we must do that while holding stream.mu, so that no
	// new messages are added to the eventstore until we've replayed all previous
	// messages, and registered our delivery function.
	var toReplay [][]byte
	if c.eventStore != nil {
		for data, err := range c.eventStore.After(ctx, c.SessionID(), s.id, lastIdx) {
			if err != nil {
				// We can't replay events, perhaps because the underlying event store
				// has garbage collected its storage.
				//
				// We must be careful here: any 404 will signal to the client that the
				// *session* is not found, rather than the stream.
				//
				// 400 is not really accurate, but should at least have no side effects.
				// Other SDKs (typescript) do not have a mechanism for events to be purged.
				http.Error(w, "failed to replay events", http.StatusBadRequest)
				return nil, nil
			}
			if len(data) > 0 {
				toReplay = append(toReplay, data)
			}
		}
	}

	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Content-Type", "text/event-stream") // Accept checked in [StreamableHTTPHandler]
	w.Header().Set("Connection", "keep-alive")

	if s.id == "" {
		// Issue #410: the standalone SSE stream is likely not to receive messages
		// for a long time. Ensure that headers are flushed.
		//
		// On HTTP/2, headers and body travel as separate frames (HEADERS and
		// DATA). Reverse proxies (e.g. Envoy, Caddy, net/http/httputil)
		// commonly buffer the HEADERS frame until they have a DATA frame to
		// coalesce it with — there is no HTTP/2 equivalent of HTTP/1.1's
		// Transfer-Encoding: chunked signal that says "this is streaming, send
		// headers now". Calling Flush() alone is not sufficient: it pushes
		// the kernel buffer to the proxy, but the proxy still holds the
		// HEADERS frame.
		//
		// Write an SSE comment (lines starting with ":" are ignored by
		// clients per RFC) so a DATA frame is produced, which forces the
		// proxy to forward both frames. See:
		//   https://github.com/golang/go/issues/31125
		//   https://github.com/caddyserver/caddy/issues/4247
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ": ok\n\n")
		rc := http.NewResponseController(w)
		// Ignore returned error as flushing is best-effort.
		_ = rc.Flush()
	}

	for _, data := range toReplay {
		lastIdx++
		e := Event{Name: "message", Data: data}
		if c.eventStore != nil {
			e.ID = formatEventID(s.id, lastIdx)
		}
		if _, err := writeEvent(w, e); err != nil {
			return nil, nil
		}
	}

	if tempStream || s.doneLocked() {
		// Nothing more to do.
		return nil, nil
	}

	// The stream is not done: set up delivery state before the stream is
	// unlocked, allowing the connection to write new events.
	s.w = w
	s.done = make(chan struct{})
	s.lastIdx = lastIdx
	s.protocolVersion = protocolVersion
	return s, s.done
}

// servePOST handles an incoming message, and replies with either an outgoing
// message stream or single response object, depending on whether the
// jsonResponse option is set.
//
// It returns an HTTP status code and error message.
func (c *streamableServerConn) servePOST(w http.ResponseWriter, req *http.Request) {
	if len(req.Header.Values(lastEventIDHeader)) > 0 {
		http.Error(w, "can't send Last-Event-ID for POST request", http.StatusBadRequest)
		return
	}

	// Read incoming messages.
	body, err := io.ReadAll(req.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, fmt.Sprintf("request body exceeds %d bytes", mbe.Limit), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "POST requires a non-empty body", http.StatusBadRequest)
		return
	}
	// TODO(#674): once we've documented the support matrix for 2025-03-26 and
	// earlier, drop support for matching entirely; that will simplify this
	// logic.
	incoming, isBatch, err := readBatch(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("malformed payload: %v", err), http.StatusBadRequest)
		return
	}

	protocolVersion := protocolVersionFromContext(req.Context())
	if protocolVersion == "" {
		protocolVersion = protocolVersion20250326
	}

	if isBatch && protocolVersion >= protocolVersion20250618 {
		http.Error(w, fmt.Sprintf("JSON-RPC batching is not supported in %s and later (request version: %s)", protocolVersion20250618, protocolVersion), http.StatusBadRequest)
		return
	}

	// TODO(rfindley): no tests fail if we reject batch JSON requests entirely.
	// We need to test this with older protocol versions.
	// if isBatch && c.jsonResponse {
	// 	http.Error(w, "server does not support batch requests", http.StatusBadRequest)
	// 	return
	// }

	calls := make(map[jsonrpc.ID]struct{})
	tokenInfo := auth.TokenInfoFromContext(req.Context())
	isInitialize := false
	isSubscriptionsListen := false
	var initializeProtocolVersion string
	for _, msg := range incoming {
		if jreq, ok := msg.(*jsonrpc.Request); ok {
			// Preemptively check that this is a valid request, so that we can fail
			// the HTTP request. If we didn't do this, a request with a bad method or
			// missing ID could be silently swallowed.
			// Use the server's receiving method infos (which include any custom
			// methods registered via AddReceivingCustomMethod) when available;
			// fall back to the standard methods otherwise, e.g. in tests that
			// exercise streamableServerConn directly without a server.
			methodInfos := serverMethodInfos
			if c.server != nil {
				methodInfos = c.server.receivingMethodInfos()
			}
			if _, err := checkRequest(jreq, methodInfos); err != nil {
				if protocolVersion >= protocolVersion20260728 && errors.Is(err, jsonrpc2.ErrNotHandled) && jreq.IsCall() {
					writeJSONRPCError(w, http.StatusNotFound, jreq.ID, &jsonrpc.Error{
						Code:    jsonrpc.CodeMethodNotFound,
						Message: err.Error(),
					})
					return
				}
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if jreq.Method == methodInitialize {
				isInitialize = true
				// Extract the protocol version from InitializeParams.
				var params InitializeParams
				if err := internaljson.Unmarshal(jreq.Params, &params); err == nil {
					initializeProtocolVersion = params.ProtocolVersion
				}
			}
			if jreq.Method == methodSubscriptionsListen {
				isSubscriptionsListen = true
			}
			// SEP-2575: requests carrying `_meta.protocolVersion` require the
			// Mcp-Protocol-Version HTTP header to be present and to match the
			// per-request `_meta.protocolVersion` value.
			// The new (>= 2026-07-28) protocol is supported on the HTTP transport
			// only when [StreamableHTTPOptions.Stateless] is true.
			//
			// TODO: this validation can be moved within validateMcpHeaders.
			var metaVersion string
			if meta := extractRequestMeta(jreq.Params); meta != nil {
				metaVersion, _ = meta[MetaKeyProtocolVersion].(string)
			}
			if protocolVersion >= protocolVersion20260728 || metaVersion != "" {
				// Extract again the protcol version from the context to see what the client
				// is advertising in the Mcp-Protocol-Version HTTP header.
				headerVersion := protocolVersionFromContext(req.Context())
				// server/discover is exempt from the stateful
				// rejection as it should learn about the supported protocols from the
				// DiscoverResult response.
				if !c.stateless && jreq.Method != methodDiscover {
					http.Error(w, fmt.Sprintf(
						"Bad Request: protocol version %q is only supported on stateless HTTP servers (set StreamableHTTPOptions.Stateless = true)",
						protocolVersion),
						http.StatusBadRequest)
					return
				}
				if headerVersion == "" {
					writeJSONRPCError(w, http.StatusBadRequest, jreq.ID, &jsonrpc.Error{
						Code: CodeHeaderMismatch,
						Message: fmt.Sprintf(
							"%s header is required for requests carrying %q",
							protocolVersionHeader, MetaKeyProtocolVersion),
					})
					return
				}
				if metaVersion == "" {
					writeJSONRPCError(w, http.StatusBadRequest, jreq.ID, &jsonrpc.Error{
						Code: jsonrpc.CodeInvalidParams,
						Message: fmt.Sprintf(
							"missing or invalid _meta field %q",
							MetaKeyProtocolVersion),
					})
					return
				}
				if headerVersion != metaVersion {
					writeJSONRPCError(w, http.StatusBadRequest, jreq.ID, &jsonrpc.Error{
						Code: CodeHeaderMismatch,
						Message: fmt.Sprintf(
							"%s header %q does not match request %s %q",
							protocolVersionHeader, headerVersion,
							MetaKeyProtocolVersion, metaVersion),
					})
					return
				}
			}
			// Include metadata for all requests (including notifications).
			jreq.Extra = &RequestExtra{
				TokenInfo: tokenInfo,
				Header:    req.Header,
			}
			if jreq.IsCall() {
				calls[jreq.ID] = struct{}{}
				// See the doc for CloseSSEStream: allow the request handler to
				// explicitly close the ongoing stream.
				jreq.Extra.(*RequestExtra).CloseSSEStream = func(args CloseSSEStreamArgs) {
					// This mechanism was designed to trigger client reconnection with
					// Last-Event-ID for server-initiated disconnect scenarios. It is
					// deprecated in protocol version 2026-07-28.
					if protocolVersion >= protocolVersion20260728 {
						return
					}
					c.mu.Lock()
					streamID, ok := c.requestStreams[jreq.ID]
					var stream *stream
					if ok {
						stream = c.streams[streamID]
					}
					c.mu.Unlock()

					if stream != nil {
						stream.close(args.RetryAfter)
					}
				}
			}
		}
	}

	// Validate MCP standard headers (Mcp-Method, Mcp-Name, Mcp-Param-*)
	if !isBatch && len(incoming) == 1 {
		if err := validateMcpHeaders(req.Header, incoming[0], c.toolLookup); err != nil {
			resp := &jsonrpc.Response{
				Error: jsonrpc2.NewError(CodeHeaderMismatch, err.Error()),
			}
			if jreq, ok := incoming[0].(*jsonrpc.Request); ok {
				resp.ID = jreq.ID
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if data, err := jsonrpc2.EncodeMessage(resp); err == nil {
				w.Write(data)
			}
			return
		}
	}

	// The prime and close events were added in protocol version 2025-11-25 (SEP-1699).
	// Use the version from InitializeParams if this is an initialize request,
	// otherwise use the protocol version header.
	effectiveVersion := protocolVersion
	if isInitialize && initializeProtocolVersion != "" {
		effectiveVersion = initializeProtocolVersion
	}

	// If we don't have any calls, we can just publish the incoming messages and return.
	// No need to track a logical stream.
	//
	// See section [§2.1.4] of the spec: "If the server accepts the input, the
	// server MUST return HTTP status code 202 Accepted with no body."
	//
	// [§2.1.4]: https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#sending-messages-to-the-server
	if len(calls) == 0 {
		for _, msg := range incoming {
			select {
			case c.incoming <- msg:
			case <-c.done:
				// The session is closing. Since we haven't yet written any data to the
				// response, we can signal to the client that the session is gone.
				http.Error(w, "session is closing", http.StatusNotFound)
				return
			}
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Invariant: we have at least one call.
	//
	// Create a logical stream to track its responses.
	// Important: don't publish the incoming messages until the stream is
	// registered, as the server may attempt to respond to incoming messages as
	// soon as they're published.
	stream, err := c.newStream(req.Context(), calls, crand.Text())
	if err != nil {
		http.Error(w, fmt.Sprintf("storing stream: %v", err), http.StatusInternalServerError)
		return
	}
	stream.isListen = isSubscriptionsListen

	// subscriptions/listen is inherently a long-lived SSE endpoint (SEP-2575):
	// it has no synchronous result, the response stream stays open until the
	// client cancels, and the server pushes notifications on it as they occur.
	// Force SSE mode (bypassing JSONResponse) so the buffered application/json
	// path doesn't deadlock waiting for a completion that won't come.
	useSSE := !c.jsonResponse || isSubscriptionsListen

	// Set response headers. Accept was checked in [StreamableHTTPHandler].
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	if useSSE {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Connection", "keep-alive")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	if c.sessionID != "" && isInitialize {
		w.Header().Set(sessionIDHeader, c.sessionID)
	}

	// Set up stream delivery state.
	stream.w = w
	done := make(chan struct{})
	stream.done = done
	stream.protocolVersion = effectiveVersion

	// Reject any call whose ID is already in flight on this session,
	// atomically and without partial registration.
	c.mu.Lock()
	for reqID := range calls {
		if _, ok := c.requestStreams[reqID]; ok {
			c.mu.Unlock()
			writeJSONRPCError(w, http.StatusBadRequest, reqID, &jsonrpc.Error{
				Code:    jsonrpc.CodeInvalidRequest,
				Message: fmt.Sprintf("duplicate in-flight request ID %v", reqID.Raw()),
			})
			return
		}
	}
	c.streams[stream.id] = stream
	for reqID := range calls {
		c.requestStreams[reqID] = stream.id
	}
	c.mu.Unlock()

	// TODO(rfindley): if we have no event store, we should really cancel all
	// remaining requests here, since the client will never get the results.
	defer stream.release()

	if !useSSE {
		// JSON mode: collect messages in pendingJSONMessages until done.
		// Set pendingJSONMessages to a non-nil value to signal that this is an
		// application/json stream.
		stream.pendingJSONMessages = []json.RawMessage{}
	} else {
		// SSE mode: write a priming event if supported.
		//
		// SEP-2575 removes Last-Event-ID-based resumable streams for protocol
		// version >= 2026-07-28.
		if c.eventStore != nil && effectiveVersion >= protocolVersion20251125 && effectiveVersion < protocolVersion20260728 {
			// Write a priming event, as defined by [§2.1.6] of the spec.
			//
			// [§2.1.6]: https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#sending-messages-to-the-server
			//
			// We must also write it to the event store in order for indexes to
			// align.
			if err := c.eventStore.Append(req.Context(), c.sessionID, stream.id, nil); err != nil {
				c.logger.Warn(fmt.Sprintf("Storing priming event: %v", err))
			}
			stream.lastIdx++
			e := Event{Name: "prime", ID: formatEventID(stream.id, stream.lastIdx)}
			if _, err := writeEvent(w, e); err != nil {
				c.logger.Warn(fmt.Sprintf("Writing priming event: %v", err))
			}
		}
	}

	// Publish incoming messages.
	for _, msg := range incoming {
		select {
		case c.incoming <- msg:
		// Note: don't select on req.Context().Done() here, since we've already
		// received the requests and may have already published a response message
		// or notification. The client could resume the stream.
		//
		// In fact, this send could be in a separate goroutine.
		case <-c.done:
			// Session closed: we don't know if any data has been written, so it's
			// too late to write a status code here.
			return
		}
	}

	c.hangResponse(req.Context(), done)
}

// Event IDs: encode both the logical connection ID and the index, as
// <streamID>_<idx>, to be consistent with the typescript implementation.

// formatEventID returns the event ID to use for the logical connection ID
// streamID and message index idx.
//
// See also [parseEventID].
func formatEventID(sid string, idx int) string {
	return fmt.Sprintf("%s_%d", sid, idx)
}

// parseEventID parses a Last-Event-ID value into a logical stream id and
// index.
//
// See also [formatEventID].
func parseEventID(eventID string) (streamID string, idx int, ok bool) {
	parts := strings.Split(eventID, "_")
	if len(parts) != 2 {
		return "", 0, false
	}
	streamID = parts[0]
	idx, err := strconv.Atoi(parts[1])
	if err != nil || idx < 0 {
		return "", 0, false
	}
	return streamID, idx, true
}

// Read implements the [Connection] interface.
func (c *streamableServerConn) Read(ctx context.Context) (jsonrpc.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-c.incoming:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	case <-c.done:
		return nil, io.EOF
	}
}

// Write implements the [Connection] interface.
func (c *streamableServerConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	// Throughout this function, note that any error that wraps ErrRejected
	// indicates a does not cause the connection to break.
	//
	// Most errors don't break the connection: unlike a true bidirectional
	// stream, a failure to deliver to a stream is not an indication that the
	// logical session is broken.
	data, err := jsonrpc2.EncodeMessage(msg)
	if err != nil {
		return err
	}

	if req, ok := msg.(*jsonrpc.Request); ok && req.IsCall() && (c.stateless || c.sessionID == "") {
		// Requests aren't possible with stateless servers, or when there's no session ID.
		return fmt.Errorf("%w: stateless servers cannot make requests", jsonrpc2.ErrRejected)
	}

	// Find the incoming request that this write relates to, if any.
	var (
		relatedRequest jsonrpc.ID
		responseTo     jsonrpc.ID // if valid, the message is a response to this request
	)
	if resp, ok := msg.(*jsonrpc.Response); ok {
		// If the message is a response, it relates to its request (of course).
		relatedRequest = resp.ID
		responseTo = resp.ID
	} else {
		// Otherwise, we check to see if it request was made in the context of an
		// ongoing request. This may not be the case if the request was made with
		// an unrelated context.
		if v := ctx.Value(idContextKey{}); v != nil {
			relatedRequest = v.(jsonrpc.ID)
		}
	}

	// If the stream is application/json, but the message is not a response, we
	// must send it out of band to the standalone SSE stream.
	if c.jsonResponse && !responseTo.IsValid() {
		relatedRequest = jsonrpc.ID{}
	}

	// Write the message to the stream.
	var s *stream
	c.mu.Lock()
	if relatedRequest.IsValid() {
		if streamID, ok := c.requestStreams[relatedRequest]; ok {
			s = c.streams[streamID]
		}
	} else {
		// In stateless mode there will always be only one stream per connection.
		// If that stream was open to listen for subscription notifications,
		// automatically select as the one to write the notification to.
		for _, stream := range c.streams {
			if stream.isListen {
				s = stream
				break
			}
		}
		if s == nil {
			s = c.streams[""] // standalone SSE stream
		}
	}
	if responseTo.IsValid() {
		// Once we've responded to a request, disallow related messages by removing
		// the stream association. This also releases memory.
		delete(c.requestStreams, responseTo)
	}
	sessionClosed := c.isDone
	c.mu.Unlock()

	if s == nil {
		// The request was made in the context of an ongoing request, but that
		// request is complete.
		//
		// In the future, we could be less strict and allow the request to land on
		// the standalone SSE stream.
		return fmt.Errorf("%w: write to closed stream", jsonrpc2.ErrRejected)
	}
	if sessionClosed {
		return errors.New("session is closed")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store in eventStore before delivering.
	// TODO(rfindley): we should only append if the response is SSE, not JSON, by
	// pushing down into the delivery layer.
	delivered := false
	var errs []error
	protocolVersion := protocolVersionFromContext(ctx)
	if c.eventStore != nil && protocolVersion < protocolVersion20260728 {
		if err := c.eventStore.Append(ctx, c.sessionID, s.id, data); err != nil {
			errs = append(errs, err)
		} else {
			delivered = true
		}
	}

	// Compute eventID for SSE streams with event store.
	// Use s.lastIdx + 1 because deliverLocked increments before writing.
	var eventID string
	if c.eventStore != nil && protocolVersion < protocolVersion20260728 {
		eventID = formatEventID(s.id, s.lastIdx+1)
	}

	// SEP-2575: map protocol-level JSON-RPC error codes to HTTP status codes
	// on the new protocol (>= 2026-07-28). When non-zero, deliverLocked will
	// write the body as raw application/json with the override status.
	overrideStatus := extractErrorStatus(ctx, msg)

	done, err := s.deliverLocked(data, eventID, responseTo, overrideStatus)
	if err != nil {
		errs = append(errs, err)
	} else {
		delivered = true
	}

	if done {
		c.mu.Lock()
		delete(c.streams, s.id)
		c.mu.Unlock()
	}

	if !delivered {
		return fmt.Errorf("%w: undelivered message: %v", jsonrpc2.ErrRejected, errors.Join(errs...))
	}
	return nil
}

// Close implements the [Connection] interface.
func (c *streamableServerConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isDone {
		c.isDone = true
		close(c.done)
		if c.eventStore != nil {
			// TODO: find a way to plumb a context here, or an event store with a long-running
			// close operation can take arbitrary time. Alternative: impose a fixed timeout here.
			return c.eventStore.SessionClosed(context.TODO(), c.sessionID)
		}
	}
	return nil
}

// A StreamableClientTransport is a [Transport] that can communicate with an MCP
// endpoint serving the streamable HTTP transport defined by the 2025-03-26
// version of the spec.
type StreamableClientTransport struct {
	Endpoint   string
	HTTPClient *http.Client
	// MaxRetries is the maximum number of times to attempt a reconnect before giving up.
	// It defaults to 5. To disable retries, use a negative number.
	MaxRetries int

	// DisableStandaloneSSE controls whether the client establishes a standalone SSE stream
	// for receiving server-initiated messages.
	//
	// When false (the default), after initialization the client sends an HTTP GET request
	// to establish a persistent server-sent events (SSE) connection. This allows the server
	// to send messages to the client at any time, such as ToolListChangedNotification or
	// other server-initiated requests and notifications. The connection persists for the
	// lifetime of the session and automatically reconnects if interrupted.
	//
	// When true, the client does not establish the standalone SSE stream. The client will
	// only receive responses to its own POST requests. Server-initiated messages will not
	// be received.
	//
	// According to the MCP specification, the standalone SSE stream is optional.
	// Setting DisableStandaloneSSE to true is useful when:
	//   - You only need request-response communication and don't need server-initiated notifications
	//   - The server doesn't properly handle GET requests for SSE streams
	//   - You want to avoid maintaining a persistent connection
	DisableStandaloneSSE bool

	// OAuthHandler is an optional field that, if provided, will be used to authorize the requests.
	OAuthHandler auth.OAuthHandler

	// TODO(rfindley): propose exporting these.
	// If strict is set, the transport is in 'strict mode', where any violation
	// of the MCP spec causes a failure.
	strict bool
	// If logger is set, it is used to log aspects of the transport, such as spec
	// violations that were ignored.
	logger *slog.Logger
}

// These settings are not (yet) exposed to the user in
// StreamableClientTransport.
const (
	// reconnectGrowFactor is the multiplicative factor by which the delay increases after each attempt.
	// A value of 1.0 results in a constant delay, while a value of 2.0 would double it each time.
	// It must be 1.0 or greater if MaxRetries is greater than 0.
	reconnectGrowFactor = 1.5
	// reconnectMaxDelay caps the backoff delay, preventing it from growing indefinitely.
	reconnectMaxDelay = 30 * time.Second
)

var (
	// reconnectInitialDelay is the base delay for the first reconnect attempt.
	//
	// Mutable for testing.
	reconnectInitialDelay atomic.Int64
)

func init() {
	reconnectInitialDelay.Store(int64(1 * time.Second))
}

// Connect implements the [Transport] interface.
//
// The resulting [Connection] writes messages via POST requests to the
// transport URL, and reads messages from hanging requests. If the server
// provides a session ID via the Mcp-Session-Id response header, subsequent
// requests include it; sessionless servers that omit the header are fully
// supported.
//
// When closed, the connection issues a DELETE request to terminate the
// session, unless no session was established.
func (t *StreamableClientTransport) Connect(ctx context.Context) (Connection, error) {
	client := t.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	maxRetries := t.MaxRetries
	if maxRetries == 0 {
		maxRetries = 5
	} else if maxRetries < 0 {
		maxRetries = 0
	}
	// Create a new cancellable context that will manage the connection's lifecycle.
	// This is crucial for cleanly shutting down the background SSE listener by
	// cancelling its blocking network operations, which prevents hangs on exit.
	//
	// This context should be detached from the incoming context: the standalone
	// SSE request should not break when the connection context is done.
	//
	// For example, consider that the user may want to wait at most 5s to connect
	// to the server, and therefore uses a context with a 5s timeout when calling
	// client.Connect. Let's suppose that Connect returns after 1s, and the user
	// starts using the resulting session. If we didn't detach here, the session
	// would break after 4s, when the background SSE stream is terminated.
	//
	// Instead, creating a cancellable context detached from the incoming context
	// allows us to preserve context values (which may be necessary for auth
	// middleware), yet only cancel the standalone stream when the connection is closed.
	connCtx, cancel := context.WithCancel(xcontext.Detach(ctx))
	conn := &streamableClientConn{
		url:                  t.Endpoint,
		client:               client,
		incoming:             make(chan jsonrpc.Message, 10),
		done:                 make(chan struct{}),
		maxRetries:           maxRetries,
		strict:               t.strict,
		logger:               ensureLogger(t.logger), // must be non-nil for safe logging
		ctx:                  connCtx,
		cancel:               cancel,
		failed:               make(chan struct{}),
		disableStandaloneSSE: t.DisableStandaloneSSE,
		oauthHandler:         t.OAuthHandler,
	}
	return conn, nil
}

type streamableClientConn struct {
	url        string
	client     *http.Client
	ctx        context.Context    // connection context, detached from Connect
	cancel     context.CancelFunc // cancels ctx
	incoming   chan jsonrpc.Message
	maxRetries int
	strict     bool         // from [StreamableClientTransport.strict]
	logger     *slog.Logger // from [StreamableClientTransport.logger]

	// disableStandaloneSSE controls whether to disable the standalone SSE stream
	// for receiving server-to-client notifications when no request is in flight.
	disableStandaloneSSE bool // from [StreamableClientTransport.DisableStandaloneSSE]

	// oauthHandler is the OAuth handler for the connection.
	oauthHandler auth.OAuthHandler // from [StreamableClientTransport.OAuthHandler]

	// Guard calls to Close, as it may be called multiple times.
	closeOnce sync.Once
	closeErr  error
	done      chan struct{} // signal graceful termination

	// Logical reads are distributed across multiple http requests. Whenever any
	// of them fails to process their response, we must break the connection, by
	// failing the pending Read.
	//
	// Achieve this by storing the failure message, and signalling when reads are
	// broken. See also [streamableClientConn.fail] and
	// [streamableClientConn.failure].
	failOnce sync.Once
	_failure error
	failed   chan struct{} // signal failure

	// Guard the initialization state.
	mu                sync.Mutex
	initializedResult *InitializeResult
	sessionID         string
}

var _ clientConnection = (*streamableClientConn)(nil)

func (c *streamableClientConn) sessionUpdated(state clientSessionState) {
	c.mu.Lock()
	c.initializedResult = state.InitializeResult
	c.mu.Unlock()

	// Under SEP-2575 (protocol version >= 2026-07-28) the standalone HTTP GET
	// SSE stream is removed.
	if state.InitializeResult == nil ||
		state.InitializeResult.ProtocolVersion >= protocolVersion20260728 {
		return
	}

	// Start the standalone SSE stream as soon as we have the initialized
	// result, if continuous listening is enabled.
	//
	// § 2.2: The client MAY issue an HTTP GET to the MCP endpoint. This can be
	// used to open an SSE stream, allowing the server to communicate to the
	// client, without the client first sending data via HTTP POST.
	//
	// We have to wait for initialized, because until we've received
	// initialized, we don't know whether the server requires a sessionID.
	//
	// § 2.5: A server using the Streamable HTTP transport MAY assign a session
	// ID at initialization time, by including it in a Mcp-Session-Id header
	// on the HTTP response containing the InitializeResult.
	if !c.disableStandaloneSSE {
		c.connectStandaloneSSE()
	}
}

func (c *streamableClientConn) connectStandaloneSSE() {
	resp, err := c.connectSSE(c.ctx, "", 0, true)
	if err != nil {
		// If the client didn't cancel the request, and failure breaks the logical
		// session.
		if c.ctx.Err() == nil {
			c.fail(fmt.Errorf("standalone SSE request failed (session ID: %v): %v", c.sessionID, err))
		}
		return
	}

	// [§2.2.3]: "The server MUST either return Content-Type:
	// text/event-stream in response to this HTTP GET, or else return HTTP
	// 405 Method Not Allowed, indicating that the server does not offer an
	// SSE stream at this endpoint."
	//
	// [§2.2.3]: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#listening-for-messages-from-the-server
	if resp.StatusCode == http.StatusMethodNotAllowed {
		// The server doesn't support the standalone SSE stream.
		resp.Body.Close()
		return
	}
	if baseMediaType(resp.Header.Get("Content-Type")) != "text/event-stream" {
		// modelcontextprotocol/go-sdk#736: some servers return 200 OK or redirect with
		// non-SSE content type instead of text/event-stream for the standalone
		// SSE stream.
		c.logger.Warn(fmt.Sprintf("got Content-Type %s instead of text/event-stream for standalone SSE stream", resp.Header.Get("Content-Type")))
		resp.Body.Close()
		return
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 && !c.strict {
		// modelcontextprotocol/go-sdk#393,#610: some servers return NotFound or
		// other status codes instead of MethodNotAllowed for the standalone SSE
		// stream.
		//
		// Treat this like MethodNotAllowed in non-strict mode.
		c.logger.Warn(fmt.Sprintf("got %d instead of 405 for standalone SSE stream", resp.StatusCode))
		resp.Body.Close()
		return
	}
	summary := "standalone SSE stream"
	if err := c.checkResponse(c.ctx, summary, resp); err != nil {
		c.fail(err)
		return
	}
	go c.handleSSE(c.ctx, summary, resp, nil)
}

// fail handles an asynchronous error while reading.
//
// If err is non-nil, it is terminal, and subsequent (or pending) Reads will
// fail.
//
// If err wraps ErrSessionMissing, the failure indicates that the session is no
// longer present on the server, and no final DELETE will be performed when
// closing the connection.
func (c *streamableClientConn) fail(err error) {
	if err != nil {
		c.failOnce.Do(func() {
			c._failure = err
			close(c.failed)
		})
	}
}

func (c *streamableClientConn) failure() error {
	select {
	case <-c.failed:
		return c._failure
	default:
		return nil
	}
}

func (c *streamableClientConn) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

// Read implements the [Connection] interface.
func (c *streamableClientConn) Read(ctx context.Context) (jsonrpc.Message, error) {
	if err := c.failure(); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.failed:
		return nil, c.failure()
	case <-c.done:
		return nil, io.EOF
	case msg := <-c.incoming:
		return msg, nil
	}
}

// Write implements the [Connection] interface.
func (c *streamableClientConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	if err := c.failure(); err != nil {
		return err
	}

	var requestSummary string
	var requestMethod string
	var forCall *jsonrpc.Request
	switch msg := msg.(type) {
	case *jsonrpc.Request:
		requestSummary = fmt.Sprintf("sending %q", msg.Method)
		if msg.IsCall() {
			forCall = msg
			requestMethod = msg.Method
		}
	case *jsonrpc.Response:
		requestSummary = fmt.Sprintf("sending jsonrpc response #%d", msg.ID)
	default:
		panic("unreachable")
	}

	data, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("%s: %v", requestSummary, err)
	}

	doRequest := func() (*http.Request, *http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(data))
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")

		if err := c.setMCPHeaders(req, msg); err != nil {
			// Failure to set headers means that the request was not sent.
			// Wrap with ErrRejected so the jsonrpc2 connection doesn't set writeErr
			// and permanently break the connection.
			return nil, nil, fmt.Errorf("%s: %w: %w", requestSummary, jsonrpc2.ErrRejected, err)
		}
		// Keep this after the setMCPHeaders call to ensure that the
		// protocol version header is set.
		setStandardHeaders(ctx, req.Header, msg)
		resp, err := c.client.Do(req)
		if err != nil {
			// Any error from client.Do means the request didn't reach the server.
			// Wrap with ErrRejected so the jsonrpc2 connection doesn't set writeErr
			// and permanently break the connection.
			err = fmt.Errorf("%s: %w: %w", requestSummary, jsonrpc2.ErrRejected, err)
		}
		return req, resp, err
	}

	req, resp, err := doRequest()
	if err != nil {
		return err
	}

	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && c.oauthHandler != nil {
		if err := c.oauthHandler.Authorize(ctx, req, resp); err != nil {
			// If the caller's context was cancelled while we were running the
			// authorization flow, treat the connection as failed so subsequent
			// operations on it (e.g. the cancellation notify the call layer
			// sends in response to ctx cancellation) short-circuit instead of
			// re-invoking the OAuth handler. Otherwise the user gets prompted
			// to authorize a request they have already abandoned. See #882.
			//
			// We check ctx.Err() rather than the error returned by Authorize,
			// because the handler is user-implemented and may return an error
			// that does not wrap context.Canceled (e.g. a custom sentinel or
			// a fmt.Errorf with %v). The context itself is the authoritative
			// source for whether the caller abandoned the request.
			ctxErr := ctx.Err()
			if errors.Is(ctxErr, context.Canceled) || errors.Is(ctxErr, context.DeadlineExceeded) {
				c.fail(fmt.Errorf("%s: authorization cancelled: %w", requestSummary, err))
			}
			// Wrap with ErrRejected so the jsonrpc2 connection doesn't set writeErr
			// and permanently break the connection.
			// Wrap the authorization error as well for client inspection.
			return fmt.Errorf("%s: %w: %w", requestSummary, jsonrpc2.ErrRejected, err)
		}
		// Retry the request after successful authorization.
		_, resp, err = doRequest()
		if err != nil {
			return err
		}
	}

	if err := c.checkResponse(ctx, requestSummary, resp); err != nil {
		if requestMethod == methodDiscover {
			// Wrap the discover failure with ErrRejected so the jsonrpc2 layer
			// doesn't set writeErr, which would prevent the legacy initialize
			// fallback from succeeding on the same connection.
			err = fmt.Errorf("%w: %w", err, jsonrpc2.ErrRejected)
		} else if !errors.Is(err, jsonrpc2.ErrRejected) {
			// Only fail the connection for non-transient errors.
			// Transient errors (wrapped with ErrRejected) should not break the connection.
			c.fail(err)
		}
		return err
	}

	if sessionID := resp.Header.Get(sessionIDHeader); sessionID != "" {
		c.mu.Lock()
		hadSessionID := c.sessionID
		if hadSessionID == "" {
			c.sessionID = sessionID
		}
		c.mu.Unlock()
		if hadSessionID != "" && hadSessionID != sessionID {
			resp.Body.Close()
			return fmt.Errorf("mismatching session IDs %q and %q", hadSessionID, sessionID)
		}
	}

	if forCall == nil {
		resp.Body.Close()

		// [§2.1.4]: "If the input is a JSON-RPC response or notification:
		// If the server accepts the input, the server MUST return HTTP status code 202 Accepted with no body."
		//
		// [§2.1.4]: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#listening-for-messages-from-the-server
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
			errMsg := fmt.Sprintf("unexpected status code %d from non-call", resp.StatusCode)
			// Some servers return 200, even with an empty json body.
			//
			// In strict mode, return an error to the caller.
			c.logger.Warn(errMsg)
			if c.strict {
				return errors.New(errMsg)
			}
		}
		return nil
	}

	contentType := baseMediaType(resp.Header.Get("Content-Type"))
	switch contentType {
	case "application/json":
		go c.handleJSON(requestSummary, resp)

	case "text/event-stream":
		var forCall *jsonrpc.Request
		if jsonReq, ok := msg.(*jsonrpc.Request); ok && jsonReq.IsCall() {
			forCall = jsonReq
		}
		// Handle the resulting stream. Note that ctx comes from the call, and
		// therefore is already cancelled when the JSON-RPC request is cancelled
		// (or rather, context cancellation is what *triggers* JSON-RPC
		// cancellation)
		go c.handleSSE(ctx, requestSummary, resp, forCall)

	default:
		resp.Body.Close()
		return fmt.Errorf("%s: unsupported content type %q", requestSummary, contentType)
	}
	return nil
}

func (c *streamableClientConn) setMCPHeaders(req *http.Request, msg jsonrpc.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.oauthHandler != nil {
		ts, err := c.oauthHandler.TokenSource(c.ctx)
		if err != nil {
			return err
		}
		if ts != nil {
			token, err := ts.Token()
			if err != nil {
				// If the error is an invalid_grant oauth2.RetrieveError it indicates
				// that the token source doesn't have valid authorization for the token
				// endpoint, per RFC 6749 section 5.2. For example, the refresh token
				// may be expired or invalid.
				//
				// In that case, ignore the error, skip setting the Authorization
				// header, and proceed with the request. Callers that support
				// authorization flows get a 401/403 response and trigger the
				// Authorize() flow to refresh their token.
				var retrieveErr *oauth2.RetrieveError
				if !errors.As(err, &retrieveErr) || retrieveErr.ErrorCode != "invalid_grant" {
					return err
				}
			} else if token != nil {
				req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			}
		}
	}
	if pv := protocolVersionFromMessage(msg); pv != "" {
		req.Header.Set(protocolVersionHeader, pv)
	} else if pv := protocolVersionFromContext(req.Context()); pv != "" {
		req.Header.Set(protocolVersionHeader, pv)
	} else if c.initializedResult != nil {
		req.Header.Set(protocolVersionHeader, c.initializedResult.ProtocolVersion)
	}
	if c.sessionID != "" {
		req.Header.Set(sessionIDHeader, c.sessionID)
	}

	return nil
}

// protocolVersionFromMessage recovers the SEP-2575 `_meta.protocolVersion`
// value from an outgoing JSON-RPC request, if present. It returns "" for
// notifications, responses, requests without a `_meta.protocolVersion`, or a
// nil msg.
func protocolVersionFromMessage(msg jsonrpc.Message) string {
	req, ok := msg.(*jsonrpc.Request)
	if !ok || req == nil {
		return ""
	}
	meta := extractRequestMeta(req.Params)
	if meta == nil {
		return ""
	}
	v, _ := meta[MetaKeyProtocolVersion].(string)
	return v
}

func (c *streamableClientConn) handleJSON(requestSummary string, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		c.fail(fmt.Errorf("%s: failed to read body: %v", requestSummary, err))
		return
	}
	msg, err := jsonrpc.DecodeMessage(body)
	if err != nil {
		c.fail(fmt.Errorf("%s: failed to decode response: %v", requestSummary, err))
		return
	}
	select {
	case c.incoming <- msg:
	case <-c.done:
		// The connection was closed by the client; exit gracefully.
	}
}

// handleSSE manages the lifecycle of an SSE connection. It can be either
// persistent (for the main GET listener) or temporary (for a POST response).
//
// If forCall is set, it is the call that initiated the stream, and the
// stream is complete when we receive its response. Otherwise, this is the
// standalone stream.
func (c *streamableClientConn) handleSSE(ctx context.Context, requestSummary string, resp *http.Response, forCall *jsonrpc2.Request) {
	// Track the last event ID to detect progress.
	// The retry counter is only reset when progress is made (lastEventID advances).
	// This prevents infinite retry loops when a server repeatedly terminates
	// connections without making progress (#679).
	var prevLastEventID string
	retriesWithoutProgress := 0

	for {
		lastEventID, reconnectDelay, clientClosed := c.processStream(ctx, requestSummary, resp, forCall)

		// If the connection was closed by the client, we're done.
		if clientClosed {
			return
		}
		// If we don't have a last event ID, we can never get the call response, so
		// there's nothing to resume. For the standalone stream, we can reconnect,
		// but we may just miss messages.
		if lastEventID == "" && forCall != nil {
			return
		}

		// Check if we made progress (lastEventID advanced).
		// Only reset the retry counter when actual progress is made.
		if lastEventID != "" && lastEventID != prevLastEventID {
			// Progress was made: reset the retry counter.
			retriesWithoutProgress = 0
			prevLastEventID = lastEventID
		} else {
			// No progress: increment the retry counter.
			retriesWithoutProgress++
			if retriesWithoutProgress > c.maxRetries {
				if ctx.Err() == nil {
					c.fail(fmt.Errorf("%s: exceeded %d retries without progress (session ID: %v)", requestSummary, c.maxRetries, c.sessionID))
				}
				return
			}
		}

		// The stream was interrupted or ended by the server. Attempt to reconnect.
		newResp, err := c.connectSSE(ctx, lastEventID, reconnectDelay, false)
		if err != nil {
			// If the client didn't cancel this request, any failure to execute it
			// breaks the logical MCP session.
			if ctx.Err() == nil {
				// All reconnection attempts failed: fail the connection.
				c.fail(fmt.Errorf("%s: failed to reconnect (session ID: %v): %v", requestSummary, c.sessionID, err))
			}
			return
		}

		resp = newResp
		if err := c.checkResponse(ctx, requestSummary, resp); err != nil {
			c.fail(err)
			return
		}
	}
}

// checkResponse checks the status code of the provided response, and
// translates it into an error if the request was unsuccessful.
//
// The response body is close if a non-nil error is returned.
func (c *streamableClientConn) checkResponse(ctx context.Context, requestSummary string, resp *http.Response) (err error) {
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()
	// §2.5.3: "The server MAY terminate the session at any time, after
	// which it MUST respond to requests containing that session ID with HTTP
	// 404 Not Found."
	if resp.StatusCode == http.StatusNotFound {
		// Return an ErrSessionMissing to avoid sending a redundant DELETE when the
		// session is already gone.
		return fmt.Errorf("%s: failed to connect (session ID: %v): %w", requestSummary, c.sessionID, ErrSessionMissing)
	}
	// Transient server errors (502, 503, 504, 429) should not break the connection.
	// Wrap them with ErrRejected so the jsonrpc2 layer doesn't set writeErr.
	if isTransientHTTPStatus(resp.StatusCode) {
		return fmt.Errorf("%w: %s: %v", jsonrpc2.ErrRejected, requestSummary, http.StatusText(resp.StatusCode))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// By default, always try to decode the body and surface the underlying
		// JSON-RPC error.
		// Setting MCPGODEBUG=noprotocolerrorbody=1 restores the previous behavior.
		if noprotocolerrorbody == "1" {
			return fmt.Errorf("%s: %v", requestSummary, http.StatusText(resp.StatusCode))
		}
		body, _ := io.ReadAll(resp.Body)
		msg, _ := jsonrpc.DecodeMessage(body)
		if response, ok := msg.(*jsonrpc.Response); ok && response.Error != nil {
			return fmt.Errorf("%s: %w: %v", requestSummary, response.Error, http.StatusText(resp.StatusCode))
		}
		return fmt.Errorf("%s: %v", requestSummary, http.StatusText(resp.StatusCode))
	}
	return nil
}

// processStream reads from a single response body, sending events to the
// incoming channel. It returns the ID of the last processed event and a flag
// indicating if the connection was closed by the client. If resp is nil, it
// returns "", false.
func (c *streamableClientConn) processStream(ctx context.Context, requestSummary string, resp *http.Response, forCall *jsonrpc.Request) (lastEventID string, reconnectDelay time.Duration, clientClosed bool) {
	defer func() {
		// Drain any remaining unprocessed body. This allows the connection to be re-used after closing.
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	for evt, err := range scanEvents(resp.Body) {
		if err != nil {
			if ctx.Err() != nil {
				return "", 0, true // don't reconnect: client cancelled
			}

			// Malformed events are hard errors that indicate corrupted data or protocol
			// violations. These should fail the connection permanently.
			if errors.Is(err, errMalformedEvent) {
				c.fail(fmt.Errorf("%s: %v", requestSummary, err))
				return "", 0, true
			}

			break
		}

		if evt.ID != "" {
			lastEventID = evt.ID
		}

		if evt.Retry != "" {
			if n, err := strconv.ParseInt(evt.Retry, 10, 64); err == nil {
				reconnectDelay = time.Duration(n) * time.Millisecond
			}
		}

		// According to SSE specification
		// (https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation)
		// events with an empty data buffer are allowed.
		// In MCP these can be priming events (SEP-1699) that carry only a Last-Event-ID for stream resumption.
		if len(evt.Data) == 0 {
			continue
		}

		// According to SSE spec, events with no name default to "message"
		if evt.Name != "" && evt.Name != "message" {
			continue
		}

		msg, err := jsonrpc.DecodeMessage(evt.Data)
		if err != nil {
			c.fail(fmt.Errorf("%s: failed to decode event: %v", requestSummary, err))
			return "", 0, true
		}

		select {
		case c.incoming <- msg:
			// Check if this is the response to our call, which terminates the request.
			// (it could also be a server->client request or notification).
			if jsonResp, ok := msg.(*jsonrpc.Response); ok && forCall != nil {
				// TODO: we should never get a response when forReq is nil (the standalone SSE request).
				// We should detect this case.
				if jsonResp.ID == forCall.ID {
					return "", 0, true
				}
			}

		case <-c.done:
			// The connection was closed by the client; exit gracefully.
			return "", 0, true
		}
	}
	// The loop finished without an error, indicating the server closed the stream.
	//
	// If the lastEventID is "", the stream is not retryable and we should
	// report a synthetic error for the call.
	//
	// Note that this is different from the cancellation case above, since the
	// caller is still waiting for a response that will never come.
	if lastEventID == "" && forCall != nil {
		errmsg := &jsonrpc2.Response{
			ID:    forCall.ID,
			Error: fmt.Errorf("request terminated without response"),
		}
		select {
		case c.incoming <- errmsg:
		case <-c.done:
		}
	}
	return lastEventID, reconnectDelay, false
}

// connectSSE handles the logic of connecting a text/event-stream connection.
//
// If lastEventID is set, it is the last-event ID of a stream being resumed.
//
// If connection fails, connectSSE retries with an exponential backoff
// strategy. It returns a new, valid HTTP response if successful, or an error
// if all retries are exhausted.
//
// reconnectDelay is the delay set by the server using the SSE retry field, or
// 0.
//
// If initial is set, this is the initial attempt.
//
// If connectSSE exits due to context cancellation, the result is (nil, ctx.Err()).
func (c *streamableClientConn) connectSSE(ctx context.Context, lastEventID string, reconnectDelay time.Duration, initial bool) (*http.Response, error) {
	var finalErr error
	attempt := 0
	if !initial {
		// We've already connected successfully once, so delay subsequent
		// reconnections. Otherwise, if the server returns 200 but terminates the
		// connection, we'll reconnect as fast as we can, ad infinitum.
		//
		// TODO: we should consider also setting a limit on total attempts for one
		// logical request.
		attempt = 1
	}
	delay := calculateReconnectDelay(attempt)
	if reconnectDelay > 0 {
		delay = reconnectDelay // honor the server's requested initial delay
	}
	for ; attempt <= c.maxRetries; attempt++ {
		select {
		case <-c.done:
			return nil, fmt.Errorf("connection closed by client during reconnect")

		case <-ctx.Done():
			// If the connection context is canceled, the request below will not
			// succeed anyway.
			return nil, ctx.Err()

		case <-time.After(delay):
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
			if err != nil {
				return nil, err
			}
			if err := c.setMCPHeaders(req, nil); err != nil {
				return nil, err
			}
			if lastEventID != "" {
				req.Header.Set(lastEventIDHeader, lastEventID)
			}
			req.Header.Set("Accept", "text/event-stream")
			resp, err := c.client.Do(req)
			if err != nil {
				finalErr = err // Store the error and try again.
				delay = calculateReconnectDelay(attempt + 1)
				continue
			}
			return resp, nil
		}
	}
	// If the loop completes, all retries have failed, or the client is closing.
	if finalErr != nil {
		return nil, fmt.Errorf("connection failed after %d attempts: %w", c.maxRetries, finalErr)
	}
	return nil, fmt.Errorf("connection aborted after %d attempts", c.maxRetries)
}

// Close implements the [Connection] interface.
func (c *streamableClientConn) Close() error {
	c.closeOnce.Do(func() {
		if errors.Is(c.failure(), ErrSessionMissing) {
			// If the session is missing, no need to delete it.
		} else if c.SessionID() == "" {
			// No session was established (e.g. the server is stateless),
			// so there is nothing to delete.
		} else {
			req, err := http.NewRequestWithContext(c.ctx, http.MethodDelete, c.url, nil)
			if err != nil {
				c.closeErr = err
			} else {
				if err := c.setMCPHeaders(req, nil); err != nil {
					c.closeErr = err
				} else if resp, err := c.client.Do(req); err != nil {
					c.closeErr = err
				} else {
					resp.Body.Close()
				}
			}
		}

		// Cancel any hanging network requests after cleanup.
		c.cancel()
		close(c.done)
	})
	return c.closeErr
}

// calculateReconnectDelay calculates a delay using exponential backoff with full jitter.
func calculateReconnectDelay(attempt int) time.Duration {
	if attempt == 0 {
		return 0
	}
	// Calculate the exponential backoff using the grow factor.
	backoffDuration := time.Duration(float64(reconnectInitialDelay.Load()) * math.Pow(reconnectGrowFactor, float64(attempt-1)))
	// Cap the backoffDuration at maxDelay.
	backoffDuration = min(backoffDuration, reconnectMaxDelay)

	// Use a full jitter using backoffDuration
	jitter := rand.N(backoffDuration)

	return backoffDuration + jitter
}

// isTransientHTTPStatus reports whether the HTTP status code indicates a
// transient server error that should not permanently break the connection.
func isTransientHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusInternalServerError, // 500
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout,     // 504
		http.StatusTooManyRequests:    // 429
		return true
	}
	return false
}
