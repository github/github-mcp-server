package github

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
)

// Output schemas for tools whose response shape varies by the `method`
// argument.
//
// These are hand-authored rather than inferred from Go types, because no
// single Go type describes a method-dispatched tool's output. They live as
// .json files so they stay reviewable and diffable, and are embedded rather
// than pasted into Go string literals (their descriptions contain markdown
// backticks, which raw string literals cannot hold).
//
// Every union uses anyOf, never oneOf. oneOf requires EXACTLY ONE branch to
// match, which is provably wrong here: actions_run_trigger has four
// structurally identical branches, an empty array vacuously satisfies every
// array branch, and an issue_read `get` on a sub-issue also satisfies the
// `get_parent` branch. See AnyOfSchema in output_schema.go.
//
// Schemas whose root is not {"type":"object"} are legal only from protocol
// version 2026-07-28 (SEP-2106); inventory.OutputSchemaVersionGate strips them
// per-request for older clients. TestPolymorphicOutputSchemaRootKinds pins
// which schemas fall on which side of that line.
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
// resolves — panicking during package initialization on any failure so a
// malformed schema or dangling $ref fails the build rather than a request.
//
// Compacting is not cosmetic: it strips inter-token whitespace, so a checkout
// that rewrote the files' line endings (git's core.autocrlf does this on
// Windows, and it is what corrupts pkg/octicons' embedded data URIs) cannot
// leak stray carriage returns into what is sent to clients. It preserves
// string contents exactly, so descriptions are untouched.
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
