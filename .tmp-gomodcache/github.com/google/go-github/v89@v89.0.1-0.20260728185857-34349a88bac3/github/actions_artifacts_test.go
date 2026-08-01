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

func TestActionsService_ListArtifacts(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/artifacts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2", "name": "TheArtifact"})
		fmt.Fprint(w,
			`{
				"total_count":1,
				"artifacts":[{"id":1}]
			}`,
		)
	})

	opts := &ListArtifactsOptions{
		Name:        Ptr("TheArtifact"),
		ListOptions: ListOptions{Page: 2},
	}
	ctx := t.Context()
	artifacts, _, err := client.Actions.ListArtifacts(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("Actions.ListArtifacts returned error: %v", err)
	}

	want := &ArtifactList{TotalCount: Ptr(int64(1)), Artifacts: []*Artifact{{ID: Ptr(int64(1))}}}
	if !cmp.Equal(artifacts, want) {
		t.Errorf("Actions.ListArtifacts returned %+v, want %+v", artifacts, want)
	}

	const methodName = "ListArtifacts"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListArtifacts(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListArtifacts(ctx, "o", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListArtifacts_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Actions.ListArtifacts(ctx, "%", "r", nil)
	testURLParseError(t, err)
}

func TestActionsService_ListArtifacts_invalidRepo(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Actions.ListArtifacts(ctx, "o", "%", nil)
	testURLParseError(t, err)
}

func TestActionsService_ListArtifacts_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/artifacts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	artifacts, resp, err := client.Actions.ListArtifacts(ctx, "o", "r", nil)
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Actions.ListArtifacts return status %v, want %v", got, want)
	}
	if artifacts != nil {
		t.Errorf("Actions.ListArtifacts return %+v, want nil", artifacts)
	}
}

func TestActionsService_ListWorkflowRunArtifacts(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/1/artifacts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w,
			`{
				"total_count":1,
				"artifacts":[{"id":1}]
			}`,
		)
	})

	opts := &ListOptions{Page: 2}
	ctx := t.Context()
	artifacts, _, err := client.Actions.ListWorkflowRunArtifacts(ctx, "o", "r", 1, opts)
	if err != nil {
		t.Errorf("Actions.ListWorkflowRunArtifacts returned error: %v", err)
	}

	want := &ArtifactList{TotalCount: Ptr(int64(1)), Artifacts: []*Artifact{{ID: Ptr(int64(1))}}}
	if !cmp.Equal(artifacts, want) {
		t.Errorf("Actions.ListWorkflowRunArtifacts returned %+v, want %+v", artifacts, want)
	}

	const methodName = "ListWorkflowRunArtifacts"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.ListWorkflowRunArtifacts(ctx, "\n", "\n", -1, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.ListWorkflowRunArtifacts(ctx, "o", "r", 1, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_ListWorkflowRunArtifacts_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Actions.ListWorkflowRunArtifacts(ctx, "%", "r", 1, nil)
	testURLParseError(t, err)
}

func TestActionsService_ListWorkflowRunArtifacts_invalidRepo(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Actions.ListWorkflowRunArtifacts(ctx, "o", "%", 1, nil)
	testURLParseError(t, err)
}

func TestActionsService_ListWorkflowRunArtifacts_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/runs/1/artifacts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	artifacts, resp, err := client.Actions.ListWorkflowRunArtifacts(ctx, "o", "r", 1, nil)
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Actions.ListWorkflowRunArtifacts return status %v, want %v", got, want)
	}
	if artifacts != nil {
		t.Errorf("Actions.ListWorkflowRunArtifacts return %+v, want nil", artifacts)
	}
}

func TestActionsService_GetArtifact(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/artifacts/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id":1,
			"node_id":"xyz",
			"name":"a",
			"size_in_bytes":5,
			"archive_download_url":"u"
		}`)
	})

	ctx := t.Context()
	artifact, _, err := client.Actions.GetArtifact(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Actions.GetArtifact returned error: %v", err)
	}

	want := &Artifact{
		ID:                 Ptr(int64(1)),
		NodeID:             Ptr("xyz"),
		Name:               Ptr("a"),
		SizeInBytes:        Ptr(int64(5)),
		ArchiveDownloadURL: Ptr("u"),
	}
	if !cmp.Equal(artifact, want) {
		t.Errorf("Actions.GetArtifact returned %+v, want %+v", artifact, want)
	}

	const methodName = "GetArtifact"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Actions.GetArtifact(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Actions.GetArtifact(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestActionsService_GetArtifact_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Actions.GetArtifact(ctx, "%", "r", 1)
	testURLParseError(t, err)
}

func TestActionsService_GetArtifact_invalidRepo(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Actions.GetArtifact(ctx, "o", "%", 1)
	testURLParseError(t, err)
}

func TestActionsService_GetArtifact_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/artifacts/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	artifact, resp, err := client.Actions.GetArtifact(ctx, "o", "r", 1)
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Actions.GetArtifact return status %v, want %v", got, want)
	}
	if artifact != nil {
		t.Errorf("Actions.GetArtifact return %+v, want nil", artifact)
	}
}

func TestActionsService_DownloadArtifact(t *testing.T) {
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

			mux.HandleFunc("/repos/o/r/actions/artifacts/1/zip", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/artifact", http.StatusFound)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.DownloadArtifact(ctx, "o", "r", 1, 1)
			if err != nil {
				t.Errorf("Actions.DownloadArtifact returned error: %v", err)
			}
			if resp.StatusCode != http.StatusFound {
				t.Errorf("Actions.DownloadArtifact returned status: %v, want %v", resp.StatusCode, http.StatusFound)
			}

			want := "https://github.com/artifact"
			if url.String() != want {
				t.Errorf("Actions.DownloadArtifact returned %+v, want %+v", url, want)
			}

			const methodName = "DownloadArtifact"
			testBadOptions(t, methodName, func() (err error) {
				_, _, err = client.Actions.DownloadArtifact(ctx, "\n", "\n", -1, 1)
				return err
			})

			// Create "bad" client with custom round tripper
			badClient, err := client.Clone(WithTransport(roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("failed to download artifact")
			})))
			if err != nil {
				t.Fatalf("failed to clone client: %v", err)
			}
			testBadOptions(t, methodName, func() (err error) {
				_, _, err = badClient.Actions.DownloadArtifact(ctx, "o", "r", 1, 1)
				return err
			})
		})
	}
}

func TestActionsService_DownloadArtifact_invalidOwner(t *testing.T) {
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
			client, _, _ := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			ctx := t.Context()
			_, _, err := client.Actions.DownloadArtifact(ctx, "%", "r", 1, 1)
			testURLParseError(t, err)
		})
	}
}

func TestActionsService_DownloadArtifact_invalidRepo(t *testing.T) {
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
			client, _, _ := setup(t)
			client.rateLimitRedirectionalEndpoints = tc.respectRateLimits

			ctx := t.Context()
			_, _, err := client.Actions.DownloadArtifact(ctx, "o", "%", 1, 1)
			testURLParseError(t, err)
		})
	}
}

func TestActionsService_DownloadArtifact_StatusMovedPermanently_dontFollowRedirects(t *testing.T) {
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

			mux.HandleFunc("/repos/o/r/actions/artifacts/1/zip", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/artifact", http.StatusMovedPermanently)
			})

			ctx := t.Context()
			_, resp, _ := client.Actions.DownloadArtifact(ctx, "o", "r", 1, 0)
			if resp.StatusCode != http.StatusMovedPermanently {
				t.Errorf("Actions.DownloadArtifact return status %v, want %v", resp.StatusCode, http.StatusMovedPermanently)
			}
		})
	}
}

func TestActionsService_DownloadArtifact_StatusMovedPermanently_followRedirects(t *testing.T) {
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

			mux.HandleFunc("/repos/o/r/actions/artifacts/1/zip", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				redirectURL, _ := url.Parse(serverURL + baseURLPath + "/redirect")
				http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
			})
			mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				http.Redirect(w, r, "https://github.com/artifact", http.StatusFound)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.DownloadArtifact(ctx, "o", "r", 1, 1)
			if err != nil {
				t.Errorf("Actions.DownloadArtifact return error: %v", err)
			}
			if resp.StatusCode != http.StatusFound {
				t.Errorf("Actions.DownloadArtifact return status %v, want %v", resp.StatusCode, http.StatusFound)
			}
			want := "https://github.com/artifact"
			if url.String() != want {
				t.Errorf("Actions.DownloadArtifact returned %+v, want %+v", url, want)
			}
		})
	}
}

func TestActionsService_DownloadArtifact_unexpectedCode(t *testing.T) {
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

			mux.HandleFunc("/repos/o/r/actions/artifacts/1/zip", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				redirectURL, _ := url.Parse(serverURL + baseURLPath + "/redirect")
				http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
			})
			mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				w.WriteHeader(http.StatusNoContent)
			})

			ctx := t.Context()
			url, resp, err := client.Actions.DownloadArtifact(ctx, "o", "r", 1, 1)
			if err == nil {
				t.Fatal("Actions.DownloadArtifact should return error on unexpected code")
			}
			if !strings.Contains(err.Error(), "unexpected status code") {
				t.Error("Actions.DownloadArtifact should return unexpected status code")
			}
			if got, want := resp.Response.StatusCode, http.StatusNoContent; got != want {
				t.Errorf("Actions.DownloadArtifact return status %v, want %v", got, want)
			}
			if url != nil {
				t.Errorf("Actions.DownloadArtifact return %+v, want nil", url)
			}
		})
	}
}

func TestActionsService_DeleteArtifact(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/artifacts/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Actions.DeleteArtifact(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Actions.DeleteArtifact return error: %v", err)
	}

	const methodName = "DeleteArtifact"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Actions.DeleteArtifact(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Actions.DeleteArtifact(ctx, "o", "r", 1)
	})
}

func TestActionsService_DeleteArtifact_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Actions.DeleteArtifact(ctx, "%", "r", 1)
	testURLParseError(t, err)
}

func TestActionsService_DeleteArtifact_invalidRepo(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, err := client.Actions.DeleteArtifact(ctx, "o", "%", 1)
	testURLParseError(t, err)
}

func TestActionsService_DeleteArtifact_notFound(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/actions/artifacts/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	resp, err := client.Actions.DeleteArtifact(ctx, "o", "r", 1)
	if err == nil {
		t.Error("Expected HTTP 404 response")
	}
	if got, want := resp.Response.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("Actions.DeleteArtifact return status %v, want %v", got, want)
	}
}
