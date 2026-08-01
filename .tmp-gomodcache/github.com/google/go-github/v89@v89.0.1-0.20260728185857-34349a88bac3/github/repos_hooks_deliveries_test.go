// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRepositoriesService_ListHookDeliveries(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/hooks/1/deliveries", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testFormValues(t, r, values{"cursor": "v1_12077215967"})
		fmt.Fprint(w, `[{"id":1}, {"id":2}]`)
	})

	opt := &ListCursorOptions{Cursor: "v1_12077215967"}

	ctx := t.Context()
	hooks, _, err := client.Repositories.ListHookDeliveries(ctx, "o", "r", 1, opt)
	if err != nil {
		t.Errorf("Repositories.ListHookDeliveries returned error: %v", err)
	}

	want := []*HookDelivery{{ID: Ptr(int64(1))}, {ID: Ptr(int64(2))}}
	if d := cmp.Diff(hooks, want); d != "" {
		t.Errorf("Repositories.ListHooks want (-), got (+):\n%v", d)
	}

	const methodName = "ListHookDeliveries"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.ListHookDeliveries(ctx, "\n", "\n", -1, opt)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.ListHookDeliveries(ctx, "o", "r", 1, opt)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_ListHookDeliveries_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.ListHookDeliveries(ctx, "%", "%", 1, nil)
	testURLParseError(t, err)
}

func TestRepositoriesService_GetHookDelivery(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/hooks/1/deliveries/1", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Repositories.GetHookDelivery(ctx, "o", "r", 1, 1)
	if err != nil {
		t.Errorf("Repositories.GetHookDelivery returned error: %v", err)
	}

	want := &HookDelivery{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Repositories.GetHookDelivery returned %+v, want %+v", hook, want)
	}

	const methodName = "GetHookDelivery"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetHookDelivery(ctx, "\n", "\n", -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetHookDelivery(ctx, "o", "r", 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestRepositoriesService_GetHookDelivery_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Repositories.GetHookDelivery(ctx, "%", "%", 1, 1)
	testURLParseError(t, err)
}

func TestRepositoriesService_RedeliverHookDelivery(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/hooks/1/deliveries/1/attempts", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"id":1}`)
	})

	ctx := t.Context()
	hook, _, err := client.Repositories.RedeliverHookDelivery(ctx, "o", "r", 1, 1)
	if err != nil {
		t.Errorf("Repositories.RedeliverHookDelivery returned error: %v", err)
	}

	want := &HookDelivery{ID: Ptr(int64(1))}
	if !cmp.Equal(hook, want) {
		t.Errorf("Repositories.RedeliverHookDelivery returned %+v, want %+v", hook, want)
	}

	const methodName = "RedeliverHookDelivery"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.RedeliverHookDelivery(ctx, "\n", "\n", -1, -1)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.RedeliverHookDelivery(ctx, "o", "r", 1, 1)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

var hookDeliveryPayloadTypeToStruct = map[string]any{
	"check_run":                      &CheckRunEvent{},
	"check_suite":                    &CheckSuiteEvent{},
	"code_scanning_alert":            &CodeScanningAlertEvent{},
	"commit_comment":                 &CommitCommentEvent{},
	"content_reference":              &ContentReferenceEvent{},
	"create":                         &CreateEvent{},
	"delete":                         &DeleteEvent{},
	"dependabot_alert":               &DependabotAlertEvent{},
	"deploy_key":                     &DeployKeyEvent{},
	"deployment":                     &DeploymentEvent{},
	"deployment_status":              &DeploymentStatusEvent{},
	"discussion_comment":             &DiscussionCommentEvent{},
	"discussion":                     &DiscussionEvent{},
	"fork":                           &ForkEvent{},
	"github_app_authorization":       &GitHubAppAuthorizationEvent{},
	"gollum":                         &GollumEvent{},
	"installation":                   &InstallationEvent{},
	"installation_repositories":      &InstallationRepositoriesEvent{},
	"issue_comment":                  &IssueCommentEvent{},
	"issues":                         &IssuesEvent{},
	"label":                          &LabelEvent{},
	"marketplace_purchase":           &MarketplacePurchaseEvent{},
	"member":                         &MemberEvent{},
	"membership":                     &MembershipEvent{},
	"meta":                           &MetaEvent{},
	"milestone":                      &MilestoneEvent{},
	"organization":                   &OrganizationEvent{},
	"org_block":                      &OrgBlockEvent{},
	"package":                        &PackageEvent{},
	"page_build":                     &PageBuildEvent{},
	"ping":                           &PingEvent{},
	"projects_v2":                    &ProjectV2Event{},
	"projects_v2_item":               &ProjectV2ItemEvent{},
	"public":                         &PublicEvent{},
	"pull_request":                   &PullRequestEvent{},
	"pull_request_review":            &PullRequestReviewEvent{},
	"pull_request_review_comment":    &PullRequestReviewCommentEvent{},
	"pull_request_review_thread":     &PullRequestReviewThreadEvent{},
	"pull_request_target":            &PullRequestTargetEvent{},
	"push":                           &PushEvent{},
	"registry_package":               &RegistryPackageEvent{},
	"release":                        &ReleaseEvent{},
	"repository":                     &RepositoryEvent{},
	"repository_dispatch":            &RepositoryDispatchEvent{},
	"repository_import":              &RepositoryImportEvent{},
	"repository_vulnerability_alert": &RepositoryVulnerabilityAlertEvent{},
	"secret_scanning_alert":          &SecretScanningAlertEvent{},
	"security_advisory":              &SecurityAdvisoryEvent{},
	"security_and_analysis":          &SecurityAndAnalysisEvent{},
	"star":                           &StarEvent{},
	"status":                         &StatusEvent{},
	"team":                           &TeamEvent{},
	"team_add":                       &TeamAddEvent{},
	"user":                           &UserEvent{},
	"watch":                          &WatchEvent{},
	"workflow_dispatch":              &WorkflowDispatchEvent{},
	"workflow_job":                   &WorkflowJobEvent{},
	"workflow_run":                   &WorkflowRunEvent{},
}

func TestHookDelivery_ParsePayload(t *testing.T) {
	t.Parallel()
	for evt, obj := range hookDeliveryPayloadTypeToStruct {
		t.Run(evt, func(t *testing.T) {
			t.Parallel()
			bs, err := json.Marshal(obj)
			if err != nil {
				t.Fatal(err)
			}

			p := json.RawMessage(bs)

			d := &HookDelivery{
				Event: &evt,
				Request: &HookRequest{
					RawPayload: &p,
				},
			}

			got, err := d.ParseRequestPayload()
			if err != nil {
				t.Error(err)
			}

			if !cmp.Equal(obj, got) {
				t.Errorf("want %T %v, got %T %v", obj, obj, got, got)
			}
		})
	}
}

func TestHookDelivery_ParsePayload_invalidEvent(t *testing.T) {
	t.Parallel()
	p := json.RawMessage(nil)

	d := &HookDelivery{
		Event: Ptr("some_invalid_event"),
		Request: &HookRequest{
			RawPayload: &p,
		},
	}

	_, err := d.ParseRequestPayload()
	if err == nil || err.Error() != `unsupported event type "some_invalid_event"` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHookDelivery_ParsePayload_invalidPayload(t *testing.T) {
	t.Parallel()
	p := json.RawMessage([]byte(`{"check_run":{"id":"invalid"}}`))

	d := &HookDelivery{
		Event: Ptr("check_run"),
		Request: &HookRequest{
			RawPayload: &p,
		},
	}

	_, err := d.ParseRequestPayload()
	if err == nil || !strings.Contains(err.Error(), "json: cannot unmarshal") || !strings.Contains(err.Error(), "check_run.id") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHookRequest_GetHeader(t *testing.T) {
	t.Parallel()

	header := make(map[string]string)
	header["key1"] = "value1"
	header["Key+2"] = "value2"
	header["kEy-3"] = "value3"
	header["KEY_4"] = "value4"

	r := &HookRequest{
		Headers: header,
	}

	// Checking positive cases
	testPrefixes := []string{"key", "Key", "kEy", "KEY"}
	for hdrKey, hdrValue := range header {
		for _, prefix := range testPrefixes {
			key := prefix + hdrKey[3:]
			if val := r.GetHeader(key); val != hdrValue {
				t.Errorf("GetHeader(%q) is not working: %q != %q", key, val, hdrValue)
			}
		}
	}

	// Checking negative case
	key := "asd"
	if val := r.GetHeader(key); val != "" {
		t.Errorf("GetHeader(%q) should return empty value: %q != %q", key, val, "")
	}
	key = "kay1"
	if val := r.GetHeader(key); val != "" {
		t.Errorf("GetHeader(%q) should return empty value: %q != %q", key, val, "")
	}
}

func TestHookResponse_GetHeader(t *testing.T) {
	t.Parallel()

	header := make(map[string]string)
	header["key1"] = "value1"
	header["Key+2"] = "value2"
	header["kEy-3"] = "value3"
	header["KEY_4"] = "value4"

	r := &HookResponse{
		Headers: header,
	}

	// Checking positive cases
	testPrefixes := []string{"key", "Key", "kEy", "KEY"}
	for hdrKey, hdrValue := range header {
		for _, prefix := range testPrefixes {
			key := prefix + hdrKey[3:]
			if val := r.GetHeader(key); val != hdrValue {
				t.Errorf("GetHeader(%q) is not working: %q != %q", key, val, hdrValue)
			}
		}
	}

	// Checking negative case
	key := "asd"
	if val := r.GetHeader(key); val != "" {
		t.Errorf("GetHeader(%q) should return empty value: %q != %q", key, val, "")
	}
	key = "kay1"
	if val := r.GetHeader(key); val != "" {
		t.Errorf("GetHeader(%q) should return empty value: %q != %q", key, val, "")
	}
}
