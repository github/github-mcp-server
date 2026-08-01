// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Logging levels.
const (
	LevelDebug     = slog.LevelDebug
	LevelInfo      = slog.LevelInfo
	LevelNotice    = (slog.LevelInfo + slog.LevelWarn) / 2
	LevelWarning   = slog.LevelWarn
	LevelError     = slog.LevelError
	LevelCritical  = slog.LevelError + 4
	LevelAlert     = slog.LevelError + 8
	LevelEmergency = slog.LevelError + 12
)

var slogToMCP = map[slog.Level]LoggingLevel{
	LevelDebug:     "debug",
	LevelInfo:      "info",
	LevelNotice:    "notice",
	LevelWarning:   "warning",
	LevelError:     "error",
	LevelCritical:  "critical",
	LevelAlert:     "alert",
	LevelEmergency: "emergency",
}

var mcpToSlog = make(map[LoggingLevel]slog.Level)

func init() {
	for sl, ml := range slogToMCP {
		mcpToSlog[ml] = sl
	}
}

func slogLevelToMCP(sl slog.Level) LoggingLevel {
	if ml, ok := slogToMCP[sl]; ok {
		return ml
	}
	return "debug" // for lack of a better idea
}

func mcpLevelToSlog(ll LoggingLevel) slog.Level {
	if sl, ok := mcpToSlog[ll]; ok {
		return sl
	}
	// TODO: is there a better default?
	return LevelDebug
}

// compareLevels behaves like [cmp.Compare] for [LoggingLevel]s.
func compareLevels(l1, l2 LoggingLevel) int {
	return cmp.Compare(mcpLevelToSlog(l1), mcpLevelToSlog(l2))
}

// LoggingHandlerOptions are options for a LoggingHandler.
//
// Deprecated: the logging feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
type LoggingHandlerOptions struct {
	// The value for the "logger" field of logging notifications.
	LoggerName string
	// Limits the rate at which log messages are sent.
	// Excess messages are dropped.
	// If zero, there is no rate limiting.
	MinInterval time.Duration
}

// A LoggingHandler is a [slog.Handler] for MCP.
//
// Deprecated: the logging feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
type LoggingHandler struct {
	opts LoggingHandlerOptions
	ss   *ServerSession
	// Ensures that the buffer reset is atomic with the write (see Handle).
	// A pointer so that clones share the mutex. See
	// https://github.com/golang/example/blob/master/slog-handler-guide/README.md#getting-the-mutex-right.
	mu      *sync.Mutex
	limiter *rate.Limiter // for rate-limiting
	buf     *bytes.Buffer
	handler slog.Handler
}

// ensureLogger returns l if non-nil, otherwise a discard logger.
func ensureLogger(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.New(slog.DiscardHandler)
}

// NewLoggingHandler creates a [LoggingHandler] that logs to the given [ServerSession] using a
// [slog.JSONHandler].
//
// Deprecated: the logging feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
func NewLoggingHandler(ss *ServerSession, opts *LoggingHandlerOptions) *LoggingHandler {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Remove level: it appears in LoggingMessageParams.
			if a.Key == slog.LevelKey {
				return slog.Attr{}
			}
			return a
		},
	})
	lh := &LoggingHandler{
		ss:      ss,
		mu:      new(sync.Mutex),
		buf:     &buf,
		handler: jsonHandler,
	}
	if opts != nil {
		lh.opts = *opts
		if opts.MinInterval > 0 {
			lh.limiter = rate.NewLimiter(rate.Every(opts.MinInterval), 1)
		}
	}
	return lh
}

// Enabled implements [slog.Handler.Enabled] by comparing level to the [ServerSession]'s level.
func (h *LoggingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// This is also checked in ServerSession.LoggingMessage, so checking it here
	// is just an optimization that skips building the JSON.
	h.ss.mu.Lock()
	mcpLevel := h.ss.state.LogLevel
	h.ss.mu.Unlock()
	return level >= mcpLevelToSlog(mcpLevel)
}

// WithAttrs implements [slog.Handler.WithAttrs].
func (h *LoggingHandler) WithAttrs(as []slog.Attr) slog.Handler {
	h2 := *h
	h2.handler = h.handler.WithAttrs(as)
	return &h2
}

// WithGroup implements [slog.Handler.WithGroup].
func (h *LoggingHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.handler = h.handler.WithGroup(name)
	return &h2
}

// Handle implements [slog.Handler.Handle] by writing the Record to a JSONHandler,
// then calling [ServerSession.LoggingMessage] with the result.
func (h *LoggingHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.handle(ctx, r)
	// TODO(jba): find a way to surface the error.
	// The return value will probably be ignored.
	return err
}

func (h *LoggingHandler) handle(ctx context.Context, r slog.Record) error {
	// Observe the rate limit.
	if h.limiter != nil && !h.limiter.Allow() {
		return nil
	}

	var err error
	var data json.RawMessage
	// Make the buffer reset atomic with the record write.
	// We are careful here in the unlikely event that the handler panics.
	// We don't want to hold the lock for the entire function, because Notify is
	// an I/O operation.
	// This can result in out-of-order delivery.
	func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.buf.Reset()
		err = h.handler.Handle(ctx, r)
		// Clone the buffer as Bytes() references the internal buffer.
		data = json.RawMessage(slices.Clone(h.buf.Bytes()))
	}()
	if err != nil {
		return err
	}

	params := &LoggingMessageParams{
		Logger: h.opts.LoggerName,
		Level:  slogLevelToMCP(r.Level),
		Data:   data,
	}
	// We pass the argument context to Notify, even though slog.Handler.Handle's
	// documentation says not to.
	// In this case logging is a service to clients, not a means for debugging the
	// server, so we want to cancel the log message.
	return h.ss.Log(ctx, params)
}
