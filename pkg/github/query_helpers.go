package github

import (
	"regexp"
)

var (
	repoQualifierPattern      = regexp.MustCompile(`(?i)repo:([^/\s]+)/([^/\s]+)`)
	orgOrUserQualifierPattern = regexp.MustCompile(`(?i)(?:org|user):([^\s]+)`)
)

// repoQueryInfo holds extracted repository information from a query
type repoQueryInfo struct {
	owner string
	repo  string
}

// extractAllReposFromQuery returns all "repo:owner/repo" qualifiers found in the query.
// Used by denylist enforcement to prevent bypass via multiple repo: qualifiers
// (e.g. "repo:allowed/repo repo:denied/repo").
func extractAllReposFromQuery(query string) []repoQueryInfo {
	matches := repoQualifierPattern.FindAllStringSubmatch(query, -1)
	results := make([]repoQueryInfo, 0, len(matches))
	for _, m := range matches {
		if len(m) == 3 {
			results = append(results, repoQueryInfo{owner: m[1], repo: m[2]})
		}
	}
	return results
}

// extractOrgFromQuery attempts to extract an organization or user name from a search query.
// It looks for patterns like "org:squareup" or "user:octocat".
func extractOrgFromQuery(query string) string {
	matches := orgOrUserQualifierPattern.FindStringSubmatch(query)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}
