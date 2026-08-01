package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

var (
	// Color is a valid hex color or name of a web safe color
	Color = regexp.MustCompile(`(?i)^(#[0-9a-fA-F]{1,6}|black|silver|gray|white|maroon|red|purple|fuchsia|green|lime|olive|yellow|navy|blue|teal|aqua|orange|aliceblue|antiquewhite|aquamarine|azure|beige|bisque|blanchedalmond|blueviolet|brown|burlywood|cadetblue|chartreuse|chocolate|coral|cornflowerblue|cornsilk|crimson|darkblue|darkcyan|darkgoldenrod|darkgray|darkgreen|darkgrey|darkkhaki|darkmagenta|darkolivegreen|darkorange|darkorchid|darkred|darksalmon|darkseagreen|darkslateblue|darkslategray|darkslategrey|darkturquoise|darkviolet|deeppink|deepskyblue|dimgray|dimgrey|dodgerblue|firebrick|floralwhite|forestgreen|gainsboro|ghostwhite|gold|goldenrod|greenyellow|grey|honeydew|hotpink|indianred|indigo|ivory|khaki|lavender|lavenderblush|lawngreen|lemonchiffon|lightblue|lightcoral|lightcyan|lightgoldenrodyellow|lightgray|lightgreen|lightgrey|lightpink|lightsalmon|lightseagreen|lightskyblue|lightslategray|lightslategrey|lightsteelblue|lightyellow|limegreen|linen|mediumaquamarine|mediumblue|mediumorchid|mediumpurple|mediumseagreen|mediumslateblue|mediumspringgreen|mediumturquoise|mediumvioletred|midnightblue|mintcream|mistyrose|moccasin|navajowhite|oldlace|olivedrab|orangered|orchid|palegoldenrod|palegreen|paleturquoise|palevioletred|papayawhip|peachpuff|peru|pink|plum|powderblue|rosybrown|royalblue|saddlebrown|salmon|sandybrown|seagreen|seashell|sienna|skyblue|slateblue|slategray|slategrey|snow|springgreen|steelblue|tan|thistle|tomato|turquoise|violet|wheat|whitesmoke|yellowgreen|rebeccapurple)$`)

	// ButtonType is a button type, or a style type, i.e. "submit"
	ButtonType = regexp.MustCompile(`(?i)^[a-zA-Z][a-zA-Z-]{1,30}[a-zA-Z]$`)

	// StyleType is the valid type attribute on a style tag in the <head>
	StyleType = regexp.MustCompile(`(?i)^text\/css$`)
)

func main() {
	// Define a policy, we are using the UGC policy as a base.
	p := bluemonday.UGCPolicy()

	// HTML email is often displayed in iframes and needs to preserve core
	// structure
	p.AllowElements("html", "head", "body", "title")

	// There are not safe, and is only being done here to demonstrate how to
	// process HTML emails where styling has to be preserved. This is at the
	// expense of security.
	p.AllowAttrs("type").Matching(StyleType).OnElements("style")
	p.AllowAttrs("style").Globally()

	// HTML email frequently contains obselete and basic HTML
	p.AllowElements("font", "main", "nav", "header", "footer", "kbd", "legend")

	// Need to permit the style tag, and buttons are often found in emails (why?)
	p.AllowAttrs("type").Matching(ButtonType).OnElements("button")

	// HTML email tends to see the use of obselete spacing and styling attributes
	p.AllowAttrs("bgcolor", "color").Matching(Color).OnElements("basefont", "font", "hr")
	p.AllowAttrs("border").Matching(bluemonday.Integer).OnElements("img", "table")
	p.AllowAttrs("cellpadding", "cellspacing").Matching(bluemonday.Integer).OnElements("table")

	// Allow "class" attributes on all elements
	p.AllowStyling()

	// Allow images to be embedded via data-uri
	p.AllowDataURIImages()

	// Add "rel=nofollow" to links
	p.RequireNoFollowOnLinks(true)
	p.RequireNoFollowOnFullyQualifiedLinks(true)

	// Open external links in a new window/tab
	p.AddTargetBlankToFullyQualifiedLinks(true)

	// Read input from stdin so that this is a nice unix utility and can receive
	// piped input
	dirty, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	// Apply the policy and write to stdout
	fmt.Fprint(
		os.Stdout,
		p.Sanitize(
			string(dirty),
		),
	)
}
