package github_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
)

func testExporters() observability.Exporters {
	obs, _ := observability.NewExporters(slog.New(slog.DiscardHandler), metrics.NewNoopMetrics())
	return obs
}

func TestIsFeatureEnabled_WithEnabledFlag(t *testing.T) {
	t.Parallel()

	// Create 一个feature 检查er that 返回 真 f或"test_flag"
	checker := func(_ context.Context, flagName string) (bool, error) {
		return flagName == "test_flag", nil
	}

	// Create deps 使用检查er using NewBaseDeps
	deps := github.NewBaseDeps(
		nil, // 客户端
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // 内容WindowSize
		checker, // featureChecker
		testExporters(),
	)

	// Test 启用 flag
	result := deps.IsFeatureEnabled(context.Background(), "test_flag")
	assert.True(t, result, "Expected test_flag to be enabled")

	// Test 禁用 flag
	result = deps.IsFeatureEnabled(context.Background(), "other_flag")
	assert.False(t, result, "Expected other_flag to be disabled")
}

func TestIsFeatureEnabled_WithoutChecker(t *testing.T) {
	t.Parallel()

	// Create deps without feature 检查er (nil)
	deps := github.NewBaseDeps(
		nil, // 客户端
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,   // 内容WindowSize
		nil, // featureChecker (nil)
		testExporters(),
	)

	// Should 返回 假 when 检查er is nil
	result := deps.IsFeatureEnabled(context.Background(), "any_flag")
	assert.False(t, result, "Expected false when checker is nil")
}

func TestIsFeatureEnabled_EmptyFlagName(t *testing.T) {
	t.Parallel()

	// Create 一个feature 检查er
	checker := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}

	deps := github.NewBaseDeps(
		nil, // 客户端
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // 内容WindowSize
		checker, // featureChecker
		testExporters(),
	)

	// Should 返回 假 f或空 flag name
	result := deps.IsFeatureEnabled(context.Background(), "")
	assert.False(t, result, "Expected false for empty flag name")
}

func TestIsFeatureEnabled_CheckerError(t *testing.T) {
	t.Parallel()

	// Create 一个feature 检查er that 返回 一个错误
	checker := func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("checker error")
	}

	deps := github.NewBaseDeps(
		nil, // 客户端
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // 内容WindowSize
		checker, // featureChecker
		testExporters(),
	)

	// Should 返回 假 和log 错误 (不crash)
	result := deps.IsFeatureEnabled(context.Background(), "error_flag")
	assert.False(t, result, "Expected false when checker returns error")
}
