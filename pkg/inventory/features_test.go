package inventory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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

func TestFeatureRuleRejectsUndeclaredFeature(t *testing.T) {
	assert.PanicsWithValue(t, `feature rule used undeclared feature "undeclared"`, func() {
		NewFeatureRule(
			[]FeatureFlag{"declared"},
			func(featureAsBool FeatureResolver) bool {
				return featureAsBool("undeclared")
			},
		)
	})
}

func TestFeatureRuleValidatesShortCircuitedBranches(t *testing.T) {
	assert.PanicsWithValue(t, `feature rule used undeclared feature "undeclared"`, func() {
		NewFeatureRule(
			[]FeatureFlag{"declared"},
			func(featureAsBool FeatureResolver) bool {
				return featureAsBool("declared") || featureAsBool("undeclared")
			},
		)
	})
}

func TestFeatureRuleRejectsEmptyFeature(t *testing.T) {
	assert.PanicsWithValue(t, `feature rule used undeclared feature ""`, func() {
		NewFeatureRule(
			[]FeatureFlag{"declared"},
			func(featureAsBool FeatureResolver) bool {
				return !featureAsBool("")
			},
		)
	})
}

func TestFeatureRuleRejectsDeclaredFlagsWithoutPredicate(t *testing.T) {
	assert.PanicsWithValue(t, "feature rule declares flags without a predicate", func() {
		NewFeatureRule([]FeatureFlag{"declared"}, nil)
	})
	assert.True(t, FeatureRule{}.IsZero())
}

func TestFeatureStateDeduplicatesChecks(t *testing.T) {
	calls := make(map[FeatureFlag]int)
	checker := func(_ context.Context, flag string) (bool, error) {
		calls[FeatureFlag(flag)]++
		if flag == "error" {
			return false, errors.New("failed")
		}
		return flag == "enabled", nil
	}

	ctx := WithFeatureState(context.Background(), checker)

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

func TestFeatureRuleShortCircuitsAndMemoizes(t *testing.T) {
	rule := NewFeatureRule(
		[]FeatureFlag{"first", "second"},
		func(featureAsBool FeatureResolver) bool {
			return featureAsBool("first") && featureAsBool("second") && featureAsBool("first")
		},
	)

	tests := []struct {
		name  string
		first bool
		want  int
	}{
		{name: "false prerequisite", want: 1},
		{name: "true prerequisite", first: true, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			checker := func(_ context.Context, flag string) (bool, error) {
				calls++
				return flag == "first" && tt.first, nil
			}
			ctx := WithFeatureState(context.Background(), checker)
			assert.False(t, rule.Enabled(func(flag FeatureFlag) bool {
				return ResolveFeature(ctx, checker, flag)
			}))
			assert.Equal(t, tt.want, calls)
		})
	}
}

func TestFeatureStateAllowsNilChecker(t *testing.T) {
	ctx := WithFeatureState(context.Background(), nil)
	assert.False(t, ResolveFeature(ctx, nil, "feature"))
}

func TestLazyFeatureResolutionUsesLiveContext(t *testing.T) {
	type contextKey struct{}
	checker := func(ctx context.Context, _ string) (bool, error) {
		enabled, _ := ctx.Value(contextKey{}).(bool)
		return enabled, nil
	}

	ctx := WithFeatureState(context.Background(), checker)
	ctx = context.WithValue(ctx, contextKey{}, true)
	assert.True(t, ResolveFeature(ctx, checker, "handler_only"))
}

func TestNestedFeatureResolutionFailsClosed(t *testing.T) {
	var checker FeatureFlagChecker
	checker = func(ctx context.Context, flag string) (bool, error) {
		if flag == "meta" {
			return ResolveFeature(ctx, checker, "base"), nil
		}
		return flag == "base", nil
	}

	ctx := WithFeatureState(context.Background(), checker)
	assert.False(t, ResolveFeature(ctx, nil, "meta"))
	assert.True(t, ResolveFeature(ctx, nil, "base"))
}

func TestDirectFeatureResolutionCycleFailsClosed(t *testing.T) {
	var checker FeatureFlagChecker
	checker = func(ctx context.Context, flag string) (bool, error) {
		return ResolveFeature(ctx, checker, FeatureFlag(flag)), nil
	}

	ctx := WithFeatureState(context.Background(), checker)
	assert.False(t, ResolveFeature(ctx, nil, "cycle"))
}

func TestSelfNegatingFeatureResolutionCycleFailsClosed(t *testing.T) {
	var checker FeatureFlagChecker
	checker = func(ctx context.Context, flag string) (bool, error) {
		return !ResolveFeature(ctx, checker, FeatureFlag(flag)), nil
	}

	ctx := WithFeatureState(context.Background(), checker)
	assert.False(t, ResolveFeature(ctx, nil, "cycle"))
}

func TestMutualFeatureResolutionCycleFailsClosed(t *testing.T) {
	var checker FeatureFlagChecker
	checker = func(ctx context.Context, flag string) (bool, error) {
		switch flag {
		case "a":
			return !ResolveFeature(ctx, checker, "b"), nil
		case "b":
			return !ResolveFeature(ctx, checker, "a"), nil
		default:
			return false, nil
		}
	}

	ctx := WithFeatureState(context.Background(), checker)
	assert.False(t, ResolveFeature(ctx, nil, "a"))
	assert.False(t, ResolveFeature(ctx, nil, "b"))
}

func TestThreeNodeFeatureResolutionCycleFailsClosed(t *testing.T) {
	var checker FeatureFlagChecker
	checker = func(ctx context.Context, flag string) (bool, error) {
		switch flag {
		case "a":
			return !ResolveFeature(ctx, checker, "b"), nil
		case "b":
			return !ResolveFeature(ctx, checker, "c"), nil
		case "c":
			return !ResolveFeature(ctx, checker, "a"), nil
		default:
			return false, nil
		}
	}

	ctx := WithFeatureState(context.Background(), checker)
	assert.False(t, ResolveFeature(ctx, nil, "a"))
	assert.False(t, ResolveFeature(ctx, nil, "b"))
	assert.False(t, ResolveFeature(ctx, nil, "c"))
}

func TestConcurrentFeatureResolutionIsDeduplicated(t *testing.T) {
	var (
		calls   int
		callsMu sync.Mutex
		started = make(chan struct{})
		release = make(chan struct{})
	)
	checker := func(context.Context, string) (bool, error) {
		callsMu.Lock()
		calls++
		callsMu.Unlock()
		close(started)
		<-release
		return true, nil
	}

	ctx := WithFeatureState(context.Background(), checker)
	results := make(chan bool, 2)
	go func() { results <- ResolveFeature(ctx, nil, "shared") }()
	<-started
	go func() { results <- ResolveFeature(ctx, nil, "shared") }()
	close(release)

	for range 2 {
		select {
		case result := <-results:
			assert.True(t, result)
		case <-time.After(time.Second):
			t.Fatal("feature resolution did not complete")
		}
	}
	callsMu.Lock()
	assert.Equal(t, 1, calls)
	callsMu.Unlock()
}

func TestConcurrentCrossFeatureCycleFailsClosed(t *testing.T) {
	startedA := make(chan struct{})
	startedB := make(chan struct{})
	var checker FeatureFlagChecker
	checker = func(ctx context.Context, flag string) (bool, error) {
		switch flag {
		case "a":
			close(startedA)
			<-startedB
			return !ResolveFeature(ctx, checker, "b"), nil
		case "b":
			close(startedB)
			<-startedA
			return !ResolveFeature(ctx, checker, "a"), nil
		default:
			return false, nil
		}
	}

	ctx := WithFeatureState(context.Background(), checker)
	results := make(chan bool, 2)
	go func() { results <- ResolveFeature(ctx, nil, "a") }()
	go func() { results <- ResolveFeature(ctx, nil, "b") }()

	for range 2 {
		select {
		case result := <-results:
			assert.False(t, result)
		case <-time.After(time.Second):
			t.Fatal("concurrent feature cycle deadlocked")
		}
	}
}

func TestContextFeatureStateTakesPrecedence(t *testing.T) {
	var fallbackCalls int
	stateChecker := func(context.Context, string) (bool, error) {
		return true, nil
	}
	fallbackChecker := func(context.Context, string) (bool, error) {
		fallbackCalls++
		return false, nil
	}

	ctx := WithFeatureState(context.Background(), stateChecker)
	assert.True(t, ResolveFeature(ctx, fallbackChecker, "feature"))
	assert.Zero(t, fallbackCalls)
}
