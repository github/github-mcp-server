package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RunWorkflow creates a tool to run an Actions workflow
func RunWorkflow(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("run_workflow",
			mcp.WithDescription(t("TOOL_RUN_WORKFLOW_DESCRIPTION", "Trigger a workflow run")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("workflow_file",
				mcp.Required(),
				mcp.Description("The workflow file name or ID of the workflow entity."),
			),
			mcp.WithString("ref",
				mcp.Required(),
				mcp.Description("Git reference (branch or tag name)"),
			),
			mcp.WithObject("inputs",
				mcp.Description("Input keys and values configured in the workflow file."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			workflowFileName, err := requiredParam[string](request, "workflow_file")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ref, err := requiredParam[string](request, "ref")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get the optional inputs parameter
			var inputs map[string]any
			if inputsObj, exists := request.Params.Arguments["inputs"]; exists && inputsObj != nil {
				inputs, _ = inputsObj.(map[string]any)
			}

			// Convert inputs to the format expected by the GitHub API
			inputsMap := make(map[string]any)
			for k, v := range inputs {
				inputsMap[k] = v
			}

			// Create the event to dispatch
			event := github.CreateWorkflowDispatchEventRequest{
				Ref:    ref,
				Inputs: inputsMap,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, workflowFileName, event)
			if err != nil {
				return nil, fmt.Errorf("failed to trigger workflow: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			result := map[string]any{
				"success": true,
				"message": "Workflow triggered successfully",
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
