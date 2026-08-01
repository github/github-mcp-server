// Copyright 2017 The go-github AUTHORS. All rights reserved.
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

func TestRequestReviewers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ReviewersRequest{Reviewers: []string{"octocat", "googlebot"}, TeamReviewers: []string{"justice-league", "injustice-league"}}

	mux.HandleFunc("/repos/o/r/pulls/1/requested_reviewers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":1}`)
	})

	// This returns a PR, unmarshaling of which is tested elsewhere
	ctx := t.Context()
	got, _, err := client.PullRequests.RequestReviewers(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("PullRequests.RequestReviewers returned error: %v", err)
	}
	want := &PullRequest{Number: Ptr(1)}
	if !cmp.Equal(got, want) {
		t.Errorf("PullRequests.RequestReviewers returned %+v, want %+v", got, want)
	}

	const methodName = "RequestReviewers"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PullRequests.RequestReviewers(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRemoveReviewers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ReviewersRequest{Reviewers: []string{"octocat", "googlebot"}, TeamReviewers: []string{"justice-league"}}

	mux.HandleFunc("/repos/o/r/pulls/1/requested_reviewers", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, input)
	})

	ctx := t.Context()
	_, err := client.PullRequests.RemoveReviewers(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("PullRequests.RemoveReviewers returned error: %v", err)
	}

	const methodName = "RemoveReviewers"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.PullRequests.RemoveReviewers(ctx, "o", "r", 1, input)
	})
}

func TestRemoveReviewers_teamsOnly(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ReviewersRequest{TeamReviewers: []string{"justice-league"}}

	mux.HandleFunc("/repos/o/r/pulls/1/requested_reviewers", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		want := ReviewersRequest{
			NodeID:        nil,
			Reviewers:     []string{},
			TeamReviewers: input.TeamReviewers,
		}
		testJSONBody(t, r, want)
	})

	ctx := t.Context()
	_, err := client.PullRequests.RemoveReviewers(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("PullRequests.RemoveReviewers returned error: %v", err)
	}

	const methodName = "RemoveReviewers"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.PullRequests.RemoveReviewers(ctx, "o", "r", 1, input)
	})
}

func TestListReviewers(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/pulls/1/requested_reviewers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"users":[{"login":"octocat","id":1}],"teams":[{"id":1,"name":"Justice League"}]}`)
	})

	ctx := t.Context()
	got, _, err := client.PullRequests.ListReviewers(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("PullRequests.ListReviewers returned error: %v", err)
	}

	want := &Reviewers{
		Users: []*User{
			{
				Login: Ptr("octocat"),
				ID:    Ptr(int64(1)),
			},
		},
		Teams: []*Team{
			{
				ID:   Ptr(int64(1)),
				Name: Ptr("Justice League"),
			},
		},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("PullRequests.ListReviewers returned %+v, want %+v", got, want)
	}

	const methodName = "ListReviewers"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.PullRequests.ListReviewers(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
