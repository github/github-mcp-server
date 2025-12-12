package toolsets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

func NewServerTool[In any, Out any](tool mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) ServerTool {
	return ServerTool{Tool: tool, RegisterFunc: func(s *mcp.Server) {
		th := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var arguments In
			if err := json.Unmarshal(req.Params.Arguments, &arguments); err != nil {
				return nil, err
			}

			resp, _, err := handler(ctx, req, arguments)

			return resp, err
		}

		s.AddTool(&tool, th)
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
	Toolsets          map[string]*Toolset
	deprecatedAliases map[string]string
	everythingOn      bool
	readOnly          bool
}

func NewToolsetGroup(readOnly bool) *ToolsetGroup {
	return &ToolsetGroup{
		Toolsets:          make(map[string]*Toolset),
		deprecatedAliases: make(map[string]string),
		everythingOn:      false,
		readOnly:          readOnly,
	}
}

func (tg *ToolsetGroup) AddDeprecatedToolAliases(aliases map[string]string) {
	for oldName, newName := range aliases {
		tg.deprecatedAliases[oldName] = newName
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

type ToolDoesNotExistError struct {
	Name string
}

func (e *ToolDoesNotExistError) Error() string {
	return fmt.Sprintf("tool %s does not exist", e.Name)
}

func NewToolDoesNotExistError(name string) *ToolDoesNotExistError {
	return &ToolDoesNotExistError{Name: name}
}

// ResolveToolAliases resolves deprecated tool aliases to their canonical names.
// It logs a warning to stderr for each deprecated alias that is resolved.
// Returns:
//   - resolved: tool names with aliases replaced by canonical names
//   - aliasesUsed: map of oldName → newName for each alias that was resolved
func (tg *ToolsetGroup) ResolveToolAliases(toolNames []string) (resolved []string, aliasesUsed map[string]string) {
	resolved = make([]string, 0, len(toolNames))
	aliasesUsed = make(map[string]string)
	for _, toolName := range toolNames {
		if canonicalName, isAlias := tg.deprecatedAliases[toolName]; isAlias {
			fmt.Fprintf(os.Stderr, "Warning: tool %q is deprecated, use %q instead\n", toolName, canonicalName)
			aliasesUsed[toolName] = canonicalName
			resolved = append(resolved, canonicalName)
		} else {
			resolved = append(resolved, toolName)
		}
	}
	return resolved, aliasesUsed
}

// FindToolByName searches all toolsets (enabled or disabled) for a tool by name.
// Returns the tool, its parent toolset name, and an error if not found.
func (tg *ToolsetGroup) FindToolByName(toolName string) (*ServerTool, string, error) {
	for toolsetName, toolset := range tg.Toolsets {
		// Check read tools
		for _, tool := range toolset.readTools {
			if tool.Tool.Name == toolName {
				return &tool, toolsetName, nil
			}
		}
		// Check write tools
		for _, tool := range toolset.writeTools {
			if tool.Tool.Name == toolName {
				return &tool, toolsetName, nil
			}
		}
	}
	return nil, "", NewToolDoesNotExistError(toolName)
}

// RegisterSpecificTools registers only the specified tools.
// Respects read-only mode (skips write tools if readOnly=true).
// Returns error if any tool is not found.
func (tg *ToolsetGroup) RegisterSpecificTools(s *mcp.Server, toolNames []string, readOnly bool) error {
	var skippedTools []string
	for _, toolName := range toolNames {
		tool, _, err := tg.FindToolByName(toolName)
		if err != nil {
			return fmt.Errorf("tool %s not found: %w", toolName, err)
		}

		if !tool.Tool.Annotations.ReadOnlyHint && readOnly {
			// Skip write tools in read-only mode
			skippedTools = append(skippedTools, toolName)
			continue
		}

		// Register the tool
		tool.RegisterFunc(s)
	}

	// Log skipped write tools if any
	if len(skippedTools) > 0 {
		fmt.Fprintf(os.Stderr, "Write tools skipped due to read-only mode: %s\n", strings.Join(skippedTools, ", "))
	}

	return nil
}
