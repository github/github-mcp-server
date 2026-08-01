// Copyright 2014 The go-github AUTHORS. All rights reserved.
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

func TestMetaService_Get(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/meta", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"web":["w"],"api":["a"],"hooks":["h"], "git":["g"], "pages":["p"], "importer":["i"], "github_enterprise_importer": ["gei"], "actions":["a"], "actions_macos": ["example.com/1", "example.com/2"], "codespaces": ["cs"], "copilot": ["c"], "dependabot":["d"], "verifiable_password_authentication": true, "domains":{"actions_inbound": { "full_domains": ["github.com"], "wildcard_domains": ["*.github.com"]},"website":["*.github.com","*.github.dev","*.github.io","*.example.com/assets","*.example.com"],"artifact_attestations":{"trust_domain":"","services":["*.actions.github.com","tuf-repo.github.com","fulcio.github.com","timestamp.github.com"]}}}`)
	})

	ctx := t.Context()
	meta, _, err := client.Meta.Get(ctx)
	if err != nil {
		t.Errorf("Get returned error: %v", err)
	}

	want := &APIMeta{
		Hooks:                    []string{"h"},
		Git:                      []string{"g"},
		Pages:                    []string{"p"},
		Importer:                 []string{"i"},
		GithubEnterpriseImporter: []string{"gei"},
		Actions:                  []string{"a"},
		ActionsMacos:             []string{"example.com/1", "example.com/2"},
		Codespaces:               []string{"cs"},
		Copilot:                  []string{"c"},
		Dependabot:               []string{"d"},
		API:                      []string{"a"},
		Web:                      []string{"w"},
		Domains: &APIMetaDomains{
			Website: []string{
				"*.github.com",
				"*.github.dev",
				"*.github.io",
				"*.example.com/assets",
				"*.example.com",
			},
			ArtifactAttestations: &APIMetaArtifactAttestations{
				TrustDomain: "",
				Services: []string{
					"*.actions.github.com",
					"tuf-repo.github.com",
					"fulcio.github.com",
					"timestamp.github.com",
				},
			},
			ActionsInbound: &ActionsInboundDomains{
				FullDomains:     []string{"github.com"},
				WildcardDomains: []string{"*.github.com"},
			},
		},

		VerifiablePasswordAuthentication: Ptr(true),
	}
	if !cmp.Equal(want, meta) {
		t.Errorf("Get returned %+v, want %+v", meta, want)
	}

	const methodName = "Get"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Meta.Get(ctx)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestMetaService_Octocat(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := "input"
	output := "sample text"

	mux.HandleFunc("/octocat", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"s": input})
		w.Header().Set("Content-Type", "application/octocat-stream")
		fmt.Fprint(w, output)
	})

	ctx := t.Context()
	got, _, err := client.Meta.Octocat(ctx, input)
	if err != nil {
		t.Errorf("Octocat returned error: %v", err)
	}

	if want := output; got != want {
		t.Errorf("Octocat returned %+v, want %+v", got, want)
	}

	const methodName = "Octocat"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Meta.Octocat(ctx, input)
		if got != "" {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestMetaService_Zen(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	output := "sample text"

	mux.HandleFunc("/zen", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.Header().Set("Content-Type", "text/plain;charset=utf-8")
		fmt.Fprint(w, output)
	})

	ctx := t.Context()
	got, _, err := client.Meta.Zen(ctx)
	if err != nil {
		t.Errorf("Zen returned error: %v", err)
	}

	if want := output; got != want {
		t.Errorf("Zen returned %+v, want %+v", got, want)
	}

	const methodName = "Zen"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Meta.Zen(ctx)
		if got != "" {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
