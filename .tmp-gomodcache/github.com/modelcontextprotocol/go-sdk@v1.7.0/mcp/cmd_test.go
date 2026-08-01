// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp_test

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const runAsServer = "_MCP_RUN_AS_SERVER"

type SayHiParams struct {
	Name string `json:"name"`
}

func SayHi(ctx context.Context, req *mcp.CallToolRequest, args SayHiParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hi " + args.Name},
		},
	}, nil, nil
}

func TestMain(m *testing.M) {
	// If the runAsServer variable is set, execute the relevant serverFunc
	// instead of running tests (aka the fork and exec trick).
	if name := os.Getenv(runAsServer); name != "" {
		run := serverFuncs[name]
		if run == nil {
			log.Fatalf("Unknown server %q", name)
		}
		os.Unsetenv(runAsServer)
		run()
		return
	}
	os.Exit(m.Run())
}

// serverFuncs defines server functions that may be run as subprocesses via
// [TestMain].
var serverFuncs = map[string]func(){
	"default":       runServer,
	"cancelContext": runCancelContextServer,
}

func runServer() {
	ctx := context.Background()

	server := mcp.NewServer(testImpl, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func runCancelContextServer() {
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer done()

	server := mcp.NewServer(testImpl, nil)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func TestServerRunContextCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := mcp.NewServer(&mcp.Implementation{Name: "greeter", Version: "v0.0.1"}, nil)
		mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		serverTransport, clientTransport := mcp.NewInMemoryTransports()

		// run the server and capture the exit error
		onServerExit := make(chan error)
		go func() {
			onServerExit <- server.Run(ctx, serverTransport)
		}()

		// send a ping to the server to ensure it's running
		client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)
		session, err := client.Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { session.Close() })

		if err := session.Ping(context.Background(), nil); err != nil {
			t.Fatal(err)
		}

		// cancel the context to stop the server
		cancel()

		// wait for the server to exit

		err = <-onServerExit
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("server did not exit after context cancellation, got error: %v", err)
		}
	})
}

func TestServerInterrupt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX signals")
	}
	requireExec(t)

	t.Log("Starting server command")
	cmd := createServerCommand(t, "default")

	client := mcp.NewClient(testImpl, nil)
	t.Log("Connecting to server")

	ctx := context.Background()
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Send a signal to the server process to terminate it")
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}

	t.Log("Closing client session so server can exit immediately")
	session.Close()

	t.Log("Wait for process to terminate after interrupt signal")
	_, err = cmd.Process.Wait()
	if err == nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStdioContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX signals")
	}
	requireExec(t)

	// This test is a variant of TestServerInterrupt reproducing the conditions
	// of #224, where interrupt failed to shut down the server because reads of
	// Stdin were not unblocked.

	cmd := createServerCommand(t, "cancelContext")
	// Creating a stdin pipe causes os.Stdin.Close to not immediately unblock
	// pending reads.
	_, _ = cmd.StdinPipe()

	// Just Start the command, rather than connecting to the server, because we
	// don't want the client connection to indirectly flush stdin through writes.
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting command: %v", err)
	}

	// Sleep to make it more likely that the server is blocked in the read loop.
	//
	// This sleep isn't necessary for the test to pass, but *was* necessary for
	// it to fail, before closing was fixed. Unfortunately, it is too invasive a
	// change to have the jsonrpc2 package signal across packages when it is
	// actually blocked in its read loop.
	time.Sleep(100 * time.Millisecond)

	onExit := make(chan struct{})
	go func() {
		cmd.Process.Wait()
		close(onExit)
	}()

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("server did not exit after SIGINT")
	case <-onExit:
		t.Logf("done.")
	}
}

func TestCmdTransport(t *testing.T) {
	requireExec(t)

	ctx := t.Context()

	cmd := createServerCommand(t, "default")

	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "greet",
		Arguments: map[string]any{"name": "user"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hi user"},
		},
	}
	if diff := cmp.Diff(want, got, ctrCmpOpts...); diff != "" {
		t.Errorf("greet returned unexpected content (-want +got):\n%s", diff)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("closing server: %v", err)
	}
}

// createServerCommand creates a command to fork and exec the test binary as an
// MCP server.
//
// serverName must refer to an entry in the [serverFuncs] map.
func createServerCommand(t *testing.T, serverName string) *exec.Cmd {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), runAsServer+"="+serverName)

	return cmd
}

func TestCommandTransportTerminateDuration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX signals")
	}
	requireExec(t)

	// Unfortunately, since it does I/O, this test needs to rely on timing (we
	// can't use synctest). However, we can still decrease the default
	// termination duration to speed up the test.
	const defaultDur = 50 * time.Millisecond
	defer mcp.SetDefaultTerminateDuration(defaultDur)()

	tests := []struct {
		name            string
		duration        time.Duration
		wantMinDuration time.Duration
		wantMaxDuration time.Duration
	}{
		{
			name:            "default duration (zero)",
			duration:        0,
			wantMinDuration: defaultDur,
			wantMaxDuration: 1 * time.Second, // default + buffer
		},
		{
			name:            "below minimum duration",
			duration:        -500 * time.Millisecond,
			wantMinDuration: defaultDur,
			wantMaxDuration: 1 * time.Second, // should use default + buffer
		},
		{
			name:            "custom valid duration",
			duration:        200 * time.Millisecond,
			wantMinDuration: 200 * time.Millisecond,
			wantMaxDuration: 1 * time.Second, // custom + buffer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			// Use a command that won't exit when stdin is closed
			cmd := exec.Command("sleep", "20")
			transport := &mcp.CommandTransport{
				Command:           cmd,
				TerminateDuration: tt.duration,
			}

			conn, err := transport.Connect(ctx)
			if err != nil {
				t.Fatal(err)
			}

			start := time.Now()
			err = conn.Close()
			elapsed := time.Since(start)

			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("Close() failed with unexpected error: %v", err)
				}
			}
			if elapsed < tt.wantMinDuration {
				t.Errorf("Close() took %v, expected at least %v", elapsed, tt.wantMinDuration)
			}
			if elapsed > tt.wantMaxDuration {
				t.Errorf("Close() took %v, expected at most %v", elapsed, tt.wantMaxDuration)
			}

			// Ensure the process was actually terminated
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		})
	}
}

func requireExec(t *testing.T) {
	t.Helper()

	// Conservatively, limit to major OS where we know that os.Exec is
	// supported.
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
	default:
		t.Skip("unsupported OS")
	}
}

var testImpl = &mcp.Implementation{Name: "test", Version: "v1.0.0"}
