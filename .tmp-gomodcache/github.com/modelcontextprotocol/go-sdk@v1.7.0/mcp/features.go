// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"iter"
	"maps"
	"slices"
)

// This file contains implementations that are common to all features.
// A feature is an item provided to a peer. In the 2025-03-26 spec,
// the features are prompt, tool, resource and root.

// A featureSet is a collection of features of type T.
// Every feature has a unique ID, and the spec never mentions
// an ordering for the List calls, so what it calls a "list" is actually a set.
//
// An alternative implementation would use an ordered map, but that's probably
// not necessary as adds and removes are rare, and usually batched.
type featureSet[T any] struct {
	uniqueID   func(T) string
	features   map[string]T
	sortedKeys []string // lazily computed; nil after add or remove
}

// newFeatureSet creates a new featureSet for features of type T.
// The argument function should return the unique ID for a single feature.
func newFeatureSet[T any](uniqueIDFunc func(T) string) *featureSet[T] {
	return &featureSet[T]{
		uniqueID: uniqueIDFunc,
		features: make(map[string]T),
	}
}

// add adds each feature to the set if it is not present,
// or replaces an existing feature.
func (s *featureSet[T]) add(fs ...T) {
	for _, f := range fs {
		s.features[s.uniqueID(f)] = f
	}
	s.sortedKeys = nil
}

// remove removes all features with the given uids from the set if present,
// and returns whether any were removed.
// It is not an error to remove a nonexistent feature.
func (s *featureSet[T]) remove(uids ...string) bool {
	changed := false
	for _, uid := range uids {
		if _, ok := s.features[uid]; ok {
			changed = true
			delete(s.features, uid)
		}
	}
	if changed {
		s.sortedKeys = nil
	}
	return changed
}

// get returns the feature with the given uid.
// If there is none, it returns zero, false.
func (s *featureSet[T]) get(uid string) (T, bool) {
	t, ok := s.features[uid]
	return t, ok
}

// len returns the number of features in the set.
func (s *featureSet[T]) len() int { return len(s.features) }

// all returns an iterator over of all the features in the set
// sorted by unique ID.
func (s *featureSet[T]) all() iter.Seq[T] {
	s.sortKeys()
	return func(yield func(T) bool) {
		s.yieldFrom(0, yield)
	}
}

// above returns an iterator over features in the set whose unique IDs are
// greater than `uid`, in ascending ID order.
func (s *featureSet[T]) above(uid string) iter.Seq[T] {
	s.sortKeys()
	index, found := slices.BinarySearch(s.sortedKeys, uid)
	if found {
		index++
	}
	return func(yield func(T) bool) {
		s.yieldFrom(index, yield)
	}
}

// sortKeys is a helper that maintains a sorted list of feature IDs. It
// computes this list lazily upon its first call after a modification, or
// if it's nil.
func (s *featureSet[T]) sortKeys() {
	if s.sortedKeys != nil {
		return
	}
	s.sortedKeys = slices.Sorted(maps.Keys(s.features))
}

// yieldFrom is a helper that iterates over the features in the set,
// starting at the given index, and calls the yield function for each one.
func (s *featureSet[T]) yieldFrom(index int, yield func(T) bool) {
	for i := index; i < len(s.sortedKeys); i++ {
		if !yield(s.features[s.sortedKeys[i]]) {
			return
		}
	}
}
