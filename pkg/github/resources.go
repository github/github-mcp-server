package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// AllResources 返回 所有资源 templates with their embedded 工具集 元数据.
// Resource definitions are stateless - 处理器s are generated on-dem和during registration.
func AllResources(t translations.TranslationHelperFunc) []inventory.ServerResourceTemplate {
	return []inventory.ServerResourceTemplate{
		// Repository 资源
		GetRepositoryResourceContent(t),
		GetRepositoryResourceBranchContent(t),
		GetRepositoryResourceCommitContent(t),
		GetRepositoryResourceTagContent(t),
		GetRepositoryResourcePrContent(t),
	}
}
