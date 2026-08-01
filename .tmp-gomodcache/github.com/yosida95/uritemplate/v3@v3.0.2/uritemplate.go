// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

import (
	"log"
	"regexp"
	"strings"
	"sync"
)

var (
	debug = debugT(false)
)

type debugT bool

func (t debugT) Printf(format string, v ...interface{}) {
	if t {
		log.Printf(format, v...)
	}
}

// Template represents a URI Template.
type Template struct {
	raw   string
	exprs []template

	// protects the rest of fields
	mu       sync.Mutex
	varnames []string
	re       *regexp.Regexp
	prog     *prog
}

// New parses and constructs a new Template instance based on the template.
// New returns an error if the template cannot be recognized.
func New(template string) (*Template, error) {
	return (&parser{r: template}).parseURITemplate()
}

// MustNew panics if the template cannot be recognized.
func MustNew(template string) *Template {
	ret, err := New(template)
	if err != nil {
		panic(err)
	}
	return ret
}

// Raw returns a raw URI template passed to New in string.
func (t *Template) Raw() string {
	return t.raw
}

// Varnames returns variable names used in the template.
func (t *Template) Varnames() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.varnames != nil {
		return t.varnames
	}

	reg := map[string]struct{}{}
	t.varnames = []string{}
	for i := range t.exprs {
		expr, ok := t.exprs[i].(*expression)
		if !ok {
			continue
		}
		for _, spec := range expr.vars {
			if _, ok := reg[spec.name]; ok {
				continue
			}
			reg[spec.name] = struct{}{}
			t.varnames = append(t.varnames, spec.name)
		}
	}

	return t.varnames
}

// Expand returns a URI reference corresponding to the template expanded using the passed variables.
func (t *Template) Expand(vars Values) (string, error) {
	var w strings.Builder
	for i := range t.exprs {
		expr := t.exprs[i]
		if err := expr.expand(&w, vars); err != nil {
			return w.String(), err
		}
	}
	return w.String(), nil
}

// Regexp converts the template to regexp and returns compiled *regexp.Regexp.
func (t *Template) Regexp() *regexp.Regexp {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.re != nil {
		return t.re
	}

	var b strings.Builder
	b.WriteByte('^')
	for _, expr := range t.exprs {
		expr.regexp(&b)
	}
	b.WriteByte('$')
	t.re = regexp.MustCompile(b.String())

	return t.re
}
