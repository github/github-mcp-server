// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIssuesService_ListAllIssues(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{
			"filter":    "all",
			"state":     "closed",
			"labels":    "a,b",
			"sort":      "updated",
			"direction": "asc",
			"since":     referenceTimeRaw,
			"collab":    "true",
			"orgs":      "true",
			"owned":     "true",
			"pulls":     "true",
			"page":      "5",
			"per_page":  "10",
		})
		fmt.Fprint(w, `[{"number":1}]`)
	})

	opt := &ListAllIssuesOptions{
		Filter:      "all",
		State:       "closed",
		Labels:      []string{"a", "b"},
		Sort:        "updated",
		Direction:   "asc",
		Since:       referenceTime,
		Collab:      true,
		Orgs:        true,
		Owned:       true,
		Pulls:       true,
		ListOptions: ListOptions{Page: 5, PerPage: 10},
	}
	ctx := t.Context()
	issues, _, err := client.Issues.ListAllIssues(ctx, opt)
	if err != nil {
		t.Errorf("Issues.ListAllIssues returned error: %v", err)
	}

	want := []*Issue{{Number: Ptr(1)}}
	if !cmp.Equal(issues, want) {
		t.Errorf("Issues.ListAllIssues = %+v, want %+v", issues, want)
	}

	const methodName = "ListAllIssues"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListAllIssues(ctx, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListUserIssues(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{
			"filter":    "all",
			"state":     "closed",
			"labels":    "a,b",
			"sort":      "updated",
			"direction": "asc",
			"since":     referenceTimeRaw,
			"per_page":  "4",
			"page":      "2",
		})
		fmt.Fprint(w, `[{"number":1}]`)
	})

	ctx := t.Context()
	opts := &ListUserIssuesOptions{
		Filter:      "all",
		State:       "closed",
		Labels:      []string{"a", "b"},
		Sort:        "updated",
		Direction:   "asc",
		Since:       referenceTime,
		ListOptions: ListOptions{Page: 2, PerPage: 4},
	}
	issues, _, err := client.Issues.ListUserIssues(ctx, opts)
	if err != nil {
		t.Errorf("Issues.ListUserIssues returned error: %v", err)
	}

	want := []*Issue{{Number: Ptr(1)}}
	if !cmp.Equal(issues, want) {
		t.Errorf("Issues.ListUserIssues = %+v, want %+v", issues, want)
	}

	const methodName = "ListUserIssues"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListUserIssues(ctx, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, nil)
		}
		return resp, err
	})
}

func TestIssuesService_ListByOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"filter":    "all",
			"state":     "closed",
			"labels":    "a,b",
			"type":      "bug",
			"sort":      "updated",
			"direction": "asc",
			"since":     referenceTimeRaw,
			"per_page":  "4",
			"page":      "2",
		})
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		fmt.Fprint(w, `[{"number":1}]`)
	})

	ctx := t.Context()
	opts := &IssueListByOrgOptions{
		Filter:      "all",
		State:       "closed",
		Labels:      []string{"a", "b"},
		Type:        "bug",
		Sort:        "updated",
		Direction:   "asc",
		Since:       referenceTime,
		ListOptions: ListOptions{Page: 2, PerPage: 4},
	}
	issues, _, err := client.Issues.ListByOrg(ctx, "o", opts)
	if err != nil {
		t.Errorf("Issues.ListByOrg returned error: %v", err)
	}

	want := []*Issue{{Number: Ptr(1)}}
	if !cmp.Equal(issues, want) {
		t.Errorf("Issues.ListByOrg returned %+v, want %+v", issues, want)
	}

	const methodName = "ListByOrg"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListByOrg(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListByOrg(ctx, "o", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListByRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		testFormValues(t, r, values{
			"milestone": "*",
			"state":     "closed",
			"assignee":  "a",
			"creator":   "c",
			"mentioned": "m",
			"labels":    "a,b",
			"sort":      "updated",
			"direction": "asc",
			"since":     referenceTimeRaw,
			"per_page":  "1",
		})
		fmt.Fprint(w, `[{"number":1}]`)
	})

	// IssueListByRepoOptions uses standard strings (not pointers) and ListCursorOptions
	opt := &IssueListByRepoOptions{
		Milestone:         "*",
		State:             "closed",
		Assignee:          "a",
		Creator:           "c",
		Mentioned:         "m",
		Labels:            []string{"a", "b"},
		Sort:              "updated",
		Direction:         "asc",
		Since:             referenceTime,
		ListCursorOptions: ListCursorOptions{PerPage: 1},
	}

	ctx := t.Context()
	issues, _, err := client.Issues.ListByRepo(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Issues.ListByRepo returned error: %v", err)
	}

	want := []*Issue{{Number: Ptr(1)}}
	if !cmp.Equal(issues, want) {
		t.Errorf("Issues.ListByRepo returned %+v, want %+v", issues, want)
	}

	const methodName = "ListByRepo"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListByRepo(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListByRepo(ctx, "o", "r", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_Get(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeReactionsPreview)
		fmt.Fprint(w, `{"number":1, "author_association": "MEMBER","labels": [{"url": "u", "name": "n", "color": "c"}]}`)
	})

	ctx := t.Context()
	issue, _, err := client.Issues.Get(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Issues.Get returned error: %v", err)
	}

	want := &Issue{
		Number:            Ptr(1),
		AuthorAssociation: Ptr("MEMBER"),
		Labels: []*Label{{
			URL:   "u",
			Name:  "n",
			Color: "c",
		}},
	}
	if !cmp.Equal(issue, want) {
		t.Errorf("Issues.Get returned %+v, want %+v", issue, want)
	}

	const methodName = "Get"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.Get(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.Get(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_Get_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.Get(ctx, "%", "r", 1)
	testURLParseError(t, err)
}

func TestIssuesService_Create(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateIssueRequest{
		Title:    "t",
		Body:     Ptr("b"),
		Assignee: Ptr("a"),
		Labels:   []string{"l1", "l2"},
	}

	mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":1}`)
	})

	ctx := t.Context()
	issue, _, err := client.Issues.Create(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Issues.Create returned error: %v", err)
	}

	want := &Issue{Number: Ptr(1)}
	if !cmp.Equal(issue, want) {
		t.Errorf("Issues.Create returned %+v, want %+v", issue, want)
	}

	const methodName = "Create"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.Create(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.Create(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_Create_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.Create(ctx, "%", "r", CreateIssueRequest{})
	testURLParseError(t, err)
}

func TestIssuesService_Update(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := UpdateIssueRequest{Title: Ptr("t"), Type: Ptr("bug")}

	mux.HandleFunc("/repos/o/r/issues/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"number":1, "type": {"name": "bug"}}`)
	})

	ctx := t.Context()
	issue, _, err := client.Issues.Update(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Issues.Update returned error: %v", err)
	}

	want := &Issue{Number: Ptr(1), Type: &IssueType{Name: Ptr("bug")}}
	if !cmp.Equal(issue, want) {
		t.Errorf("Issues.Update returned %+v, want %+v", issue, want)
	}

	const methodName = "Update"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.Update(ctx, "\n", "\n", -1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.Update(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_RemoveMilestone(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"number":1}`)
	})

	ctx := t.Context()
	issue, _, err := client.Issues.RemoveMilestone(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Issues.RemoveMilestone returned error: %v", err)
	}

	want := &Issue{Number: Ptr(1)}
	if !cmp.Equal(issue, want) {
		t.Errorf("Issues.RemoveMilestone returned %+v, want %+v", issue, want)
	}

	const methodName = "RemoveMilestone"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.RemoveMilestone(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.RemoveMilestone(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_Update_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.Update(ctx, "%", "r", 1, UpdateIssueRequest{})
	testURLParseError(t, err)
}

// TestIssueRequest_Marshal_LabelsAndAssignees verifies the omitzero behavior of
// the Labels and Assignees fields: a nil slice is omitted from the request body
// (leaving the existing values unchanged), whereas a non-nil slice — including
// an explicit empty slice — is sent (clearing the values when empty).
func TestIssueRequest_Marshal_LabelsAndAssignees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input UpdateIssueRequest
		want  string
	}{
		{
			name:  "nil labels and assignees are omitted",
			input: UpdateIssueRequest{},
			want:  `{}`,
		},
		{
			name:  "empty non-nil labels and assignees are sent to clear them",
			input: UpdateIssueRequest{Labels: []string{}, Assignees: []string{}},
			want:  `{"labels":[],"assignees":[]}`,
		},
		{
			name:  "populated labels and assignees are sent",
			input: UpdateIssueRequest{Labels: []string{"bug"}, Assignees: []string{"octocat"}},
			want:  `{"labels":["bug"],"assignees":["octocat"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal(%#v) returned error: %v", tt.input, err)
			}
			if got := string(b); got != tt.want {
				t.Errorf("json.Marshal(%#v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIssuesService_Lock(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/lock", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Issues.Lock(ctx, "o", "r", 1, nil); err != nil {
		t.Errorf("Issues.Lock returned error: %v", err)
	}

	const methodName = "Lock"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Issues.Lock(ctx, "\n", "\n", -1, nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Issues.Lock(ctx, "o", "r", 1, nil)
	})
}

func TestIssuesService_LockWithReason(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/lock", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		w.WriteHeader(http.StatusNoContent)
	})

	opt := &LockIssueOptions{LockReason: "off-topic"}

	ctx := t.Context()
	if _, err := client.Issues.Lock(ctx, "o", "r", 1, opt); err != nil {
		t.Errorf("Issues.Lock returned error: %v", err)
	}
}

func TestIssuesService_Unlock(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/issues/1/lock", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Issues.Unlock(ctx, "o", "r", 1); err != nil {
		t.Errorf("Issues.Unlock returned error: %v", err)
	}

	const methodName = "Unlock"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Issues.Unlock(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Issues.Unlock(ctx, "o", "r", 1)
	})
}

func TestIsPullRequest(t *testing.T) {
	t.Parallel()
	var i Issue
	if i.IsPullRequest() {
		t.Errorf("expected i.IsPullRequest (%v) to return false, got true", i)
	}
	i.PullRequestLinks = &PullRequestLinks{URL: Ptr("http://example.com")}
	if !i.IsPullRequest() {
		t.Errorf("expected i.IsPullRequest (%v) to return true, got false", i)
	}
}
