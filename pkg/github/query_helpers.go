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

// extractAllOrgsFromQuery returns all "org:X" and "user:X" qualifiers found in
// the query. Used by denylist enforcement to prevent bypass via multiple
// qualifiers (e.g. "user:allowed org:denied").
func extractAllOrgsFromQuery(query string) []string {
	matches := orgOrUserQualifierPattern.FindAllStringSubmatch(query, -1)
	results := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) == 2 {
			results = append(results, m[1])
		}
	}
	return results
}
