// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"
)

func TestMarkdownService_Markdown(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := &markdownRenderRequest{
		Text:    Ptr("# text #"),
		Mode:    Ptr("gfm"),
		Context: Ptr("google/go-github"),
	}
	mux.HandleFunc("/markdown", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `<h1>text</h1>`)
	})

	ctx := t.Context()
	md, _, err := client.Markdown.Render(ctx, "# text #", &MarkdownOptions{
		Mode:    "gfm",
		Context: "google/go-github",
	})
	if err != nil {
		t.Errorf("Render returned error: %v", err)
	}

	if want := "<h1>text</h1>"; want != md {
		t.Errorf("Render returned %+v, want %+v", md, want)
	}

	const methodName = "Render"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Markdown.Render(ctx, "# text #", &MarkdownOptions{
			Mode:    "gfm",
			Context: "google/go-github",
		})
		if got != "" {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}
