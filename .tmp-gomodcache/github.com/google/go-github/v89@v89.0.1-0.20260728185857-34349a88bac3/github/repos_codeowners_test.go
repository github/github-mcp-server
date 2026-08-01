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

func TestRepositoriesService_GetCodeownersErrors_noRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/codeowners/errors", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3)
		fmt.Fprint(w, `{
		  "errors": [
			{
			  "line": 1,
			  "column": 1,
			  "kind": "Invalid pattern",
			  "source": "***/*.rb @monalisa",
			  "suggestion": "Did you mean **/*.rb?",
			  "message": "Invalid pattern on line 3: Did you mean **/*.rb?\n\n  ***/*.rb @monalisa\n  ^",
			  "path": ".github/CODEOWNERS"
			}
		  ]
		}
	`)
	})

	ctx := t.Context()
	codeownersErrors, _, err := client.Repositories.GetCodeownersErrors(ctx, "o", "r", nil)
	if err != nil {
		t.Errorf("Repositories.GetCodeownersErrors returned error: %v", err)
	}

	want := &CodeownersErrors{
		Errors: []*CodeownersError{
			{
				Line:       1,
				Column:     1,
				Kind:       "Invalid pattern",
				Source:     "***/*.rb @monalisa",
				Suggestion: Ptr("Did you mean **/*.rb?"),
				Message:    "Invalid pattern on line 3: Did you mean **/*.rb?\n\n  ***/*.rb @monalisa\n  ^",
				Path:       ".github/CODEOWNERS",
			},
		},
	}
	if !cmp.Equal(codeownersErrors, want) {
		t.Errorf("Repositories.GetCodeownersErrors returned %+v, want %+v", codeownersErrors, want)
	}

	const methodName = "GetCodeownersErrors"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetCodeownersErrors(ctx, "\n", "\n", nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetCodeownersErrors(ctx, "o", "r", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetCodeownersErrors_specificRef(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/codeowners/errors", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", mediaTypeV3)
		testFormValues(t, r, values{"ref": "mybranch"})
		fmt.Fprint(w, `{
		  "errors": [
			{
			  "line": 1,
			  "column": 1,
			  "kind": "Invalid pattern",
			  "source": "***/*.rb @monalisa",
			  "suggestion": "Did you mean **/*.rb?",
			  "message": "Invalid pattern on line 3: Did you mean **/*.rb?\n\n  ***/*.rb @monalisa\n  ^",
			  "path": ".github/CODEOWNERS"
			}
		  ]
		}
	`)
	})

	opts := &GetCodeownersErrorsOptions{Ref: "mybranch"}
	ctx := t.Context()
	codeownersErrors, _, err := client.Repositories.GetCodeownersErrors(ctx, "o", "r", opts)
	if err != nil {
		t.Errorf("Repositories.GetCodeownersErrors returned error: %v", err)
	}

	want := &CodeownersErrors{
		Errors: []*CodeownersError{
			{
				Line:       1,
				Column:     1,
				Kind:       "Invalid pattern",
				Source:     "***/*.rb @monalisa",
				Suggestion: Ptr("Did you mean **/*.rb?"),
				Message:    "Invalid pattern on line 3: Did you mean **/*.rb?\n\n  ***/*.rb @monalisa\n  ^",
				Path:       ".github/CODEOWNERS",
			},
		},
	}
	if !cmp.Equal(codeownersErrors, want) {
		t.Errorf("Repositories.GetCodeownersErrors returned %+v, want %+v", codeownersErrors, want)
	}

	const methodName = "GetCodeownersErrors"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetCodeownersErrors(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetCodeownersErrors(ctx, "o", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
