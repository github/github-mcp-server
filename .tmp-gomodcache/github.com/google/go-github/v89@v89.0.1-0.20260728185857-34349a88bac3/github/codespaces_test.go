// Copyright 2023 The go-github AUTHORS. All rights reserved.
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

func TestCodespacesService_ListInRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/owner/repo/codespaces", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":     "1",
			"per_page": "2",
		})
		fmt.Fprint(w, `{"total_count":2,"codespaces":[{"id":1,"name":"monalisa-octocat-hello-world-g4wpq6h95q","environment_id":"26a7c758-7299-4a73-b978-5a92a7ae98a0","owner":{"login":"octocat"},"billable_owner":{"login":"octocat"},"repository":{"id":1296269},"machine":{"name":"standardLinux","display_name":"4 cores, 8 GB RAM, 64 GB storage","operating_system":"linux","storage_in_bytes":68719476736,"memory_in_bytes":8589934592,"cpus":4},"prebuild":false,"devcontainer_path":".devcontainer/devcontainer.json","created_at":`+refTimeStr(1136178000)+`,"updated_at":`+refTimeStr(1136178001)+`,"last_used_at":`+refTimeStr(1136178002)+`,"state":"Available","url":"https://api.github.com/user/codespaces/monalisa-octocat-hello-world-g4wpq6h95q","git_status":{"ahead":0,"behind":0,"has_unpushed_changes":false,"has_uncommitted_changes":false,"ref":"main"},"location":"WestUs2","idle_timeout_minutes":60,"web_url":"https://monalisa-octocat-hello-world-g4wpq6h95q.github.dev","machines_url":"https://api.github.com/user/codespaces/monalisa-octocat-hello-world-g4wpq6h95q/machines","start_url":"https://api.github.com/user/codespaces/monalisa-octocat-hello-world-g4wpq6h95q/start","stop_url":"https://api.github.com/user/codespaces/monalisa-octocat-hello-world-g4wpq6h95q/stop","recent_folders":["testfolder1","testfolder2"]},{"id":2}]}`)
	})

	opt := &ListOptions{Page: 1, PerPage: 2}
	ctx := t.Context()
	codespaces, _, err := client.Codespaces.ListInRepo(ctx, "owner", "repo", opt)
	if err != nil {
		t.Errorf("Codespaces.ListInRepo returned error: %v", err)
	}

	want := &ListCodespaces{TotalCount: Ptr(2), Codespaces: []*Codespace{
		{
			ID:            Ptr(int64(1)),
			Name:          Ptr("monalisa-octocat-hello-world-g4wpq6h95q"),
			EnvironmentID: Ptr("26a7c758-7299-4a73-b978-5a92a7ae98a0"),
			Owner: &User{
				Login: Ptr("octocat"),
			},
			BillableOwner: &User{
				Login: Ptr("octocat"),
			},
			Repository: &Repository{
				ID: Ptr(int64(1296269)),
			},
			Machine: &CodespacesMachine{
				Name:            Ptr("standardLinux"),
				DisplayName:     Ptr("4 cores, 8 GB RAM, 64 GB storage"),
				OperatingSystem: Ptr("linux"),
				StorageInBytes:  Ptr(int64(68719476736)),
				MemoryInBytes:   Ptr(int64(8589934592)),
				CPUs:            Ptr(4),
			},
			Prebuild:         Ptr(false),
			DevcontainerPath: Ptr(".devcontainer/devcontainer.json"),
			CreatedAt:        refTimestamp(1136178000),
			UpdatedAt:        refTimestamp(1136178001),
			LastUsedAt:       refTimestamp(1136178002),
			State:            Ptr("Available"),
			URL:              Ptr("https://api.github.com/user/codespaces/monalisa-octocat-hello-world-g4wpq6h95q"),
			GitStatus: &CodespacesGitStatus{
				Ahead:                 Ptr(0),
				Behind:                Ptr(0),
				HasUnpushedChanges:    Ptr(false),
				HasUncommittedChanges: Ptr(false),
				Ref:                   Ptr("main"),
			},
			Location:           Ptr("WestUs2"),
			IdleTimeoutMinutes: Ptr(60),
			WebURL:             Ptr("https://monalisa-octocat-hello-world-g4wpq6h95q.github.dev"),
			MachinesURL:        Ptr("https://api.github.com/user/codespaces/monalisa-octocat-hello-world-g4wpq6h95q/machines"),
			StartURL:           Ptr("https://api.github.com/user/codespaces/monalisa-octocat-hello-world-g4wpq6h95q/start"),
			StopURL:            Ptr("https://api.github.com/user/codespaces/monalisa-octocat-hello-world-g4wpq6h95q/stop"),
			RecentFolders: []string{
				"testfolder1",
				"testfolder2",
			},
		},
		{
			ID: Ptr(int64(2)),
		},
	}}
	if !cmp.Equal(codespaces, want) {
		t.Errorf("Codespaces.ListInRepo returned %+v, want %+v", codespaces, want)
	}

	const methodName = "ListInRepo"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.ListInRepo(ctx, "", "", nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_List(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/codespaces", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{
			"page":          "1",
			"per_page":      "2",
			"repository_id": "1296269",
		})
		fmt.Fprint(w, `{"total_count":1,"codespaces":[{"id":1, "repository": {"id": 1296269}}]}`)
	})

	opt := &ListCodespacesOptions{ListOptions: ListOptions{Page: 1, PerPage: 2}, RepositoryID: 1296269}
	ctx := t.Context()
	codespaces, _, err := client.Codespaces.List(ctx, opt)
	if err != nil {
		t.Errorf("Codespaces.List returned error: %v", err)
	}

	want := &ListCodespaces{TotalCount: Ptr(1), Codespaces: []*Codespace{
		{
			ID: Ptr(int64(1)),
			Repository: &Repository{
				ID: Ptr(int64(1296269)),
			},
		},
	}}
	if !cmp.Equal(codespaces, want) {
		t.Errorf("Codespaces.List returned %+v, want %+v", codespaces, want)
	}

	const methodName = "List"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.List(ctx, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_CreateInRepo(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateCodespaceOptions{
		Ref:                Ptr("main"),
		Geo:                Ptr("WestUs2"),
		Machine:            Ptr("standardLinux"),
		IdleTimeoutMinutes: Ptr(60),
	}

	mux.HandleFunc("/repos/owner/repo/codespaces", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1, "repository": {"id": 1296269}}`)
	})

	ctx := t.Context()
	codespace, _, err := client.Codespaces.CreateInRepo(ctx, "owner", "repo", input)
	if err != nil {
		t.Errorf("Codespaces.CreateInRepo returned error: %v", err)
	}
	want := &Codespace{
		ID: Ptr(int64(1)),
		Repository: &Repository{
			ID: Ptr(int64(1296269)),
		},
	}

	if !cmp.Equal(codespace, want) {
		t.Errorf("Codespaces.CreateInRepo returned %+v, want %+v", codespace, want)
	}

	const methodName = "CreateInRepo"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Codespaces.CreateInRepo(ctx, "\n", "", input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.CreateInRepo(ctx, "o", "r", input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_Start(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/codespaces/codespace_1/start", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":1, "repository": {"id": 1296269}}`)
	})
	ctx := t.Context()
	codespace, _, err := client.Codespaces.Start(ctx, "codespace_1")
	if err != nil {
		t.Errorf("Codespaces.Start returned error: %v", err)
	}
	want := &Codespace{
		ID: Ptr(int64(1)),
		Repository: &Repository{
			ID: Ptr(int64(1296269)),
		},
	}

	if !cmp.Equal(codespace, want) {
		t.Errorf("Codespaces.Start returned %+v, want %+v", codespace, want)
	}

	const methodName = "Start"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Codespaces.Start(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.Start(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_Stop(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/codespaces/codespace_1/stop", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":1, "repository": {"id": 1296269}}`)
	})
	ctx := t.Context()
	codespace, _, err := client.Codespaces.Stop(ctx, "codespace_1")
	if err != nil {
		t.Errorf("Codespaces.Stop returned error: %v", err)
	}
	want := &Codespace{
		ID: Ptr(int64(1)),
		Repository: &Repository{
			ID: Ptr(int64(1296269)),
		},
	}

	if !cmp.Equal(codespace, want) {
		t.Errorf("Codespaces.Stop returned %+v, want %+v", codespace, want)
	}

	const methodName = "Stop"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Codespaces.Stop(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.Stop(ctx, "o")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_Delete(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/codespaces/codespace_1", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Codespaces.Delete(ctx, "codespace_1")
	if err != nil {
		t.Errorf("Codespaces.Delete return error: %v", err)
	}

	const methodName = "Delete"
	testBadOptions(t, methodName, func() (err error) {
		_, err = client.Codespaces.Delete(ctx, "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Codespaces.Delete(ctx, "codespace_1")
	})
}

func TestCodespacesService_ListDevContainerConfigurations(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/codespaces/devcontainers", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"total_count": 1,
			"devcontainers": [{
				"path": ".devcontainer/foobar/devcontainer.json",
				"name": "foobar",
				"display_name": "foobar"
			}]
		}`)
	})

	ctx := t.Context()
	opts := &ListOptions{Page: 1, PerPage: 10}

	got, _, err := client.Codespaces.ListDevContainerConfigurations(ctx, "o", "r", opts)
	if err != nil {
		t.Fatalf("Codespaces.ListDevContainerConfigurations returned error: %v", err)
	}

	want := &DevContainerConfigurations{
		TotalCount: 1,
		Devcontainers: []*DevContainer{
			{
				Path:        ".devcontainer/foobar/devcontainer.json",
				Name:        Ptr("foobar"),
				DisplayName: Ptr("foobar"),
			},
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Codespaces.ListDevContainerConfigurations = %+v, want %+v", got, want)
	}

	const methodName = "ListDevContainerConfigurations"

	testBadOptions(t, methodName, func() error {
		_, _, err := client.Codespaces.ListDevContainerConfigurations(ctx, "\n", "\n", opts)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.ListDevContainerConfigurations(ctx, "e", "r", opts)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_GetDefaultAttributes(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/codespaces/new", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"billable_owner": {
				"login": "user1",
				"id": 1001,
				"url": "https://example.com/user1"
			},
			"defaults": {
				"devcontainer_path": ".devcontainer/devcontainer.json",
				"location": "WestUs2"
			}
		}`)
	})

	ctx := t.Context()

	opt := &CodespaceGetDefaultAttributesOptions{
		Ref:      Ptr("main"),
		ClientIP: Ptr("1.2.3.4"),
	}

	got, _, err := client.Codespaces.GetDefaultAttributes(ctx, "o", "r", opt)
	if err != nil {
		t.Fatalf("Codespaces.GetDefaultAttributes returned error: %v", err)
	}

	want := &CodespaceDefaultAttributes{
		BillableOwner: &User{
			Login: Ptr("user1"),
			ID:    Ptr(int64(1001)),
			URL:   Ptr("https://example.com/user1"),
		},
		Defaults: &CodespaceDefaults{
			DevcontainerPath: Ptr(".devcontainer/devcontainer.json"),
			Location:         "WestUs2",
		},
	}

	if !cmp.Equal(got, want) {
		t.Errorf("Codespaces.GetDefaultAttributes = %+v, want %+v", got, want)
	}

	const methodName = "GetDefaultAttributes"

	testBadOptions(t, methodName, func() error {
		_, _, err := client.Codespaces.GetDefaultAttributes(ctx, "\n", "\n", opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.GetDefaultAttributes(ctx, "e", "r", opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_CheckPermissions(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/codespaces/permissions_check", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"accepted": true}`)
	})

	ctx := t.Context()
	hasPermission, _, err := client.Codespaces.CheckPermissions(ctx, "o", "r", "main", "path")
	if err != nil {
		t.Errorf("Codespaces.CheckPermissions returned error: %v", err)
	}

	want := CodespacePermissions{Accepted: true}
	if !cmp.Equal(hasPermission, &want) {
		t.Errorf("Codespaces.CheckPermissions = %+v, want %+v", hasPermission, want)
	}

	const methodName = "CheckPermissions"

	testBadOptions(t, methodName, func() error {
		_, _, err := client.Codespaces.CheckPermissions(ctx, "\n", "\n", "main", "path")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.CheckPermissions(ctx, "o", "r", "main", "path")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_CreateFromPullRequest(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &CreateCodespaceOptions{
		Machine:            Ptr("standardLinux"),
		IdleTimeoutMinutes: Ptr(60),
	}

	mux.HandleFunc("/repos/owner/repo/pulls/42/codespaces", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id":1, "repository": {"id": 1}}`)
	})

	ctx := t.Context()
	codespace, _, err := client.Codespaces.CreateFromPullRequest(ctx, "owner", "repo", 42, input)
	if err != nil {
		t.Errorf("Codespaces.CreateFromPullRequest returned error: %v", err)
	}
	want := &Codespace{
		ID: Ptr(int64(1)),
		Repository: &Repository{
			ID: Ptr(int64(1)),
		},
	}

	if !cmp.Equal(codespace, want) {
		t.Errorf("Codespaces.CreateFromPullRequest returned %+v, want %+v", codespace, want)
	}

	const methodName = "CreateFromPullRequest"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Codespaces.CreateFromPullRequest(ctx, "\n", "", 0, input)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.CreateFromPullRequest(ctx, "o", "r", 42, input)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_Create(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &CodespaceCreateForUserOptions{
		Ref:                Ptr("main"),
		Geo:                Ptr("WestUs2"),
		Machine:            Ptr("standardLinux"),
		IdleTimeoutMinutes: Ptr(60),
		RepositoryID:       int64(111),
		PullRequest:        nil,
	}

	mux.HandleFunc("/user/codespaces", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, opt)
		fmt.Fprint(w, `{"id":1,"repository":{"id":111}}`)
	})

	ctx := t.Context()
	codespace, _, err := client.Codespaces.Create(
		ctx,
		opt,
	)
	if err != nil {
		t.Fatalf("Codespaces.Create returned error: %v", err)
	}

	want := &Codespace{
		ID: Ptr(int64(1)),
		Repository: &Repository{
			ID: Ptr(int64(111)),
		},
	}

	if !cmp.Equal(codespace, want) {
		t.Errorf("Codespaces.Create returned %+v, want %+v", codespace, want)
	}

	const methodName = "Create"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.Create(
			ctx,
			opt,
		)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_Get(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/codespaces/codespace_1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1,"repository":{"id":111}}`)
	})

	ctx := t.Context()
	codespace, _, err := client.Codespaces.Get(ctx, "codespace_1")
	if err != nil {
		t.Fatalf("Codespaces.Get returned error: %v", err)
	}

	want := &Codespace{
		ID: Ptr(int64(1)),
		Repository: &Repository{
			ID: Ptr(int64(111)),
		},
	}

	if !cmp.Equal(codespace, want) {
		t.Errorf("Codespaces.Get returned %+v, want %+v", codespace, want)
	}

	const methodName = "Get"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.Get(ctx, "codespace_1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_Update(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &UpdateCodespaceOptions{
		Machine: Ptr("standardLinux"),
		RecentFolders: []string{
			"folder1",
			"folder2",
		},
	}

	mux.HandleFunc("/user/codespaces/codespace_1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		testJSONBody(t, r, opt)
		fmt.Fprint(w, `{"id":1,"repository":{"id":111}}`)
	})

	ctx := t.Context()
	codespace, _, err := client.Codespaces.Update(
		ctx,
		"codespace_1",
		opt,
	)
	if err != nil {
		t.Fatalf("Codespaces.Update returned error: %v", err)
	}

	want := &Codespace{
		ID: Ptr(int64(1)),
		Repository: &Repository{
			ID: Ptr(int64(111)),
		},
	}

	if !cmp.Equal(codespace, want) {
		t.Errorf("Codespaces.Update returned %+v, want %+v", codespace, want)
	}

	const methodName = "Update"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.Update(
			ctx,
			"codespace_1",
			opt,
		)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_ExportCodespace(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/codespaces/codespace_1/exports", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
			"state": "succeeded",
			"completed_at": `+referenceTimeStr+`,
			"branch": "main",
			"export_url": "https://api.github.com/user/codespaces/:name/exports/latest"
		}`)
	})

	ctx := t.Context()
	export, _, err := client.Codespaces.ExportCodespace(ctx, "codespace_1")
	if err != nil {
		t.Fatalf("Codespaces.ExportCodespace returned error: %v", err)
	}

	want := &CodespaceExport{
		State:       Ptr("succeeded"),
		CompletedAt: &referenceTimestamp,
		Branch:      Ptr("main"),
		ExportURL:   Ptr("https://api.github.com/user/codespaces/:name/exports/latest"),
	}

	if !cmp.Equal(export, want) {
		t.Errorf("Codespaces.ExportCodespace returned %+v, want %+v", export, want)
	}

	const methodName = "ExportCodespace"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.ExportCodespace(ctx, "codespace_1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_GetLatestCodespaceExport(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/user/codespaces/codespace_1/exports/latest", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"state": "succeeded",
			"completed_at": `+referenceTimeStr+`,
			"branch": "main",
			"export_url": "https://api.github.com/user/codespaces/:name/exports/latest"
		}`)
	})

	ctx := t.Context()
	export, _, err := client.Codespaces.GetLatestCodespaceExport(ctx, "codespace_1")
	if err != nil {
		t.Fatalf("Codespaces.GetLatestCodespaceExport returned error: %v", err)
	}

	want := &CodespaceExport{
		State:       Ptr("succeeded"),
		CompletedAt: &referenceTimestamp,
		Branch:      Ptr("main"),
		ExportURL:   Ptr("https://api.github.com/user/codespaces/:name/exports/latest"),
	}

	if !cmp.Equal(export, want) {
		t.Errorf("Codespaces.GetLatestCodespaceExport returned %+v, want %+v", export, want)
	}

	const methodName = "GetLatestCodespaceExport"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.GetLatestCodespaceExport(ctx, "codespace_1")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCodespacesService_Publish(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	opt := &PublishCodespaceOptions{
		Name:    Ptr("repo"),
		Private: Ptr(true),
	}

	mux.HandleFunc("/user/codespaces/codespace_1/publish", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, opt)
		fmt.Fprint(w, `{"id":1,"repository":{"id":111}}`)
	})

	ctx := t.Context()
	repo, _, err := client.Codespaces.Publish(
		ctx,
		"codespace_1",
		opt,
	)
	if err != nil {
		t.Fatalf("Codespaces.Publish returned error: %v", err)
	}

	want := &Codespace{
		ID: Ptr(int64(1)),
		Repository: &Repository{
			ID: Ptr(int64(111)),
		},
	}
	if !cmp.Equal(repo, want) {
		t.Errorf("Codespaces.Publish returned %+v, want %+v", repo, want)
	}

	const methodName = "Publish"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Codespaces.Publish(
			ctx,
			"codespace_1",
			opt,
		)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
