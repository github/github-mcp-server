package binding

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bindTool produces a scoped copy of a tool: its advertised schema has the
// bound, rejected, and disallowed-method fields removed, its description is
// rewritten for the bound context, and its handler injects the fixed values
// and enforces the boundary at call time. The original ServerTool (a
// package-level singleton) is never mutated.
func bindTool(st inventory.ServerTool, tb ToolBinding, ctx Context) (inventory.ServerTool, error) {
	schema, ok := st.Tool.InputSchema.(*jsonschema.Schema)
	if !ok {
		return inventory.ServerTool{}, fmt.Errorf("tool %q: unexpected input schema type %T", st.Tool.Name, st.Tool.InputSchema)
	}

	newSchema, err := transformSchema(schema, tb)
	if err != nil {
		return inventory.ServerTool{}, fmt.Errorf("tool %q: %w", st.Tool.Name, err)
	}

	bound := st // shallow struct copy; Tool is a value, so edits below are local
	bound.Tool.InputSchema = newSchema
	if tb.Description != "" {
		desc, err := renderDescription(tb.Description, ctx)
		if err != nil {
			return inventory.ServerTool{}, fmt.Errorf("tool %q: %w", st.Tool.Name, err)
		}
		bound.Tool.Description = desc
	}
	if tb.Title != "" {
		bound.Tool.Title = tb.Title
	}
	bound.HandlerFunc = wrapHandler(st.HandlerFunc, tb, ctx)
	return bound, nil
}

// renderDescription expands a manifest description against the bound context so
// it names the concrete resource (e.g. "octocat/hello-world"). A description is
// a Go text/template with the Context in scope, exposing the RepoRef/PullRef/
// ProjectRef helpers and the Context fields. Plain descriptions (no template
// actions) are returned unchanged. A malformed template is a manifest bug and
// fails loudly at bind time.
func renderDescription(tmpl string, ctx Context) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}
	t, err := template.New("description").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid description template %q: %w", tmpl, err)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, ctx); err != nil {
		return "", fmt.Errorf("rendering description %q: %w", tmpl, err)
	}
	return sb.String(), nil
}

// transformSchema returns a deep copy of the tool's input schema with bound and
// rejected parameters removed and the method enum narrowed to the allowed set.
func transformSchema(orig *jsonschema.Schema, tb ToolBinding) (*jsonschema.Schema, error) {
	s := orig.CloneSchemas()

	remove := func(name string) {
		delete(s.Properties, name)
		s.Required = removeString(s.Required, name)
	}

	for param := range tb.Bind {
		if _, ok := s.Properties[param]; !ok {
			return nil, fmt.Errorf("bound parameter %q is not present in the tool schema", param)
		}
		remove(param)
	}
	for _, param := range tb.ParamReject {
		if _, ok := s.Properties[param]; !ok {
			return nil, fmt.Errorf("rejected parameter %q is not present in the tool schema", param)
		}
		remove(param)
	}

	if len(tb.MethodAllow) > 0 || len(tb.MethodDeny) > 0 {
		method, ok := s.Properties["method"]
		if !ok {
			return nil, fmt.Errorf("a method allow/deny list is set but the tool schema has no %q parameter", "method")
		}
		narrowed, err := narrowEnum(method.Enum, tb.MethodAllow, tb.MethodDeny)
		if err != nil {
			return nil, err
		}
		method.Enum = narrowed
	}

	if len(s.Required) == 0 {
		s.Required = nil
	}
	return s, nil
}

// narrowEnum returns the advertised method enum after applying a manifest's
// allow and deny lists. It keeps the original values (preserving their order)
// that survive both filters: a value is kept if it is not denied and, when an
// allow list is given, is in it. Every allow and deny value must exist in the
// original enum, so a stale manifest entry fails loudly rather than silently
// advertising — or pretending to remove — a method the tool does not implement.
func narrowEnum(enum []any, allow, deny []string) ([]any, error) {
	enumSet := make(map[string]bool, len(enum))
	for _, e := range enum {
		if s, ok := e.(string); ok {
			enumSet[s] = true
		}
	}

	denySet := make(map[string]bool, len(deny))
	for _, d := range deny {
		if !enumSet[d] {
			return nil, fmt.Errorf("denied method %q is not one of the tool's methods", d)
		}
		denySet[d] = true
	}

	var allowSet map[string]bool
	if len(allow) > 0 {
		allowSet = make(map[string]bool, len(allow))
		for _, a := range allow {
			if !enumSet[a] {
				return nil, fmt.Errorf("allowed method %q is not one of the tool's methods", a)
			}
			allowSet[a] = true
		}
	}

	var out []any
	for _, e := range enum {
		s, ok := e.(string)
		if !ok {
			continue
		}
		if denySet[s] {
			continue
		}
		if allowSet != nil && !allowSet[s] {
			continue
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("method enum is empty after narrowing")
	}
	return out, nil
}

// wrapHandler returns a HandlerFunc that validates and injects the bound
// context before delegating to the original handler. It rejects any
// caller-supplied value for a fixed parameter, enforces the method allow/deny
// lists (the SDK does not validate the narrowed enum), guards search queries,
// and injects the fixed values into the raw arguments.
func wrapHandler(orig inventory.HandlerFunc, tb ToolBinding, ctx Context) inventory.HandlerFunc {
	return func(deps any) mcp.ToolHandler {
		inner := orig(deps)
		return func(c context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := map[string]any{}
			if raw := req.Params.Arguments; len(raw) > 0 {
				if err := json.Unmarshal(raw, &args); err != nil {
					return toolError("invalid arguments: %s", err), nil
				}
				if args == nil { // arguments were JSON null
					args = map[string]any{}
				}
			}

			for param := range tb.Bind {
				if _, supplied := args[param]; supplied {
					return toolError("parameter %q is fixed by this server and must not be supplied", param), nil
				}
			}
			for _, param := range tb.ParamReject {
				if _, supplied := args[param]; supplied {
					return toolError("parameter %q is not available on this server", param), nil
				}
			}

			if len(tb.MethodAllow) > 0 || len(tb.MethodDeny) > 0 {
				method, ok := args["method"].(string)
				if !ok || method == "" {
					return toolError("parameter %q is required on this server", "method"), nil
				}
				if !methodPermitted(method, tb) {
					return toolError("method %q is not available on this server", method), nil
				}
			}

			if tb.QueryGuard {
				if q, ok := args["query"].(string); ok && queryCanEscapeScope(q) {
					return toolError("search query may not contain repo:, org:, or user: qualifiers or boolean grouping on this server"), nil
				}
			}

			for param, key := range tb.Bind {
				v, ok := ctx.value(key)
				if !ok {
					return toolError("server misconfigured: no value bound for parameter %q", param), nil
				}
				args[param] = v
			}

			raw, err := json.Marshal(args)
			if err != nil {
				return toolError("failed to encode arguments: %s", err), nil
			}

			// Copy the request and params so the caller's request is untouched.
			newParams := *req.Params
			newParams.Arguments = raw
			newReq := *req
			newReq.Params = &newParams
			return inner(c, &newReq)
		}
	}
}

func methodPermitted(method string, tb ToolBinding) bool {
	if slices.Contains(tb.MethodDeny, method) {
		return false
	}
	if len(tb.MethodAllow) > 0 && !slices.Contains(tb.MethodAllow, method) {
		return false
	}
	return true
}

// crossContextQualifiers are GitHub search qualifiers that can redirect a query
// at a different owner/repo/user than the bound context. "-repo:" and friends
// contain these substrings and are caught too.
var crossContextQualifiers = []string{"repo:", "org:", "user:"}

// booleanOperators are the GitHub search boolean keywords. A scoped search
// prepends a "repo:<bound>" qualifier to the caller's query; a disjunction or
// negation could move part of the query outside that qualifier and search other
// contexts, so any of these (and grouping parentheses) is rejected outright
// rather than rewritten.
var booleanOperators = map[string]bool{"OR": true, "AND": true, "NOT": true}

// queryCanEscapeScope reports whether a search query could reach beyond the
// bound context, either via an explicit cross-context qualifier or via boolean
// grouping that would not inherit the injected repo: qualifier.
func queryCanEscapeScope(query string) bool {
	return hasCrossContextQualifier(query) || hasBooleanGrouping(query)
}

func hasCrossContextQualifier(query string) bool {
	lower := strings.ToLower(query)
	for _, qualifier := range crossContextQualifiers {
		if strings.Contains(lower, qualifier) {
			return true
		}
	}
	return false
}

func hasBooleanGrouping(query string) bool {
	if strings.ContainsAny(query, "()") {
		return true
	}
	for field := range strings.FieldsSeq(query) {
		if booleanOperators[field] {
			return true
		}
	}
	return false
}

func removeString(ss []string, target string) []string {
	if len(ss) == 0 {
		return ss
	}
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s != target {
			out = append(out, s)
		}
	}
	return out
}

func toolError(format string, a ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, a...)}},
		IsError: true,
	}
}
