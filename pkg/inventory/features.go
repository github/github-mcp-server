package inventory

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
)

// FeatureFlag identifies a feature consistently across inventory consumers.
type FeatureFlag string

// FeatureFlagChecker resolves one feature flag for the current request.
type FeatureFlagChecker func(ctx context.Context, flag FeatureFlag) (bool, error)

// FeatureResolver returns the resolved value of a feature flag.
// Implementations absorb resolution errors and fail closed.
type FeatureResolver func(flag FeatureFlag) bool

// FeaturePredicate determines whether an inventory item is available.
type FeaturePredicate func(featureAsBool FeatureResolver) bool

// FeatureRule declares the feature flags used by an availability predicate.
// The declaration lets the service resolve and deduplicate checks before the
// predicate runs, while the predicate retains normal Go boolean semantics.
type FeatureRule struct {
	features   []FeatureFlag
	featureSet map[FeatureFlag]struct{}
	predicate  FeaturePredicate
}

// NewFeatureRule creates an availability rule over the supplied feature flags.
func NewFeatureRule(features []FeatureFlag, predicate FeaturePredicate) FeatureRule {
	declared := make([]FeatureFlag, 0, len(features))
	featureSet := make(map[FeatureFlag]struct{}, len(features))
	for _, feature := range features {
		if feature == "" {
			continue
		}
		if _, ok := featureSet[feature]; ok {
			continue
		}
		featureSet[feature] = struct{}{}
		declared = append(declared, feature)
	}
	return FeatureRule{
		features:   declared,
		featureSet: featureSet,
		predicate:  predicate,
	}
}

// Features returns the feature flags referenced by the rule.
func (r FeatureRule) Features() []FeatureFlag {
	return append([]FeatureFlag(nil), r.features...)
}

// IsZero reports whether no feature availability rule is configured.
func (r FeatureRule) IsZero() bool {
	return r.predicate == nil
}

// Enabled evaluates the rule against resolved feature values.
func (r FeatureRule) Enabled(featureAsBool FeatureResolver) bool {
	if r.predicate == nil {
		return true
	}
	if featureAsBool == nil {
		return false
	}

	var undeclared FeatureFlag
	enabled := r.predicate(func(feature FeatureFlag) bool {
		if _, ok := r.featureSet[feature]; !ok {
			undeclared = feature
			return false
		}
		return featureAsBool(feature)
	})
	if undeclared != "" {
		fmt.Fprintf(os.Stderr, "Feature rule used undeclared feature %q\n", undeclared)
		return false
	}
	return enabled
}

type featureStateContextKey struct{}

type featureState struct {
	checker FeatureFlagChecker

	mu     sync.Mutex
	values map[FeatureFlag]bool
}

func newFeatureState(checker FeatureFlagChecker) *featureState {
	return &featureState{
		checker: checker,
		values:  make(map[FeatureFlag]bool),
	}
}

func (s *featureState) enabled(ctx context.Context, feature FeatureFlag) bool {
	if feature == "" || s.checker == nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if enabled, ok := s.values[feature]; ok {
		return enabled
	}

	enabled, err := s.checker(ctx, feature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", feature, err)
		enabled = false
	}
	s.values[feature] = enabled
	return enabled
}

// WithResolvedFeatures resolves the deduplicated feature names into state owned
// by the returned context. Repeated calls extend and reuse that state.
func WithResolvedFeatures(ctx context.Context, checker FeatureFlagChecker, features []FeatureFlag) context.Context {
	state, _ := ctx.Value(featureStateContextKey{}).(*featureState)
	if state == nil {
		if checker == nil {
			return ctx
		}
		state = newFeatureState(checker)
		ctx = context.WithValue(ctx, featureStateContextKey{}, state)
	}

	features = append([]FeatureFlag(nil), features...)
	slices.Sort(features)
	for _, feature := range features {
		state.enabled(ctx, feature)
	}
	return ctx
}

// ResolveFeature returns a feature value from request-owned resolution state.
// Features not resolved up front are resolved lazily and cached.
func ResolveFeature(ctx context.Context, checker FeatureFlagChecker, feature FeatureFlag) bool {
	if feature == "" {
		return false
	}
	if state, _ := ctx.Value(featureStateContextKey{}).(*featureState); state != nil {
		return state.enabled(ctx, feature)
	}
	if checker == nil {
		return false
	}
	return newFeatureState(checker).enabled(ctx, feature)
}

func featureResolver(ctx context.Context, checker FeatureFlagChecker) FeatureResolver {
	if state, _ := ctx.Value(featureStateContextKey{}).(*featureState); state != nil {
		return func(feature FeatureFlag) bool {
			return state.enabled(ctx, feature)
		}
	}
	if checker == nil {
		return func(FeatureFlag) bool { return false }
	}
	state := newFeatureState(checker)
	return func(feature FeatureFlag) bool {
		return state.enabled(ctx, feature)
	}
}

func collectFeatures(rules ...FeatureRule) []FeatureFlag {
	seen := make(map[FeatureFlag]struct{})
	for _, rule := range rules {
		for _, feature := range rule.features {
			seen[feature] = struct{}{}
		}
	}

	features := make([]FeatureFlag, 0, len(seen))
	for feature := range seen {
		features = append(features, feature)
	}
	slices.Sort(features)
	return features
}
