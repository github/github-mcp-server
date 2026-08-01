// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"testing"
)

func TestIssueEvents(t *testing.T) {
	events, _, err := client.Issues.ListRepositoryEvents(t.Context(), "google", "go-github", nil)
	if err != nil {
		t.Fatalf("Issues.ListRepositoryEvents returned error: %v", err)
	}

	if len(events) == 0 {
		t.Error("ListRepositoryEvents returned no events")
	}

	events, _, err = client.Issues.ListIssueEvents(t.Context(), "google", "go-github", 1, nil)
	if err != nil {
		t.Fatalf("Issues.ListIssueEvents returned error: %v", err)
	}

	if len(events) == 0 {
		t.Error("ListIssueEvents returned no events")
	}

	event, _, err := client.Issues.GetEvent(t.Context(), "google", "go-github", *events[0].ID)
	if err != nil {
		t.Fatalf("Issues.GetEvent returned error: %v", err)
	}

	if *event.URL != *events[0].URL {
		t.Fatalf("Issues.GetEvent returned event URL: %v, want %v", *event.URL, *events[0].URL)
	}
}
