// Copyright 2020 The go-github AUTHORS. All rights reserved.
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

func TestCodeScanningService_Alert_ID(t *testing.T) {
	t.Parallel()
	// Test: nil Alert ID == 0
	var a *Alert
	id := a.ID()
	var want int64
	if id != want {
		t.Errorf("Alert.ID error returned %+v, want %+v", id, want)
	}

	// Test: Valid HTMLURL
	a = &Alert{
		HTMLURL: Ptr("https://github.com/o/r/security/code-scanning/88"),
	}
	id = a.ID()
	want = 88
	if !cmp.Equal(id, want) {
		t.Errorf("Alert.ID error returned %+v, want %+v", id, want)
	}

	// Test: HTMLURL is nil
	a = &Alert{}
	id = a.ID()
	want = 0
	if !cmp.Equal(id, want) {
		t.Errorf("Alert.ID error returned %+v, want %+v", id, want)
	}

	// Test: ID can't be parsed as an int
	a = &Alert{
		HTMLURL: Ptr("https://github.com/o/r/security/code-scanning/bad88"),
	}
	id = a.ID()
	want = 0
	if !cmp.Equal(id, want) {
		t.Errorf("Alert.ID error returned %+v, want %+v", id, want)
	}
}

func TestCodeScanningService_UploadSarif(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	expectedSarifID := &SarifID{
		ID:  Ptr("testid"),
		URL: Ptr("https://example.com/testurl"),
	}

	sarifAnalysis := SarifAnalysis{CommitSHA: "abc", Ref: "ref/head/main", Sarif: "abc", CheckoutURI: Ptr("uri"), StartedAt: &referenceTimestamp, ToolName: Ptr("codeql-cli"), Validate: Ptr(true)}

	mux.HandleFunc("/repos/o/r/code-scanning/sarifs", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, sarifAnalysis)
		w.WriteHeader(http.StatusAccepted)
		respBody, _ := json.Marshal(expectedSarifID)
		_, _ = w.Write(respBody)
	})

	ctx := t.Context()
	respSarifID, _, err := client.CodeScanning.UploadSarif(ctx, "o", "r", sarifAnalysis)
	if err != nil {
		t.Errorf("CodeScanning.UploadSarif returned error: %v", err)
	}
	if !cmp.Equal(expectedSarifID, respSarifID) {
		t.Errorf("Sarif response = %+v, want %+v", respSarifID, expectedSarifID)
	}

	const methodName = "UploadSarif"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.UploadSarif(ctx, "\n", "\n", sarifAnalysis)
		return err
	})

	testNewRequestAndDoFailureCategory(t, methodName, client, CodeScanningUploadCategory, func() (*Response, error) {
		_, resp, err := client.CodeScanning.UploadSarif(ctx, "o", "r", sarifAnalysis)
		return resp, err
	})
}

func TestCodeScanningService_GetSARIF(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/sarifs/abc", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"processing_status": "s",
			"analyses_url": "u"
		}`)
	})

	ctx := t.Context()
	sarifUpload, _, err := client.CodeScanning.GetSARIF(ctx, "o", "r", "abc")
	if err != nil {
		t.Errorf("CodeScanning.GetSARIF returned error: %v", err)
	}

	want := &SARIFUpload{
		ProcessingStatus: Ptr("s"),
		AnalysesURL:      Ptr("u"),
	}
	if !cmp.Equal(sarifUpload, want) {
		t.Errorf("CodeScanning.GetSARIF returned %+v, want %+v", sarifUpload, want)
	}

	const methodName = "GetSARIF"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.GetSARIF(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.GetSARIF(ctx, "o", "r", "abc")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_ListAlertsForOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/code-scanning/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open", "ref": "heads/master", "severity": "warning", "tool_name": "CodeQL", "tool_guid": "guid", "direction": "asc", "sort": "updated"})
		fmt.Fprint(w, `[{
				"repository": {
					"id": 1,
					"name": "n",
					"url": "url"
				},
				"rule_id":"js/trivial-conditional",
				"rule_severity":"warning",
				"rule_description":"Useless conditional",
				"tool": {
					"name": "CodeQL",
					"guid": "guid",
					"version": "1.4.0"
				},
				"rule": {
					"id": "js/trivial-conditional",
					"severity": "warning",
					"description": "Useless conditional",
					"name": "js/trivial-conditional",
					"full_description": "Expression has no effect",
					"help": "Expression has no effect"
				},
				"most_recent_instance": {
					"ref": "refs/heads/main",
					"state": "open",
					"commit_sha": "abcdefg12345",
					"message": {
						"text": "This path depends on a user-provided value."
					},
					"location": {
						"path": "spec-main/api-session-spec.ts",
						"start_line": 917,
						"end_line": 917,
						"start_column": 7,
						"end_column": 18
					},
					"classifications": [
						"test"
					]
				},
				"created_at":`+referenceTimeStr+`,
				"state":"open",
				"closed_by":null,
				"closed_at":null,
				"url":"https://api.github.com/repos/o/r/code-scanning/alerts/25",
				"html_url":"https://github.com/o/r/security/code-scanning/25"
				},
				{
				"rule_id":"js/useless-expression",
				"rule_severity":"warning",
				"rule_description":"Expression has no effect",
				"tool": {
					"name": "CodeQL",
					"guid": null,
					"version": "1.4.0"
				},
				"rule": {
					"id": "js/useless-expression",
					"severity": "warning",
					"description": "Expression has no effect",
					"name": "js/useless-expression",
					"full_description": "Expression has no effect",
					"help": "Expression has no effect"
				},
				"most_recent_instance": {
					"ref": "refs/heads/main",
					"state": "open",
					"commit_sha": "abcdefg12345",
					"message": {
						"text": "This path depends on a user-provided value."
					},
					"location": {
						"path": "spec-main/api-session-spec.ts",
						"start_line": 917,
						"end_line": 917,
						"start_column": 7,
						"end_column": 18
					},
					"classifications": [
						"test"
					]
				},
				"created_at":`+referenceTimeStr+`,
				"state":"open",
				"closed_by":null,
				"closed_at":null,
				"url":"https://api.github.com/repos/o/r/code-scanning/alerts/88",
				"html_url":"https://github.com/o/r/security/code-scanning/88"
				}]`)
	})

	opts := &AlertListOptions{State: "open", Ref: "heads/master", Severity: "warning", ToolName: "CodeQL", ToolGUID: "guid", Direction: "asc", Sort: "updated"}
	ctx := t.Context()
	alerts, _, err := client.CodeScanning.ListAlertsForOrg(ctx, "o", opts)
	if err != nil {
		t.Errorf("CodeScanning.ListAlertsForOrg returned error: %v", err)
	}

	want := []*Alert{
		{
			Repository: &Repository{
				ID:   Ptr(int64(1)),
				URL:  Ptr("url"),
				Name: Ptr("n"),
			},
			RuleID:          Ptr("js/trivial-conditional"),
			RuleSeverity:    Ptr("warning"),
			RuleDescription: Ptr("Useless conditional"),
			Tool:            &Tool{Name: Ptr("CodeQL"), GUID: Ptr("guid"), Version: Ptr("1.4.0")},
			Rule: &Rule{
				ID:              Ptr("js/trivial-conditional"),
				Severity:        Ptr("warning"),
				Description:     Ptr("Useless conditional"),
				Name:            Ptr("js/trivial-conditional"),
				FullDescription: Ptr("Expression has no effect"),
				Help:            Ptr("Expression has no effect"),
			},
			CreatedAt: &referenceTimestamp,
			State:     Ptr("open"),
			ClosedBy:  nil,
			ClosedAt:  nil,
			URL:       Ptr("https://api.github.com/repos/o/r/code-scanning/alerts/25"),
			HTMLURL:   Ptr("https://github.com/o/r/security/code-scanning/25"),
			MostRecentInstance: &MostRecentInstance{
				Ref:       Ptr("refs/heads/main"),
				State:     Ptr("open"),
				CommitSHA: Ptr("abcdefg12345"),
				Message: &Message{
					Text: Ptr("This path depends on a user-provided value."),
				},
				Location: &Location{
					Path:        Ptr("spec-main/api-session-spec.ts"),
					StartLine:   Ptr(917),
					EndLine:     Ptr(917),
					StartColumn: Ptr(7),
					EndColumn:   Ptr(18),
				},
				Classifications: []string{"test"},
			},
		},
		{
			RuleID:          Ptr("js/useless-expression"),
			RuleSeverity:    Ptr("warning"),
			RuleDescription: Ptr("Expression has no effect"),
			Tool:            &Tool{Name: Ptr("CodeQL"), GUID: nil, Version: Ptr("1.4.0")},
			Rule: &Rule{
				ID:              Ptr("js/useless-expression"),
				Severity:        Ptr("warning"),
				Description:     Ptr("Expression has no effect"),
				Name:            Ptr("js/useless-expression"),
				FullDescription: Ptr("Expression has no effect"),
				Help:            Ptr("Expression has no effect"),
			},
			CreatedAt: &referenceTimestamp,
			State:     Ptr("open"),
			ClosedBy:  nil,
			ClosedAt:  nil,
			URL:       Ptr("https://api.github.com/repos/o/r/code-scanning/alerts/88"),
			HTMLURL:   Ptr("https://github.com/o/r/security/code-scanning/88"),
			MostRecentInstance: &MostRecentInstance{
				Ref:       Ptr("refs/heads/main"),
				State:     Ptr("open"),
				CommitSHA: Ptr("abcdefg12345"),
				Message: &Message{
					Text: Ptr("This path depends on a user-provided value."),
				},
				Location: &Location{
					Path:        Ptr("spec-main/api-session-spec.ts"),
					StartLine:   Ptr(917),
					EndLine:     Ptr(917),
					StartColumn: Ptr(7),
					EndColumn:   Ptr(18),
				},
				Classifications: []string{"test"},
			},
		},
	}
	if !cmp.Equal(alerts, want) {
		t.Errorf("CodeScanning.ListAlertsForOrg returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListAlertsForOrg"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.ListAlertsForOrg(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.ListAlertsForOrg(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_ListAlertsForOrgLisCursorOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/code-scanning/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open", "ref": "heads/master", "severity": "warning", "tool_name": "CodeQL", "per_page": "1", "before": "deadbeefb", "after": "deadbeefa"})
		fmt.Fprint(w, `[{
				"repository": {
					"id": 1,
					"name": "n",
					"url": "url"
				},
				"rule_id":"js/trivial-conditional",
				"rule_severity":"warning",
				"rule_description":"Useless conditional",
				"tool": {
					"name": "CodeQL",
					"guid": null,
					"version": "1.4.0"
				},
				"rule": {
					"id": "js/trivial-conditional",
					"severity": "warning",
					"description": "Useless conditional",
					"name": "js/trivial-conditional",
					"full_description": "Expression has no effect",
					"help": "Expression has no effect"
				},
				"most_recent_instance": {
					"ref": "refs/heads/main",
					"state": "open",
					"commit_sha": "abcdefg12345",
					"message": {
						"text": "This path depends on a user-provided value."
					},
					"location": {
						"path": "spec-main/api-session-spec.ts",
						"start_line": 917,
						"end_line": 917,
						"start_column": 7,
						"end_column": 18
					},
					"classifications": [
						"test"
					]
				},
				"created_at":`+referenceTimeStr+`,
				"state":"open",
				"closed_by":null,
				"closed_at":null,
				"url":"https://api.github.com/repos/o/r/code-scanning/alerts/25",
				"html_url":"https://github.com/o/r/security/code-scanning/25"
				}]`)
	})

	opts := &AlertListOptions{State: "open", Ref: "heads/master", Severity: "warning", ToolName: "CodeQL", ListCursorOptions: ListCursorOptions{PerPage: 1, Before: "deadbeefb", After: "deadbeefa"}}
	ctx := t.Context()
	alerts, _, err := client.CodeScanning.ListAlertsForOrg(ctx, "o", opts)
	if err != nil {
		t.Errorf("CodeScanning.ListAlertsForOrg returned error: %v", err)
	}

	want := []*Alert{
		{
			Repository: &Repository{
				ID:   Ptr(int64(1)),
				URL:  Ptr("url"),
				Name: Ptr("n"),
			},
			RuleID:          Ptr("js/trivial-conditional"),
			RuleSeverity:    Ptr("warning"),
			RuleDescription: Ptr("Useless conditional"),
			Tool:            &Tool{Name: Ptr("CodeQL"), GUID: nil, Version: Ptr("1.4.0")},
			Rule: &Rule{
				ID:              Ptr("js/trivial-conditional"),
				Severity:        Ptr("warning"),
				Description:     Ptr("Useless conditional"),
				Name:            Ptr("js/trivial-conditional"),
				FullDescription: Ptr("Expression has no effect"),
				Help:            Ptr("Expression has no effect"),
			},
			CreatedAt: &referenceTimestamp,
			State:     Ptr("open"),
			ClosedBy:  nil,
			ClosedAt:  nil,
			URL:       Ptr("https://api.github.com/repos/o/r/code-scanning/alerts/25"),
			HTMLURL:   Ptr("https://github.com/o/r/security/code-scanning/25"),
			MostRecentInstance: &MostRecentInstance{
				Ref:       Ptr("refs/heads/main"),
				State:     Ptr("open"),
				CommitSHA: Ptr("abcdefg12345"),
				Message: &Message{
					Text: Ptr("This path depends on a user-provided value."),
				},
				Location: &Location{
					Path:        Ptr("spec-main/api-session-spec.ts"),
					StartLine:   Ptr(917),
					EndLine:     Ptr(917),
					StartColumn: Ptr(7),
					EndColumn:   Ptr(18),
				},
				Classifications: []string{"test"},
			},
		},
	}
	if !cmp.Equal(alerts, want) {
		t.Errorf("CodeScanning.ListAlertsForOrg returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListAlertsForOrg"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.ListAlertsForOrg(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.ListAlertsForOrg(ctx, "o", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_ListAlertsForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open", "ref": "heads/master", "severity": "warning", "tool_name": "CodeQL", "tool_guid": "guid", "direction": "asc", "sort": "updated"})
		fmt.Fprint(w, `[{
				"rule_id":"js/trivial-conditional",
				"rule_severity":"warning",
				"rule_description":"Useless conditional",
				"tool": {
					"name": "CodeQL",
					"guid": "guid",
					"version": "1.4.0"
				},
				"rule": {
					"id": "js/trivial-conditional",
					"severity": "warning",
					"description": "Useless conditional",
					"name": "js/trivial-conditional",
					"full_description": "Expression has no effect",
					"help": "Expression has no effect"
				},
				"most_recent_instance": {
					"ref": "refs/heads/main",
					"state": "open",
					"commit_sha": "abcdefg12345",
					"message": {
						"text": "This path depends on a user-provided value."
					},
					"location": {
						"path": "spec-main/api-session-spec.ts",
						"start_line": 917,
						"end_line": 917,
						"start_column": 7,
						"end_column": 18
					},
					"classifications": [
						"test"
					]
				},
				"created_at":`+referenceTimeStr+`,
				"state":"open",
				"closed_by":null,
				"closed_at":null,
				"url":"https://api.github.com/repos/o/r/code-scanning/alerts/25",
				"html_url":"https://github.com/o/r/security/code-scanning/25"
				},
				{
				"rule_id":"js/useless-expression",
				"rule_severity":"warning",
				"rule_description":"Expression has no effect",
				"tool": {
					"name": "CodeQL",
					"guid": "guid",
					"version": "1.4.0"
				},
				"rule": {
					"id": "js/useless-expression",
					"severity": "warning",
					"description": "Expression has no effect",
					"name": "js/useless-expression",
					"full_description": "Expression has no effect",
					"help": "Expression has no effect"
				},
				"most_recent_instance": {
					"ref": "refs/heads/main",
					"state": "open",
					"commit_sha": "abcdefg12345",
					"message": {
						"text": "This path depends on a user-provided value."
					},
					"location": {
						"path": "spec-main/api-session-spec.ts",
						"start_line": 917,
						"end_line": 917,
						"start_column": 7,
						"end_column": 18
					},
					"classifications": [
						"test"
					]
				},
				"created_at":`+referenceTimeStr+`,
				"state":"open",
				"closed_by":null,
				"closed_at":null,
				"url":"https://api.github.com/repos/o/r/code-scanning/alerts/88",
				"html_url":"https://github.com/o/r/security/code-scanning/88"
				}]`)
	})

	opts := &AlertListOptions{State: "open", Ref: "heads/master", Severity: "warning", ToolName: "CodeQL", ToolGUID: "guid", Direction: "asc", Sort: "updated"}
	ctx := t.Context()
	alerts, _, err := client.CodeScanning.ListAlertsForRepo(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("CodeScanning.ListAlertsForRepo returned error: %v", err)
	}

	want := []*Alert{
		{
			RuleID:          Ptr("js/trivial-conditional"),
			RuleSeverity:    Ptr("warning"),
			RuleDescription: Ptr("Useless conditional"),
			Tool:            &Tool{Name: Ptr("CodeQL"), GUID: Ptr("guid"), Version: Ptr("1.4.0")},
			Rule: &Rule{
				ID:              Ptr("js/trivial-conditional"),
				Severity:        Ptr("warning"),
				Description:     Ptr("Useless conditional"),
				Name:            Ptr("js/trivial-conditional"),
				FullDescription: Ptr("Expression has no effect"),
				Help:            Ptr("Expression has no effect"),
			},
			CreatedAt: &referenceTimestamp,
			State:     Ptr("open"),
			ClosedBy:  nil,
			ClosedAt:  nil,
			URL:       Ptr("https://api.github.com/repos/o/r/code-scanning/alerts/25"),
			HTMLURL:   Ptr("https://github.com/o/r/security/code-scanning/25"),
			MostRecentInstance: &MostRecentInstance{
				Ref:       Ptr("refs/heads/main"),
				State:     Ptr("open"),
				CommitSHA: Ptr("abcdefg12345"),
				Message: &Message{
					Text: Ptr("This path depends on a user-provided value."),
				},
				Location: &Location{
					Path:        Ptr("spec-main/api-session-spec.ts"),
					StartLine:   Ptr(917),
					EndLine:     Ptr(917),
					StartColumn: Ptr(7),
					EndColumn:   Ptr(18),
				},
				Classifications: []string{"test"},
			},
		},
		{
			RuleID:          Ptr("js/useless-expression"),
			RuleSeverity:    Ptr("warning"),
			RuleDescription: Ptr("Expression has no effect"),
			Tool:            &Tool{Name: Ptr("CodeQL"), GUID: Ptr("guid"), Version: Ptr("1.4.0")},
			Rule: &Rule{
				ID:              Ptr("js/useless-expression"),
				Severity:        Ptr("warning"),
				Description:     Ptr("Expression has no effect"),
				Name:            Ptr("js/useless-expression"),
				FullDescription: Ptr("Expression has no effect"),
				Help:            Ptr("Expression has no effect"),
			},
			CreatedAt: &referenceTimestamp,
			State:     Ptr("open"),
			ClosedBy:  nil,
			ClosedAt:  nil,
			URL:       Ptr("https://api.github.com/repos/o/r/code-scanning/alerts/88"),
			HTMLURL:   Ptr("https://github.com/o/r/security/code-scanning/88"),
			MostRecentInstance: &MostRecentInstance{
				Ref:       Ptr("refs/heads/main"),
				State:     Ptr("open"),
				CommitSHA: Ptr("abcdefg12345"),
				Message: &Message{
					Text: Ptr("This path depends on a user-provided value."),
				},
				Location: &Location{
					Path:        Ptr("spec-main/api-session-spec.ts"),
					StartLine:   Ptr(917),
					EndLine:     Ptr(917),
					StartColumn: Ptr(7),
					EndColumn:   Ptr(18),
				},
				Classifications: []string{"test"},
			},
		},
	}
	if !cmp.Equal(alerts, want) {
		t.Errorf("CodeScanning.ListAlertsForRepo returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListAlertsForRepo"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.ListAlertsForRepo(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.ListAlertsForRepo(ctx, "o", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_UpdateAlert(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/alerts/88", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		fmt.Fprint(w, `{"rule_id":"js/useless-expression",
				"rule_severity":"warning",
				"rule_description":"Expression has no effect",
				"tool": {
					"name": "CodeQL",
					"guid": null,
					"version": "1.4.0"
				},
				"rule": {
					"id": "useless expression",
					"severity": "warning",
					"description": "Expression has no effect",
					"name": "useless expression",
					"full_description": "Expression has no effect",
					"help": "Expression has no effect"
				},
				"most_recent_instance": {
					"ref": "refs/heads/main",
					"state": "dismissed",
					"commit_sha": "abcdefg12345",
					"message": {
						"text": "This path depends on a user-provided value."
					},
					"location": {
						"path": "spec-main/api-session-spec.ts",
						"start_line": 917,
						"end_line": 917,
						"start_column": 7,
						"end_column": 18
					},
					"classifications": [
						"test"
					]
				},
				"created_at":`+referenceTimeStr+`,
				"state":"dismissed",
				"dismissed_reason": "false positive",
				"dismissed_comment": "This alert is not actually correct as sanitizer is used",
				"closed_by":null,
				"closed_at":null,
				"url":"https://api.github.com/repos/o/r/code-scanning/alerts/88",
				"html_url":"https://github.com/o/r/security/code-scanning/88"}`)
	})

	ctx := t.Context()
	dismissedComment := Ptr("This alert is not actually correct as sanitizer is used")
	dismissedReason := Ptr("false positive")
	state := Ptr("dismissed")
	stateInfo := &CodeScanningAlertState{State: *state, DismissedReason: dismissedReason, DismissedComment: dismissedComment}
	alert, _, err := client.CodeScanning.UpdateAlert(ctx, "o", "r", 88, stateInfo)
	if err != nil {
		t.Errorf("CodeScanning.UpdateAlert returned error: %v", err)
	}

	want := &Alert{
		RuleID:          Ptr("js/useless-expression"),
		RuleSeverity:    Ptr("warning"),
		RuleDescription: Ptr("Expression has no effect"),
		Tool:            &Tool{Name: Ptr("CodeQL"), GUID: nil, Version: Ptr("1.4.0")},
		Rule: &Rule{
			ID:              Ptr("useless expression"),
			Severity:        Ptr("warning"),
			Description:     Ptr("Expression has no effect"),
			Name:            Ptr("useless expression"),
			FullDescription: Ptr("Expression has no effect"),
			Help:            Ptr("Expression has no effect"),
		},
		CreatedAt:        &referenceTimestamp,
		State:            state,
		DismissedReason:  dismissedReason,
		DismissedComment: dismissedComment,
		ClosedBy:         nil,
		ClosedAt:         nil,
		URL:              Ptr("https://api.github.com/repos/o/r/code-scanning/alerts/88"),
		HTMLURL:          Ptr("https://github.com/o/r/security/code-scanning/88"),
		MostRecentInstance: &MostRecentInstance{
			Ref:       Ptr("refs/heads/main"),
			State:     Ptr("dismissed"),
			CommitSHA: Ptr("abcdefg12345"),
			Message: &Message{
				Text: Ptr("This path depends on a user-provided value."),
			},
			Location: &Location{
				Path:        Ptr("spec-main/api-session-spec.ts"),
				StartLine:   Ptr(917),
				EndLine:     Ptr(917),
				StartColumn: Ptr(7),
				EndColumn:   Ptr(18),
			},
			Classifications: []string{"test"},
		},
	}
	if !cmp.Equal(alert, want) {
		t.Errorf("CodeScanning.UpdateAlert returned %+v, want %+v", alert, want)
	}

	const methodName = "UpdateAlert"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.UpdateAlert(ctx, "\n", "\n", -88, stateInfo)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.UpdateAlert(ctx, "o", "r", 88, stateInfo)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_ListAlertInstances(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/alerts/88/instances", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
			  "ref": "refs/heads/main",
			  "analysis_key": ".github/workflows/codeql-analysis.yml:analyze",
			  "environment": "",
			  "category": ".github/workflows/codeql-analysis.yml:analyze",
			  "state": "open",
			  "fixed_at": null,
			  "commit_sha": "abcdefg12345",
			  "message": {
				"text": "This path depends on a user-provided value."
			  },
			  "location": {
				"path": "spec-main/api-session-spec.ts",
				"start_line": 917,
				"end_line": 917,
				"start_column": 7,
				"end_column": 18
			  },
			  "classifications": [
				"test"
			  ]
			}
		  ]`)
	})

	opts := &AlertInstancesListOptions{Ref: "heads/main", ListOptions: ListOptions{Page: 1}}
	ctx := t.Context()
	instances, _, err := client.CodeScanning.ListAlertInstances(ctx, "o", "r", 88, opts)
	if err != nil {
		t.Errorf("CodeScanning.ListAlertInstances returned error: %v", err)
	}

	want := []*MostRecentInstance{
		{
			Ref:         Ptr("refs/heads/main"),
			AnalysisKey: Ptr(".github/workflows/codeql-analysis.yml:analyze"),
			Category:    Ptr(".github/workflows/codeql-analysis.yml:analyze"),
			Environment: Ptr(""),
			State:       Ptr("open"),
			CommitSHA:   Ptr("abcdefg12345"),
			Message: &Message{
				Text: Ptr("This path depends on a user-provided value."),
			},
			Location: &Location{
				Path:        Ptr("spec-main/api-session-spec.ts"),
				StartLine:   Ptr(917),
				EndLine:     Ptr(917),
				StartColumn: Ptr(7),
				EndColumn:   Ptr(18),
			},
			Classifications: []string{"test"},
		},
	}
	if !cmp.Equal(instances, want) {
		t.Errorf("CodeScanning.ListAlertInstances returned %+v, want %+v", instances, want)
	}

	const methodName = "ListAlertInstances"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.ListAlertInstances(ctx, "\n", "\n", -1, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.ListAlertInstances(ctx, "o", "r", 88, opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_GetAlert(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/alerts/88", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"rule_id":"js/useless-expression",
			"rule_severity":"warning",
			"rule_description":"Expression has no effect",
			"tool": {
				"name": "CodeQL",
				"guid": null,
				"version": "1.4.0"
			},
			"rule": {
				"id": "useless expression",
				"severity": "warning",
				"description": "Expression has no effect",
				"name": "useless expression",
				"full_description": "Expression has no effect",
				"help": "Expression has no effect"
			},
			"most_recent_instance": {
				"ref": "refs/heads/main",
				"state": "open",
				"commit_sha": "abcdefg12345",
				"message": {
					"text": "This path depends on a user-provided value."
				},
				"location": {
					"path": "spec-main/api-session-spec.ts",
					"start_line": 917,
					"end_line": 917,
					"start_column": 7,
					"end_column": 18
				},
				"classifications": [
					"test"
				]
			},
			"created_at":`+referenceTimeStr+`,
			"state":"open",
			"closed_by":null,
			"closed_at":null,
			"url":"https://api.github.com/repos/o/r/code-scanning/alerts/88",
			"html_url":"https://github.com/o/r/security/code-scanning/88"
		}`)
	})

	ctx := t.Context()
	alert, _, err := client.CodeScanning.GetAlert(ctx, "o", "r", 88)
	if err != nil {
		t.Errorf("CodeScanning.GetAlert returned error: %v", err)
	}

	want := &Alert{
		RuleID:          Ptr("js/useless-expression"),
		RuleSeverity:    Ptr("warning"),
		RuleDescription: Ptr("Expression has no effect"),
		Tool:            &Tool{Name: Ptr("CodeQL"), GUID: nil, Version: Ptr("1.4.0")},
		Rule: &Rule{
			ID:              Ptr("useless expression"),
			Severity:        Ptr("warning"),
			Description:     Ptr("Expression has no effect"),
			Name:            Ptr("useless expression"),
			FullDescription: Ptr("Expression has no effect"),
			Help:            Ptr("Expression has no effect"),
		},
		CreatedAt: &referenceTimestamp,
		State:     Ptr("open"),
		ClosedBy:  nil,
		ClosedAt:  nil,
		URL:       Ptr("https://api.github.com/repos/o/r/code-scanning/alerts/88"),
		HTMLURL:   Ptr("https://github.com/o/r/security/code-scanning/88"),
		MostRecentInstance: &MostRecentInstance{
			Ref:       Ptr("refs/heads/main"),
			State:     Ptr("open"),
			CommitSHA: Ptr("abcdefg12345"),
			Message: &Message{
				Text: Ptr("This path depends on a user-provided value."),
			},
			Location: &Location{
				Path:        Ptr("spec-main/api-session-spec.ts"),
				StartLine:   Ptr(917),
				EndLine:     Ptr(917),
				StartColumn: Ptr(7),
				EndColumn:   Ptr(18),
			},
			Classifications: []string{"test"},
		},
	}
	if !cmp.Equal(alert, want) {
		t.Errorf("CodeScanning.GetAlert returned %+v, want %+v", alert, want)
	}

	const methodName = "GetAlert"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.GetAlert(ctx, "\n", "\n", -88)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.GetAlert(ctx, "o", "r", 88)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_ListAnalysesForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/analyses", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"sarif_id": "8981cd8e-b078-4ac3-a3be-1dad7dbd0b582", "ref": "heads/master"})
		fmt.Fprint(w, `[
			  {
				"ref": "refs/heads/main",
				"commit_sha": "d99612c3e1f2970085cfbaeadf8f010ef69bad83",
				"analysis_key": ".github/workflows/codeql-analysis.yml:analyze",
				"environment": "{\"language\":\"python\"}",
				"error": "",
				"category": ".github/workflows/codeql-analysis.yml:analyze/language:python",
				"created_at": `+referenceTimeStr+`,
				"results_count": 17,
				"rules_count": 49,
				"id": 201,
				"url": "https://api.github.com/repos/o/r/code-scanning/analyses/201",
				"sarif_id": "8981cd8e-b078-4ac3-a3be-1dad7dbd0b582",
				"tool": {
				  "name": "CodeQL",
				  "guid": null,
				  "version": "2.4.0"
				},
				"deletable": true,
				"warning": ""
			  },
			  {
				"ref": "refs/heads/my-branch",
				"commit_sha": "c8cff6510d4d084fb1b4aa13b64b97ca12b07321",
				"analysis_key": ".github/workflows/shiftleft.yml:build",
				"environment": "{}",
				"error": "",
				"category": ".github/workflows/shiftleft.yml:build/",
				"created_at": `+referenceTimeStr+`,
				"results_count": 17,
				"rules_count": 32,
				"id": 200,
				"url": "https://api.github.com/repos/o/r/code-scanning/analyses/200",
				"sarif_id": "8981cd8e-b078-4ac3-a3be-1dad7dbd0b582",
				"tool": {
				  "name": "Python Security ScanningAnalysis",
				  "guid": null,
				  "version": "1.2.0"
				},
				"deletable": true,
				"warning": ""
			  }
			]`)
	})

	opts := &AnalysesListOptions{SarifID: Ptr("8981cd8e-b078-4ac3-a3be-1dad7dbd0b582"), Ref: Ptr("heads/master")}
	ctx := t.Context()
	analyses, _, err := client.CodeScanning.ListAnalysesForRepo(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("CodeScanning.ListAnalysesForRepo returned error: %v", err)
	}

	want := []*ScanningAnalysis{
		{
			ID:           Ptr(int64(201)),
			Ref:          Ptr("refs/heads/main"),
			CommitSHA:    Ptr("d99612c3e1f2970085cfbaeadf8f010ef69bad83"),
			AnalysisKey:  Ptr(".github/workflows/codeql-analysis.yml:analyze"),
			Environment:  Ptr("{\"language\":\"python\"}"),
			Error:        Ptr(""),
			Category:     Ptr(".github/workflows/codeql-analysis.yml:analyze/language:python"),
			CreatedAt:    &referenceTimestamp,
			ResultsCount: Ptr(17),
			RulesCount:   Ptr(49),
			URL:          Ptr("https://api.github.com/repos/o/r/code-scanning/analyses/201"),
			SarifID:      Ptr("8981cd8e-b078-4ac3-a3be-1dad7dbd0b582"),
			Tool: &Tool{
				Name:    Ptr("CodeQL"),
				GUID:    nil,
				Version: Ptr("2.4.0"),
			},
			Deletable: Ptr(true),
			Warning:   Ptr(""),
		},
		{
			ID:           Ptr(int64(200)),
			Ref:          Ptr("refs/heads/my-branch"),
			CommitSHA:    Ptr("c8cff6510d4d084fb1b4aa13b64b97ca12b07321"),
			AnalysisKey:  Ptr(".github/workflows/shiftleft.yml:build"),
			Environment:  Ptr("{}"),
			Error:        Ptr(""),
			Category:     Ptr(".github/workflows/shiftleft.yml:build/"),
			CreatedAt:    &referenceTimestamp,
			ResultsCount: Ptr(17),
			RulesCount:   Ptr(32),
			URL:          Ptr("https://api.github.com/repos/o/r/code-scanning/analyses/200"),
			SarifID:      Ptr("8981cd8e-b078-4ac3-a3be-1dad7dbd0b582"),
			Tool: &Tool{
				Name:    Ptr("Python Security ScanningAnalysis"),
				GUID:    nil,
				Version: Ptr("1.2.0"),
			},
			Deletable: Ptr(true),
			Warning:   Ptr(""),
		},
	}
	if !cmp.Equal(analyses, want) {
		t.Errorf("CodeScanning.ListAnalysesForRepo returned %+v, want %+v", analyses, want)
	}

	const methodName = "ListAnalysesForRepo"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.ListAnalysesForRepo(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.ListAnalysesForRepo(ctx, "o", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_GetAnalysis(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/analyses/3602840", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			  "ref": "refs/heads/main",
			  "commit_sha": "c18c69115654ff0166991962832dc2bd7756e655",
			  "analysis_key": ".github/workflows/codeql-analysis.yml:analyze",
			  "environment": "{\"language\":\"javascript\"}",
			  "error": "",
			  "category": ".github/workflows/codeql-analysis.yml:analyze/language:javascript",
			  "created_at": `+referenceTimeStr+`,
			  "results_count": 3,
			  "rules_count": 67,
			  "id": 3602840,
			  "url": "https://api.github.com/repos/o/r/code-scanning/analyses/201",
			  "sarif_id": "47177e22-5596-11eb-80a1-c1e54ef945c6",
			  "tool": {
				"name": "CodeQL",
				"guid": null,
				"version": "2.4.0"
			  },
			  "deletable": true,
			  "warning": ""
			}`)
	})

	ctx := t.Context()
	analysis, _, err := client.CodeScanning.GetAnalysis(ctx, "o", "r", 3602840)
	if err != nil {
		t.Errorf("CodeScanning.GetAnalysis returned error: %v", err)
	}

	want := &ScanningAnalysis{
		ID:           Ptr(int64(3602840)),
		Ref:          Ptr("refs/heads/main"),
		CommitSHA:    Ptr("c18c69115654ff0166991962832dc2bd7756e655"),
		AnalysisKey:  Ptr(".github/workflows/codeql-analysis.yml:analyze"),
		Environment:  Ptr("{\"language\":\"javascript\"}"),
		Error:        Ptr(""),
		Category:     Ptr(".github/workflows/codeql-analysis.yml:analyze/language:javascript"),
		CreatedAt:    &referenceTimestamp,
		ResultsCount: Ptr(3),
		RulesCount:   Ptr(67),
		URL:          Ptr("https://api.github.com/repos/o/r/code-scanning/analyses/201"),
		SarifID:      Ptr("47177e22-5596-11eb-80a1-c1e54ef945c6"),
		Tool: &Tool{
			Name:    Ptr("CodeQL"),
			GUID:    nil,
			Version: Ptr("2.4.0"),
		},
		Deletable: Ptr(true),
		Warning:   Ptr(""),
	}
	if !cmp.Equal(analysis, want) {
		t.Errorf("CodeScanning.GetAnalysis returned %+v, want %+v", analysis, want)
	}

	const methodName = "GetAnalysis"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.GetAnalysis(ctx, "\n", "\n", -123)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.GetAnalysis(ctx, "o", "r", 3602840)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_DeleteAnalysis(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/analyses/40", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		fmt.Fprint(w, `{
			"next_analysis_url": "a",
			"confirm_delete_url": "b"
		}`)
	})

	ctx := t.Context()
	analysis, _, err := client.CodeScanning.DeleteAnalysis(ctx, "o", "r", 40)
	if err != nil {
		t.Errorf("CodeScanning.DeleteAnalysis returned error: %v", err)
	}

	want := &DeleteAnalysis{
		NextAnalysisURL:  Ptr("a"),
		ConfirmDeleteURL: Ptr("b"),
	}
	if !cmp.Equal(analysis, want) {
		t.Errorf("CodeScanning.DeleteAnalysis returned %+v, want %+v", analysis, want)
	}

	const methodName = "DeleteAnalysis"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.DeleteAnalysis(ctx, "\n", "\n", -123)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.DeleteAnalysis(ctx, "o", "r", 40)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_ListCodeQLDatabases(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/codeql/databases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
				"id": 1,
				"name": "name",
				"language": "language",
				"uploader": {
					"login": "a",
					"id": 1,
					"node_id": "b",
					"avatar_url": "c",
					"gravatar_id": "d",
					"url": "e",
					"html_url": "f",
					"followers_url": "g",
					"following_url": "h",
					"gists_url": "i",
					"starred_url": "j",
					"subscriptions_url": "k",
					"organizations_url": "l",
					"repos_url": "m",
					"events_url": "n",
					"received_events_url": "o",
					"type": "p",
					"site_admin": false
				},
				"content_type": "r",
				"size": 1024,
				"created_at": `+refTimeStr(1136178000)+`,
				"updated_at": `+refTimeStr(1136178001)+`,
				"url": "s"
			}
		]`)
	})

	ctx := t.Context()
	databases, _, err := client.CodeScanning.ListCodeQLDatabases(ctx, "o", "r")
	if err != nil {
		t.Errorf("CodeScanning.ListCodeQLDatabases returned error: %v", err)
	}

	want := []*CodeQLDatabase{
		{
			ID:       Ptr(int64(1)),
			Name:     Ptr("name"),
			Language: Ptr("language"),
			Uploader: &User{
				Login:             Ptr("a"),
				ID:                Ptr(int64(1)),
				NodeID:            Ptr("b"),
				AvatarURL:         Ptr("c"),
				GravatarID:        Ptr("d"),
				URL:               Ptr("e"),
				HTMLURL:           Ptr("f"),
				FollowersURL:      Ptr("g"),
				FollowingURL:      Ptr("h"),
				GistsURL:          Ptr("i"),
				StarredURL:        Ptr("j"),
				SubscriptionsURL:  Ptr("k"),
				OrganizationsURL:  Ptr("l"),
				ReposURL:          Ptr("m"),
				EventsURL:         Ptr("n"),
				ReceivedEventsURL: Ptr("o"),
				Type:              Ptr("p"),
				SiteAdmin:         Ptr(false),
			},
			ContentType: Ptr("r"),
			Size:        Ptr(int64(1024)),
			CreatedAt:   refTimestamp(1136178000),
			UpdatedAt:   refTimestamp(1136178001),
			URL:         Ptr("s"),
		},
	}

	if !cmp.Equal(databases, want) {
		t.Errorf("CodeScanning.ListCodeQLDatabases returned %+v, want %+v", databases, want)
	}

	const methodName = "ListCodeQLDatabases"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.ListCodeQLDatabases(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.ListCodeQLDatabases(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_GetCodeQLDatabase(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/codeql/databases/lang", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 1,
			"name": "name",
			"language": "language",
			"uploader": {
				"login": "a",
				"id": 1,
				"node_id": "b",
				"avatar_url": "c",
				"gravatar_id": "d",
				"url": "e",
				"html_url": "f",
				"followers_url": "g",
				"following_url": "h",
				"gists_url": "i",
				"starred_url": "j",
				"subscriptions_url": "k",
				"organizations_url": "l",
				"repos_url": "m",
				"events_url": "n",
				"received_events_url": "o",
				"type": "p",
				"site_admin": false
			},
			"content_type": "r",
			"size": 1024,
			"created_at": `+refTimeStr(1136178000)+`,
			"updated_at": `+refTimeStr(1136178001)+`,
			"url": "s"
		}`)
	})

	ctx := t.Context()
	database, _, err := client.CodeScanning.GetCodeQLDatabase(ctx, "o", "r", "lang")
	if err != nil {
		t.Errorf("CodeScanning.GetCodeQLDatabase returned error: %v", err)
	}

	want := &CodeQLDatabase{
		ID:       Ptr(int64(1)),
		Name:     Ptr("name"),
		Language: Ptr("language"),
		Uploader: &User{
			Login:             Ptr("a"),
			ID:                Ptr(int64(1)),
			NodeID:            Ptr("b"),
			AvatarURL:         Ptr("c"),
			GravatarID:        Ptr("d"),
			URL:               Ptr("e"),
			HTMLURL:           Ptr("f"),
			FollowersURL:      Ptr("g"),
			FollowingURL:      Ptr("h"),
			GistsURL:          Ptr("i"),
			StarredURL:        Ptr("j"),
			SubscriptionsURL:  Ptr("k"),
			OrganizationsURL:  Ptr("l"),
			ReposURL:          Ptr("m"),
			EventsURL:         Ptr("n"),
			ReceivedEventsURL: Ptr("o"),
			Type:              Ptr("p"),
			SiteAdmin:         Ptr(false),
		},
		ContentType: Ptr("r"),
		Size:        Ptr(int64(1024)),
		CreatedAt:   refTimestamp(1136178000),
		UpdatedAt:   refTimestamp(1136178001),
		URL:         Ptr("s"),
	}

	if !cmp.Equal(database, want) {
		t.Errorf("CodeScanning.GetCodeQLDatabase returned %+v, want %+v", database, want)
	}

	const methodName = "GetCodeQLDatabase"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.GetCodeQLDatabase(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.GetCodeQLDatabase(ctx, "o", "r", "lang")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_GetDefaultSetupConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/default-setup", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		_, err := fmt.Fprint(w, `{
		"state": "configured",
		"languages": [
			"javascript",
			"javascript-typescript",
			"typescript"
		],
		"query_suite": "default",
		"updated_at": `+referenceTimeStr+`
		}`)
		if err != nil {
			t.Fatal(err)
		}
	})

	ctx := t.Context()
	cfg, _, err := client.CodeScanning.GetDefaultSetupConfiguration(ctx, "o", "r")
	if err != nil {
		t.Errorf("CodeScanning.GetDefaultSetupConfiguration returned error: %v", err)
	}

	want := &DefaultSetupConfiguration{
		State:      Ptr("configured"),
		Languages:  []string{"javascript", "javascript-typescript", "typescript"},
		QuerySuite: Ptr("default"),
		UpdatedAt:  &referenceTimestamp,
	}
	if !cmp.Equal(cfg, want) {
		t.Errorf("CodeScanning.GetDefaultSetupConfiguration returned %+v, want %+v", cfg, want)
	}

	const methodName = "GetDefaultSetupConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.GetDefaultSetupConfiguration(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.GetDefaultSetupConfiguration(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodeScanningService_UpdateDefaultSetupConfiguration(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/code-scanning/default-setup", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		_, err := fmt.Fprint(w, `{
		"run_id": 5301214200,
		"run_url": "https://api.github.com/repos/o/r/actions/runs/5301214200"
		}`)
		if err != nil {
			t.Fatal(err)
		}
	})

	ctx := t.Context()
	options := &UpdateDefaultSetupConfigurationOptions{
		State:      "configured",
		Languages:  []string{"go"},
		QuerySuite: Ptr("default"),
	}
	got, _, err := client.CodeScanning.UpdateDefaultSetupConfiguration(ctx, "o", "r", options)
	if err != nil {
		t.Errorf("CodeScanning.UpdateDefaultSetupConfiguration returned error: %v", err)
	}

	want := &UpdateDefaultSetupConfigurationResponse{
		RunID:  Ptr(int64(5301214200)),
		RunURL: Ptr("https://api.github.com/repos/o/r/actions/runs/5301214200"),
	}
	if !cmp.Equal(got, want) {
		t.Errorf("CodeScanning.UpdateDefaultSetupConfiguration returned %+v, want %+v", got, want)
	}

	const methodName = "UpdateDefaultSetupConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.CodeScanning.UpdateDefaultSetupConfiguration(ctx, "\n", "\n", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.CodeScanning.UpdateDefaultSetupConfiguration(ctx, "o", "r", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
