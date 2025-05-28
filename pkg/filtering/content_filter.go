package filtering

import (
	"regexp"
	"strings"
)

var (
	// Invisible Unicode characters
	// This includes zero-width spaces, zero-width joiners, zero-width non-joiners, 
	// bidirectional marks, and other invisible unicode characters
	invisibleCharsRegex = regexp.MustCompile(`[\x{200B}-\x{200F}\x{2028}-\x{202E}\x{2060}-\x{2064}\x{FEFF}]`)

	// HTML comments
	htmlCommentsRegex = regexp.MustCompile(`<!--[\s\S]*?-->`)

	// HTML elements that could contain hidden content
	// This is a simple approach that targets specific dangerous tags
	// Go's regexp doesn't support backreferences, so we list each tag explicitly
	htmlScriptRegex = regexp.MustCompile(`<script[^>]*>[\s\S]*?</script>`)
	htmlStyleRegex = regexp.MustCompile(`<style[^>]*>[\s\S]*?</style>`)
	htmlIframeRegex = regexp.MustCompile(`<iframe[^>]*>[\s\S]*?</iframe>`)
	htmlObjectRegex = regexp.MustCompile(`<object[^>]*>[\s\S]*?</object>`)
	htmlEmbedRegex = regexp.MustCompile(`<embed[^>]*>[\s\S]*?</embed>`)
	htmlSvgRegex = regexp.MustCompile(`<svg[^>]*>[\s\S]*?</svg>`)
	htmlMathRegex = regexp.MustCompile(`<math[^>]*>[\s\S]*?</math>`)
	htmlLinkRegex = regexp.MustCompile(`<link[^>]*>[\s\S]*?</link>`)

	// HTML attributes that might be used for hiding content
	htmlAttributesRegex = regexp.MustCompile(`<[^>]*(?:style|data-[\w-]+|hidden|class)="[^"]*"[^>]*>`)

	// Detect collapsed sections (details/summary)
	collapsedSectionsRegex = regexp.MustCompile(`<details>[\s\S]*?</details>`)

	// Very small text (font-size or similar CSS tricks)
	smallTextRegex = regexp.MustCompile(`<[^>]*style="[^"]*font-size:\s*(?:0|0\.\d+|[0-3])(?:px|pt|em|%)[^"]*"[^>]*>[\s\S]*?</[^>]+>`)

	// Excessive whitespace (more than 3 consecutive newlines)
	excessiveWhitespaceRegex = regexp.MustCompile(`\n{4,}`)
	
	// Excessive spaces (15 or more consecutive spaces)
	excessiveSpacesRegex = regexp.MustCompile(` {15,}`)
	
	// Excessive tabs (6 or more consecutive tabs)
	excessiveTabsRegex = regexp.MustCompile(`\t{6,}`)
)

// Config holds configuration for content filtering
type Config struct {
	// DisableContentFiltering disables all content filtering when true
	DisableContentFiltering bool
}

// DefaultConfig returns the default content filtering configuration
func DefaultConfig() *Config {
	return &Config{
		DisableContentFiltering: false,
	}
}

// FilterContent filters potentially hidden content from the input text
// This includes invisible Unicode characters, HTML comments, and other methods of hiding content
func FilterContent(input string, cfg *Config) string {
	if cfg != nil && cfg.DisableContentFiltering {
		return input
	}

	if input == "" {
		return input
	}

	// Process the input text through each filter
	result := input

	// Remove invisible characters
	result = invisibleCharsRegex.ReplaceAllString(result, "")

	// Replace HTML comments with a marker
	result = htmlCommentsRegex.ReplaceAllString(result, "[HTML_COMMENT]")

	// Replace potentially dangerous HTML elements
	result = htmlScriptRegex.ReplaceAllString(result, "[HTML_ELEMENT]")
	result = htmlStyleRegex.ReplaceAllString(result, "[HTML_ELEMENT]")
	result = htmlIframeRegex.ReplaceAllString(result, "[HTML_ELEMENT]")
	result = htmlObjectRegex.ReplaceAllString(result, "[HTML_ELEMENT]")
	result = htmlEmbedRegex.ReplaceAllString(result, "[HTML_ELEMENT]")
	result = htmlSvgRegex.ReplaceAllString(result, "[HTML_ELEMENT]")
	result = htmlMathRegex.ReplaceAllString(result, "[HTML_ELEMENT]")
	result = htmlLinkRegex.ReplaceAllString(result, "[HTML_ELEMENT]")

	// Replace HTML attributes that might be used for hiding
	result = htmlAttributesRegex.ReplaceAllStringFunc(result, cleanHTMLAttributes)

	// Replace collapsed sections with visible indicator
	result = collapsedSectionsRegex.ReplaceAllStringFunc(result, makeCollapsedSectionVisible)

	// Replace very small text with visible indicator
	result = smallTextRegex.ReplaceAllString(result, "[SMALL_TEXT]")

	// Normalize excessive whitespace
	result = excessiveWhitespaceRegex.ReplaceAllString(result, "\n\n\n")
	
	// Normalize excessive spaces
	result = excessiveSpacesRegex.ReplaceAllString(result, "              ")
	
	// Normalize excessive tabs
	result = excessiveTabsRegex.ReplaceAllString(result, "     ")

	return result
}

// cleanHTMLAttributes removes potentially dangerous attributes from HTML tags
func cleanHTMLAttributes(tag string) string {
	// This is a simple implementation that removes style, data-* and hidden attributes
	// A more sophisticated implementation would parse the HTML and selectively remove attributes
	tagWithoutStyle := regexp.MustCompile(`\s+(?:style|data-[\w-]+|hidden|class)="[^"]*"`).ReplaceAllString(tag, "")
	return tagWithoutStyle
}

// makeCollapsedSectionVisible transforms a <details> section to make it visible
func makeCollapsedSectionVisible(detailsSection string) string {
	// Extract the summary if present
	summaryRegex := regexp.MustCompile(`<summary>(.*?)</summary>`)
	summaryMatches := summaryRegex.FindStringSubmatch(detailsSection)
	
	summary := "Collapsed section"
	if len(summaryMatches) > 1 {
		summary = summaryMatches[1]
	}

	// Extract the content (everything after </summary> and before </details>)
	parts := strings.SplitN(detailsSection, "</summary>", 2)
	content := detailsSection
	if len(parts) > 1 {
		content = parts[1]
		content = strings.TrimSuffix(content, "</details>")
	} else {
		// No summary tag found, remove the details tags
		content = strings.TrimPrefix(content, "<details>")
		content = strings.TrimSuffix(content, "</details>")
	}

	// Format as a visible section
	return "\n\n**" + summary + ":**\n" + content + "\n\n"
}