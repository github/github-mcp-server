// Copyright 2020 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIssueImportService_Create(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &IssueImportRequest{
		IssueImport: IssueImport{
			Assignee:  Ptr("developer"),
			Body:      "Dummy description",
			CreatedAt: &referenceTimestamp,
			Labels:    []string{"l1", "l2"},
			Milestone: Ptr(1),
			Title:     "Dummy Issue",
		},
		Comments: []*Comment{{
			CreatedAt: &referenceTimestamp,
			Body:      "Comment body",
		}},
	}

	mux.HandleFunc("/repos/o/r/import/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeIssueImportAPI)
		testJSONBody(t, r, input)
		assertWrite(t, w, issueImportResponseJSON)
	})

	ctx := t.Context()
	got, _, err := client.IssueImport.Create(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Create returned error: %v", err)
	}

	want := wantIssueImportResponse
	if !cmp.Equal(got, want) {
		t.Errorf("Create = %+v, want %+v", got, want)
	}

	const methodName = "Create"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.IssueImport.Create(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.IssueImport.Create(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssueImportService_Create_deferred(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &IssueImportRequest{
		IssueImport: IssueImport{
			Assignee:  Ptr("developer"),
			Body:      "Dummy description",
			CreatedAt: &referenceTimestamp,
			Labels:    []string{"l1", "l2"},
			Milestone: Ptr(1),
			Title:     "Dummy Issue",
		},
		Comments: []*Comment{{
			CreatedAt: &referenceTimestamp,
			Body:      "Comment body",
		}},
	}

	mux.HandleFunc("/repos/o/r/import/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeIssueImportAPI)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusAccepted)
		assertWrite(t, w, issueImportResponseJSON)
	})

	ctx := t.Context()
	got, _, err := client.IssueImport.Create(ctx, "o", "r", input)

	if !errors.As(err, new(*AcceptedError)) {
		t.Errorf("Create returned error: %v (want AcceptedError)", err)
	}

	want := wantIssueImportResponse
	if !cmp.Equal(got, want) {
		t.Errorf("Create = %+v, want %+v", got, want)
	}
}

func TestIssueImportService_Create_badResponse(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &IssueImportRequest{
		IssueImport: IssueImport{
			Assignee:  Ptr("developer"),
			Body:      "Dummy description",
			CreatedAt: &referenceTimestamp,
			Labels:    []string{"l1", "l2"},
			Milestone: Ptr(1),
			Title:     "Dummy Issue",
		},
		Comments: []*Comment{{
			CreatedAt: &referenceTimestamp,
			Body:      "Comment body",
		}},
	}

	mux.HandleFunc("/repos/o/r/import/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeIssueImportAPI)
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusAccepted)
		assertWrite(t, w, []byte("{[}"))
	})

	ctx := t.Context()
	_, _, err := client.IssueImport.Create(ctx, "o", "r", input)

	if err == nil || err.Error() != "invalid character '[' looking for beginning of object key string" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIssueImportService_Create_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.IssueImport.Create(ctx, "%", "r", nil)
	testURLParseError(t, err)
}

func TestIssueImportService_CheckStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/import/issues/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeIssueImportAPI)
		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, issueImportResponseJSON)
	})

	ctx := t.Context()
	got, _, err := client.IssueImport.CheckStatus(ctx, "o", "r", 3)
	if err != nil {
		t.Errorf("CheckStatus returned error: %v", err)
	}

	want := wantIssueImportResponse
	if !cmp.Equal(got, want) {
		t.Errorf("CheckStatus = %+v, want %+v", got, want)
	}

	const methodName = "CheckStatus"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.IssueImport.CheckStatus(ctx, "\n", "\n", -3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.IssueImport.CheckStatus(ctx, "o", "r", 3)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssueImportService_CheckStatus_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.IssueImport.CheckStatus(ctx, "%", "r", 1)
	testURLParseError(t, err)
}

func TestIssueImportService_CheckStatusSince(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/import/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeIssueImportAPI)
		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, fmt.Appendf(nil, "[%s]", issueImportResponseJSON))
	})

	ctx := t.Context()
	got, _, err := client.IssueImport.CheckStatusSince(ctx, "o", "r", referenceTimestamp)
	if err != nil {
		t.Errorf("CheckStatusSince returned error: %v", err)
	}

	want := []*IssueImportResponse{wantIssueImportResponse}
	if !cmp.Equal(want, got) {
		t.Errorf("CheckStatusSince = %v, want = %v", got, want)
	}

	const methodName = "CheckStatusSince"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.IssueImport.CheckStatusSince(ctx, "\n", "\n", referenceTimestamp)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.IssueImport.CheckStatusSince(ctx, "o", "r", referenceTimestamp)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestIssueImportService_CheckStatusSince_badResponse(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/import/issues", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeIssueImportAPI)
		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte("{badly-formed JSON"))
	})

	ctx := t.Context()
	if _, _, err := client.IssueImport.CheckStatusSince(ctx, "o", "r", referenceTimestamp); err == nil {
		t.Error("CheckStatusSince returned no error, want JSON err")
	}
}

func TestIssueImportService_CheckStatusSince_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.IssueImport.CheckStatusSince(ctx, "%", "r", referenceTimestamp)
	testURLParseError(t, err)
}

var issueImportResponseJSON = []byte(`{
	"id": 3,
	"status": "pending",
	"url": "https://api.github.com/repos/o/r/import/issues/3",
	"import_issues_url": "https://api.github.com/repos/o/r/import/issues",
	"repository_url": "https://api.github.com/repos/o/r"
}`)

var wantIssueImportResponse = &IssueImportResponse{
	ID:              Ptr(3),
	Status:          Ptr("pending"),
	URL:             Ptr("https://api.github.com/repos/o/r/import/issues/3"),
	ImportIssuesURL: Ptr("https://api.github.com/repos/o/r/import/issues"),
	RepositoryURL:   Ptr("https://api.github.com/repos/o/r"),
}
