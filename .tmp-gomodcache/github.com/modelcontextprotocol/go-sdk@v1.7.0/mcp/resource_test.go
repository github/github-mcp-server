// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFileRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: fix for Windows")
	}

	for _, tt := range []struct {
		uri     string
		want    string
		wantErr string // error must contain this string
	}{
		{uri: "file:///foo", want: "/foo"},
		{uri: "file:///foo/bar", want: "/foo/bar"},
		{uri: "file:///foo/../bar", want: "/bar"},
		{uri: "file:/foo", want: "/foo"},
		{uri: "http:///foo", wantErr: "not a file"},
		{uri: "file://foo", wantErr: "empty path"},
		{uri: ":", wantErr: "missing protocol scheme"},
	} {
		got, err := fileRoot(&Root{URI: tt.uri})
		if err != nil {
			if tt.wantErr == "" {
				t.Errorf("%s: got %v, want success", tt.uri, err)
				continue
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("%s: got %v, does not contain %q", tt.uri, err, tt.wantErr)
				continue
			}
		} else if tt.wantErr != "" {
			t.Errorf("%s: succeeded, but wanted error with %q", tt.uri, tt.wantErr)
		} else if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestComputeURIFilepath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: fix for Windows")
	}
	// TODO(jba): test with Windows \\host paths and C: paths
	dirFilepath := filepath.FromSlash("/files")
	rootFilepaths := []string{
		filepath.FromSlash("/files/public"),
		filepath.FromSlash("/files/shared"),
	}
	for _, tt := range []struct {
		uri     string
		want    string
		wantErr string // error must contain this
	}{
		{"file:///public", "public", ""},
		{"file:///public/file", "public/file", ""},
		{"file:///shared/file", "shared/file", ""},
		{"http:///foo", "", "not a file"},
		{"file://foo", "", "empty"},
		{"file://foo/../bar", "", "localized"},
		{"file:///secret", "", "root"},
		{"file:///secret/file", "", "root"},
		{"file:///private/file", "", "root"},
	} {
		t.Run(tt.uri, func(t *testing.T) {
			tt.want = filepath.FromSlash(tt.want) // handle Windows
			got, gotErr := computeURIFilepath(tt.uri, dirFilepath, rootFilepaths)
			if gotErr != nil {
				if tt.wantErr == "" {
					t.Fatalf("got %v, wanted success", gotErr)
				}
				if !strings.Contains(gotErr.Error(), tt.wantErr) {
					t.Fatalf("got error %v, does not contain %q", gotErr, tt.wantErr)
				}
				return
			}
			if tt.wantErr != "" {
				t.Fatal("succeeded unexpectedly")
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadFileResource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: fix for Windows")
	}
	abs, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	dirFilepath := filepath.Join(abs, "files")
	got, err := readFileResource("file:///info.txt", dirFilepath, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Contents\n"
	if g := string(got); g != want {
		t.Errorf("got %q, want %q", g, want)
	}
}

func TestTemplateMatch(t *testing.T) {
	uri := "file:///path/to/file"
	for _, tt := range []struct {
		template string
		want     bool
	}{
		{"file:///{}/{a}/{b}", false}, // invalid: empty variable expression "{}" is not allowed in RFC 6570
		{"file:///{a}/{b}", false},
		{"file:///{+path}", true},
		{"file:///{a}/{+path}", true},
	} {
		resourceTmpl := serverResourceTemplate{resourceTemplate: &ResourceTemplate{URITemplate: tt.template}}
		if matched := resourceTmpl.Matches(uri); matched != tt.want {
			t.Errorf("%s: got %t, want %t", tt.template, matched, tt.want)
		}
	}
}
