package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func projectsTypedSession(t *testing.T, deps ToolDependencies, protocol string) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
	t.Helper()
	tr := translations.NullTranslationHelper
	tools := []inventory.ServerTool{ProjectsList(tr), ProjectsGet(tr), ProjectsWrite(tr)}
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "projects", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	inv.RegisterTools(context.Background(), server, deps)
	if protocol == "" || protocol == "unknown" {
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				switch req := req.(type) {
				case *mcp.ListToolsRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				case *mcp.CallToolRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				}
				return next(ctx, method, req)
			}
		})
	}
	version := protocol
	if version == "" || version == "unknown" {
		version = inventory.ProtocolVersionMultiRoundTrip
	}
	session := connectCommentVisibilityClient(t, server, version)
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 3)
	schemas := make(map[string]*jsonschema.Resolved)
	for _, tool := range list.Tools {
		definition := map[string]inventory.ServerTool{
			"projects_list": tools[0], "projects_get": tools[1], "projects_write": tools[2],
		}[tool.Name]
		inventory.AnnotateHeaderParams(&definition.Tool)
		assert.JSONEq(t, mustMarshalJSON(t, definition.Tool.InputSchema), mustMarshalJSON(t, tool.InputSchema))
		if protocol != inventory.ProtocolVersionMultiRoundTrip {
			assert.Nil(t, tool.OutputSchema, tool.Name)
			continue
		}
		require.NotNil(t, tool.OutputSchema, tool.Name)
		assert.JSONEq(t, mustMarshalJSON(t, definition.Tool.OutputSchema), mustMarshalJSON(t, tool.OutputSchema))
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err)
		schemas[tool.Name] = resolved
	}
	return session, schemas
}

type projectWireStep struct {
	method string
	path   string
	query  string
	body   string
	status int
	header http.Header
}

type projectWireCase struct {
	tool   string
	method string
	args   map[string]any
	steps  []projectWireStep
	text   string
}

const (
	projectWireProject     = `{"id":1,"node_id":"P","title":"Roadmap","public":true,"number":2}`
	projectWireProjectText = `{"id":1,"node_id":"P","title":"Roadmap","description":"","public":true,"closed_at":"0001-01-01T00:00:00Z","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","deleted_at":"0001-01-01T00:00:00Z","number":2,"short_description":""}`
	projectWireField       = `{"id":3,"node_id":"F","name":"Status","data_type":"SINGLE_SELECT","options":[{"id":"o","name":{"raw":"Ready","html":"Ready"},"color":"GREEN"}]}`
	projectWireFieldText   = `{"id":3,"node_id":"F","name":"Status","data_type":"SINGLE_SELECT","options":[{"id":"o","color":"GREEN","name":{"html":"Ready","raw":"Ready"}}]}`
	projectWireItem        = `{"id":4,"node_id":"I","content_type":"Issue","fields":[{"id":3,"name":"Custom","data_type":"CUSTOM","value":{"custom":{"a":7},"flag":true}}]}`
	projectWireItemText    = `{"id":4,"node_id":"I","content_type":"Issue","fields":[{"id":3,"name":"Custom","data_type":"CUSTOM","value":{"custom":{"a":7},"flag":true}}]}`
	projectWireView        = `{"id":"V","number":1,"name":"Board","layout":"BOARD_LAYOUT","filter":null,"configuration":{"visibleFields":{"nodes":[]}}}`
	projectWireViewText    = `{"id":"V","number":1,"name":"Board","layout":"board","filter":"","visible_fields":[]}`
	projectWireStatus      = `{"id":"S","body":"Update","status":"ON_TRACK","createdAt":"2026-01-01T00:00:00Z","startDate":null,"targetDate":null,"creator":{"login":"octocat"}}`
	projectWireStatusText  = `{"id":"S","body":"Update","status":"ON_TRACK","created_at":"2026-01-01T00:00:00Z","creator":{"login":"octocat"}}`
	projectWireResolve     = `{"data":{"organization":{"projectV2":{"id":"P"}}}}`
	projectWireParent      = `{"data":{"node":{"id":"V","layout":"BOARD_LAYOUT","project":{"id":"P"}}}}`
)

func projectWireCases() []projectWireCase {
	rest := func(method, path, query, body string) projectWireStep {
		return projectWireStep{method: method, path: path, query: query, body: body}
	}
	gql := func(body string) projectWireStep {
		return rest("POST", "/graphql", "", body)
	}
	return []projectWireCase{
		{tool: "projects_list", method: projectsMethodListProjects, args: map[string]any{"perPage": "2e0", "after": "next", "query": "is:open", "project_number": false},
			steps: []projectWireStep{rest("GET", "/orgs/o/projectsV2", "after=next&per_page=2&q=is%3Aopen", "["+projectWireProject+"]")},
			text:  `{"pageInfo":{"hasNextPage":false,"hasPreviousPage":false},"projects":[` + projectWireProjectText[:len(projectWireProjectText)-1] + `,"owner_type":"org"}]}`},
		{tool: "projects_list", method: projectsMethodListProjectFields, args: map[string]any{"perPage": 0},
			steps: []projectWireStep{rest("GET", "/orgs/o/projectsV2/2/fields", "per_page=50", "["+projectWireField+"]")},
			text:  `{"fields":[` + projectWireFieldText + `],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`},
		{tool: "projects_list", method: projectsMethodListProjectItems, args: map[string]any{"fields": []any{"3", "5"}, "before": "prev"},
			steps: []projectWireStep{rest("GET", "/orgs/o/projectsV2/2/items", "before=prev&fields=3%2C5&per_page=50", "["+projectWireItem+"]")},
			text:  `{"items":[` + projectWireItemText + `],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`},
		{tool: "projects_list", method: projectsMethodListProjectStatusUpdates, steps: []projectWireStep{gql(`{"data":{"organization":{"projectV2":{"public":true,"statusUpdates":{"nodes":[` + projectWireStatus + `],"pageInfo":{"hasNextPage":true,"hasPreviousPage":false,"endCursor":"next","startCursor":""}}}}}}`)},
			text: `{"pageInfo":{"hasNextPage":true,"hasPreviousPage":false,"nextCursor":"next","prevCursor":""},"statusUpdates":[` + projectWireStatusText + `]}`},
		{tool: "projects_list", method: projectsMethodListProjectViews, steps: []projectWireStep{gql(`{"data":{"organization":{"projectV2":{"id":"P","public":true,"views":{"nodes":[` + projectWireView + `],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"endCursor":"","startCursor":""}}}}}}`)},
			text: `{"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"nextCursor":"","prevCursor":""},"views":[` + projectWireViewText + `]}`},
		{tool: "projects_get", method: projectsMethodGetProject, steps: []projectWireStep{rest("GET", "/orgs/o/projectsV2/2", "", projectWireProject)}, text: projectWireProjectText},
		{tool: "projects_get", method: projectsMethodGetProjectField, args: map[string]any{"field_id": "3e0"}, steps: []projectWireStep{rest("GET", "/orgs/o/projectsV2/2/fields/3", "", projectWireField)}, text: projectWireFieldText},
		{tool: "projects_get", method: projectsMethodGetProjectItem, args: map[string]any{"item_id": "4", "fields": []any{"3"}},
			steps: []projectWireStep{rest("GET", "/orgs/o/projectsV2/2/items/4", "fields=3", projectWireItem)}, text: projectWireItemText},
		{tool: "projects_get", method: projectsMethodGetProjectStatusUpdate, args: map[string]any{"status_update_id": "S", "owner": false, "project_number": nil},
			steps: []projectWireStep{gql(`{"data":{"node":` + projectWireStatus[:len(projectWireStatus)-1] + `,"project":{"public":true}}}}`)}, text: projectWireStatusText},
		{tool: "projects_get", method: projectsMethodGetProjectView, args: map[string]any{"view_id": "V"},
			steps: []projectWireStep{gql(`{"data":{"node":` + projectWireView[:len(projectWireView)-1] + `,"project":{"public":true}}}}`)}, text: projectWireViewText},
		{tool: "projects_write", method: projectsMethodAddProjectItem, args: map[string]any{"item_type": "issue", "item_owner": "o", "item_repo": "r", "issue_number": "7"},
			steps: []projectWireStep{gql(`{"data":{"repository":{"issue":{"id":"ISSUE"}}}}`), gql(projectWireResolve), gql(`{"data":{"addProjectV2ItemById":{"item":{"id":"I","fullDatabaseId":"4"}}}}`)},
			text:  `{"full_database_id":"4","id":"I","item_id":4,"message":"Successfully added issue o/r#7 to project o/2"}`},
		{tool: "projects_write", method: projectsMethodDeleteProjectItem, args: map[string]any{"item_id": "4"},
			steps: []projectWireStep{rest("DELETE", "/orgs/o/projectsV2/2/items/4", "", "")}, text: "project item successfully deleted"},
		{tool: "projects_write", method: projectsMethodUpdateProjectItem, args: map[string]any{"item_id": "4", "updated_field": map[string]any{"id": 3, "value": nil}},
			steps: []projectWireStep{rest("PATCH", "/orgs/o/projectsV2/2/items/4", "", projectWireItem)}, text: projectWireItemText},
		{tool: "projects_write", method: projectsMethodUpdateProjectItem, args: map[string]any{"item_id": "4", "updated_field": map[string]any{"name": "Customer", "value": "Acme"}},
			steps: []projectWireStep{
				gql(`{"data":{"organization":{"projectV2":{"fields":{"nodes":[{"id":"F","databaseId":3,"name":"Customer","dataType":"TEXT"}],"pageInfo":{"hasNextPage":false}}}}}}`),
				gql(`{"data":{"organization":{"projectV2":{"fields":{"nodes":[{"__typename":"ProjectV2Field","databaseId":3,"isIssueField":true,"issueField":{"id":"IF"}}],"pageInfo":{"hasNextPage":false}}}}}}`),
				rest("GET", "/orgs/o/projectsV2/2/items/4", "", `{"id":4,"content_type":"Issue","content":{"node_id":"ISSUE"}}`),
				gql(`{"data":{"setIssueFieldValue":{"issue":{"id":"ISSUE","url":"https://github.com/o/r/issues/7"}}}}`),
			}, text: `{"id":"ISSUE","url":"https://github.com/o/r/issues/7"}`},
		{tool: "projects_write", method: projectsMethodUpdateProjectItems, args: map[string]any{"items": []any{map[string]any{"node_id": "I"}, map[string]any{"node_id": "I"}}, "updated_field": map[string]any{"name": "Notes", "value": "hello"}},
			steps: []projectWireStep{
				gql(projectWireResolve),
				gql(`{"data":{"organization":{"projectV2":{"fields":{"nodes":[{"id":"F","databaseId":3,"name":"Notes","dataType":"TEXT"}],"pageInfo":{"hasNextPage":false}}}}}}`),
				gql(`{"data":{"item0":{"projectV2Item":{"id":"I","fullDatabaseId":"4"}}}}`),
			}, text: `{"failed":1,"results":[{"index":0,"status":"succeeded","item":{"node_id":"I","full_database_id":"4"},"ref":{"node_id":"I"}},{"index":1,"status":"failed","error":{"code":"duplicate_target","message":"items[1] targets the same project item as items[0]; each item may only be written once per call"},"ref":{"node_id":"I"}}],"succeeded":1,"total":2,"unknown":0}`},
		{tool: "projects_write", method: projectsMethodCreateProjectStatusUpdate, args: map[string]any{"body": "Update", "status": "ON_TRACK"},
			steps: []projectWireStep{gql(projectWireResolve), gql(`{"data":{"createProjectV2StatusUpdate":{"statusUpdate":` + projectWireStatus + `}}}`)}, text: projectWireStatusText},
		{tool: "projects_write", method: projectsMethodCreateProjectView, args: map[string]any{"name": "Board", "layout": "board", "filter": nil, "visible_fields": []any{}},
			steps: []projectWireStep{gql(projectWireResolve), gql(`{"data":{"createProjectV2View":{"projectV2View":` + projectWireView + `}}}`)}, text: projectWireViewText},
		{tool: "projects_write", method: projectsMethodUpdateProjectView, args: map[string]any{"view_id": "V", "filter": nil, "visible_fields": []any{}},
			steps: []projectWireStep{gql(projectWireResolve), gql(projectWireParent), gql(`{"data":{"updateProjectV2View":{"projectV2View":` + projectWireView + `}}}`)}, text: projectWireViewText},
		{tool: "projects_write", method: projectsMethodDeleteProjectView, args: map[string]any{"view_id": "V"},
			steps: []projectWireStep{gql(projectWireResolve), gql(projectWireParent), gql(`{"data":{"deleteProjectV2View":{"projectV2View":{"id":"V"}}}}`)}, text: `{"deleted_view_id":"V"}`},
		{tool: "projects_write", method: projectsMethodCreateProject, args: map[string]any{"title": "Roadmap", "project_number": false},
			steps: []projectWireStep{gql(`{"data":{"organization":{"id":"O"}}}`), gql(`{"data":{"createProjectV2":{"projectV2":{"id":"P","number":2,"title":"Roadmap","url":"https://github.com/orgs/o/projects/2"}}}}`)},
			text:  `{"id":"P","number":2,"title":"Roadmap","url":"https://github.com/orgs/o/projects/2"}`},
		{tool: "projects_write", method: projectsMethodCreateIterationField, args: map[string]any{"field_name": "Sprint", "start_date": "2026-01-01", "iteration_duration": "7"},
			steps: []projectWireStep{gql(projectWireResolve), gql(`{"data":{"createProjectV2Field":{"projectV2Field":{"id":"F","name":"Sprint"}}}}`), gql(`{"data":{"updateProjectV2Field":{"projectV2Field":{"id":"F","name":"Sprint","configuration":{"iterations":[{"id":"ITER","title":"Week","startDate":"2026-01-01","duration":7}]}}}}}`)},
			text:  `{"configuration":{"iterations":[{"duration":7,"id":"ITER","start_date":"2026-01-01","title":"Week"}]},"id":"F","name":"Sprint"}`},
	}

}

func TestTypedProjectsOutputSchemaContracts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		schema *jsonschema.Schema
		bad    string
	}{
		{"list", projectsListOutputSchema(), `{"method":"list_project_items","items":{"items":[{"id":"bad"}],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}}`},
		{"get", projectsGetOutputSchema(), `{"method":"get_project_item","item":{"id":4,"fields":[{"name":5}]}}`},
		{"write", projectsWriteOutputSchema(), `{"method":"delete_project_view","deleted_view":{"deleted_view_id":123}}`},
		{"view", projectOutputSchema[MinimalProjectView](), `{"id":"V","number":1,"name":"Board","layout":"board","filter":"","visible_fields":null}`},
		{"optional", projectOutputSchema[MinimalProject](), `{"id":null}`},
		{"batch", projectOutputSchema[ProjectBatchOutput](), `{"total":1,"succeeded":1,"failed":0,"unknown":0,"results":[{"index":0,"status":"succeeded","item":{"node_id":false}}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema, err := tc.schema.Resolve(nil)
			require.NoError(t, err)
			var value any
			require.NoError(t, json.Unmarshal([]byte(tc.bad), &value))
			assert.Error(t, schema.Validate(value))
		})
	}
	for _, raw := range []string{`{"id":null,"message":"added"}`, `{"id":"I","message":"added","item_id":0}`, `{"id":"I","message":"added","full_database_id":"not-numeric"}`} {
		out, err := decodeProjectsWriteOutput(projectsMethodAddProjectItem, []byte(raw))
		require.NoError(t, err)
		assert.JSONEq(t, `{"method":"add_project_item","added":`+raw+`}`, mustMarshalJSON(t, out))
	}
	raw := `{"total":2,"succeeded":1,"failed":1,"unknown":0,"results":[{"index":0,"status":"succeeded","item":{"node_id":"I"}},{"index":1,"status":"failed","ref":{"unexpected":[1,true,null]},"error":{"code":"ambiguous","message":"many","candidates":[{"id":"I","name":"A"}]}}]}`
	out, err := decodeProjectsWriteOutput(projectsMethodUpdateProjectItems, []byte(raw))
	require.NoError(t, err)
	assert.JSONEq(t, `{"method":"update_project_items","batch":`+raw+`}`, mustMarshalJSON(t, out))
	schema, err := projectsWriteOutputSchema().Resolve(nil)
	require.NoError(t, err)
	var value any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, out)), &value))
	require.NoError(t, schema.Validate(value))
}

func projectWireDeps(t *testing.T, tc projectWireCase) BaseDeps {
	t.Helper()
	index := 0
	client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Less(t, index, len(tc.steps), "unexpected request %s %s", r.Method, r.URL)
		step := tc.steps[index]
		index++
		assert.Equal(t, step.method, r.Method)
		assert.Equal(t, step.path, r.URL.Path)
		assert.Equal(t, step.query, r.URL.RawQuery)
		for name, values := range step.header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		if step.status != 0 {
			w.WriteHeader(step.status)
		}
		if step.method == "DELETE" {
			if step.status == 0 {
				w.WriteHeader(http.StatusNoContent)
			}
		}
		_, _ = w.Write([]byte(step.body))
	})}}
	t.Cleanup(func() { assert.Equal(t, len(tc.steps), index, tc.method) })
	return BaseDeps{Client: mustNewGHClient(t, client), GQLClient: githubv4.NewEnterpriseClient("https://api.github.com/graphql", client)}
}

func projectWireArgs(tc projectWireCase) map[string]any {
	args := map[string]any{"method": tc.method, "owner": "o", "owner_type": "org", "project_number": "2e0"}
	maps.Copy(args, tc.args)
	return args
}

func TestTypedProjectsWireOutputs(t *testing.T) {
	for _, protocol := range typedGitGistProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			for _, tc := range projectWireCases() {
				t.Run(tc.method, func(t *testing.T) {
					session, schemas := projectsTypedSession(t, projectWireDeps(t, tc), protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: projectWireArgs(tc)})
					require.NoError(t, err)
					require.False(t, result.IsError, mustMarshalJSON(t, result))
					require.Len(t, result.Content, 1)
					if schema := schemas[tc.tool]; schema != nil {
						var output any
						require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
						require.NoError(t, schema.Validate(output))
						assert.JSONEq(t, projectWireStructured(t, tc), mustMarshalJSON(t, output))
						assert.Equal(t, mustMarshalJSON(t, output), getTextResult(t, result).Text)
					} else {
						assert.Equal(t, tc.text, getTextResult(t, result).Text)
						assert.Nil(t, result.StructuredContent)
					}
				})
			}
		})
	}
}

func projectWireStructured(t *testing.T, tc projectWireCase) string {
	t.Helper()
	fields := map[string]string{
		projectsMethodListProjects: "projects", projectsMethodListProjectFields: "fields",
		projectsMethodListProjectItems: "items", projectsMethodListProjectStatusUpdates: "status_updates",
		projectsMethodListProjectViews: "views", projectsMethodGetProject: "project",
		projectsMethodGetProjectField: "field", projectsMethodGetProjectItem: "item",
		projectsMethodGetProjectStatusUpdate: "status_update", projectsMethodGetProjectView: "view",
		projectsMethodAddProjectItem: "added", projectsMethodUpdateProjectItem: "item",
		projectsMethodUpdateProjectItems: "batch", projectsMethodDeleteProjectItem: "deleted_item",
		projectsMethodCreateProjectStatusUpdate: "status_update", projectsMethodCreateProjectView: "view",
		projectsMethodUpdateProjectView: "view", projectsMethodDeleteProjectView: "deleted_view",
		projectsMethodCreateProject: "project", projectsMethodCreateIterationField: "iteration_field",
	}
	var payload map[string]any
	if tc.method == projectsMethodDeleteProjectItem {
		payload = map[string]any{"message": tc.text}
	} else {
		require.NoError(t, json.Unmarshal([]byte(tc.text), &payload))
	}
	if url, ok := payload["url"]; ok {
		payload["html_url"] = url
		delete(payload, "url")
		if tc.method == projectsMethodUpdateProjectItem {
			fields[tc.method] = "issue_fields"
		}
	}
	return mustMarshalJSON(t, map[string]any{"method": tc.method, fields[tc.method]: payload})
}

func TestTypedProjectsObjectRoots(t *testing.T) {
	for name, schema := range map[string]*jsonschema.Schema{
		"projects_list":  projectsListOutputSchema(),
		"projects_get":   projectsGetOutputSchema(),
		"projects_write": projectsWriteOutputSchema(),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "object", schema.Type)
			assert.Empty(t, schema.AnyOf)
			assert.Contains(t, schema.Required, "method")
			assert.NotEmpty(t, schema.Properties["method"].Enum)
			resolved, err := schema.Resolve(nil)
			require.NoError(t, err)
			assert.Error(t, resolved.Validate(nil))
			assert.Error(t, resolved.Validate([]any{}))
			assert.Error(t, resolved.Validate(map[string]any{}))
			assert.Error(t, resolved.Validate(map[string]any{"method": "unknown"}))
			require.NoError(t, toolsnaps.Test(name, map[string]inventory.ServerTool{
				"projects_list":  ProjectsList(translations.NullTranslationHelper),
				"projects_get":   ProjectsGet(translations.NullTranslationHelper),
				"projects_write": ProjectsWrite(translations.NullTranslationHelper),
			}[name].Tool))
		})
	}
}

func TestTypedProjectsFieldProjection(t *testing.T) {
	raw := `{"id":3,"node_id":"F","name":"Status","data_type":"SINGLE_SELECT","project_url":"https://api.github.com/orgs/o/projectsV2/2","options":[{"id":"o","name":{"raw":"Ready","html":"Ready"},"color":"GREEN"}],"configuration":{"duration":7,"start_day":1,"iterations":[{"id":"iter","title":{"raw":"Sprint","html":"Sprint"},"duration":7,"start_date":"2026-01-01"}]}}`
	out, err := decodeProjectsGetOutput(projectsMethodGetProjectField, []byte(raw))
	require.NoError(t, err)
	assert.NotContains(t, mustMarshalJSON(t, out), "project_url")
	assert.Equal(t, "Ready", *out.Field.Options[0].Name.Raw)
	assert.Equal(t, "Sprint", *out.Field.Configuration.Iterations[0].Title.Raw)
	schema, err := projectsGetOutputSchema().Resolve(nil)
	require.NoError(t, err)
	var value any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, out)), &value))
	require.NoError(t, schema.Validate(value))
}

func TestTypedProjectsEmptyOutputs(t *testing.T) {
	_, err := decodeProjectsListOutput(projectsMethodListProjectItems, nil)
	require.Error(t, err, "empty legacy JSON must not produce a fabricated success")
	for _, tc := range []struct {
		name string
		text string
	}{
		{"empty", `{"items":[],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`},
		{"null_items", `{"items":null,"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`},
		{"null_payload", `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := projectsTypedHandler[ProjectsListInput](
				func(context.Context, ToolDependencies, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, any, error) {
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: tc.text}}}, nil, nil
				}, decodeProjectsListOutput)
			result, out, err := handler(context.Background(), BaseDeps{}, nil, ProjectsListInput{
				Method: ProjectParameter[string]{Value: projectsMethodListProjectItems},
			})
			require.NoError(t, err)
			assert.Equal(t, tc.text, getTextResult(t, result).Text)
			schema, err := projectsListOutputSchema().Resolve(nil)
			require.NoError(t, err)
			var value map[string]any
			require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, out)), &value))
			require.NoError(t, schema.Validate(value))
			assert.Equal(t, projectsMethodListProjectItems, value["method"])
			if tc.name == "null_payload" {
				assert.NotContains(t, value, "items", "a null legacy payload must not become a fake populated result")
			}
		})
	}
	get, err := decodeProjectsGetOutput(projectsMethodGetProject, []byte("null"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"method":"get_project"}`, mustMarshalJSON(t, get))
	get, err = decodeProjectsGetOutput(projectsMethodGetProjectItem, []byte(`{"id":4}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"method":"get_project_item","item":{"id":4}}`, mustMarshalJSON(t, get))
	getSchema, err := projectsGetOutputSchema().Resolve(nil)
	require.NoError(t, err)
	var getValue any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, get)), &getValue))
	require.NoError(t, getSchema.Validate(getValue), "optional fields array may be absent")
	for _, method := range []string{projectsMethodCreateProject, projectsMethodUpdateProjectItem, projectsMethodAddProjectItem} {
		out, err := decodeProjectsWriteOutput(method, []byte("null"))
		require.NoError(t, err)
		assert.JSONEq(t, `{"method":"`+method+`"}`, mustMarshalJSON(t, out))
	}
}

func TestTypedProjectsRESTListPageShapes(t *testing.T) {
	nextLink := http.Header{"Link": {`<https://api.github.com/orgs/o/projectsV2?after=next>; rel="next"`}}
	cases := []struct {
		name string
		tc   projectWireCase
	}{
		{
			name: "projects_empty_final_page",
			tc: projectWireCase{
				tool: "projects_list", method: projectsMethodListProjects,
				steps: []projectWireStep{{method: "GET", path: "/orgs/o/projectsV2", query: "per_page=2", body: `[]`}},
				text:  `{"pageInfo":{"hasNextPage":false,"hasPreviousPage":false},"projects":[]}`,
			},
		},
		{
			name: "fields_empty_final_page",
			tc: projectWireCase{
				tool: "projects_list", method: projectsMethodListProjectFields,
				steps: []projectWireStep{{method: "GET", path: "/orgs/o/projectsV2/2/fields", query: "per_page=2", body: `[]`}},
				text:  `{"fields":[],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`,
			},
		},
		{
			name: "items_empty_final_page",
			tc: projectWireCase{
				tool: "projects_list", method: projectsMethodListProjectItems,
				steps: []projectWireStep{{method: "GET", path: "/orgs/o/projectsV2/2/items", query: "per_page=2", body: `[]`}},
				text:  `{"items":[],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`,
			},
		},
		{
			name: "status_updates_empty_final_page",
			tc: projectWireCase{
				tool: "projects_list", method: projectsMethodListProjectStatusUpdates,
				steps: []projectWireStep{{method: "POST", path: "/graphql", body: `{"data":{"organization":{"projectV2":{"public":true,"statusUpdates":{"nodes":[],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"endCursor":"","startCursor":""}}}}}}`}},
				text:  `{"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"nextCursor":"","prevCursor":""},"statusUpdates":[]}`,
			},
		},
		{
			name: "views_empty_final_page",
			tc: projectWireCase{
				tool: "projects_list", method: projectsMethodListProjectViews,
				steps: []projectWireStep{{method: "POST", path: "/graphql", body: `{"data":{"organization":{"projectV2":{"id":"P","public":true,"views":{"nodes":[],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"endCursor":"","startCursor":""}}}}}}`}},
				text:  `{"pageInfo":{"hasNextPage":false,"hasPreviousPage":false,"nextCursor":"","prevCursor":""},"views":[]}`,
			},
		},
		{
			name: "projects_empty_next_page",
			tc: projectWireCase{
				tool: "projects_list", method: projectsMethodListProjects,
				steps: []projectWireStep{{method: "GET", path: "/orgs/o/projectsV2", query: "per_page=2", body: `[]`, header: nextLink}},
				text:  `{"pageInfo":{"hasNextPage":true,"hasPreviousPage":false,"nextCursor":"next"},"projects":[]}`,
			},
		},
		{
			name: "fields_empty_next_page",
			tc: projectWireCase{
				tool: "projects_list", method: projectsMethodListProjectFields,
				steps: []projectWireStep{{
					method: "GET", path: "/orgs/o/projectsV2/2/fields", query: "per_page=2", body: `[]`,
					header: http.Header{"Link": []string{`<https://api.github.com/orgs/o/projectsV2/2/fields?per_page=2&after=next>; rel="next"`}},
				}},
				text: `{"fields":[],"pageInfo":{"hasNextPage":true,"hasPreviousPage":false,"nextCursor":"next"}}`,
			},
		},
		{
			name: "items_empty_next_page",
			tc: projectWireCase{
				tool: "projects_list", method: projectsMethodListProjectItems,
				steps: []projectWireStep{{
					method: "GET", path: "/orgs/o/projectsV2/2/items", query: "per_page=2", body: `[]`,
					header: http.Header{"Link": []string{`<https://api.github.com/orgs/o/projectsV2/2/items?per_page=2&after=next>; rel="next"`}},
				}},
				text: `{"items":[],"pageInfo":{"hasNextPage":true,"hasPreviousPage":false,"nextCursor":"next"}}`,
			},
		},
	}

	for _, protocol := range typedGitGistProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					tc.tc.args = map[string]any{"perPage": 2}
					session, schemas := projectsTypedSession(t, projectWireDeps(t, tc.tc), protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
						Name: tc.tc.tool, Arguments: projectWireArgs(tc.tc),
					})
					require.NoError(t, err)
					require.False(t, result.IsError, mustMarshalJSON(t, result))
					if schema := schemas[tc.tc.tool]; schema != nil {
						var output any
						require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
						require.NoError(t, schema.Validate(output))
						assert.JSONEq(t, projectWireStructured(t, tc.tc), mustMarshalJSON(t, output))
						assert.Equal(t, mustMarshalJSON(t, output), getTextResult(t, result).Text)
					} else {
						assert.Equal(t, tc.tc.text, getTextResult(t, result).Text)
						assert.Nil(t, result.StructuredContent)
					}
				})
			}
		})
	}
}

func TestTypedProjectsSequentialPagination(t *testing.T) {
	for _, protocol := range typedGitGistProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			tc := projectWireCase{
				tool: "projects_list", method: projectsMethodListProjectItems,
				args: map[string]any{"perPage": 2},
				steps: []projectWireStep{
					{method: "GET", path: "/orgs/o/projectsV2/2/items", query: "per_page=2", body: `[]`,
						header: http.Header{"Link": {`<https://api.github.com/orgs/o/projectsV2/2/items?after=next&per_page=2>; rel="next"`}}},
					{method: "GET", path: "/orgs/o/projectsV2/2/items", query: "after=next&per_page=2", body: `[]`},
				},
			}
			session, schemas := projectsTypedSession(t, projectWireDeps(t, tc), protocol)
			args := projectWireArgs(tc)
			for page, text := range []string{
				`{"items":[],"pageInfo":{"hasNextPage":true,"hasPreviousPage":false,"nextCursor":"next"}}`,
				`{"items":[],"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`,
			} {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
				require.NoError(t, err)
				require.False(t, result.IsError)
				var payload struct {
					PageInfo pageInfo `json:"pageInfo"`
				}
				if schema := schemas[tc.tool]; schema != nil {
					var output map[string]any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
					require.NoError(t, schema.Validate(output))
					tc.text = text
					assert.JSONEq(t, projectWireStructured(t, tc), mustMarshalJSON(t, output))
					assert.Equal(t, mustMarshalJSON(t, output), getTextResult(t, result).Text)
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, output["items"])), &payload))
				} else {
					assert.Equal(t, text, getTextResult(t, result).Text)
					assert.Nil(t, result.StructuredContent)
					require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &payload))
				}
				assert.Equal(t, page == 0, payload.PageInfo.HasNextPage)
				if page == 0 {
					require.Equal(t, "next", payload.PageInfo.NextCursor)
					args["after"] = payload.PageInfo.NextCursor
				} else {
					assert.Empty(t, payload.PageInfo.NextCursor)
				}
			}
		})
	}
}

func TestTypedProjectsWriteBackendErrors(t *testing.T) {
	cases := []struct {
		tc      projectWireCase
		message string
	}{{
		tc: projectWireCase{
			tool: "projects_write", method: projectsMethodUpdateProjectItem,
			args: map[string]any{"item_id": "4", "updated_field": map[string]any{"id": 3, "value": "Updated"}},
			steps: []projectWireStep{{
				method: "PATCH", path: "/orgs/o/projectsV2/2/items/4",
				body: `{"message":"server error"}`, status: http.StatusInternalServerError,
			}},
		}, message: ProjectUpdateFailedError,
	}, {
		tc: projectWireCase{
			tool: "projects_write", method: projectsMethodDeleteProjectItem,
			args: map[string]any{"item_id": 4},
			steps: []projectWireStep{{method: "DELETE", path: "/orgs/o/projectsV2/2/items/4",
				status: http.StatusInternalServerError, body: `{"message":"server error"}`}},
		}, message: ProjectDeleteFailedError,
	}, {
		tc: projectWireCase{
			tool: "projects_write", method: projectsMethodCreateProjectStatusUpdate,
			args: map[string]any{"body": "Update", "status": "ON_TRACK"},
			steps: []projectWireStep{
				{method: "POST", path: "/graphql", body: projectWireResolve},
				{method: "POST", path: "/graphql", body: `{"errors":[{"message":"mutation denied"}]}`},
			},
		}, message: ProjectStatusUpdateCreateFailedError,
	}}
	for _, protocol := range typedGitGistProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			for _, fixture := range cases {
				t.Run(fixture.tc.method, func(t *testing.T) {
					tc := fixture.tc
					session, _ := projectsTypedSession(t, projectWireDeps(t, tc), protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
						Name: tc.tool, Arguments: projectWireArgs(tc),
					})
					require.NoError(t, err)
					require.True(t, result.IsError)
					assert.Contains(t, getTextResult(t, result).Text, fixture.message)
					assert.Nil(t, result.StructuredContent)
				})
			}
		})
	}
}

func TestTypedProjectsWireIFC(t *testing.T) {
	for _, protocol := range typedGitGistProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			tc := projectWireCases()[5]
			tc.steps[0].body = `{"id":1,"node_id":"P","title":"Roadmap","public":false,"number":2}`
			tc.text = `{"id":1,"node_id":"P","title":"Roadmap","description":"","public":false,"closed_at":"0001-01-01T00:00:00Z","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","deleted_at":"0001-01-01T00:00:00Z","number":2,"short_description":""}`
			deps := projectWireDeps(t, tc)
			deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
			session, schemas := projectsTypedSession(t, deps, protocol)
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: projectWireArgs(tc)})
			require.NoError(t, err)
			require.False(t, result.IsError)
			label := unmarshalIFC(t, result.Meta["ifc"])
			assert.Equal(t, "trusted", label["integrity"])
			assert.Equal(t, "private", label["confidentiality"])
			if schemas[tc.tool] != nil {
				assert.JSONEq(t, projectWireStructured(t, tc), mustMarshalJSON(t, result.StructuredContent))
				assert.Equal(t, mustMarshalJSON(t, result.StructuredContent), getTextResult(t, result).Text)
			} else {
				assert.Equal(t, tc.text, getTextResult(t, result).Text)
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
}

func TestTypedProjectsWireErrors(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		text string
	}{
		{"projects_list", map[string]any{}, "missing required parameter: method"},
		{"projects_list", map[string]any{"method": true}, "parameter method is not of type string"},
		{"projects_list", map[string]any{"method": "unknown", "owner": "o"}, "unknown method: unknown"},
		{"projects_list", map[string]any{"method": "list_projects", "owner": "o", "owner_type": nil}, "parameter owner_type is not of type string, is <nil>"},
		{"projects_get", map[string]any{"method": "get_project", "owner": "o", "owner_type": "org", "project_number": "1.5"}, "parameter project_number is not a valid number: non-integer numeric value: 1.5"},
		{"projects_get", map[string]any{"method": "get_project_view"}, "missing required parameter: view_id"},
		{"projects_get", map[string]any{"method": "get_project_item", "owner": "o", "owner_type": "org", "project_number": 2, "item_id": 4, "fields": []any{true}}, "parameter fields is not of type string, is bool"},
		{"projects_write", map[string]any{"method": "update_project_item", "owner": "o", "owner_type": "org", "project_number": 2, "item_id": 4, "updated_field": false}, "updated_field must be an object"},
		{"projects_write", map[string]any{"method": "create_project", "owner": "o"}, "owner_type is required for create_project"},
		{"projects_write", map[string]any{"method": "create_project_status_update", "owner": "o", "owner_type": "org", "project_number": 2, "status": "BAD"}, `invalid status "BAD": must be one of INACTIVE, ON_TRACK, AT_RISK, OFF_TRACK, COMPLETE`},
		{"projects_write", map[string]any{"method": "update_project_items", "owner": "o", "owner_type": "org", "project_number": 2, "items": []any{}}, "items must contain at least one entry"},
		{"projects_write", map[string]any{"method": "delete_project_item", "owner": "o", "owner_type": "org", "project_number": 2, "ITEM_ID": 4}, "missing required parameter: item_id"},
	}
	for _, protocol := range typedGitGistProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			session, _ := projectsTypedSession(t, projectWireDeps(t, projectWireCase{}), protocol)
			for _, tc := range cases {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
				require.NoError(t, err)
				require.True(t, result.IsError, mustMarshalJSON(t, result))
				require.Len(t, result.Content, 1)
				assert.Equal(t, tc.text, getTextResult(t, result).Text)
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
}

func TestTypedProjectsInputRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input any
		want  string
	}{
		{"list", ProjectsListInput{Method: ProjectParameter[string]{Value: "list_projects"}, Owner: ProjectParameter[string]{Value: "o"}}, `{"method":"list_projects","owner":"o"}`},
		{"get", ProjectsGetInput{Method: ProjectParameter[string]{Value: "get_project_view"}, ViewID: ProjectParameter[string]{Value: "V"}}, `{"method":"get_project_view","view_id":"V"}`},
		{"issue_reference", ProjectsWriteInput{
			Method:    ProjectParameter[string]{Value: "update_project_item"},
			ItemOwner: ProjectParameter[string]{Value: "o"}, ItemRepo: ProjectParameter[string]{Value: "r"},
			IssueNumber: ProjectParameter[int]{Value: 7},
		}, `{"method":"update_project_item","item_owner":"o","item_repo":"r","issue_number":7}`},
		{"view_name_only", ProjectsWriteInput{Method: ProjectParameter[string]{Value: "update_project_view"}, Name: ProjectParameter[string]{Value: "New name"}}, `{"method":"update_project_view","name":"New name"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.JSONEq(t, tc.want, mustMarshalJSON(t, tc.input))
		})
	}
	for _, input := range []any{&ProjectsListInput{}, &ProjectsGetInput{}, &ProjectsWriteInput{}} {
		raw := `{"method":"example","owner":"o","project_number":0}`
		require.NoError(t, json.Unmarshal([]byte(raw), input))
		assert.JSONEq(t, raw, mustMarshalJSON(t, input))
	}
	raw := `{"method":"update_project_view","filter":null,"layout":"","item_id":0}`
	var input ProjectsWriteInput
	require.NoError(t, json.Unmarshal([]byte(raw), &input))
	assert.JSONEq(t, raw, mustMarshalJSON(t, input))
	require.NoError(t, json.Unmarshal([]byte(`{"method":"delete_project_item","ITEM_ID":4,"ItemID":5}`), &input))
	args, err := projectArguments(input)
	require.NoError(t, err)
	assert.NotContains(t, args, "item_id")
	assert.JSONEq(t, `{"method":"delete_project_item"}`, mustMarshalJSON(t, input))
	require.NoError(t, json.Unmarshal([]byte(`{"method":"delete_project_item","item_id":3,"ITEM_ID":4}`), &input))
	assert.JSONEq(t, `{"method":"delete_project_item","item_id":3}`, mustMarshalJSON(t, input))
}

func TestTypedProjectsCaseSensitiveMutationTarget(t *testing.T) {
	deps := projectWireDeps(t, projectWireCase{})
	request := createMCPRequest(map[string]any{
		"method": "delete_project_item", "owner": "o", "owner_type": "org",
		"project_number": 2, "ITEM_ID": 4,
	})
	tool := ProjectsWrite(translations.NullTranslationHelper)
	result, err := tool.Handler(deps)(
		ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Equal(t, "missing required parameter: item_id", getTextResult(t, result).Text)
	assert.Nil(t, result.StructuredContent)
}
