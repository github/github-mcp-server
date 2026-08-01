package inliner

import "github.com/aymerick/douceur/css"

// StyleDeclaration represents a style declaration
type StyleDeclaration struct {
	StyleRule   *StyleRule
	Declaration *css.Declaration
}

// NewStyleDeclaration instanciates a new StyleDeclaration
func NewStyleDeclaration(styleRule *StyleRule, declaration *css.Declaration) *StyleDeclaration {
	return &StyleDeclaration{
		StyleRule:   styleRule,
		Declaration: declaration,
	}
}

// StyleDeclaration computes style declaration specificity
func (styleDecl *StyleDeclaration) Specificity() int {
	if styleDecl.Declaration.Important {
		return 10000
	}

	return styleDecl.StyleRule.Specificity
}
