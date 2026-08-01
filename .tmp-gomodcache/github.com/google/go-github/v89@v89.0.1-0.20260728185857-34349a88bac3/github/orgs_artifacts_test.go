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

func TestOrganizationsService_CreateArtifactDeploymentRecord(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateArtifactDeploymentRequest{
		Name:               "test-n",
		Digest:             "sha256:123",
		Version:            Ptr("v1.0.0"),
		Status:             "deployed",
		LogicalEnvironment: "prod",
		DeploymentName:     "dep-1",
		RuntimeRisks:       []DeploymentRuntimeRisk{DeploymentRuntimeRiskCriticalResource, DeploymentRuntimeRiskInternetExposed},
		GithubRepository:   Ptr("octo-org/octo-repo"),
		Tags: map[string]string{
			"data-access": "sensitive",
		},
	}

	mux.HandleFunc("/orgs/o/artifacts/metadata/deployment-record", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"total_count":1,"deployment_records":[{"id":1}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Organizations.CreateArtifactDeploymentRecord(ctx, "o", input)
	if err != nil {
		t.Errorf("CreateArtifactDeploymentRecord returned error: %v", err)
	}

	want := &ArtifactDeploymentResponse{
		TotalCount:        Ptr(1),
		DeploymentRecords: []*ArtifactDeploymentRecord{{ID: Ptr(int64(1))}},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("CreateArtifactDeploymentRecord returned %+v, want %+v", got, want)
	}

	const methodName = "CreateArtifactDeploymentRecord"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateArtifactDeploymentRecord(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateArtifactDeploymentRecord(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_SetClusterDeploymentRecords(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := ClusterDeploymentRecordsRequest{
		LogicalEnvironment:  "prod",
		PhysicalEnvironment: Ptr("pacific-east"),
		Deployments: []*ClusterArtifactDeployment{
			{
				Name:           "awesome-image",
				Digest:         "sha256:abc",
				DeploymentName: "dep-1",
				Version:        Ptr("v2.0"),
				Status:         "deployed",
			},
		},
	}

	mux.HandleFunc("/orgs/o/artifacts/metadata/deployment-record/cluster/c1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"total_count":1,"deployment_records":[{"id":2}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Organizations.SetClusterDeploymentRecords(ctx, "o", "c1", input)
	if err != nil {
		t.Errorf("SetClusterDeploymentRecords returned error: %v", err)
	}

	want := &ArtifactDeploymentResponse{
		TotalCount:        Ptr(1),
		DeploymentRecords: []*ArtifactDeploymentRecord{{ID: Ptr(int64(2))}},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("SetClusterDeploymentRecords returned %+v, want %+v", got, want)
	}

	const methodName = "SetClusterDeploymentRecords"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.SetClusterDeploymentRecords(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.SetClusterDeploymentRecords(ctx, "o", "c1", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_CreateArtifactStorageRecord(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateArtifactStorageRequest{
		Name:             "libfoo",
		Digest:           "sha256:123",
		Version:          Ptr("v1.2.3"),
		Path:             Ptr("target/libs"),
		GithubRepository: Ptr("org/repo"),
		RegistryURL:      "https://reg.example.com",
		Status:           Ptr("active"),
	}

	mux.HandleFunc("/orgs/o/artifacts/metadata/storage-record", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"total_count":1,"storage_records":[{"name":"libfoo"}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Organizations.CreateArtifactStorageRecord(ctx, "o", input)
	if err != nil {
		t.Errorf("CreateArtifactStorageRecord returned error: %v", err)
	}

	want := &ArtifactStorageResponse{
		TotalCount:     Ptr(1),
		StorageRecords: []*ArtifactStorageRecord{{Name: Ptr("libfoo")}},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("CreateArtifactStorageRecord returned %+v, want %+v", got, want)
	}

	const methodName = "CreateArtifactStorageRecord"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.CreateArtifactStorageRecord(ctx, "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.CreateArtifactStorageRecord(ctx, "o", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListArtifactDeploymentRecords(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/artifacts/sha256:abc/metadata/deployment-records", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{})
		fmt.Fprint(w, `{"total_count":1,"deployment_records":[{"id":1, "runtime_risks": ["sensitive-data"]}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Organizations.ListArtifactDeploymentRecords(ctx, "o", "sha256:abc")
	if err != nil {
		t.Errorf("ListArtifactDeploymentRecords returned error: %v", err)
	}

	want := &ArtifactDeploymentResponse{
		TotalCount: Ptr(1),
		DeploymentRecords: []*ArtifactDeploymentRecord{
			{ID: Ptr(int64(1)), RuntimeRisks: []DeploymentRuntimeRisk{DeploymentRuntimeRiskSensitiveData}},
		},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("ListArtifactDeploymentRecords returned %+v, want %+v", got, want)
	}

	const methodName = "ListArtifactDeploymentRecords"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListArtifactDeploymentRecords(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListArtifactDeploymentRecords(ctx, "o", "sha256:abc")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_ListArtifactStorageRecords(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/artifacts/sha256:abc/metadata/storage-records", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{})
		fmt.Fprint(w, `{"total_count":1,"storage_records":[{"name":"libfoo"}]}`)
	})

	ctx := t.Context()
	got, _, err := client.Organizations.ListArtifactStorageRecords(ctx, "o", "sha256:abc")
	if err != nil {
		t.Errorf("ListArtifactStorageRecords returned error: %v", err)
	}

	want := &ArtifactStorageResponse{
		TotalCount:     Ptr(1),
		StorageRecords: []*ArtifactStorageRecord{{Name: Ptr("libfoo")}},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("ListArtifactStorageRecords returned %+v, want %+v", got, want)
	}

	const methodName = "ListArtifactStorageRecords"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListArtifactStorageRecords(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListArtifactStorageRecords(ctx, "o", "sha256:abc")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
