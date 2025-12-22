package raw

const (
	GetRawReposContentsByOwnerByRepoByPath         = "GET /{owner}/{repo}/HEAD/{path:.*}"
	GetRawReposContentsByOwnerByRepoByBranchByPath = "GET /{owner}/{repo}/refs/heads/{branch}/{path:.*}"
	GetRawReposContentsByOwnerByRepoByTagByPath    = "GET /{owner}/{repo}/refs/tags/{tag}/{path:.*}"
	GetRawReposContentsByOwnerByRepoBySHAByPath    = "GET /{owner}/{repo}/{sha}/{path:.*}"
)
