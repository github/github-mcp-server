// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"errors"
	"net/http"
	"testing"
)

func TestCredentialsService_Revoke(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	creds := []string{
		"ghp_1234567890abcdef1234567890abcdef12345678",
		"ghp_abcdef1234567890abcdef1234567890abcdef12",
	}

	mux.HandleFunc("/credentials/revoke", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, &revokeCredentialsRequest{Credentials: creds})
		w.WriteHeader(http.StatusAccepted)
	})

	ctx := t.Context()
	resp, err := client.Credentials.Revoke(ctx, creds)
	if !errors.As(err, new(*AcceptedError)) {
		t.Errorf("Credentials.Revoke returned error: %v (want AcceptedError)", err)
	}
	if resp == nil {
		t.Fatal("Credentials.Revoke returned nil response")
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Credentials.Revoke returned status %v, want %v", resp.StatusCode, http.StatusAccepted)
	}

	const methodName = "Revoke"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Credentials.Revoke(ctx, []string{"a"})
	})
}
