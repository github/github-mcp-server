package inliner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aymerick/douceur/css"
)

const (
	inlineFakeSelector = "*INLINE*"

	// Regular expressions borrowed from premailer:
	//   https://github.com/premailer/css_parser/blob/master/lib/css_parser/regexps.rb
	nonIDAttributesAndPseudoClassesRegexpConst = `(?i)(\.[\w]+)|\[(\w+)|(\:(link|visited|active|hover|focus|lang|target|enabled|disabled|checked|indeterminate|root|nth-child|nth-last-child|nth-of-type|nth-last-of-type|first-child|last-child|first-of-type|last-of-type|only-child|only-of-type|empty|contains))`
	elementsAndPseudoElementsRegexpConst       = `(?i)((^|[\s\+\>\~]+)[\w]+|\:{1,2}(after|before|first-letter|first-line|selection))`
)

var (
	nonIDAttrAndPseudoClassesRegexp *regexp.Regexp
	elementsAndPseudoElementsRegexp *regexp.Regexp
)

// StyleRule represents a Qualifier Rule for a uniq selector
type StyleRule struct {
	// The style rule selector
	Selector string

	// The style rule properties
	Declarations []*css.Declaration

	// Selector specificity
	Specificity int
}

func init() {
	nonIDAttrAndPseudoClassesRegexp, _ = regexp.Compile(nonIDAttributesAndPseudoClassesRegexpConst)
	elementsAndPseudoElementsRegexp, _ = regexp.Compile(elementsAndPseudoElementsRegexpConst)
}

// NewStyleRule instanciates a new StyleRule
func NewStyleRule(selector string, declarations []*css.Declaration) *StyleRule {
	return &StyleRule{
		Selector:     selector,
		Declarations: declarations,
		Specificity:  ComputeSpecificity(selector),
	}
}

// Returns the string representation of a style rule
func (styleRule *StyleRule) String() string {
	result := ""

	result += styleRule.Selector

	if len(styleRule.Declarations) == 0 {
		result += ";"
	} else {
		result += " {\n"

		for _, decl := range styleRule.Declarations {
			result += fmt.Sprintf("  %s\n", decl.String())
		}

		result += "}"
	}

	return result
}

// ComputeSpecificity computes style rule specificity
//
// cf. http://www.w3.org/TR/selectors/#specificity
func ComputeSpecificity(selector string) int {
	result := 0

	if selector == inlineFakeSelector {
		result += 1000
	}

	result += 100 * strings.Count(selector, "#")
	result += 10 * len(nonIDAttrAndPseudoClassesRegexp.FindAllStringSubmatch(selector, -1))
	result += len(elementsAndPseudoElementsRegexp.FindAllStringSubmatch(selector, -1))

	return result
}
