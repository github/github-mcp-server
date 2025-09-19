package toolsets

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolsetDoesNotExistError struct {
	Name string
}

func (e *ToolsetDoesNotExistError) Error() string {
	return fmt.Sprintf("toolset %s does not exist", e.Name)
}

func (e *ToolsetDoesNotExistError) Is(target error) bool {
	if target == nil {
		return false
	}
	if _, ok := target.(*ToolsetDoesNotExistError); ok {
		return true
	}
	return false
}

func NewToolsetDoesNotExistError(name string) *ToolsetDoesNotExistError {
	return &ToolsetDoesNotExistError{Name: name}
}

func NewServerTool(tool mcp.Tool, handler server.ToolHandlerFunc) server.ServerTool {
	return server.ServerTool{Tool: tool, Handler: handler}
}

func NewServerResourceTemplate(resourceTemplate mcp.ResourceTemplate, handler server.ResourceTemplateHandlerFunc) server.ServerResourceTemplate {
	return server.ServerResourceTemplate{
		Template: resourceTemplate,
		Handler:  handler,
	}
}

func NewServerPrompt(prompt mcp.Prompt, handler server.PromptHandlerFunc) server.ServerPrompt {
	return server.ServerPrompt{
		Prompt:  prompt,
		Handler: handler,
	}
}

// Toolset represents a collection of MCP functionality that can be enabled or disabled as a group.
type Toolset struct {
	Name        string
	Description string
	Enabled     bool
	readOnly    bool
	writeTools  []server.ServerTool
	readTools   []server.ServerTool
	// resources are not tools, but the community seems to be moving towards namespaces as a broader concept
	// and in order to have multiple servers running concurrently, we want to avoid overlapping resources too.
	resourceTemplates []server.ServerResourceTemplate
	// prompts are also not tools but are namespaced similarly
	prompts []server.ServerPrompt
}

func (t *Toolset) GetActiveTools() []server.ServerTool {
	if t.Enabled {
		if t.readOnly {
			return t.readTools
		}
		return append(t.readTools, t.writeTools...)
	}
	return nil
}

func (t *Toolset) GetAvailableTools() []server.ServerTool {
	if t.readOnly {
		return t.readTools
	}
	return append(t.readTools, t.writeTools...)
}

func (t *Toolset) RegisterTools(s *server.MCPServer) {
	if !t.Enabled {
		return
	}
	for _, tool := range t.readTools {
		s.AddTool(tool.Tool, tool.Handler)
	}
	if !t.readOnly {
		for _, tool := range t.writeTools {
			s.AddTool(tool.Tool, tool.Handler)
		}
	}
}

func (t *Toolset) AddResourceTemplates(templates ...server.ServerResourceTemplate) *Toolset {
	t.resourceTemplates = append(t.resourceTemplates, templates...)
	return t
}

func (t *Toolset) AddPrompts(prompts ...server.ServerPrompt) *Toolset {
	t.prompts = append(t.prompts, prompts...)
	return t
}

func (t *Toolset) GetActiveResourceTemplates() []server.ServerResourceTemplate {
	if !t.Enabled {
		return nil
	}
	return t.resourceTemplates
}

func (t *Toolset) GetAvailableResourceTemplates() []server.ServerResourceTemplate {
	return t.resourceTemplates
}

func (t *Toolset) RegisterResourcesTemplates(s *server.MCPServer) {
	if !t.Enabled {
		return
	}
	for _, resource := range t.resourceTemplates {
		s.AddResourceTemplate(resource.Template, resource.Handler)
	}
}

func (t *Toolset) RegisterPrompts(s *server.MCPServer) {
	if !t.Enabled {
		return
	}
	for _, prompt := range t.prompts {
		s.AddPrompt(prompt.Prompt, prompt.Handler)
	}
}

func (t *Toolset) SetReadOnly() {
	// Set the toolset to read-only
	t.readOnly = true
}

func (t *Toolset) AddWriteTools(tools ...server.ServerTool) *Toolset {
	// Silently ignore if the toolset is read-only to avoid any breach of that contract
	for _, tool := range tools {
		if *tool.Tool.Annotations.ReadOnlyHint {
			panic(fmt.Sprintf("tool (%s) is incorrectly annotated as read-only", tool.Tool.Name))
		}
	}
	if !t.readOnly {
		t.writeTools = append(t.writeTools, tools...)
	}
	return t
}

func (t *Toolset) AddReadTools(tools ...server.ServerTool) *Toolset {
	for _, tool := range tools {
		if !*tool.Tool.Annotations.ReadOnlyHint {
			panic(fmt.Sprintf("tool (%s) must be annotated as read-only", tool.Tool.Name))
		}
	}
	t.readTools = append(t.readTools, tools...)
	return t
}

type ToolsetGroup struct {
	Toolsets     map[string]*Toolset
	everythingOn bool
	readOnly     bool
}

func NewToolsetGroup(readOnly bool) *ToolsetGroup {
	return &ToolsetGroup{
		Toolsets:     make(map[string]*Toolset),
		everythingOn: false,
		readOnly:     readOnly,
	}
}

func (tg *ToolsetGroup) AddToolset(ts *Toolset) {
	if tg.readOnly {
		ts.SetReadOnly()
	}
	tg.Toolsets[ts.Name] = ts
}

func NewToolset(name string, description string) *Toolset {
	return &Toolset{
		Name:        name,
		Description: description,
		Enabled:     false,
		readOnly:    false,
	}
}

func (tg *ToolsetGroup) IsEnabled(name string) bool {
	// If everythingOn is true, all features are enabled
	if tg.everythingOn {
		return true
	}

	feature, exists := tg.Toolsets[name]
	if !exists {
		return false
	}
	return feature.Enabled
}

func (tg *ToolsetGroup) EnableToolsets(names []string) error {
	// Special case for "all"
	for _, name := range names {
		if name == "all" {
			tg.everythingOn = true
			break
		}
		err := tg.EnableToolset(name)
		if err != nil {
			return err
		}
	}
	// Do this after to ensure all toolsets are enabled if "all" is present anywhere in list
	if tg.everythingOn {
		for name := range tg.Toolsets {
			err := tg.EnableToolset(name)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

func (tg *ToolsetGroup) EnableToolset(name string) error {
	toolset, exists := tg.Toolsets[name]
	if !exists {
		return NewToolsetDoesNotExistError(name)
	}
	toolset.Enabled = true
	tg.Toolsets[name] = toolset
	return nil
}

func (tg *ToolsetGroup) RegisterAll(s *server.MCPServer) {
	for _, toolset := range tg.Toolsets {
		toolset.RegisterTools(s)
		toolset.RegisterResourcesTemplates(s)
		toolset.RegisterPrompts(s)
	}
}

func (tg *ToolsetGroup) GetToolset(name string) (*Toolset, error) {
	toolset, exists := tg.Toolsets[name]
	if !exists {
		return nil, NewToolsetDoesNotExistError(name)
	}
	return toolset, nil
}
