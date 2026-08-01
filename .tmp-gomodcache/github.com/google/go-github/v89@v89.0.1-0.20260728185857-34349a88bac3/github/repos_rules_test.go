// Copyright 2023 The go-github AUTHORS. All rights reserved.
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

func TestRepositoriesService_ListRulesForBranch(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rules/branches/branch", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
			  "ruleset_id": 42069,
			  "ruleset_source_type": "Repository",
			  "ruleset_source": "google/a",
			  "type": "creation"
			},
			{
			  "ruleset_id": 42069,
			  "ruleset_source_type": "Organization",
			  "ruleset_source": "google",
			  "type": "update",
			  "parameters": {
			    "update_allows_fetch_and_merge": true
			  }
			}
		]`)
	})

	ctx := t.Context()
	rules, _, err := client.Repositories.ListRulesForBranch(ctx, "o", "repo", "branch", nil)
	if err != nil {
		t.Errorf("Repositories.ListRulesForBranch returned error: %v", err)
	}

	want := &BranchRules{
		Creation: []*BranchRuleMetadata{{RulesetSourceType: RulesetSourceTypeRepository, RulesetSource: "google/a", RulesetID: 42069}},
		Update:   []*UpdateBranchRule{{BranchRuleMetadata: BranchRuleMetadata{RulesetSourceType: RulesetSourceTypeOrganization, RulesetSource: "google", RulesetID: 42069}, Parameters: UpdateRuleParameters{UpdateAllowsFetchAndMerge: true}}},
	}

	if !cmp.Equal(rules, want) {
		t.Errorf("Repositories.ListRulesForBranch returned %+v, want %+v", rules, want)
	}

	const methodName = "ListRulesForBranch"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListRulesForBranch(ctx, "o", "repo", "branch", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListRulesForBranchIter(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)
	var callNum int
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		callNum++
		switch callNum {
		case 1:
			w.Header().Set("Link", `<https://api.github.com/?page=1>; rel="next"`)
			fmt.Fprint(w, `[{"type":"creation"},{"type":"deletion"},{"type":"update"}]`)
		case 2:
			fmt.Fprint(w, `[{"type":"creation"},{"type":"deletion"},{"type":"update"},{"type":"workflows"}]`)
		case 3, 5:
			fmt.Fprint(w, `[{"type":"creation"},{"type":"deletion"}]`)
		case 4:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	iter := client.Repositories.ListRulesForBranchIter(t.Context(), "o", "r", "b", nil)
	var gotItems int
	for _, err := range iter {
		gotItems++
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	if want := 7; gotItems != want {
		t.Errorf("client.Repositories.ListRulesForBranchIter call 1 got %v items; want %v", gotItems, want)
	}

	opts := &ListOptions{}
	iter = client.Repositories.ListRulesForBranchIter(t.Context(), "o", "r", "b", opts)
	gotItems = 0
	for _, err := range iter {
		gotItems++
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}
	if want := 2; gotItems != want {
		t.Errorf("client.Repositories.ListRulesForBranchIter call 2 got %v items; want %v", gotItems, want)
	}

	iter = client.Repositories.ListRulesForBranchIter(t.Context(), "o", "r", "b", nil)
	gotItems = 0
	for _, err := range iter {
		gotItems++
		if err == nil {
			t.Error("expected error; got nil")
		}
	}
	if gotItems != 1 {
		t.Errorf("client.Repositories.ListRulesForBranchIter call 3 got %v items; want 1 (an error)", gotItems)
	}

	iter = client.Repositories.ListRulesForBranchIter(t.Context(), "o", "r", "b", nil)
	gotItems = 0
	iter(func(_ any, err error) bool {
		gotItems++
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		return false
	})
	if gotItems != 1 {
		t.Errorf("client.Repositories.ListRulesForBranchIter call 4 got %v items; want 1 (an error)", gotItems)
	}
}

func TestRepositoriesService_UpdateRuleset_OmitZero_Nil(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := RepositoryRuleset{
		Name:         "ruleset",
		Enforcement:  RulesetEnforcementActive,
		BypassActors: nil,
	}

	mux.HandleFunc("/repos/o/repo/rulesets/42", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)

		fmt.Fprint(w, `{
			"id": 42,
			"name": "ruleset",
			"source": "o/repo",
			"enforcement": "active"
		}`)
	})

	ctx := t.Context()

	_, _, err := client.Repositories.UpdateRuleset(ctx, "o", "repo", 42, input)
	if err != nil {
		t.Errorf("Repositories.UpdateRuleset returned error: %v", err)
	}
}

func TestRepositoriesService_UpdateRuleset_OmitZero_EmptySlice(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := RepositoryRuleset{
		Name:         "ruleset",
		Enforcement:  RulesetEnforcementActive,
		BypassActors: []*BypassActor{},
	}

	// Scenario 2: User passes empty slice (non-zero value).
	mux.HandleFunc("/repos/o/repo/rulesets/42", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)

		fmt.Fprint(w, `{
			"id": 42,
			"name": "ruleset",
			"source": "o/repo",
			"enforcement": "active",
			"bypass_actors": []
		}`)
	})

	ctx := t.Context()
	_, _, err := client.Repositories.UpdateRuleset(ctx, "o", "repo", 42, input)
	if err != nil {
		t.Errorf("Repositories.UpdateRuleset returned error: %v", err)
	}
}

func TestRepositoriesService_ListRulesForBranch_ListOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rules/branches/branch", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "2",
			"per_page": "35",
		})
		fmt.Fprint(w, `[
			{
			  "ruleset_id": 42069,
			  "type": "creation"
			}
		]`)
	})

	opts := &ListOptions{Page: 2, PerPage: 35}
	ctx := t.Context()
	rules, _, err := client.Repositories.ListRulesForBranch(ctx, "o", "repo", "branch", opts)
	if err != nil {
		t.Errorf("Repositories.ListRulesForBranch returned error: %v", err)
	}

	want := &BranchRules{
		Creation: []*BranchRuleMetadata{{RulesetID: 42069}},
	}

	if !cmp.Equal(rules, want) {
		t.Errorf("Repositories.ListRulesForBranch returned %+v, want %+v", rules, want)
	}

	const methodName = "ListRulesForBranch"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListRulesForBranch(ctx, "\n", "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListRulesForBranch(ctx, "o", "repo", "branch", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetAllRulesets(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rulesets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprintf(w, `[
			{
			  "id": 42,
			  "name": "ruleset",
			  "source_type": "Repository",
			  "source": "o/repo",
			  "enforcement": "active",
			  "created_at": %[1]s,
			  "updated_at": %[1]s
			},
			{
			  "id": 314,
			  "name": "Another ruleset",
			  "source_type": "Repository",
			  "source": "o/repo",
			  "enforcement": "active",
			  "created_at": %[1]s,
			  "updated_at": %[1]s
			}
		]`, referenceTimeStr)
	})

	ctx := t.Context()
	ruleSet, _, err := client.Repositories.GetAllRulesets(ctx, "o", "repo", nil)
	if err != nil {
		t.Errorf("Repositories.GetAllRulesets returned error: %v", err)
	}

	want := []*RepositoryRuleset{
		{
			ID:          Ptr(int64(42)),
			Name:        "ruleset",
			SourceType:  Ptr(RulesetSourceTypeRepository),
			Source:      "o/repo",
			Enforcement: RulesetEnforcementActive,
			CreatedAt:   &Timestamp{referenceTime},
			UpdatedAt:   &Timestamp{referenceTime},
		},
		{
			ID:          Ptr(int64(314)),
			Name:        "Another ruleset",
			SourceType:  Ptr(RulesetSourceTypeRepository),
			Source:      "o/repo",
			Enforcement: RulesetEnforcementActive,
			CreatedAt:   &Timestamp{referenceTime},
			UpdatedAt:   &Timestamp{referenceTime},
		},
	}
	if !cmp.Equal(ruleSet, want) {
		t.Errorf("Repositories.GetAllRulesets returned %+v, want %+v", ruleSet, want)
	}

	const methodName = "GetAllRulesets"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetAllRulesets(ctx, "o", "repo", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetAllRulesets_ListOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rulesets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"includes_parents": "false",
			"page":             "2",
			"per_page":         "35",
		})
		fmt.Fprint(w, `[
			{
			  "id": 42
			}
		]`)
	})

	opts := &RepositoryListRulesetsOptions{
		IncludesParents: Ptr(false),
		ListOptions: ListOptions{
			Page:    2,
			PerPage: 35,
		},
	}
	ctx := t.Context()
	ruleSet, _, err := client.Repositories.GetAllRulesets(ctx, "o", "repo", opts)
	if err != nil {
		t.Errorf("Repositories.GetAllRulesets returned error: %v", err)
	}

	want := []*RepositoryRuleset{
		{
			ID: Ptr(int64(42)),
		},
	}
	if !cmp.Equal(ruleSet, want) {
		t.Errorf("Repositories.GetAllRulesets returned %+v, want %+v", ruleSet, want)
	}

	const methodName = "GetAllRulesets"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetAllRulesets(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetAllRulesets(ctx, "o", "repo", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateRuleset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rulesets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
			"id": 42,
			"name": "ruleset",
			"source_type": "Repository",
			"source": "o/repo",
			"enforcement": "active"
		}`)
	})

	ctx := t.Context()
	ruleSet, _, err := client.Repositories.CreateRuleset(ctx, "o", "repo", RepositoryRuleset{
		Name:        "ruleset",
		Enforcement: RulesetEnforcementActive,
	})
	if err != nil {
		t.Errorf("Repositories.CreateRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(42)),
		Name:        "ruleset",
		SourceType:  Ptr(RulesetSourceTypeRepository),
		Source:      "o/repo",
		Enforcement: RulesetEnforcementActive,
	}
	if !cmp.Equal(ruleSet, want) {
		t.Errorf("Repositories.CreateRuleset returned %+v, want %+v", ruleSet, want)
	}

	const methodName = "CreateRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateRuleset(ctx, "o", "repo", RepositoryRuleset{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateRulesetWithPushRules(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rulesets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
			"id": 42,
			"name": "ruleset",
			"source_type": "Repository",
			"source": "o/repo",
			"enforcement": "active",
			"target": "push",
			"rules": [
				{
					"type": "file_path_restriction",
					"parameters": {
						"restricted_file_paths": ["/a/file"]
					}
				},
				{
					"type": "max_file_path_length",
					"parameters": {
						"max_file_path_length": 255
					}
				},
				{
					"type": "file_extension_restriction",
					"parameters": {
						"restricted_file_extensions": [".exe"]
					}
				},
				{
					"type": "max_file_size",
					"parameters": {
						"max_file_size": 1024
					}
				}
			]
		}`)
	})

	ctx := t.Context()
	ruleSet, _, err := client.Repositories.CreateRuleset(ctx, "o", "repo", RepositoryRuleset{
		Name:        "ruleset",
		Enforcement: RulesetEnforcementActive,
	})
	if err != nil {
		t.Errorf("Repositories.CreateRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(42)),
		Name:        "ruleset",
		SourceType:  Ptr(RulesetSourceTypeRepository),
		Source:      "o/repo",
		Target:      Ptr(RulesetTargetPush),
		Enforcement: RulesetEnforcementActive,
		Rules: &RepositoryRulesetRules{
			FilePathRestriction:      &FilePathRestrictionRuleParameters{RestrictedFilePaths: []string{"/a/file"}},
			MaxFilePathLength:        &MaxFilePathLengthRuleParameters{MaxFilePathLength: 255},
			FileExtensionRestriction: &FileExtensionRestrictionRuleParameters{RestrictedFileExtensions: []string{".exe"}},
			MaxFileSize:              &MaxFileSizeRuleParameters{MaxFileSize: 1024},
		},
	}
	if !cmp.Equal(ruleSet, want) {
		t.Errorf("Repositories.CreateRuleset returned %+v, want %+v", ruleSet, want)
	}

	const methodName = "CreateRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateRuleset(ctx, "o", "repo", RepositoryRuleset{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetRuleset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rulesets/42", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 42,
			"name": "ruleset",
			"source_type": "Organization",
			"source": "o",
			"enforcement": "active",
			"created_at": `+referenceTimeStr+`,
			"updated_at": `+referenceTimeStr+`
		}`)
	})

	ctx := t.Context()
	ruleSet, _, err := client.Repositories.GetRuleset(ctx, "o", "repo", 42, true)
	if err != nil {
		t.Errorf("Repositories.GetRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(42)),
		Name:        "ruleset",
		SourceType:  Ptr(RulesetSourceTypeOrganization),
		Source:      "o",
		Enforcement: RulesetEnforcementActive,
		CreatedAt:   &Timestamp{referenceTime},
		UpdatedAt:   &Timestamp{referenceTime},
	}
	if !cmp.Equal(ruleSet, want) {
		t.Errorf("Repositories.GetRuleset returned %+v, want %+v", ruleSet, want)
	}

	const methodName = "GetRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetRuleset(ctx, "o", "repo", 42, true)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdateRuleset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rulesets/42", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{
			"id": 42,
			"name": "ruleset",
			"source_type": "Repository",
			"source": "o/repo",
			"enforcement": "active"
		}`)
	})

	ctx := t.Context()
	ruleSet, _, err := client.Repositories.UpdateRuleset(ctx, "o", "repo", 42, RepositoryRuleset{
		Name:        "ruleset",
		Enforcement: RulesetEnforcementActive,
	})
	if err != nil {
		t.Errorf("Repositories.UpdateRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(42)),
		Name:        "ruleset",
		SourceType:  Ptr(RulesetSourceTypeRepository),
		Source:      "o/repo",
		Enforcement: "active",
	}

	if !cmp.Equal(ruleSet, want) {
		t.Errorf("Repositories.UpdateRuleset returned %+v, want %+v", ruleSet, want)
	}

	const methodName = "UpdateRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdateRuleset(ctx, "o", "repo", 42, RepositoryRuleset{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DeleteRuleset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/repo/rulesets/42", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Repositories.DeleteRuleset(ctx, "o", "repo", 42)
	if err != nil {
		t.Errorf("Repositories.DeleteRuleset returned error: %v", err)
	}

	const methodName = "DeleteRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeleteRuleset(ctx, "o", "repo", 42)
	})
}
