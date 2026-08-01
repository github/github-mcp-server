// Copyright 2024 The go-github AUTHORS. All rights reserved.
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

func TestRepositoriesService_ListAttestations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/attestations/digest", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"attestations": [
				{
					"repository_id": 1,
					"bundle": {}
				},
				{
					"repository_id": 2,
					"bundle": {}
				}
			]
		}`)
	})
	ctx := t.Context()
	attestations, _, err := client.Repositories.ListAttestations(ctx, "o", "r", "digest", &ListOptions{})
	if err != nil {
		t.Errorf("Repositories.ListAttestations returned error: %v", err)
	}

	want := &AttestationsResponse{
		Attestations: []*Attestation{
			{
				RepositoryID: 1,
				Bundle:       []byte(`{}`),
			},
			{
				RepositoryID: 2,
				Bundle:       []byte(`{}`),
			},
		},
	}

	if !cmp.Equal(attestations, want) {
		t.Errorf("Repositories.ListAttestations = %+v, want %+v", attestations, want)
	}
	const methodName = "ListAttestations"

	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListAttestations(ctx, "\n", "\n", "\n", &ListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListAttestations(ctx, "o", "r", "digest", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
