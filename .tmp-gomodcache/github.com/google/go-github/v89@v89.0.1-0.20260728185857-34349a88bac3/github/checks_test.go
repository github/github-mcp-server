// Copyright 2018 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestChecksService_GetCheckRun(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-runs/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		fmt.Fprint(w, `{
			"id": 1,
                        "name":"testCheckRun",
			"status": "completed",
			"conclusion": "neutral",
			"started_at": "2018-05-04T01:14:52Z",
			"completed_at": "2018-05-04T01:14:52Z"}`)
	})
	ctx := t.Context()
	checkRun, _, err := client.Checks.GetCheckRun(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Checks.GetCheckRun return error: %v", err)
	}
	startedAt, _ := time.Parse(time.RFC3339, "2018-05-04T01:14:52Z")
	completeAt, _ := time.Parse(time.RFC3339, "2018-05-04T01:14:52Z")

	want := &CheckRun{
		ID:          Ptr(int64(1)),
		Status:      Ptr("completed"),
		Conclusion:  Ptr("neutral"),
		StartedAt:   &Timestamp{startedAt},
		CompletedAt: &Timestamp{completeAt},
		Name:        Ptr("testCheckRun"),
	}
	if !cmp.Equal(checkRun, want) {
		t.Errorf("Checks.GetCheckRun return %+v, want %+v", checkRun, want)
	}

	const methodName = "GetCheckRun"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.GetCheckRun(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.GetCheckRun(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_GetCheckSuite(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-suites/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		fmt.Fprint(w, `{
			"id": 1,
                        "head_branch":"master",
			"head_sha": "deadbeef",
			"conclusion": "neutral",
                        "before": "deadbeefb",
                        "after": "deadbeefa",
			"status": "completed"}`)
	})
	ctx := t.Context()
	checkSuite, _, err := client.Checks.GetCheckSuite(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Checks.GetCheckSuite return error: %v", err)
	}
	want := &CheckSuite{
		ID:         Ptr(int64(1)),
		HeadBranch: Ptr("master"),
		HeadSHA:    Ptr("deadbeef"),
		AfterSHA:   Ptr("deadbeefa"),
		BeforeSHA:  Ptr("deadbeefb"),
		Status:     Ptr("completed"),
		Conclusion: Ptr("neutral"),
	}
	if !cmp.Equal(checkSuite, want) {
		t.Errorf("Checks.GetCheckSuite return %+v, want %+v", checkSuite, want)
	}

	const methodName = "GetCheckSuite"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.GetCheckSuite(ctx, "\n", "\n", -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.GetCheckSuite(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_CreateCheckRun(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-runs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		fmt.Fprint(w, `{
			"id": 1,
                        "name":"testCreateCheckRun",
                        "head_sha":"deadbeef",
			"status": "in_progress",
			"conclusion": null,
			"started_at": "2018-05-04T01:14:52Z",
			"completed_at": null,
                        "output":{"title": "Mighty test report", "summary":"", "text":""}}`)
	})
	startedAt, _ := time.Parse(time.RFC3339, "2018-05-04T01:14:52Z")
	checkRunOpt := CreateCheckRunOptions{
		Name:      "testCreateCheckRun",
		HeadSHA:   "deadbeef",
		Status:    Ptr("in_progress"),
		StartedAt: &Timestamp{startedAt},
		Output: &CheckRunOutput{
			Title:   Ptr("Mighty test report"),
			Summary: Ptr(""),
			Text:    Ptr(""),
		},
	}

	ctx := t.Context()
	checkRun, _, err := client.Checks.CreateCheckRun(ctx, "o", "r", checkRunOpt)
	if err != nil {
		t.Errorf("Checks.CreateCheckRun return error: %v", err)
	}

	want := &CheckRun{
		ID:        Ptr(int64(1)),
		Status:    Ptr("in_progress"),
		StartedAt: &Timestamp{startedAt},
		HeadSHA:   Ptr("deadbeef"),
		Name:      Ptr("testCreateCheckRun"),
		Output: &CheckRunOutput{
			Title:   Ptr("Mighty test report"),
			Summary: Ptr(""),
			Text:    Ptr(""),
		},
	}
	if !cmp.Equal(checkRun, want) {
		t.Errorf("Checks.CreateCheckRun return %+v, want %+v", checkRun, want)
	}

	const methodName = "CreateCheckRun"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.CreateCheckRun(ctx, "\n", "\n", CreateCheckRunOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.CreateCheckRun(ctx, "o", "r", checkRunOpt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_ListCheckRunAnnotations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-runs/1/annotations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		testFormValues(t, r, values{
			"page": "1",
		})
		fmt.Fprint(w, `[{
		                           "path": "README.md",
		                           "start_line": 2,
		                           "end_line": 2,
		                           "start_column": 1,
		                           "end_column": 5,
		                           "annotation_level": "warning",
		                           "message": "Check your spelling for 'banaas'.",
                                           "title": "Spell check",
		                           "raw_details": "Do you mean 'bananas' or 'banana'?"}]`,
		)
	})

	ctx := t.Context()
	checkRunAnnotations, _, err := client.Checks.ListCheckRunAnnotations(ctx, "o", "r", 1, &ListOptions{Page: 1})
	if err != nil {
		t.Errorf("Checks.ListCheckRunAnnotations return error: %v", err)
	}

	want := []*CheckRunAnnotation{{
		Path:            Ptr("README.md"),
		StartLine:       Ptr(2),
		EndLine:         Ptr(2),
		StartColumn:     Ptr(1),
		EndColumn:       Ptr(5),
		AnnotationLevel: Ptr("warning"),
		Message:         Ptr("Check your spelling for 'banaas'."),
		Title:           Ptr("Spell check"),
		RawDetails:      Ptr("Do you mean 'bananas' or 'banana'?"),
	}}

	if !cmp.Equal(checkRunAnnotations, want) {
		t.Errorf("Checks.ListCheckRunAnnotations returned %+v, want %+v", checkRunAnnotations, want)
	}

	const methodName = "ListCheckRunAnnotations"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.ListCheckRunAnnotations(ctx, "\n", "\n", -1, &ListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.ListCheckRunAnnotations(ctx, "o", "r", 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_UpdateCheckRun(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-runs/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		fmt.Fprint(w, `{
			"id": 1,
                        "name":"testUpdateCheckRun",
			"status": "completed",
			"conclusion": "neutral",
			"started_at": "2018-05-04T01:14:52Z",
			"completed_at": "2018-05-04T01:14:52Z",
                        "output":{"title": "Mighty test report", "summary":"There are 0 failures, 2 warnings and 1 notice", "text":"You may have misspelled some words."}}`)
	})
	startedAt, _ := time.Parse(time.RFC3339, "2018-05-04T01:14:52Z")
	updateCheckRunOpt := UpdateCheckRunOptions{
		Name:        "testUpdateCheckRun",
		Status:      Ptr("completed"),
		CompletedAt: &Timestamp{startedAt},
		Output: &CheckRunOutput{
			Title:   Ptr("Mighty test report"),
			Summary: Ptr("There are 0 failures, 2 warnings and 1 notice"),
			Text:    Ptr("You may have misspelled some words."),
		},
	}

	ctx := t.Context()
	checkRun, _, err := client.Checks.UpdateCheckRun(ctx, "o", "r", 1, updateCheckRunOpt)
	if err != nil {
		t.Errorf("Checks.UpdateCheckRun return error: %v", err)
	}

	want := &CheckRun{
		ID:          Ptr(int64(1)),
		Status:      Ptr("completed"),
		StartedAt:   &Timestamp{startedAt},
		CompletedAt: &Timestamp{startedAt},
		Conclusion:  Ptr("neutral"),
		Name:        Ptr("testUpdateCheckRun"),
		Output: &CheckRunOutput{
			Title:   Ptr("Mighty test report"),
			Summary: Ptr("There are 0 failures, 2 warnings and 1 notice"),
			Text:    Ptr("You may have misspelled some words."),
		},
	}
	if !cmp.Equal(checkRun, want) {
		t.Errorf("Checks.UpdateCheckRun return %+v, want %+v", checkRun, want)
	}

	const methodName = "UpdateCheckRun"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.UpdateCheckRun(ctx, "\n", "\n", -1, UpdateCheckRunOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.UpdateCheckRun(ctx, "o", "r", 1, updateCheckRunOpt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_ListCheckRunsForRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/commits/master/check-runs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		testFormValues(t, r, values{
			"check_name": "testing",
			"page":       "1",
			"status":     "completed",
			"filter":     "all",
			"app_id":     "1",
		})
		fmt.Fprint(w, `{"total_count":1,
                                "check_runs": [{
                                    "id": 1,
                                    "head_sha": "deadbeef",
                                    "status": "completed",
                                    "conclusion": "neutral",
                                    "started_at": "2018-05-04T01:14:52Z",
                                    "completed_at": "2018-05-04T01:14:52Z",
                                    "app": {
                                      "id": 1}}]}`,
		)
	})

	opt := &ListCheckRunsOptions{
		CheckName:   Ptr("testing"),
		Status:      Ptr("completed"),
		Filter:      Ptr("all"),
		AppID:       Ptr(int64(1)),
		ListOptions: ListOptions{Page: 1},
	}
	ctx := t.Context()
	checkRuns, _, err := client.Checks.ListCheckRunsForRef(ctx, "o", "r", "master", opt)
	if err != nil {
		t.Errorf("Checks.ListCheckRunsForRef return error: %v", err)
	}
	startedAt, _ := time.Parse(time.RFC3339, "2018-05-04T01:14:52Z")
	want := &ListCheckRunsResults{
		Total: Ptr(1),
		CheckRuns: []*CheckRun{{
			ID:          Ptr(int64(1)),
			Status:      Ptr("completed"),
			StartedAt:   &Timestamp{startedAt},
			CompletedAt: &Timestamp{startedAt},
			Conclusion:  Ptr("neutral"),
			HeadSHA:     Ptr("deadbeef"),
			App:         &App{ID: Ptr(int64(1))},
		}},
	}

	if !cmp.Equal(checkRuns, want) {
		t.Errorf("Checks.ListCheckRunsForRef returned %+v, want %+v", checkRuns, want)
	}

	const methodName = "ListCheckRunsForRef"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.ListCheckRunsForRef(ctx, "\n", "\n", "\n", &ListCheckRunsOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.ListCheckRunsForRef(ctx, "o", "r", "master", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_ListCheckRunsCheckSuite(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-suites/1/check-runs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		testFormValues(t, r, values{
			"check_name": "testing",
			"page":       "1",
			"status":     "completed",
			"filter":     "all",
		})
		fmt.Fprint(w, `{"total_count":1,
                                "check_runs": [{
                                    "id": 1,
                                    "head_sha": "deadbeef",
                                    "status": "completed",
                                    "conclusion": "neutral",
                                    "started_at": "2018-05-04T01:14:52Z",
                                    "completed_at": "2018-05-04T01:14:52Z"}]}`,
		)
	})

	opt := &ListCheckRunsOptions{
		CheckName:   Ptr("testing"),
		Status:      Ptr("completed"),
		Filter:      Ptr("all"),
		ListOptions: ListOptions{Page: 1},
	}
	ctx := t.Context()
	checkRuns, _, err := client.Checks.ListCheckRunsCheckSuite(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Checks.ListCheckRunsCheckSuite return error: %v", err)
	}
	startedAt, _ := time.Parse(time.RFC3339, "2018-05-04T01:14:52Z")
	want := &ListCheckRunsResults{
		Total: Ptr(1),
		CheckRuns: []*CheckRun{{
			ID:          Ptr(int64(1)),
			Status:      Ptr("completed"),
			StartedAt:   &Timestamp{startedAt},
			CompletedAt: &Timestamp{startedAt},
			Conclusion:  Ptr("neutral"),
			HeadSHA:     Ptr("deadbeef"),
		}},
	}

	if !cmp.Equal(checkRuns, want) {
		t.Errorf("Checks.ListCheckRunsCheckSuite returned %+v, want %+v", checkRuns, want)
	}

	const methodName = "ListCheckRunsCheckSuite"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.ListCheckRunsCheckSuite(ctx, "\n", "\n", -1, &ListCheckRunsOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.ListCheckRunsCheckSuite(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_ListCheckSuiteForRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/commits/master/check-suites", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		testFormValues(t, r, values{
			"check_name": "testing",
			"page":       "1",
			"app_id":     "2",
		})
		fmt.Fprint(w, `{"total_count":1,
                                "check_suites": [{
                                    "id": 1,
                                    "head_sha": "deadbeef",
                                    "head_branch": "master",
                                    "status": "completed",
                                    "conclusion": "neutral",
                                    "before": "deadbeefb",
                                    "after": "deadbeefa"}]}`,
		)
	})

	opt := &ListCheckSuiteOptions{
		CheckName:   Ptr("testing"),
		AppID:       Ptr(int64(2)),
		ListOptions: ListOptions{Page: 1},
	}
	ctx := t.Context()
	checkSuites, _, err := client.Checks.ListCheckSuitesForRef(ctx, "o", "r", "master", opt)
	if err != nil {
		t.Errorf("Checks.ListCheckSuitesForRef return error: %v", err)
	}
	want := &ListCheckSuiteResults{
		Total: Ptr(1),
		CheckSuites: []*CheckSuite{{
			ID:         Ptr(int64(1)),
			Status:     Ptr("completed"),
			Conclusion: Ptr("neutral"),
			HeadSHA:    Ptr("deadbeef"),
			HeadBranch: Ptr("master"),
			BeforeSHA:  Ptr("deadbeefb"),
			AfterSHA:   Ptr("deadbeefa"),
		}},
	}

	if !cmp.Equal(checkSuites, want) {
		t.Errorf("Checks.ListCheckSuitesForRef returned %+v, want %+v", checkSuites, want)
	}

	const methodName = "ListCheckSuitesForRef"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.ListCheckSuitesForRef(ctx, "\n", "\n", "\n", &ListCheckSuiteOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.ListCheckSuitesForRef(ctx, "o", "r", "master", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_SetCheckSuitePreferences(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	a := []*AutoTriggerCheck{{
		AppID:   Ptr(int64(2)),
		Setting: Ptr(false),
	}}
	opt := CheckSuitePreferenceOptions{AutoTriggerChecks: a}

	mux.HandleFunc("/repos/o/r/check-suites/preferences", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		testJSONBody(t, r, opt)
		fmt.Fprint(w, `{"preferences":{"auto_trigger_checks":[{"app_id": 2,"setting": false}]}}`)
	})

	ctx := t.Context()
	prefResults, _, err := client.Checks.SetCheckSuitePreferences(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Checks.SetCheckSuitePreferences return error: %v", err)
	}

	p := &PreferenceList{
		AutoTriggerChecks: a,
	}
	want := &CheckSuitePreferenceResults{
		Preferences: p,
	}

	if !cmp.Equal(prefResults, want) {
		t.Errorf("Checks.SetCheckSuitePreferences return %+v, want %+v", prefResults, want)
	}

	const methodName = "SetCheckSuitePreferences"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.SetCheckSuitePreferences(ctx, "\n", "\n", CheckSuitePreferenceOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.SetCheckSuitePreferences(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_CreateCheckSuite(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-suites", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		fmt.Fprint(w, `{
			"id": 2,
                        "head_branch":"master",
                        "head_sha":"deadbeef",
			"status": "completed",
			"conclusion": "neutral",
                        "before": "deadbeefb",
                        "after": "deadbeefa"}`)
	})

	checkSuiteOpt := CreateCheckSuiteOptions{
		HeadSHA:    "deadbeef",
		HeadBranch: Ptr("master"),
	}

	ctx := t.Context()
	checkSuite, _, err := client.Checks.CreateCheckSuite(ctx, "o", "r", checkSuiteOpt)
	if err != nil {
		t.Errorf("Checks.CreateCheckSuite return error: %v", err)
	}

	want := &CheckSuite{
		ID:         Ptr(int64(2)),
		Status:     Ptr("completed"),
		HeadSHA:    Ptr("deadbeef"),
		HeadBranch: Ptr("master"),
		Conclusion: Ptr("neutral"),
		BeforeSHA:  Ptr("deadbeefb"),
		AfterSHA:   Ptr("deadbeefa"),
	}
	if !cmp.Equal(checkSuite, want) {
		t.Errorf("Checks.CreateCheckSuite return %+v, want %+v", checkSuite, want)
	}

	const methodName = "CreateCheckSuite"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Checks.CreateCheckSuite(ctx, "\n", "\n", CreateCheckSuiteOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Checks.CreateCheckSuite(ctx, "o", "r", checkSuiteOpt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestChecksService_ReRequestCheckSuite(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-suites/1/rerequest", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		w.WriteHeader(http.StatusCreated)
	})
	ctx := t.Context()
	resp, err := client.Checks.ReRequestCheckSuite(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Checks.ReRequestCheckSuite return error: %v", err)
	}
	if got, want := resp.StatusCode, http.StatusCreated; got != want {
		t.Errorf("Checks.ReRequestCheckSuite = %v, want %v", got, want)
	}

	const methodName = "ReRequestCheckSuite"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Checks.ReRequestCheckSuite(ctx, "\n", "\n", 1)
		return err
	})
}

func TestChecksService_ReRequestCheckRun(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/check-runs/1/rerequest", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeCheckRunsPreview)
		w.WriteHeader(http.StatusCreated)
	})
	ctx := t.Context()
	resp, err := client.Checks.ReRequestCheckRun(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Checks.ReRequestCheckRun return error: %v", err)
	}
	if got, want := resp.StatusCode, http.StatusCreated; got != want {
		t.Errorf("Checks.ReRequestCheckRun = %v, want %v", got, want)
	}

	const methodName = "ReRequestCheckRun"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Checks.ReRequestCheckRun(ctx, "\n", "\n", 1)
		return err
	})
}
