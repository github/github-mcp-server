// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPackageVersion_GetBody(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		pv        *PackageVersion
		wantValue string
		wantOk    bool
	}{
		"pv nil": {
			pv:        nil,
			wantValue: "",
			wantOk:    false,
		},
		"body nil": {
			pv: &PackageVersion{
				Body: nil,
			},
			wantValue: "",
			wantOk:    false,
		},
		"invalid body": {
			pv: &PackageVersion{
				Body: json.RawMessage(`{
					"repository": {
						"name": "n"
					},
					"info": {
						"type": "t"
					}
				}`),
			},
			wantValue: "",
			wantOk:    false,
		},
		"valid body": {
			pv: &PackageVersion{
				Body: json.RawMessage(`"body"`),
			},
			wantValue: "body",
			wantOk:    true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resValue, resOk := test.pv.GetBody()

			if resValue != test.wantValue || resOk != test.wantOk {
				t.Errorf("PackageVersion.GetBody() - got: %v, %v; want: %v, %v", resValue, resOk, test.wantValue, test.wantOk)
			}
		})
	}
}

func TestPackageVersion_GetBodyAsPackageVersionBody(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		pv        *PackageVersion
		wantValue *PackageVersionBody
		wantOk    bool
	}{
		"pv nil": {
			pv:        nil,
			wantValue: nil,
			wantOk:    false,
		},
		"body nil": {
			pv: &PackageVersion{
				Body: nil,
			},
			wantValue: nil,
			wantOk:    false,
		},
		"invalid body": {
			pv: &PackageVersion{
				Body: json.RawMessage(`"body"`),
			},
			wantValue: nil,
			wantOk:    false,
		},
		"valid body": {
			pv: &PackageVersion{
				Body: json.RawMessage(`{
					"repository": {
						"name": "n"
					},
					"info": {
						"type": "t"
					}
				}`),
			},
			wantValue: &PackageVersionBody{
				Repo: &Repository{
					Name: Ptr("n"),
				},
				Info: &PackageVersionBodyInfo{
					Type: Ptr("t"),
				},
			},
			wantOk: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resValue, resOk := test.pv.GetBodyAsPackageVersionBody()

			if !cmp.Equal(resValue, test.wantValue) || resOk != test.wantOk {
				t.Errorf("PackageVersion.GetBodyAsPackageVersionBody() - got: %v, %v; want: %v, %v", resValue, resOk, test.wantValue, test.wantOk)
			}
		})
	}
}

func TestPackageVersion_GetMetadata(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		pv        *PackageVersion
		wantValue *PackageMetadata
		wantOk    bool
	}{
		"pv nil": {
			pv:        nil,
			wantValue: nil,
			wantOk:    false,
		},
		"metadata nil": {
			pv: &PackageVersion{
				Metadata: nil,
			},
			wantValue: nil,
			wantOk:    false,
		},
		"invalid metadata": {
			pv: &PackageVersion{
				Metadata: json.RawMessage(`[]`),
			},
			wantValue: nil,
			wantOk:    false,
		},
		"valid metadata": {
			pv: &PackageVersion{
				Metadata: json.RawMessage(`{
					"package_type": "container",
					"container": {
						"tags": ["a"]
					}
				}`),
			},
			wantValue: &PackageMetadata{
				PackageType: Ptr("container"),
				Container: &PackageContainerMetadata{
					Tags: []string{"a"},
				},
			},
			wantOk: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resValue, resOk := test.pv.GetMetadata()

			if !cmp.Equal(resValue, test.wantValue) || resOk != test.wantOk {
				t.Errorf("PackageVersion.GetMetadata() - got: %v, %v; want: %v, %v", resValue, resOk, test.wantValue, test.wantOk)
			}
		})
	}
}

func TestPackageVersion_GetRawMetadata(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		pv   *PackageVersion
		want json.RawMessage
	}{
		"pv nil": {
			pv:   nil,
			want: nil,
		},
		"metadata nil": {
			pv: &PackageVersion{
				Metadata: nil,
			},
			want: json.RawMessage{},
		},
		"valid metadata": {
			pv: &PackageVersion{
				Metadata: json.RawMessage(`"a"`),
			},
			want: json.RawMessage(`"a"`),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			res := test.pv.GetRawMetadata()

			if string(res) != string(test.want) {
				t.Errorf("PackageVersion.GetRawMetadata() - got: %v; want: %v", res, test.want)
			}
		})
	}
}
