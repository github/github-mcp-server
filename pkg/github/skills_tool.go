package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListRepoSkills exposes the per-repo Agent Skills discovery (`discoverSkills`)
// as an MCP tool the model can call directly. Bridges the autonomous-agent
// gap left by `completion/complete`, which is a client-UI feature only.
//
// The output URLs are constructed via SkillFileURI so they're guaranteed to
// match the per-file resource template registered in GetSkillResourceFile —
// the model can hand each URL straight to `resources/read`.
func ListRepoSkills(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSkills,
		mcp.Tool{
			Name: "list_repo_skills",
			Description: t("TOOL_LIST_REPO_SKILLS_DESCRIPTION",
				"List Agent Skills (SKILL.md files) defined in a GitHub repository. "+
					"Returns each discovered skill's name plus a `skill://` URI you can pass "+
					"directly to `resources/read` to fetch its SKILL.md. Recognizes the "+
					"agentskills.io directory conventions: skills/*/SKILL.md, "+
					"skills/{namespace}/*/SKILL.md, plugins/*/skills/*/SKILL.md, and "+
					"root-level */SKILL.md. Use this when you need to discover what skills "+
					"a repository exposes before reading any of them."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_REPO_SKILLS_TITLE", "List Agent Skills in a repository"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name).",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name.",
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			names, err := discoverSkills(ctx, client, owner, repo)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			type skillEntry struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			}
			entries := make([]skillEntry, 0, len(names))
			for _, name := range names {
				entries = append(entries, skillEntry{
					Name: name,
					URL:  SkillFileURI(owner, repo, name, "SKILL.md"),
				})
			}

			response := map[string]any{
				"owner":      owner,
				"repo":       repo,
				"skills":     entries,
				"totalCount": len(entries),
			}
			out, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal skill list: %w", err)
			}
			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}
