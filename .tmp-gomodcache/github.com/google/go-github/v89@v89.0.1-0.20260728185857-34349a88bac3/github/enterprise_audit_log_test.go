// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestEnterpriseService_GetAuditLog(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/audit-log", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")

		fmt.Fprint(w, `[
		{
        "workflow_id": 123456,
        "head_branch": "master",
        "org": "o",
        "trigger_id": null,
        "repo": "o/blue-crayon-1",
        "created_at": 1615077308538,
        "head_sha": "5acdeadbeef64d1a62388e901e5cdc9358644b37",
        "conclusion": "success",
        "actor": "testactor",
        "completed_at": "2021-03-07T00:35:08.000Z",
        "@timestamp": 1615077308538,
        "name": "Code scanning - action",
        "action": "workflows.completed_workflow_run",
        "started_at": "2021-03-07T00:33:04.000Z",
        "event": "schedule",
        "workflow_run_id": 628312345,
        "_document_id": "beeZYapIUe-wKg5-beadb33"
		}
		]`)
	})
	getOpts := GetAuditLogOptions{
		Include: Ptr("all"),
		Phrase:  Ptr("action:workflows"),
		Order:   Ptr("asc"),
	}
	ctx := t.Context()
	auditEntries, _, err := client.Enterprise.GetAuditLog(ctx, "e", &getOpts)
	if err != nil {
		t.Errorf("Enterprise.GetAuditLog returned error: %v", err)
	}
	timestamp := time.Unix(0, 1615077308538*1e6)
	want := []*AuditEntry{
		{
			Timestamp:  &Timestamp{timestamp},
			DocumentID: Ptr("beeZYapIUe-wKg5-beadb33"),
			Action:     Ptr("workflows.completed_workflow_run"),
			Actor:      Ptr("testactor"),
			CreatedAt:  &Timestamp{timestamp},
			Org:        Ptr("o"),
			AdditionalFields: map[string]any{
				"completed_at":    "2021-03-07T00:35:08.000Z",
				"conclusion":      "success",
				"event":           "schedule",
				"head_branch":     "master",
				"head_sha":        "5acdeadbeef64d1a62388e901e5cdc9358644b37",
				"name":            "Code scanning - action",
				"repo":            "o/blue-crayon-1",
				"started_at":      "2021-03-07T00:33:04.000Z",
				"workflow_id":     float64(123456),
				"workflow_run_id": float64(628312345),
			},
		},
	}

	assertNoDiff(t, want, auditEntries)

	const methodName = "GetAuditLog"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Enterprise.GetAuditLog(ctx, "\n", &getOpts)
		return err
	})

	testNewRequestAndDoFailureCategory(t, methodName, client, AuditLogCategory, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetAuditLog(ctx, "o", &GetAuditLogOptions{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
