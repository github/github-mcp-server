// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	internaljson "github.com/modelcontextprotocol/go-sdk/internal/json"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

const (
	protocolVersionHeader        = "Mcp-Protocol-Version"
	sessionIDHeader              = "Mcp-Session-Id"
	lastEventIDHeader            = "Last-Event-ID"
	methodHeader                 = "Mcp-Method"
	nameHeader                   = "Mcp-Name"
	paramHeaderPrefix            = "Mcp-Param-"
	minVersionForStandardHeaders = protocolVersion20260728
	base64Prefix                 = "=?base64?"
	base64Suffix                 = "?="
)

func extractName(method string, params json.RawMessage) (string, bool) {
	switch method {
	case "tools/call":
		var p CallToolParams
		if err := internaljson.Unmarshal(params, &p); err == nil {
			return p.Name, true
		}
	case "prompts/get":
		var p GetPromptParams
		if err := internaljson.Unmarshal(params, &p); err == nil {
			return p.Name, true
		}
	case "resources/read":
		var p ReadResourceParams
		if err := internaljson.Unmarshal(params, &p); err == nil {
			return p.URI, true
		}
	}

	return "", false
}

// headerSchemaProperty captures the fields needed for x-mcp-header processing.
type headerSchemaProperty struct {
	Type       string                          `json:"type"`
	XMCPHeader json.RawMessage                 `json:"x-mcp-header,omitempty"`
	Properties map[string]headerSchemaProperty `json:"properties,omitempty"`
}

// unmarshalSchemaProperties normalizes any InputSchema type
// (*jsonschema.Schema, map[string]any, or json.RawMessage) into a common
// representation by marshaling to JSON and unmarshaling only the fields we need.
func unmarshalSchemaProperties(schema any) map[string]headerSchemaProperty {
	var s headerSchemaProperty
	if err := remarshal(schema, &s); err != nil {
		return nil
	}
	return s.Properties
}

// paramHeaderBinding maps a (possibly nested) input-schema property to the
// HTTP header it carries.
type paramHeaderBinding struct {
	Path   []string
	Header string
}

// extractParamHeaderAnnotations returns the bindings for every property in
// the tool's InputSchema that has an x-mcp-header annotation
func extractParamHeaderAnnotations(tool *Tool) []paramHeaderBinding {
	props := unmarshalSchemaProperties(tool.InputSchema)
	if len(props) == 0 {
		return nil
	}
	var result []paramHeaderBinding
	result = collectParamHeaderAnnotations(props, nil, result)
	if len(result) == 0 {
		return nil
	}
	return result
}

// collectParamHeaderAnnotations walks the schema properties and records every
// x-mcp-header annotation it finds, keyed by the property-name path.
func collectParamHeaderAnnotations(props map[string]headerSchemaProperty, prefix []string, out []paramHeaderBinding) []paramHeaderBinding {
	for propName, prop := range props {
		path := make([]string, len(prefix)+1)
		copy(path, prefix)
		path[len(prefix)] = propName

		var headerName string
		if err := json.Unmarshal(prop.XMCPHeader, &headerName); err == nil && headerName != "" {
			out = append(out, paramHeaderBinding{Path: path, Header: headerName})
		}
		if len(prop.Properties) > 0 {
			out = collectParamHeaderAnnotations(prop.Properties, path, out)
		}
	}
	return out
}

// lookupArgument navigates the arguments object using the given property-name
// path and returns the raw JSON value at that location. It reports whether
// the value was found.
func lookupArgument(args map[string]json.RawMessage, path []string) (json.RawMessage, bool) {
	if len(path) == 0 {
		return nil, false
	}
	cur, ok := args[path[0]]
	if !ok {
		return nil, false
	}
	for _, part := range path[1:] {
		var obj map[string]json.RawMessage
		if err := internaljson.Unmarshal(cur, &obj); err != nil {
			return nil, false
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// maxSafeInteger and minSafeInteger bound the integer values that can be
// faithfully represented as IEEE-754 double-precision floats.
const (
	maxSafeInteger = 1<<53 - 1    // 2^53 - 1 = 9007199254740991
	minSafeInteger = -(1<<53 - 1) // -(2^53 - 1) = -9007199254740991
)

// unmarshalPrimitive unmarshals a JSON value into the Go representation used
// for x-mcp-header processing per SEP-2243:
//
//   - JSON string  -> string
//   - JSON boolean -> bool
//   - JSON integer (within the JavaScript safe-integer range) -> int64
//
// JSON numbers that are non-integers (have a fractional part, NaN, or ±Inf)
// or integers outside the safe range are rejected because the `number` type
// is not permitted for x-mcp-header parameters; only integer, string, boolean
// are allowed.
func unmarshalPrimitive(raw json.RawMessage) any {
	var val any
	if err := internaljson.Unmarshal(raw, &val); err != nil {
		return nil
	}
	switch v := val.(type) {
	case string, bool:
		return v
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
			return nil
		}
		if v < minSafeInteger || v > maxSafeInteger {
			return nil
		}
		return int64(v)
	default:
		return nil
	}
}

// primitiveToString formats an x-mcp-header value (as produced by
// [unmarshalPrimitive]) to its canonical header string representation per
// SEP-2243. Returns false if value is not one of the permitted primitive
// types (string, bool, int64).
func primitiveToString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		return fmt.Sprintf("%t", v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	default:
		return "", false
	}
}

// setStandardHeaders populates standard MCP headers.
// It requires the protocol version header to be set.
func setStandardHeaders(ctx context.Context, header http.Header, msg jsonrpc.Message) {
	if msg == nil {
		return
	}
	if header.Get(protocolVersionHeader) == "" || header.Get(protocolVersionHeader) < minVersionForStandardHeaders {
		return
	}

	switch msg := msg.(type) {
	case *jsonrpc.Request:
		header.Set(methodHeader, msg.Method)
		if name, ok := extractName(msg.Method, msg.Params); ok {
			header.Set(nameHeader, name)
		}
		if msg.Method == "tools/call" {
			if tool, ok := ctx.Value(toolContextKey).(*Tool); ok && tool != nil {
				for k, v := range generateParamHeaders(tool, msg.Params) {
					header.Set(k, v)
				}
			}
		}
	}
}

// generateParamHeaders reads x-mcp-header annotations from the tool's InputSchema
// and returns the Mcp-Param-{Name} headers to be set on the HTTP request.
func generateParamHeaders(tool *Tool, params json.RawMessage) map[string]string {
	paramHeaders := extractParamHeaderAnnotations(tool)
	if len(paramHeaders) == 0 {
		return nil
	}

	var raw struct {
		Arguments map[string]json.RawMessage `json:"arguments"`
	}
	if err := internaljson.Unmarshal(params, &raw); err != nil || raw.Arguments == nil {
		return nil
	}

	res := make(map[string]string)
	for _, b := range paramHeaders {
		argRaw, ok := lookupArgument(raw.Arguments, b.Path)
		if !ok {
			continue
		}
		if string(argRaw) == "null" {
			continue
		}
		val := unmarshalPrimitive(argRaw)
		if val == nil {
			continue
		}
		encoded, ok := encodeHeaderValue(val)
		if !ok {
			continue
		}
		res[paramHeaderPrefix+b.Header] = encoded
	}
	return res
}

// filterValidTools returns only tools that have valid
// x-mcp-header annotations. Invalid tools are logged and excluded.
func filterValidTools(logger *slog.Logger, tools []*Tool) []*Tool {
	logger = ensureLogger(logger)
	result := make([]*Tool, 0, len(tools))
	for _, tool := range tools {
		if err := validateParamHeaderAnnotations(tool); err != nil {
			logger.Error("excluding tool from tools/list", "tool", tool.Name, "error", err)
			continue
		}
		result = append(result, tool)
	}
	return result
}

// validateParamHeaderAnnotations checks that a tool's x-mcp-header annotations
// are valid. Annotations may appear on properties at any nesting
// depth within the inputSchema and must be unique across all of them.
func validateParamHeaderAnnotations(tool *Tool) error {
	props := unmarshalSchemaProperties(tool.InputSchema)
	if len(props) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	return validateParamHeadersIn(props, "", seen)
}

func validateParamHeadersIn(props map[string]headerSchemaProperty, prefix string, seen map[string]bool) error {
	for propName, prop := range props {
		path := propName
		if prefix != "" {
			path = prefix + "." + propName
		}
		if prop.XMCPHeader != nil {
			if prop.Type != "string" && prop.Type != "integer" && prop.Type != "boolean" {
				return fmt.Errorf("property %q: x-mcp-header can only be applied to primitive types (integer, string, boolean), got %q", path, prop.Type)
			}
			var headerName string
			if err := json.Unmarshal(prop.XMCPHeader, &headerName); err != nil || headerName == "" {
				return fmt.Errorf("property %q: x-mcp-header must be a non-empty string", path)
			}
			if err := validateHeaderName(headerName); err != nil {
				return fmt.Errorf("property %q: %w", path, err)
			}
			lower := strings.ToLower(headerName)
			if seen[lower] {
				return fmt.Errorf("property %q: duplicate x-mcp-header value %q (case-insensitive)", path, headerName)
			}
			seen[lower] = true
		}
		if len(prop.Properties) > 0 {
			if err := validateParamHeadersIn(prop.Properties, path, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateHeaderName checks that a header name matches the HTTP field-name
// token syntax (1*tchar).
func validateHeaderName(name string) error {
	if name == "" {
		return fmt.Errorf("x-mcp-header value must be a non-empty string")
	}
	for _, c := range name {
		if !isTChar(c) {
			return fmt.Errorf("x-mcp-header value %q contains invalid character %q", name, c)
		}
	}
	return nil
}

// isTChar reports whether c is a valid HTTP token character (tchar)
//
//	tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" / "+" / "-" / "." /
//	        "^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA
func isTChar(c rune) bool {
	switch {
	case c >= '0' && c <= '9':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'z':
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.',
		'^', '_', '`', '|', '~':
		return true
	}
	return false
}

func validateMcpHeaders(header http.Header, msg jsonrpc.Message, toolLookup func(string) (*serverTool, bool)) error {
	protocolVersion := header.Get(protocolVersionHeader)
	if protocolVersion == "" || protocolVersion < minVersionForStandardHeaders {
		return nil
	}

	switch msg := msg.(type) {
	case *jsonrpc.Request:
		methodInHeader := header.Get(methodHeader)
		if methodInHeader == "" {
			return errors.New("missing required Mcp-Method header")
		}
		if methodInHeader != msg.Method {
			return fmt.Errorf("header mismatch: Mcp-Method header value '%s' does not match body value '%s'", methodInHeader, msg.Method)
		}

		var nameInBody string
		if msg.Method == "tools/call" || msg.Method == "resources/read" || msg.Method == "prompts/get" {
			nameInHeader := header.Get(nameHeader)
			if nameInHeader == "" {
				return fmt.Errorf("missing required Mcp-Name header for method %q", msg.Method)
			}
			var ok bool
			nameInBody, ok = extractName(msg.Method, msg.Params)
			if !ok {
				return fmt.Errorf("failed to extract name from parameters for method %q", msg.Method)
			}
			if nameInHeader != nameInBody {
				return fmt.Errorf("header mismatch: Mcp-Name header value '%s' does not match body value '%s'", nameInHeader, nameInBody)
			}
		}

		if msg.Method == "tools/call" && toolLookup != nil {
			if st, ok := toolLookup(nameInBody); ok && st != nil {
				if err := validateParamHeaders(header, msg, st.tool); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateParamHeaders(header http.Header, msg *jsonrpc.Request, tool *Tool) error {
	paramHeaders := extractParamHeaderAnnotations(tool)
	if len(paramHeaders) == 0 {
		return nil
	}

	var raw struct {
		Arguments map[string]json.RawMessage `json:"arguments"`
	}
	if err := internaljson.Unmarshal(msg.Params, &raw); err != nil {
		return nil
	}

	for _, b := range paramHeaders {
		fullHeader := paramHeaderPrefix + b.Header
		headerVal := header.Get(fullHeader)
		argRaw, argExists := lookupArgument(raw.Arguments, b.Path)

		if !argExists || string(argRaw) == "null" {
			if headerVal != "" {
				return fmt.Errorf("header mismatch: unexpected %s header for absent or null parameter %q", fullHeader, strings.Join(b.Path, "."))
			}
			continue
		}

		if headerVal == "" {
			return fmt.Errorf("header mismatch: missing %s header for parameter %q", fullHeader, strings.Join(b.Path, "."))
		}

		decoded, ok := decodeHeaderValue(headerVal)
		if !ok {
			return fmt.Errorf("header mismatch: %s header contains invalid Base64 encoding", fullHeader)
		}

		bodyVal := unmarshalPrimitive(argRaw)
		if bodyVal == nil {
			return fmt.Errorf("header mismatch: %s header present but body parameter %q is not a primitive type", fullHeader, strings.Join(b.Path, "."))
		}

		if !primitiveEqual(decoded, bodyVal) {
			return fmt.Errorf("header mismatch: %s header value '%s' does not match body value", fullHeader, headerVal)
		}
	}
	return nil
}

// primitiveEqual reports whether the (decoded) header string equals the
// JSON-derived body value.
func primitiveEqual(headerStr string, bodyVal any) bool {
	if bodyInt, ok := bodyVal.(int64); ok {
		headerNum, err := strconv.ParseFloat(headerStr, 64)
		if err != nil {
			return false
		}
		if math.IsNaN(headerNum) || math.IsInf(headerNum, 0) || headerNum != math.Trunc(headerNum) {
			return false
		}
		if headerNum < minSafeInteger || headerNum > maxSafeInteger {
			return false
		}
		return int64(headerNum) == bodyInt
	}
	expected, ok := primitiveToString(bodyVal)
	if !ok {
		return false
	}
	return headerStr == expected
}

// encodeHeaderValue converts a parameter value to an HTTP header-safe string
// per the SEP-2243 encoding rules:
//   - string: used as-is if safe ASCII, otherwise Base64 encoded
//   - int64:  decimal string representation
//   - bool:   lowercase "true" or "false"
//
// Values that contain non-ASCII characters, control characters, or
// leading/trailing whitespace are Base64-encoded with the =?base64?...?= wrapper.
//
// The second return value is false if the value is not a supported primitive type.
func encodeHeaderValue(value any) (string, bool) {
	s, ok := primitiveToString(value)
	if !ok {
		return "", false
	}
	if requiresBase64Encoding(s) {
		return encodeBase64(s), true
	}
	return s, true
}

// decodeHeaderValue decodes a header value that may be Base64-encoded
// with the =?base64?...?= wrapper.
//
// The second return value is false if the header value is not a valid Base64 encoded value.
func decodeHeaderValue(headerValue string) (string, bool) {
	if len(headerValue) == 0 {
		return headerValue, true
	}

	if encoded, ok := strings.CutPrefix(headerValue, base64Prefix); ok {
		if encoded, ok = strings.CutSuffix(encoded, base64Suffix); ok {
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return "", false
			}
			return string(decoded), true
		}
	}
	return headerValue, true
}

func requiresBase64Encoding(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == ' ' || s[0] == '\t' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t' {
		return true
	}
	for _, c := range s {
		if c < 0x20 || c > 0x7E {
			return true
		}
	}
	// Per SEP-2243, plain-ASCII values that match the base64 sentinel pattern
	// must also be base64-encoded to avoid ambiguity with already-encoded values.
	if strings.HasPrefix(s, base64Prefix) && strings.HasSuffix(s, base64Suffix) {
		return true
	}
	return false
}

func encodeBase64(s string) string {
	return base64Prefix + base64.StdEncoding.EncodeToString([]byte(s)) + base64Suffix
}
