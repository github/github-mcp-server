package inventory

import (
	"context"
)

// EnableCondition represents a composable condition for tool availability.
// Conditions can be combined using And/Or/Not combinators for complex logic.
//
// Design goals:
//   - Declarative: users compose conditions without knowing implementation details
//   - Composable: complex conditions built from simple primitives
//   - Efficient: conditions are evaluated lazily, with results potentially cached
//   - Decoupled: condition definitions don't depend on specific actor types
//
// Example usage:
//
//	// Simple feature flag
//	tool.EnableCondition = FeatureFlag("web_search")
//
//	// Feature flag AND user policy
//	tool.EnableCondition = And(
//	    FeatureFlag("web_search"),
//	    ContextBool("user_has_paid_bing_access"),
//	)
//
//	// CCA bypass (CCA requests OR feature flag for non-CCA)
//	tool.EnableCondition = Or(
//	    ContextBool("is_cca"),
//	    FeatureFlag("agent_search"),
//	)
type EnableCondition interface {
	// Evaluate checks if the condition is met in the given context.
	// Returns (enabled, error). On error, the condition should be treated as false.
	Evaluate(ctx context.Context) (bool, error)
}

// ConditionFunc is an adapter that allows functions to be used as EnableConditions.
// This is useful for simple one-off conditions that don't need to be reusable.
type ConditionFunc func(ctx context.Context) (bool, error)

// Evaluate implements EnableCondition.
func (f ConditionFunc) Evaluate(ctx context.Context) (bool, error) {
	return f(ctx)
}

// --- Primitive Conditions ---

// featureFlagCondition checks if a named feature flag is enabled.
// The actual flag checking is delegated to a FeatureFlagChecker in context.
type featureFlagCondition struct {
	flagName string
}

// FeatureFlag creates a condition that checks if the named feature flag is enabled.
// The feature flag is evaluated using the FeatureFlagChecker stored in context.
// If no checker is available or if the flag check returns an error, the condition is false.
func FeatureFlag(flagName string) EnableCondition {
	return &featureFlagCondition{flagName: flagName}
}

// Evaluate implements EnableCondition.
func (c *featureFlagCondition) Evaluate(ctx context.Context) (bool, error) {
	checker := FeatureCheckerFromContext(ctx)
	if checker == nil {
		return false, nil
	}
	return checker(ctx, c.flagName)
}

// contextBoolCondition checks a named boolean value from context.
// This allows tools to depend on pre-computed boolean conditions without
// knowing how those conditions are computed.
type contextBoolCondition struct {
	key string
}

// ContextBool creates a condition that checks a named boolean from context.
// The boolean is retrieved using ContextBoolFromContext(ctx, key).
// This decouples tool definitions from specific actor/user types.
//
// Common keys might include:
//   - "is_cca" - whether this is a Copilot Coding Agent request
//   - "user_has_paid_access" - whether user has paid Copilot access
//   - "mcp_host_is_copilot_chat" - whether MCP host is copilot-chat
//
// Returns false if the key is not found in context.
func ContextBool(key string) EnableCondition {
	return &contextBoolCondition{key: key}
}

// Evaluate implements EnableCondition.
func (c *contextBoolCondition) Evaluate(ctx context.Context) (bool, error) {
	return ContextBoolFromContext(ctx, c.key), nil
}

// staticCondition always returns a fixed value.
type staticCondition struct {
	value bool
}

// Static creates a condition that always returns the given value.
// Useful for testing or for conditions that are determined at build time.
func Static(value bool) EnableCondition {
	return &staticCondition{value: value}
}

// Always returns a condition that is always true.
// Useful as a default or placeholder.
func Always() EnableCondition {
	return Static(true)
}

// Never returns a condition that is always false.
// Useful for disabling tools unconditionally.
func Never() EnableCondition {
	return Static(false)
}

// Evaluate implements EnableCondition.
func (c *staticCondition) Evaluate(_ context.Context) (bool, error) {
	return c.value, nil
}

// --- Combinators ---

// andCondition requires all conditions to be true.
type andCondition struct {
	conditions []EnableCondition
}

// And creates a condition that is true only if ALL of the given conditions are true.
// Short-circuits on the first false condition.
// Returns true if no conditions are provided.
func And(conditions ...EnableCondition) EnableCondition {
	// Filter out nil conditions
	filtered := make([]EnableCondition, 0, len(conditions))
	for _, c := range conditions {
		if c != nil {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return Always()
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return &andCondition{conditions: filtered}
}

// Evaluate implements EnableCondition.
func (c *andCondition) Evaluate(ctx context.Context) (bool, error) {
	for _, cond := range c.conditions {
		enabled, err := cond.Evaluate(ctx)
		if err != nil {
			return false, err
		}
		if !enabled {
			return false, nil
		}
	}
	return true, nil
}

// orCondition requires at least one condition to be true.
type orCondition struct {
	conditions []EnableCondition
}

// Or creates a condition that is true if ANY of the given conditions is true.
// Short-circuits on the first true condition.
// Returns false if no conditions are provided.
func Or(conditions ...EnableCondition) EnableCondition {
	// Filter out nil conditions
	filtered := make([]EnableCondition, 0, len(conditions))
	for _, c := range conditions {
		if c != nil {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return Never()
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return &orCondition{conditions: filtered}
}

// Evaluate implements EnableCondition.
func (c *orCondition) Evaluate(ctx context.Context) (bool, error) {
	for _, cond := range c.conditions {
		enabled, err := cond.Evaluate(ctx)
		if err != nil {
			// For OR, we continue checking other conditions on error
			continue
		}
		if enabled {
			return true, nil
		}
	}
	return false, nil
}

// notCondition negates a condition.
type notCondition struct {
	condition EnableCondition
}

// Not creates a condition that is the logical negation of the given condition.
// Returns true if the inner condition returns false (and vice versa).
// Errors are propagated.
func Not(condition EnableCondition) EnableCondition {
	if condition == nil {
		return Never() // Not(nil) = Not(true) = false
	}
	return &notCondition{condition: condition}
}

// Evaluate implements EnableCondition.
func (c *notCondition) Evaluate(ctx context.Context) (bool, error) {
	enabled, err := c.condition.Evaluate(ctx)
	if err != nil {
		return false, err
	}
	return !enabled, nil
}

// --- Context Keys for Conditions ---

// Context key types for storing condition-related data
type contextKey int

const (
	featureCheckerKey contextKey = iota
	contextBoolsKey
)

// ContextWithFeatureChecker returns a context with the given feature flag checker.
func ContextWithFeatureChecker(ctx context.Context, checker FeatureFlagChecker) context.Context {
	return context.WithValue(ctx, featureCheckerKey, checker)
}

// FeatureCheckerFromContext retrieves the feature flag checker from context.
// Returns nil if no checker is set.
func FeatureCheckerFromContext(ctx context.Context) FeatureFlagChecker {
	checker, _ := ctx.Value(featureCheckerKey).(FeatureFlagChecker)
	return checker
}

// ContextBools is a map of named boolean values for use with ContextBool conditions.
// This allows callers to pre-compute common checks once per request and share them.
type ContextBools map[string]bool

// ContextWithBools returns a context with the given boolean values.
// These values can be retrieved using ContextBool conditions.
func ContextWithBools(ctx context.Context, bools ContextBools) context.Context {
	// Merge with existing bools if any
	existing := contextBoolsFromContext(ctx)
	if existing != nil {
		merged := make(ContextBools, len(existing)+len(bools))
		for k, v := range existing {
			merged[k] = v
		}
		for k, v := range bools {
			merged[k] = v
		}
		return context.WithValue(ctx, contextBoolsKey, merged)
	}
	return context.WithValue(ctx, contextBoolsKey, bools)
}

// contextBoolsFromContext retrieves all context bools.
func contextBoolsFromContext(ctx context.Context) ContextBools {
	bools, _ := ctx.Value(contextBoolsKey).(ContextBools)
	return bools
}

// ContextBoolFromContext retrieves a named boolean from context.
// Returns false if the key is not found.
func ContextBoolFromContext(ctx context.Context, key string) bool {
	bools := contextBoolsFromContext(ctx)
	if bools == nil {
		return false
	}
	return bools[key]
}

// SetContextBool is a convenience function that adds a single boolean to context.
func SetContextBool(ctx context.Context, key string, value bool) context.Context {
	return ContextWithBools(ctx, ContextBools{key: value})
}
