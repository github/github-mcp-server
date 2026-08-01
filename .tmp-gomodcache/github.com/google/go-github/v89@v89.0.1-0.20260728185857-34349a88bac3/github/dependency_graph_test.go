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

func TestDependencyGraphService_GetSBOM(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/owner/repo/dependency-graph/sbom", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
   "sbom":{
      "creationInfo":{
         "created":`+referenceTimeStr+`
      },
      "name":"owner/repo",
      "packages":[
                {
                "name":"rubygems:rails",
                "versionInfo":"1.0.0"
                }
            ]
        }
    }`)
	})

	ctx := t.Context()
	sbom, _, err := client.DependencyGraph.GetSBOM(ctx, "owner", "repo")
	if err != nil {
		t.Errorf("DependencyGraph.GetSBOM returned error: %v", err)
	}

	want := &SBOM{
		&SBOMInfo{
			CreationInfo: &CreationInfo{
				Created: &referenceTimestamp,
			},
			Name: Ptr("owner/repo"),
			Packages: []*RepoDependencies{
				{
					Name:        Ptr("rubygems:rails"),
					VersionInfo: Ptr("1.0.0"),
				},
			},
		},
	}

	if !cmp.Equal(sbom, want) {
		t.Errorf("DependencyGraph.GetSBOM returned %+v, want %+v", sbom, want)
	}

	const methodName = "GetSBOM"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.DependencyGraph.GetSBOM(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.DependencyGraph.GetSBOM(ctx, "owner", "repo")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
