// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"testing"
	"time"
)

func TestEmojis(t *testing.T) {
	emoji, _, err := client.Emojis.List(t.Context())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if len(emoji) == 0 {
		t.Error("List returned no emojis")
	}

	if _, ok := emoji["+1"]; !ok {
		t.Error("List missing '+1' emoji")
	}
}

func TestAPIMeta(t *testing.T) {
	meta, _, err := client.Meta.Get(t.Context())
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if len(meta.Hooks) == 0 {
		t.Error("Get returned no hook addresses")
	}

	if len(meta.Git) == 0 {
		t.Error("Get returned no git addresses")
	}

	if *meta.VerifiablePasswordAuthentication {
		t.Error("APIMeta VerifiablePasswordAuthentication is true")
	}
}

func TestRateLimits(t *testing.T) {
	limits, _, err := client.RateLimit.Get(t.Context())
	if err != nil {
		t.Fatalf("RateLimits returned error: %v", err)
	}

	// do some sanity checks
	if limits.Core.Limit == 0 {
		t.Error("RateLimits returned 0 core limit")
	}

	if limits.Core.Limit < limits.Core.Remaining {
		t.Error("Core.Limits is less than Core.Remaining.")
	}

	if limits.Core.Reset.Time.Before(time.Now().Add(-1 * time.Minute)) {
		t.Error("Core.Reset is more than 1 minute in the past; that doesn't seem right.")
	}
}
