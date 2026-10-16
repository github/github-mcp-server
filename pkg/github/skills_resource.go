package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"
)

var (
	skillResourceContentURITemplate  = uritemplate.MustNew("skill://{owner}/{repo}/{skill_name}/SKILL.md")
	skillResourceManifestURITemplate = uritemplate.MustNew("skill://{owner}/{repo}/{skill_name}/_manifest")
)

// GetSkillResourceContent defines the resource template for reading a skill's SKILL.md.
func GetSkillResourceContent(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataSkills,
		mcp.ResourceTemplate{
			Name:        "skill_content",
			URITemplate: skillResourceContentURITemplate.Raw(),
			Description: t("RESOURCE_SKILL_CONTENT_DESCRIPTION", "Agent Skill instructions (SKILL.md) from a GitHub repository"),
			Icons:       octicons.Icons("light-bulb"),
		},
		skillResourceContentHandlerFunc(skillResourceContentURITemplate),
	)
}

// GetSkillResourceManifest defines the resource template for a skill's file manifest.
func GetSkillResourceManifest(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataSkills,
		mcp.ResourceTemplate{
			Name:        "skill_manifest",
			URITemplate: skillResourceManifestURITemplate.Raw(),
			Description: t("RESOURCE_SKILL_MANIFEST_DESCRIPTION", "File manifest for an Agent Skill in a GitHub repository"),
			Icons:       octicons.Icons("light-bulb"),
		},
		skillResourceManifestHandlerFunc(skillResourceManifestURITemplate),
	)
}

func skillResourceContentHandlerFunc(tmpl *uritemplate.Template) inventory.ResourceHandlerFunc {
	return func(_ any) mcp.ResourceHandler {
		return skillContentHandler(tmpl)
	}
}

func skillResourceManifestHandlerFunc(tmpl *uritemplate.Template) inventory.ResourceHandlerFunc {
	return func(_ any) mcp.ResourceHandler {
		return skillManifestHandler(tmpl)
	}
}

// skillContentHandler returns a handler that fetches a skill's SKILL.md content.
func skillContentHandler(tmpl *uritemplate.Template) mcp.ResourceHandler {
	return func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		deps := MustDepsFromContext(ctx)
		owner, repo, skillName, err := parseSkillURI(tmpl, request.Params.URI)
		if err != nil {
			return nil, err
		}

		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}

		skill, err := findSkill(ctx, client, owner, repo, skillName)
		if err != nil {
			return nil, err
		}

		skillMDPath := path.Join(skill.Dir, "SKILL.md")
		fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, skillMDPath, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get SKILL.md: %w", err)
		}

		content, err := fileContent.GetContent()
		if err != nil {
			return nil, fmt.Errorf("failed to decode SKILL.md content: %w", err)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      request.Params.URI,
					MIMEType: "text/markdown",
					Text:     content,
				},
			},
		}, nil
	}
}

// SkillManifestEntry represents a single file in a skill's manifest.
type SkillManifestEntry struct {
	Path string `json:"path"`
	URI  string `json:"uri"`
	Size int    `json:"size"`
}

// SkillManifest represents the file listing for a skill directory.
type SkillManifest struct {
	Skill string               `json:"skill"`
	Files []SkillManifestEntry `json:"files"`
}

// skillManifestHandler returns a handler that lists files in a skill directory.
func skillManifestHandler(tmpl *uritemplate.Template) mcp.ResourceHandler {
	return func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		deps := MustDepsFromContext(ctx)
		owner, repo, skillName, err := parseSkillURI(tmpl, request.Params.URI)
		if err != nil {
			return nil, err
		}

		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}

		skill, err := findSkill(ctx, client, owner, repo, skillName)
		if err != nil {
			return nil, err
		}

		// Use recursive tree from repo root and filter to the skill directory
		tree, _, err := client.Git.GetTree(ctx, owner, repo, "HEAD", true)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository tree: %w", err)
		}

		prefix := skill.Dir + "/"
		manifest := SkillManifest{
			Skill: skillName,
			Files: make([]SkillManifestEntry, 0),
		}
		for _, entry := range tree.Entries {
			if entry.GetType() != "blob" {
				continue
			}
			entryPath := entry.GetPath()
			if !strings.HasPrefix(entryPath, prefix) {
				continue
			}
			relativePath := strings.TrimPrefix(entryPath, prefix)
			pathParts := strings.Split(entryPath, "/")
			repoURI, err := expandRepoResourceURI(owner, repo, "", "", pathParts)
			if err != nil {
				continue
			}
			manifest.Files = append(manifest.Files, SkillManifestEntry{
				Path: relativePath,
				URI:  repoURI,
				Size: entry.GetSize(),
			})
		}

		data, err := json.Marshal(manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal manifest: %w", err)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      request.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	}
}

// parseSkillURI extracts owner, repo, and skill_name from a skill:// URI.
func parseSkillURI(tmpl *uritemplate.Template, uri string) (owner, repo, skillName string, err error) {
	values := tmpl.Match(uri)
	if values == nil {
		return "", "", "", fmt.Errorf("failed to match skill URI: %s", uri)
	}

	owner = values.Get("owner").String()
	repo = values.Get("repo").String()
	skillName = values.Get("skill_name").String()

	if owner == "" {
		return "", "", "", errors.New("owner is required")
	}
	if repo == "" {
		return "", "", "", errors.New("repo is required")
	}
	if skillName == "" {
		return "", "", "", errors.New("skill_name is required")
	}

	return owner, repo, skillName, nil
}

// discoveredSkill holds a matched skill's name and directory path.
type discoveredSkill struct {
	Name string
	Dir  string
}

// matchSkillConventions checks if a blob path matches any known skill directory convention.
// Aligned with the agentskills.io spec and common community conventions:
//   - skills/*/SKILL.md                (agentskills.io spec)
//   - skills/{namespace}/*/SKILL.md    (namespaced skills)
//   - plugins/*/skills/*/SKILL.md      (plugin marketplace convention)
//   - */SKILL.md                       (root-level skill directories)
func matchSkillConventions(entryPath string) *discoveredSkill {
	if path.Base(entryPath) != "SKILL.md" {
		return nil
	}

	dir := path.Dir(entryPath)
	parentDir := path.Dir(dir)
	skillName := path.Base(dir)

	if skillName == "." || skillName == "" {
		return nil
	}

	// Convention 1: skills/*/SKILL.md
	if parentDir == "skills" {
		return &discoveredSkill{Name: skillName, Dir: dir}
	}

	// Convention 2: skills/{namespace}/*/SKILL.md
	grandparentDir := path.Dir(parentDir)
	if grandparentDir == "skills" {
		return &discoveredSkill{Name: skillName, Dir: dir}
	}

	// Convention 3: plugins/*/skills/*/SKILL.md
	if path.Base(parentDir) == "skills" && path.Dir(grandparentDir) == "plugins" {
		return &discoveredSkill{Name: skillName, Dir: dir}
	}

	// Convention 4: */SKILL.md (root-level skill directories)
	// Exclude convention prefixes and hidden directories.
	if parentDir == "." && skillName != "skills" && skillName != "plugins" && !strings.HasPrefix(skillName, ".") {
		return &discoveredSkill{Name: skillName, Dir: dir}
	}

	return nil
}

// findSkill locates a named skill within a repository by scanning the tree.
func findSkill(ctx context.Context, client *gogithub.Client, owner, repo, skillName string) (*discoveredSkill, error) {
	tree, _, err := client.Git.GetTree(ctx, owner, repo, "HEAD", true)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository tree: %w", err)
	}

	for _, entry := range tree.Entries {
		if entry.GetType() != "blob" {
			continue
		}
		skill := matchSkillConventions(entry.GetPath())
		if skill != nil && skill.Name == skillName {
			return skill, nil
		}
	}

	return nil, fmt.Errorf("skill %q not found in repository %s/%s", skillName, owner, repo)
}

// discoverSkills finds all skill directories in a repository by scanning the tree
// for SKILL.md files matching known directory conventions.
func discoverSkills(ctx context.Context, client *gogithub.Client, owner, repo string) ([]string, error) {
	tree, _, err := client.Git.GetTree(ctx, owner, repo, "HEAD", true)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository tree: %w", err)
	}

	seen := make(map[string]bool)
	var skills []string

	for _, entry := range tree.Entries {
		if entry.GetType() != "blob" {
			continue
		}
		skill := matchSkillConventions(entry.GetPath())
		if skill == nil {
			continue
		}
		if !seen[skill.Name] {
			seen[skill.Name] = true
			skills = append(skills, skill.Name)
		}
	}

	return skills, nil
}

// SkillResourceCompletionHandler handles completions for skill:// resource URIs.
func SkillResourceCompletionHandler(getClient GetClientFn) func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		argName := req.Params.Argument.Name
		argValue := req.Params.Argument.Value
		var resolved map[string]string
		if req.Params.Context != nil && req.Params.Context.Arguments != nil {
			resolved = req.Params.Context.Arguments
		} else {
			resolved = map[string]string{}
		}

		// Reuse existing owner/repo resolvers
		switch argName {
		case "owner":
			client, err := getClient(ctx)
			if err != nil {
				return nil, err
			}
			values, err := completeOwner(ctx, client, resolved, argValue)
			if err != nil {
				return nil, err
			}
			return completionResult(values), nil

		case "repo":
			client, err := getClient(ctx)
			if err != nil {
				return nil, err
			}
			values, err := completeRepo(ctx, client, resolved, argValue)
			if err != nil {
				return nil, err
			}
			return completionResult(values), nil

		case "skill_name":
			return completeSkillName(ctx, getClient, resolved, argValue)

		default:
			return nil, fmt.Errorf("no resolver for skill argument: %s", argName)
		}
	}
}

func completeSkillName(ctx context.Context, getClient GetClientFn, resolved map[string]string, argValue string) (*mcp.CompleteResult, error) {
	owner := resolved["owner"]
	repo := resolved["repo"]
	if owner == "" || repo == "" {
		return completionResult(nil), nil
	}

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	skills, err := discoverSkills(ctx, client, owner, repo)
	if err != nil {
		return completionResult(nil), nil //nolint:nilerr // graceful degradation
	}

	if argValue != "" {
		var filtered []string
		for _, s := range skills {
			if strings.HasPrefix(s, argValue) {
				filtered = append(filtered, s)
			}
		}
		skills = filtered
	}

	return completionResult(skills), nil
}

func completionResult(values []string) *mcp.CompleteResult {
	if len(values) > 100 {
		values = values[:100]
	}
	return &mcp.CompleteResult{
		Completion: mcp.CompletionResultDetails{
			Values:  values,
			Total:   len(values),
			HasMore: false,
		},
	}
}
