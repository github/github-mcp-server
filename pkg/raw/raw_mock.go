package raw

import "github.com/github/github-mcp-server/pkg/testmock"

var GetRawReposContentsByOwnerByRepoByPath = testmock.EndpointPattern{
	Pattern: "/{owner}/{repo}/HEAD/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoByBranchByPath = testmock.EndpointPattern{
	Pattern: "/{owner}/{repo}/refs/heads/{branch}/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoByTagByPath = testmock.EndpointPattern{
	Pattern: "/{owner}/{repo}/refs/tags/{tag}/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoBySHAByPath = testmock.EndpointPattern{
	Pattern: "/{owner}/{repo}/{sha}/{path:.*}",
	Method:  "GET",
}
