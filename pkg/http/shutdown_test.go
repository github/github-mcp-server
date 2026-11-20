package http

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

const testShutdownTimeout = time.Second

func TestServeHTTPDrainsActiveRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener, client := newShutdownPipeListener(t)
		started := make(chan struct{})
		release := make(chan struct{})
		server := &http.Server{
			ReadHeaderTimeout: time.Second,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(started)
				select {
				case <-release:
					_, _ = io.WriteString(w, "finished")
				case <-r.Context().Done():
				}
			}),
		}
		defer finishFailedShutdownTest(t, server)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		serveResult := make(chan error, 1)
		go func() {
			serveResult <- serveHTTP(ctx, server, listener, testShutdownTimeout, slog.New(slog.DiscardHandler))
		}()
		requestResult := readShutdownResponse(client)
		<-started

		cancel()
		<-listener.closed
		synctest.Wait()
		select {
		case err := <-serveResult:
			t.Fatalf("server returned while the request was active: %v", err)
		case response := <-requestResult:
			t.Fatalf("request ended before its handler was released: %+v", response)
		default:
		}

		close(release)
		response := <-requestResult
		require.NoError(t, response.err)
		require.Equal(t, http.StatusOK, response.status)
		require.Equal(t, "finished", response.body)
		require.NoError(t, <-serveResult)
	})
}

func TestServeHTTPClosesActiveConnectionOnTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener, client := newShutdownPipeListener(t)
		started := make(chan struct{})
		requestCanceled := make(chan error, 1)
		server := &http.Server{
			ReadHeaderTimeout: time.Second,
			Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				close(started)
				<-r.Context().Done()
				requestCanceled <- r.Context().Err()
			}),
		}
		defer finishFailedShutdownTest(t, server)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		serveResult := make(chan error, 1)
		go func() {
			serveResult <- serveHTTP(ctx, server, listener, testShutdownTimeout, slog.New(slog.DiscardHandler))
		}()
		requestResult := readShutdownResponse(client)
		<-started

		cancel()
		<-listener.closed
		require.ErrorIs(t, <-serveResult, context.DeadlineExceeded)

		synctest.Wait()
		select {
		case err := <-requestCanceled:
			require.ErrorIs(t, err, context.Canceled)
		default:
			t.Fatal("request context is still active after shutdown timed out")
		}
		select {
		case response := <-requestResult:
			require.Error(t, response.err, "client received an HTTP response after shutdown timed out")
		default:
			t.Fatal("client connection is still open after shutdown timed out")
		}
	})
}

func TestServeHTTPReturnsListenerError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := &shutdownPipeListener{closed: make(chan struct{})}
		require.NoError(t, listener.Close())
		server := &http.Server{ReadHeaderTimeout: time.Second}

		err := serveHTTP(t.Context(), server, listener, time.Second, slog.New(slog.DiscardHandler))

		require.ErrorIs(t, err, net.ErrClosed)
	})
}

func TestServeHTTPClosesListenerWhenAlreadyCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := &shutdownPipeListener{closed: make(chan struct{})}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		server := &http.Server{ReadHeaderTimeout: time.Second}
		err := serveHTTP(ctx, server, listener, time.Second, slog.New(slog.DiscardHandler))
		require.NoError(t, err)
		select {
		case <-listener.closed:
		default:
			t.Fatal("listener is still open after shutdown")
		}
	})
}

func finishFailedShutdownTest(t *testing.T, server *http.Server) {
	if t.Failed() {
		_ = server.Close()
		// Let a detached Shutdown goroutine finish before leaving the synctest bubble.
		time.Sleep(testShutdownTimeout)
	}
}

type shutdownPipeListener struct {
	pending chan net.Conn
	closed  chan struct{}
	once    sync.Once
}

// In-memory connections allow synctest to observe blocked HTTP I/O.
func newShutdownPipeListener(t *testing.T) (*shutdownPipeListener, net.Conn) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	listener := &shutdownPipeListener{
		pending: make(chan net.Conn, 1),
		closed:  make(chan struct{}),
	}
	listener.pending <- server
	return listener, client
}

func (l *shutdownPipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.pending:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *shutdownPipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (*shutdownPipeListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
}

type shutdownResponse struct {
	status int
	body   string
	err    error
}

func readShutdownResponse(conn net.Conn) <-chan shutdownResponse {
	result := make(chan shutdownResponse, 1)
	go func() {
		_, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
		if err != nil {
			result <- shutdownResponse{err: err}
			return
		}
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			result <- shutdownResponse{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		result <- shutdownResponse{status: resp.StatusCode, body: string(body), err: err}
	}()
	return result
}
