package github_test

import (
	"context"
	"errors"
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
)

func TestIsFeatureEnabled_WithEnabledFlag(t *testing.T) {
	t.Parallel()

	// Create a feature checker that returns true for "test_flag"
	checker := func(_ context.Context, flagName string) (bool, error) {
		return flagName == "test_flag", nil
	}

	// Create deps with the checker using NewBaseDeps
	deps := github.NewBaseDeps(
		nil, // client
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // contentWindowSize
		checker, // featureChecker
	)

	// Test enabled flag
	result := deps.IsFeatureEnabled(context.Background(), "test_flag")
	assert.True(t, result, "Expected test_flag to be enabled")

	// Test disabled flag
	result = deps.IsFeatureEnabled(context.Background(), "other_flag")
	assert.False(t, result, "Expected other_flag to be disabled")
}

func TestIsFeatureEnabled_WithoutChecker(t *testing.T) {
	t.Parallel()

	// Create deps without feature checker (nil)
	deps := github.NewBaseDeps(
		nil, // client
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,   // contentWindowSize
		nil, // featureChecker (nil)
	)

	// Should return false when checker is nil
	result := deps.IsFeatureEnabled(context.Background(), "any_flag")
	assert.False(t, result, "Expected false when checker is nil")
}

func TestIsFeatureEnabled_EmptyFlagName(t *testing.T) {
	t.Parallel()

	// Create a feature checker
	checker := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}

	deps := github.NewBaseDeps(
		nil, // client
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // contentWindowSize
		checker, // featureChecker
	)

	// Should return false for empty flag name
	result := deps.IsFeatureEnabled(context.Background(), "")
	assert.False(t, result, "Expected false for empty flag name")
}

func TestIsFeatureEnabled_CheckerError(t *testing.T) {
	t.Parallel()

	// Create a feature checker that returns an error
	checker := func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("checker error")
	}

	deps := github.NewBaseDeps(
		nil, // client
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // contentWindowSize
		checker, // featureChecker
	)

	// Should return false and log error (not crash)
	result := deps.IsFeatureEnabled(context.Background(), "error_flag")
	assert.False(t, result, "Expected false when checker returns error")
}
