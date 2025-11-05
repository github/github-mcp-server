package sanitize

import (
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var policy *bluemonday.Policy
var policyOnce sync.Once

func Sanitize(input string) string {
	return FilterHTMLTags(FilterInvisibleCharacters(input))
}

// FilterInvisibleCharacters removes invisible or control characters that should not appear
// in user-facing titles or bodies. This includes:
// - Unicode tag characters: U+E0001, U+E0020–U+E007F
// - BiDi control characters: U+202A–U+202E, U+2066–U+2069
// - Hidden modifier characters: U+200B, U+200C, U+200E, U+200F, U+00AD, U+FEFF, U+180E, U+2060–U+2064
func FilterInvisibleCharacters(input string) string {
	if input == "" {
		return input
	}

	// Filter runes
	out := make([]rune, 0, len(input))
	for _, r := range input {
		if !shouldRemoveRune(r) {
			out = append(out, r)
		}
	}
	return string(out)
}

func FilterHTMLTags(input string) string {
	if input == "" {
		return input
	}
	return getPolicy().Sanitize(input)
}

func getPolicy() *bluemonday.Policy {
	policyOnce.Do(func() {
		p := bluemonday.StrictPolicy()

		p.AllowElements(
			"b", "blockquote", "br", "code", "em",
			"h1", "h2", "h3", "h4", "h5", "h6",
			"hr", "i", "li", "ol", "p", "pre",
			"strong", "sub", "sup", "table", "tbody",
			"td", "th", "thead", "tr", "ul",
			"a", "img",
		)

		p.AllowAttrs("href").OnElements("a")
		p.AllowURLSchemes("https")
		p.RequireParseableURLs(true)
		p.RequireNoFollowOnLinks(true)
		p.RequireNoReferrerOnLinks(true)
		p.AddTargetBlankToFullyQualifiedLinks(true)

		p.AllowImages()
		p.AllowAttrs("src", "alt", "title").OnElements("img")

		policy = p
	})
	return policy
}

func shouldRemoveRune(r rune) bool {
	switch r {
	case 0x200B, // ZERO WIDTH SPACE
		0x200C, // ZERO WIDTH NON-JOINER
		0x200E, // LEFT-TO-RIGHT MARK
		0x200F, // RIGHT-TO-LEFT MARK
		0x00AD, // SOFT HYPHEN
		0xFEFF, // ZERO WIDTH NO-BREAK SPACE
		0x180E: // MONGOLIAN VOWEL SEPARATOR
		return true
	case 0xE0001: // TAG
		return true
	}

	// Ranges
	// Unicode tags: U+E0020–U+E007F
	if r >= 0xE0020 && r <= 0xE007F {
		return true
	}
	// BiDi controls: U+202A–U+202E
	if r >= 0x202A && r <= 0x202E {
		return true
	}
	// BiDi isolates: U+2066–U+2069
	if r >= 0x2066 && r <= 0x2069 {
		return true
	}
	// Hidden modifiers: U+2060–U+2064
	if r >= 0x2060 && r <= 0x2064 {
		return true
	}

	return false
}
