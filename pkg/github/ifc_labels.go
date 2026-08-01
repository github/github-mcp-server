package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// setIFCLabel 写入s given IFC security label into 一个工具 结果's _meta
// under "ifc" key, allocating Meta map if necessary.
func setIFCLabel(r *mcp.CallToolResult, label ifc.SecurityLabel) {
	if r.Meta == nil {
		r.Meta = mcp.Meta{}
	}
	r.Meta["ifc"] = label
}

func shouldAttachIFCLabel(ctx context.Context, deps ToolDependencies, r *mcp.CallToolResult) bool {
	return r != nil && !r.IsError && deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels)
}

// attachStaticIFCLabel attaches 一个fixed IFC label to 一个成功ful 工具 结果
// when IFC labels are 启用. It is 由以下内容使用： 工具 whose label does 不depend
// on any 仓库 visibility lookup (e.g. security alerts, global
// advisories, team membership, notification subjects).
//
// Err或结果 are left untouched, 以及label is omitted entirely when the
// IFC 功能标志 is 禁用.
func attachStaticIFCLabel(ctx context.Context, deps ToolDependencies, r *mcp.CallToolResult, label ifc.SecurityLabel) *mcp.CallToolResult {
	if !shouldAttachIFCLabel(ctx, deps, r) {
		return r
	}
	setIFCLabel(r, label)
	return r
}

// attachRepoVisibilityIFCLabel attaches 一个IFC label derived from 一个单个
// 仓库's visibility to 一个成功ful 工具 结果 when IFC labels are
// 启用. concrete label is produced by labelFn, which receives whether
// 仓库 is 私有.
//
// 仓库 visibility is resolved via FetchRepoIsPrivate. Consistent
// 使用other IFC-labeled 工具, 如果visibility lookup fails label
// is omitted rather than risking 一个misclassification. Err或结果 和the
// 禁用-feature case are left untouched.
func attachRepoVisibilityIFCLabel(
	ctx context.Context,
	deps ToolDependencies,
	client *github.Client,
	owner, repo string,
	r *mcp.CallToolResult,
	labelFn func(isPrivate bool) ifc.SecurityLabel,
) *mcp.CallToolResult {
	if !shouldAttachIFCLabel(ctx, deps, r) {
		return r
	}
	isPrivate, err := FetchRepoIsPrivate(ctx, client, owner, repo)
	if err != nil {
		return r
	}
	setIFCLabel(r, labelFn(isPrivate))
	return r
}

// ifcSearchPostProcessOption 返回 一个searchOption that attaches IFC labels to
// 一个multi-仓库 search 结果. feature-flag 检查 is centralized here
// (mirroring attach* helpers above) rather than in 每个search 工具
// 处理器: when IFC labels are 禁用 it 返回 一个no-op option, so 调用ers
// can pass it unconditionally to searchHandler.
func ifcSearchPostProcessOption(ctx context.Context, deps ToolDependencies) searchOption {
	if !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
		return func(*searchConfig) {}
	}
	return withSearchPostProcess(searchIssuesIFCPostProcess(deps))
}

// attachRepoVisibilityIFCLabelLazy is like attachRepoVisibilityIFCLabel but
// resolves REST 客户端 itself, 仅when IFC labels are 启用. It is used
// by 工具 whose 处理器 holds 一个GraphQL 客户端 (或no 客户端 yet) 和would
// otherwise have to acquire 一个REST 客户端 solely to compute label. The
// feature-flag 检查 is centralized here so 调用ers can invoke it
// unconditionally; 如果客户端 can不be obtained 或visibility lookup
// fails, label is omitted rather than risking 一个misclassification.
func attachRepoVisibilityIFCLabelLazy(
	ctx context.Context,
	deps ToolDependencies,
	owner, repo string,
	r *mcp.CallToolResult,
	labelFn func(isPrivate bool) ifc.SecurityLabel,
) *mcp.CallToolResult {
	if !shouldAttachIFCLabel(ctx, deps, r) {
		return r
	}
	client, err := deps.GetClient(ctx)
	if err != nil {
		return r
	}
	return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, labelFn)
}

// attachJoinedIFCLabel attaches 一个IFC label computed by joining 一个set of
// per-item visibilities (真 == 私有) when IFC labels are 启用. joinFn
// is lattice join 用于relevant item kind (e.g. ifc.LabelSearchIssues or
// ifc.LabelProjectList). visibility slice is cheap to build from an
// al读取y-fetched 响应, so 调用ers may construct it unconditionally 和let
// this helper own feature-flag gate.
func attachJoinedIFCLabel(
	ctx context.Context,
	deps ToolDependencies,
	r *mcp.CallToolResult,
	visibilities []bool,
	joinFn func([]bool) ifc.SecurityLabel,
) *mcp.CallToolResult {
	if !shouldAttachIFCLabel(ctx, deps, r) {
		return r
	}
	setIFCLabel(r, joinFn(visibilities))
	return r
}

func attachProjectVisibilityIFCLabel(
	ctx context.Context,
	deps ToolDependencies,
	r *mcp.CallToolResult,
	isPrivate bool,
	labelFn func(isPrivate bool) ifc.SecurityLabel,
) *mcp.CallToolResult {
	if !shouldAttachIFCLabel(ctx, deps, r) {
		return r
	}
	setIFCLabel(r, labelFn(isPrivate))
	return r
}

// 新的RepoVisibilityIFCLabeler 返回 一个closure that attaches 一个repo-visibility
// IFC label to 一个工具 结果, f或处理器s that have several 返回 路径s and
// want to label 每个one. 返回ed 函数 owns feature-flag gate (so
// 调用ers invoke it unconditionally) 和caches 仓库 visibility
// lookup across 调用, so 一个处理器 that 返回 from many 分支 仅pays
// f或one FetchRepoIsPrivate 调用. 一个failed visibility lookup is 不cached, so
// 一个later 返回 路径 can retry; on persistent failure label is omitted
// rather than risking 一个misclassification.
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
