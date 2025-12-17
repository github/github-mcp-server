package inventory

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ResourceHandlerFunc is a function that takes dependencies and returns an MCP resource handler.
// This allows resources to be defined statically while their handlers are generated
// on-demand with the appropriate dependencies.
type ResourceHandlerFunc func(deps any) mcp.ResourceHandler

// ServerResourceTemplate pairs a resource template with its toolset metadata.
type ServerResourceTemplate struct {
	Template mcp.ResourceTemplate
	// HandlerFunc generates the handler when given dependencies.
	// This allows resources to be passed around without handlers being set up,
	// and handlers are only created when needed.
	HandlerFunc ResourceHandlerFunc
	// Toolset identifies which toolset this resource belongs to
	Toolset ToolsetMetadata
	// FeatureFlagEnable specifies a feature flag that must be enabled for this resource
	// to be available. If set and the flag is not enabled, the resource is omitted.
	FeatureFlagEnable string
	// FeatureFlagDisable specifies a feature flag that, when enabled, causes this resource
	// to be omitted. Used to disable resources when a feature flag is on.
	FeatureFlagDisable string
}

// HasHandler returns true if this resource has a handler function.
func (sr *ServerResourceTemplate) HasHandler() bool {
	return sr.HandlerFunc != nil
}

// Handler returns a resource handler by calling HandlerFunc with the given dependencies.
// Panics if HandlerFunc is nil - all resources should have handlers.
func (sr *ServerResourceTemplate) Handler(deps any) mcp.ResourceHandler {
	if sr.HandlerFunc == nil {
		panic("HandlerFunc is nil for resource: " + sr.Template.Name)
	}
	return sr.HandlerFunc(deps)
}

// NewServerResourceTemplate creates a new ServerResourceTemplate with toolset metadata.
func NewServerResourceTemplate(toolset ToolsetMetadata, resourceTemplate mcp.ResourceTemplate, handlerFn ResourceHandlerFunc) ServerResourceTemplate {
	return ServerResourceTemplate{
		Template:    resourceTemplate,
		HandlerFunc: handlerFn,
		Toolset:     toolset,
	}
}
