// This code is based on https://github.com/pmezard/go-difflib
//
// Copyright (c) 2013, Patrick Mezard
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     Redistributions in binary form must reproduce the above copyright
// notice, this list of conditions and the following disclaimer in the
// documentation and/or other materials provided with the distribution.
//     The names of its contributors may not be used to endorse or promote
// products derived from this software without specific prior written
// permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS
// IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED
// TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A
// PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED
// TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
// LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
// NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Package ztest is a copy of https://github.com/arp242/zstd/tree/master/ztest –
// vendored here so we don't add a dependency for just one file used in tests.
//
// DiffXML was removed as it depends on zgo.at/zstd/zxml.
package ztest

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type DiffOpt int

const (
	// Normalize whitespace: remove all whitespace at the start and end of every
	// line.
	DiffNormalizeWhitespace DiffOpt = iota + 1

	// Treat arguments as JSON: format them before diffing.
	DiffJSON
)

// Diff two strings and format as a unified diff.
func Diff(have, want string, opt ...DiffOpt) string {
	have, want = applyOpt(have, want, opt...)

	d := makeUnifiedDiff(unifiedDiff{
		A:       splitLines(strings.TrimSpace(have)),
		B:       splitLines(strings.TrimSpace(want)),
		Context: 3,
	})
	if len(d) == 0 {
		return ""
	}
	return "\n" + d
}

// DiffMatch formats a unified diff, but accepts various patterns in the want
// string:
//
//	%(YEAR)      current year in UTC
//	%(MONTH)     current month in UTC
//	%(DAY)       current day in UTC
//	%(UUID)      UUID format (any version).
//
//	%(ANY)       any text: .+?
//	%(ANY 5)     any text of exactly 5 characters: .{5}?
//	%(ANY 5,)    any text of at least 5 characters: .{5,}?
//	%(ANY 5,10)  any text between 5 and 10 characters: .{5,10}?
//	%(ANY 10)    any text at most 10 characters: .{,10}?
//	%(NUMBER)    any number; also allows length like ANY.
//
//	%(..)        any regular expression, but \ is not allowed.
func DiffMatch(have, want string, opt ...DiffOpt) string {
	// TODO: %(..) syntax is somewhat unfortunate, as it conflicts with fmt
	// formatting strings. Would be better to use $(..), #(..), @(..), or
	// anything else really.
	have, want = applyOpt(have, want, opt...)

	now := time.Now().UTC()
	r := strings.NewReplacer(
		`%(YEAR)`, fmt.Sprintf("%d", now.Year()),
		`%(MONTH)`, fmt.Sprintf("%02d", now.Month()),
		`%(DAY)`, fmt.Sprintf("%02d", now.Day()),
	)

	wantRe := regexp.MustCompile(`%\\\(.+?\\\)`).ReplaceAllStringFunc(
		regexp.QuoteMeta(r.Replace(want)),
		func(m string) string {
			switch {
			case m == `%\(UUID\)`:
				return `[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12}`
			case m == `%\(ANY\)`:
				return `.+?`
			case m == `%\(NUMBER\)`:
				return `\d+?`
			case strings.HasPrefix(m, `%\(ANY `):
				return fmt.Sprintf(`.{%s}?`, m[7:len(m)-2])
			case strings.HasPrefix(m, `%\(NUMBER `):
				return fmt.Sprintf(`\d{%s}?`, m[10:len(m)-2])
			default:
				// TODO: we need to undo the \ from QuoteMeta() here, but this
				// means we won't be allowed to use \. Be a bit smarter about
				// this. TODO: doesn't quite seem to work?
				return strings.ReplaceAll(m[3:len(m)-2], `\`, ``)
			}
		})

	// Quick check for exact match.
	if m := regexp.MustCompile(`^` + wantRe + `$`).MatchString(have); m {
		return ""
	}

	diff := unifiedDiff{
		A:       splitLines(strings.TrimSpace(have)),
		B:       splitLines(strings.TrimSpace(wantRe)),
		Context: 3,
	}
	m := newMatcher(diff.A, diff.B)
	m.cmp = func(a, b string) bool { return regexp.MustCompile(b).MatchString(a) }
	diff.Matcher = m

	d := makeUnifiedDiff(diff)
	if len(d) == 0 {
		return "ztest.DiffMatch: strings didn't match but no diff?" // Should never happen.
	}
	return "\n" + d
}

var (
	reNormalizeWhitespace     *regexp.Regexp
	reNormalizeWhitespaceOnce sync.Once
)

func applyOpt(have, want string, opt ...DiffOpt) (string, string) {
	for _, o := range opt {
		switch o {
		case DiffNormalizeWhitespace:
			reNormalizeWhitespaceOnce.Do(func() {
				reNormalizeWhitespace = regexp.MustCompile(`(?m)(^\s+|\s+$)`)
			})
			have = reNormalizeWhitespace.ReplaceAllString(have, "")
			want = reNormalizeWhitespace.ReplaceAllString(want, "")
		case DiffJSON:
			if have == "" {
				have = "{}"
			}
			if want == "" {
				want = "{}"
			}

			var h interface{}
			haveJ, err := indentJSON([]byte(have), &h, "", "    ")
			if err != nil {
				have = fmt.Sprintf("ztest.Diff: ERROR formatting have: %s\ntext: %s", err, have)
			} else {
				have = string(haveJ)
			}
			var w interface{}
			wantJ, err := indentJSON([]byte(want), &w, "", "    ")
			if err != nil {
				want = fmt.Sprintf("ztest.Diff: ERROR formatting want: %s\ntext: %s", err, want)
			} else {
				want = string(wantJ)
			}
		}
	}
	return have, want
}

type match struct{ A, B, Size int }

type opCode struct {
	Tag            byte
	I1, I2, J1, J2 int
}

// sequenceMatcher compares sequence of strings. The basic
// algorithm predates, and is a little fancier than, an algorithm
// published in the late 1980's by Ratcliff and Obershelp under the
// hyperbolic name "gestalt pattern matching".  The basic idea is to find
// the longest contiguous matching subsequence.
//
// Timing:  Basic R-O is cubic time worst case and quadratic time expected
// case.  sequenceMatcher is quadratic time for the worst case and has
// expected-case behavior dependent in a complicated way on how many
// elements the sequences have in common; best case time is linear.
type sequenceMatcher struct {
	a, b []string
	cmp  func(a, b string) bool
}

func newMatcher(a, b []string) *sequenceMatcher {
	return &sequenceMatcher{
		a:   a,
		b:   b,
		cmp: func(a, b string) bool { return a == b },
	}
}

// Find longest matching block in a[alo:ahi] and b[blo:bhi].
//
// Return (i,j,k) such that a[i:i+k] is equal to b[j:j+k], where
//
//	alo <= i <= i+k <= ahi
//	blo <= j <= j+k <= bhi
//
// and for all (i',j',k') meeting those conditions,
//
//	k >= k'
//	i <= i'
//	and if i == i', j <= j'
//
// In other words, of all maximal matching blocks, return one that
// starts earliest in a, and of all those maximal matching blocks that
// start earliest in a, return the one that starts earliest in b.
//
// If no blocks match, return (alo, blo, 0).
func (m *sequenceMatcher) findLongestMatch(alo, ahi, blo, bhi int) match {
	// Populate line -> index mapping
	b2j := make(map[string][]int)
	for i, s := range m.b {
		b2j[s] = append(b2j[s], i)
	}

	// CAUTION:  stripping common prefix or suffix would be incorrect.
	// E.g.,
	//    ab
	//    acab
	// Longest matching block is "ab", but if common prefix is
	// stripped, it's "a" (tied with "b").  UNIX(tm) diff does so
	// strip, so ends up claiming that ab is changed to acab by
	// inserting "ca" in the middle.  That's minimal but unintuitive:
	// "it's obvious" that someone inserted "ac" at the front.
	// Windiff ends up at the same place as diff, but by pairing up
	// the unique 'b's and then matching the first two 'a's.
	besti, bestj, bestsize := alo, blo, 0

	// find longest match. During an iteration of the loop, j2len[j] = length of
	// longest match ending with a[i-1] and b[j]
	j2len := map[int]int{}
	for i := alo; i != ahi; i++ {
		// look at all instances of a[i] in b.
		newj2len := map[int]int{}
		for _, j := range b2j[m.a[i]] {
			// a[i] matches b[j]
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			k := j2len[j-1] + 1
			newj2len[j] = k
			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}
		j2len = newj2len
	}

	// Extend the best by elements on each end.  In particular, "popular"
	// elements aren't in b2j, which greatly speeds the inner loop above.
	for besti > alo && bestj > blo && m.cmp(m.a[besti-1], m.b[bestj-1]) {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && m.cmp(m.a[besti+bestsize], m.b[bestj+bestsize]) {
		bestsize += 1
	}

	return match{A: besti, B: bestj, Size: bestsize}
}

// Return list of triples describing matching subsequences.
//
// Each triple is of the form (i, j, n), and means that
// a[i:i+n] == b[j:j+n].  The triples are monotonically increasing in
// i and in j. It's also guaranteed that if (i, j, n) and (i', j', n') are
// adjacent triples in the list, and the second is not the last triple in the
// list, then i+n != i' or j+n != j'. IOW, adjacent triples never describe
// adjacent equal blocks.
//
// The last triple is a dummy, (len(a), len(b), 0), and is the only
// triple with n==0.
func (m *sequenceMatcher) matchingBlocks() []match {
	var matchBlocks func(alo, ahi, blo, bhi int, matched []match) []match
	matchBlocks = func(alo, ahi, blo, bhi int, matched []match) []match {
		match := m.findLongestMatch(alo, ahi, blo, bhi)
		i, j, k := match.A, match.B, match.Size
		if match.Size > 0 {
			if alo < i && blo < j {
				matched = matchBlocks(alo, i, blo, j, matched)
			}
			matched = append(matched, match)
			if i+k < ahi && j+k < bhi {
				matched = matchBlocks(i+k, ahi, j+k, bhi, matched)
			}
		}
		return matched
	}
	matched := matchBlocks(0, len(m.a), 0, len(m.b), nil)

	// It's possible that we have adjacent equal blocks in the
	// matching_blocks list now.
	nonAdjacent := []match{}
	i1, j1, k1 := 0, 0, 0
	for _, b := range matched {
		// Is this block adjacent to i1, j1, k1?
		i2, j2, k2 := b.A, b.B, b.Size
		if i1+k1 == i2 && j1+k1 == j2 {
			// Yes, so collapse them -- this just increases the length of
			// the first block by the length of the second, and the first
			// block so lengthened remains the block to compare against.
			k1 += k2
		} else {
			// Not adjacent.  Remember the first block (k1==0 means it's
			// the dummy we started with), and make the second block the
			// new block to compare against.
			if k1 > 0 {
				nonAdjacent = append(nonAdjacent, match{i1, j1, k1})
			}
			i1, j1, k1 = i2, j2, k2
		}
	}
	if k1 > 0 {
		nonAdjacent = append(nonAdjacent, match{i1, j1, k1})
	}

	return append(nonAdjacent, match{len(m.a), len(m.b), 0})
}

// Return list of 5-tuples describing how to turn a into b.
//
// Each tuple is of the form (tag, i1, i2, j1, j2).  The first tuple
// has i1 == j1 == 0, and remaining tuples have i1 == the i2 from the
// tuple preceding it, and likewise for j1 == the previous j2.
//
// The tags are characters, with these meanings:
//
// 'r' (replace):  a[i1:i2] should be replaced by b[j1:j2]
//
// 'd' (delete):   a[i1:i2] should be deleted, j1==j2 in this case.
//
// 'i' (insert):   b[j1:j2] should be inserted at a[i1:i1], i1==i2 in this case.
//
// 'e' (equal):    a[i1:i2] == b[j1:j2]
func (m *sequenceMatcher) GetOpCodes() []opCode {
	matching := m.matchingBlocks()
	opCodes := make([]opCode, 0, len(matching))

	var i, j int
	for _, m := range matching {
		//  invariant:  we've pumped out correct diffs to change
		//  a[:i] into b[:j], and the next matching block is
		//  a[ai:ai+size] == b[bj:bj+size]. So we need to pump
		//  out a diff to change a[i:ai] into b[j:bj], pump out
		//  the matching block, and move (i,j) beyond the match
		ai, bj, size := m.A, m.B, m.Size
		tag := byte(0)
		if i < ai && j < bj {
			tag = 'r'
		} else if i < ai {
			tag = 'd'
		} else if j < bj {
			tag = 'i'
		}
		if tag > 0 {
			opCodes = append(opCodes, opCode{tag, i, ai, j, bj})
		}

		i, j = ai+size, bj+size
		// the list of matching blocks is terminated by a
		// sentinel with size 0
		if size > 0 {
			opCodes = append(opCodes, opCode{'e', ai, i, bj, j})
		}
	}

	return opCodes
}

// Isolate change clusters by eliminating ranges with no changes.
//
// Return a generator of groups with up to n lines of context.
// Each group is in the same format as returned by GetOpCodes().
func (m *sequenceMatcher) GetGroupedOpCodes(n int) [][]opCode {
	if n < 0 {
		n = 3
	}
	codes := m.GetOpCodes()
	if len(codes) == 0 {
		codes = []opCode{opCode{'e', 0, 1, 0, 1}}
	}

	// Fixup leading and trailing groups if they show no changes.
	if codes[0].Tag == 'e' {
		c := codes[0]
		i1, i2, j1, j2 := c.I1, c.I2, c.J1, c.J2
		codes[0] = opCode{c.Tag, max(i1, i2-n), i2, max(j1, j2-n), j2}
	}

	if codes[len(codes)-1].Tag == 'e' {
		c := codes[len(codes)-1]
		i1, i2, j1, j2 := c.I1, c.I2, c.J1, c.J2
		codes[len(codes)-1] = opCode{c.Tag, i1, min(i2, i1+n), j1, min(j2, j1+n)}
	}

	nn := n + n
	groups := [][]opCode{}
	group := []opCode{}
	for _, c := range codes {
		i1, i2, j1, j2 := c.I1, c.I2, c.J1, c.J2
		// End the current group and start a new one whenever
		// there is a large range with no changes.
		if c.Tag == 'e' && i2-i1 > nn {
			group = append(group, opCode{c.Tag, i1, min(i2, i1+n),
				j1, min(j2, j1+n)})
			groups = append(groups, group)
			group = []opCode{}
			i1, j1 = max(i1, i2-n), max(j1, j2-n)
		}
		group = append(group, opCode{c.Tag, i1, i2, j1, j2})
	}

	if len(group) > 0 && !(len(group) == 1 && group[0].Tag == 'e') {
		groups = append(groups, group)
	}
	return groups
}

// Convert range to the "ed" format
func formatRangeUnified(start, stop int) string {
	// Per the diff spec at http://www.unix.org/single_unix_specification/
	beginning := start + 1 // lines start numbering with one
	length := stop - start
	if length == 1 {
		return fmt.Sprintf("%d", beginning)
	}

	if length == 0 {
		beginning -= 1 // empty ranges begin at line just before the range
	}
	return fmt.Sprintf("%d,%d", beginning, length)
}

// Unified diff parameters
type unifiedDiff struct {
	A, B    []string
	Context int
	Matcher *sequenceMatcher
}

// Compare two sequences of lines; generate the delta as a unified diff.
//
// Unified diffs are a compact way of showing line changes and a few
// lines of context.  The number of context lines is set by 'n' which
// defaults to three.
//
// By default, the diff control lines (those with ---, +++, or @@) are
// created with a trailing newline.  This is helpful so that inputs
// created from file.readlines() result in diffs that are suitable for
// file.writelines() since both the inputs and outputs have trailing
// newlines.
//
// For inputs that do not have trailing newlines, set the lineterm
// argument to "" so that the output will be uniformly newline free.
//
// The unidiff format normally has a header for filenames and modification
// times.  Any or all of these may be specified using strings for
// 'fromfile', 'tofile', 'fromfiledate', and 'tofiledate'.
// The modification times are normally expressed in the ISO 8601 format.
func makeUnifiedDiff(diff unifiedDiff) string {
	if diff.Matcher == nil {
		diff.Matcher = newMatcher(diff.A, diff.B)
	}

	var (
		out     strings.Builder
		started bool
	)
	for _, g := range diff.Matcher.GetGroupedOpCodes(diff.Context) {
		if !started {
			started = true
			out.WriteString("--- have\n")
			out.WriteString("+++ want\n")
		}

		first, last := g[0], g[len(g)-1]
		out.WriteString(fmt.Sprintf("@@ -%s +%s @@\n",
			formatRangeUnified(first.I1, last.I2),
			formatRangeUnified(first.J1, last.J2)))

		for _, c := range g {
			i1, i2, j1, j2 := c.I1, c.I2, c.J1, c.J2
			if c.Tag == 'e' {
				for _, line := range diff.A[i1:i2] {
					out.WriteString("      " + line)
				}
				continue
			}

			if c.Tag == 'r' || c.Tag == 'd' {
				for _, line := range diff.A[i1:i2] {
					out.WriteString("-have " + line)
				}
			}

			if c.Tag == 'r' || c.Tag == 'i' {
				for _, line := range diff.B[j1:j2] {
					out.WriteString("+want " + line)
				}
			}
		}
	}

	return out.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func splitLines(s string) []string {
	lines := strings.SplitAfter(s, "\n")
	lines[len(lines)-1] += "\n"
	return lines
}

func indentJSON(data []byte, v interface{}, prefix, indent string) ([]byte, error) {
	err := json.Unmarshal(data, v)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, prefix, indent)
}
