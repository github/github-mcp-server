package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureFlagCondition(t *testing.T) {
	tests := []struct {
		name           string
		flagName       string
		checkerResult  bool
		checkerErr     error
		hasChecker     bool
		expectedResult bool
		expectedErr    bool
	}{
		{
			name:           "flag enabled",
			flagName:       "test_flag",
			checkerResult:  true,
			hasChecker:     true,
			expectedResult: true,
		},
		{
			name:           "flag disabled",
			flagName:       "test_flag",
			checkerResult:  false,
			hasChecker:     true,
			expectedResult: false,
		},
		{
			name:           "no checker in context",
			flagName:       "test_flag",
			hasChecker:     false,
			expectedResult: false,
		},
		{
			name:           "checker returns error",
			flagName:       "test_flag",
			checkerErr:     errors.New("flag check failed"),
			hasChecker:     true,
			expectedResult: false,
			expectedErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.hasChecker {
				checker := func(_ context.Context, flagName string) (bool, error) {
					assert.Equal(t, tt.flagName, flagName)
					return tt.checkerResult, tt.checkerErr
				}
				ctx = ContextWithFeatureChecker(ctx, checker)
			}

			cond := FeatureFlag(tt.flagName)
			result, err := cond.Evaluate(ctx)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestContextBoolCondition(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		contextBools   ContextBools
		expectedResult bool
	}{
		{
			name:           "bool is true",
			key:            "is_cca",
			contextBools:   ContextBools{"is_cca": true},
			expectedResult: true,
		},
		{
			name:           "bool is false",
			key:            "is_cca",
			contextBools:   ContextBools{"is_cca": false},
			expectedResult: false,
		},
		{
			name:           "key not found",
			key:            "is_cca",
			contextBools:   ContextBools{"other_key": true},
			expectedResult: false,
		},
		{
			name:           "no context bools",
			key:            "is_cca",
			contextBools:   nil,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.contextBools != nil {
				ctx = ContextWithBools(ctx, tt.contextBools)
			}

			cond := ContextBool(tt.key)
			result, err := cond.Evaluate(ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestStaticConditions(t *testing.T) {
	ctx := context.Background()

	t.Run("Static(true)", func(t *testing.T) {
		cond := Static(true)
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Static(false)", func(t *testing.T) {
		cond := Static(false)
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("Always()", func(t *testing.T) {
		cond := Always()
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Never()", func(t *testing.T) {
		cond := Never()
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.False(t, result)
	})
}

func TestAndCondition(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		conditions     []EnableCondition
		expectedResult bool
	}{
		{
			name:           "all true",
			conditions:     []EnableCondition{Always(), Always(), Always()},
			expectedResult: true,
		},
		{
			name:           "one false",
			conditions:     []EnableCondition{Always(), Never(), Always()},
			expectedResult: false,
		},
		{
			name:           "all false",
			conditions:     []EnableCondition{Never(), Never()},
			expectedResult: false,
		},
		{
			name:           "empty conditions",
			conditions:     []EnableCondition{},
			expectedResult: true,
		},
		{
			name:           "single true",
			conditions:     []EnableCondition{Always()},
			expectedResult: true,
		},
		{
			name:           "single false",
			conditions:     []EnableCondition{Never()},
			expectedResult: false,
		},
		{
			name:           "nil conditions filtered out",
			conditions:     []EnableCondition{Always(), nil, Always()},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := And(tt.conditions...)
			result, err := cond.Evaluate(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestOrCondition(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		conditions     []EnableCondition
		expectedResult bool
	}{
		{
			name:           "all true",
			conditions:     []EnableCondition{Always(), Always(), Always()},
			expectedResult: true,
		},
		{
			name:           "one true",
			conditions:     []EnableCondition{Never(), Always(), Never()},
			expectedResult: true,
		},
		{
			name:           "all false",
			conditions:     []EnableCondition{Never(), Never()},
			expectedResult: false,
		},
		{
			name:           "empty conditions",
			conditions:     []EnableCondition{},
			expectedResult: false,
		},
		{
			name:           "single true",
			conditions:     []EnableCondition{Always()},
			expectedResult: true,
		},
		{
			name:           "single false",
			conditions:     []EnableCondition{Never()},
			expectedResult: false,
		},
		{
			name:           "nil conditions filtered out",
			conditions:     []EnableCondition{Never(), nil, Never()},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := Or(tt.conditions...)
			result, err := cond.Evaluate(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestNotCondition(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		condition      EnableCondition
		expectedResult bool
	}{
		{
			name:           "not true",
			condition:      Always(),
			expectedResult: false,
		},
		{
			name:           "not false",
			condition:      Never(),
			expectedResult: true,
		},
		{
			name:           "not nil",
			condition:      nil,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := Not(tt.condition)
			result, err := cond.Evaluate(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestConditionFunc(t *testing.T) {
	ctx := context.Background()

	t.Run("simple function", func(t *testing.T) {
		cond := ConditionFunc(func(_ context.Context) (bool, error) {
			return true, nil
		})
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("function with error", func(t *testing.T) {
		expectedErr := errors.New("test error")
		cond := ConditionFunc(func(_ context.Context) (bool, error) {
			return false, expectedErr
		})
		result, err := cond.Evaluate(ctx)
		assert.Equal(t, expectedErr, err)
		assert.False(t, result)
	})
}

func TestComplexConditionCombinations(t *testing.T) {
	// These tests match the real-world scenarios from the remote server

	t.Run("feature flag AND user policy (web search pattern)", func(t *testing.T) {
		// Pattern: feature flag must be enabled AND user must have paid Bing access
		cond := And(
			FeatureFlag("web_search"),
			ContextBool("user_has_paid_bing_access"),
		)

		// Test: both conditions true
		ctx := context.Background()
		ctx = ContextWithFeatureChecker(ctx, func(_ context.Context, _ string) (bool, error) {
			return true, nil
		})
		ctx = ContextWithBools(ctx, ContextBools{"user_has_paid_bing_access": true})
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.True(t, result)

		// Test: feature flag true, but no user access
		ctx2 := context.Background()
		ctx2 = ContextWithFeatureChecker(ctx2, func(_ context.Context, _ string) (bool, error) {
			return true, nil
		})
		ctx2 = ContextWithBools(ctx2, ContextBools{"user_has_paid_bing_access": false})
		result2, err2 := cond.Evaluate(ctx2)
		require.NoError(t, err2)
		assert.False(t, result2)

		// Test: feature flag false
		ctx3 := context.Background()
		ctx3 = ContextWithFeatureChecker(ctx3, func(_ context.Context, _ string) (bool, error) {
			return false, nil
		})
		ctx3 = ContextWithBools(ctx3, ContextBools{"user_has_paid_bing_access": true})
		result3, err3 := cond.Evaluate(ctx3)
		require.NoError(t, err3)
		assert.False(t, result3)
	})

	t.Run("CCA bypass pattern (CCA OR feature flag)", func(t *testing.T) {
		// Pattern: CCA requests bypass feature flag, non-CCA requires feature flag
		cond := Or(
			ContextBool("is_cca"),
			FeatureFlag("agent_search"),
		)

		// Test: CCA request (bypass feature flag)
		ctx := context.Background()
		ctx = ContextWithBools(ctx, ContextBools{"is_cca": true})
		// No feature checker - CCA should pass without it
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.True(t, result)

		// Test: non-CCA with feature flag enabled
		ctx2 := context.Background()
		ctx2 = ContextWithFeatureChecker(ctx2, func(_ context.Context, _ string) (bool, error) {
			return true, nil
		})
		ctx2 = ContextWithBools(ctx2, ContextBools{"is_cca": false})
		result2, err2 := cond.Evaluate(ctx2)
		require.NoError(t, err2)
		assert.True(t, result2)

		// Test: non-CCA with feature flag disabled
		ctx3 := context.Background()
		ctx3 = ContextWithFeatureChecker(ctx3, func(_ context.Context, _ string) (bool, error) {
			return false, nil
		})
		ctx3 = ContextWithBools(ctx3, ContextBools{"is_cca": false})
		result3, err3 := cond.Evaluate(ctx3)
		require.NoError(t, err3)
		assert.False(t, result3)
	})

	t.Run("CCA AND feature flag pattern", func(t *testing.T) {
		// Pattern: must be CCA AND have feature flag enabled
		cond := And(
			ContextBool("is_cca"),
			FeatureFlag("complex_workflows"),
		)

		// Test: CCA with feature flag
		ctx := context.Background()
		ctx = ContextWithFeatureChecker(ctx, func(_ context.Context, _ string) (bool, error) {
			return true, nil
		})
		ctx = ContextWithBools(ctx, ContextBools{"is_cca": true})
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.True(t, result)

		// Test: CCA without feature flag
		ctx2 := context.Background()
		ctx2 = ContextWithFeatureChecker(ctx2, func(_ context.Context, _ string) (bool, error) {
			return false, nil
		})
		ctx2 = ContextWithBools(ctx2, ContextBools{"is_cca": true})
		result2, err2 := cond.Evaluate(ctx2)
		require.NoError(t, err2)
		assert.False(t, result2)

		// Test: non-CCA with feature flag
		ctx3 := context.Background()
		ctx3 = ContextWithFeatureChecker(ctx3, func(_ context.Context, _ string) (bool, error) {
			return true, nil
		})
		ctx3 = ContextWithBools(ctx3, ContextBools{"is_cca": false})
		result3, err3 := cond.Evaluate(ctx3)
		require.NoError(t, err3)
		assert.False(t, result3)
	})

	t.Run("copilot-chat bypass pattern", func(t *testing.T) {
		// Pattern: copilot-chat bypasses feature flag check
		cond := Or(
			ContextBool("mcp_host_is_copilot_chat"),
			FeatureFlag("semantic_code_search"),
		)

		// Test: copilot-chat host (bypass feature flag)
		ctx := context.Background()
		ctx = ContextWithBools(ctx, ContextBools{"mcp_host_is_copilot_chat": true})
		result, err := cond.Evaluate(ctx)
		require.NoError(t, err)
		assert.True(t, result)

		// Test: other host with feature flag
		ctx2 := context.Background()
		ctx2 = ContextWithFeatureChecker(ctx2, func(_ context.Context, _ string) (bool, error) {
			return true, nil
		})
		ctx2 = ContextWithBools(ctx2, ContextBools{"mcp_host_is_copilot_chat": false})
		result2, err2 := cond.Evaluate(ctx2)
		require.NoError(t, err2)
		assert.True(t, result2)

		// Test: other host without feature flag
		ctx3 := context.Background()
		ctx3 = ContextWithFeatureChecker(ctx3, func(_ context.Context, _ string) (bool, error) {
			return false, nil
		})
		ctx3 = ContextWithBools(ctx3, ContextBools{"mcp_host_is_copilot_chat": false})
		result3, err3 := cond.Evaluate(ctx3)
		require.NoError(t, err3)
		assert.False(t, result3)
	})
}

func TestContextBoolsMerging(t *testing.T) {
	ctx := context.Background()

	// Add first set of bools
	ctx = ContextWithBools(ctx, ContextBools{"key1": true, "key2": false})

	// Add second set - should merge
	ctx = ContextWithBools(ctx, ContextBools{"key3": true, "key2": true}) // key2 overwritten

	// Check all keys
	assert.True(t, ContextBoolFromContext(ctx, "key1"))
	assert.True(t, ContextBoolFromContext(ctx, "key2")) // overwritten value
	assert.True(t, ContextBoolFromContext(ctx, "key3"))
	assert.False(t, ContextBoolFromContext(ctx, "nonexistent"))
}

func TestSetContextBool(t *testing.T) {
	ctx := context.Background()
	ctx = SetContextBool(ctx, "my_flag", true)

	assert.True(t, ContextBoolFromContext(ctx, "my_flag"))
	assert.False(t, ContextBoolFromContext(ctx, "other_flag"))
}

func TestAndShortCircuit(t *testing.T) {
	callCount := 0
	ctx := context.Background()

	// First condition returns false, second should not be called
	cond := And(
		Never(),
		ConditionFunc(func(_ context.Context) (bool, error) {
			callCount++
			return true, nil
		}),
	)

	result, err := cond.Evaluate(ctx)
	require.NoError(t, err)
	assert.False(t, result)
	assert.Equal(t, 0, callCount, "second condition should not be called due to short-circuit")
}

func TestOrShortCircuit(t *testing.T) {
	callCount := 0
	ctx := context.Background()

	// First condition returns true, second should not be called
	cond := Or(
		Always(),
		ConditionFunc(func(_ context.Context) (bool, error) {
			callCount++
			return false, nil
		}),
	)

	result, err := cond.Evaluate(ctx)
	require.NoError(t, err)
	assert.True(t, result)
	assert.Equal(t, 0, callCount, "second condition should not be called due to short-circuit")
}

func TestOrContinuesOnError(t *testing.T) {
	ctx := context.Background()

	// First condition errors, but second is true - should return true
	cond := Or(
		ConditionFunc(func(_ context.Context) (bool, error) {
			return false, errors.New("error")
		}),
		Always(),
	)

	result, err := cond.Evaluate(ctx)
	require.NoError(t, err)
	assert.True(t, result)
}
