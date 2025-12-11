// deprecated_tool_aliases.go
package github

// DeprecatedToolAliases maps old tool names to their new canonical names.
// When tools are renamed, add an entry here to maintain backward compatibility.
// Users referencing the old name will receive the new tool with a deprecation warning.
//
// Example:
//
//	"get_issue": "issue_read",
//	"create_pr": "pull_request_create",
var DeprecatedToolAliases = map[string]string{
	// Add entries as tools are renamed
}
