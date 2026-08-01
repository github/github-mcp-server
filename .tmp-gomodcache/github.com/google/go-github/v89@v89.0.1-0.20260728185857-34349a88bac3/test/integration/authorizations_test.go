// Copyright 2016 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
)

const (
	msgEnvMissing      = "Skipping test because the required environment variable (%v) is not present."
	envKeyClientID     = "GITHUB_CLIENT_ID"
	envKeyClientSecret = "GITHUB_CLIENT_SECRET"
	envKeyAccessToken  = "GITHUB_ACCESS_TOKEN"
	InvalidTokenValue  = "iamnotacroken"
)

// TestAuthorizationsAppOperations tests the application/token related operations, such
// as creating, testing, resetting and revoking application OAuth tokens.
func TestAuthorizationsAppOperations(t *testing.T) {
	appAuthenticatedClient := getOAuthAppClient(t)

	// We know these vars are set because getOAuthAppClient would have
	// skipped the test by now
	clientID := os.Getenv(envKeyClientID)
	accessToken := os.Getenv(envKeyAccessToken)

	// Verify the token
	appAuth, resp, err := appAuthenticatedClient.Authorizations.Check(t.Context(), clientID, accessToken)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	// Quick sanity check
	if *appAuth.Token != accessToken {
		t.Fatal("The returned auth/token does not match.")
	}

	// Let's verify that we get a 404 for a nonexistent token
	_, resp, err = appAuthenticatedClient.Authorizations.Check(t.Context(), clientID, InvalidTokenValue)
	if err == nil {
		t.Fatal("An error should have been returned because of the invalid token.")
	}
	failIfNotStatusCode(t, resp, 404)

	// Let's reset the token
	resetAuth, resp, err := appAuthenticatedClient.Authorizations.Reset(t.Context(), clientID, accessToken)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	// Let's verify that we get a 404 for a nonexistent token
	_, resp, err = appAuthenticatedClient.Authorizations.Reset(t.Context(), clientID, InvalidTokenValue)
	if err == nil {
		t.Fatal("An error should have been returned because of the invalid token.")
	}
	failIfNotStatusCode(t, resp, 404)

	// Verify that the token has changed
	if *resetAuth.Token == accessToken {
		t.Fatal("The reset token should be different from the original.")
	}

	// Verify that we do have a token value
	if *resetAuth.Token == "" {
		t.Fatal("A token value should have been returned.")
	}

	// Verify that the original token is now invalid
	_, resp, err = appAuthenticatedClient.Authorizations.Check(t.Context(), clientID, accessToken)
	if err == nil {
		t.Fatal("The original token should be invalid.")
	}
	failIfNotStatusCode(t, resp, 404)

	// Check that the reset token is valid
	_, resp, err = appAuthenticatedClient.Authorizations.Check(t.Context(), clientID, *resetAuth.Token)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 200)

	// Let's revoke the token
	resp, err = appAuthenticatedClient.Authorizations.Revoke(t.Context(), clientID, *resetAuth.Token)
	failOnError(t, err)
	failIfNotStatusCode(t, resp, 204)

	// Sleep for two seconds... I've seen cases where the revocation appears not
	// to have take place immediately.
	time.Sleep(time.Second * 2)

	// Now, the reset token should also be invalid
	_, resp, err = appAuthenticatedClient.Authorizations.Check(t.Context(), clientID, *resetAuth.Token)
	if err == nil {
		t.Fatal("The reset token should be invalid.")
	}
	failIfNotStatusCode(t, resp, 404)
}

// failOnError invokes t.Fatal() if err is present.
func failOnError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

// failIfNotStatusCode invokes t.Fatal() if the response's status code doesn't match the expected code.
func failIfNotStatusCode(t *testing.T, resp *github.Response, expectedCode int) {
	if resp.StatusCode != expectedCode {
		t.Fatalf("Expected HTTP status code [%v] but received [%v]", expectedCode, resp.StatusCode)
	}
}

// getOAuthAppClient returns a GitHub client for authorization testing. The client
// uses BasicAuth, but instead of username and password, it uses the client id
// and client secret passed in via environment variables
// (and will skip the calling test if those vars are not present). Certain API operations (check
// an authorization; reset an authorization; revoke an authorization for an app)
// require this authentication mechanism.
//
// See GitHub API docs: https://developer.github.com/v3/oauth_authorizations/#check-an-authorization
func getOAuthAppClient(t *testing.T) *github.Client {
	t.Helper()

	username, ok := os.LookupEnv(envKeyClientID)
	if !ok {
		t.Skipf(msgEnvMissing, envKeyClientID)
	}

	password, ok := os.LookupEnv(envKeyClientSecret)
	if !ok {
		t.Skipf(msgEnvMissing, envKeyClientSecret)
	}

	tp := github.BasicAuthTransport{
		Username: strings.TrimSpace(username),
		Password: strings.TrimSpace(password),
	}

	c, err := github.NewClient(github.WithHTTPClient(tp.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
