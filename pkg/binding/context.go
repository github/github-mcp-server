// Package binding turns the full GitHub MCP tool universe into a bespoke,
// context-scoped tool surface.
//
// A scoped server is bound once, by the operator, to a fixed GitHub context
// (a repository, a pull request, or a project). Inside that mode the binding
// layer presents what looks like a purpose-built server for that single
// context: the context-identifying parameters (owner, repo, pull number, …)
// are removed from each tool's advertised input schema and injected
// server-side, unsupported method values are pruned from the schema, and
// tools that cannot be structurally confined to the bound context are omitted
// entirely.
//
// The design is interface-first. The per-mode Manifest (see manifest.go) is
// the product: it declares exactly which tools appear, how they are described,
// which parameters they expose, and which values are fixed. Everything else in
// this package is the shared plumbing that adapts the existing tool handlers to
// serve that declared interface without duplicating their logic.
package binding

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind identifies a scoped server mode.
type Kind string

const (
	// KindRepo binds the server to a single repository ({owner, repo}).
	KindRepo Kind = "repo"
	// KindPullRequest binds the server to a single pull request
	// ({owner, repo, pullNumber}).
	KindPullRequest Kind = "pull_request"
	// KindProject binds the server to a single ProjectsV2 project
	// ({ownerType, owner, projectNumber}).
	KindProject Kind = "project"
)

// Context is the fixed GitHub context a scoped server is bound to. It is
// constructed once from operator configuration (a CLI flag or an HTTP route)
// and is immutable for the lifetime of the server.
type Context struct {
	Kind Kind

	// Owner is the repository or project owner login. Set for every kind.
	Owner string
	// Repo is the repository name. Set for repo and pull_request kinds.
	Repo string
	// PullNumber is the pull request number. Set for pull_request kind.
	PullNumber int

	// OwnerType is "user" or "org" (the value the projects tools expect).
	// Set for project kind.
	OwnerType string
	// ProjectNumber is the ProjectsV2 project number. Set for project kind.
	ProjectNumber int
}

// ctxKey names a single bound value within a Context. Manifest entries map a
// tool's schema parameter to one of these keys; the binding wrapper then
// injects the correctly typed value (string vs number) at call time.
type ctxKey string

const (
	keyOwner         ctxKey = "owner"
	keyRepo          ctxKey = "repo"
	keyPullNumber    ctxKey = "pullNumber"
	keyOwnerType     ctxKey = "ownerType"
	keyProjectNumber ctxKey = "projectNumber"
)

// value returns the JSON-typed value for a bound key (string for logins,
// int for numbers) and whether it is set on this Context. A missing value is
// a server misconfiguration for any manifest that references the key, and the
// wrapper rejects the call rather than guessing.
func (c Context) value(k ctxKey) (any, bool) {
	switch k {
	case keyOwner:
		return c.Owner, c.Owner != ""
	case keyRepo:
		return c.Repo, c.Repo != ""
	case keyPullNumber:
		return c.PullNumber, c.PullNumber > 0
	case keyOwnerType:
		return c.OwnerType, c.OwnerType != ""
	case keyProjectNumber:
		return c.ProjectNumber, c.ProjectNumber > 0
	default:
		return nil, false
	}
}

// NewRepoContext builds a validated repo-mode context.
func NewRepoContext(owner, repo string) (Context, error) {
	if owner == "" || repo == "" {
		return Context{}, fmt.Errorf("repository context requires owner and repo, got %q/%q", owner, repo)
	}
	return Context{Kind: KindRepo, Owner: owner, Repo: repo}, nil
}

// NewPullRequestContext builds a validated pull-request-mode context.
func NewPullRequestContext(owner, repo string, pullNumber int) (Context, error) {
	if owner == "" || repo == "" {
		return Context{}, fmt.Errorf("pull request context requires owner and repo, got %q/%q", owner, repo)
	}
	if pullNumber <= 0 {
		return Context{}, fmt.Errorf("pull request context requires a positive pull number, got %d", pullNumber)
	}
	return Context{Kind: KindPullRequest, Owner: owner, Repo: repo, PullNumber: pullNumber}, nil
}

// NewProjectContext builds a validated project-mode context. ownerType must be
// "user" or "org" (the values the projects tools accept).
func NewProjectContext(ownerType, owner string, projectNumber int) (Context, error) {
	if owner == "" {
		return Context{}, fmt.Errorf("project context requires an owner")
	}
	if projectNumber <= 0 {
		return Context{}, fmt.Errorf("project context requires a positive project number, got %d", projectNumber)
	}
	switch ownerType {
	case "user", "org":
	default:
		return Context{}, fmt.Errorf("project owner type must be %q or %q, got %q", "user", "org", ownerType)
	}
	return Context{Kind: KindProject, Owner: owner, OwnerType: ownerType, ProjectNumber: projectNumber}, nil
}

// ParseRepository parses the repo-mode flag value "owner/repo".
func ParseRepository(s string) (Context, error) {
	owner, repo, ok := splitOwnerRepo(s)
	if !ok {
		return Context{}, fmt.Errorf("invalid --repository %q, want owner/repo", s)
	}
	return NewRepoContext(owner, repo)
}

// ParsePullRequest parses the pull-request-mode flag value "owner/repo#N".
func ParsePullRequest(s string) (Context, error) {
	repoPart, numPart, ok := strings.Cut(s, "#")
	if !ok {
		return Context{}, fmt.Errorf("invalid --pull-request %q, want owner/repo#number", s)
	}
	owner, repo, ok := splitOwnerRepo(repoPart)
	if !ok {
		return Context{}, fmt.Errorf("invalid --pull-request %q, want owner/repo#number", s)
	}
	n, err := strconv.Atoi(strings.TrimSpace(numPart))
	if err != nil {
		return Context{}, fmt.Errorf("invalid --pull-request %q: %q is not a number", s, numPart)
	}
	return NewPullRequestContext(owner, repo, n)
}

// ParseProject parses the project-mode flag value. Accepted forms:
//
//	org/owner/N   user/owner/N    (canonical owner-type prefixes)
//	orgs/owner/N  users/owner/N   (plural convenience forms)
//
// The owner-type prefix is required so the bound surface is unambiguous.
func ParseProject(s string) (Context, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return Context{}, fmt.Errorf("invalid --project %q, want org|user/owner/number", s)
	}
	ownerType, err := normalizeOwnerType(parts[0])
	if err != nil {
		return Context{}, fmt.Errorf("invalid --project %q: %w", s, err)
	}
	owner := strings.TrimSpace(parts[1])
	n, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return Context{}, fmt.Errorf("invalid --project %q: %q is not a number", s, parts[2])
	}
	return NewProjectContext(ownerType, owner, n)
}

func normalizeOwnerType(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "org", "orgs", "organization":
		return "org", nil
	case "user", "users":
		return "user", nil
	default:
		return "", fmt.Errorf("owner type %q must be org or user", s)
	}
}

func splitOwnerRepo(s string) (owner, repo string, ok bool) {
	owner, repo, _ = strings.Cut(strings.TrimSpace(s), "/")
	owner, repo = strings.TrimSpace(owner), strings.TrimSpace(repo)
	if owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", false
	}
	return owner, repo, true
}
