// Copyright 2025 The go-github AUTHORS. All rights reserved.
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

func TestEnterpriseService_CheckSystemRequirements(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/checks/system-requirements", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"status": "OK",
			"nodes": [{
				"hostname": "primary",
				"status": "OK",
				"roles_status": [{
					"status": "OK",
					"role": "ConsulServer"
					}
				]}
			]}`)
	})

	ctx := t.Context()
	systemRequirements, _, err := client.Enterprise.CheckSystemRequirements(ctx)
	if err != nil {
		t.Errorf("Enterprise.CheckSystemRequirements returned error: %v", err)
	}

	want := &SystemRequirements{
		Status: Ptr("OK"),
		Nodes: []*SystemRequirementsNode{{
			Hostname: Ptr("primary"),
			Status:   Ptr("OK"),
			RolesStatus: []*SystemRequirementsNodeRoleStatus{{
				Status: Ptr("OK"),
				Role:   Ptr("ConsulServer"),
			}},
		}},
	}
	if !cmp.Equal(systemRequirements, want) {
		t.Errorf("Enterprise.CheckSystemRequirements returned %+v, want %+v", systemRequirements, want)
	}

	const methodName = "CheckSystemRequirements"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CheckSystemRequirements(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ClusterStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/cluster/status", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
    		"status": "OK",
   			"nodes": [{
            	"hostname": "primary",
            	"status": "OK",
            	"services": []
        	}]
		}`)
	})

	ctx := t.Context()
	clusterStatus, _, err := client.Enterprise.ClusterStatus(ctx)
	if err != nil {
		t.Errorf("Enterprise.ClusterStatus returned error: %v", err)
	}

	want := &ClusterStatus{
		Status: Ptr("OK"),
		Nodes: []*ClusterStatusNode{{
			Hostname: Ptr("primary"),
			Status:   Ptr("OK"),
			Services: []*ClusterStatusNodeServiceItem{},
		}},
	}
	if !cmp.Equal(clusterStatus, want) {
		t.Errorf("Enterprise.ClusterStatus returned %+v, want %+v", clusterStatus, want)
	}

	const methodName = "ClusterStatus"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ClusterStatus(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_ReplicationStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/replication/status", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"uuid":          "1234-1234",
			"cluster_roles": "primary",
		})
		fmt.Fprint(w, `{
    		"status": "OK",
   			"nodes": [{
            	"hostname": "primary",
            	"status": "OK",
            	"services": []
        	}]
		}`)
	})

	opt := &NodeQueryOptions{
		UUID: Ptr("1234-1234"), ClusterRoles: Ptr("primary"),
	}
	ctx := t.Context()
	replicationStatus, _, err := client.Enterprise.ReplicationStatus(ctx, opt)
	if err != nil {
		t.Errorf("Enterprise.ReplicationStatus returned error: %v", err)
	}

	want := &ClusterStatus{
		Status: Ptr("OK"),
		Nodes: []*ClusterStatusNode{{
			Hostname: Ptr("primary"),
			Status:   Ptr("OK"),
			Services: []*ClusterStatusNodeServiceItem{},
		}},
	}
	if !cmp.Equal(replicationStatus, want) {
		t.Errorf("Enterprise.ReplicationStatus returned %+v, want %+v", replicationStatus, want)
	}

	const methodName = "ReplicationStatus"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.ReplicationStatus(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_Versions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/version", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"uuid":          "1234-1234",
			"cluster_roles": "primary",
		})
		fmt.Fprint(w, `[{
    		"hostname": "primary",
   			"version": {
            	"version": "3.9.0",
            	"platform": "azure",
            	"build_id": "fc542058b5",
				"build_date": "2023-05-02"
        	}
		}]`)
	})

	opt := &NodeQueryOptions{
		UUID: Ptr("1234-1234"), ClusterRoles: Ptr("primary"),
	}
	ctx := t.Context()
	releaseVersions, _, err := client.Enterprise.GetNodeReleaseVersions(ctx, opt)
	if err != nil {
		t.Errorf("Enterprise.GetNodeReleaseVersions returned error: %v", err)
	}

	want := []*NodeReleaseVersion{{
		Hostname: Ptr("primary"),
		Version: &ReleaseVersion{
			Version:   Ptr("3.9.0"),
			Platform:  Ptr("azure"),
			BuildID:   Ptr("fc542058b5"),
			BuildDate: Ptr("2023-05-02"),
		},
	}}
	if !cmp.Equal(releaseVersions, want) {
		t.Errorf("Enterprise.GetNodeReleaseVersions returned %+v, want %+v", releaseVersions, want)
	}

	const methodName = "GetNodeReleaseVersions"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetNodeReleaseVersions(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
