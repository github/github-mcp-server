package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aymerick/douceur/css"
)

func MustParse(t *testing.T, txt string, nbRules int) *css.Stylesheet {
	stylesheet, err := Parse(txt)
	if err != nil {
		t.Fatal("Failed to parse css", err, txt)
	}

	if len(stylesheet.Rules) != nbRules {
		t.Fatal("Failed to parse Qualified Rules", txt)
	}

	return stylesheet
}

func MustEqualRule(t *testing.T, parsedRule *css.Rule, expectedRule *css.Rule) {
	if !parsedRule.Equal(expectedRule) {
		diff := parsedRule.Diff(expectedRule)

		t.Fatal(fmt.Sprintf("Rule parsing error\nExpected:\n\"%s\"\nGot:\n\"%s\"\nDiff:\n%s", expectedRule, parsedRule, strings.Join(diff, "\n")))
	}
}

func MustEqualCSS(t *testing.T, ruleString string, expected string) {
	if ruleString != expected {
		t.Fatal(fmt.Sprintf("CSS generation error\n   Expected:\n\"%s\"\n   Got:\n\"%s\"", expected, ruleString))
	}
}

func TestQualifiedRule(t *testing.T) {
	input := `/* This is a comment */
p > a {
    color: blue;
    text-decoration: underline; /* This is a comment */
}`

	expectedRule := &css.Rule{
		Kind:      css.QualifiedRule,
		Prelude:   "p > a",
		Selectors: []string{"p > a"},
		Declarations: []*css.Declaration{
			&css.Declaration{
				Property: "color",
				Value:    "blue",
			},
			&css.Declaration{
				Property: "text-decoration",
				Value:    "underline",
			},
		},
	}

	expectedOutput := `p > a {
  color: blue;
  text-decoration: underline;
}`

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), expectedOutput)
}

func TestQualifiedRuleImportant(t *testing.T) {
	input := `/* This is a comment */
p > a {
    color: blue;
    text-decoration: underline !important;
    font-weight: normal   !IMPORTANT    ;
}`

	expectedRule := &css.Rule{
		Kind:      css.QualifiedRule,
		Prelude:   "p > a",
		Selectors: []string{"p > a"},
		Declarations: []*css.Declaration{
			&css.Declaration{
				Property:  "color",
				Value:     "blue",
				Important: false,
			},
			&css.Declaration{
				Property:  "text-decoration",
				Value:     "underline",
				Important: true,
			},
			&css.Declaration{
				Property:  "font-weight",
				Value:     "normal",
				Important: true,
			},
		},
	}

	expectedOutput := `p > a {
  color: blue;
  text-decoration: underline !important;
  font-weight: normal !important;
}`

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), expectedOutput)
}

func TestQualifiedRuleSelectors(t *testing.T) {
	input := `table, tr, td {
  padding: 0;
}

body,
  h1,   h2,
    h3   {
  color: #fff;
}`

	expectedRule1 := &css.Rule{
		Kind:      css.QualifiedRule,
		Prelude:   "table, tr, td",
		Selectors: []string{"table", "tr", "td"},
		Declarations: []*css.Declaration{
			&css.Declaration{
				Property: "padding",
				Value:    "0",
			},
		},
	}

	expectedRule2 := &css.Rule{
		Kind: css.QualifiedRule,
		Prelude: `body,
  h1,   h2,
    h3`,
		Selectors: []string{"body", "h1", "h2", "h3"},
		Declarations: []*css.Declaration{
			&css.Declaration{
				Property: "color",
				Value:    "#fff",
			},
		},
	}

	expectedOutput := `table, tr, td {
  padding: 0;
}
body, h1, h2, h3 {
  color: #fff;
}`

	stylesheet := MustParse(t, input, 2)

	MustEqualRule(t, stylesheet.Rules[0], expectedRule1)
	MustEqualRule(t, stylesheet.Rules[1], expectedRule2)

	MustEqualCSS(t, stylesheet.String(), expectedOutput)
}

func TestAtRuleCharset(t *testing.T) {
	input := `@charset "UTF-8";`

	expectedRule := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@charset",
		Prelude: "\"UTF-8\"",
	}

	expectedOutput := `@charset "UTF-8";`

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), expectedOutput)
}

func TestAtRuleCounterStyle(t *testing.T) {
	input := `@counter-style footnote {
  system: symbolic;
  symbols: '*' ⁑ † ‡;
  suffix: '';
}`

	expectedRule := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@counter-style",
		Prelude: "footnote",
		Declarations: []*css.Declaration{
			&css.Declaration{
				Property: "system",
				Value:    "symbolic",
			},
			&css.Declaration{
				Property: "symbols",
				Value:    "'*' ⁑ † ‡",
			},
			&css.Declaration{
				Property: "suffix",
				Value:    "''",
			},
		},
	}

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), input)
}

func TestAtRuleDocument(t *testing.T) {
	input := `@document url(http://www.w3.org/),
               url-prefix(http://www.w3.org/Style/),
               domain(mozilla.org),
               regexp("https:.*")
{
  /* CSS rules here apply to:
     + The page "http://www.w3.org/".
     + Any page whose URL begins with "http://www.w3.org/Style/"
     + Any page whose URL's host is "mozilla.org" or ends with
       ".mozilla.org"
     + Any page whose URL starts with "https:" */

  /* make the above-mentioned pages really ugly */
  body { color: purple; background: yellow; }
}`

	expectedRule := &css.Rule{
		Kind: css.AtRule,
		Name: "@document",
		Prelude: `url(http://www.w3.org/),
               url-prefix(http://www.w3.org/Style/),
               domain(mozilla.org),
               regexp("https:.*")`,
		Rules: []*css.Rule{
			&css.Rule{
				Kind:      css.QualifiedRule,
				Prelude:   "body",
				Selectors: []string{"body"},
				Declarations: []*css.Declaration{
					&css.Declaration{
						Property: "color",
						Value:    "purple",
					},
					&css.Declaration{
						Property: "background",
						Value:    "yellow",
					},
				},
			},
		},
	}

	expectCSS := `@document url(http://www.w3.org/),
               url-prefix(http://www.w3.org/Style/),
               domain(mozilla.org),
               regexp("https:.*") {
  body {
    color: purple;
    background: yellow;
  }
}`

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), expectCSS)
}

func TestAtRuleFontFace(t *testing.T) {
	input := `@font-face {
  font-family: MyHelvetica;
  src: local("Helvetica Neue Bold"),
       local("HelveticaNeue-Bold"),
       url(MgOpenModernaBold.ttf);
  font-weight: bold;
}`

	expectedRule := &css.Rule{
		Kind: css.AtRule,
		Name: "@font-face",
		Declarations: []*css.Declaration{
			&css.Declaration{
				Property: "font-family",
				Value:    "MyHelvetica",
			},
			&css.Declaration{
				Property: "src",
				Value: `local("Helvetica Neue Bold"),
       local("HelveticaNeue-Bold"),
       url(MgOpenModernaBold.ttf)`,
			},
			&css.Declaration{
				Property: "font-weight",
				Value:    "bold",
			},
		},
	}

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), input)
}

func TestAtRuleFontFeatureValues(t *testing.T) {
	input := `@font-feature-values Font Two { /* How to activate nice-style in Font Two */
  @styleset {
    nice-style: 4;
  }
}`
	expectedRule := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@font-feature-values",
		Prelude: "Font Two",
		Rules: []*css.Rule{
			&css.Rule{
				Kind: css.AtRule,
				Name: "@styleset",
				Declarations: []*css.Declaration{
					&css.Declaration{
						Property: "nice-style",
						Value:    "4",
					},
				},
			},
		},
	}

	expectedOutput := `@font-feature-values Font Two {
  @styleset {
    nice-style: 4;
  }
}`

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), expectedOutput)
}

func TestAtRuleImport(t *testing.T) {
	input := `@import "my-styles.css";
@import url('landscape.css') screen and (orientation:landscape);`

	expectedRule1 := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@import",
		Prelude: "\"my-styles.css\"",
	}

	expectedRule2 := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@import",
		Prelude: "url('landscape.css') screen and (orientation:landscape)",
	}

	stylesheet := MustParse(t, input, 2)

	MustEqualRule(t, stylesheet.Rules[0], expectedRule1)
	MustEqualRule(t, stylesheet.Rules[1], expectedRule2)

	MustEqualCSS(t, stylesheet.String(), input)
}

func TestAtRuleKeyframes(t *testing.T) {
	input := `@keyframes identifier {
  0% { top: 0; left: 0; }
  100% { top: 100px; left: 100%; }
}`
	expectedRule := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@keyframes",
		Prelude: "identifier",
		Rules: []*css.Rule{
			&css.Rule{
				Kind:      css.QualifiedRule,
				Prelude:   "0%",
				Selectors: []string{"0%"},
				Declarations: []*css.Declaration{
					&css.Declaration{
						Property: "top",
						Value:    "0",
					},
					&css.Declaration{
						Property: "left",
						Value:    "0",
					},
				},
			},
			&css.Rule{
				Kind:      css.QualifiedRule,
				Prelude:   "100%",
				Selectors: []string{"100%"},
				Declarations: []*css.Declaration{
					&css.Declaration{
						Property: "top",
						Value:    "100px",
					},
					&css.Declaration{
						Property: "left",
						Value:    "100%",
					},
				},
			},
		},
	}

	expectedOutput := `@keyframes identifier {
  0% {
    top: 0;
    left: 0;
  }
  100% {
    top: 100px;
    left: 100%;
  }
}`

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), expectedOutput)
}

func TestAtRuleMedia(t *testing.T) {
	input := `@media screen, print {
  body { line-height: 1.2 }
}`
	expectedRule := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@media",
		Prelude: "screen, print",
		Rules: []*css.Rule{
			&css.Rule{
				Kind:      css.QualifiedRule,
				Prelude:   "body",
				Selectors: []string{"body"},
				Declarations: []*css.Declaration{
					&css.Declaration{
						Property: "line-height",
						Value:    "1.2",
					},
				},
			},
		},
	}

	expectedOutput := `@media screen, print {
  body {
    line-height: 1.2;
  }
}`

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), expectedOutput)
}

func TestAtRuleNamespace(t *testing.T) {
	input := `@namespace svg url(http://www.w3.org/2000/svg);`
	expectedRule := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@namespace",
		Prelude: "svg url(http://www.w3.org/2000/svg)",
	}

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), input)
}

func TestAtRulePage(t *testing.T) {
	input := `@page :left {
  margin-left: 4cm;
  margin-right: 3cm;
}`
	expectedRule := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@page",
		Prelude: ":left",
		Declarations: []*css.Declaration{
			&css.Declaration{
				Property: "margin-left",
				Value:    "4cm",
			},
			&css.Declaration{
				Property: "margin-right",
				Value:    "3cm",
			},
		},
	}

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), input)
}

func TestAtRuleSupports(t *testing.T) {
	input := `@supports (animation-name: test) {
    /* specific CSS applied when animations are supported unprefixed */
    @keyframes { /* @supports being a CSS conditional group at-rule, it can includes other relevent at-rules */
      0% { top: 0; left: 0; }
      100% { top: 100px; left: 100%; }
    }
}`
	expectedRule := &css.Rule{
		Kind:    css.AtRule,
		Name:    "@supports",
		Prelude: "(animation-name: test)",
		Rules: []*css.Rule{
			&css.Rule{
				Kind: css.AtRule,
				Name: "@keyframes",
				Rules: []*css.Rule{
					&css.Rule{
						Kind:      css.QualifiedRule,
						Prelude:   "0%",
						Selectors: []string{"0%"},
						Declarations: []*css.Declaration{
							&css.Declaration{
								Property: "top",
								Value:    "0",
							},
							&css.Declaration{
								Property: "left",
								Value:    "0",
							},
						},
					},
					&css.Rule{
						Kind:      css.QualifiedRule,
						Prelude:   "100%",
						Selectors: []string{"100%"},
						Declarations: []*css.Declaration{
							&css.Declaration{
								Property: "top",
								Value:    "100px",
							},
							&css.Declaration{
								Property: "left",
								Value:    "100%",
							},
						},
					},
				},
			},
		},
	}

	expectedOutput := `@supports (animation-name: test) {
  @keyframes {
    0% {
      top: 0;
      left: 0;
    }
    100% {
      top: 100px;
      left: 100%;
    }
  }
}`

	stylesheet := MustParse(t, input, 1)
	rule := stylesheet.Rules[0]

	MustEqualRule(t, rule, expectedRule)

	MustEqualCSS(t, stylesheet.String(), expectedOutput)
}

func TestParseDeclarations(t *testing.T) {
	input := `color: blue; text-decoration:underline;`

	declarations, err := ParseDeclarations(input)
	if err != nil {
		t.Fatal("Failed to parse Declarations:", input)
	}

	expectedOutput := []*css.Declaration{
		&css.Declaration{
			Property: "color",
			Value:    "blue",
		},
		&css.Declaration{
			Property: "text-decoration",
			Value:    "underline",
		},
	}

	if len(declarations) != len(expectedOutput) {
		t.Fatal("Failed to parse Declarations:", input)
	}

	for i, decl := range declarations {
		if !decl.Equal(expectedOutput[i]) {
			t.Fatal("Failed to parse Declarations: ", decl.String(), expectedOutput[i].String())
		}
	}
}
