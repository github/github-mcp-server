// Copyright 2026 The go-github AUTHORS. All rights reserved.
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

func TestCodeQualityService_GetSetup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-quality/setup", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"state": "configured",
			"languages": ["javascript-typescript", "python"],
			"runner_type": "standard",
			"runner_label": null,
			"updated_at": `+referenceTimeStr+`,
			"schedule": "weekly"
		}`)
	})

	ctx := t.Context()
	cfg, _, err := client.CodeQuality.GetSetup(ctx, "o", "r")
	if err != nil {
		t.Fatalf("CodeQuality.GetSetup returned error: %v", err)
	}

	want := &CodeQualitySetupConfiguration{
		State:      Ptr("configured"),
		Languages:  []string{"javascript-typescript", "python"},
		RunnerType: Ptr("standard"),
		UpdatedAt:  &referenceTimestamp,
		Schedule:   Ptr("weekly"),
	}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("CodeQuality.GetSetup mismatch (-want +got):\n%v", diff)
	}

	const methodName = "GetSetup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeQuality.GetSetup(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeQuality.GetSetup(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeQualityService_UpdateSetup(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CodeQualityUpdateSetupRequest{
		State:     Ptr("configured"),
		Languages: []string{"javascript-typescript", "python", "ruby"},
	}

	mux.HandleFunc("/repos/o/r/code-quality/setup", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"run_id": 42,
			"run_url": "https://api.github.com/repos/octocat/hello-world/actions/runs/42"
		}`)
	})

	ctx := t.Context()
	result, _, err := client.CodeQuality.UpdateSetup(ctx, "o", "r", *input)
	if err != nil {
		t.Fatalf("CodeQuality.UpdateSetup returned error: %v", err)
	}

	want := &CodeQualityUpdateSetupResponse{
		RunID:  Ptr(int64(42)),
		RunURL: Ptr("https://api.github.com/repos/octocat/hello-world/actions/runs/42"),
	}
	if diff := cmp.Diff(want, result); diff != "" {
		t.Errorf("CodeQuality.UpdateSetup mismatch (-want +got):\n%v", diff)
	}

	const methodName = "UpdateSetup"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeQuality.UpdateSetup(ctx, "\n", "\n", *input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeQuality.UpdateSetup(ctx, "o", "r", *input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeQualityService_UpdateSetup_withRunnerLabel(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CodeQualityUpdateSetupRequest{
		State:       Ptr("configured"),
		RunnerType:  Ptr("labeled"),
		RunnerLabel: Ptr("my-runner"),
		Languages:   []string{"go", "python"},
	}

	mux.HandleFunc("/repos/o/r/code-quality/setup", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"run_id": 99,
			"run_url": "https://api.github.com/repos/octocat/hello-world/actions/runs/99"
		}`)
	})

	ctx := t.Context()
	result, _, err := client.CodeQuality.UpdateSetup(ctx, "o", "r", *input)
	if err != nil {
		t.Fatalf("CodeQuality.UpdateSetup returned error: %v", err)
	}

	want := &CodeQualityUpdateSetupResponse{
		RunID:  Ptr(int64(99)),
		RunURL: Ptr("https://api.github.com/repos/octocat/hello-world/actions/runs/99"),
	}
	if diff := cmp.Diff(want, result); diff != "" {
		t.Errorf("CodeQuality.UpdateSetup mismatch (-want +got):\n%v", diff)
	}
}

func TestCodeQualityService_UpdateSetup_notConfigured(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CodeQualityUpdateSetupRequest{
		State: Ptr("not-configured"),
	}

	mux.HandleFunc("/repos/o/r/code-quality/setup", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	})

	ctx := t.Context()
	_, _, err := client.CodeQuality.UpdateSetup(ctx, "o", "r", *input)
	if err != nil {
		t.Fatalf("CodeQuality.UpdateSetup returned error: %v", err)
	}
}

func TestCodeQualityService_GetSetup_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.CodeQuality.GetSetup(ctx, "%", "r")
	testURLParseError(t, err)
}

func TestCodeQualityService_UpdateSetup_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.CodeQuality.UpdateSetup(ctx, "%", "r", CodeQualityUpdateSetupRequest{})
	testURLParseError(t, err)
}

func TestCodeQualityService_ListFindings(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-quality/findings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"state":     "open",
			"direction": "desc",
		})
		fmt.Fprint(w, `[
			{
				"number": 1,
				"state": "open",
				"url": "https://api.github.com/repos/o/r/code-quality/findings/1",
				"rule": {
					"id": "rule-1",
					"title": "Example Rule",
					"description": "An example rule description",
					"help": "How to fix it",
					"severity": "warning",
					"category": "maintainability"
				},
				"location": {
					"path": "src/main.go",
					"start_line": 10,
					"end_line": 10,
					"start_column": 1,
					"end_column": 20
				},
				"message": {
					"text": "Issue found",
					"markdown": "**Issue found**"
				},
				"created_at": `+referenceTimeStr+`
			}
		]`)
	})

	ctx := t.Context()
	opts := &ListCodeQualityFindingsOptions{
		State:     "open",
		Direction: "desc",
	}
	findings, _, err := client.CodeQuality.ListFindings(ctx, "o", "r", opts)
	if err != nil {
		t.Fatalf("CodeQuality.ListFindings returned error: %v", err)
	}

	want := []*CodeQualityFinding{
		{
			Number: 1,
			State:  "open",
			URL:    "https://api.github.com/repos/o/r/code-quality/findings/1",
			Rule: CodeQualityFindingRule{
				ID:          "rule-1",
				Title:       "Example Rule",
				Description: "An example rule description",
				Help:        Ptr("How to fix it"),
				Severity:    "warning",
				Category:    "maintainability",
			},
			Location: CodeQualityFindingLocation{
				Path:        "src/main.go",
				StartLine:   Ptr(10),
				EndLine:     Ptr(10),
				StartColumn: Ptr(1),
				EndColumn:   Ptr(20),
			},
			Message: CodeQualityFindingMessage{
				Text:     "Issue found",
				Markdown: "**Issue found**",
			},
			CreatedAt: &referenceTimestamp,
		},
	}
	if diff := cmp.Diff(want, findings); diff != "" {
		t.Errorf("CodeQuality.ListFindings mismatch (-want +got):\n%v", diff)
	}

	const methodName = "ListFindings"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeQuality.ListFindings(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeQuality.ListFindings(ctx, "o", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeQualityService_ListFindings_noOpts(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-quality/findings", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[]`)
	})

	ctx := t.Context()
	findings, _, err := client.CodeQuality.ListFindings(ctx, "o", "r", nil)
	if err != nil {
		t.Fatalf("CodeQuality.ListFindings returned error: %v", err)
	}

	if len(findings) != 0 {
		t.Errorf("CodeQuality.ListFindings returned %v findings, want 0", len(findings))
	}
}

func TestCodeQualityService_GetFinding(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-quality/findings/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"number": 1,
			"state": "open",
			"url": "https://api.github.com/repos/o/r/code-quality/findings/1",
			"rule": {
				"id": "rule-1",
				"title": "Example Rule",
				"description": "An example rule description",
				"severity": "error",
				"category": "reliability"
			},
			"location": {
				"path": "src/main.go",
				"start_line": 5,
				"end_line": 5
			},
			"message": {
				"text": "Critical issue",
				"markdown": "**Critical issue**"
			},
			"created_at": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()
	finding, _, err := client.CodeQuality.GetFinding(ctx, "o", "r", 1)
	if err != nil {
		t.Fatalf("CodeQuality.GetFinding returned error: %v", err)
	}

	want := &CodeQualityFinding{
		Number: 1,
		State:  "open",
		URL:    "https://api.github.com/repos/o/r/code-quality/findings/1",
		Rule: CodeQualityFindingRule{
			ID:          "rule-1",
			Title:       "Example Rule",
			Description: "An example rule description",
			Severity:    "error",
			Category:    "reliability",
		},
		Location: CodeQualityFindingLocation{
			Path:      "src/main.go",
			StartLine: Ptr(5),
			EndLine:   Ptr(5),
		},
		Message: CodeQualityFindingMessage{
			Text:     "Critical issue",
			Markdown: "**Critical issue**",
		},
		CreatedAt: &referenceTimestamp,
	}
	if diff := cmp.Diff(want, finding); diff != "" {
		t.Errorf("CodeQuality.GetFinding mismatch (-want +got):\n%v", diff)
	}

	const methodName = "GetFinding"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeQuality.GetFinding(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeQuality.GetFinding(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeQualityService_ListFindings_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.CodeQuality.ListFindings(ctx, "%", "r", nil)
	testURLParseError(t, err)
}

func TestCodeQualityService_GetFinding_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.CodeQuality.GetFinding(ctx, "%", "r", 1)
	testURLParseError(t, err)
}
