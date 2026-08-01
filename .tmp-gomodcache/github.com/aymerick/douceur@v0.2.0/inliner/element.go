package inliner

import (
	"sort"

	"github.com/PuerkitoBio/goquery"

	"github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
)

// Element represents a HTML element with matching CSS rules
type Element struct {
	// The goquery handler
	elt *goquery.Selection

	// The style rules to apply on that element
	styleRules []*StyleRule
}

// ElementAttr represents a HTML element attribute
type ElementAttr struct {
	attr     string
	elements []string
}

// Index is style property name
var styleToAttr map[string]*ElementAttr

func init() {
	// Borrowed from premailer:
	//   https://github.com/premailer/premailer/blob/master/lib/premailer/premailer.rb
	styleToAttr = map[string]*ElementAttr{
		"text-align": &ElementAttr{
			"align",
			[]string{"h1", "h2", "h3", "h4", "h5", "h6", "p", "div", "blockquote", "tr", "th", "td"},
		},
		"background-color": &ElementAttr{
			"bgcolor",
			[]string{"body", "table", "tr", "th", "td"},
		},
		"background-image": &ElementAttr{
			"background",
			[]string{"table"},
		},
		"vertical-align": &ElementAttr{
			"valign",
			[]string{"th", "td"},
		},
		"float": &ElementAttr{
			"align",
			[]string{"img"},
		},
		// @todo width and height ?
	}
}

// NewElement instanciates a new element
func NewElement(elt *goquery.Selection) *Element {
	return &Element{
		elt: elt,
	}
}

// Add a Style Rule to Element
func (element *Element) addStyleRule(styleRule *StyleRule) {
	element.styleRules = append(element.styleRules, styleRule)
}

// Inline styles on element
func (element *Element) inline() error {
	// compute declarations
	declarations, err := element.computeDeclarations()
	if err != nil {
		return err
	}

	// set style attribute
	styleValue := computeStyleValue(declarations)
	if styleValue != "" {
		element.elt.SetAttr("style", styleValue)
	}

	// set additionnal attributes
	element.setAttributesFromStyle(declarations)

	return nil
}

// Compute css declarations
func (element *Element) computeDeclarations() ([]*css.Declaration, error) {
	result := []*css.Declaration{}

	styles := make(map[string]*StyleDeclaration)

	// First: parsed stylesheets rules
	mergeStyleDeclarations(element.styleRules, styles)

	// Then: inline rules
	inlineRules, err := element.parseInlineStyle()
	if err != nil {
		return result, err
	}

	mergeStyleDeclarations(inlineRules, styles)

	// map to array
	for _, styleDecl := range styles {
		result = append(result, styleDecl.Declaration)
	}

	// sort declarations by property name
	sort.Sort(css.DeclarationsByProperty(result))

	return result, nil
}

// Parse inline style rules
func (element *Element) parseInlineStyle() ([]*StyleRule, error) {
	result := []*StyleRule{}

	styleValue, exists := element.elt.Attr("style")
	if (styleValue == "") || !exists {
		return result, nil
	}

	declarations, err := parser.ParseDeclarations(styleValue)
	if err != nil {
		return result, err
	}

	result = append(result, NewStyleRule(inlineFakeSelector, declarations))

	return result, nil
}

// Set additional attributes from style declarations
func (element *Element) setAttributesFromStyle(declarations []*css.Declaration) {
	// for each style declarations
	for _, declaration := range declarations {
		if eltAttr := styleToAttr[declaration.Property]; eltAttr != nil {
			// check if element is allowed for that attribute
			for _, eltAllowed := range eltAttr.elements {
				if element.elt.Nodes[0].Data == eltAllowed {
					element.elt.SetAttr(eltAttr.attr, declaration.Value)

					break
				}
			}
		}
	}
}

// helper
func computeStyleValue(declarations []*css.Declaration) string {
	result := ""

	// set style attribute value
	for _, declaration := range declarations {
		if result != "" {
			result += " "
		}

		result += declaration.StringWithImportant(false)
	}

	return result
}

// helper
func mergeStyleDeclarations(styleRules []*StyleRule, output map[string]*StyleDeclaration) {
	for _, styleRule := range styleRules {
		for _, declaration := range styleRule.Declarations {
			styleDecl := NewStyleDeclaration(styleRule, declaration)

			if (output[declaration.Property] == nil) || (styleDecl.Specificity() >= output[declaration.Property].Specificity()) {
				output[declaration.Property] = styleDecl
			}
		}
	}
}
