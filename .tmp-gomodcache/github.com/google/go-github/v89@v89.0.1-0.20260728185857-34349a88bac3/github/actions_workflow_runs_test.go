// Copyright 2020 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestActionsService_ListWorkflowRunsByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/workflows/29679449/runs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":4,"workflow_runs":[{"id":399444496,"run_number":296,"created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`},{"id":399444497,"run_number":296,"created_at":`+refTimeStr(1136178002)+`,"updated_at":`+refTimeStr(1136178003)+`}]}`)
	})

	opts := &ListWorkflowRunsOptions{ListOptions: ListOptions{Page: 2, PerPage: 2}}
	ctx := t.Context()
	runs, _, err := client.Actions.ListWorkflowRunsByID(ctx, "o", "r", 29679449, opts)
	if err != nil {
		t.Errorf("Actions.ListWorkflowRunsByID returned error: %v", err)
	}

	want := &WorkflowRuns{
		TotalCount: Ptr(4),
		WorkflowRuns: []*WorkflowRun{
			{ID: Ptr(int64(399444496)), RunNumber: Ptr(296), CreatedAt: refTimestamp(1136178000), UpdatedAt: refTimestamp(1136178001)},
			{ID: Ptr(int64(399444497)), RunNumber: Ptr(296), CreatedAt: refTimestamp(1136178002), UpdatedAt: refTimestamp(1136178003)},
		},
	}
	if !cmp.Equal(runs, want) {
		t.Errorf("Actions.ListWorkflowRunsByID returned %+v, want %+v", runs, want)
	}

	const methodName = "ListWorkflowRunsByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListWorkflowRunsByID(ctx, "\n", "\n", 29679449, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListWorkflowRunsByID(ctx, "o", "r", 29679449, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListWorkflowRunsFileName(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/workflows/29679449/runs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":4,"workflow_runs":[{"id":399444496,"run_number":296,"created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`},{"id":399444497,"run_number":296,"created_at":`+refTimeStr(1136178002)+`,"updated_at":`+refTimeStr(1136178003)+`}]}`)
	})

	opts := &ListWorkflowRunsOptions{ListOptions: ListOptions{Page: 2, PerPage: 2}}
	ctx := t.Context()
	runs, _, err := client.Actions.ListWorkflowRunsByFileName(ctx, "o", "r", "29679449", opts)
	if err != nil {
		t.Errorf("Actions.ListWorkflowRunsByFileName returned error: %v", err)
	}

	want := &WorkflowRuns{
		TotalCount: Ptr(4),
		WorkflowRuns: []*WorkflowRun{
			{ID: Ptr(int64(399444496)), RunNumber: Ptr(296), CreatedAt: refTimestamp(1136178000), UpdatedAt: refTimestamp(1136178001)},
			{ID: Ptr(int64(399444497)), RunNumber: Ptr(296), CreatedAt: refTimestamp(1136178002), UpdatedAt: refTimestamp(1136178003)},
		},
	}
	if !cmp.Equal(runs, want) {
		t.Errorf("Actions.ListWorkflowRunsByFileName returned %+v, want %+v", runs, want)
	}

	const methodName = "ListWorkflowRunsByFileName"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListWorkflowRunsByFileName(ctx, "\n", "\n", "29679449", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListWorkflowRunsByFileName(ctx, "o", "r", "29679449", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetWorkflowRunByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/29679449", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":399444496,"run_number":296,"created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`}`)
	})

	ctx := t.Context()
	runs, _, err := client.Actions.GetWorkflowRunByID(ctx, "o", "r", 29679449)
	if err != nil {
		t.Errorf("Actions.GetWorkflowRunByID returned error: %v", err)
	}

	want := &WorkflowRun{
		ID:        Ptr(int64(399444496)),
		RunNumber: Ptr(296),
		CreatedAt: refTimestamp(1136178000),
		UpdatedAt: refTimestamp(1136178001),
	}

	if !cmp.Equal(runs, want) {
		t.Errorf("Actions.GetWorkflowRunByID returned %+v, want %+v", runs, want)
	}

	const methodName = "GetWorkflowRunByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetWorkflowRunByID(ctx, "\n", "\n", 29679449)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetWorkflowRunByID(ctx, "o", "r", 29679449)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetWorkflowRunAttempt(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/29679449/attempts/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"exclude_pull_requests": "true"})
		fmt.Fprint(w, `{"id":399444496,"run_number":296,"run_attempt":3,"created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`}`)
	})

	opts := &WorkflowRunAttemptOptions{ExcludePullRequests: Ptr(true)}
	ctx := t.Context()
	runs, _, err := client.Actions.GetWorkflowRunAttempt(ctx, "o", "r", 29679449, 3, opts)
	if err != nil {
		t.Errorf("Actions.GetWorkflowRunAttempt returned error: %v", err)
	}

	want := &WorkflowRun{
		ID:         Ptr(int64(399444496)),
		RunNumber:  Ptr(296),
		RunAttempt: Ptr(3),
		CreatedAt:  refTimestamp(1136178000),
		UpdatedAt:  refTimestamp(1136178001),
	}

	if !cmp.Equal(runs, want) {
		t.Errorf("Actions.GetWorkflowRunAttempt returned %+v, want %+v", runs, want)
	}

	const methodName = "GetWorkflowRunAttempt"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetWorkflowRunAttempt(ctx, "\n", "\n", 29679449, 3, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetWorkflowRunAttempt(ctx, "o", "r", 29679449, 3, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetWorkflowRunAttemptLogs(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name              string
		respectRateLimits bool
	}{
		{
			name:              "withoutRateLimits",
			respectRateLimits: false,
		},
		{
			name:              "withRateLimits",
			respectRateLimits: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, mux, _ := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			mux.HandleFunc("/repos/o/r/actions/runs/399444496/attempts/2/logs", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/a", http.StatusFound)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.GetWorkflowRunAttemptLogs(ctx, "o", "r", 399444496, 2, 1)
			if err != nil {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs returned error: %v", err)
			}
			if resp.StatusCode != http.StatusFound {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs returned status: %v, want %v", resp.StatusCode, http.StatusFound)
			}
			want := "https://github.com/a"
			if url.String() != want {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs returned %+v, want %+v", url, want)
			}

			const methodName = "GetWorkflowRunAttemptLogs"
			testBadOptions(t, methodName, func() (err error) {
				_, _, err = client.Actions.GetWorkflowRunAttemptLogs(ctx, "\n", "\n", 399444496, 2, 1)
				return err
			})
		})
	}
}

func TestActionsService_GetWorkflowRunAttemptLogs_StatusMovedPermanently_dontFollowRedirects(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name              string
		respectRateLimits bool
	}{
		{
			name:              "withoutRateLimits",
			respectRateLimits: false,
		},
		{
			name:              "withRateLimits",
			respectRateLimits: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, mux, _ := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			mux.HandleFunc("/repos/o/r/actions/runs/399444496/attempts/2/logs", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/a", http.StatusMovedPermanently)
			})

			ctx := t.Context()
			_, resp, _ := client.Actions.GetWorkflowRunAttemptLogs(ctx, "o", "r", 399444496, 2, 0)
			if resp.StatusCode != http.StatusMovedPermanently {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs returned status: %v, want %v", resp.StatusCode, http.StatusMovedPermanently)
			}
		})
	}
}

func TestActionsService_GetWorkflowRunAttemptLogs_StatusMovedPermanently_followRedirects(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name              string
		respectRateLimits bool
	}{
		{
			name:              "withoutRateLimits",
			respectRateLimits: false,
		},
		{
			name:              "withRateLimits",
			respectRateLimits: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, mux, serverURL := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			// Mock a redirect link, which leads to an archive link
			mux.HandleFunc("/repos/o/r/actions/runs/399444496/attempts/2/logs", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				redirectURL, _ := url.Parse(serverURL + baseURLPath + "/redirect")
				http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
			})

			mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/a", http.StatusFound)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.GetWorkflowRunAttemptLogs(ctx, "o", "r", 399444496, 2, 1)
			if err != nil {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs returned error: %v", err)
			}

			if resp.StatusCode != http.StatusFound {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs returned status: %v, want %v", resp.StatusCode, http.StatusFound)
			}

			want := "https://github.com/a"
			if url.String() != want {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs returned %+v, want %+v", url, want)
			}

			const methodName = "GetWorkflowRunAttemptLogs"
			testBadOptions(t, methodName, func() (err error) {
				_, _, err = client.Actions.GetWorkflowRunAttemptLogs(ctx, "\n", "\n", 399444496, 2, 1)
				return err
			})
		})
	}
}

func TestActionsService_GetWorkflowRunAttemptLogs_unexpectedCode(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name              string
		respectRateLimits bool
	}{
		{
			name:              "withoutRateLimits",
			respectRateLimits: false,
		},
		{
			name:              "withRateLimits",
			respectRateLimits: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, mux, serverURL := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			// Mock a redirect link, which leads to an archive link
			mux.HandleFunc("/repos/o/r/actions/runs/399444496/attempts/2/logs", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				redirectURL, _ := url.Parse(serverURL + baseURLPath + "/redirect")
				http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
			})

			mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				w.WriteHeader(http.StatusNoContent)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.GetWorkflowRunAttemptLogs(ctx, "o", "r", 399444496, 2, 1)
			if err == nil {
				t.Fatal("Actions.GetWorkflowRunAttemptLogs should return error on unexpected code")
			}
			if !strings.Contains(err.Error(), "unexpected status code") {
				t.Error("Actions.GetWorkflowRunAttemptLogs should return unexpected status code")
			}
			if got, want := resp.Response.StatusCode, http.StatusNoContent; got != want {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs return status %v, want %v", got, want)
			}
			if url != nil {
				t.Errorf("Actions.GetWorkflowRunAttemptLogs return %+v, want nil", url)
			}
		})
	}
}

func TestActionsService_RerunWorkflowByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/3434/rerun", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		w.WriteHeader(http.StatusCreated)
	})

	ctx := t.Context()
	resp, err := client.Actions.RerunWorkflowByID(ctx, "o", "r", 3434)
	if err != nil {
		t.Errorf("Actions.RerunWorkflowByID returned error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Actions.RerunWorkflowByID returned status: %v, want %v", resp.StatusCode, http.StatusCreated)
	}

	const methodName = "RerunWorkflowByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RerunWorkflowByID(ctx, "\n", "\n", 3434)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RerunWorkflowByID(ctx, "o", "r", 3434)
	})
}

func TestActionsService_RerunFailedJobsByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/3434/rerun-failed-jobs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		w.WriteHeader(http.StatusCreated)
	})

	ctx := t.Context()
	resp, err := client.Actions.RerunFailedJobsByID(ctx, "o", "r", 3434)
	if err != nil {
		t.Errorf("Actions.RerunFailedJobsByID returned error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Actions.RerunFailedJobsByID returned status: %v, want %v", resp.StatusCode, http.StatusCreated)
	}

	const methodName = "RerunFailedJobsByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RerunFailedJobsByID(ctx, "\n", "\n", 3434)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RerunFailedJobsByID(ctx, "o", "r", 3434)
	})
}

func TestActionsService_RerunJobByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/jobs/3434/rerun", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		w.WriteHeader(http.StatusCreated)
	})

	ctx := t.Context()
	resp, err := client.Actions.RerunJobByID(ctx, "o", "r", 3434)
	if err != nil {
		t.Errorf("Actions.RerunJobByID returned error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Actions.RerunJobByID returned status: %v, want %v", resp.StatusCode, http.StatusCreated)
	}

	const methodName = "RerunJobByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.RerunJobByID(ctx, "\n", "\n", 3434)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.RerunJobByID(ctx, "o", "r", 3434)
	})
}

func TestActionsService_CancelWorkflowRunByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/3434/cancel", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		w.WriteHeader(http.StatusAccepted)
	})

	ctx := t.Context()
	resp, err := client.Actions.CancelWorkflowRunByID(ctx, "o", "r", 3434)
	if !errors.As(err, new(*AcceptedError)) {
		t.Errorf("Actions.CancelWorkflowRunByID returned error: %v (want AcceptedError)", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Actions.CancelWorkflowRunByID returned status: %v, want %v", resp.StatusCode, http.StatusAccepted)
	}

	const methodName = "CancelWorkflowRunByID"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.CancelWorkflowRunByID(ctx, "\n", "\n", 3434)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.CancelWorkflowRunByID(ctx, "o", "r", 3434)
	})
}

func TestActionsService_GetWorkflowRunLogs(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name              string
		respectRateLimits bool
	}{
		{
			name:              "withoutRateLimits",
			respectRateLimits: false,
		},
		{
			name:              "withRateLimits",
			respectRateLimits: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, mux, _ := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			mux.HandleFunc("/repos/o/r/actions/runs/399444496/logs", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/a", http.StatusFound)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, "o", "r", 399444496, 1)
			if err != nil {
				t.Errorf("Actions.GetWorkflowRunLogs returned error: %v", err)
			}
			if resp.StatusCode != http.StatusFound {
				t.Errorf("Actions.GetWorkflowRunLogs returned status: %v, want %v", resp.StatusCode, http.StatusFound)
			}
			want := "https://github.com/a"
			if url.String() != want {
				t.Errorf("Actions.GetWorkflowRunLogs returned %+v, want %+v", url, want)
			}

			const methodName = "GetWorkflowRunLogs"
			testBadOptions(t, methodName, func() (err error) {
				_, _, err = client.Actions.GetWorkflowRunLogs(ctx, "\n", "\n", 399444496, 1)
				return err
			})
		})
	}
}

func TestActionsService_GetWorkflowRunLogs_StatusMovedPermanently_dontFollowRedirects(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name              string
		respectRateLimits bool
	}{
		{
			name:              "withoutRateLimits",
			respectRateLimits: false,
		},
		{
			name:              "withRateLimits",
			respectRateLimits: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, mux, _ := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			mux.HandleFunc("/repos/o/r/actions/runs/399444496/logs", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/a", http.StatusMovedPermanently)
			})

			ctx := t.Context()
			_, resp, _ := client.Actions.GetWorkflowRunLogs(ctx, "o", "r", 399444496, 0)
			if resp.StatusCode != http.StatusMovedPermanently {
				t.Errorf("Actions.GetWorkflowRunLogs returned status: %v, want %v", resp.StatusCode, http.StatusMovedPermanently)
			}
		})
	}
}

func TestActionsService_GetWorkflowRunLogs_StatusMovedPermanently_followRedirects(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name              string
		respectRateLimits bool
	}{
		{
			name:              "withoutRateLimits",
			respectRateLimits: false,
		},
		{
			name:              "withRateLimits",
			respectRateLimits: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, mux, serverURL := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			// Mock a redirect link, which leads to an archive link
			mux.HandleFunc("/repos/o/r/actions/runs/399444496/logs", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				redirectURL, _ := url.Parse(serverURL + baseURLPath + "/redirect")
				http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
			})

			mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/a", http.StatusFound)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, "o", "r", 399444496, 1)
			if err != nil {
				t.Errorf("Actions.GetWorkflowRunLogs returned error: %v", err)
			}

			if resp.StatusCode != http.StatusFound {
				t.Errorf("Actions.GetWorkflowRunLogs returned status: %v, want %v", resp.StatusCode, http.StatusFound)
			}

			want := "https://github.com/a"
			if url.String() != want {
				t.Errorf("Actions.GetWorkflowRunLogs returned %+v, want %+v", url, want)
			}

			const methodName = "GetWorkflowRunLogs"
			testBadOptions(t, methodName, func() (err error) {
				_, _, err = client.Actions.GetWorkflowRunLogs(ctx, "\n", "\n", 399444496, 1)
				return err
			})
		})
	}
}

func TestActionsService_GetWorkflowRunLogs_unexpectedCode(t *testing.T) {
	t.Parallel()
	tcs := []struct {
		name              string
		respectRateLimits bool
	}{
		{
			name:              "withoutRateLimits",
			respectRateLimits: false,
		},
		{
			name:              "withRateLimits",
			respectRateLimits: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, mux, serverURL := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			// Mock a redirect link, which leads to an archive link
			mux.HandleFunc("/repos/o/r/actions/runs/399444496/logs", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				redirectURL, _ := url.Parse(serverURL + baseURLPath + "/redirect")
				http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
			})

			mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				w.WriteHeader(http.StatusNoContent)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, "o", "r", 399444496, 1)
			if err == nil {
				t.Fatal("Actions.GetWorkflowRunLogs should return error on unexpected code")
			}
			if !strings.Contains(err.Error(), "unexpected status code") {
				t.Error("Actions.GetWorkflowRunLogs should return unexpected status code")
			}
			if got, want := resp.Response.StatusCode, http.StatusNoContent; got != want {
				t.Errorf("Actions.GetWorkflowRunLogs return status %v, want %v", got, want)
			}
			if url != nil {
				t.Errorf("Actions.GetWorkflowRunLogs return %+v, want nil", url)
			}
		})
	}
}

func TestActionsService_ListRepositoryWorkflowRuns(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "2"})
		fmt.Fprint(w, `{"total_count":2,
		"workflow_runs":[
			{"id":298499444,"run_number":301,"created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`},
			{"id":298499445,"run_number":302,"created_at":`+refTimeStr(1136178002)+`,"updated_at":`+refTimeStr(1136178003)+`}]}`)
	})

	opts := &ListWorkflowRunsOptions{ListOptions: ListOptions{Page: 2, PerPage: 2}}
	ctx := t.Context()
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("Actions.ListRepositoryWorkflowRuns returned error: %v", err)
	}

	expected := &WorkflowRuns{
		TotalCount: Ptr(2),
		WorkflowRuns: []*WorkflowRun{
			{ID: Ptr(int64(298499444)), RunNumber: Ptr(301), CreatedAt: refTimestamp(1136178000), UpdatedAt: refTimestamp(1136178001)},
			{ID: Ptr(int64(298499445)), RunNumber: Ptr(302), CreatedAt: refTimestamp(1136178002), UpdatedAt: refTimestamp(1136178003)},
		},
	}

	if !cmp.Equal(runs, expected) {
		t.Errorf("Actions.ListRepositoryWorkflowRuns returned %+v, want %+v", runs, expected)
	}

	const methodName = "ListRepositoryWorkflowRuns"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListRepositoryWorkflowRuns(ctx, "\n", "\n", opts)

		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListRepositoryWorkflowRuns(ctx, "o", "r", opts)

		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_DeleteWorkflowRun(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/399444496", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Actions.DeleteWorkflowRun(ctx, "o", "r", 399444496); err != nil {
		t.Errorf("DeleteWorkflowRun returned error: %v", err)
	}

	const methodName = "DeleteWorkflowRun"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.DeleteWorkflowRun(ctx, "\n", "\n", 399444496)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.DeleteWorkflowRun(ctx, "o", "r", 399444496)
	})
}

func TestActionsService_DeleteWorkflowRunLogs(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/399444496/logs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Actions.DeleteWorkflowRunLogs(ctx, "o", "r", 399444496); err != nil {
		t.Errorf("DeleteWorkflowRunLogs returned error: %v", err)
	}

	const methodName = "DeleteWorkflowRunLogs"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.DeleteWorkflowRunLogs(ctx, "\n", "\n", 399444496)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.DeleteWorkflowRunLogs(ctx, "o", "r", 399444496)
	})
}

func TestActionsService_ReviewCustomDeploymentProtectionRule(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/9444496/deployment_protection_rule", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")

		w.WriteHeader(http.StatusNoContent)
	})

	request := ReviewCustomDeploymentProtectionRuleRequest{
		EnvironmentName: "production",
		State:           "approved",
		Comment:         "Approve deployment",
	}

	ctx := t.Context()
	if _, err := client.Actions.ReviewCustomDeploymentProtectionRule(ctx, "o", "r", 9444496, &request); err != nil {
		t.Errorf("ReviewCustomDeploymentProtectionRule returned error: %v", err)
	}

	const methodName = "ReviewCustomDeploymentProtectionRule"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.ReviewCustomDeploymentProtectionRule(ctx, "\n", "\n", 9444496, &request)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.ReviewCustomDeploymentProtectionRule(ctx, "o", "r", 9444496, &request)
	})
}

func TestActionsService_GetWorkflowRunUsageByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/29679449/timing", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"billable":{"UBUNTU":{"total_ms":180000,"jobs":1,"job_runs":[{"job_id":1,"duration_ms":60000}]},"MACOS":{"total_ms":240000,"jobs":2,"job_runs":[{"job_id":2,"duration_ms":30000},{"job_id":3,"duration_ms":10000}]},"WINDOWS":{"total_ms":300000,"jobs":2}},"run_duration_ms":500000}`)
	})

	ctx := t.Context()
	workflowRunUsage, _, err := client.Actions.GetWorkflowRunUsageByID(ctx, "o", "r", 29679449)
	if err != nil {
		t.Errorf("Actions.GetWorkflowRunUsageByID returned error: %v", err)
	}

	want := &WorkflowRunUsage{
		Billable: &WorkflowRunBillMap{
			"UBUNTU": &WorkflowRunBill{
				TotalMS: Ptr(int64(180000)),
				Jobs:    Ptr(1),
				JobRuns: []*WorkflowRunJobRun{
					{
						JobID:      Ptr(1),
						DurationMS: Ptr(int64(60000)),
					},
				},
			},
			"MACOS": &WorkflowRunBill{
				TotalMS: Ptr(int64(240000)),
				Jobs:    Ptr(2),
				JobRuns: []*WorkflowRunJobRun{
					{
						JobID:      Ptr(2),
						DurationMS: Ptr(int64(30000)),
					},
					{
						JobID:      Ptr(3),
						DurationMS: Ptr(int64(10000)),
					},
				},
			},
			"WINDOWS": &WorkflowRunBill{
				TotalMS: Ptr(int64(300000)),
				Jobs:    Ptr(2),
			},
		},
		RunDurationMS: Ptr(int64(500000)),
	}

	if !cmp.Equal(workflowRunUsage, want) {
		t.Errorf("Actions.GetWorkflowRunUsageByID returned %+v, want %+v", workflowRunUsage, want)
	}

	const methodName = "GetWorkflowRunUsageByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetWorkflowRunUsageByID(ctx, "\n", "\n", 29679449)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetWorkflowRunUsageByID(ctx, "o", "r", 29679449)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_PendingDeployments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &PendingDeploymentsRequest{EnvironmentIDs: []int64{3, 4}, State: "approved", Comment: ""}

	mux.HandleFunc("/repos/o/r/actions/runs/399444496/pending_deployments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	ctx := t.Context()
	deployments, _, err := client.Actions.PendingDeployments(ctx, "o", "r", 399444496, input)
	if err != nil {
		t.Errorf("Actions.PendingDeployments returned error: %v", err)
	}

	want := []*Deployment{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(deployments, want) {
		t.Errorf("Actions.PendingDeployments returned %+v, want %+v", deployments, want)
	}

	const methodName = "PendingDeployments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.PendingDeployments(ctx, "\n", "\n", 399444496, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.PendingDeployments(ctx, "o", "r", 399444496, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetPendingDeployments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/399444496/pending_deployments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
				"environment": {
					"id": 1,
					"node_id": "nid",
					"name": "n",
					"url": "u",
					"html_url": "hu"
				},
				"wait_timer": 0,
				"wait_timer_started_at": `+refTimeStr(1136178000)+`,
				"current_user_can_approve": false,
				"reviewers": []
			},
			{
				"environment": {
					"id": 2,
					"node_id": "nid",
					"name": "n",
					"url": "u",
					"html_url": "hu"
				},
				"wait_timer": 13,
				"wait_timer_started_at": `+refTimeStr(1136178001)+`,
				"current_user_can_approve": true,
				"reviewers": [
					{
						"type": "User",
						"reviewer": {
							"login": "l"
						}
					},
					{
						"type": "Team",
						"reviewer": {
							"name": "t",
							"slug": "s"
						}
					}
				]
			}
		]`)
	})

	ctx := t.Context()
	deployments, _, err := client.Actions.GetPendingDeployments(ctx, "o", "r", 399444496)
	if err != nil {
		t.Errorf("Actions.GetPendingDeployments returned error: %v", err)
	}

	want := []*PendingDeployment{
		{
			Environment: &PendingDeploymentEnvironment{
				ID:      Ptr(int64(1)),
				NodeID:  Ptr("nid"),
				Name:    Ptr("n"),
				URL:     Ptr("u"),
				HTMLURL: Ptr("hu"),
			},
			WaitTimer:             Ptr(int64(0)),
			WaitTimerStartedAt:    refTimestamp(1136178000),
			CurrentUserCanApprove: Ptr(false),
			Reviewers:             []*RequiredReviewer{},
		},
		{
			Environment: &PendingDeploymentEnvironment{
				ID:      Ptr(int64(2)),
				NodeID:  Ptr("nid"),
				Name:    Ptr("n"),
				URL:     Ptr("u"),
				HTMLURL: Ptr("hu"),
			},
			WaitTimer:             Ptr(int64(13)),
			WaitTimerStartedAt:    refTimestamp(1136178001),
			CurrentUserCanApprove: Ptr(true),
			Reviewers: []*RequiredReviewer{
				{
					Type: Ptr("User"),
					Reviewer: &User{
						Login: Ptr("l"),
					},
				},
				{
					Type: Ptr("Team"),
					Reviewer: &Team{
						Name: Ptr("t"),
						Slug: Ptr("s"),
					},
				},
			},
		},
	}

	if !cmp.Equal(deployments, want) {
		t.Errorf("Actions.GetPendingDeployments returned %+v, want %+v", deployments, want)
	}

	const methodName = "GetPendingDeployments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetPendingDeployments(ctx, "\n", "\n", 399444496)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetPendingDeployments(ctx, "o", "r", 399444496)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
