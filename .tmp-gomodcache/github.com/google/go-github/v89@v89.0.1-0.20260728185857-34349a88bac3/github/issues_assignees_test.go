// Copyright 2013 The go-github AUTHORS. All rights reserved.
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

func TestIssuesService_ListAssignees(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/assignees", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	assignees, _, err := client.Issues.ListAssignees(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Issues.ListAssignees returned error: %v", err)
	}

	want := []*User{{ID: Ptr(int64(1))}}
	if !cmp.Equal(assignees, want) {
		t.Errorf("Issues.ListAssignees returned %+v, want %+v", assignees, want)
	}

	const methodName = "ListAssignees"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.ListAssignees(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.ListAssignees(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_ListAssignees_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.ListAssignees(ctx, "%", "r", nil)
	testURLParseError(t, err)
}

func TestIssuesService_IsAssignee_true(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/assignees/u", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
	})

	ctx := t.Context()
	assignee, _, err := client.Issues.IsAssignee(ctx, "o", "r", "u")
	if err != nil {
		t.Errorf("Issues.IsAssignee returned error: %v", err)
	}
	if want := true; assignee != want {
		t.Errorf("Issues.IsAssignee returned %+v, want %+v", assignee, want)
	}

	const methodName = "IsAssignee"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.IsAssignee(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.IsAssignee(ctx, "o", "r", "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_IsAssignee_false(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/assignees/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	assignee, _, err := client.Issues.IsAssignee(ctx, "o", "r", "u")
	if err != nil {
		t.Errorf("Issues.IsAssignee returned error: %v", err)
	}
	if want := false; assignee != want {
		t.Errorf("Issues.IsAssignee returned %+v, want %+v", assignee, want)
	}

	const methodName = "IsAssignee"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.IsAssignee(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.IsAssignee(ctx, "o", "r", "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_IsAssignee_error(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/assignees/u", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		http.Error(w, "BadRequest", http.StatusBadRequest)
	})

	ctx := t.Context()
	assignee, _, err := client.Issues.IsAssignee(ctx, "o", "r", "u")
	if err == nil {
		t.Error("Expected HTTP 400 response")
	}
	if want := false; assignee != want {
		t.Errorf("Issues.IsAssignee returned %+v, want %+v", assignee, want)
	}

	const methodName = "IsAssignee"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.IsAssignee(ctx, "o", "r", "u")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.IsAssignee(ctx, "o", "r", "u")
		if got {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want false", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_IsAssignee_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Issues.IsAssignee(ctx, "%", "r", "u")
	testURLParseError(t, err)
}

func TestIssuesService_AddAssignees(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	assignees := struct {
		Assignees []string `json:"assignees,omitempty"`
	}{
		Assignees: []string{"user1", "user2"},
	}

	mux.HandleFunc("/repos/o/r/issues/1/assignees", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, assignees)
		fmt.Fprint(w, `{"number":1,"assignees":[{"login":"user1"},{"login":"user2"}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Issues.AddAssignees(ctx, "o", "r", 1, assignees.Assignees)
	if err != nil {
		t.Errorf("Issues.AddAssignees returned error: %v", err)
	}

	want := &Issue{Number: Ptr(1), Assignees: []*User{{Login: Ptr("user1")}, {Login: Ptr("user2")}}}
	if !cmp.Equal(got, want) {
		t.Errorf("Issues.AddAssignees = %+v, want %+v", got, want)
	}

	const methodName = "AddAssignees"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.AddAssignees(ctx, "\n", "\n", -1, []string{"\n", "\n"})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.AddAssignees(ctx, "o", "r", 1, []string{"user1", "user2"})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssuesService_RemoveAssignees(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	assignees := struct {
		Assignees []string `json:"assignees,omitempty"`
	}{
		Assignees: []string{"user1", "user2"},
	}

	mux.HandleFunc("/repos/o/r/issues/1/assignees", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, assignees)
		fmt.Fprint(w, `{"number":1,"assignees":[]}`)
	})

	ctx := t.Context()
	got, _, err := client.Issues.RemoveAssignees(ctx, "o", "r", 1, assignees.Assignees)
	if err != nil {
		t.Errorf("Issues.RemoveAssignees returned error: %v", err)
	}

	want := &Issue{Number: Ptr(1), Assignees: []*User{}}
	if !cmp.Equal(got, want) {
		t.Errorf("Issues.RemoveAssignees = %+v, want %+v", got, want)
	}

	const methodName = "RemoveAssignees"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Issues.RemoveAssignees(ctx, "\n", "\n", -1, []string{"\n", "\n"})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Issues.RemoveAssignees(ctx, "o", "r", 1, []string{"user1", "user2"})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
