// Copyright 2016 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMessageMAC_BadHashTypePrefix(t *testing.T) {
	t.Parallel()
	const signature = "bogus1=1234567"
	if _, _, err := messageMAC(signature); err == nil {
		t.Fatal("messageMAC returned nil; wanted error")
	}
}

func TestValidatePayload(t *testing.T) {
	t.Parallel()
	const defaultBody = `{"yo":true}` // All tests below use the default request body and signature.
	const defaultSignature = "sha1=126f2c800419c60137ce748d7672e77b65cf16d6"
	secretKey := []byte("0123456789abcdef")
	tests := []struct {
		secretKey       []byte
		signature       string
		signatureHeader string
		wantPayload     string
	}{
		// The following tests generate expected errors:
		{secretKey: secretKey},                           // Missing signature
		{secretKey: secretKey, signature: "yo"},          // Missing signature prefix
		{secretKey: secretKey, signature: "sha1=yo"},     // Signature not hex string
		{secretKey: secretKey, signature: "sha1=012345"}, // Invalid signature
		{signature: defaultSignature},                    // signature without secretKey

		// The following tests expect err=nil:
		{
			// no secretKey and no signature still passes validation
			wantPayload: defaultBody,
		},
		{
			secretKey:   secretKey,
			signature:   defaultSignature,
			wantPayload: defaultBody,
		},
		{
			secretKey:   secretKey,
			signature:   "sha256=b1f8020f5b4cd42042f807dd939015c4a418bc1ff7f604dd55b0a19b5d953d9b",
			wantPayload: defaultBody,
		},
		{
			secretKey:       secretKey,
			signature:       "sha256=b1f8020f5b4cd42042f807dd939015c4a418bc1ff7f604dd55b0a19b5d953d9b",
			signatureHeader: SHA256SignatureHeader,
			wantPayload:     defaultBody,
		},
		{
			secretKey:   secretKey,
			signature:   "sha512=8456767023c1195682e182a23b3f5d19150ecea598fde8cb85918f7281b16079471b1329f92b912c4d8bd7455cb159777db8f29608b20c7c87323ba65ae62e1f",
			wantPayload: defaultBody,
		},
	}

	for _, test := range tests {
		buf := bytes.NewBufferString(defaultBody)
		req, err := http.NewRequest("GET", "http://localhost/event", buf)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		if test.signature != "" {
			if test.signatureHeader != "" {
				req.Header.Set(test.signatureHeader, test.signature)
			} else {
				req.Header.Set(SHA1SignatureHeader, test.signature)
			}
		}
		req.Header.Set("Content-Type", "application/json")

		got, err := ValidatePayload(req, test.secretKey)
		if err != nil {
			if test.wantPayload != "" {
				t.Errorf("ValidatePayload(%#v): err = %v, want nil", test, err)
			}
			continue
		}
		if string(got) != test.wantPayload {
			t.Errorf("ValidatePayload = %q, want %q", got, test.wantPayload)
		}
	}
}

func TestValidatePayload_FormGet(t *testing.T) {
	t.Parallel()
	payload := `{"yo":true}`
	signature := "sha1=3374ef144403e8035423b23b02e2c9d7a4c50368"
	secretKey := []byte("0123456789abcdef")

	form := url.Values{}
	form.Add("payload", payload)
	req, err := http.NewRequest("POST", "http://localhost/event", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.PostForm = form
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(SHA1SignatureHeader, signature)

	got, err := ValidatePayload(req, secretKey)
	if err != nil {
		t.Errorf("ValidatePayload(%#v): err = %v, want nil", payload, err)
	}
	if string(got) != payload {
		t.Errorf("ValidatePayload = %q, want %q", got, payload)
	}

	// check that if payload is invalid we get error
	req.Header.Set(SHA1SignatureHeader, "invalid signature")
	if _, err = ValidatePayload(req, []byte{0}); err == nil {
		t.Error("ValidatePayload = nil, want err")
	}
}

func TestValidatePayload_FormPost(t *testing.T) {
	t.Parallel()
	payload := `{"yo":true}`
	signature := "sha1=3374ef144403e8035423b23b02e2c9d7a4c50368"
	secretKey := []byte("0123456789abcdef")

	form := url.Values{}
	form.Set("payload", payload)
	buf := bytes.NewBufferString(form.Encode())
	req, err := http.NewRequest("POST", "http://localhost/event", buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(SHA1SignatureHeader, signature)

	got, err := ValidatePayload(req, secretKey)
	if err != nil {
		t.Errorf("ValidatePayload(%#v): err = %v, want nil", payload, err)
	}
	if string(got) != payload {
		t.Errorf("ValidatePayload = %q, want %q", got, payload)
	}

	// check that if payload is invalid we get error
	req.Header.Set(SHA1SignatureHeader, "invalid signature")
	if _, err = ValidatePayload(req, []byte{0}); err == nil {
		t.Error("ValidatePayload = nil, want err")
	}
}

func TestValidatePayload_InvalidContentType(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequest("POST", "http://localhost/event", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "invalid content type")
	if _, err = ValidatePayload(req, nil); err == nil {
		t.Error("ValidatePayload = nil, want err")
	}
}

func TestValidatePayload_NoSecretKey(t *testing.T) {
	t.Parallel()
	payload := `{"yo":true}`

	form := url.Values{}
	form.Set("payload", payload)
	buf := bytes.NewBufferString(form.Encode())
	req, err := http.NewRequest("POST", "http://localhost/event", buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got, err := ValidatePayload(req, nil)
	if err != nil {
		t.Errorf("ValidatePayload(%#v): err = %v, want nil", payload, err)
	}
	if string(got) != payload {
		t.Errorf("ValidatePayload = %q, want %q", got, payload)
	}
}

// badReader satisfies io.Reader but always returns an error.
type badReader struct{}

func (b *badReader) Read([]byte) (int, error) {
	return 0, errors.New("bad reader")
}

func (b *badReader) Close() error { return errors.New("bad reader") }

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func TestValidatePayload_BadRequestBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		contentType string
	}{
		{contentType: "application/json"},
		{contentType: "application/x-www-form-urlencoded"},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test #%v", i), func(t *testing.T) {
			t.Parallel()
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{tt.contentType}},
				Body:   &badReader{},
			}
			if _, err := ValidatePayload(req, nil); err == nil {
				t.Fatal("ValidatePayload returned nil; want error")
			}
		})
	}
}

func TestValidatePayload_OversizedBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		contentType string
	}{
		{contentType: "application/json"},
		{contentType: "application/x-www-form-urlencoded"},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test #%v", i), func(t *testing.T) {
			t.Parallel()
			// Simulate a reader that reports more than maxPayloadSize bytes.
			oversized := io.LimitReader(readerFunc(func(p []byte) (int, error) {
				for i := range p {
					p[i] = 0
				}
				return len(p), nil
			}), maxPayloadSize+1)
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{tt.contentType}},
				Body:   io.NopCloser(oversized),
			}
			_, err := ValidatePayload(req, nil)
			if err == nil {
				t.Fatal("ValidatePayload returned nil; want error for oversized body")
			}
			if want := "webhook payload exceeds maximum allowed size"; err.Error() != want {
				t.Errorf("ValidatePayload error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestValidatePayload_InvalidContentTypeParams(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequest("POST", "http://localhost/event", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=")
	if _, err = ValidatePayload(req, nil); err == nil {
		t.Error("ValidatePayload = nil, want err")
	}
}

func TestValidatePayload_ValidContentTypeParams(t *testing.T) {
	t.Parallel()
	requestBody := `{"yo":true}`
	buf := bytes.NewBufferString(requestBody)

	req, err := http.NewRequest("POST", "http://localhost/event", buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	_, err = ValidatePayload(req, nil)
	if err != nil {
		t.Error("ValidatePayload = nil, want err")
	}
}

func TestParseWebHook(t *testing.T) {
	t.Parallel()
	tests := []struct {
		payload     any
		messageType string
	}{
		{
			payload:     &BranchProtectionConfigurationEvent{},
			messageType: "branch_protection_configuration",
		},
		{
			payload:     &BranchProtectionRuleEvent{},
			messageType: "branch_protection_rule",
		},
		{
			payload:     &CheckRunEvent{},
			messageType: "check_run",
		},
		{
			payload:     &CheckSuiteEvent{},
			messageType: "check_suite",
		},
		{
			payload:     &CodeScanningAlertEvent{},
			messageType: "code_scanning_alert",
		},
		{
			payload:     &CommitCommentEvent{},
			messageType: "commit_comment",
		},
		{
			payload:     &ContentReferenceEvent{},
			messageType: "content_reference",
		},
		{
			payload:     &CreateEvent{},
			messageType: "create",
		},
		{
			payload:     &CustomPropertyEvent{},
			messageType: "custom_property",
		},
		{
			payload:     &CustomPropertyValuesEvent{},
			messageType: "custom_property_values",
		},
		{
			payload:     &DeleteEvent{},
			messageType: "delete",
		},
		{
			payload:     &DependabotAlertEvent{},
			messageType: "dependabot_alert",
		},
		{
			payload:     &DeployKeyEvent{},
			messageType: "deploy_key",
		},
		{
			payload:     &DeploymentEvent{},
			messageType: "deployment",
		},
		{
			payload:     &DeploymentProtectionRuleEvent{},
			messageType: "deployment_protection_rule",
		},
		{
			payload:     &DeploymentReviewEvent{},
			messageType: "deployment_review",
		},
		{
			payload:     &DeploymentStatusEvent{},
			messageType: "deployment_status",
		},
		{
			payload:     &DiscussionCommentEvent{},
			messageType: "discussion_comment",
		},
		{
			payload:     &DiscussionEvent{},
			messageType: "discussion",
		},
		{
			payload:     &ForkEvent{},
			messageType: "fork",
		},
		{
			payload:     &GitHubAppAuthorizationEvent{},
			messageType: "github_app_authorization",
		},
		{
			payload:     &GollumEvent{},
			messageType: "gollum",
		},
		{
			payload:     &InstallationEvent{},
			messageType: "installation",
		},
		{
			payload:     &InstallationRepositoriesEvent{},
			messageType: "installation_repositories",
		},
		{
			payload:     &InstallationTargetEvent{},
			messageType: "installation_target",
		},
		{
			payload:     &IssueCommentEvent{},
			messageType: "issue_comment",
		},
		{
			payload:     &IssuesEvent{},
			messageType: "issues",
		},
		{
			payload:     &LabelEvent{},
			messageType: "label",
		},
		{
			payload:     &MarketplacePurchaseEvent{},
			messageType: "marketplace_purchase",
		},
		{
			payload:     &MemberEvent{},
			messageType: "member",
		},
		{
			payload:     &MembershipEvent{},
			messageType: "membership",
		},
		{
			payload:     &MergeGroupEvent{},
			messageType: "merge_group",
		},
		{
			payload:     &MetaEvent{},
			messageType: "meta",
		},
		{
			payload:     &MilestoneEvent{},
			messageType: "milestone",
		},
		{
			payload:     &OrganizationEvent{},
			messageType: "organization",
		},
		{
			payload:     &OrgBlockEvent{},
			messageType: "org_block",
		},
		{
			payload:     &PackageEvent{},
			messageType: "package",
		},
		{
			payload:     &PageBuildEvent{},
			messageType: "page_build",
		},
		{
			payload:     &PersonalAccessTokenRequestEvent{},
			messageType: "personal_access_token_request",
		},
		{
			payload:     &PingEvent{},
			messageType: "ping",
		},
		{
			payload:     &ProjectV2Event{},
			messageType: "projects_v2",
		},
		{
			payload:     &ProjectV2ItemEvent{},
			messageType: "projects_v2_item",
		},
		{
			payload:     &PublicEvent{},
			messageType: "public",
		},
		{
			payload:     &PullRequestEvent{},
			messageType: "pull_request",
		},
		{
			payload:     &PullRequestReviewEvent{},
			messageType: "pull_request_review",
		},
		{
			payload:     &PullRequestReviewCommentEvent{},
			messageType: "pull_request_review_comment",
		},
		{
			payload:     &PullRequestReviewThreadEvent{},
			messageType: "pull_request_review_thread",
		},
		{
			payload:     &PullRequestTargetEvent{},
			messageType: "pull_request_target",
		},
		{
			payload:     &PushEvent{},
			messageType: "push",
		},
		{
			payload:     &RegistryPackageEvent{},
			messageType: "registry_package",
		},
		{
			payload:     &ReleaseEvent{},
			messageType: "release",
		},
		{
			payload:     &RepositoryEvent{},
			messageType: "repository",
		},
		{
			payload:     &RepositoryRulesetEvent{},
			messageType: "repository_ruleset",
		},
		{
			payload:     &RepositoryVulnerabilityAlertEvent{},
			messageType: "repository_vulnerability_alert",
		},
		{
			payload:     &SecretScanningAlertEvent{},
			messageType: "secret_scanning_alert",
		},
		{
			payload:     &SecretScanningAlertLocationEvent{},
			messageType: "secret_scanning_alert_location",
		},
		{
			payload:     &SecurityAdvisoryEvent{},
			messageType: "security_advisory",
		},
		{
			payload:     &SecurityAndAnalysisEvent{},
			messageType: "security_and_analysis",
		},
		{
			payload:     &SponsorshipEvent{},
			messageType: "sponsorship",
		},
		{
			payload:     &StarEvent{},
			messageType: "star",
		},
		{
			payload:     &StatusEvent{},
			messageType: "status",
		},
		{
			payload:     &TeamEvent{},
			messageType: "team",
		},
		{
			payload:     &TeamAddEvent{},
			messageType: "team_add",
		},
		{
			payload:     &UserEvent{},
			messageType: "user",
		},
		{
			payload:     &WatchEvent{},
			messageType: "watch",
		},
		{
			payload:     &RepositoryImportEvent{},
			messageType: "repository_import",
		},
		{
			payload:     &RepositoryDispatchEvent{},
			messageType: "repository_dispatch",
		},
		{
			payload:     &WorkflowDispatchEvent{},
			messageType: "workflow_dispatch",
		},
		{
			payload:     &WorkflowJobEvent{},
			messageType: "workflow_job",
		},
		{
			payload:     &WorkflowRunEvent{},
			messageType: "workflow_run",
		},
	}

	for _, test := range tests {
		p, err := json.Marshal(test.payload)
		if err != nil {
			t.Fatalf("Marshal(%#v): %v", test.payload, err)
		}
		got, err := ParseWebHook(test.messageType, p)
		if err != nil {
			t.Fatalf("ParseWebHook: %v", err)
		}
		if want := test.payload; !cmp.Equal(got, want) {
			t.Errorf("ParseWebHook(%#v, %#v) = %#v, want %#v", test.messageType, p, got, want)
		}
	}
}

func TestAllMessageTypesMapped(t *testing.T) {
	t.Parallel()
	for _, mt := range MessageTypes() {
		if obj := EventForType(mt); obj == nil {
			t.Errorf("messageMap missing message type %q", mt)
		}
	}
}

func TestUnknownMessageType(t *testing.T) {
	t.Parallel()
	if obj := EventForType("unknown"); obj != nil {
		t.Errorf("EventForType(unknown) = %#v, want nil", obj)
	}
	if obj := EventForType(""); obj != nil {
		t.Errorf(`EventForType("") = %#v, want nil`, obj)
	}
}

func TestParseWebHook_BadMessageType(t *testing.T) {
	t.Parallel()
	if _, err := ParseWebHook("bogus message type", []byte("{}")); err == nil {
		t.Fatal("ParseWebHook returned nil; wanted error")
	}
}

func TestValidatePayloadFromBody_UnableToParseBody(t *testing.T) {
	t.Parallel()
	if _, err := ValidatePayloadFromBody("application/x-www-form-urlencoded", bytes.NewReader([]byte(`%`)), "sha1=", []byte{}); err == nil {
		t.Error("ValidatePayloadFromBody returned nil; wanted error")
	}
}

func TestValidatePayloadFromBody_UnsupportedContentType(t *testing.T) {
	t.Parallel()
	if _, err := ValidatePayloadFromBody("invalid", bytes.NewReader([]byte(`{}`)), "sha1=", []byte{}); err == nil {
		t.Error("ValidatePayloadFromBody returned nil; wanted error")
	}
}

func TestDeliveryID(t *testing.T) {
	t.Parallel()
	id := "8970a780-244e-11e7-91ca-da3aabcb9793"
	req, err := http.NewRequest("POST", "http://localhost", nil)
	if err != nil {
		t.Fatalf("DeliveryID: %v", err)
	}
	req.Header.Set("X-Github-Delivery", id)

	got := DeliveryID(req)
	if got != id {
		t.Errorf("DeliveryID(%#v) = %q, want %q", req, got, id)
	}
}

func TestWebHookType(t *testing.T) {
	t.Parallel()
	want := "yo"
	req := &http.Request{
		Header: http.Header{EventTypeHeader: []string{want}},
	}
	if got := WebHookType(req); got != want {
		t.Errorf("WebHookType = %q, want %q", got, want)
	}
}
