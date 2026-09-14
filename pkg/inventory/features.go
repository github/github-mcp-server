package inventory

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
)

const maxFeatureRuleFlags = 16

// FeatureFlag identifies a feature consistently across inventory consumers.
type FeatureFlag string

// FeatureFlagChecker resolves one feature flag for the current request. Checkers
// must not call ResolveFeature; nested resolution fails the owning check closed.
type FeatureFlagChecker func(ctx context.Context, flag string) (bool, error)

// FeatureResolver returns the resolved value of a feature flag.
// Implementations absorb resolution errors and fail closed.
type FeatureResolver func(flag FeatureFlag) bool

// FeaturePredicate determines whether an inventory item is available. Predicates
// must be pure: their result may depend only on calls to the supplied resolver.
type FeaturePredicate func(featureAsBool FeatureResolver) bool

// FeatureRule declares the feature flags used by an availability predicate.
// The predicate resolves reached flags lazily with normal Go boolean semantics,
// while request state deduplicates repeated checks.
type FeatureRule struct {
	features  []FeatureFlag
	predicate FeaturePredicate
}

// NewFeatureRule creates an availability rule over the supplied feature flags.
func NewFeatureRule(features []FeatureFlag, predicate FeaturePredicate) FeatureRule {
	declared := make([]FeatureFlag, 0, len(features))
	for _, feature := range features {
		if feature == "" {
			continue
		}
		if slices.Contains(declared, feature) {
			continue
		}
		declared = append(declared, feature)
	}
	rule := FeatureRule{
		features:  declared,
		predicate: predicate,
	}
	if len(declared) > 0 && predicate == nil {
		panic("feature rule declares flags without a predicate")
	}
	rule.validate()
	return rule
}

func (r FeatureRule) validate() {
	if r.predicate == nil {
		return
	}
	if len(r.features) > maxFeatureRuleFlags {
		panic(fmt.Sprintf("feature rule declares %d flags; maximum is %d", len(r.features), maxFeatureRuleFlags))
	}

	for assignment := range 1 << len(r.features) {
		r.evaluate(func(feature FeatureFlag) bool {
			for i, declared := range r.features {
				if feature == declared {
					return assignment&(1<<i) != 0
				}
			}
			return false
		})
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

func appendUniqueFeature(features []FeatureFlag, feature FeatureFlag) []FeatureFlag {
	if feature != "" && !slices.Contains(features, feature) {
		return append(features, feature)
	}
	return features
}

// Enabled evaluates the rule against resolved feature values.
func (r FeatureRule) Enabled(featureAsBool FeatureResolver) bool {
	if r.predicate == nil {
		return true
	}
	if featureAsBool == nil {
		return false
	}
	return r.predicate(featureAsBool)
}

func (r FeatureRule) evaluate(featureAsBool FeatureResolver) bool {
	var undeclared FeatureFlag
	usedUndeclared := false
	enabled := r.predicate(func(feature FeatureFlag) bool {
		if !slices.Contains(r.features, feature) {
			undeclared = feature
			usedUndeclared = true
			return false
		}
		return featureAsBool(feature)
	})
	if usedUndeclared {
		panic(fmt.Sprintf("feature rule used undeclared feature %q", undeclared))
	}
	return enabled
}

type featureStateContextKey struct{}
type resolvingFeatureContextKey struct{}

type featureState struct {
	checker FeatureFlagChecker

	mu      sync.Mutex
	cond    sync.Cond
	results map[FeatureFlag]*featureResult
}

type featureResult struct {
	enabled bool
	failed  bool
	done    bool
}

type resolvingFeature struct {
	flag FeatureFlag
}

func newFeatureState(checker FeatureFlagChecker) *featureState {
	state := &featureState{
		checker: checker,
	}
	state.cond.L = &state.mu
	return state
}

func (s *featureState) enabled(ctx context.Context, feature FeatureFlag) bool {
	if feature == "" || s.checker == nil {
		return false
	}

	owner := resolvingFeatureFromContext(ctx)
	if owner != nil {
		s.mu.Lock()
		if ownerResult := s.results[owner.flag]; ownerResult != nil {
			ownerResult.failed = true
		}
		s.mu.Unlock()
		fmt.Fprintf(os.Stderr, "Feature flag checker attempted nested resolution of %q\n", feature)
		return false
	}

	s.mu.Lock()
	if s.results == nil {
		s.results = make(map[FeatureFlag]*featureResult)
	}
	result, found := s.results[feature]
	if found {
		if result.done {
			enabled := result.enabled
			s.mu.Unlock()
			return enabled
		}
		for !result.done {
			s.cond.Wait()
		}
		enabled := result.enabled
		s.mu.Unlock()
		return enabled
	}
	result = &featureResult{}
	s.results[feature] = result
	s.mu.Unlock()

	completed := false
	defer func() {
		if !completed {
			s.mu.Lock()
			result.failed = true
			result.done = true
			s.cond.Broadcast()
			s.mu.Unlock()
		}
	}()

	resolutionCtx := context.WithValue(ctx, resolvingFeatureContextKey{}, &resolvingFeature{flag: feature})
	enabled, err := s.checker(resolutionCtx, string(feature))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Feature flag check error for %q: %v\n", feature, err)
		enabled = false
	}
	s.mu.Lock()
	if result.failed {
		enabled = false
	}
	result.enabled = enabled
	result.done = true
	completed = true
	s.cond.Broadcast()
	s.mu.Unlock()
	return enabled
}

func resolvingFeatureFromContext(ctx context.Context) *resolvingFeature {
	feature, _ := ctx.Value(resolvingFeatureContextKey{}).(*resolvingFeature)
	return feature
}

// WithFeatureState installs request-owned lazy feature state. When state already
// exists, its checker is authoritative and checker is ignored.
func WithFeatureState(ctx context.Context, checker FeatureFlagChecker) context.Context {
	state, _ := ctx.Value(featureStateContextKey{}).(*featureState)
	if state == nil {
		if checker == nil {
			return ctx
		}
		state = newFeatureState(checker)
		ctx = context.WithValue(ctx, featureStateContextKey{}, state)
	}
	return ctx
}

// ResolveFeature returns a feature value from request-owned resolution state.
// Context state and its checker are authoritative. fallbackChecker is used only
// when the context has no state; that uncached compatibility path lets handlers
// invoked directly outside a server continue to resolve features.
func ResolveFeature(ctx context.Context, fallbackChecker FeatureFlagChecker, feature FeatureFlag) bool {
	if feature == "" {
		return false
	}
	if state, _ := ctx.Value(featureStateContextKey{}).(*featureState); state != nil {
		return state.enabled(ctx, feature)
	}
	if fallbackChecker == nil {
		return false
	}
	return newFeatureState(fallbackChecker).enabled(ctx, feature)
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
