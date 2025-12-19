package raw

// EndpointPattern represents an HTTP method and URL path pattern for mock matching.
type EndpointPattern struct {
	Pattern string
	Method  string
}

var GetRawReposContentsByOwnerByRepoByPath = EndpointPattern{
	Pattern: "/{owner}/{repo}/HEAD/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoByBranchByPath = EndpointPattern{
	Pattern: "/{owner}/{repo}/refs/heads/{branch}/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoByTagByPath = EndpointPattern{
	Pattern: "/{owner}/{repo}/refs/tags/{tag}/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoBySHAByPath = EndpointPattern{
	Pattern: "/{owner}/{repo}/{sha}/{path:.*}",
	Method:  "GET",
}
