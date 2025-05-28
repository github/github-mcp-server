package filtering

import (
	"testing"
)

func TestFilterContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		cfg      *Config
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Normal text without hidden content",
			input:    "This is normal text without any hidden content.",
			expected: "This is normal text without any hidden content.",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with invisible characters",
			input:    "Hidden\u200Bcharacters\u200Bin\u200Bthis\u200Btext",
			expected: "Hiddencharactersinthistext",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with HTML comments",
			input:    "This has a <!-- hidden comment --> in it.",
			expected: "This has a [HTML_COMMENT] in it.",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with HTML elements",
			input:    "This has <script>alert('hidden')</script> scripts.",
			expected: "This has [HTML_ELEMENT] scripts.",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with details/summary",
			input:    "Collapsed content: <details><summary>Click me</summary>Hidden content</details>",
			expected: "Collapsed content: \n\n**Click me:**\nHidden content\n\n",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with small font",
			input:    "This has <span style=\"font-size:1px\">hidden tiny text</span> in it.",
			expected: "This has <span>hidden tiny text</span> in it.",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with excessive whitespace",
			input:    "Line 1\n\n\n\n\n\nLine 2",
			expected: "Line 1\n\n\nLine 2",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with excessive spaces",
			input:    "Normal                               Excessive",
			expected: "Normal              Excessive",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with excessive tabs",
			input:    "Normal\t\t\t\t\t\t\t\tExcessive",
			expected: "Normal     Excessive",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Text with HTML attributes",
			input:    "<p data-hidden=\"true\" style=\"display:none\">Hidden paragraph</p>",
			expected: "<p>Hidden paragraph</p>",
			cfg:      DefaultConfig(),
		},
		{
			name:     "Filtering disabled",
			input:    "Hidden\u200Bcharacters and <!-- comments -->",
			expected: "Hidden\u200Bcharacters and <!-- comments -->",
			cfg:      &Config{DisableContentFiltering: true},
		},
		{
			name:     "Nil config uses default (filtering enabled)",
			input:    "Hidden\u200Bcharacters",
			expected: "Hiddencharacters",
			cfg:      nil,
		},
		{
			name:     "Normal markdown with code blocks",
			input:    "# Title\n\n```go\nfunc main() {\n    fmt.Println(\"Hello, world!\")\n}\n```",
			expected: "# Title\n\n```go\nfunc main() {\n    fmt.Println(\"Hello, world!\")\n}\n```",
			cfg:      DefaultConfig(),
		},
		{
			name:     "GitHub flavored markdown with tables",
			input:    "| Header 1 | Header 2 |\n| -------- | -------- |\n| Cell 1   | Cell 2   |",
			expected: "| Header 1 | Header 2 |\n| -------- | -------- |\n| Cell 1   | Cell 2   |",
			cfg:      DefaultConfig(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FilterContent(tc.input, tc.cfg)
			if result != tc.expected {
				t.Errorf("FilterContent() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestMakeCollapsedSectionVisible(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple details/summary",
			input:    "<details><summary>Click me</summary>Hidden content</details>",
			expected: "\n\n**Click me:**\nHidden content\n\n",
		},
		{
			name:     "Details without summary",
			input:    "<details>Hidden content</details>",
			expected: "\n\n**Collapsed section:**\nHidden content\n\n",
		},
		{
			name:     "Nested content",
			input:    "<details><summary>Outer</summary>Content<details><summary>Inner</summary>Nested</details></details>",
			expected: "\n\n**Outer:**\nContent<details><summary>Inner</summary>Nested</details>\n\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := makeCollapsedSectionVisible(tc.input)
			if result != tc.expected {
				t.Errorf("makeCollapsedSectionVisible() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestCleanHTMLAttributes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Tag with style attribute",
			input:    "<p style=\"display:none\">Hidden</p>",
			expected: "<p>Hidden</p>",
		},
		{
			name:     "Tag with data attribute",
			input:    "<p data-hidden=\"true\">Hidden</p>",
			expected: "<p>Hidden</p>",
		},
		{
			name:     "Tag with multiple attributes",
			input:    "<p id=\"para\" style=\"display:none\" data-test=\"value\">Hidden</p>",
			expected: "<p id=\"para\">Hidden</p>",
		},
		{
			name:     "Tag with allowed attributes",
			input:    "<a href=\"https://example.com\" target=\"_blank\">Link</a>",
			expected: "<a href=\"https://example.com\" target=\"_blank\">Link</a>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := cleanHTMLAttributes(tc.input)
			if result != tc.expected {
				t.Errorf("cleanHTMLAttributes() = %q, want %q", result, tc.expected)
			}
		})
	}
}