package toolsets

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

type ServerTool struct {
	Tool         mcp.Tool
	RegisterFunc func(s *mcp.Server)
}

func NewServerTool[In, Out any](tool mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) ServerTool {
	return ServerTool{Tool: tool, RegisterFunc: func(s *mcp.Server) {
		mcp.AddTool(s, &tool, handler)
	}}
}

type ServerResourceTemplate struct {
	Template mcp.ResourceTemplate
	Handler  mcp.ResourceHandler
}

func NewServerResourceTemplate(resourceTemplate mcp.ResourceTemplate, handler mcp.ResourceHandler) ServerResourceTemplate {
	return ServerResourceTemplate{
		Template: resourceTemplate,
		Handler:  handler,
	}
}

type ServerPrompt struct {
	Prompt  mcp.Prompt
	Handler mcp.PromptHandler
}

func NewServerPrompt(prompt mcp.Prompt, handler mcp.PromptHandler) ServerPrompt {
	return ServerPrompt{
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
	writeTools  []ServerTool
	readTools   []ServerTool
	// resources are not tools, but the community seems to be moving towards namespaces as a broader concept
	// and in order to have multiple servers running concurrently, we want to avoid overlapping resources too.
	resourceTemplates []ServerResourceTemplate
	// prompts are also not tools but are namespaced similarly
	prompts []ServerPrompt
}

func (t *Toolset) GetActiveTools() []ServerTool {
	if t.Enabled {
		if t.readOnly {
			return t.readTools
		}
		return append(t.readTools, t.writeTools...)
	}
	return nil
}

func (t *Toolset) GetAvailableTools() []ServerTool {
	if t.readOnly {
		return t.readTools
	}
	return append(t.readTools, t.writeTools...)
}

func (t *Toolset) RegisterTools(s *mcp.Server) {
	if !t.Enabled {
		return
	}
	for _, tool := range t.readTools {
		tool.RegisterFunc(s)
	}
	if !t.readOnly {
		for _, tool := range t.writeTools {
			tool.RegisterFunc(s)
		}
	}
}

func (t *Toolset) AddResourceTemplates(templates ...ServerResourceTemplate) *Toolset {
	t.resourceTemplates = append(t.resourceTemplates, templates...)
	return t
}

func (t *Toolset) AddPrompts(prompts ...ServerPrompt) *Toolset {
	t.prompts = append(t.prompts, prompts...)
	return t
}

func (t *Toolset) GetActiveResourceTemplates() []ServerResourceTemplate {
	if !t.Enabled {
		return nil
	}
	return t.resourceTemplates
}

func (t *Toolset) GetAvailableResourceTemplates() []ServerResourceTemplate {
	return t.resourceTemplates
}

func (t *Toolset) RegisterResourcesTemplates(s *mcp.Server) {
	if !t.Enabled {
		return
	}
	for _, resource := range t.resourceTemplates {
		s.AddResourceTemplate(&resource.Template, resource.Handler)
	}
}

func (t *Toolset) RegisterPrompts(s *mcp.Server) {
	if !t.Enabled {
		return
	}
	for _, prompt := range t.prompts {
		s.AddPrompt(&prompt.Prompt, prompt.Handler)
	}
}

func (t *Toolset) SetReadOnly() {
	// Set the toolset to read-only
	t.readOnly = true
}

func (t *Toolset) AddWriteTools(tools ...ServerTool) *Toolset {
	// Silently ignore if the toolset is read-only to avoid any breach of that contract
	for _, tool := range tools {
		if tool.Tool.Annotations.ReadOnlyHint {
			panic(fmt.Sprintf("tool (%s) is incorrectly annotated as read-only", tool.Tool.Name))
		}
	}
	if !t.readOnly {
		t.writeTools = append(t.writeTools, tools...)
	}
	return t
}

func (t *Toolset) AddReadTools(tools ...ServerTool) *Toolset {
	for _, tool := range tools {
		if !tool.Tool.Annotations.ReadOnlyHint {
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

type EnableToolsetsOptions struct {
	ErrorOnUnknown bool
}

func (tg *ToolsetGroup) EnableToolsets(names []string, options *EnableToolsetsOptions) error {
	if options == nil {
		options = &EnableToolsetsOptions{
			ErrorOnUnknown: false,
		}
	}

	// Special case for "all"
	for _, name := range names {
		if name == "all" {
			tg.everythingOn = true
			break
		}
		err := tg.EnableToolset(name)
		if err != nil && options.ErrorOnUnknown {
			return err
		}
	}
	// Do this after to ensure all toolsets are enabled if "all" is present anywhere in list
	if tg.everythingOn {
		for name := range tg.Toolsets {
			err := tg.EnableToolset(name)
			if err != nil && options.ErrorOnUnknown {
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

func (tg *ToolsetGroup) RegisterAll(s *mcp.Server) {
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
