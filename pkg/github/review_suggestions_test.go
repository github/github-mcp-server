package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gogithub "github.com/google/go-github/v87/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeNodeDatabaseID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		nodeID  string
		want    int64
		wantErr bool
	}{
		{
			name:   "pull request review thread",
			nodeID: "PRRT_kwDORGz4i851Fgp1",
			want:   1964378741,
		},
		{
			name:   "pull request review thread with url-safe padding char",
			nodeID: "PRRT_kwDORGz4i851Fgo-",
			want:   1964378686,
		},
		{
			name:   "pull request review comment",
			nodeID: "PRRC_kwDORGz4i86v72Xc",
			want:   2951701980,
		},
		{
			name:    "invalid node id",
			nodeID:  "invalid",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeNodeDatabaseID(tc.nodeID)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseSuggestionsFromBody(t *testing.T) {
	t.Parallel()

	body := "Please update this.\n\n```suggestion\nimport pytest\n\npytest.importorskip(\"torch\")\n```\n"
	suggestions := parseSuggestionsFromBody(body)
	require.Len(t, suggestions, 1)
	assert.Equal(t, suggestionSourceBody, suggestions[0].Source)
	assert.Equal(t, "import pytest\n\npytest.importorskip(\"torch\")", suggestions[0].Suggestion)
}

func TestParseAutomatedSuggestionsFromHTML(t *testing.T) {
	t.Parallel()

	html := `<html><body>` + automatedSuggestionHTMLFixture + `</body></html>`
	suggestions, err := parseAutomatedSuggestionsFromHTML(html)
	require.NoError(t, err)
	require.Len(t, suggestions, 1)
	assert.Equal(t, suggestionSourceAutomated, suggestions[0].Source)
	assert.Equal(t, "glmocr/cli.py", suggestions[0].Path)
	assert.Contains(t, suggestions[0].Suggestion, "import re")
	require.NotNil(t, suggestions[0].StartLine)
	assert.Equal(t, 10, *suggestions[0].StartLine)
}

func TestParseAutomatedSuggestionsFromHTMLWithDeletions(t *testing.T) {
	t.Parallel()

	html := `<html><body>` + automatedSuggestionWithDeletionsFixture + `</body></html>`
	suggestions, err := parseAutomatedSuggestionsFromHTML(html)
	require.NoError(t, err)
	require.Len(t, suggestions, 1)

	s := suggestions[0]
	assert.Equal(t, suggestionSourceAutomated, s.Source)
	assert.Equal(t, "glmocr/tests/test_layout_device.py", s.Path)
	assert.NotContains(t, s.Suggestion, "from glmocr.layout.layout_detector import PPDocLayoutDetector")
	assert.Contains(t, s.Suggestion, "from glmocr import layout as layout_mod")
	assert.Contains(t, s.Suggestion, "pytest.skip")
	require.NotNil(t, s.StartLine)
	assert.Equal(t, 132, *s.StartLine)
}

func TestFetchAutomatedSuggestionsForThread(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/owner/repo/pull/42/threads/1964378741", r.URL.Path)
		assert.Equal(t, "rendering_on_files_tab=true", r.URL.RawQuery)
		_, _ = w.Write([]byte(automatedSuggestionHTMLFixture))
	}))
	defer server.Close()

	client, err := gogithub.NewClient(gogithub.WithHTTPClient(server.Client()), gogithub.WithEnterpriseURLs(server.URL+"/", server.URL+"/"))
	require.NoError(t, err)

	suggestions, err := fetchAutomatedSuggestionsForThread(
		context.Background(),
		client,
		"owner",
		"repo",
		42,
		"PRRT_kwDORGz4i851Fgp1",
	)
	require.NoError(t, err)
	require.Len(t, suggestions, 1)
	assert.Equal(t, "glmocr/cli.py", suggestions[0].Path)
}

func TestEnrichReviewThreadsWithSuggestions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(automatedSuggestionHTMLFixture))
	}))
	defer server.Close()

	client, err := gogithub.NewClient(gogithub.WithHTTPClient(server.Client()), gogithub.WithEnterpriseURLs(server.URL+"/", server.URL+"/"))
	require.NoError(t, err)

	threads := []MinimalReviewThread{
		{
			ID: "PRRT_kwDORGz4i851Fgp1",
			Comments: []MinimalReviewComment{
				{
					Body:   "Consider adding validation.\n```suggestion\nvalidated = True\n```",
					Author: "copilot-pull-request-reviewer",
					Path:   "glmocr/cli.py",
				},
			},
		},
	}

	enrichReviewThreadsWithSuggestions(context.Background(), client, "owner", "repo", 42, threads)

	require.Len(t, threads[0].Comments[0].Suggestions, 2)
	assert.Equal(t, suggestionSourceBody, threads[0].Comments[0].Suggestions[0].Source)
	assert.Equal(t, "validated = True", threads[0].Comments[0].Suggestions[0].Suggestion)
	assert.Equal(t, suggestionSourceAutomated, threads[0].Comments[0].Suggestions[1].Source)
	assert.Equal(t, "glmocr/cli.py", threads[0].Comments[0].Suggestions[1].Path)
}

const automatedSuggestionHTMLFixture = `<script type="application/json" data-target="react-partial.embeddedData">{"props":{"comment":{"automatedComment":{"suggestion":{"diffEntries":[{"path":"glmocr/cli.py","diffLines":[{"type":"HUNK","text":"@@ -9,6 +9,7 @@","left":8,"right":8},{"type":"CONTEXT","text":"from pathlib import Path","left":10,"right":10},{"type":"ADDITION","text":"import re","left":11,"right":12}]}]}}}}}</script>`

// Fixture derived from a real Copilot review thread partial (zai-org/GLM-OCR#131).
const automatedSuggestionWithDeletionsFixture = `<script type="application/json" data-target="react-partial.embeddedData">{"props":{"comment":{"automatedComment":{"suggestion":{"diffEntries":[{"path":"glmocr/tests/test_layout_device.py","diffLines":[{"type":"HUNK","text":"@@ -132,7 +132,11 @@","left":131,"right":131},{"type":"CONTEXT","text":"    def _mock_detector(self, device_val):","left":132,"right":132},{"type":"CONTEXT","text":"        from glmocr.config import LayoutConfig","left":134,"right":134},{"type":"DELETION","text":"        from glmocr.layout.layout_detector import PPDocLayoutDetector","left":135,"right":134},{"type":"ADDITION","text":"        try:","left":135,"right":135},{"type":"ADDITION","text":"            from glmocr import layout as layout_mod","left":135,"right":136},{"type":"ADDITION","text":"            PPDocLayoutDetector = layout_mod.PPDocLayoutDetector  # type: ignore[attr-defined]","left":135,"right":137},{"type":"ADDITION","text":"        except Exception:","left":135,"right":138},{"type":"ADDITION","text":"            pytest.skip(\"PPDocLayoutDetector (and optional layout deps) not available; skipping mocked detector tests.\")","left":135,"right":139},{"type":"CONTEXT","text":"        cfg = LayoutConfig(device=device_val, **self._MOCK_LAYOUT_KWARGS)","left":137,"right":141}]}]}}}}}</script>`
