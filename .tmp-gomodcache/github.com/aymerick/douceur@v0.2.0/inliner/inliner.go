package inliner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
	"golang.org/x/net/html"
)

const (
	eltMarkerAttr = "douceur-mark"
)

var unsupportedSelectors = []string{
	":active", ":after", ":before", ":checked", ":disabled", ":enabled",
	":first-line", ":first-letter", ":focus", ":hover", ":invalid", ":in-range",
	":lang", ":link", ":root", ":selection", ":target", ":valid", ":visited"}

// Inliner presents a CSS Inliner
type Inliner struct {
	// Raw HTML
	html string

	// Parsed HTML document
	doc *goquery.Document

	// Parsed stylesheets
	stylesheets []*css.Stylesheet

	// Collected inlinable style rules
	rules []*StyleRule

	// HTML elements matching collected inlinable style rules
	elements map[string]*Element

	// CSS rules that are not inlinable but that must be inserted in output document
	rawRules []fmt.Stringer

	// current element marker value
	eltMarker int
}

// NewInliner instanciates a new Inliner
func NewInliner(html string) *Inliner {
	return &Inliner{
		html:     html,
		elements: make(map[string]*Element),
	}
}

// Inline inlines css into html document
func Inline(html string) (string, error) {
	result, err := NewInliner(html).Inline()
	if err != nil {
		return "", err
	}

	return result, nil
}

// Inline inlines CSS and returns HTML
func (inliner *Inliner) Inline() (string, error) {
	// parse HTML document
	if err := inliner.parseHTML(); err != nil {
		return "", err
	}

	// parse stylesheets
	if err := inliner.parseStylesheets(); err != nil {
		return "", err
	}

	// collect elements and style rules
	inliner.collectElementsAndRules()

	// inline css
	if err := inliner.inlineStyleRules(); err != nil {
		return "", err
	}

	// insert raw stylesheet
	inliner.insertRawStylesheet()

	// generate HTML document
	return inliner.genHTML()
}

// Parses raw html
func (inliner *Inliner) parseHTML() error {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(inliner.html))
	if err != nil {
		return err
	}

	inliner.doc = doc

	return nil
}

// Parses and removes stylesheets from HTML document
func (inliner *Inliner) parseStylesheets() error {
	var result error

	inliner.doc.Find("style").EachWithBreak(func(i int, s *goquery.Selection) bool {
		stylesheet, err := parser.Parse(s.Text())
		if err != nil {
			result = err
			return false
		}

		inliner.stylesheets = append(inliner.stylesheets, stylesheet)

		// removes parsed stylesheet
		s.Remove()

		return true
	})

	return result
}

// Collects HTML elements matching parsed stylesheets, and thus collect used style rules
func (inliner *Inliner) collectElementsAndRules() {
	for _, stylesheet := range inliner.stylesheets {
		for _, rule := range stylesheet.Rules {
			if rule.Kind == css.QualifiedRule {
				// Let's go!
				inliner.handleQualifiedRule(rule)
			} else {
				// Keep it 'as is'
				inliner.rawRules = append(inliner.rawRules, rule)
			}
		}
	}
}

// Handles parsed qualified rule
func (inliner *Inliner) handleQualifiedRule(rule *css.Rule) {
	for _, selector := range rule.Selectors {
		if Inlinable(selector) {
			inliner.doc.Find(selector).Each(func(i int, s *goquery.Selection) {
				// get marker
				eltMarker, exists := s.Attr(eltMarkerAttr)
				if !exists {
					// mark element
					eltMarker = strconv.Itoa(inliner.eltMarker)
					s.SetAttr(eltMarkerAttr, eltMarker)
					inliner.eltMarker++

					// add new element
					inliner.elements[eltMarker] = NewElement(s)
				}

				// add style rule for element
				inliner.elements[eltMarker].addStyleRule(NewStyleRule(selector, rule.Declarations))
			})
		} else {
			// Keep it 'as is'
			inliner.rawRules = append(inliner.rawRules, NewStyleRule(selector, rule.Declarations))
		}
	}
}

// Inline style rules in HTML document
func (inliner *Inliner) inlineStyleRules() error {
	for _, element := range inliner.elements {
		// remove marker
		element.elt.RemoveAttr(eltMarkerAttr)

		// inline element
		err := element.inline()
		if err != nil {
			return err
		}
	}

	return nil
}

// Computes raw CSS rules
func (inliner *Inliner) computeRawCSS() string {
	result := ""

	for _, rawRule := range inliner.rawRules {
		result += rawRule.String()
		result += "\n"
	}

	return result
}

// Insert raw CSS rules into HTML document
func (inliner *Inliner) insertRawStylesheet() {
	rawCSS := inliner.computeRawCSS()
	if rawCSS != "" {
		// create <style> element
		cssNode := &html.Node{
			Type: html.TextNode,
			Data: "\n" + rawCSS,
		}

		styleNode := &html.Node{
			Type: html.ElementNode,
			Data: "style",
			Attr: []html.Attribute{html.Attribute{Key: "type", Val: "text/css"}},
		}

		styleNode.AppendChild(cssNode)

		// append to <head> element
		headNode := inliner.doc.Find("head")
		if headNode == nil {
			// @todo Create head node !
			panic("NOT IMPLEMENTED: create missing <head> node")
		}

		headNode.AppendNodes(styleNode)
	}
}

// Generates HTML
func (inliner *Inliner) genHTML() (string, error) {
	return inliner.doc.Html()
}

// Inlinable returns true if given selector is inlinable
func Inlinable(selector string) bool {
	if strings.Contains(selector, "::") {
		return false
	}

	for _, badSel := range unsupportedSelectors {
		if strings.Contains(selector, badSel) {
			return false
		}
	}

	return true
}
