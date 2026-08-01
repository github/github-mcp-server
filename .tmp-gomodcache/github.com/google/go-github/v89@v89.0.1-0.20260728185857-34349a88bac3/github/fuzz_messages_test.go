// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"encoding/json"
	"fmt"
	"testing"
)

// FuzzParseWebHook tests ParseWebHook against arbitrary event types and payloads.
// It verifies that no input triggers a panic or nil pointer dereference.
//
// This fuzz test is intended for integration with OSS-Fuzz (https://google.github.io/oss-fuzz/)
// for continuous fuzzing in the cloud.
//
// To run:
//
//	go test -fuzz=^FuzzParseWebHook$ -fuzztime=30s .
func FuzzParseWebHook(f *testing.F) {
	seeds := []struct {
		eventType string
		payload   string
	}{
		{"push", `{"ref": "refs/heads/main", "before": "000000", "after": "123456", "commits": [{"id": "abc", "message": "msg", "added": [], "removed": [], "modified": []}]}`},
		{"pull_request", `{"action": "opened", "number": 1, "pull_request": {"title": "test", "state": "open", "user": {"login": "u"}}}`},
		{"issues", `{"action": "opened", "issue": {"number": 42, "title": "bug", "state": "open"}}`},
		{"release", `{"action": "published", "release": {"tag_name": "v1.0.0", "draft": false}}`},
		{"check_run", `{"action": "created", "check_run": {"status": "in_progress", "id": 1}}`},
		{"check_suite", `{"action": "completed", "check_suite": {"id": 1, "status": "completed"}}`},
		{"workflow_run", `{"action": "requested", "workflow_run": {"id": 123, "status": "queued"}}`},
		{"workflow_job", `{"action": "queued", "workflow_job": {"id": 1, "status": "queued"}}`},
		{"discussion", `{"action": "created", "discussion": {"title": "hello", "number": 1}}`},
		{"ping", `{"zen": "Keep it logically awesome.", "hook_id": 1}`},
		{"repository", `{"action": "created", "repository": {"name": "test-repo", "private": false}}`},
		{"star", `{"action": "created", "starred_at": "2026-03-11T00:00:00Z"}`},
		{"create", `{"ref": "main", "ref_type": "branch"}`},
		{"delete", `{"ref": "old-branch", "ref_type": "branch"}`},
		{"fork", `{"forkee": {"name": "forked-repo"}}`},
		{"deployment", `{"action": "created", "deployment": {"id": 1, "ref": "main"}}`},
		{"deployment_status", `{"action": "created", "deployment_status": {"id": 1, "state": "pending"}}`},
		{"member", `{"action": "added", "member": {"login": "user"}}`},
		{"public", `{"repository": {"name": "now-public"}}`},
		{"commit_comment", `{"action": "created", "comment": {"id": 1, "body": "comment"}}`},
	}
	for _, s := range seeds {
		f.Add(s.eventType, []byte(s.payload))
	}

	for _, messageType := range MessageTypes() {
		proto := EventForType(messageType)
		if proto == nil {
			f.Add(messageType, []byte(`{}`))
			continue
		}
		// Generate a seed by marshaling the zero-value struct so the fuzzer
		// starts from a structurally valid JSON skeleton for each event type.
		b, err := json.Marshal(proto)
		if err != nil {
			f.Add(messageType, []byte(`{}`))
			continue
		}
		f.Add(messageType, b)
	}

	f.Fuzz(func(_ *testing.T, eventType string, payload []byte) {
		if len(payload) > 1<<20 {
			return
		}
		event, err := ParseWebHook(eventType, payload)
		if err != nil {
			return
		}
		if event != nil {
			// Traverse all fields recursively to catch nil pointer dereferences
			_ = fmt.Sprintf("%+v", event)
		}
	})
}
