// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"testing"
)

// TestOrganizationAuditLog test that the client can read an org's audit log.
// Note: Org must be part of an enterprise.
// Test requires auth - set env var GITHUB_AUTH_TOKEN.
func TestOrganizationAuditLog(t *testing.T) {
	skipIfMissingAuth(t)

	org := "example_org"
	entries, _, err := client.Organizations.GetAuditLog(t.Context(), org, nil)
	if err != nil {
		t.Fatalf("Organizations.GetAuditLog returned error: %v", err)
	}

	if len(entries) == 0 {
		t.Error("No AuditLog events returned for org")
	}

	for _, e := range entries {
		t.Log(e.GetAction(), e.GetActor(), e.GetTimestamp(), e.GetUser())
	}
}
