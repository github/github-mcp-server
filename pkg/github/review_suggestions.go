package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	gogithub "github.com/google/go-github/v87/github"
)

const (
	suggestionSourceBody      = "body"
	suggestionSourceAutomated = "automated"
)

var suggestionBlockPattern = regexp.MustCompile("(?s)```suggestion\\s*\n(.*?)```")

type automatedDiffLine struct {
	Text  string `json:"text"`
	Type  string `json:"type"`
	Left  *int   `json:"left"`
	Right *int   `json:"right"`
}

type automatedDiffEntry struct {
	Path      string              `json:"path"`
	DiffLines []automatedDiffLine `json:"diffLines"`
}

type automatedSuggestionPayload struct {
	Props struct {
		Comment struct {
			AutomatedComment struct {
				Suggestion struct {
					DiffEntries []automatedDiffEntry `json:"diffEntries"`
				} `json:"suggestion"`
			} `json:"automatedComment"`
		} `json:"comment"`
	} `json:"props"`
}

// decodeNodeDatabaseID extracts the numeric database ID encoded in a GitHub GraphQL node ID.
func decodeNodeDatabaseID(nodeID string) (int64, error) {
	_, payload, ok := strings.Cut(nodeID, "_")
	if !ok || payload == "" {
		return 0, fmt.Errorf("invalid node ID: %q", nodeID)
	}

	padded := payload + strings.Repeat("=", (4-len(payload)%4)%4)
	raw, err := base64.RawURLEncoding.DecodeString(padded)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(padded)
		if err != nil {
			return 0, fmt.Errorf("decode node ID %q: %w", nodeID, err)
		}
	}

	if len(raw) < 4 {
		return 0, fmt.Errorf("node ID payload too short: %q", nodeID)
	}

	dbID := int64(raw[len(raw)-4])<<24 | int64(raw[len(raw)-3])<<16 | int64(raw[len(raw)-2])<<8 | int64(raw[len(raw)-1])
	return dbID, nil
}

func parseSuggestionsFromBody(body string) []MinimalReviewSuggestion {
	matches := suggestionBlockPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	suggestions := make([]MinimalReviewSuggestion, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		suggestions = append(suggestions, MinimalReviewSuggestion{
			Suggestion: strings.TrimRight(match[1], "\n"),
			Source:     suggestionSourceBody,
		})
	}
	return suggestions
}

func suggestionsFromAutomatedPayload(payload automatedSuggestionPayload) []MinimalReviewSuggestion {
	diffEntries := payload.Props.Comment.AutomatedComment.Suggestion.DiffEntries
	if len(diffEntries) == 0 {
		return nil
	}

	suggestions := make([]MinimalReviewSuggestion, 0, len(diffEntries))
	for _, entry := range diffEntries {
		suggestionText, startLine, endLine := buildSuggestionFromDiffLines(entry.DiffLines)
		if suggestionText == "" {
			continue
		}
		suggestions = append(suggestions, MinimalReviewSuggestion{
			Path:       entry.Path,
			Suggestion: suggestionText,
			StartLine:  startLine,
			EndLine:    endLine,
			Source:     suggestionSourceAutomated,
		})
	}
	return suggestions
}

func buildSuggestionFromDiffLines(lines []automatedDiffLine) (string, *int, *int) {
	var builder strings.Builder
	var startLine, endLine *int

	for _, line := range lines {
		switch line.Type {
		case "HUNK":
			continue
		case "ADDITION", "CONTEXT":
			if builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(line.Text)
			if line.Right != nil {
				if startLine == nil {
					startLine = line.Right
				}
				endLine = line.Right
			}
		}
	}

	if builder.Len() == 0 {
		return "", nil, nil
	}
	return builder.String(), startLine, endLine
}

func webBaseURLFromClient(client *gogithub.Client) (*url.URL, error) {
	if client == nil {
		return url.Parse("https://github.com")
	}

	apiURL, err := url.Parse(client.BaseURL())
	if err != nil || apiURL.Hostname() == "" {
		return url.Parse("https://github.com")
	}

	host := apiURL.Hostname()
	switch {
	case host == "api.github.com":
		return url.Parse("https://github.com")
	case strings.HasPrefix(host, "api."):
		webHost := strings.TrimPrefix(host, "api.")
		return url.Parse("https://" + webHost)
	default:
		webURL := *apiURL
		webURL.Path = strings.TrimSuffix(webURL.Path, "/api/v3/")
		webURL.Path = strings.TrimSuffix(webURL.Path, "/api/v3")
		webURL.Path = ""
		webURL.RawQuery = ""
		webURL.Fragment = ""
		return &webURL, nil
	}
}

func fetchAutomatedSuggestionsForThread(
	ctx context.Context,
	client *gogithub.Client,
	owner, repo string,
	pullNumber int,
	threadNodeID string,
) ([]MinimalReviewSuggestion, error) {
	threadDBID, err := decodeNodeDatabaseID(threadNodeID)
	if err != nil {
		return nil, err
	}

	webBase, err := webBaseURLFromClient(client)
	if err != nil {
		return nil, err
	}

	threadURL := fmt.Sprintf("%s/%s/%s/pull/%d/threads/%d?rendering_on_files_tab=true",
		strings.TrimRight(webBase.String(), "/"), owner, repo, pullNumber, threadDBID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, threadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")

	resp, err := client.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("thread partial request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	html, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}

	return parseAutomatedSuggestionsFromHTML(string(html))
}

func parseAutomatedSuggestionsFromHTML(html string) ([]MinimalReviewSuggestion, error) {
	const marker = `<script type="application/json" data-target="react-partial.embeddedData">`
	start := 0
	for {
		idx := strings.Index(html[start:], marker)
		if idx == -1 {
			break
		}
		idx += start
		contentStart := idx + len(marker)
		contentEnd := strings.Index(html[contentStart:], "</script>")
		if contentEnd == -1 {
			break
		}

		var payload automatedSuggestionPayload
		if err := json.Unmarshal([]byte(html[contentStart:contentStart+contentEnd]), &payload); err == nil {
			if suggestions := suggestionsFromAutomatedPayload(payload); len(suggestions) > 0 {
				return suggestions, nil
			}
		}

		start = contentStart + contentEnd
	}

	return nil, nil
}

func enrichReviewThreadsWithSuggestions(
	ctx context.Context,
	client *gogithub.Client,
	owner, repo string,
	pullNumber int,
	threads []MinimalReviewThread,
) {
	for i := range threads {
		thread := &threads[i]
		if len(thread.Comments) == 0 {
			continue
		}

		for j := range thread.Comments {
			if suggestions := parseSuggestionsFromBody(thread.Comments[j].Body); len(suggestions) > 0 {
				thread.Comments[j].Suggestions = append(thread.Comments[j].Suggestions, suggestions...)
			}
		}

		automatedSuggestions, err := fetchAutomatedSuggestionsForThread(ctx, client, owner, repo, pullNumber, thread.ID)
		if err != nil || len(automatedSuggestions) == 0 {
			continue
		}

		targetIdx := 0
		for j, comment := range thread.Comments {
			if strings.Contains(strings.ToLower(comment.Author), "copilot") {
				targetIdx = j
				break
			}
		}

		thread.Comments[targetIdx].Suggestions = append(thread.Comments[targetIdx].Suggestions, automatedSuggestions...)
	}
}
