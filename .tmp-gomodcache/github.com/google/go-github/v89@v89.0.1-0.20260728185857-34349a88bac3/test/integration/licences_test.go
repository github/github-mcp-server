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

func TestLicenses_ListIter(t *testing.T) {
	opts := &github.ListLicensesOptions{
		Featured: github.Ptr(true),
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 1,
		},
	}

	var featuredLicensesCount int
	for _, err := range client.Licenses.ListIter(t.Context(), opts) {
		if err != nil {
			t.Fatalf("Licenses.ListIter returned error during iteration: %v", err)
		}
		featuredLicensesCount++
	}

	if featuredLicensesCount < 2 {
		t.Errorf("Licenses.ListIter returned fewer than 2 featured licenses: %v", featuredLicensesCount)
	}
}
