package github

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
)

// Output schemas for tools whose response shape varies by the `method`
// argument. Hand-authored, because no single Go type describes a
// method-dispatched tool's output, and kept as .json so they stay reviewable
// and diffable. Every union uses anyOf, never oneOf — see AnyOfSchema.
//
//go:embed output_schemas/*.json
var outputSchemaFS embed.FS

var (
	// Object-root schemas: legal under every protocol revision that supports
	// outputSchema at all, so these ship to every client ungated even though
	// they use anyOf internally.
	actionsGetOutputSchema             = mustLoadOutputSchema("actions_get")
	actionsListOutputSchema            = mustLoadOutputSchema("actions_list")
	actionsRunTriggerOutputSchema      = mustLoadOutputSchema("actions_run_trigger")
	discussionCommentWriteOutputSchema = mustLoadOutputSchema("discussion_comment_write")

	// Not actually a union: both methods of issue_dependency_read return the
	// same shape, so this is a single object schema.
	issueDependencyReadOutputSchema = mustLoadOutputSchema("issue_dependency_read")

	// Non-object roots (bare anyOf spanning objects and arrays): gated to
	// clients speaking 2026-07-28 or later.
	issueReadOutputSchema       = mustLoadOutputSchema("issue_read")
	pullRequestReadOutputSchema = mustLoadOutputSchema("pull_request_read")
)

// mustLoadOutputSchema reads an embedded schema, compacts it, and verifies it
// resolves, panicking at init so a malformed schema fails the build.
//
// Compacting also strips inter-token whitespace, so a checkout that rewrote
// line endings (core.autocrlf on Windows, which is what corrupts
// pkg/octicons' embedded data URIs) cannot leak carriage returns to clients.
func mustLoadOutputSchema(name string) json.RawMessage {
	raw, err := outputSchemaFS.ReadFile(fmt.Sprintf("output_schemas/%s.json", name))
	if err != nil {
		panic(fmt.Sprintf("output schema %q: %v", name, err))
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		panic(fmt.Sprintf("output schema %q: %v", name, err))
	}
	return MustRawOutputSchema(compact.String())
}
