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

import "testing"

func TestStrictPolicy(t *testing.T) {

	p := StrictPolicy()

	tests := []test{
		{
			in:       "Hello, <b>World</b>!",
			expected: "Hello, World!",
		},
		{
			in:       "<blockquote>Hello, <b>World</b>!",
			expected: "Hello, World!",
		},
		{ // Real world example from a message board
			in:       `<quietly>email me - addy in profile</quiet>`,
			expected: `email me - addy in profile`,
		},
		{},
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

func TestUGCPolicy(t *testing.T) {

	tests := []test{
		// Simple formatting
		{in: "Hello, World!", expected: "Hello, World!"},
		{in: "Hello, <b>World</b>!", expected: "Hello, <b>World</b>!"},
		// Blocks and formatting
		{
			in:       "<p>Hello, <b onclick=alert(1337)>World</b>!</p>",
			expected: "<p>Hello, <b>World</b>!</p>",
		},
		{
			in:       "<p onclick=alert(1337)>Hello, <b>World</b>!</p>",
			expected: "<p>Hello, <b>World</b>!</p>",
		},
		// Images
		{
			in:       `<a href="javascript:alert(1337)">foo</a>`,
			expected: `foo`,
		},
		{
			in:       `<img src="http://example.org/foo.gif">`,
			expected: `<img src="http://example.org/foo.gif">`,
		},
		{
			in:       `<img src="http://example.org/x.gif" alt="y" width=96 height=64 border=0>`,
			expected: `<img src="http://example.org/x.gif" alt="y" width="96" height="64">`,
		},
		{
			in:       `<img src="http://example.org/x.png" alt="y" width="widgy" height=64 border=0>`,
			expected: `<img src="http://example.org/x.png" alt="y" height="64">`,
		},
		// Anchors
		{
			in:       `<a href="foo.html">Link text</a>`,
			expected: `<a href="foo.html" rel="nofollow">Link text</a>`,
		},
		{
			in:       `<a href="foo.html" onclick="alert(1337)">Link text</a>`,
			expected: `<a href="foo.html" rel="nofollow">Link text</a>`,
		},
		{
			in:       `<a href="http://example.org/x.html" onclick="alert(1337)">Link text</a>`,
			expected: `<a href="http://example.org/x.html" rel="nofollow">Link text</a>`,
		},
		{
			in:       `<a href="https://example.org/x.html" onclick="alert(1337)">Link text</a>`,
			expected: `<a href="https://example.org/x.html" rel="nofollow">Link text</a>`,
		},
		{
			in:       `<a href="HTTPS://example.org/x.html" onclick="alert(1337)">Link text</a>`,
			expected: `<a href="https://example.org/x.html" rel="nofollow">Link text</a>`,
		},
		{
			in:       `<a href="//example.org/x.html" onclick="alert(1337)">Link text</a>`,
			expected: `<a href="//example.org/x.html" rel="nofollow">Link text</a>`,
		},
		{
			in:       `<a href="javascript:alert(1337).html" onclick="alert(1337)">Link text</a>`,
			expected: `Link text`,
		},
		{
			in:       `<a name="header" id="header">Header text</a>`,
			expected: `<a id="header">Header text</a>`,
		},
		// Image map and links
		{
			in:       `<img src="planets.gif" width="145" height="126" alt="" usemap="#demomap"><map name="demomap"><area shape="rect" coords="0,0,82,126" href="demo.htm" alt="1"><area shape="circle" coords="90,58,3" href="demo.htm" alt="2"><area shape="circle" coords="124,58,8" href="demo.htm" alt="3"></map>`,
			expected: `<img src="planets.gif" width="145" height="126" alt="" usemap="#demomap"><map name="demomap"><area shape="rect" coords="0,0,82,126" href="demo.htm" alt="1" rel="nofollow"><area shape="circle" coords="90,58,3" href="demo.htm" alt="2" rel="nofollow"><area shape="circle" coords="124,58,8" href="demo.htm" alt="3" rel="nofollow"></map>`,
		},
		// Tables
		{
			in: `<table style="color: rgb(0, 0, 0);">` +
				`<tbody>` +
				`<tr>` +
				`<th>Column One</th><th>Column Two</th>` +
				`</tr>` +
				`<tr>` +
				`<td align="center"` +
				` style="background-color: rgb(255, 255, 254);">` +
				`<font size="2">Size 2</font></td>` +
				`<td align="center"` +
				` style="background-color: rgb(255, 255, 254);">` +
				`<font size="7">Size 7</font></td>` +
				`</tr>` +
				`</tbody>` +
				`</table>`,
			expected: "" +
				`<table>` +
				`<tbody>` +
				`<tr>` +
				`<th>Column One</th><th>Column Two</th>` +
				`</tr>` +
				`<tr>` +
				`<td align="center">Size 2</td>` +
				`<td align="center">Size 7</td>` +
				`</tr>` +
				`</tbody>` +
				`</table>`,
		},
		// Ordering
		{
			in:       `xss<a href="http://www.google.de" style="color:red;" onmouseover=alert(1) onmousemove="alert(2)" onclick=alert(3)>g<img src="http://example.org"/>oogle</a>`,
			expected: `xss<a href="http://www.google.de" rel="nofollow">g<img src="http://example.org"/>oogle</a>`,
		},
		// OWASP 25 June 2014 09:15 Strange behaviour
		{
			in:       "<table>Hallo\r\n<script>SCRIPT</script>\nEnde\n\r",
			expected: "<table>Hallo\n\nEnde\n\n",
		},
	}

	p := UGCPolicy()

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
