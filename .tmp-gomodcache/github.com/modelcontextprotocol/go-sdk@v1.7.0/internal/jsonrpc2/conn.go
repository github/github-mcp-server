// Copyright 2018 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonrpc2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/internal/json"
	"github.com/modelcontextprotocol/go-sdk/internal/mcpgodebug"
)

// nomethodnotfoundcodeinerror is a compatibility parameter that restores the
// pre-fix behavior of [processResult], where wrapped [ErrNotHandled] or
// [ErrMethodNotFound] errors returned by request handlers were not
// recognized as "method not found" signals. The original switch statement
// compared sentinel errors with ==, which never matched errors returned via
// fmt.Errorf("%w: ...", ErrNotHandled, ...) — including the ones produced
// by checkRequest. As a result the wire error response carried code 0
// instead of code -32601. The fix uses errors.Is to recognize wrapped
// sentinels and append the method name to the message.
//
// To restore the previous behavior, set MCPGODEBUG=nomethodnotfoundcodeinerror=1.
// This option will be removed in a future SDK version.
// See the documentation for the mcpgodebug package for instructions on how
// to use it.
var nomethodnotfoundcodeinerror = mcpgodebug.Value("nomethodnotfoundcodeinerror")

// Connection manages the jsonrpc2 protocol, connecting responses back to their
// calls. Connection is bidirectional; it does not have a designated server or
// client end.
//
// Note that the word 'Connection' is overloaded: the mcp.Connection represents
// the bidirectional stream of messages between client an server. The
// jsonrpc2.Connection layers RPC logic on top of that stream, dispatching RPC
// handlers, and correlating requests with responses from the peer.
//
// Some of the complexity of the Connection type is grown out of its usage in
// gopls: it could probably be simplified based on our usage in MCP.
type Connection struct {
	seq int64 // must only be accessed using atomic operations

	stateMu sync.Mutex
	state   inFlightState // accessed only in updateInFlight
	done    chan struct{} // closed (under stateMu) when state.closed is true and all goroutines have completed

	writer  Writer
	handler Handler

	onInternalError func(error)
	onDone          func()
}

// inFlightState records the state of the incoming and outgoing calls on a
// Connection.
type inFlightState struct {
	connClosing bool  // true when the Connection's Close method has been called
	reading     bool  // true while the readIncoming goroutine is running
	readErr     error // non-nil when the readIncoming goroutine exits (typically io.EOF)
	writeErr    error // non-nil if a call to the Writer has failed with a non-canceled Context

	// closer shuts down and cleans up the Reader and Writer state, ideally
	// interrupting any Read or Write call that is currently blocked. It is closed
	// when the state is idle and one of: connClosing is true, readErr is non-nil,
	// or writeErr is non-nil.
	//
	// After the closer has been invoked, the closer field is set to nil
	// and the closeErr field is simultaneously set to its result.
	closer   io.Closer
	closeErr error // error returned from closer.Close

	outgoingCalls         map[ID]*AsyncCall // calls only
	outgoingNotifications int               // # of notifications awaiting "write"

	// incoming stores the total number of incoming calls and notifications
	// that have not yet written or processed a result.
	incoming int

	incomingByID map[ID]*incomingRequest // calls only

	// handlerQueue stores the backlog of calls and notifications that were not
	// already handled by a preempter.
	// The queue does not include the request currently being handled (if any).
	handlerQueue   []*incomingRequest
	handlerRunning bool
}

// updateInFlight locks the state of the connection's in-flight requests, allows
// f to mutate that state, and closes the connection if it is idle and either
// is closing or has a read or write error.
func (c *Connection) updateInFlight(f func(*inFlightState)) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	s := &c.state

	f(s)

	select {
	case <-c.done:
		// The connection was already completely done at the start of this call to
		// updateInFlight, so it must remain so. (The call to f should have noticed
		// that and avoided making any updates that would cause the state to be
		// non-idle.)
		if !s.idle() {
			panic("jsonrpc2: updateInFlight transitioned to non-idle when already done")
		}
		return
	default:
	}

	if s.idle() && s.shuttingDown(ErrUnknown) != nil {
		if s.closer != nil {
			s.closeErr = s.closer.Close()
			s.closer = nil // prevent duplicate Close calls
		}
		if s.reading {
			// The readIncoming goroutine is still running. Our call to Close should
			// cause it to exit soon, at which point it will make another call to
			// updateInFlight, set s.reading to false, and mark the Connection done.
		} else {
			// The readIncoming goroutine has exited, or never started to begin with.
			// Since everything else is idle, we're completely done.
			if c.onDone != nil {
				c.onDone()
			}
			close(c.done)
		}
	}
}

// idle reports whether the connection is in a state with no pending calls or
// notifications.
//
// If idle returns true, the readIncoming goroutine may still be running,
// but no other goroutines are doing work on behalf of the connection.
func (s *inFlightState) idle() bool {
	return len(s.outgoingCalls) == 0 && s.outgoingNotifications == 0 && s.incoming == 0 && !s.handlerRunning
}

// shuttingDown reports whether the connection is in a state that should
// disallow new (incoming and outgoing) calls. It returns either nil or
// an error that is or wraps the provided errClosing.
func (s *inFlightState) shuttingDown(errClosing error) error {
	if s.connClosing {
		// If Close has been called explicitly, it doesn't matter what state the
		// Reader and Writer are in: we shouldn't be starting new work because the
		// caller told us not to start new work.
		return errClosing
	}
	if s.readErr != nil {
		// If the read side of the connection is broken, we cannot read new call
		// requests, and cannot read responses to our outgoing calls.
		return fmt.Errorf("%w: %v", errClosing, s.readErr)
	}
	if s.writeErr != nil {
		// If the write side of the connection is broken, we cannot write responses
		// for incoming calls, and cannot write requests for outgoing calls.
		return fmt.Errorf("%w: %v", errClosing, s.writeErr)
	}
	return nil
}

// incomingRequest is used to track an incoming request as it is being handled
type incomingRequest struct {
	*Request // the request being processed
	ctx      context.Context
	cancel   context.CancelFunc
}

// Reader abstracts the transport mechanics from the JSON RPC protocol.
// A Connection reads messages from the reader it was provided on construction,
// and assumes that each call to Read fully transfers a single message,
// or returns an error.
//
// A reader is not safe for concurrent use, it is expected it will be used by
// a single Connection in a safe manner.
type Reader interface {
	// Read gets the next message from the stream.
	Read(context.Context) (Message, error)
}

// Writer abstracts the transport mechanics from the JSON RPC protocol.
// A Connection writes messages using the writer it was provided on construction,
// and assumes that each call to Write fully transfers a single message,
// or returns an error.
//
// A writer must be safe for concurrent use, as writes may occur concurrently
// in practice: libraries may make calls or respond to requests asynchronously.
type Writer interface {
	// Write sends a message to the stream.
	Write(context.Context, Message) error
}

// A ConnectionConfig configures a bidirectional jsonrpc2 connection.
type ConnectionConfig struct {
	Reader          Reader                    // required
	Writer          Writer                    // required
	Closer          io.Closer                 // required
	Preempter       Preempter                 // optional
	Bind            func(*Connection) Handler // required
	OnDone          func()                    // optional
	OnInternalError func(error)               // optional

	// PropagateCancellation controls whether cancellation of the context
	// passed to [NewConnection] is observable by request handlers.
	//
	// By default (false), the connection wraps that context (see [notDone])
	// so handlers' Done channels do not fire when the connection's root
	// context is cancelled. Cancellation of an in-flight handler is then
	// expected to flow only through the jsonrpc2 layer's explicit channels
	// (the [Preempter] reacting to the peer's cancel notification, or a
	// transport read/write failure cancelling every in-flight request).
	//
	// Set this true when cancellation of the connection's context is itself
	// a meaningful signal that handlers should react to (for example, when
	// the connection is tied to a carrier that owns it and whose end means
	// the request is cancelled).
	PropagateCancellation bool // optional
}

// NewConnection creates a new [Connection] object and starts processing
// incoming messages.
func NewConnection(ctx context.Context, cfg ConnectionConfig) *Connection {
	if !cfg.PropagateCancellation {
		ctx = notDone{ctx}
	}

	c := &Connection{
		state:           inFlightState{closer: cfg.Closer},
		done:            make(chan struct{}),
		writer:          cfg.Writer,
		onDone:          cfg.OnDone,
		onInternalError: cfg.OnInternalError,
	}
	c.handler = cfg.Bind(c)
	c.start(ctx, cfg.Reader, cfg.Preempter)
	return c
}

func (c *Connection) start(ctx context.Context, reader Reader, preempter Preempter) {
	c.updateInFlight(func(s *inFlightState) {
		select {
		case <-c.done:
			// The connection was already closed; don't start a goroutine to read it.
			return
		default:
		}

		// The goroutine started here will continue until the underlying stream is closed.
		s.reading = true
		go c.readIncoming(ctx, reader, preempter)
	})
}

// Notify invokes the target method but does not wait for a response.
// The params will be marshaled to JSON before sending over the wire, and will
// be handed to the method invoked.
func (c *Connection) Notify(ctx context.Context, method string, params any) (err error) {
	attempted := false

	defer func() {
		if attempted {
			c.updateInFlight(func(s *inFlightState) {
				s.outgoingNotifications--
			})
		}
	}()

	c.updateInFlight(func(s *inFlightState) {
		// If the connection is shutting down, allow outgoing notifications only if
		// there is at least one call still in flight. The number of calls in flight
		// cannot increase once shutdown begins, and allowing outgoing notifications
		// may permit notifications that will cancel in-flight calls.
		if len(s.outgoingCalls) == 0 && len(s.incomingByID) == 0 {
			err = s.shuttingDown(ErrClientClosing)
			if err != nil {
				return
			}
		}
		s.outgoingNotifications++
		attempted = true
	})
	if err != nil {
		return err
	}

	notify, err := NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("marshaling notify parameters: %v", err)
	}

	return c.write(ctx, notify)
}

// Call invokes the target method and returns an object that can be used to await the response.
// The params will be marshaled to JSON before sending over the wire, and will
// be handed to the method invoked.
// You do not have to wait for the response, it can just be ignored if not needed.
// If sending the call failed, the response will be ready and have the error in it.
func (c *Connection) Call(ctx context.Context, method string, params any) *AsyncCall {
	// Generate a new request identifier.
	id := Int64ID(atomic.AddInt64(&c.seq, 1))

	ac := &AsyncCall{
		id:    id,
		ready: make(chan struct{}),
	}
	// When this method returns, either ac is retired, or the request has been
	// written successfully and the call is awaiting a response (to be provided by
	// the readIncoming goroutine).

	call, err := NewCall(ac.id, method, params)
	if err != nil {
		ac.retire(&Response{ID: id, Error: fmt.Errorf("marshaling call parameters: %w", err)})
		return ac
	}

	c.updateInFlight(func(s *inFlightState) {
		err = s.shuttingDown(ErrClientClosing)
		if err != nil {
			return
		}
		if s.outgoingCalls == nil {
			s.outgoingCalls = make(map[ID]*AsyncCall)
		}
		s.outgoingCalls[ac.id] = ac
	})
	if err != nil {
		ac.retire(&Response{ID: id, Error: err})
		return ac
	}

	if err := c.write(ctx, call); err != nil {
		// Sending failed. We will never get a response, so deliver a fake one if it
		// wasn't already retired by the connection breaking.
		c.Retire(ac, err)
	}
	return ac
}

// Retire stops tracking the call, and reports err as its terminal error.
//
// Retire is safe to call multiple times: if the call is already no longer
// tracked, Retire is a no op.
func (c *Connection) Retire(ac *AsyncCall, err error) {
	c.updateInFlight(func(s *inFlightState) {
		if s.outgoingCalls[ac.id] == ac {
			delete(s.outgoingCalls, ac.id)
			ac.retire(&Response{ID: ac.id, Error: err})
		} else {
			// ac was already retired elsewhere.
		}
	})
}

// Async, signals that the current jsonrpc2 request may be handled
// asynchronously to subsequent requests, when ctx is the request context.
//
// Async must be called at most once on each request's context (and its
// descendants).
func Async(ctx context.Context) {
	if r, ok := ctx.Value(asyncKey).(*releaser); ok {
		r.release(false)
	}
}

type asyncKeyType struct{}

var asyncKey = asyncKeyType{}

// A releaser implements concurrency safe 'releasing' of async requests. (A
// request is released when it is allowed to run concurrent with other
// requests, via a call to [Async].)
type releaser struct {
	mu       sync.Mutex
	ch       chan struct{}
	released bool
}

// release closes the associated channel. If soft is set, multiple calls to
// release are allowed.
func (r *releaser) release(soft bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.released {
		if !soft {
			panic("jsonrpc2.Async called multiple times")
		}
	} else {
		close(r.ch)
		r.released = true
	}
}

type AsyncCall struct {
	id       ID
	ready    chan struct{} // closed after response has been set
	response *Response
}

// ID used for this call.
// This can be used to cancel the call if needed.
func (ac *AsyncCall) ID() ID { return ac.id }

// retire processes the response to the call.
//
// It is an error to call retire more than once: retire is guarded by the
// connection's outgoingCalls map.
func (ac *AsyncCall) retire(response *Response) {
	select {
	case <-ac.ready:
		panic(fmt.Sprintf("jsonrpc2: retire called twice for ID %v", ac.id))
	default:
	}

	ac.response = response
	close(ac.ready)
}

// Await waits for (and decodes) the results of a Call.
// The response will be unmarshaled from JSON into the result.
//
// If the call is cancelled due to context cancellation, the result is
// ctx.Err().
func (ac *AsyncCall) Await(ctx context.Context, result any) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ac.ready:
	}
	if ac.response.Error != nil {
		return ac.response.Error
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(ac.response.Result, result)
}

// Cancel cancels the Context passed to the Handle call for the inbound message
// with the given ID.
//
// Cancel will not complain if the ID is not a currently active message, and it
// will not cause any messages that have not arrived yet with that ID to be
// cancelled.
func (c *Connection) Cancel(id ID) {
	var req *incomingRequest
	c.updateInFlight(func(s *inFlightState) {
		req = s.incomingByID[id]
	})
	if req != nil {
		req.cancel()
	}
}

// Wait blocks until the connection is fully closed, but does not close it.
func (c *Connection) Wait() error {
	return c.wait(true)
}

// wait for the connection to close, and aggregates the most cause of its
// termination, if abnormal.
//
// The fromWait argument allows this logic to be shared with Close, where we
// only want to expose the closeErr.
//
// (Previously, Wait also only returned the closeErr, which was misleading if
// the connection was broken for another reason).
func (c *Connection) wait(fromWait bool) error {
	var err error
	<-c.done
	c.updateInFlight(func(s *inFlightState) {
		if fromWait {
			if !errors.Is(s.readErr, io.EOF) {
				err = s.readErr
			}
			if err == nil && !errors.Is(s.writeErr, io.EOF) {
				err = s.writeErr
			}
		}
		if err == nil {
			err = s.closeErr
		}
	})
	return err
}

// Close stops accepting new requests, waits for in-flight requests and enqueued
// Handle calls to complete, and then closes the underlying stream.
//
// After the start of a Close, notification requests (that lack IDs and do not
// receive responses) will continue to be passed to the Preempter, but calls
// with IDs will receive immediate responses with ErrServerClosing, and no new
// requests (not even notifications!) will be enqueued to the Handler.
func (c *Connection) Close() error {
	// Stop handling new requests, and interrupt the reader (by closing the
	// connection) as soon as the active requests finish.
	c.updateInFlight(func(s *inFlightState) { s.connClosing = true })
	return c.wait(false)
}

// readIncoming collects inbound messages from the reader and delivers them, either responding
// to outgoing calls or feeding requests to the queue.
func (c *Connection) readIncoming(ctx context.Context, reader Reader, preempter Preempter) {
	var err error
	for {
		var msg Message
		msg, err = reader.Read(ctx)
		if err != nil {
			break
		}

		switch msg := msg.(type) {
		case *Request:
			c.acceptRequest(ctx, msg, preempter)

		case *Response:
			c.updateInFlight(func(s *inFlightState) {
				if ac, ok := s.outgoingCalls[msg.ID]; ok {
					delete(s.outgoingCalls, msg.ID)
					ac.retire(msg)
				} else {
					// TODO: How should we report unexpected responses?
				}
			})

		default:
			c.internalErrorf("Read returned an unexpected message of type %T", msg)
		}
	}

	c.updateInFlight(func(s *inFlightState) {
		s.reading = false
		s.readErr = err

		// Retire any outgoing requests that were still in flight: with the Reader no
		// longer being processed, they necessarily cannot receive a response.
		for id, ac := range s.outgoingCalls {
			ac.retire(&Response{ID: id, Error: err})
		}
		s.outgoingCalls = nil

		// Cancel any incoming requests still in flight: with the reader gone we
		// cannot receive cancellation notifications, and likely cannot write a
		// response either, so parked handlers have nothing useful left to do.
		// Mirrors the equivalent cleanup on write failure.
		for _, r := range s.incomingByID {
			r.cancel()
		}
	})
}

// acceptRequest either handles msg synchronously or enqueues it to be handled
// asynchronously.
func (c *Connection) acceptRequest(ctx context.Context, msg *Request, preempter Preempter) {
	// In theory notifications cannot be cancelled, but we build them a cancel
	// context anyway.
	reqCtx, cancel := context.WithCancel(ctx)
	req := &incomingRequest{
		Request: msg,
		ctx:     reqCtx,
		cancel:  cancel,
	}

	// If the request is a call, add it to the incoming map so it can be
	// cancelled (or responded) by ID.
	var err error
	c.updateInFlight(func(s *inFlightState) {
		s.incoming++

		if req.IsCall() {
			if s.incomingByID[req.ID] != nil {
				err = fmt.Errorf("%w: request ID %v already in use", ErrInvalidRequest, req.ID)
				req.ID = ID{} // Don't misattribute this error to the existing request.
				return
			}

			if s.incomingByID == nil {
				s.incomingByID = make(map[ID]*incomingRequest)
			}
			s.incomingByID[req.ID] = req

			// When shutting down, reject all new Call requests, even if they could
			// theoretically be handled by the preempter. The preempter could return
			// ErrAsyncResponse, which would increase the amount of work in flight
			// when we're trying to ensure that it strictly decreases.
			err = s.shuttingDown(ErrServerClosing)
		}
	})
	if err != nil {
		c.processResult("acceptRequest", req, nil, err)
		return
	}

	if preempter != nil {
		result, err := preempter.Preempt(req.ctx, req.Request)

		if !errors.Is(err, ErrNotHandled) {
			c.processResult("Preempt", req, result, err)
			return
		}
	}

	c.updateInFlight(func(s *inFlightState) {
		// If the connection is shutting down, don't enqueue anything to the
		// handler — not even notifications. That ensures that if the handler
		// continues to make progress, it will eventually become idle and
		// close the connection.
		err = s.shuttingDown(ErrServerClosing)
		if err != nil {
			return
		}

		// We enqueue requests that have not been preempted to an unbounded slice.
		// Unfortunately, we cannot in general limit the size of the handler
		// queue: we have to read every response that comes in on the wire
		// (because it may be responding to a request issued by, say, an
		// asynchronous handler), and in order to get to that response we have
		// to read all of the requests that came in ahead of it.
		s.handlerQueue = append(s.handlerQueue, req)
		if !s.handlerRunning {
			// We start the handleAsync goroutine when it has work to do, and let it
			// exit when the queue empties.
			//
			// Otherwise, in order to synchronize the handler we would need some other
			// goroutine (probably readIncoming?) to explicitly wait for handleAsync
			// to finish, and that would complicate error reporting: either the error
			// report from the goroutine would be blocked on the handler emptying its
			// queue (which was tried, and introduced a deadlock detected by
			// TestCloseCallRace), or the error would need to be reported separately
			// from synchronizing completion. Allowing the handler goroutine to exit
			// when idle seems simpler than trying to implement either of those
			// alternatives correctly.
			s.handlerRunning = true
			go c.handleAsync()
		}
	})
	if err != nil {
		c.processResult("acceptRequest", req, nil, err)
	}
}

// handleAsync invokes the handler on the requests in the handler queue
// sequentially until the queue is empty.
func (c *Connection) handleAsync() {
	for {
		var req *incomingRequest
		c.updateInFlight(func(s *inFlightState) {
			if len(s.handlerQueue) > 0 {
				req, s.handlerQueue = s.handlerQueue[0], s.handlerQueue[1:]
			} else {
				s.handlerRunning = false
			}
		})
		if req == nil {
			return
		}

		// Only deliver to the Handler if not already canceled.
		if err := req.ctx.Err(); err != nil {
			c.updateInFlight(func(s *inFlightState) {
				if s.writeErr != nil {
					// Assume that req.ctx was canceled due to s.writeErr.
					// TODO(#51365): use a Context API to plumb this through req.ctx.
					err = fmt.Errorf("%w: %v", ErrServerClosing, s.writeErr)
				}
			})
			c.processResult("handleAsync", req, nil, err)
			continue
		}

		releaser := &releaser{ch: make(chan struct{})}
		ctx := context.WithValue(req.ctx, asyncKey, releaser)
		go func() {
			defer releaser.release(true)
			result, err := c.handler.Handle(ctx, req.Request)
			c.processResult(c.handler, req, result, err)
		}()
		<-releaser.ch
	}
}

// processResult processes the result of a request and, if appropriate, sends a response.
func (c *Connection) processResult(from any, req *incomingRequest, result any, err error) error {
	if nomethodnotfoundcodeinerror != "1" && (errors.Is(err, ErrNotHandled) || errors.Is(err, ErrMethodNotFound)) {
		err = fmt.Errorf("%w: %q", ErrMethodNotFound, req.Method)
	}

	if result != nil && err != nil {
		c.internalErrorf("%#v returned a non-nil result with a non-nil error for %s:\n%v\n%#v", from, req.Method, err, result)
		result = nil // Discard the spurious result and respond with err.
	}

	if req.IsCall() {
		if result == nil && err == nil {
			err = c.internalErrorf("%#v returned a nil result and nil error for a %q Request that requires a Response", from, req.Method)
		}

		response, respErr := NewResponse(req.ID, result, err)

		// The caller could theoretically reuse the request's ID as soon as we've
		// sent the response, so ensure that it is removed from the incoming map
		// before sending.
		c.updateInFlight(func(s *inFlightState) {
			delete(s.incomingByID, req.ID)
		})
		if respErr == nil {
			writeErr := c.write(notDone{req.ctx}, response)
			if err == nil {
				err = writeErr
			}
		} else {
			err = c.internalErrorf("%#v returned a malformed result for %q: %w", from, req.Method, respErr)
		}
	} else { // req is a notification
		if result != nil {
			err = c.internalErrorf("%#v returned a non-nil result for a %q Request without an ID", from, req.Method)
		} else if err != nil {
			err = fmt.Errorf("%w: %q notification failed: %v", ErrInternal, req.Method, err)
		}
	}
	if err != nil {
		// TODO: can/should we do anything with this error beyond writing it to the event log?
		// (Is this the right label to attach to the log?)
	}

	// Cancel the request to free any associated resources.
	req.cancel()
	c.updateInFlight(func(s *inFlightState) {
		if s.incoming == 0 {
			panic("jsonrpc2: processResult called when incoming count is already zero")
		}
		s.incoming--
	})
	return nil
}

// write is used by all things that write outgoing messages, including replies.
// it makes sure that writes are atomic
func (c *Connection) write(ctx context.Context, msg Message) error {
	var err error
	// Fail writes immediately if the connection is shutting down.
	//
	// Allow outgoing "notifications" forwarded by the Notify method.
	// This will allow to send the cancelled notification when the client is shutting down.
	c.updateInFlight(func(s *inFlightState) {
		if req, ok := msg.(*Request); ok && !req.IsCall() && s.outgoingNotifications > 0 {
			return
		}
		err = s.shuttingDown(ErrServerClosing)
	})
	if err == nil {
		err = c.writer.Write(ctx, msg)
	}

	// For cancelled or rejected requests, we don't set the writeErr (which would
	// break the connection). They can just be returned to the caller.
	if err != nil && ctx.Err() == nil && !errors.Is(err, ErrRejected) {
		// The call to Write failed, and since ctx.Err() is nil we can't attribute
		// the failure (even indirectly) to Context cancellation. The writer appears
		// to be broken, and future writes are likely to also fail.
		//
		// If the read side of the connection is also broken, we might not even be
		// able to receive cancellation notifications. Since we can't reliably write
		// the results of incoming calls and can't receive explicit cancellations,
		// cancel the calls now.
		c.updateInFlight(func(s *inFlightState) {
			if s.writeErr == nil {
				s.writeErr = err
				for _, r := range s.incomingByID {
					r.cancel()
				}
			}
		})
	}

	return err
}

// internalErrorf reports an internal error. By default it panics, but if
// c.onInternalError is non-nil it instead calls that and returns an error
// wrapping ErrInternal.
func (c *Connection) internalErrorf(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	if c.onInternalError == nil {
		panic("jsonrpc2: " + err.Error())
	}
	c.onInternalError(err)

	return fmt.Errorf("%w: %v", ErrInternal, err)
}

// notDone is a context.Context wrapper that returns a nil Done channel.
//
// Request handlers' contexts are derived from the connection's root context,
// which by default is wrapped in notDone so a transport-level cancellation
// does not implicitly cancel every in-flight handler. Cancellation of an
// in-flight handler is instead expected to flow only through the jsonrpc2
// layer's explicit channels: the [Preempter] calling [Connection.Cancel] in
// response to the peer's cancel notification, or the transport itself
// failing (the read loop exits on EOF or a write fails) — both of which
// cancel every in-flight incoming request in turn.
type notDone struct{ ctx context.Context }

func (ic notDone) Value(key any) any {
	return ic.ctx.Value(key)
}

func (notDone) Done() <-chan struct{}       { return nil }
func (notDone) Err() error                  { return nil }
func (notDone) Deadline() (time.Time, bool) { return time.Time{}, false }
