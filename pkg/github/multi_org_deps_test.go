package github

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestMultiOrgDeps creates a MultiOrgDeps backed by a real MultiOrgClientFactory
// using the testRSAPEM key defined in multi_org_factory_test.go.
func newTestMultiOrgDeps(installations map[string]int64, lockdownMode bool, opts ...lockdown.RepoAccessOption) *MultiOrgDeps {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(
		1234,
		testRSAPEM,
		installations,
		nil, // apiHost — nil means dotcom defaults
		rawURL,
		"test",
	)
	t := translations.NullTranslationHelper
	flags := FeatureFlags{LockdownMode: lockdownMode}
	return NewMultiOrgDeps(factory, t, flags, 512, nil, lockdownMode, opts)
}

// --- Compile-time interface check ---
// This is also asserted via var _ ToolDependencies = (*MultiOrgDeps)(nil) in
// multi_org_deps.go, but we include it here so the test file documents the
// contract explicitly.
func TestMultiOrgDeps_ImplementsToolDependencies(t *testing.T) {
	var _ ToolDependencies = (*MultiOrgDeps)(nil)
}

// --- GetClient ---

func TestMultiOrgDeps_GetClient_RoutesOwnerFromContext(t *testing.T) {
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), false)

	ctx := ContextWithOwner(context.Background(), "my-org")
	client, err := deps.GetClient(ctx)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestMultiOrgDeps_GetClient_EmptyOwnerUsesDefault(t *testing.T) {
	// No owner in context → factory uses "" → falls back to defaultInstall.
	deps := newTestMultiOrgDeps(makeInstallations("_default", 999), false)

	// No ContextWithOwner call — owner is "".
	client, err := deps.GetClient(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestMultiOrgDeps_GetClient_ErrorWhenNoInstallation(t *testing.T) {
	// No installation for "unknown-org" and no default → fail-closed.
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), false)

	ctx := ContextWithOwner(context.Background(), "unknown-org")
	_, err := deps.GetClient(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no GitHub App installation")
}

// --- GetGQLClient ---

func TestMultiOrgDeps_GetGQLClient_RoutesOwnerFromContext(t *testing.T) {
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), false)

	ctx := ContextWithOwner(context.Background(), "my-org")
	client, err := deps.GetGQLClient(ctx)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestMultiOrgDeps_GetGQLClient_ErrorWhenNoInstallation(t *testing.T) {
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), false)

	ctx := ContextWithOwner(context.Background(), "no-such-org")
	_, err := deps.GetGQLClient(ctx)
	require.Error(t, err)
}

// --- GetRawClient ---

func TestMultiOrgDeps_GetRawClient_RoutesOwnerFromContext(t *testing.T) {
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), false)

	ctx := ContextWithOwner(context.Background(), "my-org")
	client, err := deps.GetRawClient(ctx)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestMultiOrgDeps_GetRawClient_ErrorWhenNoInstallation(t *testing.T) {
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), false)

	ctx := ContextWithOwner(context.Background(), "no-such-org")
	_, err := deps.GetRawClient(ctx)
	require.Error(t, err)
}

// --- GetRepoAccessCache ---

func TestMultiOrgDeps_GetRepoAccessCache_NilWhenLockdownDisabled(t *testing.T) {
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), false /* lockdownMode */)

	ctx := ContextWithOwner(context.Background(), "my-org")
	cache, err := deps.GetRepoAccessCache(ctx)
	require.NoError(t, err)
	assert.Nil(t, cache, "should return nil when lockdown mode is disabled")
}

func TestMultiOrgDeps_GetRepoAccessCache_NonNilWhenLockdownEnabled(t *testing.T) {
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), true /* lockdownMode */)

	ctx := ContextWithOwner(context.Background(), "my-org")
	cache, err := deps.GetRepoAccessCache(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cache, "should return a cache when lockdown mode is enabled")
}

func TestMultiOrgDeps_GetRepoAccessCache_ErrorPropagatesFromGQL(t *testing.T) {
	// lockdownMode=true but no installation for the owner → GQL client error propagates.
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), true)

	ctx := ContextWithOwner(context.Background(), "no-such-org")
	_, err := deps.GetRepoAccessCache(ctx)
	require.Error(t, err)
}

// --- GetT, GetFlags, GetContentWindowSize ---

func TestMultiOrgDeps_GetT_ReturnsConfiguredHelper(t *testing.T) {
	called := false
	customT := translations.TranslationHelperFunc(func(key, fallback string) string {
		called = true
		return fallback
	})

	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, rawURL, "test")
	deps := NewMultiOrgDeps(factory, customT, FeatureFlags{}, 256, nil, false, nil)

	result := deps.GetT()("some.key", "fallback-value")
	assert.True(t, called, "GetT should return the configured translation helper")
	assert.Equal(t, "fallback-value", result)
}

func TestMultiOrgDeps_GetFlags_ReturnsConfiguredFlags(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, rawURL, "test")
	flags := FeatureFlags{LockdownMode: true, InsidersMode: true}
	deps := NewMultiOrgDeps(factory, translations.NullTranslationHelper, flags, 256, nil, true, nil)

	got := deps.GetFlags(context.Background())
	assert.Equal(t, flags, got)
}

func TestMultiOrgDeps_GetContentWindowSize_ReturnsConfiguredValue(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, rawURL, "test")
	deps := NewMultiOrgDeps(factory, translations.NullTranslationHelper, FeatureFlags{}, 1024, nil, false, nil)

	assert.Equal(t, 1024, deps.GetContentWindowSize())
}

// --- IsFeatureEnabled ---

func TestMultiOrgDeps_IsFeatureEnabled_NilCheckerReturnsFalse(t *testing.T) {
	deps := newTestMultiOrgDeps(makeInstallations("my-org", 111), false)
	// featureChecker is nil in newTestMultiOrgDeps
	assert.False(t, deps.IsFeatureEnabled(context.Background(), "some-flag"))
}

func TestMultiOrgDeps_IsFeatureEnabled_EmptyFlagReturnsFalse(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, rawURL, "test")
	checker := inventory.FeatureFlagChecker(func(_ context.Context, _ string) (bool, error) {
		return true, nil
	})
	deps := NewMultiOrgDeps(factory, translations.NullTranslationHelper, FeatureFlags{}, 256, checker, false, nil)

	assert.False(t, deps.IsFeatureEnabled(context.Background(), ""),
		"empty flag name should return false even when checker is configured")
}

func TestMultiOrgDeps_IsFeatureEnabled_CheckerErrorReturnsFalse(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, rawURL, "test")
	checker := inventory.FeatureFlagChecker(func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("checker error")
	})
	deps := NewMultiOrgDeps(factory, translations.NullTranslationHelper, FeatureFlags{}, 256, checker, false, nil)

	assert.False(t, deps.IsFeatureEnabled(context.Background(), "some-flag"),
		"checker error should return false")
}

func TestMultiOrgDeps_IsFeatureEnabled_CheckerEnabledReturnsTrue(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, rawURL, "test")
	checker := inventory.FeatureFlagChecker(func(_ context.Context, flagName string) (bool, error) {
		return flagName == "enabled-flag", nil
	})
	deps := NewMultiOrgDeps(factory, translations.NullTranslationHelper, FeatureFlags{}, 256, checker, false, nil)

	assert.True(t, deps.IsFeatureEnabled(context.Background(), "enabled-flag"))
	assert.False(t, deps.IsFeatureEnabled(context.Background(), "other-flag"))
}

// --- Per-org lockdown cache tests ---

func TestMultiOrgDeps_GetRepoAccessCache_DisabledReturnsNil(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM, makeInstallations("my-org", 111), nil, rawURL, "test")
	deps := NewMultiOrgDeps(factory, translations.NullTranslationHelper, FeatureFlags{}, 256, nil, false, nil)

	cache, err := deps.GetRepoAccessCache(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, cache, "lockdown disabled should return nil cache")
}

func TestMultiOrgDeps_GetRepoAccessCache_PerOrgIsolation(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM,
		makeInstallations("org-a", 111, "org-b", 222), nil, rawURL, "test")
	deps := NewMultiOrgDeps(factory, translations.NullTranslationHelper, FeatureFlags{}, 256, nil, true, nil)

	ctxA := ContextWithOwner(context.Background(), "org-a")
	cacheA, err := deps.GetRepoAccessCache(ctxA)
	require.NoError(t, err)
	require.NotNil(t, cacheA, "lockdown enabled should return non-nil cache")

	ctxB := ContextWithOwner(context.Background(), "org-b")
	cacheB, err := deps.GetRepoAccessCache(ctxB)
	require.NoError(t, err)
	require.NotNil(t, cacheB)

	assert.NotSame(t, cacheA, cacheB,
		"different orgs should get different lockdown cache instances")

	// Same org should return the same cached instance.
	cacheA2, err := deps.GetRepoAccessCache(ctxA)
	require.NoError(t, err)
	assert.Same(t, cacheA, cacheA2,
		"same org should return the same cached lockdown instance")
}

func TestMultiOrgDeps_GetRepoAccessCache_DefaultOwner(t *testing.T) {
	rawURL, _ := url.Parse("https://raw.githubusercontent.com/")
	factory := NewMultiOrgClientFactory(1234, testRSAPEM,
		makeInstallations("my-org", 111, "_default", 999), nil, rawURL, "test")
	deps := NewMultiOrgDeps(factory, translations.NullTranslationHelper, FeatureFlags{}, 256, nil, true, nil)

	// No owner in context → should use "_default" key.
	cache, err := deps.GetRepoAccessCache(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cache, "should create cache for default installation")
}
