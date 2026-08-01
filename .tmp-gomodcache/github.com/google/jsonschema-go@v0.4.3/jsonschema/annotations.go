// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema

import "maps"

// An annotations tracks certain properties computed by keywords that are used by validation.
// ("Annotation" is the spec's term.)
// In particular, the unevaluatedItems and unevaluatedProperties keywords need to know which
// items and properties were evaluated (validated successfully).
type annotations struct {
	allItems            bool            // all items were evaluated
	endIndex            int             // 1+largest index evaluated by prefixItems
	evaluatedIndexes    map[int]bool    // set of indexes evaluated by contains
	allProperties       bool            // all properties were evaluated
	evaluatedProperties map[string]bool // set of properties evaluated by various keywords
}

// noteIndex marks i as evaluated.
func (a *annotations) noteIndex(i int) {
	if a.evaluatedIndexes == nil {
		a.evaluatedIndexes = map[int]bool{}
	}
	a.evaluatedIndexes[i] = true
}

// noteEndIndex marks items with index less than end as evaluated.
func (a *annotations) noteEndIndex(end int) {
	if end > a.endIndex {
		a.endIndex = end
	}
}

// noteProperty marks prop as evaluated.
func (a *annotations) noteProperty(prop string) {
	if a.evaluatedProperties == nil {
		a.evaluatedProperties = map[string]bool{}
	}
	a.evaluatedProperties[prop] = true
}

// noteProperties marks all the properties in props as evaluated.
func (a *annotations) noteProperties(props map[string]bool) {
	a.evaluatedProperties = merge(a.evaluatedProperties, props)
}

// merge adds b's annotations to a.
// a must not be nil.
func (a *annotations) merge(b *annotations) {
	if b == nil {
		return
	}
	if b.allItems {
		a.allItems = true
	}
	if b.endIndex > a.endIndex {
		a.endIndex = b.endIndex
	}
	a.evaluatedIndexes = merge(a.evaluatedIndexes, b.evaluatedIndexes)
	if b.allProperties {
		a.allProperties = true
	}
	a.evaluatedProperties = merge(a.evaluatedProperties, b.evaluatedProperties)
}

// merge adds t's keys to s and returns s.
// If s is nil, it returns a copy of t.
func merge[K comparable](s, t map[K]bool) map[K]bool {
	if s == nil {
		return maps.Clone(t)
	}
	maps.Copy(s, t)
	return s
}
