// Copyright 2022 The go-github AUTHORS. All rights reserved.
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

func TestSecretScanningService_ListAlertsForEnterprise(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/secret-scanning/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open", "secret_type": "mailchimp_api_key", "sort": "updated", "direction": "asc"})

		fmt.Fprint(w, `[{
			"number": 1,
			"created_at": `+refTimeStr(1136178000)+`,
			"url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1",
			"html_url": "https://github.com/o/r/security/secret-scanning/1",
			"locations_url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations",
			"state": "open",
			"resolution": null,
			"resolved_at": null,
			"resolved_by": null,
			"secret_type": "mailchimp_api_key",
			"secret": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2",
			"repository": {
				"id": 1,
				"name": "n",
				"url": "url"
			}
		}]`)
	})

	ctx := t.Context()
	opts := &SecretScanningAlertListOptions{State: "open", SecretType: "mailchimp_api_key", Direction: "asc", Sort: "updated"}

	alerts, _, err := client.SecretScanning.ListAlertsForEnterprise(ctx, "e", opts)
	if err != nil {
		t.Errorf("SecretScanning.ListAlertsForEnterprise returned error: %v", err)
	}

	want := []*SecretScanningAlert{
		{
			Number:       Ptr(1),
			CreatedAt:    refTimestamp(1136178000),
			URL:          Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1"),
			HTMLURL:      Ptr("https://github.com/o/r/security/secret-scanning/1"),
			LocationsURL: Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations"),
			State:        Ptr("open"),
			Resolution:   nil,
			ResolvedAt:   nil,
			ResolvedBy:   nil,
			SecretType:   Ptr("mailchimp_api_key"),
			Secret:       Ptr("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"),
			Repository: &Repository{
				ID:   Ptr(int64(1)),
				URL:  Ptr("url"),
				Name: Ptr("n"),
			},
		},
	}

	if !cmp.Equal(alerts, want) {
		t.Errorf("SecretScanning.ListAlertsForEnterprise returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListAlertsForEnterprise"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.ListAlertsForEnterprise(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.ListAlertsForEnterprise(ctx, "e", opts)
		return resp, err
	})
}

func TestSecretScanningService_ListAlertsForOrg(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/secret-scanning/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open", "secret_type": "mailchimp_api_key", "sort": "updated", "direction": "asc"})

		fmt.Fprint(w, `[{
			"number": 1,
			"created_at": `+refTimeStr(1136178000)+`,
			"url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1",
			"html_url": "https://github.com/o/r/security/secret-scanning/1",
			"locations_url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations",
			"state": "open",
			"resolution": null,
			"resolved_at": null,
			"resolved_by": null,
			"secret_type": "mailchimp_api_key",
			"secret": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"
		}]`)
	})

	ctx := t.Context()
	opts := &SecretScanningAlertListOptions{State: "open", SecretType: "mailchimp_api_key", Direction: "asc", Sort: "updated"}

	alerts, _, err := client.SecretScanning.ListAlertsForOrg(ctx, "o", opts)
	if err != nil {
		t.Errorf("SecretScanning.ListAlertsForOrg returned error: %v", err)
	}

	want := []*SecretScanningAlert{
		{
			Number:       Ptr(1),
			CreatedAt:    refTimestamp(1136178000),
			URL:          Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1"),
			HTMLURL:      Ptr("https://github.com/o/r/security/secret-scanning/1"),
			LocationsURL: Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations"),
			State:        Ptr("open"),
			Resolution:   nil,
			ResolvedAt:   nil,
			ResolvedBy:   nil,
			SecretType:   Ptr("mailchimp_api_key"),
			Secret:       Ptr("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"),
		},
	}

	if !cmp.Equal(alerts, want) {
		t.Errorf("SecretScanning.ListAlertsForOrg returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListAlertsForOrg"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.ListAlertsForOrg(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.ListAlertsForOrg(ctx, "o", opts)
		return resp, err
	})
}

func TestSecretScanningService_ListAlertsForOrgListOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/secret-scanning/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open", "secret_type": "mailchimp_api_key", "per_page": "1", "page": "1", "sort": "updated", "direction": "asc"})

		fmt.Fprint(w, `[{
			"number": 1,
			"created_at": `+refTimeStr(1136178000)+`,
			"url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1",
			"html_url": "https://github.com/o/r/security/secret-scanning/1",
			"locations_url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations",
			"state": "open",
			"resolution": null,
			"resolved_at": null,
			"resolved_by": null,
			"secret_type": "mailchimp_api_key",
			"secret": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"
		}]`)
	})

	ctx := t.Context()

	// Testing pagination by index
	opts := &SecretScanningAlertListOptions{State: "open", SecretType: "mailchimp_api_key", ListOptions: ListOptions{Page: 1, PerPage: 1}, Direction: "asc", Sort: "updated"}

	alerts, _, err := client.SecretScanning.ListAlertsForOrg(ctx, "o", opts)
	if err != nil {
		t.Errorf("SecretScanning.ListAlertsForOrg returned error: %v", err)
	}

	want := []*SecretScanningAlert{
		{
			Number:       Ptr(1),
			CreatedAt:    refTimestamp(1136178000),
			URL:          Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1"),
			HTMLURL:      Ptr("https://github.com/o/r/security/secret-scanning/1"),
			LocationsURL: Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations"),
			State:        Ptr("open"),
			Resolution:   nil,
			ResolvedAt:   nil,
			ResolvedBy:   nil,
			SecretType:   Ptr("mailchimp_api_key"),
			Secret:       Ptr("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"),
		},
	}

	if !cmp.Equal(alerts, want) {
		t.Errorf("SecretScanning.ListAlertsForOrg returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListAlertsForOrg"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.ListAlertsForOrg(ctx, "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.ListAlertsForOrg(ctx, "o", opts)
		return resp, err
	})
}

func TestSecretScanningService_ListAlertsForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/secret-scanning/alerts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"state": "open", "secret_type": "mailchimp_api_key", "sort": "updated", "direction": "asc"})

		fmt.Fprint(w, `[{
			"number": 1,
			"created_at": `+refTimeStr(1136178000)+`,
			"url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1",
			"html_url": "https://github.com/o/r/security/secret-scanning/1",
			"locations_url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations",
			"state": "open",
			"resolution": null,
			"resolved_at": null,
			"resolved_by": null,
			"secret_type": "mailchimp_api_key",
			"secret": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"
		}]`)
	})

	ctx := t.Context()
	opts := &SecretScanningAlertListOptions{State: "open", SecretType: "mailchimp_api_key", Direction: "asc", Sort: "updated"}

	alerts, _, err := client.SecretScanning.ListAlertsForRepo(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("SecretScanning.ListAlertsForRepo returned error: %v", err)
	}

	want := []*SecretScanningAlert{
		{
			Number:       Ptr(1),
			CreatedAt:    refTimestamp(1136178000),
			URL:          Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1"),
			HTMLURL:      Ptr("https://github.com/o/r/security/secret-scanning/1"),
			LocationsURL: Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations"),
			State:        Ptr("open"),
			Resolution:   nil,
			ResolvedAt:   nil,
			ResolvedBy:   nil,
			SecretType:   Ptr("mailchimp_api_key"),
			Secret:       Ptr("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"),
		},
	}

	if !cmp.Equal(alerts, want) {
		t.Errorf("SecretScanning.ListAlertsForRepo returned %+v, want %+v", alerts, want)
	}

	const methodName = "ListAlertsForRepo"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.ListAlertsForRepo(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.ListAlertsForRepo(ctx, "o", "r", opts)
		return resp, err
	})
}

func TestSecretScanningService_GetAlert(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/secret-scanning/alerts/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")

		fmt.Fprint(w, `{
			"number": 1,
			"created_at": `+refTimeStr(1136178000)+`,
			"url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1",
			"html_url": "https://github.com/o/r/security/secret-scanning/1",
			"locations_url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations",
			"state": "open",
			"resolution": null,
			"resolved_at": null,
			"resolved_by": null,
			"secret_type": "mailchimp_api_key",
			"secret": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"
		}`)
	})

	ctx := t.Context()

	alert, _, err := client.SecretScanning.GetAlert(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("SecretScanning.GetAlert returned error: %v", err)
	}

	want := &SecretScanningAlert{
		Number:       Ptr(1),
		CreatedAt:    refTimestamp(1136178000),
		URL:          Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1"),
		HTMLURL:      Ptr("https://github.com/o/r/security/secret-scanning/1"),
		LocationsURL: Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations"),
		State:        Ptr("open"),
		Resolution:   nil,
		ResolvedAt:   nil,
		ResolvedBy:   nil,
		SecretType:   Ptr("mailchimp_api_key"),
		Secret:       Ptr("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"),
	}

	if !cmp.Equal(alert, want) {
		t.Errorf("SecretScanning.GetAlert returned %+v, want %+v", alert, want)
	}

	const methodName = "GetAlert"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.GetAlert(ctx, "\n", "\n", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.GetAlert(ctx, "o", "r", 1)
		return resp, err
	})
}

func TestSecretScanningService_UpdateAlert(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opts := &SecretScanningAlertUpdateOptions{State: "resolved", Resolution: Ptr("used_in_tests")}

	mux.HandleFunc("/repos/o/r/secret-scanning/alerts/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, opts)
		fmt.Fprint(w, `{
			"number": 1,
			"created_at": `+refTimeStr(1136178000)+`,
			"url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1",
			"html_url": "https://github.com/o/r/security/secret-scanning/1",
			"locations_url": "https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations",
			"state": "resolved",
			"resolution": "used_in_tests",
			"resolution_comment": "resolution comment",
			"resolved_at": `+refTimeStr(1136178001)+`,
			"resolved_by": null,
			"secret_type": "mailchimp_api_key",
			"secret": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"
		}`)
	})

	ctx := t.Context()
	alert, _, err := client.SecretScanning.UpdateAlert(ctx, "o", "r", 1, opts)
	if err != nil {
		t.Errorf("SecretScanning.UpdateAlert returned error: %v", err)
	}

	want := &SecretScanningAlert{
		Number:            Ptr(1),
		CreatedAt:         refTimestamp(1136178000),
		URL:               Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1"),
		HTMLURL:           Ptr("https://github.com/o/r/security/secret-scanning/1"),
		LocationsURL:      Ptr("https://api.github.com/repos/o/r/secret-scanning/alerts/1/locations"),
		State:             Ptr("resolved"),
		Resolution:        Ptr("used_in_tests"),
		ResolutionComment: Ptr("resolution comment"),
		ResolvedAt:        refTimestamp(1136178001),
		ResolvedBy:        nil,
		SecretType:        Ptr("mailchimp_api_key"),
		Secret:            Ptr("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX-us2"),
	}

	if !cmp.Equal(alert, want) {
		t.Errorf("SecretScanning.UpdateAlert returned %+v, want %+v", alert, want)
	}

	const methodName = "UpdateAlert"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.UpdateAlert(ctx, "\n", "\n", 1, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.UpdateAlert(ctx, "o", "r", 1, opts)
		return resp, err
	})
}

func TestSecretScanningService_ListLocationsForAlert(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/secret-scanning/alerts/1/locations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "1", "per_page": "100"})

		fmt.Fprint(w, `[{
			"type": "commit",
			"details": {
			  "path": "/example/secrets.txt",
			  "start_line": 1,
			  "end_line": 1,
			  "start_column": 1,
			  "end_column": 64,
			  "blob_sha": "af5626b4a114abcb82d63db7c8082c3c4756e51b",
			  "blob_url": "https://api.github.com/repos/o/r/git/blobs/af5626b4a114abcb82d63db7c8082c3c4756e51b",
			  "commit_sha": "f14d7debf9775f957cf4f1e8176da0786431f72b",
			  "commit_url": "https://api.github.com/repos/o/r/git/commits/f14d7debf9775f957cf4f1e8176da0786431f72b"
			}
		}]`)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 100}

	locations, _, err := client.SecretScanning.ListLocationsForAlert(ctx, "o", "r", 1, opts)
	if err != nil {
		t.Errorf("SecretScanning.ListLocationsForAlert returned error: %v", err)
	}

	want := []*SecretScanningAlertLocation{
		{
			Type: Ptr("commit"),
			Details: &SecretScanningAlertLocationDetails{
				Path:        Ptr("/example/secrets.txt"),
				Startline:   Ptr(1),
				EndLine:     Ptr(1),
				StartColumn: Ptr(1),
				EndColumn:   Ptr(64),
				BlobSHA:     Ptr("af5626b4a114abcb82d63db7c8082c3c4756e51b"),
				BlobURL:     Ptr("https://api.github.com/repos/o/r/git/blobs/af5626b4a114abcb82d63db7c8082c3c4756e51b"),
				CommitSHA:   Ptr("f14d7debf9775f957cf4f1e8176da0786431f72b"),
				CommitURL:   Ptr("https://api.github.com/repos/o/r/git/commits/f14d7debf9775f957cf4f1e8176da0786431f72b"),
			},
		},
	}

	if !cmp.Equal(locations, want) {
		t.Errorf("SecretScanning.ListLocationsForAlert returned %+v, want %+v", locations, want)
	}

	const methodName = "ListLocationsForAlert"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.ListLocationsForAlert(ctx, "\n", "\n", 1, opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.ListLocationsForAlert(ctx, "o", "r", 1, opts)
		return resp, err
	})
}

func TestSecretScanningService_CreatePushProtectionBypass(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	owner := "o"
	repo := "r"
	opts := PushProtectionBypassRequest{Reason: "valid reason", PlaceholderID: "bypass-123"}

	mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/secret-scanning/push-protection-bypasses", owner, repo), func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, opts)
		fmt.Fprint(w, `{
			"reason": "valid reason",
			"expire_at": `+refTimeStr(1136178000)+`,
			"token_type": "github_token"
		}`)
	})

	ctx := t.Context()

	bypass, _, err := client.SecretScanning.CreatePushProtectionBypass(ctx, owner, repo, opts)
	if err != nil {
		t.Errorf("SecretScanning.CreatePushProtectionBypass returned error: %v", err)
	}

	want := &PushProtectionBypass{
		Reason:    "valid reason",
		ExpireAt:  refTimestamp(1136178000),
		TokenType: "github_token",
	}

	if !cmp.Equal(bypass, want) {
		t.Errorf("SecretScanning.CreatePushProtectionBypass returned %+v, want %+v", bypass, want)
	}
	const methodName = "CreatePushProtectionBypass"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.CreatePushProtectionBypass(ctx, "\n", "\n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.CreatePushProtectionBypass(ctx, "o", "r", opts)
		return resp, err
	})
}

func TestSecretScanningService_GetScanHistory(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	owner := "o"
	repo := "r"

	mux.HandleFunc(fmt.Sprintf("/repos/%v/%v/secret-scanning/scan-history", owner, repo), func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"incremental_scans": [
				{
					"type": "incremental",
					"status": "success",
					"completed_at": `+refTimeStr(1136178000)+`,
					"started_at": `+refTimeStr(1136178001)+`
				}
			],
			"backfill_scans": [],
			"pattern_update_scans": [],
			"custom_pattern_backfill_scans": [
				{
					"type": "custom_backfill",
					"status": "in_progress",
					"completed_at": null,
					"started_at": `+refTimeStr(1136178002)+`,
					"pattern_slug": "my-custom-pattern",
					"pattern_scope": "organization"
				}
			]
		}`)
	})

	ctx := t.Context()

	history, _, err := client.SecretScanning.GetScanHistory(ctx, owner, repo)
	if err != nil {
		t.Errorf("SecretScanning.GetScanHistory returned error: %v", err)
	}

	want := &SecretScanningScanHistory{
		IncrementalScans: []*SecretsScan{
			{Type: "incremental", Status: "success", CompletedAt: refTimestamp(1136178000), StartedAt: refTimestamp(1136178001)},
		},
		BackfillScans:      []*SecretsScan{},
		PatternUpdateScans: []*SecretsScan{},
		CustomPatternBackfillScans: []*CustomPatternBackfillScan{
			{
				SecretsScan:  SecretsScan{Type: "custom_backfill", Status: "in_progress", CompletedAt: nil, StartedAt: refTimestamp(1136178002)},
				PatternSlug:  Ptr("my-custom-pattern"),
				PatternScope: Ptr("organization"),
			},
		},
	}

	if !cmp.Equal(history, want) {
		t.Errorf("SecretScanning.GetScanHistory returned %+v, want %+v", history, want)
	}
	const methodName = "GetScanHistory"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.GetScanHistory(ctx, "\n", "\n")
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.GetScanHistory(ctx, "o", "r")
		return resp, err
	})
}
