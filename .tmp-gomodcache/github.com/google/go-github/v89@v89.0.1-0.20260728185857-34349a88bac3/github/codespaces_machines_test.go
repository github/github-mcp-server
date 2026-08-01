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

func TestCodespacesService_ListRepositoryMachineTypes(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/owner/repo/codespaces/machines", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"ref":       "main",
			"location":  "WestUs2",
			"client_ip": "1.2.3.4",
		})
		fmt.Fprint(w, `{
			"total_count": 1,
			"machines": [
				{
					"name": "standardLinux",
					"display_name": "4 cores, 8 GB RAM, 64 GB storage",
					"operating_system": "linux",
					"storage_in_bytes": 68719476736,
					"memory_in_bytes": 17179869184,
					"cpus": 4,
					"prebuild_availability": "ready"
				}
			]
		}`)
	})

	ctx := t.Context()
	opts := &ListRepoMachineTypesOptions{
		Ref:      Ptr("main"),
		Location: Ptr("WestUs2"),
		ClientIP: Ptr("1.2.3.4"),
	}

	got, _, err := client.Codespaces.ListRepositoryMachineTypes(
		ctx,
		"owner",
		"repo",
		opts,
	)
	if err != nil {
		t.Fatalf("Codespaces.ListRepositoryMachineTypes returned error: %v", err)
	}

	want := &CodespacesMachines{
		TotalCount: 1,
		Machines: []*CodespacesMachine{
			{
				Name:                 Ptr("standardLinux"),
				DisplayName:          Ptr("4 cores, 8 GB RAM, 64 GB storage"),
				OperatingSystem:      Ptr("linux"),
				StorageInBytes:       Ptr(int64(68719476736)),
				MemoryInBytes:        Ptr(int64(17179869184)),
				CPUs:                 Ptr(4),
				PrebuildAvailability: Ptr("ready"),
			},
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Codespaces.ListRepositoryMachineTypes returned %+v, want %+v", got, want)
	}

	const methodName = "ListRepositoryMachineTypes"
	testBadOptions(t, methodName, func() error {
		_, _, err := client.Codespaces.ListRepositoryMachineTypes(ctx, "\n", "/n", opts)
		return err
	})
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.ListRepositoryMachineTypes(ctx, "/n", "/n", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_ListCodespaceMachineTypes(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/codespaces/codespace_1/machines", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")

		fmt.Fprint(w, `{
			"total_count": 1,
			"machines": [
				{
					"name": "standardLinux",
					"display_name": "4 cores, 8 GB RAM, 64 GB storage",
					"operating_system": "linux",
					"storage_in_bytes": 68719476736,
					"memory_in_bytes": 17179869184,
					"cpus": 4,
					"prebuild_availability": "ready"
				}
			]
		}`)
	})

	ctx := t.Context()
	got, _, err := client.Codespaces.ListCodespaceMachineTypes(ctx, "codespace_1")
	if err != nil {
		t.Fatalf("Codespaces.ListCodespaceMachineTypes returned error: %v", err)
	}

	want := &CodespacesMachines{
		TotalCount: 1,
		Machines: []*CodespacesMachine{
			{
				Name:                 Ptr("standardLinux"),
				DisplayName:          Ptr("4 cores, 8 GB RAM, 64 GB storage"),
				OperatingSystem:      Ptr("linux"),
				StorageInBytes:       Ptr(int64(68719476736)),
				MemoryInBytes:        Ptr(int64(17179869184)),
				CPUs:                 Ptr(4),
				PrebuildAvailability: Ptr("ready"),
			},
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Codespaces.ListCodespaceMachineTypes returned %+v, want %+v", got, want)
	}

	const methodName = "ListCodespaceMachineTypes"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.ListCodespaceMachineTypes(ctx, "/n")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
