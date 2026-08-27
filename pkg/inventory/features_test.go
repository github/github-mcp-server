package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureRuleSupportsBooleanExpressions(t *testing.T) {
	rule := NewFeatureRule(
		[]FeatureFlag{"x", "y"},
		func(featureAsBool FeatureResolver) bool {
			return !featureAsBool("x") || !featureAsBool("y")
		},
	)

	tests := []struct {
		name   string
		values map[FeatureFlag]bool
		want   bool
	}{
		{name: "neither enabled", values: map[FeatureFlag]bool{}, want: true},
		{name: "one enabled", values: map[FeatureFlag]bool{"x": true}, want: true},
		{name: "both enabled", values: map[FeatureFlag]bool{"x": true, "y": true}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rule.Enabled(func(flag FeatureFlag) bool {
				return tt.values[flag]
			}))
		})
	}

}

func TestFeatureRuleFailsClosedForUndeclaredFeature(t *testing.T) {
	rule := NewFeatureRule(
		[]FeatureFlag{"declared"},
		func(featureAsBool FeatureResolver) bool {
			return featureAsBool("undeclared")
		},
	)

	assert.False(t, rule.Enabled(func(FeatureFlag) bool { return true }))
}

func TestFeatureRuleFailsClosedForEmptyFeature(t *testing.T) {
	rule := NewFeatureRule(
		[]FeatureFlag{"declared"},
		func(featureAsBool FeatureResolver) bool {
			return !featureAsBool("")
		},
	)

	assert.False(t, rule.Enabled(func(FeatureFlag) bool { return true }))
}

func TestResolvedFeaturesDeduplicateAndCacheChecks(t *testing.T) {
	calls := make(map[FeatureFlag]int)
	checker := func(_ context.Context, flag FeatureFlag) (bool, error) {
		calls[flag]++
		if flag == "error" {
			return false, errors.New("failed")
		}
		return flag == "enabled", nil
	}

	ctx := WithResolvedFeatures(
		context.Background(),
		checker,
		[]FeatureFlag{"enabled", "disabled", "enabled", "error"},
	)

	assert.True(t, ResolveFeature(ctx, nil, "enabled"))
	assert.False(t, ResolveFeature(ctx, checker, "disabled"))
	assert.False(t, ResolveFeature(ctx, checker, "error"))
	assert.False(t, ResolveFeature(ctx, checker, "lazy"))
	assert.False(t, ResolveFeature(ctx, checker, "lazy"))

	require.Equal(t, map[FeatureFlag]int{
		"enabled":  1,
		"disabled": 1,
		"error":    1,
		"lazy":     1,
	}, calls)
}

func TestResolvedFeaturesAllowNilChecker(t *testing.T) {
	ctx := WithResolvedFeatures(context.Background(), nil, []FeatureFlag{"feature"})
	assert.False(t, ResolveFeature(ctx, nil, "feature"))
}

func TestLazyFeatureResolutionUsesLiveContext(t *testing.T) {
	type contextKey struct{}
	checker := func(ctx context.Context, _ FeatureFlag) (bool, error) {
		enabled, _ := ctx.Value(contextKey{}).(bool)
		return enabled, nil
	}

	ctx := WithResolvedFeatures(context.Background(), checker, nil)
	ctx = context.WithValue(ctx, contextKey{}, true)
	assert.True(t, ResolveFeature(ctx, checker, "handler_only"))
}
