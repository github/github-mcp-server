package github

import (
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
)

// AllResources returns all resource templates with their embedded toolset metadata.
// Resource template functions return ServerResourceTemplate directly with toolset info.
func AllResources(t translations.TranslationHelperFunc, getClient GetClientFn, getRawClient raw.GetRawClientFn) []toolsets.ServerResourceTemplate {
	return []toolsets.ServerResourceTemplate{
		// Repository resources
		GetRepositoryResourceContent(getClient, getRawClient, t),
		GetRepositoryResourceBranchContent(getClient, getRawClient, t),
		GetRepositoryResourceCommitContent(getClient, getRawClient, t),
		GetRepositoryResourceTagContent(getClient, getRawClient, t),
		GetRepositoryResourcePrContent(getClient, getRawClient, t),
	}
}
