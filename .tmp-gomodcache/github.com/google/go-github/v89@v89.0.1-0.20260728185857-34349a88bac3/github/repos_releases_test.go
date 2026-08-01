// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRepositoriesService_ListReleases(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	releases, _, err := client.Repositories.ListReleases(ctx, "o", "r", opt)
	if err != nil {
		t.Errorf("Repositories.ListReleases returned error: %v", err)
	}
	want := []*RepositoryRelease{{ID: 1}}
	if !cmp.Equal(releases, want) {
		t.Errorf("Repositories.ListReleases returned %+v, want %+v", releases, want)
	}

	const methodName = "ListReleases"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListReleases(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListReleases(ctx, "o", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GenerateReleaseNotes(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	body := GenerateNotesRequest{
		TagName: "v1.0.0",
	}

	mux.HandleFunc("/repos/o/r/releases/generate-notes", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, body)
		fmt.Fprint(w, `{"name":"v1.0.0","body":"**Full Changelog**: https://github.com/o/r/compare/v0.9.0...v1.0.0"}`)
	})

	ctx := t.Context()
	releases, _, err := client.Repositories.GenerateReleaseNotes(ctx, "o", "r", body)
	if err != nil {
		t.Errorf("Repositories.GenerateReleaseNotes returned error: %v", err)
	}
	want := &RepositoryReleaseNotes{
		Name: "v1.0.0",
		Body: "**Full Changelog**: https://github.com/o/r/compare/v0.9.0...v1.0.0",
	}
	if !cmp.Equal(releases, want) {
		t.Errorf("Repositories.GenerateReleaseNotes returned %+v, want %+v", releases, want)
	}

	const methodName = "GenerateReleaseNotes"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GenerateReleaseNotes(ctx, "\n", "\n", body)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GenerateReleaseNotes(ctx, "o", "r", body)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetRelease(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1,"author":{"login":"l"}}`)
	})

	ctx := t.Context()
	release, resp, err := client.Repositories.GetRelease(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.GetRelease returned error: %v\n%v", err, resp.Body)
	}

	want := &RepositoryRelease{ID: 1, Author: &User{Login: Ptr("l")}}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.GetRelease returned %+v, want %+v", release, want)
	}

	const methodName = "GetRelease"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetRelease(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetRelease(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetLatestRelease(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":3}`)
	})

	ctx := t.Context()
	release, resp, err := client.Repositories.GetLatestRelease(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.GetLatestRelease returned error: %v\n%v", err, resp.Body)
	}

	want := &RepositoryRelease{ID: 3}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.GetLatestRelease returned %+v, want %+v", release, want)
	}

	const methodName = "GetLatestRelease"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetLatestRelease(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetLatestRelease(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetReleaseByTag(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/tags/foo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":13}`)
	})

	ctx := t.Context()
	release, resp, err := client.Repositories.GetReleaseByTag(ctx, "o", "r", "foo")
	if err != nil {
		t.Errorf("Repositories.GetReleaseByTag returned error: %v\n%v", err, resp.Body)
	}

	want := &RepositoryRelease{ID: 13}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.GetReleaseByTag returned %+v, want %+v", release, want)
	}

	const methodName = "GetReleaseByTag"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetReleaseByTag(ctx, "\n", "\n", "foo")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetReleaseByTag(ctx, "o", "r", "foo")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_CreateRelease(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := CreateReleaseRequest{
		TagName:                "v1.0",
		Name:                   Ptr("v1.0"),
		DiscussionCategoryName: Ptr("General"),
		GenerateReleaseNotes:   Ptr(true),
	}

	mux.HandleFunc("/repos/o/r/releases", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	release, _, err := client.Repositories.CreateRelease(ctx, "o", "r", input)
	if err != nil {
		t.Errorf("Repositories.CreateRelease returned error: %v", err)
	}

	want := &RepositoryRelease{ID: 1}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.CreateRelease returned %+v, want %+v", release, want)
	}

	const methodName = "CreateRelease"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.CreateRelease(ctx, "\n", "\n", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.CreateRelease(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UpdateRelease(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := UpdateReleaseRequest{
		Name:                   Ptr("n"),
		DiscussionCategoryName: Ptr("General"),
	}

	mux.HandleFunc("/repos/o/r/releases/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	release, _, err := client.Repositories.UpdateRelease(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Repositories.UpdateRelease returned error: %v", err)
	}
	want := &RepositoryRelease{ID: 1}
	if !cmp.Equal(release, want) {
		t.Errorf("Repositories.UpdateRelease returned = %+v, want %+v", release, want)
	}

	const methodName = "UpdateRelease"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UpdateRelease(ctx, "\n", "\n", 1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdateRelease(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DeleteRelease(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Repositories.DeleteRelease(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.DeleteRelease returned error: %v", err)
	}

	const methodName = "DeleteRelease"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DeleteRelease(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeleteRelease(ctx, "o", "r", 1)
	})
}

func TestRepositoriesService_ListReleaseAssets(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/1/assets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"page": "2"})
		fmt.Fprint(w, `[{"id":1}]`)
	})

	opt := &ListOptions{Page: 2}
	ctx := t.Context()
	assets, _, err := client.Repositories.ListReleaseAssets(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Repositories.ListReleaseAssets returned error: %v", err)
	}
	want := []*ReleaseAsset{{ID: Ptr(int64(1))}}
	if !cmp.Equal(assets, want) {
		t.Errorf("Repositories.ListReleaseAssets returned %+v, want %+v", assets, want)
	}

	const methodName = "ListReleaseAssets"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListReleaseAssets(ctx, "\n", "\n", 1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListReleaseAssets(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetReleaseAsset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	asset, _, err := client.Repositories.GetReleaseAsset(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.GetReleaseAsset returned error: %v", err)
	}
	want := &ReleaseAsset{ID: Ptr(int64(1))}
	if !cmp.Equal(asset, want) {
		t.Errorf("Repositories.GetReleaseAsset returned %+v, want %+v", asset, want)
	}

	const methodName = "GetReleaseAsset"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetReleaseAsset(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetReleaseAsset(ctx, "o", "r", 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DownloadReleaseAsset_Stream(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=hello-world.txt")
		fmt.Fprint(w, "Hello World")
	})

	ctx := t.Context()
	reader, _, err := client.Repositories.DownloadReleaseAsset(ctx, "o", "r", 1, nil)
	if err != nil {
		t.Errorf("Repositories.DownloadReleaseAsset returned error: %v", err)
	}
	want := []byte("Hello World")
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("Repositories.DownloadReleaseAsset returned bad reader: %v", err)
	}
	if !bytes.Equal(want, content) {
		t.Errorf("Repositories.DownloadReleaseAsset returned %+v, want %+v", content, want)
	}

	const methodName = "DownloadReleaseAsset"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.DownloadReleaseAsset(ctx, "\n", "\n", -1, nil)
		return err
	})
}

func TestRepositoriesService_DownloadReleaseAsset_Redirect(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		http.Redirect(w, r, "/yo", http.StatusFound)
	})

	ctx := t.Context()
	_, got, err := client.Repositories.DownloadReleaseAsset(ctx, "o", "r", 1, nil)
	if err != nil {
		t.Errorf("Repositories.DownloadReleaseAsset returned error: %v", err)
	}
	want := "/yo"
	if !strings.HasSuffix(got, want) {
		t.Errorf("Repositories.DownloadReleaseAsset returned %+v, want %+v", got, want)
	}
}

func TestRepositoriesService_DownloadReleaseAsset_FollowRedirect(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		// /yo, below will be served as baseURLPath/yo
		http.Redirect(w, r, baseURLPath+"/yo", http.StatusFound)
	})
	mux.HandleFunc("/yo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=hello-world.txt")
		fmt.Fprint(w, "Hello World")
	})

	ctx := t.Context()
	reader, _, err := client.Repositories.DownloadReleaseAsset(ctx, "o", "r", 1, http.DefaultClient)
	if err != nil {
		t.Errorf("Repositories.DownloadReleaseAsset returned error: %v", err)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("Reading Repositories.DownloadReleaseAsset returned error: %v", err)
	}
	reader.Close()
	want := []byte("Hello World")
	if !bytes.Equal(want, content) {
		t.Errorf("Repositories.DownloadReleaseAsset returned %+v, want %+v", content, want)
	}
}

func TestRepositoriesService_DownloadReleaseAsset_FollowMultipleRedirects(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		// /yo, below will be served as baseURLPath/yo
		http.Redirect(w, r, baseURLPath+"/yo", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/yo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html;charset=utf-8")
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		// /yo2, below will be served as baseURLPath/yo2
		http.Redirect(w, r, baseURLPath+"/yo2", http.StatusFound)
	})
	mux.HandleFunc("/yo2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=hello-world.txt")
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		fmt.Fprint(w, "Hello World")
	})

	ctx := t.Context()
	reader, _, err := client.Repositories.DownloadReleaseAsset(ctx, "o", "r", 1, http.DefaultClient)
	if err != nil {
		t.Errorf("Repositories.DownloadReleaseAsset returned error: %v", err)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("Reading Repositories.DownloadReleaseAsset returned error: %v", err)
	}
	reader.Close()
	want := []byte("Hello World")
	if !bytes.Equal(want, content) {
		t.Errorf("Repositories.DownloadReleaseAsset returned %+v, want %+v", content, want)
	}
}

func TestRepositoriesService_DownloadReleaseAsset_FollowRedirectToError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		// /yo, below will be served as baseURLPath/yo
		http.Redirect(w, r, baseURLPath+"/yo", http.StatusFound)
	})
	mux.HandleFunc("/yo", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := t.Context()
	resp, loc, err := client.Repositories.DownloadReleaseAsset(ctx, "o", "r", 1, http.DefaultClient)
	if err == nil {
		t.Error("Repositories.DownloadReleaseAsset did not return an error")
	}
	if resp != nil {
		resp.Close()
		t.Error("Repositories.DownloadReleaseAsset returned stream, want nil")
	}
	if loc != "" {
		t.Errorf(`Repositories.DownloadReleaseAsset returned "%v", want empty ""`, loc)
	}
}

func TestRepositoriesService_DownloadReleaseAsset_APIError(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found","documentation_url":"https://developer.github.com/v3"}`)
	})

	ctx := t.Context()
	resp, loc, err := client.Repositories.DownloadReleaseAsset(ctx, "o", "r", 1, nil)
	if err == nil {
		t.Error("Repositories.DownloadReleaseAsset did not return an error")
	}

	if resp != nil {
		resp.Close()
		t.Error("Repositories.DownloadReleaseAsset returned stream, want nil")
	}

	if loc != "" {
		t.Errorf(`Repositories.DownloadReleaseAsset returned "%v", want empty ""`, loc)
	}
}

func TestRepositoriesService_UpdateReleaseAsset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := UpdateReleaseAssetRequest{Name: Ptr("n")}

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	asset, _, err := client.Repositories.UpdateReleaseAsset(ctx, "o", "r", 1, input)
	if err != nil {
		t.Errorf("Repositories.UpdateReleaseAsset returned error: %v", err)
	}
	want := &ReleaseAsset{ID: Ptr(int64(1))}
	if !cmp.Equal(asset, want) {
		t.Errorf("Repositories.UpdateReleaseAsset returned = %+v, want %+v", asset, want)
	}

	const methodName = "UpdateReleaseAsset"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UpdateReleaseAsset(ctx, "\n", "\n", 1, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UpdateReleaseAsset(ctx, "o", "r", 1, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_DeleteReleaseAsset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/assets/1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Repositories.DeleteReleaseAsset(ctx, "o", "r", 1)
	if err != nil {
		t.Errorf("Repositories.DeleteReleaseAsset returned error: %v", err)
	}

	const methodName = "DeleteReleaseAsset"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Repositories.DeleteReleaseAsset(ctx, "\n", "\n", 1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Repositories.DeleteReleaseAsset(ctx, "o", "r", 1)
	})
}

func TestRepositoriesService_UploadReleaseAsset(t *testing.T) {
	t.Parallel()
	var (
		defaultUploadOptions     = &UploadOptions{Name: "n"}
		defaultExpectedFormValue = values{"name": "n"}
		mediaTypeTextPlain       = "text/plain; charset=utf-8"
	)
	uploadTests := []struct {
		uploadOpts         *UploadOptions
		fileName           string
		expectedFormValues values
		expectedMediaType  string
	}{
		// No file extension and no explicit media type.
		{
			defaultUploadOptions,
			"upload",
			defaultExpectedFormValue,
			defaultMediaType,
		},
		// File extension and no explicit media type.
		{
			defaultUploadOptions,
			"upload.txt",
			defaultExpectedFormValue,
			mediaTypeTextPlain,
		},
		// No file extension and explicit media type.
		{
			&UploadOptions{Name: "n", MediaType: "image/png"},
			"upload",
			defaultExpectedFormValue,
			"image/png",
		},
		// File extension and explicit media type.
		{
			&UploadOptions{Name: "n", MediaType: "image/png"},
			"upload.png",
			defaultExpectedFormValue,
			"image/png",
		},
		// Label provided.
		{
			&UploadOptions{Name: "n", Label: "l"},
			"upload.txt",
			values{"name": "n", "label": "l"},
			mediaTypeTextPlain,
		},
		// No label provided.
		{
			defaultUploadOptions,
			"upload.txt",
			defaultExpectedFormValue,
			mediaTypeTextPlain,
		},
	}

	client, mux, _ := setup(t)

	for key, test := range uploadTests {
		releaseEndpoint := fmt.Sprintf("/repos/o/r/releases/%v/assets", key)
		mux.HandleFunc(releaseEndpoint, func(w http.ResponseWriter, r *http.Request) {
			testMethod(t, r, "POST")
			testHeader(t, r, "Content-Type", test.expectedMediaType)
			testHeader(t, r, "Content-Length", "12")
			testFormValues(t, r, test.expectedFormValues)
			testPlainBody(t, r, "Upload me !\n")

			fmt.Fprint(w, `{"id":1}`)
		})

		file := openTestFile(t, test.fileName, "Upload me !\n")

		ctx := t.Context()
		asset, _, err := client.Repositories.UploadReleaseAsset(ctx, "o", "r", int64(key), test.uploadOpts, file)
		if err != nil {
			t.Errorf("Repositories.UploadReleaseAsset returned error: %v", err)
		}
		want := &ReleaseAsset{ID: Ptr(int64(1))}
		if !cmp.Equal(asset, want) {
			t.Errorf("Repositories.UploadReleaseAsset returned %+v, want %+v", asset, want)
		}

		const methodName = "UploadReleaseAsset"
		testBadOptions(t, methodName, func() (err error) {
			_, _, err = client.Repositories.UploadReleaseAsset(ctx, "o", "r", int64(key), test.uploadOpts, nil)
			return err
		})
		testBadOptions(t, methodName, func() (err error) {
			_, _, err = client.Repositories.UploadReleaseAsset(ctx, "\n", "\n", int64(key), test.uploadOpts, file)
			return err
		})
	}
	testNewRequestAndDoFailure(t, "UploadReleaseAsset", client, func() (*Response, error) {
		got, resp, err := client.Repositories.UploadReleaseAsset(t.Context(), "o", "r", 1, defaultUploadOptions, openTestFile(t, "upload.txt", "Upload me !\n"))
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure UploadReleaseAsset = %#v, want nil", got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UploadReleaseAssetFromRelease(t *testing.T) {
	t.Parallel()

	var (
		defaultUploadOptions     = &UploadOptions{Name: "n.txt"}
		defaultExpectedFormValue = values{"name": "n.txt"}
		mediaTypeTextPlain       = "text/plain; charset=utf-8"
	)

	client, mux, _ := setup(t)

	// Use the same endpoint path used in other release asset tests.
	mux.HandleFunc("/repos/o/r/releases/1/assets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Content-Type", mediaTypeTextPlain)
		testHeader(t, r, "Content-Length", "12")
		testFormValues(t, r, defaultExpectedFormValue)
		testPlainBody(t, r, "Upload me !\n")

		fmt.Fprint(w, `{"id":1}`)
	})

	body := []byte("Upload me !\n")
	reader := bytes.NewReader(body)
	size := int64(len(body))

	// Provide a templated upload URL like GitHub returns.
	release := &RepositoryRelease{
		UploadURL: "/repos/o/r/releases/1/assets{?name,label}",
	}

	ctx := t.Context()
	asset, _, err := client.Repositories.UploadReleaseAssetFromRelease(ctx, release, defaultUploadOptions, reader, size)
	if err != nil {
		t.Fatalf("Repositories.UploadReleaseAssetFromRelease returned error: %v", err)
	}
	want := &ReleaseAsset{ID: Ptr(int64(1))}
	if !cmp.Equal(asset, want) {
		t.Fatalf("Repositories.UploadReleaseAssetFromRelease returned %+v, want %+v", asset, want)
	}
}

func TestRepositoriesService_UploadReleaseAssetFromRelease_AbsoluteTemplate(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/releases/1/assets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testFormValues(t, r, values{"name": "abs.txt"})
		fmt.Fprint(w, `{"id":1}`)
	})

	body := []byte("Upload me !\n")
	reader := bytes.NewReader(body)
	size := int64(len(body))

	// Build an absolute URL using the test client's BaseURL.
	absoluteUploadURL := client.baseURL.String() + "repos/o/r/releases/1/assets{?name,label}"
	release := &RepositoryRelease{UploadURL: absoluteUploadURL}

	opts := &UploadOptions{Name: "abs.txt"}
	ctx := t.Context()
	asset, _, err := client.Repositories.UploadReleaseAssetFromRelease(ctx, release, opts, reader, size)
	if err != nil {
		t.Fatalf("UploadReleaseAssetFromRelease returned error: %v", err)
	}
	want := &ReleaseAsset{ID: Ptr(int64(1))}
	if !cmp.Equal(asset, want) {
		t.Fatalf("UploadReleaseAssetFromRelease returned %+v, want %+v", asset, want)
	}
}

func TestRepositoriesService_UploadReleaseAssetFromRelease_NilRelease(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	body := []byte("Upload me !\n")
	reader := bytes.NewReader(body)
	size := int64(len(body))

	ctx := t.Context()
	_, _, err := client.Repositories.UploadReleaseAssetFromRelease(ctx, nil, &UploadOptions{Name: "n.txt"}, reader, size)
	if err == nil {
		t.Fatal("expected error for nil release, got nil")
	}

	const methodName = "UploadReleaseAssetFromRelease"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UploadReleaseAssetFromRelease(ctx, nil, &UploadOptions{Name: "n.txt"}, reader, size)
		return err
	})
}

func TestRepositoriesService_UploadReleaseAssetFromRelease_NilReader(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	release := &RepositoryRelease{UploadURL: "/repos/o/r/releases/1/assets{?name,label}"}

	ctx := t.Context()
	_, _, err := client.Repositories.UploadReleaseAssetFromRelease(ctx, release, &UploadOptions{Name: "n.txt"}, nil, 12)
	if err == nil {
		t.Fatal("expected error when reader is nil")
	}

	const methodName = "UploadReleaseAssetFromRelease"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.UploadReleaseAssetFromRelease(ctx, release, &UploadOptions{Name: "n.txt"}, nil, 12)
		return err
	})
}

func TestRepositoriesService_UploadReleaseAssetFromRelease_NegativeSize(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	release := &RepositoryRelease{UploadURL: "/repos/o/r/releases/1/assets{?name,label}"}

	body := []byte("Upload me !\n")
	reader := bytes.NewReader(body)

	ctx := t.Context()
	_, _, err := client.Repositories.UploadReleaseAssetFromRelease(ctx, release, &UploadOptions{Name: "n..txt"}, reader, -1)
	if err == nil {
		t.Fatal("expected error when size is negative")
	}
}

func TestRepositoriesService_UploadReleaseAssetFromRelease_NoOpts(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// No opts: we just assert that the handler is hit and body is as expected.
	mux.HandleFunc("/repos/o/r/releases/1/assets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testPlainBody(t, r, "Upload me !\n")
		fmt.Fprint(w, `{"id":1}`)
	})

	body := []byte("Upload me !\n")
	reader := bytes.NewReader(body)
	size := int64(len(body))

	release := &RepositoryRelease{UploadURL: "/repos/o/r/releases/1/assets{?name,label}"}

	ctx := t.Context()
	asset, _, err := client.Repositories.UploadReleaseAssetFromRelease(ctx, release, nil, reader, size)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := &ReleaseAsset{ID: Ptr(int64(1))}
	if !cmp.Equal(asset, want) {
		t.Fatalf("Repositories.UploadReleaseAssetFromRelease returned %+v, want %+v", asset, want)
	}

	const methodName = "UploadReleaseAssetFromRelease"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.UploadReleaseAssetFromRelease(ctx, release, nil, reader, size)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_UploadReleaseAssetFromRelease_WithMediaType(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// Expect explicit media type to be used.
	mux.HandleFunc("/repos/o/r/releases/1/assets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Content-Type", "image/png")
		fmt.Fprint(w, `{"id":1}`)
	})

	body := []byte("Binary!")
	reader := bytes.NewReader(body)
	size := int64(len(body))

	release := &RepositoryRelease{UploadURL: "/repos/o/r/releases/1/assets{?name,label}"}

	opts := &UploadOptions{Name: "n.txt", MediaType: "image/png"}

	ctx := t.Context()
	asset, _, err := client.Repositories.UploadReleaseAssetFromRelease(ctx, release, opts, reader, size)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := &ReleaseAsset{ID: Ptr(int64(1))}
	if !cmp.Equal(asset, want) {
		t.Fatalf("UploadReleaseAssetFromRelease returned %+v, want %+v", asset, want)
	}
}
