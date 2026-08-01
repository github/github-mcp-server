// Copyright 2016 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAppsService_Get_authenticatedApp(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	app, _, err := client.Apps.Get(ctx, "")
	if err != nil {
		t.Errorf("Apps.Get returned error: %v", err)
	}

	want := &App{ID: Ptr(int64(1))}
	if !cmp.Equal(app, want) {
		t.Errorf("Apps.Get returned %+v, want %+v", app, want)
	}

	const methodName = "Get"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.Get(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.Get(ctx, "")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_Get_specifiedApp(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/apps/a", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"html_url":"https://github.com/apps/a"}`)
	})

	ctx := t.Context()
	app, _, err := client.Apps.Get(ctx, "a")
	if err != nil {
		t.Errorf("Apps.Get returned error: %v", err)
	}

	want := &App{HTMLURL: Ptr("https://github.com/apps/a")}
	if !cmp.Equal(app, want) {
		t.Errorf("Apps.Get returned %+v, want %+v", *app.HTMLURL, *want.HTMLURL)
	}
}

func TestAppsService_ListInstallationRequests(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/installation-requests", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "2",
		})
		fmt.Fprint(w, `[{
			"id": 1,
			"account": { "id": 2 },
			"requester": { "id": 3 },
			"created_at": `+referenceTimeStr+`
		}]`,
		)
	})

	opt := &ListOptions{Page: 1, PerPage: 2}
	ctx := t.Context()
	installationRequests, _, err := client.Apps.ListInstallationRequests(ctx, opt)
	if err != nil {
		t.Errorf("Apps.ListInstallationRequests returned error: %v", err)
	}

	want := []*InstallationRequest{{
		ID:        Ptr(int64(1)),
		Account:   &User{ID: Ptr(int64(2))},
		Requester: &User{ID: Ptr(int64(3))},
		CreatedAt: &referenceTimestamp,
	}}
	if !cmp.Equal(installationRequests, want) {
		t.Errorf("Apps.ListInstallationRequests returned %+v, want %+v", installationRequests, want)
	}

	const methodName = "ListInstallationRequests"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.ListInstallationRequests(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_ListInstallations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/installations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "2",
		})
		fmt.Fprint(w, `[{
                                   "id":1,
                                   "app_id":1,
                                   "target_id":1,
                                   "target_type": "Organization",
                                   "permissions": {
                                       "actions": "read",
                                       "administration": "read",
                                       "checks": "read",
                                       "contents": "read",
                                       "content_references": "read",
                                       "deployments": "read",
                                       "environments": "read",
                                       "issues": "write",
                                       "metadata": "read",
                                       "members": "read",
                                       "organization_administration": "write",
                                       "organization_custom_roles": "write",
                                       "organization_hooks": "write",
                                       "organization_packages": "write",
                                       "organization_personal_access_tokens": "read",
                                       "organization_personal_access_token_requests": "read",
                                       "organization_plan": "read",
                                       "organization_pre_receive_hooks": "write",
                                       "organization_projects": "read",
                                       "organization_secrets": "read",
                                       "organization_self_hosted_runners": "read",
                                       "organization_user_blocking": "write",
                                       "packages": "read",
                                       "pages": "read",
                                       "pull_requests": "write",
                                       "repository_hooks": "write",
                                       "repository_projects": "read",
                                       "repository_pre_receive_hooks": "read",
                                       "secrets": "read",
                                       "secret_scanning_alerts": "read",
                                       "security_events": "read",
                                       "single_file": "write",
                                       "statuses": "write",
                                       "team_discussions": "read",
                                       "vulnerability_alerts": "read",
                                       "workflows": "write"
                                   },
                                  "events": [
                                      "push",
                                      "pull_request"
                                  ],
                                 "single_file_name": "config.yml",
                                 "repository_selection": "selected",
                                 "created_at": `+refTimeStr(1136178000)+`,
                                 "updated_at": `+refTimeStr(1136178001)+`}]`,
		)
	})

	opt := &ListOptions{Page: 1, PerPage: 2}
	ctx := t.Context()
	installations, _, err := client.Apps.ListInstallations(ctx, opt)
	if err != nil {
		t.Errorf("Apps.ListInstallations returned error: %v", err)
	}

	want := []*Installation{{
		ID:                  Ptr(int64(1)),
		AppID:               Ptr(int64(1)),
		TargetID:            Ptr(int64(1)),
		TargetType:          Ptr("Organization"),
		SingleFileName:      Ptr("config.yml"),
		RepositorySelection: Ptr("selected"),
		Permissions: &InstallationPermissions{
			Actions:                                 Ptr("read"),
			Administration:                          Ptr("read"),
			Checks:                                  Ptr("read"),
			Contents:                                Ptr("read"),
			ContentReferences:                       Ptr("read"),
			Deployments:                             Ptr("read"),
			Environments:                            Ptr("read"),
			Issues:                                  Ptr("write"),
			Metadata:                                Ptr("read"),
			Members:                                 Ptr("read"),
			OrganizationAdministration:              Ptr("write"),
			OrganizationCustomRoles:                 Ptr("write"),
			OrganizationHooks:                       Ptr("write"),
			OrganizationPackages:                    Ptr("write"),
			OrganizationPersonalAccessTokens:        Ptr("read"),
			OrganizationPersonalAccessTokenRequests: Ptr("read"),
			OrganizationPlan:                        Ptr("read"),
			OrganizationPreReceiveHooks:             Ptr("write"),
			OrganizationProjects:                    Ptr("read"),
			OrganizationSecrets:                     Ptr("read"),
			OrganizationSelfHostedRunners:           Ptr("read"),
			OrganizationUserBlocking:                Ptr("write"),
			Packages:                                Ptr("read"),
			Pages:                                   Ptr("read"),
			PullRequests:                            Ptr("write"),
			RepositoryHooks:                         Ptr("write"),
			RepositoryProjects:                      Ptr("read"),
			RepositoryPreReceiveHooks:               Ptr("read"),
			Secrets:                                 Ptr("read"),
			SecretScanningAlerts:                    Ptr("read"),
			SecurityEvents:                          Ptr("read"),
			SingleFile:                              Ptr("write"),
			Statuses:                                Ptr("write"),
			TeamDiscussions:                         Ptr("read"),
			VulnerabilityAlerts:                     Ptr("read"),
			Workflows:                               Ptr("write"),
		},
		Events:    []string{"push", "pull_request"},
		CreatedAt: refTimestamp(1136178000),
		UpdatedAt: refTimestamp(1136178001),
	}}
	if !cmp.Equal(installations, want) {
		t.Errorf("Apps.ListInstallations returned %+v, want %+v", installations, want)
	}

	const methodName = "ListInstallations"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.ListInstallations(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_GetInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/installations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "app_id":1, "target_id":1, "target_type": "Organization"}`)
	})

	ctx := t.Context()
	installation, _, err := client.Apps.GetInstallation(ctx, 1)
	if err != nil {
		t.Errorf("Apps.GetInstallation returned error: %v", err)
	}

	want := &Installation{ID: Ptr(int64(1)), AppID: Ptr(int64(1)), TargetID: Ptr(int64(1)), TargetType: Ptr("Organization")}
	if !cmp.Equal(installation, want) {
		t.Errorf("Apps.GetInstallation returned %+v, want %+v", installation, want)
	}

	const methodName = "GetInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.GetInstallation(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.GetInstallation(ctx, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_ListUserInstallations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/installations", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "2",
		})
		fmt.Fprint(w, `{"installations":[{"id":1, "app_id":1, "target_id":1, "target_type": "Organization"}]}`)
	})

	opt := &ListOptions{Page: 1, PerPage: 2}
	ctx := t.Context()
	installations, _, err := client.Apps.ListUserInstallations(ctx, opt)
	if err != nil {
		t.Errorf("Apps.ListUserInstallations returned error: %v", err)
	}

	want := []*Installation{{ID: Ptr(int64(1)), AppID: Ptr(int64(1)), TargetID: Ptr(int64(1)), TargetType: Ptr("Organization")}}
	if !cmp.Equal(installations, want) {
		t.Errorf("Apps.ListUserInstallations returned %+v, want %+v", installations, want)
	}

	const methodName = "ListUserInstallations"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.ListUserInstallations(ctx, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_SuspendInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/installations/1/suspended", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Apps.SuspendInstallation(ctx, 1); err != nil {
		t.Errorf("Apps.SuspendInstallation returned error: %v", err)
	}

	const methodName = "SuspendInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Apps.SuspendInstallation(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Apps.SuspendInstallation(ctx, 1)
	})
}

func TestAppsService_UnsuspendInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/installations/1/suspended", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")

		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	if _, err := client.Apps.UnsuspendInstallation(ctx, 1); err != nil {
		t.Errorf("Apps.UnsuspendInstallation returned error: %v", err)
	}

	const methodName = "UnsuspendInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Apps.UnsuspendInstallation(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Apps.UnsuspendInstallation(ctx, 1)
	})
}

func TestAppsService_DeleteInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/installations/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
		w.WriteHeader(http.StatusNoContent)
	})

	ctx := t.Context()
	_, err := client.Apps.DeleteInstallation(ctx, 1)
	if err != nil {
		t.Errorf("Apps.DeleteInstallation returned error: %v", err)
	}

	const methodName = "DeleteInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Apps.DeleteInstallation(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Apps.DeleteInstallation(ctx, 1)
	})
}

func TestAppsService_CreateInstallationToken(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/installations/1/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"token":"t"}`)
	})

	ctx := t.Context()
	token, _, err := client.Apps.CreateInstallationToken(ctx, 1, nil)
	if err != nil {
		t.Errorf("Apps.CreateInstallationToken returned error: %v", err)
	}

	want := &InstallationToken{Token: Ptr("t")}
	if !cmp.Equal(token, want) {
		t.Errorf("Apps.CreateInstallationToken returned %+v, want %+v", token, want)
	}

	const methodName = "CreateInstallationToken"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.CreateInstallationToken(ctx, -1, nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.CreateInstallationToken(ctx, 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_CreateInstallationTokenWithOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	installationTokenOptions := &InstallationTokenOptions{
		RepositoryIDs: []int64{1234},
		Repositories:  []string{"foo"},
		Permissions: &InstallationPermissions{
			Contents: Ptr("write"),
			Issues:   Ptr("read"),
		},
	}

	mux.HandleFunc("/app/installations/1/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, installationTokenOptions)
		fmt.Fprint(w, `{"token":"t"}`)
	})

	ctx := t.Context()
	token, _, err := client.Apps.CreateInstallationToken(ctx, 1, installationTokenOptions)
	if err != nil {
		t.Errorf("Apps.CreateInstallationToken returned error: %v", err)
	}

	want := &InstallationToken{Token: Ptr("t")}
	if !cmp.Equal(token, want) {
		t.Errorf("Apps.CreateInstallationToken returned %+v, want %+v", token, want)
	}
}

func TestAppsService_CreateInstallationTokenListReposWithOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	installationTokenListRepoOptions := &InstallationTokenListRepoOptions{
		Repositories: []string{"foo"},
		Permissions: &InstallationPermissions{
			Contents: Ptr("write"),
			Issues:   Ptr("read"),
		},
	}

	mux.HandleFunc("/app/installations/1/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, installationTokenListRepoOptions)
		fmt.Fprint(w, `{"token":"t"}`)
	})

	ctx := t.Context()
	token, _, err := client.Apps.CreateInstallationTokenListRepos(ctx, 1, installationTokenListRepoOptions)
	if err != nil {
		t.Errorf("Apps.CreateInstallationTokenListRepos returned error: %v", err)
	}

	want := &InstallationToken{Token: Ptr("t")}
	if !cmp.Equal(token, want) {
		t.Errorf("Apps.CreateInstallationTokenListRepos returned %+v, want %+v", token, want)
	}
}

func TestAppsService_CreateInstallationTokenListReposWithNoOptions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/app/installations/1/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"token":"t"}`)
	})

	ctx := t.Context()
	token, _, err := client.Apps.CreateInstallationTokenListRepos(ctx, 1, nil)
	if err != nil {
		t.Errorf("Apps.CreateInstallationTokenListRepos returned error: %v", err)
	}

	want := &InstallationToken{Token: Ptr("t")}
	if !cmp.Equal(token, want) {
		t.Errorf("Apps.CreateInstallationTokenListRepos returned %+v, want %+v", token, want)
	}

	const methodName = "CreateInstallationTokenListRepos"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.CreateInstallationTokenListRepos(ctx, -1, nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.CreateInstallationTokenListRepos(ctx, 1, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_CreateAttachment(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/content_references/11/attachments", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Accept", mediaTypeContentAttachmentsPreview)

		w.WriteHeader(http.StatusOK)
		assertWrite(t, w, []byte(`{"id":1,"title":"title1","body":"body1"}`))
	})

	ctx := t.Context()
	got, _, err := client.Apps.CreateAttachment(ctx, 11, "title1", "body1")
	if err != nil {
		t.Errorf("CreateAttachment returned error: %v", err)
	}

	want := &Attachment{ID: Ptr(int64(1)), Title: Ptr("title1"), Body: Ptr("body1")}
	if !cmp.Equal(got, want) {
		t.Errorf("CreateAttachment = %+v, want %+v", got, want)
	}

	const methodName = "CreateAttachment"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.CreateAttachment(ctx, -11, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.CreateAttachment(ctx, 11, "title1", "body1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_GetOrganizationInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/orgs/o/installation", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "app_id":1, "target_id":1, "target_type": "Organization"}`)
	})

	ctx := t.Context()
	installation, _, err := client.Apps.GetOrganizationInstallation(ctx, "o")
	if err != nil {
		t.Errorf("Apps.GetOrganizationInstallation returned error: %v", err)
	}

	want := &Installation{ID: Ptr(int64(1)), AppID: Ptr(int64(1)), TargetID: Ptr(int64(1)), TargetType: Ptr("Organization")}
	if !cmp.Equal(installation, want) {
		t.Errorf("Apps.GetOrganizationInstallation returned %+v, want %+v", installation, want)
	}

	const methodName = "GetOrganizationInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.GetOrganizationInstallation(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.GetOrganizationInstallation(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_GetEnterpriseInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/installation", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "app_id":1, "target_id":1, "target_type": "Enterprise"}`)
	})

	ctx := t.Context()
	installation, _, err := client.Apps.GetEnterpriseInstallation(ctx, "e")
	if err != nil {
		t.Errorf("Apps.GetEnterpriseInstallation returned error: %v", err)
	}

	want := &Installation{ID: Ptr(int64(1)), AppID: Ptr(int64(1)), TargetID: Ptr(int64(1)), TargetType: Ptr("Enterprise")}
	if !cmp.Equal(installation, want) {
		t.Errorf("Apps.GetEnterpriseInstallation returned %+v, want %+v", installation, want)
	}

	const methodName = "GetEnterpriseInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.GetEnterpriseInstallation(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.GetEnterpriseInstallation(ctx, "e")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_GetRepositoryInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/installation", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "app_id":1, "target_id":1, "target_type": "Organization"}`)
	})

	ctx := t.Context()
	installation, _, err := client.Apps.GetRepositoryInstallation(ctx, "o", "r")
	if err != nil {
		t.Errorf("Apps.GetRepositoryInstallation returned error: %v", err)
	}

	want := &Installation{ID: Ptr(int64(1)), AppID: Ptr(int64(1)), TargetID: Ptr(int64(1)), TargetType: Ptr("Organization")}
	if !cmp.Equal(installation, want) {
		t.Errorf("Apps.GetRepositoryInstallation returned %+v, want %+v", installation, want)
	}

	const methodName = "GetRepositoryInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.GetRepositoryInstallation(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.GetRepositoryInstallation(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_GetRepositoryInstallationByID(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repositories/1/installation", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "app_id":1, "target_id":1, "target_type": "Organization"}`)
	})

	ctx := t.Context()
	installation, _, err := client.Apps.GetRepositoryInstallationByID(ctx, 1)
	if err != nil {
		t.Errorf("Apps.GetRepositoryInstallationByID returned error: %v", err)
	}

	want := &Installation{ID: Ptr(int64(1)), AppID: Ptr(int64(1)), TargetID: Ptr(int64(1)), TargetType: Ptr("Organization")}
	if !cmp.Equal(installation, want) {
		t.Errorf("Apps.GetRepositoryInstallationByID returned %+v, want %+v", installation, want)
	}

	const methodName = "GetRepositoryInstallationByID"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.GetRepositoryInstallationByID(ctx, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.GetRepositoryInstallationByID(ctx, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestAppsService_GetUserInstallation(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/users/u/installation", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1, "app_id":1, "target_id":1, "target_type": "User"}`)
	})

	ctx := t.Context()
	installation, _, err := client.Apps.GetUserInstallation(ctx, "u")
	if err != nil {
		t.Errorf("Apps.GetUserInstallation returned error: %v", err)
	}

	want := &Installation{ID: Ptr(int64(1)), AppID: Ptr(int64(1)), TargetID: Ptr(int64(1)), TargetType: Ptr("User")}
	if !cmp.Equal(installation, want) {
		t.Errorf("Apps.GetUserInstallation returned %+v, want %+v", installation, want)
	}

	const methodName = "GetUserInstallation"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Apps.GetUserInstallation(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Apps.GetUserInstallation(ctx, "u")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
