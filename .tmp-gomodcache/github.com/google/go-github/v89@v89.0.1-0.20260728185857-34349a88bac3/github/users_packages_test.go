// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUsersService_Authenticated_ListPackages(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/packages", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"package_type": "container", "visibility": "private"})
		fmt.Fprint(w, `[{
			"id": 197,
			"name": "hello_docker",
			"package_type": "container",
			"version_count": 1,
			"visibility": "private",
			"url": "https://api.github.com/orgs/github/packages/container/hello_docker",
			"created_at": `+referenceTimeStr+`,
			"updated_at": `+referenceTimeStr+`,
			"html_url": "https://github.com/orgs/github/packages/container/package/hello_docker"
		  }]`)
	})

	ctx := t.Context()
	packages, _, err := client.Users.ListPackages(ctx, "", &PackageListOptions{PackageType: Ptr("container"), Visibility: Ptr("private")})
	if err != nil {
		t.Errorf("Users.Authenticated_ListPackages returned error: %v", err)
	}

	want := []*Package{{
		ID:           Ptr(int64(197)),
		Name:         Ptr("hello_docker"),
		PackageType:  Ptr("container"),
		VersionCount: Ptr(int64(1)),
		Visibility:   Ptr("private"),
		URL:          Ptr("https://api.github.com/orgs/github/packages/container/hello_docker"),
		HTMLURL:      Ptr("https://github.com/orgs/github/packages/container/package/hello_docker"),
		CreatedAt:    &Timestamp{referenceTime},
		UpdatedAt:    &Timestamp{referenceTime},
	}}
	if !cmp.Equal(packages, want) {
		t.Errorf("Users.Authenticated_ListPackages returned %+v, want %+v", packages, want)
	}

	const methodName = "ListPackages"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListPackages(ctx, "\n", &PackageListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListPackages(ctx, "", &PackageListOptions{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_specifiedUser_ListPackages(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/packages", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"package_type": "container", "visibility": "public"})
		fmt.Fprint(w, `[{
			"id": 197,
			"name": "hello_docker",
			"package_type": "container",
			"version_count": 1,
			"visibility": "public",
			"url": "https://api.github.com/orgs/github/packages/container/hello_docker",
			"created_at": `+referenceTimeStr+`,
			"updated_at": `+referenceTimeStr+`,
			"html_url": "https://github.com/orgs/github/packages/container/package/hello_docker"
		  }]`)
	})

	ctx := t.Context()
	packages, _, err := client.Users.ListPackages(ctx, "u", &PackageListOptions{PackageType: Ptr("container"), Visibility: Ptr("public")})
	if err != nil {
		t.Errorf("Users.specifiedUser_ListPackages returned error: %v", err)
	}

	want := []*Package{{
		ID:           Ptr(int64(197)),
		Name:         Ptr("hello_docker"),
		PackageType:  Ptr("container"),
		VersionCount: Ptr(int64(1)),
		Visibility:   Ptr("public"),
		URL:          Ptr("https://api.github.com/orgs/github/packages/container/hello_docker"),
		HTMLURL:      Ptr("https://github.com/orgs/github/packages/container/package/hello_docker"),
		CreatedAt:    &Timestamp{referenceTime},
		UpdatedAt:    &Timestamp{referenceTime},
	}}
	if !cmp.Equal(packages, want) {
		t.Errorf("Users.specifiedUser_ListPackages returned %+v, want %+v", packages, want)
	}

	const methodName = "ListPackages"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListPackages(ctx, "\n", &PackageListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListPackages(ctx, "", &PackageListOptions{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_specifiedUser_GetPackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/packages/container/hello%2fhello_docker", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		_, err := io.WriteString(w, `{
			"id": 197,
			"name": "hello/hello_docker",
			"package_type": "container",
			"version_count": 1,
			"visibility": "private",
			"url": "https://api.github.com/orgs/github/packages/container/hello%2Fhello_docker",
			"created_at": `+referenceTimeStr+`,
			"updated_at": `+referenceTimeStr+`,
			"html_url": "https://github.com/orgs/github/packages/container/package/hello%2Fhello_docker"
		  }`)
		if err != nil {
			t.Fatal("Failed to write test response: ", err)
		}
	})

	ctx := t.Context()
	packages, _, err := client.Users.GetPackage(ctx, "u", "container", "hello/hello_docker")
	if err != nil {
		t.Errorf("Users.GetPackage returned error: %v", err)
	}

	want := &Package{
		ID:           Ptr(int64(197)),
		Name:         Ptr("hello/hello_docker"),
		PackageType:  Ptr("container"),
		VersionCount: Ptr(int64(1)),
		Visibility:   Ptr("private"),
		URL:          Ptr("https://api.github.com/orgs/github/packages/container/hello%2Fhello_docker"),
		HTMLURL:      Ptr("https://github.com/orgs/github/packages/container/package/hello%2Fhello_docker"),
		CreatedAt:    &Timestamp{referenceTime},
		UpdatedAt:    &Timestamp{referenceTime},
	}
	if !cmp.Equal(packages, want) {
		t.Errorf("Users.specifiedUser_GetPackage returned %+v, want %+v", packages, want)
	}
	const methodName = "GetPackage"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.GetPackage(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.GetPackage(ctx, "", "", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_Authenticated_GetPackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/packages/container/hello_docker", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 197,
			"name": "hello_docker",
			"package_type": "container",
			"version_count": 1,
			"visibility": "private",
			"url": "https://api.github.com/orgs/github/packages/container/hello_docker",
			"created_at": `+referenceTimeStr+`,
			"updated_at": `+referenceTimeStr+`,
			"html_url": "https://github.com/orgs/github/packages/container/package/hello_docker"
		  }`)
	})

	ctx := t.Context()
	packages, _, err := client.Users.GetPackage(ctx, "", "container", "hello_docker")
	if err != nil {
		t.Errorf("Users.Authenticated_GetPackage returned error: %v", err)
	}

	want := &Package{
		ID:           Ptr(int64(197)),
		Name:         Ptr("hello_docker"),
		PackageType:  Ptr("container"),
		VersionCount: Ptr(int64(1)),
		Visibility:   Ptr("private"),
		URL:          Ptr("https://api.github.com/orgs/github/packages/container/hello_docker"),
		HTMLURL:      Ptr("https://github.com/orgs/github/packages/container/package/hello_docker"),
		CreatedAt:    &Timestamp{referenceTime},
		UpdatedAt:    &Timestamp{referenceTime},
	}
	if !cmp.Equal(packages, want) {
		t.Errorf("Users.Authenticated_GetPackage returned %+v, want %+v", packages, want)
	}

	const methodName = "GetPackage"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.GetPackage(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.GetPackage(ctx, "", "", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_Authenticated_DeletePackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/packages/container/hello_docker", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Users.DeletePackage(ctx, "", "container", "hello_docker")
	if err != nil {
		t.Errorf("Users.Authenticated_DeletePackage returned error: %v", err)
	}

	const methodName = "DeletePackage"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.DeletePackage(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.DeletePackage(ctx, "", "", "")
	})
}

func TestUsersService_specifiedUser_DeletePackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/packages/container/hello_docker", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Users.DeletePackage(ctx, "u", "container", "hello_docker")
	if err != nil {
		t.Errorf("Users.specifiedUser_DeletePackage returned error: %v", err)
	}

	const methodName = "DeletePackage"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.DeletePackage(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.DeletePackage(ctx, "", "", "")
	})
}

func TestUsersService_Authenticated_RestorePackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/packages/container/hello_docker/restore", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
	})

	ctx := t.Context()
	_, err := client.Users.RestorePackage(ctx, "", "container", "hello_docker")
	if err != nil {
		t.Errorf("Users.Authenticated_RestorePackage returned error: %v", err)
	}

	const methodName = "RestorePackage"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.RestorePackage(ctx, "\n", "", "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.RestorePackage(ctx, "", "container", "hello_docker")
	})
}

func TestUsersService_specifiedUser_RestorePackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/packages/container/hello_docker/restore", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
	})

	ctx := t.Context()
	_, err := client.Users.RestorePackage(ctx, "u", "container", "hello_docker")
	if err != nil {
		t.Errorf("Users.specifiedUser_RestorePackage returned error: %v", err)
	}

	const methodName = "RestorePackage"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.RestorePackage(ctx, "\n", "", "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.RestorePackage(ctx, "", "container", "hello_docker")
	})
}

func TestUsersService_ListPackageVersions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	m := `{
		"package_type": "container",
		"container": {
			"tags": [
			"latest"
			]
		}
	}`

	mux.HandleFunc("/user/packages/container/hello_docker/versions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
			  "id": 45763,
			  "name": "sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9",
			  "url": "https://api.github.com/users/octocat/packages/container/hello_docker/versions/45763",
			  "package_html_url": "https://github.com/users/octocat/packages/container/package/hello_docker",
			  "created_at": `+referenceTimeStr+`,
			  "updated_at": `+referenceTimeStr+`,
			  "html_url": "https://github.com/users/octocat/packages/container/hello_docker/45763",
			  "metadata": `+m+`
			}]`)
	})

	ctx := t.Context()
	opts := &ListPackageVersionsOptions{
		ListOptions: ListOptions{Page: 1, PerPage: 2},
	}
	packages, _, err := client.Users.ListPackageVersions(ctx, "container", "hello_docker", opts)
	if err != nil {
		t.Errorf("Users.ListPackageVersions returned error: %v", err)
	}

	want := []*PackageVersion{{
		ID:             Ptr(int64(45763)),
		Name:           Ptr("sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9"),
		URL:            Ptr("https://api.github.com/users/octocat/packages/container/hello_docker/versions/45763"),
		PackageHTMLURL: Ptr("https://github.com/users/octocat/packages/container/package/hello_docker"),
		CreatedAt:      &Timestamp{referenceTime},
		UpdatedAt:      &Timestamp{referenceTime},
		HTMLURL:        Ptr("https://github.com/users/octocat/packages/container/hello_docker/45763"),
		Metadata:       json.RawMessage(m),
	}}
	if !cmp.Equal(packages, want) {
		t.Errorf("Users.ListPackageVersions returned %+v, want %+v", packages, want)
	}

	const methodName = "ListPackageVersions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListPackageVersions(ctx, "\n", "\n", &ListPackageVersionsOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListPackageVersions(ctx, "", "", &ListPackageVersionsOptions{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_ListUserPackageVersions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	m := `{
		"package_type": "container",
		"container": {
			"tags": [
			"latest"
			]
		}
	}`

	mux.HandleFunc("/users/u/packages/container/hello_docker/versions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `[
			{
			  "id": 45763,
			  "name": "sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9",
			  "url": "https://api.github.com/users/octocat/packages/container/hello_docker/versions/45763",
			  "package_html_url": "https://github.com/users/octocat/packages/container/package/hello_docker",
			  "created_at": `+referenceTimeStr+`,
			  "updated_at": `+referenceTimeStr+`,
			  "html_url": "https://github.com/users/octocat/packages/container/hello_docker/45763",
			  "metadata": `+m+`
			}]`)
	})

	ctx := t.Context()
	packages, _, err := client.Users.ListUserPackageVersions(ctx, "u", "container", "hello_docker")
	if err != nil {
		t.Errorf("Users.ListUserPackageVersions returned error: %v", err)
	}

	want := []*PackageVersion{{
		ID:             Ptr(int64(45763)),
		Name:           Ptr("sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9"),
		URL:            Ptr("https://api.github.com/users/octocat/packages/container/hello_docker/versions/45763"),
		PackageHTMLURL: Ptr("https://github.com/users/octocat/packages/container/package/hello_docker"),
		CreatedAt:      &Timestamp{referenceTime},
		UpdatedAt:      &Timestamp{referenceTime},
		HTMLURL:        Ptr("https://github.com/users/octocat/packages/container/hello_docker/45763"),
		Metadata:       json.RawMessage(m),
	}}
	if !cmp.Equal(packages, want) {
		t.Errorf("Users.ListUserPackageVersions returned %+v, want %+v", packages, want)
	}

	const methodName = "ListUserPackageVersions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.ListUserPackageVersions(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.ListUserPackageVersions(ctx, "", "", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_Authenticated_PackageGetVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	m := `{
		"package_type": "container",
		"container": {
		"tags": [
			"latest"
		]
		}
	}`

	mux.HandleFunc("/user/packages/container/hello_docker/versions/45763", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `
			{
			  "id": 45763,
			  "name": "sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9",
			  "url": "https://api.github.com/users/octocat/packages/container/hello_docker/versions/45763",
			  "package_html_url": "https://github.com/users/octocat/packages/container/package/hello_docker",
			  "created_at": `+referenceTimeStr+`,
			  "updated_at": `+referenceTimeStr+`,
			  "html_url": "https://github.com/users/octocat/packages/container/hello_docker/45763",
			  "metadata": `+m+`
			}`)
	})

	ctx := t.Context()
	packages, _, err := client.Users.PackageGetVersion(ctx, "", "container", "hello_docker", 45763)
	if err != nil {
		t.Errorf("Users.PackageGetVersion returned error: %v", err)
	}

	want := &PackageVersion{
		ID:             Ptr(int64(45763)),
		Name:           Ptr("sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9"),
		URL:            Ptr("https://api.github.com/users/octocat/packages/container/hello_docker/versions/45763"),
		PackageHTMLURL: Ptr("https://github.com/users/octocat/packages/container/package/hello_docker"),
		CreatedAt:      &Timestamp{referenceTime},
		UpdatedAt:      &Timestamp{referenceTime},
		HTMLURL:        Ptr("https://github.com/users/octocat/packages/container/hello_docker/45763"),
		Metadata:       json.RawMessage(m),
	}
	if !cmp.Equal(packages, want) {
		t.Errorf("Users.Authenticated_PackageGetVersion returned %+v, want %+v", packages, want)
	}

	const methodName = "PackageGetVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.PackageGetVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.PackageGetVersion(ctx, "", "", "", 45763)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_specifiedUser_PackageGetVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	m := `{
		"package_type": "container",
		"container": {
			"tags": [
			"latest"
			]
		}
	}`

	mux.HandleFunc("/users/u/packages/container/hello_docker/versions/45763", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `
			{
			  "id": 45763,
			  "name": "sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9",
			  "url": "https://api.github.com/users/octocat/packages/container/hello_docker/versions/45763",
			  "package_html_url": "https://github.com/users/octocat/packages/container/package/hello_docker",
			  "created_at": `+referenceTimeStr+`,
			  "updated_at": `+referenceTimeStr+`,
			  "html_url": "https://github.com/users/octocat/packages/container/hello_docker/45763",
			  "metadata": `+m+`
			}`)
	})

	ctx := t.Context()
	packages, _, err := client.Users.PackageGetVersion(ctx, "u", "container", "hello_docker", 45763)
	if err != nil {
		t.Errorf("Users.specifiedUser_PackageGetVersion returned error: %v", err)
	}

	want := &PackageVersion{
		ID:             Ptr(int64(45763)),
		Name:           Ptr("sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9"),
		URL:            Ptr("https://api.github.com/users/octocat/packages/container/hello_docker/versions/45763"),
		PackageHTMLURL: Ptr("https://github.com/users/octocat/packages/container/package/hello_docker"),
		CreatedAt:      &Timestamp{referenceTime},
		UpdatedAt:      &Timestamp{referenceTime},
		HTMLURL:        Ptr("https://github.com/users/octocat/packages/container/hello_docker/45763"),
		Metadata:       json.RawMessage(m),
	}
	if !cmp.Equal(packages, want) {
		t.Errorf("Users.specifiedUser_PackageGetVersion returned %+v, want %+v", packages, want)
	}

	const methodName = "PackageGetVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Users.PackageGetVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Users.PackageGetVersion(ctx, "", "", "", 45763)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestUsersService_Authenticated_PackageDeleteVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/packages/container/hello_docker/versions/45763", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Users.PackageDeleteVersion(ctx, "", "container", "hello_docker", 45763)
	if err != nil {
		t.Errorf("Users.Authenticated_PackageDeleteVersion returned error: %v", err)
	}

	const methodName = "PackageDeleteVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.PackageDeleteVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.PackageDeleteVersion(ctx, "", "", "", 45763)
	})
}

func TestUsersService_specifiedUser_PackageDeleteVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/packages/container/hello_docker/versions/45763", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Users.PackageDeleteVersion(ctx, "u", "container", "hello_docker", 45763)
	if err != nil {
		t.Errorf("Users.specifiedUser_PackageDeleteVersion returned error: %v", err)
	}

	const methodName = "PackageDeleteVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.PackageDeleteVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.PackageDeleteVersion(ctx, "", "", "", 45763)
	})
}

func TestUsersService_Authenticated_PackageRestoreVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/packages/container/hello_docker/versions/45763/restore", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
	})

	ctx := t.Context()
	_, err := client.Users.PackageRestoreVersion(ctx, "", "container", "hello_docker", 45763)
	if err != nil {
		t.Errorf("Users.Authenticated_PackageRestoreVersion returned error: %v", err)
	}

	const methodName = "PackageRestoreVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.PackageRestoreVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.PackageRestoreVersion(ctx, "", "", "", 45763)
	})
}

func TestUsersService_specifiedUser_PackageRestoreVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/packages/container/hello_docker/versions/45763/restore", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
	})

	ctx := t.Context()
	_, err := client.Users.PackageRestoreVersion(ctx, "u", "container", "hello_docker", 45763)
	if err != nil {
		t.Errorf("Users.specifiedUser_PackageRestoreVersion returned error: %v", err)
	}

	const methodName = "PackageRestoreVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Users.PackageRestoreVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Users.PackageRestoreVersion(ctx, "", "", "", 45763)
	})
}
