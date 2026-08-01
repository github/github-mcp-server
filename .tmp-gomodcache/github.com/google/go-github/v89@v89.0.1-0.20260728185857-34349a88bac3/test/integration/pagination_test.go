// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"testing"

	"github.com/google/go-github/v89/github"
)

func TestSecurityAdvisories_ListGlobalSecurityAdvisories(t *testing.T) {
	opt := &github.ListGlobalSecurityAdvisoriesOptions{
		ListCursorOptions: github.ListCursorOptions{
			PerPage: 2,
		},
	}
	advisories, resp, err := client.SecurityAdvisories.ListGlobalSecurityAdvisories(t.Context(), opt)
	if err != nil {
		t.Fatalf("ListGlobalSecurityAdvisories returned error: %v", err)
	}

	if got, want := len(advisories), 2; got != want {
		t.Errorf("ListGlobalSecurityAdvisories returned %v advisories, want %v", got, want)
	}

	if resp.After == "" {
		t.Error("ListGlobalSecurityAdvisories returned an empty 'after' cursor")
	}

	if resp.Cursor != "" {
		t.Error("ListGlobalSecurityAdvisories returned a non-empty 'cursor' value")
	}
}
