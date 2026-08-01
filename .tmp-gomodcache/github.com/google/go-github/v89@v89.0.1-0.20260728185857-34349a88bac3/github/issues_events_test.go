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

func TestIssuesService_ListIssueEvents(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/events", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeProjectCardDetailsPreview)
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "2",
		})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 1, PerPage: 2}
	ctx := t.Context()
	events, _, err := client.Issues.ListIssueEvents(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Issues.ListIssueEvents returned error: %v", err)
	}

	want := []*IssueEvent{{ID: Ptr(int64(1))}}
	if !cmp.Equal(events, want) {
		t.Errorf("Issues.ListIssueEvents returned %+v, want %+v", events, want)
	}

	const methodName = "ListIssueEvents"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListIssueEvents(ctx, "\n", "\n", -1, &ListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListIssueEvents(ctx, "o", "r", 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListRepositoryEvents(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/events", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "2",
		})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 1, PerPage: 2}
	ctx := t.Context()
	events, _, err := client.Issues.ListRepositoryEvents(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Issues.ListRepositoryEvents returned error: %v", err)
	}

	want := []*IssueEvent{{ID: Ptr(int64(1))}}
	if !cmp.Equal(events, want) {
		t.Errorf("Issues.ListRepositoryEvents returned %+v, want %+v", events, want)
	}

	const methodName = "ListRepositoryEvents"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListRepositoryEvents(ctx, "\n", "\n", &ListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListRepositoryEvents(ctx, "o", "r", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_GetEvent(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/events/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	event, _, err := client.Issues.GetEvent(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Issues.GetEvent returned error: %v", err)
	}

	want := &IssueEvent{ID: Ptr(int64(1))}
	if !cmp.Equal(event, want) {
		t.Errorf("Issues.GetEvent returned %+v, want %+v", event, want)
	}

	const methodName = "GetEvent"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.GetEvent(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.GetEvent(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
