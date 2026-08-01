// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scanner

import (
	"testing"
)

func TestMatchers(t *testing.T) {
	// Just basic checks, not exhaustive at all.
	checkMatch := func(s string, ttList ...interface{}) {
		scanner := New(s)

		i := 0
		for i < len(ttList) {
			tt := ttList[i].(tokenType)
			tVal := ttList[i+1].(string)
			if tok := scanner.Next(); tok.Type != tt || tok.Value != tVal {
				t.Errorf("did not match: %s (got %v)", s, tok)
			}

			i += 2
		}

		if tok := scanner.Next(); tok.Type != TokenEOF {
			t.Errorf("missing EOF after token %s, got %+v", s, tok)
		}
	}

	checkMatch("abcd", TokenIdent, "abcd")
	checkMatch(`"abcd"`, TokenString, `"abcd"`)
	checkMatch(`"ab'cd"`, TokenString, `"ab'cd"`)
	checkMatch(`"ab\"cd"`, TokenString, `"ab\"cd"`)
	checkMatch(`"ab\\cd"`, TokenString, `"ab\\cd"`)
	checkMatch("'abcd'", TokenString, "'abcd'")
	checkMatch(`'ab"cd'`, TokenString, `'ab"cd'`)
	checkMatch(`'ab\'cd'`, TokenString, `'ab\'cd'`)
	checkMatch(`'ab\\cd'`, TokenString, `'ab\\cd'`)
	checkMatch("#name", TokenHash, "#name")
	checkMatch("42''", TokenNumber, "42", TokenString, "''")
	checkMatch("4.2", TokenNumber, "4.2")
	checkMatch(".42", TokenNumber, ".42")
	checkMatch("42%", TokenPercentage, "42%")
	checkMatch("4.2%", TokenPercentage, "4.2%")
	checkMatch(".42%", TokenPercentage, ".42%")
	checkMatch("42px", TokenDimension, "42px")
	checkMatch("url(http://domain.com)", TokenURI, "url(http://domain.com)")
	checkMatch("url( http://domain.com/uri/between/space )", TokenURI, "url( http://domain.com/uri/between/space )")
	checkMatch("url('http://domain.com/uri/between/single/quote')", TokenURI, "url('http://domain.com/uri/between/single/quote')")
	checkMatch(`url("http://domain.com/uri/between/double/quote")`, TokenURI, `url("http://domain.com/uri/between/double/quote")`)
	checkMatch("url(http://domain.com/?parentheses=%28)", TokenURI, "url(http://domain.com/?parentheses=%28)")
	checkMatch("url( http://domain.com/?parentheses=%28&between=space )", TokenURI, "url( http://domain.com/?parentheses=%28&between=space )")
	checkMatch("url('http://domain.com/uri/(parentheses)/between/single/quote')", TokenURI, "url('http://domain.com/uri/(parentheses)/between/single/quote')")
	checkMatch(`url("http://domain.com/uri/(parentheses)/between/double/quote")`, TokenURI, `url("http://domain.com/uri/(parentheses)/between/double/quote")`)
	checkMatch("url(http://domain.com/uri/1)url(http://domain.com/uri/2)",
		TokenURI, "url(http://domain.com/uri/1)",
		TokenURI, "url(http://domain.com/uri/2)",
	)
	checkMatch("U+0042", TokenUnicodeRange, "U+0042")
	checkMatch("<!--", TokenCDO, "<!--")
	checkMatch("-->", TokenCDC, "-->")
	checkMatch("   \n   \t   \n", TokenS, "   \n   \t   \n")
	checkMatch("/* foo */", TokenComment, "/* foo */")
	checkMatch("bar(", TokenFunction, "bar(")
	checkMatch("~=", TokenIncludes, "~=")
	checkMatch("|=", TokenDashMatch, "|=")
	checkMatch("^=", TokenPrefixMatch, "^=")
	checkMatch("$=", TokenSuffixMatch, "$=")
	checkMatch("*=", TokenSubstringMatch, "*=")
	checkMatch("{", TokenChar, "{")
	checkMatch("\uFEFF", TokenBOM, "\uFEFF")
	checkMatch(`╯︵┻━┻"stuff"`, TokenIdent, "╯︵┻━┻", TokenString, `"stuff"`)
}

func TestPreprocess(t *testing.T) {
	tcs := []struct{ desc, input, expected string }{
		{
			"CR",
			".a{ \r color:red}",
			".a{ \n color:red}",
		},
		{
			"FF",
			".a{ \f color:red}",
			".a{ \n color:red}",
		},
		{
			"CRLF",
			".a{ \r\n color:red}",
			".a{ \n color:red}",
		},
		{
			"NULL",
			".a{ \u0000 color:red}",
			".a{ \ufffd color:red}",
		},
		{
			"mixture",
			".a{ \r\r\n\u0000\f color:red}",
			".a{ \n\n\ufffd\n color:red}",
		},
	}
	for _, tc := range tcs {
		s := New(tc.input)
		if s.input != tc.expected {
			t.Errorf("%s: got=%q, want=%q", tc.desc, s.input, tc.expected)
		}
	}
}
