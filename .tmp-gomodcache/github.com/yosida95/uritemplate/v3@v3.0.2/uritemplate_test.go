// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

import (
	"fmt"
	"testing"
)

func ExampleTemplate_Expand() {
	tmpl := MustNew("https://example.com/dictionary/{term:1}/{term}")

	vars := Values{}
	vars.Set("term", String("cat"))
	ret, err := tmpl.Expand(vars)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(ret)

	// Output:
	// https://example.com/dictionary/c/cat
}

func ExampleTemplate_Regexp() {
	tmpl := MustNew("https://example.com/dictionary/{term:1}/{term}")
	re := tmpl.Regexp()

	fmt.Println(re.MatchString("https://example.com/dictionary/c/cat"))
	fmt.Println(re.MatchString("https://example.com/dictionary/c/a/cat"))

	// Output:
	// true
	// false
}

var (
	testTemplateCases = []struct {
		raw       string
		expected  string
		failMatch bool
	}{
		// below cases are quoted from the RFC
		// http://tools.ietf.org/html/rfc6570
		// § 2.1
		{"'{count}'", "'one,two,three'", false},
		// § 3.2.1
		{"{count}", "one,two,three", false},
		{"{count*}", "one,two,three", false},
		{"{/count}", "/one,two,three", false},
		{"{/count*}", "/one/two/three", false},
		{"{;count}", ";count=one,two,three", false},
		{"{;count*}", ";count=one;count=two;count=three", false},
		{"{?count}", "?count=one,two,three", false},
		{"{?count*}", "?count=one&count=two&count=three", false},
		{"{&count*}", "&count=one&count=two&count=three", false},
		// § 3.2.2
		{"{var}", "value", false},
		{"{hello}", "Hello%20World%21", false},
		{"{half}", "50%25", false},
		{"O{empty}X", "OX", false},
		{"O{undef}X", "OX", false},
		{"{x,y}", "1024,768", false},
		{"{x,hello,y}", "1024,Hello%20World%21,768", true},
		{"?{x,empty}", "?1024,", false},
		{"?{x,undef}", "?1024", false},
		{"?{undef,y}", "?768", true},
		{"{var:3}", "val", false},
		{"{var:30}", "value", false},
		{"{list}", "red,green,blue", false},
		{"{list*}", "red,green,blue", false},
		{"{keys}", "semi,%3B,dot,.,comma,%2C", true},
		{"{keys*}", "semi=%3B,dot=.,comma=%2C", true},
		// § 3.2.3
		{"{+var}", "value", false},
		{"{+hello}", "Hello%20World!", false},
		{"{+half}", "50%25", false},
		{"{base}index", "http%3A%2F%2Fexample.com%2Fhome%2Findex", false},
		{"{+base}index", "http://example.com/home/index", false},
		{"O{+empty}X", "OX", false},
		{"O{+undef}X", "OX", false},
		{"{+path}/here", "/foo/bar/here", false},
		{"here?ref={+path}", "here?ref=/foo/bar", false},
		{"up{+path}{var}/here", "up/foo/barvalue/here", true},
		{"{+x,hello,y}", "1024,Hello%20World!,768", true},
		{"{+path,x}/here", "/foo/bar,1024/here", true},
		{"{+path:6}/here", "/foo/b/here", false},
		{"{+list}", "red,green,blue", true},
		{"{+list*}", "red,green,blue", true},
		{"{+keys}", "semi,;,dot,.,comma,,", true},
		{"{+keys*}", "semi=;,dot=.,comma=,", true},
		// § 3.2.4
		{"{#var}", "#value", false},
		{"{#hello}", "#Hello%20World!", false},
		{"{#half}", "#50%25", false},
		{"foo{#empty}", "foo#", false},
		{"foo{#undef}", "foo", false},
		{"{#x,hello,y}", "#1024,Hello%20World!,768", true},
		{"{#path,x}/here", "#/foo/bar,1024/here", true},
		{"{#path:6}/here", "#/foo/b/here", false},
		{"{#list}", "#red,green,blue", true},
		{"{#list*}", "#red,green,blue", true},
		{"{#keys}", "#semi,;,dot,.,comma,,", true},
		{"{#keys*}", "#semi=;,dot=.,comma=,", true},
		// § 3.2.5
		{"{.who}", ".fred", false},
		{"{.who,who}", ".fred.fred", true},
		{"{.half,who}", ".50%25.fred", true},
		{"www{.dom*}", "www.example.com", true},
		{"X{.var}", "X.value", false},
		{"X{.empty}", "X.", false},
		{"X{.undef}", "X", false},
		{"X{.var:3}", "X.val", false},
		{"X{.list}", "X.red,green,blue", true},
		{"X{.list*}", "X.red.green.blue", true},
		{"X{.keys}", "X.semi,%3B,dot,.,comma,%2C", true},
		{"X{.keys*}", "X.semi=%3B.dot=..comma=%2C", true},
		{"X{.empty_keys}", "X", false},
		{"X{.empty_keys*}", "X", false},
		// § 3.2.6
		{"{/who}", "/fred", false},
		{"{/who,who}", "/fred/fred", true},
		{"{/half,who}", "/50%25/fred", false},
		{"{/who,dub}", "/fred/me%2Ftoo", false},
		{"{/var}", "/value", false},
		{"{/var,empty}", "/value/", false},
		{"{/var,undef}", "/value", false},
		{"{/var,x}/here", "/value/1024/here", false},
		{"{/var:1,var}", "/v/value", false},
		{"{/list}", "/red,green,blue", false},
		{"{/list*}", "/red/green/blue", false},
		{"{/list*,path:4}", "/red/green/blue/%2Ffoo", false},
		{"{/keys}", "/semi,%3B,dot,.,comma,%2C", true},
		{"{/keys*}", "/semi=%3B/dot=./comma=%2C", true},
		// § 3.2.7
		{"{;who}", ";who=fred", false},
		{"{;half}", ";half=50%25", false},
		{"{;empty}", ";empty", false},
		{"{;v,empty,who}", ";v=6;empty;who=fred", false},
		{"{;v,bar,who}", ";v=6;who=fred", false},
		{"{;x,y}", ";x=1024;y=768", false},
		{"{;x,y,empty}", ";x=1024;y=768;empty", false},
		{"{;x,y,undef}", ";x=1024;y=768", false},
		{"{;hello:5}", ";hello=Hello", false},
		{"{;list}", ";list=red,green,blue", false},
		{"{;list*}", ";list=red;list=green;list=blue", false},
		{"{;keys}", ";keys=semi,%3B,dot,.,comma,%2C", true},
		{"{;keys*}", ";semi=%3B;dot=.;comma=%2C", true},
		// § 3.2.8
		{"{?who}", "?who=fred", false},
		{"{?half}", "?half=50%25", false},
		{"{?x,y}", "?x=1024&y=768", false},
		{"{?x,y,empty}", "?x=1024&y=768&empty=", false},
		{"{?x,y,undef}", "?x=1024&y=768", false},
		{"{?var:3}", "?var=val", false},
		{"{?list}", "?list=red,green,blue", false},
		{"{?list*}", "?list=red&list=green&list=blue", false},
		{"{?keys}", "?keys=semi,%3B,dot,.,comma,%2C", true},
		{"{?keys*}", "?semi=%3B&dot=.&comma=%2C", true},
		// § 3.2.9
		{"{&who}", "&who=fred", false},
		{"{&half}", "&half=50%25", false},
		{"?fixed=yes{&x}", "?fixed=yes&x=1024", false},
		{"{&x,y,empty}", "&x=1024&y=768&empty=", false},
		{"{&x,y,undef}", "&x=1024&y=768", false},
		{"{&var:3}", "&var=val", false},
		{"{&list}", "&list=red,green,blue", false},
		{"{&list*}", "&list=red&list=green&list=blue", false},
		{"{&keys}", "&keys=semi,%3B,dot,.,comma,%2C", true},
		{"{&keys*}", "&semi=%3B&dot=.&comma=%2C", true},
		// others
		{"{special_chars}", "2001%3Adb8%3A%3A35", false},
	}
	testExpressionExpandVarMap = Values{
		"count":         List("one", "two", "three"),
		"dom":           List("example", "com"),
		"dub":           String("me/too"),
		"hello":         String("Hello World!"),
		"half":          String("50%"),
		"var":           String("value"),
		"who":           String("fred"),
		"base":          String("http://example.com/home/"),
		"path":          String("/foo/bar"),
		"list":          List("red", "green", "blue"),
		"keys":          KV("semi", ";", "dot", ".", "comma", ","),
		"v":             String("6"),
		"x":             String("1024"),
		"y":             String("768"),
		"empty":         String(""),
		"empty_keys":    KV(),
		"special_chars": String("2001:db8::35"),
		// undef is omitted. uritemplate.go treats variables that could not
		// found in the varmap as null.
	}
)

func TestTemplateExpand(t *testing.T) {
	for _, c := range testTemplateCases {
		tmpl, err := New(c.raw)
		if err != nil {
			t.Errorf("unexpected error on %q: %#v", c.raw, err)
			continue
		}

		got, err := tmpl.Expand(testExpressionExpandVarMap)
		if err != nil {
			t.Errorf("unexpected error on %q: %#v", c.raw, err)
			continue
		}
		if c.expected != got {
			t.Errorf("on %q: expected: %#v, got: %#v", c.raw, c.expected, got)
		}
	}
}

func TestTemplateRegexp(t *testing.T) {
	for _, c := range testTemplateCases {
		tmpl, err := New(c.raw)
		if err != nil {
			t.Errorf("unexpected error on %q: %#v", c.raw, err)
			continue
		}
		re := tmpl.Regexp()
		if !re.MatchString(c.expected) {
			t.Errorf("on %q: regexp unexpectedly does not match: %q against %q", c.raw, re, c.expected)
		}
	}
}

func TestTemplateRegexp_NotMatch(t *testing.T) {
	tmpl := MustNew("https://example.com/foo{?bar}")
	if tmpl.Regexp().MatchString("https://example.com/foobaz") {
		t.Errorf("must not match")
	}
}

func BenchmarkExpressionExpand(b *testing.B) {
	c := testTemplateCases[0]
	tmpl, err := New(c.raw)
	if err != nil {
		b.Errorf("got unexpected error; %#v", err)
		return
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tmpl.Expand(testExpressionExpandVarMap); err != nil {
			b.Errorf("got unexpected error; %#v", err)
			return
		}
	}
}

func BenchmarkMatch(b *testing.B) {
	tmpl := MustNew("https://{host}/users{/user}{/media}")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if nil == tmpl.Match("https://example.com/users/kevin/pics") {
			b.Errorf("Must match")
			return
		}
	}
}

func BenchmarkRegexpMatch(b *testing.B) {
	tmpl := MustNew("https://{host}/users{/user}{/media}")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !tmpl.Regexp().MatchString("https://example.com/users/kevin/pics") {
			b.Errorf("Must match")
			return
		}
	}
}

func BenchmarkRegexpFindAll(b *testing.B) {
	tmpl := MustNew("https://{host}/users{/user}{/media}")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if tmpl.Regexp().FindStringSubmatch("https://example.com/users/kevin/pics") == nil {
			b.Errorf("Must match")
			return
		}
	}
}
