package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/microcosm-cc/bluemonday"
)

func main() {
	// Define a policy, we are using the UGC policy as a base.
	p := bluemonday.UGCPolicy()

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
