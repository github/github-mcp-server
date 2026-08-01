// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRepositoriesService_ListDeployments(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/deployments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"environment": "test"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &DeploymentsListOptions{Environment: "test"}
	ctx := t.Context()
	deployments, _, err := client.Repositories.ListDeployments(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListDeployments returned error: %v", err)
	}

	want := []*Deployment{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(deployments, want) {
		t.Errorf("Repositories.ListDeployments returned %+v, want %+v", deployments, want)
	}

	const methodName = "ListDeployments"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListDeployments(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListDeployments(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetDeployment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/deployments/3", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":3}`)
	})

	ctx := t.Context()
	deployment, _, err := client.Repositories.GetDeployment(ctx, "o", "r", 3)
	if err != nil {
		t.Errorf("Repositories.GetDeployment returned error: %v", err)
	}

	want := &Deployment{ID: Ptr(int64(3))}

	if !cmp.Equal(deployment, want) {
		t.Errorf("Repositories.GetDeployment returned %+v, want %+v", deployment, want)
	}

	const methodName = "GetDeployment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetDeployment(ctx, "\n", "\n", 3)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetDeployment(ctx, "o", "r", 3)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateDeployment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := DeploymentRequest{Ref: "1111", Task: Ptr("deploy"), TransientEnvironment: Ptr(true)}

	mux.HandleFunc("/repos/o/r/deployments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		wantAcceptHeaders := []string{mediaTypeDeploymentStatusPreview, mediaTypeExpandDeploymentStatusPreview}
		testHeader(t, r, "Accept", strings.Join(wantAcceptHeaders, ", "))
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"ref": "1111", "task": "deploy"}`)
	})

	ctx := t.Context()
	deployment, _, err := client.Repositories.CreateDeployment(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Repositories.CreateDeployment returned error: %v", err)
	}

	want := &Deployment{Ref: Ptr("1111"), Task: Ptr("deploy")}
	if !cmp.Equal(deployment, want) {
		t.Errorf("Repositories.CreateDeployment returned %+v, want %+v", deployment, want)
	}

	const methodName = "CreateDeployment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.CreateDeployment(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateDeployment(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DeleteDeployment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/deployments/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	resp, err := client.Repositories.DeleteDeployment(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.DeleteDeployment returned error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Error("Repositories.DeleteDeployment should return a 204 status")
	}

	resp, err = client.Repositories.DeleteDeployment(ctx, "o", "r", 2)
	if err == nil {
		t.Error("Repositories.DeleteDeployment should return an error")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Error("Repositories.DeleteDeployment should return a 404 status")
	}

	const methodName = "DeleteDeployment"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DeleteDeployment(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeleteDeployment(ctx, "o", "r", 1)
	})
}

func TestRepositoriesService_ListDeploymentStatuses(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	wantAcceptHeaders := []string{mediaTypeDeploymentStatusPreview, mediaTypeExpandDeploymentStatusPreview}
	mux.HandleFunc("/repos/o/r/deployments/1/statuses", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", strings.Join(wantAcceptHeaders, ", "))
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	statuses, _, err := client.Repositories.ListDeploymentStatuses(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Repositories.ListDeploymentStatuses returned error: %v", err)
	}

	want := []*DeploymentStatus{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if !cmp.Equal(statuses, want) {
		t.Errorf("Repositories.ListDeploymentStatuses returned %+v, want %+v", statuses, want)
	}

	const methodName = "ListDeploymentStatuses"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListDeploymentStatuses(ctx, "\n", "\n", 1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListDeploymentStatuses(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetDeploymentStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	wantAcceptHeaders := []string{mediaTypeDeploymentStatusPreview, mediaTypeExpandDeploymentStatusPreview}
	mux.HandleFunc("/repos/o/r/deployments/3/statuses/4", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", strings.Join(wantAcceptHeaders, ", "))
		fmt.Fprint(w, `{"id":4}`)
	})

	ctx := t.Context()
	deploymentStatus, _, err := client.Repositories.GetDeploymentStatus(ctx, "o", "r", 3, 4)
	if err != nil {
		t.Errorf("Repositories.GetDeploymentStatus returned error: %v", err)
	}

	want := &DeploymentStatus{ID: Ptr(int64(4))}
	if !cmp.Equal(deploymentStatus, want) {
		t.Errorf("Repositories.GetDeploymentStatus returned %+v, want %+v", deploymentStatus, want)
	}

	const methodName = "GetDeploymentStatus"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetDeploymentStatus(ctx, "\n", "\n", 3, 4)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetDeploymentStatus(ctx, "o", "r", 3, 4)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateDeploymentStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := DeploymentStatusRequest{State: "inactive", Description: Ptr("deploy"), AutoInactive: Ptr(false)}

	mux.HandleFunc("/repos/o/r/deployments/1/statuses", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		wantAcceptHeaders := []string{mediaTypeDeploymentStatusPreview, mediaTypeExpandDeploymentStatusPreview}
		testHeader(t, r, "Accept", strings.Join(wantAcceptHeaders, ", "))
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"state": "inactive", "description": "deploy"}`)
	})

	ctx := t.Context()
	deploymentStatus, _, err := client.Repositories.CreateDeploymentStatus(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Repositories.CreateDeploymentStatus returned error: %v", err)
	}

	want := &DeploymentStatus{State: Ptr("inactive"), Description: Ptr("deploy")}
	if !cmp.Equal(deploymentStatus, want) {
		t.Errorf("Repositories.CreateDeploymentStatus returned %+v, want %+v", deploymentStatus, want)
	}

	const methodName = "CreateDeploymentStatus"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.CreateDeploymentStatus(ctx, "\n", "\n", 1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateDeploymentStatus(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
