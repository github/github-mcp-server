package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/google/go-github/v87/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// setIFCLabel writes the given IFC security label into a tool result's _meta
// under the "ifc" key, allocating the Meta map if necessary.
func setIFCLabel(r *mcp.CallToolResult, label ifc.SecurityLabel) {
	if r.Meta == nil {
		r.Meta = mcp.Meta{}
	}
	r.Meta["ifc"] = label
}

// attachStaticIFCLabel attaches a fixed IFC label to a successful tool result
// when IFC labels are enabled. It is used by tools whose label does not depend
// on any repository visibility lookup (e.g. security alerts, global
// advisories, team membership, notification subjects).
//
// Error results are left untouched, and the label is omitted entirely when the
// IFC feature flag is disabled.
func attachStaticIFCLabel(ctx context.Context, deps ToolDependencies, r *mcp.CallToolResult, label ifc.SecurityLabel) *mcp.CallToolResult {
	if r == nil || r.IsError || !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
		return r
	}
	setIFCLabel(r, label)
	return r
}

// attachRepoVisibilityIFCLabel attaches an IFC label derived from a single
// repository's visibility to a successful tool result when IFC labels are
// enabled. The concrete label is produced by labelFn, which receives whether
// the repository is private.
//
// The repository visibility is resolved via FetchRepoIsPrivate. Consistent
// with the other IFC-labeled tools, if the visibility lookup fails the label
// is omitted rather than risking a misclassification. Error results and the
// disabled-feature case are left untouched.
func attachRepoVisibilityIFCLabel(
	ctx context.Context,
	deps ToolDependencies,
	client *github.Client,
	owner, repo string,
	r *mcp.CallToolResult,
	labelFn func(isPrivate bool) ifc.SecurityLabel,
) *mcp.CallToolResult {
	if r == nil || r.IsError || !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
		return r
	}
	isPrivate, err := FetchRepoIsPrivate(ctx, client, owner, repo)
	if err != nil {
		return r
	}
	setIFCLabel(r, labelFn(isPrivate))
	return r
}

// ifcSearchPostProcessOption returns a searchOption that attaches IFC labels to
// a multi-repository search result. The feature-flag check is centralized here
// (mirroring the attach* helpers above) rather than in each search tool
// handler: when IFC labels are disabled it returns a no-op option, so callers
// can pass it unconditionally to searchHandler.
func ifcSearchPostProcessOption(ctx context.Context, deps ToolDependencies) searchOption {
	if !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
		return func(*searchConfig) {}
	}
	return withSearchPostProcess(searchIssuesIFCPostProcess(deps))
}

// attachRepoVisibilityIFCLabelLazy is like attachRepoVisibilityIFCLabel but
// resolves the REST client itself, only when IFC labels are enabled. It is used
// by tools whose handler holds a GraphQL client (or no client yet) and would
// otherwise have to acquire a REST client solely to compute the label. The
// feature-flag check is centralized here so callers can invoke it
// unconditionally; if the client cannot be obtained or the visibility lookup
// fails, the label is omitted rather than risking a misclassification.
func attachRepoVisibilityIFCLabelLazy(
	ctx context.Context,
	deps ToolDependencies,
	owner, repo string,
	r *mcp.CallToolResult,
	labelFn func(isPrivate bool) ifc.SecurityLabel,
) *mcp.CallToolResult {
	if r == nil || r.IsError || !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
		return r
	}
	client, err := deps.GetClient(ctx)
	if err != nil {
		return r
	}
	return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, labelFn)
}

// attachJoinedIFCLabel attaches an IFC label computed by joining a set of
// per-item visibilities (true == private) when IFC labels are enabled. joinFn
// is the lattice join for the relevant item kind (e.g. ifc.LabelSearchIssues or
// ifc.LabelGistList). The visibility slice is cheap to build from an
// already-fetched response, so callers may construct it unconditionally and let
// this helper own the feature-flag gate.
func attachJoinedIFCLabel(
	ctx context.Context,
	deps ToolDependencies,
	r *mcp.CallToolResult,
	visibilities []bool,
	joinFn func([]bool) ifc.SecurityLabel,
) *mcp.CallToolResult {
	if r == nil || r.IsError || !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
		return r
	}
	setIFCLabel(r, joinFn(visibilities))
	return r
}

// newRepoVisibilityIFCLabeler returns a closure that attaches a repo-visibility
// IFC label to a tool result, for handlers that have several return paths and
// want to label each one. The returned function owns the feature-flag gate (so
// callers invoke it unconditionally) and caches the repository visibility
// lookup across calls, so a handler that returns from many branches only pays
// for one FetchRepoIsPrivate call. A failed visibility lookup is not cached, so
// a later return path can retry; on persistent failure the label is omitted
// rather than risking a misclassification.
func newRepoVisibilityIFCLabeler(
	ctx context.Context,
	deps ToolDependencies,
	client *github.Client,
	owner, repo string,
	labelFn func(isPrivate bool) ifc.SecurityLabel,
) func(*mcp.CallToolResult) *mcp.CallToolResult {
	var (
		known     bool
		isPrivate bool
	)
	return func(r *mcp.CallToolResult) *mcp.CallToolResult {
		if r == nil || r.IsError || !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
			return r
		}
		if !known {
			p, err := FetchRepoIsPrivate(ctx, client, owner, repo)
			if err != nil {
				return r
			}
			isPrivate = p
			known = true
		}
		setIFCLabel(r, labelFn(isPrivate))
		return r
	}
}
