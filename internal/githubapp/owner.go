package githubapp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// maxGraphQLBodyPeek caps how much of a GraphQL request body is buffered while
// looking for the owner variable. Queries are small; a body larger than this is
// not worth inspecting.
const maxGraphQLBodyPeek = 1 << 20

// graphQLOwnerVariables are the GraphQL variable names that carry an account
// login, in priority order.
var graphQLOwnerVariables = []string{"owner", "org", "organization", "login", "repositoryOwner"}

// OwnerFromRequest reports the account login that owns the resource an outbound
// GitHub API request addresses, or "" when it cannot be determined.
//
// REST paths are read directly: /repos/{owner}/..., /orgs/{org}/... and
// /users/{user}/... all name their owner, and the leading segments of GitHub
// Enterprise Server paths (/api/v3/...) are skipped by scanning for the first
// segment that introduces an owner. GraphQL requests carry the owner in their
// variables instead, so the body is inspected for one of the conventional
// variable names.
//
// Endpoints that are not owner-scoped (/user, /rate_limit, /repositories/{id})
// yield "". So does a GraphQL query that names no owner.
func OwnerFromRequest(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	if owner := ownerFromPath(req.URL.Path); owner != "" {
		return owner
	}
	if isGraphQL(req.URL.Path) {
		return ownerFromGraphQLBody(req)
	}
	return ""
}

func ownerFromPath(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for i, segment := range segments {
		switch segment {
		case "repos", "orgs", "users":
			if i+1 < len(segments) && segments[i+1] != "" {
				return segments[i+1]
			}
			return ""
		}
	}
	return ""
}

func isGraphQL(path string) bool {
	trimmed := strings.Trim(path, "/")
	return trimmed == "graphql" || strings.HasSuffix(trimmed, "/graphql")
}

// ownerFromGraphQLBody reads the request body through GetBody so the original
// body stays intact for the transport below. Requests without GetBody (a
// streamed body) are skipped rather than consumed.
func ownerFromGraphQLBody(req *http.Request) string {
	if req.GetBody == nil {
		return ""
	}
	body, err := req.GetBody()
	if err != nil || body == nil {
		return ""
	}
	defer func() { _ = body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(body, maxGraphQLBodyPeek))
	if err != nil {
		return ""
	}

	var payload struct {
		Variables map[string]json.RawMessage `json:"variables"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	for _, name := range graphQLOwnerVariables {
		value, ok := payload.Variables[name]
		if !ok {
			continue
		}
		var login string
		if err := json.Unmarshal(value, &login); err == nil && login != "" {
			return login
		}
	}
	return ""
}
