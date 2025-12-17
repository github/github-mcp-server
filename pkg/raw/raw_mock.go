package raw

import "github.com/migueleliasweb/go-github-mock/src/mock"

var GetRawReposContentsByOwnerByRepoByPath mock.EndpointPattern = mock.EndpointPattern{
	Pattern: "/{owner}/{repo}/HEAD/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoByBranchByPath mock.EndpointPattern = mock.EndpointPattern{
	Pattern: "/{owner}/{repo}/refs/heads/{branch}/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoByTagByPath mock.EndpointPattern = mock.EndpointPattern{
	Pattern: "/{owner}/{repo}/refs/tags/{tag}/{path:.*}",
	Method:  "GET",
}
var GetRawReposContentsByOwnerByRepoBySHAByPath mock.EndpointPattern = mock.EndpointPattern{
	Pattern: "/{owner}/{repo}/{sha}/{path:.*}",
	Method:  "GET",
}
