package github

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// URLType identifies what kind of GitHub resource a URL points to.
type URLType int

const (
	URLTypeRepo  URLType = iota // https://{host}/{owner}/{repo}
	URLTypeIssue                // https://{host}/{owner}/{repo}/issues/{number}
	URLTypePR                   // https://{host}/{owner}/{repo}/pull/{number}
	URLTypeFile                 // https://{host}/{owner}/{repo}/blob/{ref}/{path...}
)

// ParsedGitHubURL holds the components extracted from a GitHub resource URL.
type ParsedGitHubURL struct {
	Owner  string
	Repo   string
	Type   URLType
	Number int    // issue or PR number; 0 for repo and file URLs
	Ref    string // branch, tag, or SHA; populated for file URLs
	Path   string // file path within the repo; populated for file URLs
}

// ParseGitHubURL parses a GitHub resource URL and returns its components.
// It supports repo, issue, PR, and file blob URLs across github.com and
// self-hosted GitHub Enterprise instances.
//
// Supported forms:
//
//	https://{host}/{owner}/{repo}
//	https://{host}/{owner}/{repo}/issues/{number}
//	https://{host}/{owner}/{repo}/pull/{number}
//	https://{host}/{owner}/{repo}/blob/{ref}/{path...}
func ParseGitHubURL(rawURL string) (*ParsedGitHubURL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("unsupported URL scheme %q: expected http or https", u.Scheme)
	}

	// Strip leading/trailing slashes and split path segments
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")

	// Minimum: /{owner}/{repo}
	if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
		return nil, fmt.Errorf("URL must contain at least /{owner}/{repo}: %s", rawURL)
	}

	parsed := &ParsedGitHubURL{
		Owner: segments[0],
		Repo:  segments[1],
		Type:  URLTypeRepo,
	}

	// Nothing after /{owner}/{repo}
	if len(segments) == 2 {
		return parsed, nil
	}

	resourceType := segments[2]

	switch resourceType {
	case "issues":
		if len(segments) < 4 || segments[3] == "" {
			return nil, fmt.Errorf("issue URL missing number: %s", rawURL)
		}
		n, err := strconv.Atoi(segments[3])
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("issue URL has invalid number %q: %s", segments[3], rawURL)
		}
		parsed.Type = URLTypeIssue
		parsed.Number = n

	case "pull":
		if len(segments) < 4 || segments[3] == "" {
			return nil, fmt.Errorf("pull request URL missing number: %s", rawURL)
		}
		n, err := strconv.Atoi(segments[3])
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("pull request URL has invalid number %q: %s", segments[3], rawURL)
		}
		parsed.Type = URLTypePR
		parsed.Number = n

	case "blob":
		// /{owner}/{repo}/blob/{ref}/{path...}
		if len(segments) < 5 || segments[3] == "" || segments[4] == "" {
			return nil, fmt.Errorf("file URL must contain /{owner}/{repo}/blob/{ref}/{path}: %s", rawURL)
		}
		parsed.Type = URLTypeFile
		parsed.Ref = segments[3]
		parsed.Path = strings.Join(segments[4:], "/")

	default:
		return nil, fmt.Errorf("unsupported GitHub URL type %q in: %s", resourceType, rawURL)
	}

	return parsed, nil
}

// ApplyURLParam checks whether args contains a "url" key. If it does, the URL
// is parsed and the extracted fields (owner, repo, and optionally number/ref/path)
// are written into args — but only when the target key is not already set.
// This lets callers explicitly override individual fields even when providing a URL.
//
// Call this at the top of a tool handler before reading any other parameters.
func ApplyURLParam(args map[string]any) error {
	raw, ok := args["url"]
	if !ok {
		return nil
	}

	rawStr, ok := raw.(string)
	if !ok || rawStr == "" {
		return nil
	}

	parsed, err := ParseGitHubURL(rawStr)
	if err != nil {
		return fmt.Errorf("invalid GitHub URL: %w", err)
	}

	setIfAbsent(args, "owner", parsed.Owner)
	setIfAbsent(args, "repo", parsed.Repo)

	if parsed.Number > 0 {
		setIfAbsent(args, "number", float64(parsed.Number))
	}
	if parsed.Ref != "" {
		setIfAbsent(args, "ref", parsed.Ref)
	}
	if parsed.Path != "" {
		setIfAbsent(args, "path", parsed.Path)
	}

	return nil
}

// setIfAbsent writes value into args[key] only when the key is not already
// present or is set to a zero-value string/number.
func setIfAbsent(args map[string]any, key string, value any) {
	existing, exists := args[key]
	if !exists {
		args[key] = value
		return
	}
	// Treat empty string as absent
	if s, ok := existing.(string); ok && s == "" {
		args[key] = value
	}
}
