package github

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/github/github-mcp-server/skills"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This file backs the skills extension's Dynamic* hooks for repo-hosted
// skills. Repo-hosted skills are the SEP's "unenumerable catalog" case:
// they never appear in skills/list (any GitHub repository may hold one),
// but a host holding a skill://{owner}/{repo}/{skill_name}/SKILL.md URI —
// from list_repo_skills, server instructions, or the user — can call
// skills/get on it and receive the same digested, content-bindable entry a
// listing would have carried.

// RepoSkillEntry resolves a repo-hosted skill URI to its SEP-2640 entry:
// frontmatter parsed from the skill's SKILL.md and a complete resources set
// with the digest of every file in the skill directory. It returns (nil, nil)
// when the URI does not identify a repo-hosted skill, letting the caller
// answer -32602.
func RepoSkillEntry(ctx context.Context, uri string) (*skills.Entry, error) {
	owner, repo, skillName, filePath, err := parseSkillFileURI(skillResourceFileURITemplate, uri)
	if err != nil || filePath != skills.SkillFile {
		// Not shaped like a repo-hosted skill's SKILL.md URI.
		return nil, nil //nolint:nilerr // absence, not failure — caller answers -32602
	}

	deps := MustDepsFromContext(ctx)
	client, err := deps.GetClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub client: %w", err)
	}

	dir, entries, err := repoSkillTree(ctx, client, owner, repo, skillName)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, nil // skill not found in the repository
	}

	// Fetch every file in the skill directory and digest its raw bytes.
	// Skills are small by design (a SKILL.md plus a handful of supporting
	// files), so one blob fetch per file is acceptable for skills/get,
	// which hosts call at approval/refresh time rather than per read.
	var files []skills.File
	for _, entry := range entries {
		if entry.GetType() != "blob" || !strings.HasPrefix(entry.GetPath(), dir+"/") {
			continue
		}
		content, _, err := client.Git.GetBlobRaw(ctx, owner, repo, entry.GetSHA())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s: %w", entry.GetPath(), err)
		}
		files = append(files, skills.File{
			Path:    strings.TrimPrefix(entry.GetPath(), dir+"/"),
			Content: content,
		})
	}

	s, err := skills.New(path.Join(owner, repo, skillName), files)
	if err != nil {
		// The directory exists but does not form a conforming skill (bad or
		// missing frontmatter, name mismatch). The server cannot serve it as
		// a skill; surface why under the SEP's -32602 contract.
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: fmt.Sprintf("not a conforming skill: %v", err),
		}
	}
	entry := s.Entry()
	return &entry, nil
}

// RepoSkillDirectory lists the direct children of a directory resource inside
// a repo-hosted skill namespace: skill://{owner}/{repo} enumerates the
// repository's discovered skills as directories, and deeper URIs walk a
// skill's own tree. It returns (nil, nil) when the URI is not a directory
// this server serves, letting the caller answer -32602.
func RepoSkillDirectory(ctx context.Context, uri string) ([]*mcp.Resource, error) {
	trimmed, ok := strings.CutPrefix(uri, "skill://")
	if !ok || strings.Contains(trimmed, "..") {
		return nil, nil
	}
	segs := strings.Split(trimmed, "/")
	if len(segs) < 2 || segs[0] == "" || segs[1] == "" {
		return nil, nil
	}
	owner, repo := segs[0], segs[1]

	deps := MustDepsFromContext(ctx)
	client, err := deps.GetClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub client: %w", err)
	}

	// skill://{owner}/{repo}: the repository's skill namespace root.
	if len(segs) == 2 {
		names, err := discoverSkills(ctx, client, owner, repo)
		if err != nil {
			return nil, err
		}
		children := make([]*mcp.Resource, 0, len(names))
		for _, name := range names {
			children = append(children, &mcp.Resource{
				URI:      "skill://" + owner + "/" + repo + "/" + name,
				Name:     name,
				MIMEType: skills.DirectoryMIMEType,
			})
		}
		return children, nil
	}

	skillName := segs[2]
	subPath := strings.Join(segs[3:], "/")

	dir, entries, err := repoSkillTree(ctx, client, owner, repo, skillName)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, nil
	}

	target := dir
	if subPath != "" {
		target = dir + "/" + subPath
	}

	var children []*mcp.Resource
	targetExists := subPath == "" // the skill root exists by discovery
	for _, entry := range entries {
		entryPath := entry.GetPath()
		if entryPath == target && entry.GetType() == "tree" {
			targetExists = true
		}
		if entryPath == target && entry.GetType() == "blob" {
			// A file, not a directory — not listable.
			return nil, nil
		}
		rel, ok := strings.CutPrefix(entryPath, target+"/")
		if !ok || strings.Contains(rel, "/") {
			continue // not a direct child
		}
		childRel := rel
		if subPath != "" {
			childRel = subPath + "/" + rel
		}
		switch entry.GetType() {
		case "tree":
			children = append(children, &mcp.Resource{
				URI:      SkillFileURI(owner, repo, skillName, childRel),
				Name:     rel,
				MIMEType: skills.DirectoryMIMEType,
			})
		case "blob":
			children = append(children, &mcp.Resource{
				URI:      SkillFileURI(owner, repo, skillName, childRel),
				Name:     rel,
				MIMEType: skills.FileMIMEType(rel),
			})
		}
	}
	if !targetExists {
		return nil, nil
	}
	slices.SortFunc(children, func(a, b *mcp.Resource) int { return strings.Compare(a.URI, b.URI) })
	return children, nil
}

// repoSkillTree fetches the repository tree and locates the named skill's
// directory. It returns the skill directory (empty when the skill is not
// found) alongside the full tree entries for further filtering.
func repoSkillTree(ctx context.Context, client *gogithub.Client, owner, repo, skillName string) (string, []*gogithub.TreeEntry, error) {
	tree, _, err := client.Git.GetTree(ctx, owner, repo, "HEAD", true)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get repository tree: %w", err)
	}
	for _, entry := range tree.Entries {
		if entry.GetType() != "blob" {
			continue
		}
		if skill := matchSkillConventions(entry.GetPath()); skill != nil && skill.Name == skillName {
			return skill.Dir, tree.Entries, nil
		}
	}
	return "", tree.Entries, nil
}
