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

package bluemonday_test

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

func Example() {
	// Create a new policy
	p := bluemonday.NewPolicy()

	// Add elements to a policy without attributes
	p.AllowElements("b", "strong")

	// Add elements as a virtue of adding an attribute
	p.AllowAttrs("nowrap").OnElements("td", "th")

	// Attributes can either be added to all elements
	p.AllowAttrs("dir").Globally()

	//Or attributes can be added to specific elements
	p.AllowAttrs("value").OnElements("li")

	// It is ALWAYS recommended that an attribute be made to match a pattern
	// XSS in HTML attributes is a very easy attack vector

	// \p{L} matches unicode letters, \p{N} matches unicode numbers
	p.AllowAttrs("title").Matching(regexp.MustCompile(`[\p{L}\p{N}\s\-_',:\[\]!\./\\\(\)&]*`)).Globally()

	// You can stop at any time and call .Sanitize()

	// Assumes that string htmlIn was passed in from a HTTP POST and contains
	// untrusted user generated content
	htmlIn := `untrusted user generated content <body onload="alert('XSS')">`
	fmt.Println(p.Sanitize(htmlIn))

	// And you can take any existing policy and extend it
	p = bluemonday.UGCPolicy()
	p.AllowElements("fieldset", "select", "option")

	// Links are complex beasts and one of the biggest attack vectors for
	// malicious content so we have included features specifically to help here.

	// This is not recommended:
	p = bluemonday.NewPolicy()
	p.AllowAttrs("href").Matching(regexp.MustCompile(`(?i)mailto|https?`)).OnElements("a")

	// The regexp is insufficient in this case to have prevented a malformed
	// value doing something unexpected.

	// This will ensure that URLs are not considered invalid by Go's net/url
	// package.
	p.RequireParseableURLs(true)

	// If you have enabled parseable URLs then the following option will allow
	// relative URLs. By default this is disabled and will prevent all local and
	// schema relative URLs (i.e. `href="//www.google.com"` is schema relative).
	p.AllowRelativeURLs(true)

	// If you have enabled parseable URLs then you can allow the schemas
	// that are permitted. Bear in mind that allowing relative URLs in the above
	// option allows for blank schemas.
	p.AllowURLSchemes("mailto", "http", "https")

	// Regardless of whether you have enabled parseable URLs, you can force all
	// URLs to have a rel="nofollow" attribute. This will be added if it does
	// not exist.

	// This applies to "a" "area" "link" elements that have a "href" attribute
	p.RequireNoFollowOnLinks(true)

	// We provide a convenience function that applies all of the above, but you
	// will still need to allow the linkable elements:
	p = bluemonday.NewPolicy()
	p.AllowStandardURLs()
	p.AllowAttrs("cite").OnElements("blockquote")
	p.AllowAttrs("href").OnElements("a", "area")
	p.AllowAttrs("src").OnElements("img")

	// Policy Building Helpers

	// If you've got this far and you're bored already, we also bundle some
	// other convenience functions
	p = bluemonday.NewPolicy()
	p.AllowStandardAttributes()
	p.AllowImages()
	p.AllowLists()
	p.AllowTables()
}

func ExampleNewPolicy() {
	// NewPolicy is a blank policy and we need to explicitly allow anything
	// that we wish to allow through
	p := bluemonday.NewPolicy()

	// We ensure any URLs are parseable and have rel="nofollow" where applicable
	p.AllowStandardURLs()

	// AllowStandardURLs already ensures that the href will be valid, and so we
	// can skip the .Matching()
	p.AllowAttrs("href").OnElements("a")

	// We allow paragraphs too
	p.AllowElements("p")

	html := p.Sanitize(
		`<p><a onblur="alert(secret)" href="http://www.google.com">Google</a></p>`,
	)

	fmt.Println(html)

	// Output:
	//<p><a href="http://www.google.com" rel="nofollow">Google</a></p>
}

func ExampleStrictPolicy() {
	// StrictPolicy is equivalent to NewPolicy and as nothing else is declared
	// we are stripping all elements (and their attributes)
	p := bluemonday.StrictPolicy()

	html := p.Sanitize(
		`Goodbye <a onblur="alert(secret)" href="http://en.wikipedia.org/wiki/Goodbye_Cruel_World_(Pink_Floyd_song)">Cruel</a> World`,
	)

	fmt.Println(html)

	// Output:
	//Goodbye Cruel World
}

func ExampleUGCPolicy() {
	// UGCPolicy is a convenience policy for user generated content.
	p := bluemonday.UGCPolicy()

	html := p.Sanitize(
		`<a onblur="alert(secret)" href="http://www.google.com">Google</a>`,
	)

	fmt.Println(html)

	// Output:
	//<a href="http://www.google.com" rel="nofollow">Google</a>
}

func ExamplePolicy_AllowAttrs() {
	p := bluemonday.NewPolicy()

	// Allow the 'title' attribute on every HTML element that has been
	// allowed
	p.AllowAttrs("title").Matching(bluemonday.Paragraph).Globally()

	// Allow the 'abbr' attribute on only the 'td' and 'th' elements.
	p.AllowAttrs("abbr").Matching(bluemonday.Paragraph).OnElements("td", "th")

	// Allow the 'colspan' and 'rowspan' attributes, matching a positive integer
	// pattern, on only the 'td' and 'th' elements.
	p.AllowAttrs("colspan", "rowspan").Matching(
		bluemonday.Integer,
	).OnElements("td", "th")
}

func ExamplePolicy_AllowStyles() {
	p := bluemonday.NewPolicy()

	// Allow only 'span' and 'p' elements
	p.AllowElements("span", "p", "strong")

	// Only allow 'style' attributes on 'span' and 'p' elements
	p.AllowAttrs("style").OnElements("span", "p")

	// Allow the 'text-decoration' property to be set to 'underline', 'line-through' or 'none'
	// on 'span' elements only
	p.AllowStyles("text-decoration").MatchingEnum("underline", "line-through", "none").OnElements("span")

	// Allow the 'color' property with valid RGB(A) hex values only
	// on every HTML element that has been allowed
	p.AllowStyles("color").Matching(regexp.MustCompile("(?i)^#([0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$")).Globally()

	// Default handler
	p.AllowStyles("background-origin").Globally()

	// The span has an invalid 'color' which will be stripped along with other disallowed properties
	html := p.Sanitize(
		`<p style="color:#f00;">
	<span style="text-decoration: underline; background-image: url(javascript:alert('XSS')); color: #f00ba; background-origin: invalidValue">
		Red underlined <strong style="text-decoration:none;">text</strong>
	</span>
</p>`,
	)

	fmt.Println(html)

	// Output:
	//<p style="color: #f00">
	//	<span style="text-decoration: underline">
	//		Red underlined <strong>text</strong>
	//	</span>
	//</p>
}

func ExamplePolicy_AllowElements() {
	p := bluemonday.NewPolicy()

	// Allow styling elements without attributes
	p.AllowElements("br", "div", "hr", "p", "span")
}

func ExamplePolicy_Sanitize() {
	// UGCPolicy is a convenience policy for user generated content.
	p := bluemonday.UGCPolicy()

	// string in, string out
	html := p.Sanitize(`<a onblur="alert(secret)" href="http://www.google.com">Google</a>`)

	fmt.Println(html)

	// Output:
	//<a href="http://www.google.com" rel="nofollow">Google</a>
}

func ExamplePolicy_SanitizeBytes() {
	// UGCPolicy is a convenience policy for user generated content.
	p := bluemonday.UGCPolicy()

	// []byte in, []byte out
	b := []byte(`<a onblur="alert(secret)" href="http://www.google.com">Google</a>`)
	b = p.SanitizeBytes(b)

	fmt.Println(string(b))

	// Output:
	//<a href="http://www.google.com" rel="nofollow">Google</a>
}

func ExamplePolicy_SanitizeReader() {
	// UGCPolicy is a convenience policy for user generated content.
	p := bluemonday.UGCPolicy()

	// io.Reader in, bytes.Buffer out
	r := strings.NewReader(`<a onblur="alert(secret)" href="http://www.google.com">Google</a>`)
	buf := p.SanitizeReader(r)

	fmt.Println(buf.String())

	// Output:
	//<a href="http://www.google.com" rel="nofollow">Google</a>
}
