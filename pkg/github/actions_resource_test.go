package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
)

func TestGetJobLogsWithResourceLinks(t *testing.T) {
	// Test that the tool has the new parameter
	tool, _ := GetJobLogs(stubGetClientFn(nil), translations.NullTranslationHelper, 1000)

	// Verify tool has the new parameter
	schema := tool.InputSchema
	assert.Contains(t, schema.Properties, "return_resource_links")

	// Check that the parameter exists (we can't easily check types in this interface)
	resourceLinkParam := schema.Properties["return_resource_links"]
	assert.NotNil(t, resourceLinkParam)
}

func TestJobLogsResourceCreation(t *testing.T) {
	// Test that we can create the resource templates without errors
	jobLogsResource, jobLogsHandler := GetJobLogsResource(stubGetClientFn(nil), translations.NullTranslationHelper)
	workflowRunLogsResource, workflowRunLogsHandler := GetWorkflowRunLogsResource(stubGetClientFn(nil), translations.NullTranslationHelper)

	// Verify resource templates are created
	assert.NotNil(t, jobLogsResource)
	assert.NotNil(t, jobLogsHandler)
	assert.Equal(t, "actions://{owner}/{repo}/jobs/{jobId}/logs", jobLogsResource.URITemplate.Raw())

	assert.NotNil(t, workflowRunLogsResource)
	assert.NotNil(t, workflowRunLogsHandler)
	assert.Equal(t, "actions://{owner}/{repo}/runs/{runId}/logs", workflowRunLogsResource.URITemplate.Raw())
}
