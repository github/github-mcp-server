package github

import (
	"github.com/github/github-mcp-server/pkg/registry"
	"github.com/github/github-mcp-server/pkg/translations"
)

// AllResources returns all resource templates with their embedded toolset metadata.
// Resource definitions are stateless - handlers are generated on-demand during registration.
func AllResources(t translations.TranslationHelperFunc) []registry.ServerResourceTemplate {
	return []registry.ServerResourceTemplate{
		// Repository resources
		GetRepositoryResourceContent(t),
		GetRepositoryResourceBranchContent(t),
		GetRepositoryResourceCommitContent(t),
		GetRepositoryResourceTagContent(t),
		GetRepositoryResourcePrContent(t),
	}
}
