package sanitize

import (
	"strings"
	"sync"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
)

var policy *bluemonday.Policy
var policyOnce sync.Once

func Sanitize(input string) string {
	cleaned := FilterCodeFenceMetadata(FilterInvisibleCharacters(input))
	// Protect angle brackets inside code blocks and inline code spans
	// from being stripped by the HTML sanitizer. bluemonday treats <int>,
	// <T>, etc. as unknown HTML tags and removes them.
	// See https://github.com/github/github-mcp-server/issues/2202
	protected := protectCodeAngleBrackets(cleaned)
	sanitized := FilterHTMLTags(protected)
	return restoreCodeAngleBrackets(sanitized)
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

// FilterCodeFenceMetadata removes hidden or suspicious info strings from fenced code blocks.
func FilterCodeFenceMetadata(input string) string {
	if input == "" {
		return input
	}

	lines := strings.Split(input, "\n")
	insideFence := false
	currentFenceLen := 0
	for i, line := range lines {
		sanitized, toggled, fenceLen := sanitizeCodeFenceLine(line, insideFence, currentFenceLen)
		lines[i] = sanitized
		if toggled {
			insideFence = !insideFence
			if insideFence {
				currentFenceLen = fenceLen
			} else {
				currentFenceLen = 0
			}
		}
	}
	return strings.Join(lines, "\n")
}

const maxCodeFenceInfoLength = 48

func sanitizeCodeFenceLine(line string, insideFence bool, expectedFenceLen int) (string, bool, int) {
	idx := strings.Index(line, "```")
	if idx == -1 {
		return line, false, expectedFenceLen
	}

	if hasNonWhitespace(line[:idx]) {
		return line, false, expectedFenceLen
	}

	fenceEnd := idx
	for fenceEnd < len(line) && line[fenceEnd] == '`' {
		fenceEnd++
	}

	fenceLen := fenceEnd - idx
	if fenceLen < 3 {
		return line, false, expectedFenceLen
	}

	rest := line[fenceEnd:]

	if insideFence {
		if expectedFenceLen != 0 && fenceLen != expectedFenceLen {
			return line, false, expectedFenceLen
		}
		return line[:fenceEnd], true, fenceLen
	}

	trimmed := strings.TrimSpace(rest)

	if trimmed == "" {
		return line[:fenceEnd], true, fenceLen
	}

	if strings.IndexFunc(trimmed, unicode.IsSpace) != -1 {
		return line[:fenceEnd], true, fenceLen
	}

	if len(trimmed) > maxCodeFenceInfoLength {
		return line[:fenceEnd], true, fenceLen
	}

	if !isSafeCodeFenceToken(trimmed) {
		return line[:fenceEnd], true, fenceLen
	}

	if len(rest) > 0 && unicode.IsSpace(rune(rest[0])) {
		return line[:fenceEnd] + " " + trimmed, true, fenceLen
	}

	return line[:fenceEnd] + trimmed, true, fenceLen
}

func hasNonWhitespace(segment string) bool {
	for _, r := range segment {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func isSafeCodeFenceToken(token string) bool {
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '+', '-', '_', '#', '.':
			continue
		}
		return false
	}
	return true
}

// Sentinels used to protect angle brackets inside code from HTML sanitization.
// These are chosen to be unlikely to appear in real content.
const (
	ltSentinel = "\x00LT\x00"
	gtSentinel = "\x00GT\x00"
)

// protectCodeAngleBrackets replaces < and > inside fenced code blocks and
// inline code spans with sentinels so bluemonday does not strip them as HTML.
func protectCodeAngleBrackets(input string) string {
	var b strings.Builder
	b.Grow(len(input))

	runes := []rune(input)
	i := 0
	n := len(runes)

	for i < n {
		// Fenced code block: ``` ... ```
		if i+2 < n && runes[i] == '`' && runes[i+1] == '`' && runes[i+2] == '`' {
			// Find the fence length
			fenceStart := i
			fenceLen := 0
			for i < n && runes[i] == '`' {
				fenceLen++
				i++
			}
			// Write opening fence + rest of line (info string)
			for range fenceLen {
				b.WriteRune('`')
			}
			for i < n && runes[i] != '\n' {
				b.WriteRune(runes[i])
				i++
			}
			if i < n {
				b.WriteRune(runes[i]) // newline
				i++
			}
			// Inside fence: protect angle brackets until closing fence
			for i < n {
				// Check for closing fence
				if runes[i] == '`' {
					closeLen := 0
					j := i
					for j < n && runes[j] == '`' {
						closeLen++
						j++
					}
					if closeLen >= fenceLen {
						for range closeLen {
							b.WriteRune('`')
						}
						i = j
						break
					}
				}
				switch runes[i] {
				case '<':
					b.WriteString(ltSentinel)
				case '>':
					b.WriteString(gtSentinel)
				default:
					b.WriteRune(runes[i])
				}
				i++
			}
			_ = fenceStart
			continue
		}

		// Inline code: `...`
		if runes[i] == '`' {
			// Count opening backticks
			openLen := 0
			j := i
			for j < n && runes[j] == '`' {
				openLen++
				j++
			}
			// Don't treat ``` as inline code (handled above for fenced blocks)
			if openLen >= 3 {
				for range openLen {
					b.WriteRune('`')
				}
				i = j
				continue
			}
			// Find matching closing backticks
			closeStart := -1
			for k := j; k <= n-openLen; k++ {
				match := true
				for m := range openLen {
					if runes[k+m] != '`' {
						match = false
						break
					}
				}
				if match {
					// Verify it's exactly openLen backticks (not more)
					if k+openLen < n && runes[k+openLen] == '`' {
						continue
					}
					closeStart = k
					break
				}
			}
			if closeStart == -1 {
				// No closing backticks found; treat as literal
				for range openLen {
					b.WriteRune('`')
				}
				i = j
				continue
			}
			// Write opening backticks
			for range openLen {
				b.WriteRune('`')
			}
			// Protect content
			for i = j; i < closeStart; i++ {
				switch runes[i] {
				case '<':
					b.WriteString(ltSentinel)
				case '>':
					b.WriteString(gtSentinel)
				default:
					b.WriteRune(runes[i])
				}
			}
			// Write closing backticks
			for range openLen {
				b.WriteRune('`')
			}
			i = closeStart + openLen
			continue
		}

		b.WriteRune(runes[i])
		i++
	}

	return b.String()
}

// restoreCodeAngleBrackets converts sentinels back to angle brackets.
func restoreCodeAngleBrackets(input string) string {
	s := strings.ReplaceAll(input, ltSentinel, "<")
	return strings.ReplaceAll(s, gtSentinel, ">")
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
		p.AllowURLSchemes("http", "https")
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
