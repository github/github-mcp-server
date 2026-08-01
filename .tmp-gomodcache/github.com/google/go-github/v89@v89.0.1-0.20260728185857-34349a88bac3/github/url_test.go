// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"strings"
	"testing"
)

func Test_parseURL(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:    "empty_string_returns_error",
			input:   "",
			wantErr: "url cannot be empty",
		},
		{
			name:    "invalid_url_returns_error",
			input:   "://invalid-url\n",
			wantErr: "invalid url",
		},
		{
			name:  "valid_url",
			input: "https://api.github.com/",
			want:  "https://api.github.com/",
		},
		{
			name:  "valid_url_without_trailing_slash_adds_slash",
			input: "https://api.github.com",
			want:  "https://api.github.com/",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseURL(tt.input)
			if err != nil {
				if tt.wantErr == "" {
					t.Fatalf("unexpected error: %v", err)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %v, got %v", tt.wantErr, err.Error())
				}

				return
			}

			if tt.wantErr != "" {
				t.Fatalf("expected error to contain %v, got nil", tt.wantErr)
				return
			}

			if got == nil {
				t.Fatal("expected non-nil URL, got nil")
				return
			}

			if got.String() != tt.want {
				t.Fatalf("expected URL %v, got %v", tt.want, got.String())
			}
		})
	}
}
