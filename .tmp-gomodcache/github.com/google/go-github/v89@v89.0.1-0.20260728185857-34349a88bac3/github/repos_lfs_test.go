// Copyright 2022 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"net/http"
	"testing"
)

func TestRepositoriesService_EnableLFS(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/lfs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Repositories.EnableLFS(ctx, "o", "r"); err != nil {
		t.Errorf("Repositories.EnableLFS returned error: %v", err)
	}

	const methodName = "EnableLFS"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.EnableLFS(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.EnableLFS(ctx, "o", "r")
	})
}

func TestRepositoriesService_DisableLFS(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/lfs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Repositories.DisableLFS(ctx, "o", "r"); err != nil {
		t.Errorf("Repositories.DisableLFS returned error: %v", err)
	}

	const methodName = "DisableLFS"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DisableLFS(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DisableLFS(ctx, "o", "r")
	})
}
