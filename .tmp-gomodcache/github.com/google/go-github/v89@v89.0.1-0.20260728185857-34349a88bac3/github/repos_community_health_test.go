// Copyright 2017 The go-github AUTHORS. All rights reserved.
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

func TestRepositoriesService_GetCommunityHealthMetrics(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/community/profile", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
				"health_percentage": 100,
				"description": "My first repository on GitHub!",
				"documentation": null,
				"files": {
					"code_of_conduct": {
						"name": "Contributor Covenant",
						"key": "contributor_covenant",
						"url": null,
						"html_url": "https://github.com/octocat/Hello-World/blob/master/CODE_OF_CONDUCT.md"
					},
					"code_of_conduct_file": {
						"url": "https://api.github.com/repos/octocat/Hello-World/contents/CODE_OF_CONDUCT.md",
						"html_url": "https://github.com/octocat/Hello-World/blob/master/CODE_OF_CONDUCT.md"
					},
					"contributing": {
						"url": "https://api.github.com/repos/octocat/Hello-World/contents/CONTRIBUTING",
						"html_url": "https://github.com/octocat/Hello-World/blob/master/CONTRIBUTING"
					},
					"issue_template": {
						"url": "https://api.github.com/repos/octocat/Hello-World/contents/ISSUE_TEMPLATE",
						"html_url": "https://github.com/octocat/Hello-World/blob/master/ISSUE_TEMPLATE"
					},
					"pull_request_template": {
						"url": "https://api.github.com/repos/octocat/Hello-World/contents/PULL_REQUEST_TEMPLATE",
						"html_url": "https://github.com/octocat/Hello-World/blob/master/PULL_REQUEST_TEMPLATE"
					},
					"license": {
						"name": "MIT License",
						"key": "mit",
						"spdx_id": "MIT",
						"url": "https://api.github.com/licenses/mit",
						"html_url": "https://github.com/octocat/Hello-World/blob/master/LICENSE",
						"node_id": "MDc6TGljZW5zZW1pdA=="
					},
					"readme": {
						"url": "https://api.github.com/repos/octocat/Hello-World/contents/README.md",
						"html_url": "https://github.com/octocat/Hello-World/blob/master/README.md"
					}
				},
				"updated_at": `+referenceTimeStr+`,
				"content_reports_enabled": true
			}`)
	})

	ctx := t.Context()
	got, _, err := client.Repositories.GetCommunityHealthMetrics(ctx, "o", "r")
	if err != nil {
		t.Errorf("Repositories.GetCommunityHealthMetrics returned error: %v", err)
	}

	want := &CommunityHealthMetrics{
		HealthPercentage:      Ptr(100),
		Description:           Ptr("My first repository on GitHub!"),
		UpdatedAt:             &referenceTimestamp,
		ContentReportsEnabled: Ptr(true),
		Files: &CommunityHealthFiles{
			CodeOfConduct: &Metric{
				Name:    Ptr("Contributor Covenant"),
				Key:     Ptr("contributor_covenant"),
				HTMLURL: Ptr("https://github.com/octocat/Hello-World/blob/master/CODE_OF_CONDUCT.md"),
			},
			CodeOfConductFile: &Metric{
				URL:     Ptr("https://api.github.com/repos/octocat/Hello-World/contents/CODE_OF_CONDUCT.md"),
				HTMLURL: Ptr("https://github.com/octocat/Hello-World/blob/master/CODE_OF_CONDUCT.md"),
			},
			Contributing: &Metric{
				URL:     Ptr("https://api.github.com/repos/octocat/Hello-World/contents/CONTRIBUTING"),
				HTMLURL: Ptr("https://github.com/octocat/Hello-World/blob/master/CONTRIBUTING"),
			},
			IssueTemplate: &Metric{
				URL:     Ptr("https://api.github.com/repos/octocat/Hello-World/contents/ISSUE_TEMPLATE"),
				HTMLURL: Ptr("https://github.com/octocat/Hello-World/blob/master/ISSUE_TEMPLATE"),
			},
			PullRequestTemplate: &Metric{
				URL:     Ptr("https://api.github.com/repos/octocat/Hello-World/contents/PULL_REQUEST_TEMPLATE"),
				HTMLURL: Ptr("https://github.com/octocat/Hello-World/blob/master/PULL_REQUEST_TEMPLATE"),
			},
			License: &Metric{
				Name:    Ptr("MIT License"),
				Key:     Ptr("mit"),
				SPDXID:  Ptr("MIT"),
				URL:     Ptr("https://api.github.com/licenses/mit"),
				HTMLURL: Ptr("https://github.com/octocat/Hello-World/blob/master/LICENSE"),
				NodeID:  Ptr("MDc6TGljZW5zZW1pdA=="),
			},
			Readme: &Metric{
				URL:     Ptr("https://api.github.com/repos/octocat/Hello-World/contents/README.md"),
				HTMLURL: Ptr("https://github.com/octocat/Hello-World/blob/master/README.md"),
			},
		},
	}
	if !cmp.Equal(got, want) {
		t.Errorf("Repositories.GetCommunityHealthMetrics:\ngot:\n%v\nwant:\n%v", Stringify(got), Stringify(want))
	}

	const methodName = "GetCommunityHealthMetrics"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Repositories.GetCommunityHealthMetrics(ctx, "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Repositories.GetCommunityHealthMetrics(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
