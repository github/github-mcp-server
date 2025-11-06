package sanitize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterInvisibleCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "normal text without invisible characters",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "text with zero width space",
			input:    "Hello\u200BWorld",
			expected: "HelloWorld",
		},
		{
			name:     "text with zero width non-joiner",
			input:    "Hello\u200CWorld",
			expected: "HelloWorld",
		},
		{
			name:     "text with left-to-right mark",
			input:    "Hello\u200EWorld",
			expected: "HelloWorld",
		},
		{
			name:     "text with right-to-left mark",
			input:    "Hello\u200FWorld",
			expected: "HelloWorld",
		},
		{
			name:     "text with soft hyphen",
			input:    "Hello\u00ADWorld",
			expected: "HelloWorld",
		},
		{
			name:     "text with zero width no-break space (BOM)",
			input:    "Hello\uFEFFWorld",
			expected: "HelloWorld",
		},
		{
			name:     "text with mongolian vowel separator",
			input:    "Hello\u180EWorld",
			expected: "HelloWorld",
		},
		{
			name:     "text with unicode tag character",
			input:    "Hello\U000E0001World",
			expected: "HelloWorld",
		},
		{
			name:     "text with unicode tag range characters",
			input:    "Hello\U000E0020World\U000E007FTest",
			expected: "HelloWorldTest",
		},
		{
			name:     "text with bidi control characters",
			input:    "Hello\u202AWorld\u202BTest\u202CEnd\u202DMore\u202EFinal",
			expected: "HelloWorldTestEndMoreFinal",
		},
		{
			name:     "text with bidi isolate characters",
			input:    "Hello\u2066World\u2067Test\u2068End\u2069Final",
			expected: "HelloWorldTestEndFinal",
		},
		{
			name:     "text with hidden modifier characters",
			input:    "Hello\u2060World\u2061Test\u2062End\u2063More\u2064Final",
			expected: "HelloWorldTestEndMoreFinal",
		},
		{
			name:     "multiple invisible characters mixed",
			input:    "Hello\u200B\u200C\u200E\u200F\u00AD\uFEFF\u180E\U000E0001World",
			expected: "HelloWorld",
		},
		{
			name:     "text with normal unicode characters (should be preserved)",
			input:    "Hello 世界 🌍 αβγ",
			expected: "Hello 世界 🌍 αβγ",
		},
		{
			name:     "invisible characters at start and end",
			input:    "\u200BHello World\u200C",
			expected: "Hello World",
		},
		{
			name:     "only invisible characters",
			input:    "\u200B\u200C\u200E\u200F",
			expected: "",
		},
		{
			name:     "real-world example with title",
			input:    "Fix\u200B bug\u00AD in\u202A authentication\u202C",
			expected: "Fix bug in authentication",
		},
		{
			name:     "issue body with mixed content",
			input:    "This is a\u200B bug report.\n\nSteps to reproduce:\u200C\n1. Do this\u200E\n2. Do that\u200F",
			expected: "This is a bug report.\n\nSteps to reproduce:\n1. Do this\n2. Do that",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterInvisibleCharacters(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldRemoveRune(t *testing.T) {
	tests := []struct {
		name     string
		rune     rune
		expected bool
	}{
		// Individual characters that should be removed
		{name: "zero width space", rune: 0x200B, expected: true},
		{name: "zero width non-joiner", rune: 0x200C, expected: true},
		{name: "left-to-right mark", rune: 0x200E, expected: true},
		{name: "right-to-left mark", rune: 0x200F, expected: true},
		{name: "soft hyphen", rune: 0x00AD, expected: true},
		{name: "zero width no-break space", rune: 0xFEFF, expected: true},
		{name: "mongolian vowel separator", rune: 0x180E, expected: true},
		{name: "unicode tag", rune: 0xE0001, expected: true},

		// Range tests - Unicode tags: U+E0020–U+E007F
		{name: "unicode tag range start", rune: 0xE0020, expected: true},
		{name: "unicode tag range middle", rune: 0xE0050, expected: true},
		{name: "unicode tag range end", rune: 0xE007F, expected: true},
		{name: "before unicode tag range", rune: 0xE001F, expected: false},
		{name: "after unicode tag range", rune: 0xE0080, expected: false},

		// Range tests - BiDi controls: U+202A–U+202E
		{name: "bidi control range start", rune: 0x202A, expected: true},
		{name: "bidi control range middle", rune: 0x202C, expected: true},
		{name: "bidi control range end", rune: 0x202E, expected: true},
		{name: "before bidi control range", rune: 0x2029, expected: false},
		{name: "after bidi control range", rune: 0x202F, expected: false},

		// Range tests - BiDi isolates: U+2066–U+2069
		{name: "bidi isolate range start", rune: 0x2066, expected: true},
		{name: "bidi isolate range middle", rune: 0x2067, expected: true},
		{name: "bidi isolate range end", rune: 0x2069, expected: true},
		{name: "before bidi isolate range", rune: 0x2065, expected: false},
		{name: "after bidi isolate range", rune: 0x206A, expected: false},

		// Range tests - Hidden modifiers: U+2060–U+2064
		{name: "hidden modifier range start", rune: 0x2060, expected: true},
		{name: "hidden modifier range middle", rune: 0x2062, expected: true},
		{name: "hidden modifier range end", rune: 0x2064, expected: true},
		{name: "before hidden modifier range", rune: 0x205F, expected: false},
		{name: "after hidden modifier range", rune: 0x2065, expected: false},

		// Characters that should NOT be removed
		{name: "regular ascii letter", rune: 'A', expected: false},
		{name: "regular ascii digit", rune: '1', expected: false},
		{name: "regular ascii space", rune: ' ', expected: false},
		{name: "newline", rune: '\n', expected: false},
		{name: "tab", rune: '\t', expected: false},
		{name: "unicode letter", rune: '世', expected: false},
		{name: "emoji", rune: '🌍', expected: false},
		{name: "greek letter", rune: 'α', expected: false},
		{name: "punctuation", rune: '.', expected: false},
		{name: "hyphen (normal)", rune: '-', expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldRemoveRune(tt.rune)
			assert.Equal(t, tt.expected, result, "rune: U+%04X (%c)", tt.rune, tt.rune)
		})
	}
}

func TestFilterHtmlTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "allowed simple tags preserved",
			input:    "<b>bold</b>",
			expected: "<b>bold</b>",
		},
		{
			name:     "multiple allowed tags",
			input:    "<b>bold</b> and <em>italic</em>",
			expected: "<b>bold</b> and <em>italic</em>",
		},
		{
			name:     "code tag preserved",
			input:    "<code>fmt.Println(\"hi\")</code>",
			expected: "<code>fmt.Println(&#34;hi&#34;)</code>", // quotes are escaped by sanitizer
		},
		{
			name:     "disallowed script removed entirely",
			input:    "<script>alert(1)</script>",
			expected: "", // StrictPolicy should drop script element and contents
		},
		{
			name:     "allow anchor with https href",
			input:    "Click <a href=\"https://example.com\">here</a> now",
			expected: "Click <a href=\"https://example.com\" rel=\"nofollow noreferrer noopener\" target=\"_blank\">here</a> now",
		},
		{
			name:     "anchor removed but inner text kept",
			input:    "before <a href='https://example.com' onclick='alert(1)' title='foo' alt='bar'>link</a> after",
			expected: "before <a href=\"https://example.com\" rel=\"nofollow noreferrer noopener\" target=\"_blank\">link</a> after",
		},
		{
			name:     "image removed (no textual fallback)",
			input:    "<img src='x' alt='y'>",
			expected: "<img src=\"x\" alt=\"y\">", // images are allowed via AllowImages()
		},
		{
			name:     "mixed allowed and disallowed",
			input:    "<b>bold</b> <script>alert(1)</script> <em>italic</em>",
			expected: "<b>bold</b>  <em>italic</em>",
		},
		{
			name:     "idempotent sanitization",
			input:    FilterHTMLTags("<b>bold</b> and <em>italic</em>"),
			expected: "<b>bold</b> and <em>italic</em>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterHTMLTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
