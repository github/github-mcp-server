package github

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"
)

// skillResourceFileURITemplate is the single SEP-2640-aligned template for
// reading any file inside a discovered Agent Skill in any GitHub repository.
//
// `{+file_path}` is RFC 6570 reserved expansion: it allows `/` and other
// reserved characters, so a multi-segment relative path inside the skill
// directory (e.g. `references/GUIDE.md`) round-trips through the template
// as a single value.
//
// SEP-2640 says relative paths inside SKILL.md (e.g. `references/GUIDE.md`
// in the body) MUST resolve to `skill://<skill-path>/<file-path>` resources.
// This template is what makes that resolution work for repo-discovered skills.
//
// The canonical discovery URL we publish in `skill://index.json` uses the
// SKILL.md anchor (`skill://{owner}/{repo}/{skill_name}/SKILL.md`) so hosts
// know where to start; per-file reads then follow naturally by extending
// the URI suffix.
var skillResourceFileURITemplate = uritemplate.MustNew("skill://{owner}/{repo}/{skill_name}/{+file_path}")

// SkillResourceDiscoveryURL is the URL string we advertise in the discovery
// index for the per-repo skill template (the SKILL.md anchor — what hosts
// fill in placeholders against to pull SKILL.md). Per-file reads follow by
// extending the trailing path segment.
const SkillResourceDiscoveryURL = "skill://{owner}/{repo}/{skill_name}/SKILL.md"

// SkillFileURI returns the canonical skill:// URI for a file inside a
// discovered repo-hosted Agent Skill. The shape MUST match the per-file
// resource template registered by GetSkillResourceFile so the URIs handed
// out by callers (e.g. the list_repo_skills tool) are routable back through
// `resources/read`.
func SkillFileURI(owner, repo, skillName, filePath string) string {
	return fmt.Sprintf("skill://%s/%s/%s/%s", owner, repo, skillName, filePath)
}

// GetSkillResourceFile returns the resource template registration for the
// SEP-aligned per-file skill resource. Reads any file inside any discovered
// skill directory in any GitHub repository.
func GetSkillResourceFile(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataSkills,
		mcp.ResourceTemplate{
			Name:        "skill_file",
			URITemplate: skillResourceFileURITemplate.Raw(),
			Description: t("RESOURCE_SKILL_FILE_DESCRIPTION", "A file inside an Agent Skill in a GitHub repository (SKILL.md or any relative reference). Path is the file's location relative to the skill directory."),
			Icons:       octicons.Icons("light-bulb"),
		},
		skillResourceFileHandlerFunc(skillResourceFileURITemplate),
	)
}

func skillResourceFileHandlerFunc(tmpl *uritemplate.Template) inventory.ResourceHandlerFunc {
	return func(_ any) mcp.ResourceHandler {
		return skillFileHandler(tmpl)
	}
}

// skillFileHandler returns a handler that fetches any file inside a
// discovered skill directory. SKILL.md and arbitrary relative paths
// (e.g. `references/GUIDE.md`) flow through the same code path — there
// is no SEP-defined manifest endpoint, since per-file resolution is the
// SEP's answer to multi-file skill discovery.
func skillFileHandler(tmpl *uritemplate.Template) mcp.ResourceHandler {
	return func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		deps := MustDepsFromContext(ctx)
		owner, repo, skillName, filePath, err := parseSkillFileURI(tmpl, request.Params.URI)
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

		fullPath := path.Join(skill.Dir, filePath)
		fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, fullPath, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s: %w", filePath, err)
		}

		content, err := fileContent.GetContent()
		if err != nil {
			return nil, fmt.Errorf("failed to decode %s content: %w", filePath, err)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      request.Params.URI,
					MIMEType: skillFileMIMEType(filePath),
					Text:     content,
				},
			},
		}, nil
	}
}

// skillFileMIMEType picks a content type for a skill file. SKILL.md and
// other .md files are always text/markdown (the agentskills convention,
// independent of any OS-level MIME registry). For other extensions we
// infer from the system MIME table, falling back to text/plain.
func skillFileMIMEType(filePath string) string {
	if strings.EqualFold(path.Ext(filePath), ".md") {
		return "text/markdown"
	}
	if mt := mime.TypeByExtension(path.Ext(filePath)); mt != "" {
		return mt
	}
	return "text/plain"
}

// parseSkillFileURI extracts owner, repo, skill_name, and the relative
// file path from a `skill://` URI matching the per-file template.
func parseSkillFileURI(tmpl *uritemplate.Template, uri string) (owner, repo, skillName, filePath string, err error) {
	values := tmpl.Match(uri)
	if values == nil {
		return "", "", "", "", fmt.Errorf("failed to match skill URI: %s", uri)
	}

	owner = values.Get("owner").String()
	repo = values.Get("repo").String()
	skillName = values.Get("skill_name").String()
	filePath = values.Get("file_path").String()

	if owner == "" {
		return "", "", "", "", errors.New("owner is required")
	}
	if repo == "" {
		return "", "", "", "", errors.New("repo is required")
	}
	if skillName == "" {
		return "", "", "", "", errors.New("skill_name is required")
	}
	if filePath == "" {
		return "", "", "", "", errors.New("file_path is required")
	}
	// Reject path traversal — file_path is supposed to be relative to the
	// skill dir and stay inside it.
	if strings.Contains(filePath, "..") {
		return "", "", "", "", fmt.Errorf("file_path must not contain ..: %s", filePath)
	}
	if strings.HasPrefix(filePath, "/") {
		return "", "", "", "", fmt.Errorf("file_path must be relative: %s", filePath)
	}

	return owner, repo, skillName, filePath, nil
}

// discoveredSkill holds a matched skill's name and directory path.
type discoveredSkill struct {
	Name string
	Dir  string
}

// matchSkillConventions checks if a blob path matches any known skill
// directory convention. Aligned with the agentskills.io spec and common
// community conventions:
//
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

// discoverSkills finds all skill directories in a repository by scanning the
// tree for SKILL.md files matching known directory conventions.
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

		// Reuse existing owner/repo resolvers from the repo:// resource family
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
			return skillCompletionResult(values), nil

		case "repo":
			client, err := getClient(ctx)
			if err != nil {
				return nil, err
			}
			values, err := completeRepo(ctx, client, resolved, argValue)
			if err != nil {
				return nil, err
			}
			return skillCompletionResult(values), nil

		case "skill_name":
			return completeSkillName(ctx, getClient, resolved, argValue)

		case "file_path":
			// file_path is open-ended within the skill directory; SKILL.md
			// is always present, so suggest it as a default. Listing every
			// file would require a tree fetch per keystroke — too costly.
			return skillCompletionResult([]string{"SKILL.md"}), nil

		default:
			return nil, fmt.Errorf("no resolver for skill argument: %s", argName)
		}
	}
}

func completeSkillName(ctx context.Context, getClient GetClientFn, resolved map[string]string, argValue string) (*mcp.CompleteResult, error) {
	owner := resolved["owner"]
	repo := resolved["repo"]
	if owner == "" || repo == "" {
		return skillCompletionResult(nil), nil
	}

	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}

	skills, err := discoverSkills(ctx, client, owner, repo)
	if err != nil {
		return skillCompletionResult(nil), nil //nolint:nilerr // graceful degradation
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

	return skillCompletionResult(skills), nil
}

func skillCompletionResult(values []string) *mcp.CompleteResult {
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
