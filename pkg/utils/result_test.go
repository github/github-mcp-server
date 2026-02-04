package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"encoding/base64"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToolResultResourceWithFlag_DisabledFlag(t *testing.T) {
	// When flag is disabled, should return embedded resource (default behavior)
	contents := &mcp.ResourceContents{
		URI:      "test://file.txt",
		Text:     "Hello, World!",
		MIMEType: "text/plain",
	}

	result := NewToolResultResourceWithFlag("Test message", contents, false)

	require.NotNil(t, result)
	require.Len(t, result.Content, 2)
	assert.False(t, result.IsError)

	// First content should be text message
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "Test message", textContent.Text)

	// Second content should be embedded resource
	embeddedResource, ok := result.Content[1].(*mcp.EmbeddedResource)
	require.True(t, ok)
	assert.Equal(t, contents.URI, embeddedResource.Resource.URI)
	assert.Equal(t, contents.Text, embeddedResource.Resource.Text)
	assert.Equal(t, contents.MIMEType, embeddedResource.Resource.MIMEType)
}

func TestNewToolResultResourceWithFlag_EnabledFlag_TextContent(t *testing.T) {
	// When flag is enabled with text content, should return TextContent
	contents := &mcp.ResourceContents{
		URI:      "test://file.txt",
		Text:     "Hello, World!",
		MIMEType: "text/plain",
	}

	result := NewToolResultResourceWithFlag("Test message", contents, true)

	require.NotNil(t, result)
	require.Len(t, result.Content, 2)
	assert.False(t, result.IsError)

	// First content should be text message
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "Test message", textContent.Text)

	// Second content should be TextContent (not embedded resource)
	textContent, ok = result.Content[1].(*mcp.TextContent)
	require.True(t, ok, "Expected TextContent but got %T", result.Content[1])
	assert.Equal(t, "Hello, World!", textContent.Text)
	assert.NotNil(t, textContent.Meta)
	assert.Equal(t, "text/plain", textContent.Meta["mimeType"])
	assert.Equal(t, "test://file.txt", textContent.Meta["uri"])
	assert.NotNil(t, textContent.Annotations)
	assert.Contains(t, textContent.Annotations.Audience, mcp.Role("user"))
}

func TestNewToolResultResourceWithFlag_EnabledFlag_BinaryContent(t *testing.T) {
	// When flag is enabled with binary content, should return ImageContent
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	contents := &mcp.ResourceContents{
		URI:      "test://image.png",
		Blob:     binaryData,
		MIMEType: "image/png",
	}

	result := NewToolResultResourceWithFlag("Binary message", contents, true)

	require.NotNil(t, result)
	require.Len(t, result.Content, 2)
	assert.False(t, result.IsError)

	// First content should be text message
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "Binary message", textContent.Text)

	// Second content should be ImageContent (not embedded resource)
	imageContent, ok := result.Content[1].(*mcp.ImageContent)
	require.True(t, ok, "Expected ImageContent but got %T", result.Content[1])

	// Data should be base64 encoded
	expectedBase64 := base64.StdEncoding.EncodeToString(binaryData)
	assert.Equal(t, []byte(expectedBase64), imageContent.Data)
	assert.Equal(t, "image/png", imageContent.MIMEType)
	assert.NotNil(t, imageContent.Meta)
	assert.Equal(t, "test://image.png", imageContent.Meta["uri"])
	assert.NotNil(t, imageContent.Annotations)
	assert.Contains(t, imageContent.Annotations.Audience, mcp.Role("user"))
}

func TestNewToolResultResourceWithFlag_EnabledFlag_EmptyContent(t *testing.T) {
	// When flag is enabled but neither text nor blob exists, should fallback to embedded resource
	contents := &mcp.ResourceContents{
		URI:      "test://empty",
		MIMEType: "application/octet-stream",
	}

	result := NewToolResultResourceWithFlag("Empty message", contents, true)

	require.NotNil(t, result)
	require.Len(t, result.Content, 2)

	// Should fallback to embedded resource
	_, ok := result.Content[1].(*mcp.EmbeddedResource)
	assert.True(t, ok, "Expected fallback to EmbeddedResource for empty content")
}
