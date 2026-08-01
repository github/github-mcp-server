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

func TestEnterpriseService_GetMaintenanceStatus(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/manage/v1/maintenance", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"uuid":          "1234-1234",
			"cluster_roles": "primary",
		})
		fmt.Fprint(w, `[{
			"hostname": "primary",
			"uuid": "1b6cf518-f97c-11ed-8544-061d81f7eedb",
			"status": "scheduled",
			"scheduled_time": `+referenceTimeStr+`,
			"connection_services": [
			{
				"name": "git operations",
				"number": 15
			}
			],
			"can_unset_maintenance": true,
			"ip_exception_list": [
			"1.1.1.1"
			],
			"maintenance_mode_message": "Scheduled maintenance for upgrading."
		}]`)
	})

	opt := &NodeQueryOptions{
		UUID: Ptr("1234-1234"), ClusterRoles: Ptr("primary"),
	}
	ctx := t.Context()
	maintenanceStatus, _, err := client.Enterprise.GetMaintenanceStatus(ctx, opt)
	if err != nil {
		t.Errorf("Enterprise.GetMaintenanceStatus returned error: %v", err)
	}

	want := []*MaintenanceStatus{{
		Hostname:      Ptr("primary"),
		UUID:          Ptr("1b6cf518-f97c-11ed-8544-061d81f7eedb"),
		Status:        Ptr("scheduled"),
		ScheduledTime: &referenceTimestamp,
		ConnectionServices: []*ConnectionServiceItem{{
			Name:   Ptr("git operations"),
			Number: Ptr(15),
		}},
		CanUnsetMaintenance:    Ptr(true),
		IPExceptionList:        []string{"1.1.1.1"},
		MaintenanceModeMessage: Ptr("Scheduled maintenance for upgrading."),
	}}
	if !cmp.Equal(maintenanceStatus, want) {
		t.Errorf("Enterprise.GetMaintenanceStatus returned %+v, want %+v", maintenanceStatus, want)
	}

	const methodName = "GetMaintenanceStatus"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetMaintenanceStatus(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateMaintenance(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &MaintenanceOptions{
		Enabled: true,
		UUID:    Ptr("1234-1234"),
		When:    Ptr("now"),
		IPExceptionList: []string{
			"1.1.1.1",
		},
		MaintenanceModeMessage: Ptr("Scheduled maintenance for upgrading."),
	}

	mux.HandleFunc("/manage/v1/maintenance", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `[ { "hostname": "primary", "uuid": "1b6cf518-f97c-11ed-8544-061d81f7eedb", "message": "Scheduled maintenance for upgrading." } ]`)
	})

	ctx := t.Context()
	maintenanceStatus, _, err := client.Enterprise.CreateMaintenance(ctx, true, input)
	if err != nil {
		t.Errorf("Enterprise.CreateMaintenance returned error: %v", err)
	}

	want := []*MaintenanceOperationStatus{{Hostname: Ptr("primary"), UUID: Ptr("1b6cf518-f97c-11ed-8544-061d81f7eedb"), Message: Ptr("Scheduled maintenance for upgrading.")}}
	if diff := cmp.Diff(want, maintenanceStatus); diff != "" {
		t.Errorf("diff mismatch (-want +got):\n%v", diff)
	}

	const methodName = "CreateMaintenance"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateMaintenance(ctx, true, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
