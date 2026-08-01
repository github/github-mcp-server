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

func TestIssuesService_ListMilestones(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/milestones", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"state":     "closed",
			"sort":      "due_date",
			"direction": "asc",
			"page":      "2",
		})
		fmt.Fprint(w, `[{"number":1}]`)
	})

	opt := &MilestoneListOptions{"closed", "due_date", "asc", ListOptions{Page: 2}}
	ctx := t.Context()
	milestones, _, err := client.Issues.ListMilestones(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("IssuesService.ListMilestones returned error: %v", err)
	}

	want := []*Milestone{{Number: Ptr(1)}}
	if !cmp.Equal(milestones, want) {
		t.Errorf("IssuesService.ListMilestones returned %+v, want %+v", milestones, want)
	}

	const methodName = "ListMilestones"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListMilestones(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListMilestones(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListMilestones_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ListMilestones(ctx, "%", "r", nil)
	testURLParseError(t, err)
}

func TestIssuesService_GetMilestone(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/milestones/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"number":1}`)
	})

	ctx := t.Context()
	milestone, _, err := client.Issues.GetMilestone(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("IssuesService.GetMilestone returned error: %v", err)
	}

	want := &Milestone{Number: Ptr(1)}
	if !cmp.Equal(milestone, want) {
		t.Errorf("IssuesService.GetMilestone returned %+v, want %+v", milestone, want)
	}

	const methodName = "GetMilestone"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.GetMilestone(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.GetMilestone(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_GetMilestone_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.GetMilestone(ctx, "%", "r", 1)
	testURLParseError(t, err)
}

func TestIssuesService_CreateMilestone(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Milestone{Title: Ptr("t")}

	mux.HandleFunc("/repos/o/r/milestones", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":1}`)
	})

	ctx := t.Context()
	milestone, _, err := client.Issues.CreateMilestone(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("IssuesService.CreateMilestone returned error: %v", err)
	}

	want := &Milestone{Number: Ptr(1)}
	if !cmp.Equal(milestone, want) {
		t.Errorf("IssuesService.CreateMilestone returned %+v, want %+v", milestone, want)
	}

	const methodName = "CreateMilestone"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.CreateMilestone(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.CreateMilestone(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_CreateMilestone_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.CreateMilestone(ctx, "%", "r", nil)
	testURLParseError(t, err)
}

func TestIssuesService_EditMilestone(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &Milestone{Title: Ptr("t")}

	mux.HandleFunc("/repos/o/r/milestones/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":1}`)
	})

	ctx := t.Context()
	milestone, _, err := client.Issues.EditMilestone(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("IssuesService.EditMilestone returned error: %v", err)
	}

	want := &Milestone{Number: Ptr(1)}
	if !cmp.Equal(milestone, want) {
		t.Errorf("IssuesService.EditMilestone returned %+v, want %+v", milestone, want)
	}

	const methodName = "EditMilestone"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.EditMilestone(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.EditMilestone(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_EditMilestone_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.EditMilestone(ctx, "%", "r", 1, nil)
	testURLParseError(t, err)
}

func TestIssuesService_DeleteMilestone(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/milestones/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Issues.DeleteMilestone(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("IssuesService.DeleteMilestone returned error: %v", err)
	}

	const methodName = "DeleteMilestone"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Issues.DeleteMilestone(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Issues.DeleteMilestone(ctx, "o", "r", 1)
	})
}

func TestIssuesService_DeleteMilestone_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Issues.DeleteMilestone(ctx, "%", "r", 1)
	testURLParseError(t, err)
}
