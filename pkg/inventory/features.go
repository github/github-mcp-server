package inventory

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"sync"
)

const maxFeatureRuleFlags = 16

// FeatureFlag identifies a feature consistently across inventory consumers.
type FeatureFlag string

// FeatureFlagChecker resolves one feature flag for the current request. Every
// context value needed for availability checks must be installed before the
// inventory is resolved. Handler-only checks receive the live tool-call context.
type FeatureFlagChecker func(ctx context.Context, flag FeatureFlag) (bool, error)

// FeatureResolver returns the resolved value of a feature flag.
// Implementations absorb resolution errors and fail closed.
type FeatureResolver func(flag FeatureFlag) bool

// FeaturePredicate determines whether an inventory item is available. Predicates
// must be pure: their result may depend only on calls to the supplied resolver.
type FeaturePredicate func(featureAsBool FeatureResolver) bool

// FeatureRule declares the feature flags used by an availability predicate.
// The declaration lets the service resolve and deduplicate checks before the
// predicate runs, while the predicate retains normal Go boolean semantics.
type FeatureRule struct {
	features   []FeatureFlag
	featureSet map[FeatureFlag]struct{}
	predicate  FeaturePredicate
}

func (r FeatureRule) clone() FeatureRule {
	return FeatureRule{
		features:   slices.Clone(r.features),
		featureSet: maps.Clone(r.featureSet),
		predicate:  r.predicate,
	}
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
	rule := FeatureRule{
		features:   declared,
		featureSet: featureSet,
		predicate:  predicate,
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

// Enabled evaluates the rule against resolved feature values.
func (r FeatureRule) Enabled(featureAsBool FeatureResolver) bool {
	if r.predicate == nil {
		return true
	}
	if featureAsBool == nil {
		return false
	}
	return r.evaluate(featureAsBool)
}

func (r FeatureRule) evaluate(featureAsBool FeatureResolver) bool {
	var undeclared FeatureFlag
	usedUndeclared := false
	enabled := r.predicate(func(feature FeatureFlag) bool {
		if _, ok := r.featureSet[feature]; !ok {
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
	results map[FeatureFlag]*featureResult
	waiting map[FeatureFlag]map[FeatureFlag]int
}

type featureResult struct {
	ready   chan struct{}
	enabled bool
	failed  bool
	done    bool
}

type resolvingFeature struct {
	flag FeatureFlag
}

func newFeatureState(checker FeatureFlagChecker) *featureState {
	return &featureState{
		checker: checker,
		results: make(map[FeatureFlag]*featureResult),
		waiting: make(map[FeatureFlag]map[FeatureFlag]int),
	}
}

func (s *featureState) enabled(ctx context.Context, feature FeatureFlag) bool {
	if feature == "" || s.checker == nil {
		return false
	}

	owner := resolvingFeatureFromContext(ctx)
	s.mu.Lock()
	result, found := s.results[feature]
	if found && result.done {
		enabled := result.enabled
		s.mu.Unlock()
		return enabled
	}
	if !found {
		result = &featureResult{ready: make(chan struct{})}
		s.results[feature] = result
	}
	if owner != nil {
		if path := s.pathLocked(feature, owner.flag, nil); path != nil {
			s.failCycleLocked(owner.flag, path)
			s.mu.Unlock()
			return false
		}
		if s.waiting[owner.flag] == nil {
			s.waiting[owner.flag] = make(map[FeatureFlag]int)
		}
		s.waiting[owner.flag][feature]++
	}
	s.mu.Unlock()

	if owner != nil {
		defer s.clearWait(owner.flag, feature)
	}
	if found {
		select {
		case <-result.ready:
			return result.enabled
		case <-ctx.Done():
			return false
		}
	}

	completed := false
	defer func() {
		if !completed {
			s.mu.Lock()
			result.failed = true
			result.done = true
			close(result.ready)
			s.mu.Unlock()
		}
	}()

	resolutionCtx := context.WithValue(ctx, resolvingFeatureContextKey{}, &resolvingFeature{flag: feature})
	enabled, err := s.checker(resolutionCtx, feature)
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
	close(result.ready)
	s.mu.Unlock()
	return enabled
}

func (s *featureState) pathLocked(current, target FeatureFlag, seen map[FeatureFlag]bool) []FeatureFlag {
	if current == target {
		return []FeatureFlag{current}
	}
	if seen == nil {
		seen = make(map[FeatureFlag]bool)
	}
	if seen[current] {
		return nil
	}
	seen[current] = true
	for next := range s.waiting[current] {
		if path := s.pathLocked(next, target, seen); path != nil {
			return append([]FeatureFlag{current}, path...)
		}
	}
	return nil
}

func (s *featureState) failCycleLocked(owner FeatureFlag, path []FeatureFlag) {
	if result := s.results[owner]; result != nil {
		result.failed = true
	}
	for _, feature := range path {
		if result := s.results[feature]; result != nil {
			result.failed = true
		}
	}
	fmt.Fprintf(os.Stderr, "Feature flag resolution cycle detected for %q\n", path[0])
}

func (s *featureState) clearWait(owner, target FeatureFlag) {
	s.mu.Lock()
	if targets := s.waiting[owner]; targets != nil {
		targets[target]--
		if targets[target] == 0 {
			delete(targets, target)
		}
	}
	if len(s.waiting[owner]) == 0 {
		delete(s.waiting, owner)
	}
	s.mu.Unlock()
}

func resolvingFeatureFromContext(ctx context.Context) *resolvingFeature {
	feature, _ := ctx.Value(resolvingFeatureContextKey{}).(*resolvingFeature)
	return feature
}

// WithResolvedFeatures resolves the deduplicated feature names into state owned
// by the returned context. Repeated calls extend and reuse that state. When state
// already exists, its checker is authoritative and checker is ignored.
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
