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

func TestSecretScanningService_ListCustomPatternsForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/secret-scanning/custom-patterns", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"state":           "published",
			"push_protection": "enabled",
			"sort":            "created",
			"direction":       "desc",
			"page":            "2",
		})
		fmt.Fprint(w, `[
			{
				"id": 1,
				"name": "Custom pattern",
				"pattern": "[A-Z]{2}-[0-9]{4}",
				"slug": "custom-pattern",
				"state": "published",
				"push_protection_enabled": true,
				"start_delimiter": "\\b",
				"end_delimiter": "\\b",
				"must_match": ["ID-.*"],
				"must_not_match": ["TEST-.*"],
				"custom_pattern_version": "v1",
				"created_at": `+referenceTimeStr+`,
				"updated_at": `+referenceTimeStr+`
			}
		]`)
	})

	ctx := t.Context()
	opts := &SecretScanningCustomPatternListOptions{
		State:          "published",
		PushProtection: "enabled",
		Sort:           "created",
		Direction:      "desc",
		ListOptions:    ListOptions{Page: 2},
	}
	patterns, _, err := client.SecretScanning.ListCustomPatternsForRepo(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("SecretScanning.ListCustomPatternsForRepo returned error: %v", err)
	}

	want := []*SecretScanningCustomPattern{
		{
			ID:                    1,
			Name:                  "Custom pattern",
			Pattern:               "[A-Z]{2}-[0-9]{4}",
			Slug:                  "custom-pattern",
			State:                 "published",
			PushProtectionEnabled: true,
			StartDelimiter:        Ptr(`\b`),
			EndDelimiter:          Ptr(`\b`),
			MustMatch:             []string{"ID-.*"},
			MustNotMatch:          []string{"TEST-.*"},
			CustomPatternVersion:  Ptr("v1"),
			CreatedAt:             &referenceTimestamp,
			UpdatedAt:             &referenceTimestamp,
		},
	}
	if !cmp.Equal(patterns, want) {
		t.Errorf("SecretScanning.ListCustomPatternsForRepo returned %+v, want %+v", patterns, want)
	}

	const methodName = "ListCustomPatternsForRepo"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.ListCustomPatternsForRepo(ctx, "\n", "\n", &SecretScanningCustomPatternListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.ListCustomPatternsForRepo(ctx, "o", "r", &SecretScanningCustomPatternListOptions{})
		return resp, err
	})
}

func TestSecretScanningService_CreateCustomPatternsForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SecretScanningCreateCustomPatternsRequest{
		Patterns: []*SecretScanningCustomPatternRequest{
			{
				Name:    "Custom pattern",
				Pattern: "[A-Z]{2}-[0-9]{4}",
			},
		},
	}

	mux.HandleFunc("/repos/o/r/secret-scanning/custom-patterns", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"created_patterns": [
				{
					"id": 1,
					"name": "Custom pattern",
					"pattern": "[A-Z]{2}-[0-9]{4}",
					"slug": "custom-pattern",
					"state": "published",
					"push_protection_enabled": false
				}
			]
		}`)
	})

	ctx := t.Context()
	result, _, err := client.SecretScanning.CreateCustomPatternsForRepo(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("SecretScanning.CreateCustomPatternsForRepo returned error: %v", err)
	}

	want := &SecretScanningCreateCustomPatternsResponse{
		CreatedPatterns: []*SecretScanningCustomPattern{
			{
				ID:                    1,
				Name:                  "Custom pattern",
				Pattern:               "[A-Z]{2}-[0-9]{4}",
				Slug:                  "custom-pattern",
				State:                 "published",
				PushProtectionEnabled: false,
			},
		},
	}
	if !cmp.Equal(result, want) {
		t.Errorf("SecretScanning.CreateCustomPatternsForRepo returned %+v, want %+v", result, want)
	}

	const methodName = "CreateCustomPatternsForRepo"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.CreateCustomPatternsForRepo(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.CreateCustomPatternsForRepo(ctx, "o", "r", input)
		return resp, err
	})
}

func TestSecretScanningService_UpdateCustomPatternForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SecretScanningUpdateCustomPatternRequest{
		Pattern:              Ptr("[A-Z]{3}-[0-9]{4}"),
		CustomPatternVersion: Ptr("v1"),
	}

	mux.HandleFunc("/repos/o/r/secret-scanning/custom-patterns/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{
			"id": 1,
			"name": "Custom pattern",
			"pattern": "[A-Z]{3}-[0-9]{4}",
			"slug": "custom-pattern",
			"state": "published",
			"push_protection_enabled": false,
			"custom_pattern_version": "v2"
		}`)
	})

	ctx := t.Context()
	pattern, _, err := client.SecretScanning.UpdateCustomPatternForRepo(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("SecretScanning.UpdateCustomPatternForRepo returned error: %v", err)
	}

	want := &SecretScanningCustomPattern{
		ID:                    1,
		Name:                  "Custom pattern",
		Pattern:               "[A-Z]{3}-[0-9]{4}",
		Slug:                  "custom-pattern",
		State:                 "published",
		PushProtectionEnabled: false,
		CustomPatternVersion:  Ptr("v2"),
	}
	if !cmp.Equal(pattern, want) {
		t.Errorf("SecretScanning.UpdateCustomPatternForRepo returned %+v, want %+v", pattern, want)
	}

	const methodName = "UpdateCustomPatternForRepo"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.SecretScanning.UpdateCustomPatternForRepo(ctx, "\n", "\n", 1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		_, resp, err := client.SecretScanning.UpdateCustomPatternForRepo(ctx, "o", "r", 1, input)
		return resp, err
	})
}

func TestSecretScanningService_DeleteCustomPatternsForRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := SecretScanningDeleteCustomPatternsRequest{
		Patterns: []*SecretScanningCustomPatternToDelete{
			{PatternID: 1},
		},
	}

	mux.HandleFunc("/repos/o/r/secret-scanning/custom-patterns", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		testJSONBody(t, r, input)
	})

	ctx := t.Context()
	_, err := client.SecretScanning.DeleteCustomPatternsForRepo(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("SecretScanning.DeleteCustomPatternsForRepo returned error: %v", err)
	}

	const methodName = "DeleteCustomPatternsForRepo"

	testBadOptions(t, methodName, func() (err error) {
		_, err = client.SecretScanning.DeleteCustomPatternsForRepo(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.SecretScanning.DeleteCustomPatternsForRepo(ctx, "o", "r", input)
	})
}
