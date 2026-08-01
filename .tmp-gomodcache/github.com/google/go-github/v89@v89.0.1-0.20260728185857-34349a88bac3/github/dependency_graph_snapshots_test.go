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

func TestDependencyGraphService_CreateSnapshot(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	snapshot := &DependencyGraphSnapshot{
		Version: 0,
		Sha:     Ptr("ce587453ced02b1526dfb4cb910479d431683101"),
		Ref:     Ptr("refs/heads/main"),
		Job: &DependencyGraphSnapshotJob{
			Correlator: Ptr("yourworkflowname_youractionname"),
			ID:         Ptr("yourrunid"),
			HTMLURL:    Ptr("https://example.com"),
		},
		Detector: &DependencyGraphSnapshotDetector{
			Name:    Ptr("octo-detector"),
			Version: Ptr("0.0.1"),
			URL:     Ptr("https://github.com/octo-org/octo-repo"),
		},
		Scanned: &referenceTimestamp,
		Metadata: map[string]any{
			"key1": "value1",
			"key2": "value2",
		},
		Manifests: map[string]*DependencyGraphSnapshotManifest{
			"package-lock.json": {
				Name: Ptr("package-lock.json"),
				File: &DependencyGraphSnapshotManifestFile{SourceLocation: Ptr("src/package-lock.json")},
				Metadata: map[string]any{
					"key1": "value1",
					"key2": "value2",
				},
				Resolved: map[string]*DependencyGraphSnapshotResolvedDependency{
					"@actions/core": {
						PackageURL:   Ptr("pkg:/npm/%40actions/core@1.1.9"),
						Relationship: Ptr("direct"),
						Scope:        Ptr("runtime"),
						Metadata: map[string]any{
							"licenses": "MIT",
						},
						Dependencies: []string{"@actions/http-client"},
					},
					"@actions/http-client": {
						PackageURL:   Ptr("pkg:/npm/%40actions/http-client@1.0.7"),
						Relationship: Ptr("indirect"),
						Scope:        Ptr("runtime"),
						Dependencies: []string{"tunnel"},
					},
					"tunnel": {
						PackageURL:   Ptr("pkg:/npm/tunnel@0.0.6"),
						Relationship: Ptr("indirect"),
						Scope:        Ptr("runtime"),
					},
				},
			},
		},
	}

	mux.HandleFunc("/repos/o/r/dependency-graph/snapshots", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, snapshot)
		fmt.Fprint(w, `{"id":12345,"created_at":`+referenceTimeStr+`,"message":"Dependency results for the repo have been successfully updated.","result":"SUCCESS"}`)
	})

	ctx := t.Context()
	snapshotCreationData, _, err := client.DependencyGraph.CreateSnapshot(ctx, "o", "r", snapshot)
	if err != nil {
		t.Errorf("DependencyGraph.CreateSnapshot returned error: %v", err)
	}

	want := &DependencyGraphSnapshotCreationData{
		ID:        12345,
		CreatedAt: &referenceTimestamp,
		Message:   Ptr("Dependency results for the repo have been successfully updated."),
		Result:    Ptr("SUCCESS"),
	}
	if !cmp.Equal(snapshotCreationData, want) {
		t.Errorf("DependencyGraph.CreateSnapshot returned %+v, want %+v", snapshotCreationData, want)
	}

	const methodName = "CreateSnapshot"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.DependencyGraph.CreateSnapshot(ctx, "o", "r", snapshot)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
