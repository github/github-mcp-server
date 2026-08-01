// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestOrganizationsService_ListPackages(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/packages", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		_, err := io.WriteString(w, `[{
			"id": 197,
			"name": "hello_docker",
			"package_type": "container",
			"owner": {
			  "login": "github",
			  "id": 9919,
			  "node_id": "MDEyOk9yZ2FuaXphdGlvbjk5MTk=",
			  "avatar_url": "https://avatars.githubusercontent.com/u/9919?v=4",
			  "gravatar_id": "",
			  "url": "https://api.github.com/users/github",
			  "html_url": "https://github.com/github",
			  "followers_url": "https://api.github.com/users/github/followers",
			  "following_url": "https://api.github.com/users/github/following{/other_user}",
			  "gists_url": "https://api.github.com/users/github/gists{/gist_id}",
			  "starred_url": "https://api.github.com/users/github/starred{/owner}{/repo}",
			  "subscriptions_url": "https://api.github.com/users/github/subscriptions",
			  "organizations_url": "https://api.github.com/users/github/orgs",
			  "repos_url": "https://api.github.com/users/github/repos",
			  "events_url": "https://api.github.com/users/github/events{/privacy}",
			  "received_events_url": "https://api.github.com/users/github/received_events",
			  "type": "Organization",
			  "site_admin": false
			},
			"version_count": 1,
			"visibility": "private",
			"url": "https://api.github.com/orgs/github/packages/container/hello_docker",
			"created_at": `+referenceTimeStr+`,
			"updated_at": `+referenceTimeStr+`,
			"html_url": "https://github.com/orgs/github/packages/container/package/hello_docker"
		  }
		  ]`)
		if err != nil {
			t.Fatal("Failed to write test response: ", err)
		}
	})

	ctx := t.Context()
	packages, _, err := client.Organizations.ListPackages(ctx, "o", &PackageListOptions{})
	if err != nil {
		t.Errorf("Organizations.ListPackages returned error: %v", err)
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
		Owner: &User{
			Login:             Ptr("github"),
			ID:                Ptr(int64(9919)),
			NodeID:            Ptr("MDEyOk9yZ2FuaXphdGlvbjk5MTk="),
			AvatarURL:         Ptr("https://avatars.githubusercontent.com/u/9919?v=4"),
			GravatarID:        Ptr(""),
			URL:               Ptr("https://api.github.com/users/github"),
			HTMLURL:           Ptr("https://github.com/github"),
			FollowersURL:      Ptr("https://api.github.com/users/github/followers"),
			FollowingURL:      Ptr("https://api.github.com/users/github/following{/other_user}"),
			GistsURL:          Ptr("https://api.github.com/users/github/gists{/gist_id}"),
			StarredURL:        Ptr("https://api.github.com/users/github/starred{/owner}{/repo}"),
			SubscriptionsURL:  Ptr("https://api.github.com/users/github/subscriptions"),
			OrganizationsURL:  Ptr("https://api.github.com/users/github/orgs"),
			ReposURL:          Ptr("https://api.github.com/users/github/repos"),
			EventsURL:         Ptr("https://api.github.com/users/github/events{/privacy}"),
			ReceivedEventsURL: Ptr("https://api.github.com/users/github/received_events"),
			Type:              Ptr("Organization"),
			SiteAdmin:         Ptr(false),
		},
	}}
	if !cmp.Equal(packages, want) {
		t.Errorf("Organizations.ListPackages returned %+v, want %+v", packages, want)
	}

	const methodName = "ListPackages"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.ListPackages(ctx, "\n", &PackageListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.ListPackages(ctx, "o", &PackageListOptions{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_GetPackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// don't url escape the package name here since mux will convert it to a slash automatically
	mux.HandleFunc("/orgs/o/packages/container/hello%2fhello_docker", func(w http.ResponseWriter, r *http.Request) {
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
	packages, _, err := client.Organizations.GetPackage(ctx, "o", "container", "hello/hello_docker")
	if err != nil {
		t.Errorf("Organizations.GetPackage returned error: %v", err)
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
		t.Errorf("Organizations.GetPackage returned %+v, want %+v", packages, want)
	}

	const methodName = "GetPackage"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetPackage(ctx, "\n", "", "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetPackage(ctx, "", "", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_DeletePackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// don't url escape the package name here since mux will convert it to a slash automatically
	mux.HandleFunc("/orgs/o/packages/container/hello%2fhello_docker", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.DeletePackage(ctx, "o", "container", "hello/hello_docker")
	if err != nil {
		t.Errorf("Organizations.DeletePackage returned error: %v", err)
	}

	const methodName = "DeletePackage"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.GetPackage(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.GetPackage(ctx, "", "", "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_RestorePackage(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// don't url escape the package name here since mux will convert it to a slash automatically
	mux.HandleFunc("/orgs/o/packages/container/hello%2Fhello_docker/restore", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
	})

	ctx := t.Context()
	_, err := client.Organizations.RestorePackage(ctx, "o", "container", "hello/hello_docker")
	if err != nil {
		t.Errorf("Organizations.RestorePackage returned error: %v", err)
	}

	const methodName = "RestorePackage"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.RestorePackage(ctx, "\n", "", "")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.RestorePackage(ctx, "", "container", "hello/hello_docker")
	})
}

func TestOrganizationsService_ListPackagesVersions(t *testing.T) {
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

	// don't url escape the package name here since mux will convert it to a slash automatically
	mux.HandleFunc("/orgs/o/packages/container/hello%2Fhello_docker/versions", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"per_page": "2", "page": "1", "state": "deleted", "visibility": "internal", "package_type": "container"})
		_, err := io.WriteString(w, `[
			{
			  "id": 45763,
			  "name": "sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9",
			  "url": "https://api.github.com/users/octocat/packages/container/hello%2Fhello_docker/versions/45763",
			  "package_html_url": "https://github.com/users/octocat/packages/container/package/hello%2Fhello_docker",
			  "created_at": `+referenceTimeStr+`,
			  "updated_at": `+referenceTimeStr+`,
			  "html_url": "https://github.com/users/octocat/packages/container/hello%2Fhello_docker/45763",
			  "metadata": `+m+`
			}]`)
		if err != nil {
			t.Fatal("Failed to write test response: ", err)
		}
	})

	ctx := t.Context()
	opts := &PackageListOptions{
		Ptr("internal"), Ptr("container"), Ptr("deleted"), ListOptions{Page: 1, PerPage: 2},
	}
	packages, _, err := client.Organizations.PackageGetAllVersions(ctx, "o", "container", "hello/hello_docker", opts)
	if err != nil {
		t.Errorf("Organizations.PackageGetAllVersions returned error: %v", err)
	}

	want := []*PackageVersion{{
		ID:             Ptr(int64(45763)),
		Name:           Ptr("sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9"),
		URL:            Ptr("https://api.github.com/users/octocat/packages/container/hello%2Fhello_docker/versions/45763"),
		PackageHTMLURL: Ptr("https://github.com/users/octocat/packages/container/package/hello%2Fhello_docker"),
		CreatedAt:      &Timestamp{referenceTime},
		UpdatedAt:      &Timestamp{referenceTime},
		HTMLURL:        Ptr("https://github.com/users/octocat/packages/container/hello%2Fhello_docker/45763"),
		Metadata:       json.RawMessage(m),
	}}
	if !cmp.Equal(packages, want) {
		t.Errorf("Organizations.PackageGetAllVersions returned %+v, want %+v", packages, want)
	}

	const methodName = "PackageGetAllVersions"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.PackageGetAllVersions(ctx, "\n", "", "", &PackageListOptions{})
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.PackageGetAllVersions(ctx, "", "", "", &PackageListOptions{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_PackageGetVersion(t *testing.T) {
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

	// don't url escape the package name here since mux will convert it to a slash automatically
	mux.HandleFunc("/orgs/o/packages/container/hello%2Fhello_docker/versions/45763", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		_, err := io.WriteString(w, `
			{
			  "id": 45763,
			  "name": "sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9",
			  "url": "https://api.github.com/users/octocat/packages/container/hello%2Fhello_docker/versions/45763",
			  "package_html_url": "https://github.com/users/octocat/packages/container/package/hello%2Fhello_docker",
			  "created_at": `+referenceTimeStr+`,
			  "updated_at": `+referenceTimeStr+`,
			  "html_url": "https://github.com/users/octocat/packages/container/hello%2Fhello_docker/45763",
			  "metadata": `+m+`
			}`)
		if err != nil {
			t.Fatal("Failed to write test response: ", err)
		}
	})

	ctx := t.Context()
	packages, _, err := client.Organizations.PackageGetVersion(ctx, "o", "container", "hello/hello_docker", 45763)
	if err != nil {
		t.Errorf("Organizations.PackageGetVersion returned error: %v", err)
	}

	want := &PackageVersion{
		ID:             Ptr(int64(45763)),
		Name:           Ptr("sha256:08a44bab0bddaddd8837a8b381aebc2e4b933768b981685a9e088360af0d3dd9"),
		URL:            Ptr("https://api.github.com/users/octocat/packages/container/hello%2Fhello_docker/versions/45763"),
		PackageHTMLURL: Ptr("https://github.com/users/octocat/packages/container/package/hello%2Fhello_docker"),
		CreatedAt:      &Timestamp{referenceTime},
		UpdatedAt:      &Timestamp{referenceTime},
		HTMLURL:        Ptr("https://github.com/users/octocat/packages/container/hello%2Fhello_docker/45763"),
		Metadata:       json.RawMessage(m),
	}
	if !cmp.Equal(packages, want) {
		t.Errorf("Organizations.PackageGetVersion returned %+v, want %+v", packages, want)
	}

	const methodName = "PackageGetVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Organizations.PackageGetVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Organizations.PackageGetVersion(ctx, "", "", "", 45763)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestOrganizationsService_PackageDeleteVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// don't url escape the package name here since mux will convert it to a slash automatically
	mux.HandleFunc("/orgs/o/packages/container/hello%2Fhello_docker/versions/45763", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Organizations.PackageDeleteVersion(ctx, "o", "container", "hello/hello_docker", 45763)
	if err != nil {
		t.Errorf("Organizations.PackageDeleteVersion returned error: %v", err)
	}

	const methodName = "PackageDeleteVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.PackageDeleteVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.PackageDeleteVersion(ctx, "", "", "", 45763)
	})
}

func TestOrganizationsService_PackageRestoreVersion(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	// don't url escape the package name here since mux will convert it to a slash automatically
	mux.HandleFunc("/orgs/o/packages/container/hello%2Fhello_docker/versions/45763/restore", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
	})

	ctx := t.Context()
	_, err := client.Organizations.PackageRestoreVersion(ctx, "o", "container", "hello/hello_docker", 45763)
	if err != nil {
		t.Errorf("Organizations.PackageRestoreVersion returned error: %v", err)
	}

	const methodName = "PackageRestoreVersion"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Organizations.PackageRestoreVersion(ctx, "\n", "", "", 0)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Organizations.PackageRestoreVersion(ctx, "", "", "", 45763)
	})
}
