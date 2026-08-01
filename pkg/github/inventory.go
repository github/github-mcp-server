package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// NewInventory 创建s 一个Inventory with 所有available 工具, 资源, 和提示.
// Tools, 资源, 和提示 are self-describing with their 工具集 元数据 embedded.
// 此函数 is stateless - no dependencies are captured.
// Handlers are generated on-dem和during registration via RegisterAll(ctx, 服务器, deps).
// "default" keyword in WithToolsets will exp和to 工具集s marked with Default: 真.
func NewInventory(t translations.TranslationHelperFunc) *inventory.Builder {
	return inventory.NewBuilder().
		SetTools(AllTools(t)).
		SetResources(AllResources(t)).
		SetPrompts(AllPrompts(t))
}
