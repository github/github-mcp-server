// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRepositoriesService_Merge(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := RepositoryMergeRequest{
		Base:          "b",
		Head:          "h",
		CommitMessage: Ptr("c"),
	}

	mux.HandleFunc("/repos/o/r/merges", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"sha":"s"}`)
	})

	ctx := t.Context()
	commit, _, err := client.Repositories.Merge(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Repositories.Merge returned error: %v", err)
	}

	want := &RepositoryCommit{SHA: Ptr("s")}
	if !cmp.Equal(commit, want) {
		t.Errorf("Repositories.Merge returned %+v, want %+v", commit, want)
	}

	const methodName = "Merge"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.Merge(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.Merge(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_MergeUpstream(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := RepoMergeUpstreamRequest{
		Branch: "b",
	}

	mux.HandleFunc("/repos/o/r/merge-upstream", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"merge_type":"m"}`)
	})

	ctx := t.Context()
	result, _, err := client.Repositories.MergeUpstream(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Repositories.MergeUpstream returned error: %v", err)
	}

	want := &RepoMergeUpstreamResult{MergeType: Ptr("m")}
	if !cmp.Equal(result, want) {
		t.Errorf("Repositories.MergeUpstream returned %+v, want %+v", result, want)
	}

	const methodName = "MergeUpstream"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.MergeUpstream(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.MergeUpstream(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
