// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// TODO(findleyr): update JSON marshalling of all content types to preserve required fields.
// (See [TextContent.MarshalJSON], which handles this for text content).

package mcp

import (
	"encoding/json"
	"fmt"

	internaljson "github.com/modelcontextprotocol/go-sdk/internal/json"
)

// A Content is a [TextContent], [ImageContent], [AudioContent],
// [ResourceLink], [EmbeddedResource], [ToolUseContent], or [ToolResultContent].
//
// Note: [ToolUseContent] and [ToolResultContent] are only valid in sampling
// message contexts (CreateMessageParams/CreateMessageResult).
type Content interface {
	MarshalJSON() ([]byte, error)
	fromWire(*wireContent)
}

// TextContent is a textual content.
type TextContent struct {
	Text        string
	Meta        Meta
	Annotations *Annotations
}

func (c *TextContent) MarshalJSON() ([]byte, error) {
	// Custom wire format to ensure the required "text" field is always included, even when empty.
	wire := struct {
		Type        string       `json:"type"`
		Text        string       `json:"text"`
		Meta        Meta         `json:"_meta,omitempty"`
		Annotations *Annotations `json:"annotations,omitempty"`
	}{
		Type:        "text",
		Text:        c.Text,
		Meta:        c.Meta,
		Annotations: c.Annotations,
	}
	return json.Marshal(wire)
}

func (c *TextContent) fromWire(wire *wireContent) {
	c.Text = wire.Text
	c.Meta = wire.Meta
	c.Annotations = wire.Annotations
}

// ImageContent contains base64-encoded image data.
type ImageContent struct {
	Meta        Meta
	Annotations *Annotations
	Data        []byte // base64-encoded
	MIMEType    string
}

func (c *ImageContent) MarshalJSON() ([]byte, error) {
	// Custom wire format to ensure required fields are always included, even when empty.
	data := c.Data
	if data == nil {
		data = []byte{}
	}
	wire := imageAudioWire{
		Type:        "image",
		MIMEType:    c.MIMEType,
		Data:        data,
		Meta:        c.Meta,
		Annotations: c.Annotations,
	}
	return json.Marshal(wire)
}

func (c *ImageContent) fromWire(wire *wireContent) {
	c.MIMEType = wire.MIMEType
	c.Data = wire.Data
	c.Meta = wire.Meta
	c.Annotations = wire.Annotations
}

// AudioContent contains base64-encoded audio data.
type AudioContent struct {
	Data        []byte
	MIMEType    string
	Meta        Meta
	Annotations *Annotations
}

func (c AudioContent) MarshalJSON() ([]byte, error) {
	// Custom wire format to ensure required fields are always included, even when empty.
	data := c.Data
	if data == nil {
		data = []byte{}
	}
	wire := imageAudioWire{
		Type:        "audio",
		MIMEType:    c.MIMEType,
		Data:        data,
		Meta:        c.Meta,
		Annotations: c.Annotations,
	}
	return json.Marshal(wire)
}

func (c *AudioContent) fromWire(wire *wireContent) {
	c.MIMEType = wire.MIMEType
	c.Data = wire.Data
	c.Meta = wire.Meta
	c.Annotations = wire.Annotations
}

// Custom wire format to ensure required fields are always included, even when empty.
type imageAudioWire struct {
	Type        string       `json:"type"`
	MIMEType    string       `json:"mimeType"`
	Data        []byte       `json:"data"`
	Meta        Meta         `json:"_meta,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

// ResourceLink is a link to a resource
type ResourceLink struct {
	URI         string
	Name        string
	Title       string
	Description string
	MIMEType    string
	Size        *int64
	Meta        Meta
	Annotations *Annotations
	// Icons for the resource link, if any.
	Icons []Icon `json:"icons,omitempty"`
}

func (c *ResourceLink) MarshalJSON() ([]byte, error) {
	return json.Marshal(&wireContent{
		Type:        "resource_link",
		URI:         c.URI,
		Name:        c.Name,
		Title:       c.Title,
		Description: c.Description,
		MIMEType:    c.MIMEType,
		Size:        c.Size,
		Meta:        c.Meta,
		Annotations: c.Annotations,
		Icons:       c.Icons,
	})
}

func (c *ResourceLink) fromWire(wire *wireContent) {
	c.URI = wire.URI
	c.Name = wire.Name
	c.Title = wire.Title
	c.Description = wire.Description
	c.MIMEType = wire.MIMEType
	c.Size = wire.Size
	c.Meta = wire.Meta
	c.Annotations = wire.Annotations
	c.Icons = wire.Icons
}

// EmbeddedResource contains embedded resources.
type EmbeddedResource struct {
	Resource    *ResourceContents
	Meta        Meta
	Annotations *Annotations
}

func (c *EmbeddedResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(&wireContent{
		Type:        "resource",
		Resource:    c.Resource,
		Meta:        c.Meta,
		Annotations: c.Annotations,
	})
}

func (c *EmbeddedResource) fromWire(wire *wireContent) {
	c.Resource = wire.Resource
	c.Meta = wire.Meta
	c.Annotations = wire.Annotations
}

// ToolUseContent represents a request from the assistant to invoke a tool.
// This content type is only valid in sampling messages.
//
// Deprecated: the sampling feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
type ToolUseContent struct {
	// ID is a unique identifier for this tool use, used to match with ToolResultContent.
	ID string
	// Name is the name of the tool to invoke.
	Name string
	// Input contains the tool arguments as a JSON object.
	Input map[string]any
	Meta  Meta
}

func (c *ToolUseContent) MarshalJSON() ([]byte, error) {
	input := c.Input
	if input == nil {
		input = map[string]any{}
	}
	wire := struct {
		Type  string         `json:"type"`
		ID    string         `json:"id"`
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
		Meta  Meta           `json:"_meta,omitempty"`
	}{
		Type:  "tool_use",
		ID:    c.ID,
		Name:  c.Name,
		Input: input,
		Meta:  c.Meta,
	}
	return json.Marshal(wire)
}

func (c *ToolUseContent) fromWire(wire *wireContent) {
	c.ID = wire.ID
	c.Name = wire.Name
	c.Input = wire.Input
	c.Meta = wire.Meta
}

// ToolResultContent represents the result of a tool invocation.
// This content type is only valid in sampling messages with role "user".
//
// Deprecated: the sampling feature is deprecated as of protocol version
// 2026-07-28 (SEP-2577). It remains functional during the deprecation window
// (at least twelve months). See
// https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging.
type ToolResultContent struct {
	// ToolUseID references the ID from the corresponding ToolUseContent.
	ToolUseID string
	// Content holds the unstructured result of the tool call.
	Content []Content
	// StructuredContent holds an optional structured result. Per SEP-2106, it
	// may be any valid JSON value (object, array, or primitive) conforming to
	// the tool's output schema.
	StructuredContent any
	// IsError indicates whether the tool call ended in an error.
	IsError bool
	Meta    Meta
}

func (c *ToolResultContent) MarshalJSON() ([]byte, error) {
	// Marshal nested content
	var contentWire []*wireContent
	for _, content := range c.Content {
		data, err := content.MarshalJSON()
		if err != nil {
			return nil, err
		}
		var w wireContent
		if err := internaljson.Unmarshal(data, &w); err != nil {
			return nil, err
		}
		contentWire = append(contentWire, &w)
	}
	if contentWire == nil {
		contentWire = []*wireContent{} // avoid JSON null
	}

	wire := struct {
		Type              string         `json:"type"`
		ToolUseID         string         `json:"toolUseId"`
		Content           []*wireContent `json:"content"`
		StructuredContent any            `json:"structuredContent,omitempty"`
		IsError           bool           `json:"isError,omitempty"`
		Meta              Meta           `json:"_meta,omitempty"`
	}{
		Type:              "tool_result",
		ToolUseID:         c.ToolUseID,
		Content:           contentWire,
		StructuredContent: c.StructuredContent,
		IsError:           c.IsError,
		Meta:              c.Meta,
	}
	return json.Marshal(wire)
}

func (c *ToolResultContent) fromWire(wire *wireContent) {
	c.ToolUseID = wire.ToolUseID
	c.StructuredContent = wire.StructuredContent
	c.IsError = wire.IsError
	c.Meta = wire.Meta
	// Content is handled separately in contentFromWire due to nested content
}

// ResourceContents contains the contents of a specific resource or
// sub-resource.
type ResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     []byte `json:"blob,omitzero"`
	Meta     Meta   `json:"_meta,omitempty"`
}

// wireContent is the wire format for content.
// It represents the protocol types TextContent, ImageContent, AudioContent,
// ResourceLink, EmbeddedResource, ToolUseContent, and ToolResultContent.
// The Type field distinguishes them. In the protocol, each type has a constant
// value for the field.
type wireContent struct {
	Type              string            `json:"type"`
	Text              string            `json:"text,omitempty"`              // TextContent
	MIMEType          string            `json:"mimeType,omitempty"`          // ImageContent, AudioContent, ResourceLink
	Data              []byte            `json:"data,omitempty"`              // ImageContent, AudioContent
	Resource          *ResourceContents `json:"resource,omitempty"`          // EmbeddedResource
	URI               string            `json:"uri,omitempty"`               // ResourceLink
	Name              string            `json:"name,omitempty"`              // ResourceLink, ToolUseContent
	Title             string            `json:"title,omitempty"`             // ResourceLink
	Description       string            `json:"description,omitempty"`       // ResourceLink
	Size              *int64            `json:"size,omitempty"`              // ResourceLink
	Meta              Meta              `json:"_meta,omitempty"`             // all types
	Annotations       *Annotations      `json:"annotations,omitempty"`       // all types except ToolUseContent, ToolResultContent
	Icons             []Icon            `json:"icons,omitempty"`             // ResourceLink
	ID                string            `json:"id,omitempty"`                // ToolUseContent
	Input             map[string]any    `json:"input,omitempty"`             // ToolUseContent
	ToolUseID         string            `json:"toolUseId,omitempty"`         // ToolResultContent
	NestedContent     []*wireContent    `json:"content,omitempty"`           // ToolResultContent
	StructuredContent any               `json:"structuredContent,omitempty"` // ToolResultContent
	IsError           bool              `json:"isError,omitempty"`           // ToolResultContent
}

// unmarshalContent unmarshals JSON that is either a single content object or
// an array of content objects. A single object is wrapped in a one-element slice.
func unmarshalContent(raw json.RawMessage, allow map[string]bool) ([]Content, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("nil content")
	}
	// Try array first, then fall back to single object.
	var wires []*wireContent
	if err := internaljson.Unmarshal(raw, &wires); err == nil {
		return contentsFromWire(wires, allow)
	}
	var wire wireContent
	if err := internaljson.Unmarshal(raw, &wire); err != nil {
		return nil, err
	}
	c, err := contentFromWire(&wire, allow)
	if err != nil {
		return nil, err
	}
	return []Content{c}, nil
}

func contentsFromWire(wires []*wireContent, allow map[string]bool) ([]Content, error) {
	blocks := make([]Content, 0, len(wires))
	for _, wire := range wires {
		block, err := contentFromWire(wire, allow)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func contentFromWire(wire *wireContent, allow map[string]bool) (Content, error) {
	if wire == nil {
		return nil, fmt.Errorf("nil content")
	}
	if allow != nil && !allow[wire.Type] {
		return nil, fmt.Errorf("invalid content type %q", wire.Type)
	}
	switch wire.Type {
	case "text":
		v := new(TextContent)
		v.fromWire(wire)
		return v, nil
	case "image":
		v := new(ImageContent)
		v.fromWire(wire)
		return v, nil
	case "audio":
		v := new(AudioContent)
		v.fromWire(wire)
		return v, nil
	case "resource_link":
		v := new(ResourceLink)
		v.fromWire(wire)
		return v, nil
	case "resource":
		v := new(EmbeddedResource)
		v.fromWire(wire)
		return v, nil
	case "tool_use":
		v := new(ToolUseContent)
		v.fromWire(wire)
		return v, nil
	case "tool_result":
		v := new(ToolResultContent)
		v.fromWire(wire)
		// Handle nested content - tool_result content can contain text, image, audio,
		// resource_link, and resource (same as CallToolResult.content)
		if wire.NestedContent != nil {
			toolResultContentAllow := map[string]bool{
				"text": true, "image": true, "audio": true,
				"resource_link": true, "resource": true,
			}
			nestedContent, err := contentsFromWire(wire.NestedContent, toolResultContentAllow)
			if err != nil {
				return nil, fmt.Errorf("tool_result nested content: %w", err)
			}
			v.Content = nestedContent
		}
		return v, nil
	}
	return nil, fmt.Errorf("unrecognized content type %q", wire.Type)
}
