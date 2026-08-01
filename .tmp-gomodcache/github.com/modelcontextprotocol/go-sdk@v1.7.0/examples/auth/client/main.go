// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	// URL of the MCP server.
	serverURL = flag.String("server_url", "http://localhost:8000/mcp", "URL of the MCP server.")
	// Port for the local HTTP server that will receive the authorization code.
	callbackPort = flag.Int("callback_port", 3142, "Port for the local HTTP server that will receive the authorization code.")
)

type codeReceiver struct {
	authChan chan *auth.AuthorizationResult
	errChan  chan error
	server   *http.Server
}

func (r *codeReceiver) serveRedirectHandler(listener net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		r.authChan <- &auth.AuthorizationResult{
			Code:  req.URL.Query().Get("code"),
			State: req.URL.Query().Get("state"),
			Iss:   req.URL.Query().Get("iss"),
		}
		fmt.Fprint(w, "Authentication successful. You can close this window.")
	})

	r.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", *callbackPort),
		Handler: mux,
	}
	if err := r.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.errChan <- err
	}
}

func (r *codeReceiver) getAuthorizationCode(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	fmt.Printf("Please open the following URL in your browser: %s\n", args.URL)
	select {
	case authRes := <-r.authChan:
		return authRes, nil
	case err := <-r.errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *codeReceiver) close() {
	if r.server != nil {
		r.server.Close()
	}
}

func main() {
	flag.Parse()
	receiver := &codeReceiver{
		authChan: make(chan *auth.AuthorizationResult),
		errChan:  make(chan error),
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *callbackPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	go receiver.serveRedirectHandler(listener)
	defer receiver.close()

	authHandler, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              fmt.Sprintf("http://localhost:%d", *callbackPort),
		AuthorizationCodeFetcher: receiver.getAuthorizationCode,
		// Uncomment the client configuration you want to use.
		// PreregisteredClient: &oauthex.ClientCredentials{
		// 		ClientID:     "",
		// 		ClientSecretAuth: &oauthex.ClientSecretAuth{
		// 			ClientSecret: "",
		// 		},
		// 	},
		// },
		// DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
		// 	Metadata: &oauthex.ClientRegistrationMetadata{
		// 		ClientName: "Dynamically registered MCP client",
		// 		RedirectURIs: []string{fmt.Sprintf("http://localhost:%d", *callbackPort)},
		// 		Scope: "read",
		// 	},
		// },
	})
	if err != nil {
		log.Fatalf("failed to create auth handler: %v", err)
	}

	transport := &mcp.StreamableClientTransport{
		Endpoint:     *serverURL,
		OAuthHandler: authHandler,
	}

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("client.Connect(): %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("session.ListTools(): %v", err)
	}
	log.Println("Tools:")
	for _, tool := range tools.Tools {
		log.Printf("- %q", tool.Name)
	}
}
