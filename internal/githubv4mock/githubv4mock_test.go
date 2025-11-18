package githubv4mock

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueryMatcher_WithString(t *testing.T) {
	query := "query{viewer{login}}"
	variables := map[string]any{"foo": "bar"}
	response := DataResponse(map[string]any{"viewer": map[string]any{"login": "testuser"}})

	matcher := NewQueryMatcher(query, variables, response)

	assert.Equal(t, query, matcher.Request)
	assert.Equal(t, variables, matcher.Variables)
	assert.Equal(t, response, matcher.Response)
}

func TestNewQueryMatcher_WithStruct(t *testing.T) {
	type Query struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	query := Query{}
	variables := map[string]any{}
	response := DataResponse(map[string]any{"viewer": map[string]any{"login": "testuser"}})

	matcher := NewQueryMatcher(query, variables, response)

	assert.Contains(t, matcher.Request, "viewer")
	assert.Contains(t, matcher.Request, "login")
	assert.Equal(t, response, matcher.Response)
}

func TestNewQueryMatcher_WithVariables(t *testing.T) {
	type Query struct {
		Repository struct {
			Name githubv4.String
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	query := Query{}
	variables := map[string]any{
		"owner": githubv4.String("github"),
		"name":  githubv4.String("github-mcp-server"),
	}
	response := DataResponse(map[string]any{"repository": map[string]any{"name": "github-mcp-server"}})

	matcher := NewQueryMatcher(query, variables, response)

	assert.Contains(t, matcher.Request, "query(")
	assert.Contains(t, matcher.Request, "$owner")
	assert.Contains(t, matcher.Request, "$name")
	assert.Equal(t, variables, matcher.Variables)
}

func TestNewMutationMatcher_WithString(t *testing.T) {
	mutation := "mutation{createIssue(input:{repositoryId:\"test\"})}"
	variables := map[string]any{"foo": "bar"}
	response := DataResponse(map[string]any{"createIssue": map[string]any{"issue": map[string]any{"id": "123"}}})

	matcher := NewMutationMatcher(mutation, nil, variables, response)

	assert.Equal(t, mutation, matcher.Request)
	assert.Equal(t, variables, matcher.Variables)
	assert.Equal(t, response, matcher.Response)
}

func TestNewMutationMatcher_WithStruct(t *testing.T) {
	type Mutation struct {
		CloseIssue struct {
			Issue struct {
				ID githubv4.ID
			}
		} `graphql:"closeIssue(input: $input)"`
	}

	type CloseIssueInput struct {
		IssueID githubv4.ID `json:"issueId"`
	}

	mutation := Mutation{}
	input := CloseIssueInput{IssueID: "ISSUE_123"}
	response := DataResponse(map[string]any{"closeIssue": map[string]any{"issue": map[string]any{"id": "ISSUE_123"}}})

	matcher := NewMutationMatcher(mutation, input, nil, response)

	assert.Contains(t, matcher.Request, "mutation(")
	assert.Contains(t, matcher.Request, "$input")
	assert.Contains(t, matcher.Request, "closeIssue")
	assert.NotNil(t, matcher.Variables)
	assert.Contains(t, matcher.Variables, "input")

	// The input should be converted to a map
	inputMap, ok := matcher.Variables["input"].(map[string]any)
	assert.True(t, ok, "input should be converted to map[string]any")
	assert.Equal(t, "ISSUE_123", inputMap["issueId"])
}

func TestNewMutationMatcher_WithExistingVariables(t *testing.T) {
	type Mutation struct {
		UpdateIssue struct {
			Issue struct {
				ID githubv4.ID
			}
		} `graphql:"updateIssue(input: $input)"`
	}

	type UpdateIssueInput struct {
		IssueID githubv4.ID `json:"issueId"`
		Title   string      `json:"title"`
	}

	mutation := Mutation{}
	input := UpdateIssueInput{IssueID: "ISSUE_456", Title: "Updated Title"}
	existingVars := map[string]any{"otherVar": "value"}
	response := DataResponse(map[string]any{"updateIssue": map[string]any{"issue": map[string]any{"id": "ISSUE_456"}}})

	matcher := NewMutationMatcher(mutation, input, existingVars, response)

	assert.Contains(t, matcher.Variables, "input")
	assert.Contains(t, matcher.Variables, "otherVar")
	assert.Equal(t, "value", matcher.Variables["otherVar"])
}

func TestDataResponse(t *testing.T) {
	data := map[string]any{
		"viewer": map[string]any{
			"login": "testuser",
			"name":  "Test User",
		},
	}

	response := DataResponse(data)

	assert.Equal(t, data, response.Data)
	assert.Nil(t, response.Errors)
}

func TestErrorResponse(t *testing.T) {
	errorMsg := "Something went wrong"

	response := ErrorResponse(errorMsg)

	assert.Nil(t, response.Data)
	require.Len(t, response.Errors, 1)
	assert.Equal(t, errorMsg, response.Errors[0].Message)
}

func TestGithubv4InputStructToMap(t *testing.T) {
	type TestInput struct {
		Field1 string `json:"field1"`
		Field2 int    `json:"field2"`
		Field3 bool   `json:"field3,omitempty"`
	}

	input := TestInput{
		Field1: "value1",
		Field2: 42,
		Field3: true,
	}

	result, err := githubv4InputStructToMap(input)
	require.NoError(t, err)

	assert.Equal(t, "value1", result["field1"])
	assert.Equal(t, float64(42), result["field2"]) // JSON numbers are float64
	assert.Equal(t, true, result["field3"])
}

func TestGithubv4InputStructToMap_WithOmittedFields(t *testing.T) {
	type TestInput struct {
		Required string  `json:"required"`
		Optional *string `json:"optional,omitempty"`
	}

	input := TestInput{
		Required: "value",
		Optional: nil,
	}

	result, err := githubv4InputStructToMap(input)
	require.NoError(t, err)

	assert.Equal(t, "value", result["required"])
	assert.NotContains(t, result, "optional")
}

func TestParseBody(t *testing.T) {
	body := `{"query":"query{viewer{login}}","variables":{"foo":"bar"}}`
	reader := strings.NewReader(body)

	result, err := parseBody(reader)
	require.NoError(t, err)

	assert.Equal(t, "query{viewer{login}}", result.Query)
	assert.Equal(t, "bar", result.Variables["foo"])
}

func TestParseBody_InvalidJSON(t *testing.T) {
	body := `{invalid json}`
	reader := strings.NewReader(body)

	_, err := parseBody(reader)
	assert.Error(t, err)
}

func TestPtr(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		val := "test"
		ptr := Ptr(val)
		require.NotNil(t, ptr)
		assert.Equal(t, val, *ptr)
	})

	t.Run("int", func(t *testing.T) {
		val := 42
		ptr := Ptr(val)
		require.NotNil(t, ptr)
		assert.Equal(t, val, *ptr)
	})

	t.Run("bool", func(t *testing.T) {
		val := true
		ptr := Ptr(val)
		require.NotNil(t, ptr)
		assert.Equal(t, val, *ptr)
	})

	t.Run("float", func(t *testing.T) {
		val := 3.14
		ptr := Ptr(val)
		require.NotNil(t, ptr)
		assert.Equal(t, val, *ptr)
	})
}

func TestNewMockedHTTPClient_SuccessfulQuery(t *testing.T) {
	type Query struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	matcher := NewQueryMatcher(
		Query{},
		nil,
		DataResponse(map[string]any{
			"viewer": map[string]any{
				"login": "testuser",
			},
		}),
	)

	client := NewMockedHTTPClient(matcher)
	require.NotNil(t, client)

	// Create a request
	query := constructQuery(Query{}, nil)
	reqBody, _ := json.Marshal(gqlRequest{Query: query})
	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", bytes.NewReader(reqBody))

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Parse response
	var gqlResp GQLResponse
	err = json.NewDecoder(resp.Body).Decode(&gqlResp)
	require.NoError(t, err)

	viewerLogin := gqlResp.Data["viewer"].(map[string]any)["login"]
	assert.Equal(t, "testuser", viewerLogin)
}

func TestNewMockedHTTPClient_WithVariables(t *testing.T) {
	type Query struct {
		Repository struct {
			Name githubv4.String
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner": githubv4.String("github"),
		"name":  githubv4.String("test-repo"),
	}

	matcher := NewQueryMatcher(
		Query{},
		variables,
		DataResponse(map[string]any{
			"repository": map[string]any{
				"name": "test-repo",
			},
		}),
	)

	client := NewMockedHTTPClient(matcher)

	query := constructQuery(Query{}, variables)
	reqBody, _ := json.Marshal(gqlRequest{Query: query, Variables: variables})
	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", bytes.NewReader(reqBody))

	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewMockedHTTPClient_ErrorResponse(t *testing.T) {
	type Query struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	matcher := NewQueryMatcher(
		Query{},
		nil,
		ErrorResponse("GraphQL error occurred"),
	)

	client := NewMockedHTTPClient(matcher)

	query := constructQuery(Query{}, nil)
	reqBody, _ := json.Marshal(gqlRequest{Query: query})
	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", bytes.NewReader(reqBody))

	resp, err := client.Do(req)
	require.NoError(t, err)

	var gqlResp GQLResponse
	err = json.NewDecoder(resp.Body).Decode(&gqlResp)
	require.NoError(t, err)

	require.Len(t, gqlResp.Errors, 1)
	assert.Equal(t, "GraphQL error occurred", gqlResp.Errors[0].Message)
}

func TestNewMockedHTTPClient_MethodNotAllowed(t *testing.T) {
	client := NewMockedHTTPClient()

	req, _ := http.NewRequest("GET", "http://api.github.com/graphql", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestNewMockedHTTPClient_InvalidRequestBody(t *testing.T) {
	client := NewMockedHTTPClient()

	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", strings.NewReader("{invalid json}"))
	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestNewMockedHTTPClient_NoMatcherFound(t *testing.T) {
	type Query struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	matcher := NewQueryMatcher(
		Query{},
		nil,
		DataResponse(map[string]any{"viewer": map[string]any{"login": "testuser"}}),
	)

	client := NewMockedHTTPClient(matcher)

	// Send a different query
	reqBody, _ := json.Marshal(gqlRequest{Query: "query{different{query}}"})
	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", bytes.NewReader(reqBody))

	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestNewMockedHTTPClient_VariableLengthMismatch(t *testing.T) {
	type Query struct {
		Repository struct {
			Name githubv4.String
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner": githubv4.String("github"),
		"name":  githubv4.String("test-repo"),
	}

	matcher := NewQueryMatcher(Query{}, variables, DataResponse(map[string]any{}))
	client := NewMockedHTTPClient(matcher)

	query := constructQuery(Query{}, variables)
	// Send with different number of variables
	wrongVariables := map[string]any{"owner": githubv4.String("github")}
	reqBody, _ := json.Marshal(gqlRequest{Query: query, Variables: wrongVariables})
	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", bytes.NewReader(reqBody))

	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestNewMockedHTTPClient_VariableValueMismatch(t *testing.T) {
	type Query struct {
		Repository struct {
			Name githubv4.String
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner": githubv4.String("github"),
		"name":  githubv4.String("test-repo"),
	}

	matcher := NewQueryMatcher(Query{}, variables, DataResponse(map[string]any{}))
	client := NewMockedHTTPClient(matcher)

	query := constructQuery(Query{}, variables)
	// Send with different variable values
	wrongVariables := map[string]any{
		"owner": githubv4.String("different-owner"),
		"name":  githubv4.String("test-repo"),
	}
	reqBody, _ := json.Marshal(gqlRequest{Query: query, Variables: wrongVariables})
	req, _ := http.NewRequest("POST", "http://api.github.com/graphql", bytes.NewReader(reqBody))

	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestNewMockedHTTPClient_MultipleMatchers(t *testing.T) {
	type Query1 struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	type Query2 struct {
		Repository struct {
			Name githubv4.String
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	matcher1 := NewQueryMatcher(
		Query1{},
		nil,
		DataResponse(map[string]any{"viewer": map[string]any{"login": "user1"}}),
	)

	variables2 := map[string]any{
		"owner": githubv4.String("github"),
		"name":  githubv4.String("repo"),
	}
	matcher2 := NewQueryMatcher(
		Query2{},
		variables2,
		DataResponse(map[string]any{"repository": map[string]any{"name": "repo"}}),
	)

	client := NewMockedHTTPClient(matcher1, matcher2)

	// Test first matcher
	query1 := constructQuery(Query1{}, nil)
	reqBody1, _ := json.Marshal(gqlRequest{Query: query1})
	req1, _ := http.NewRequest("POST", "http://api.github.com/graphql", bytes.NewReader(reqBody1))

	resp1, err := client.Do(req1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Test second matcher
	query2 := constructQuery(Query2{}, variables2)
	reqBody2, _ := json.Marshal(gqlRequest{Query: query2, Variables: variables2})
	req2, _ := http.NewRequest("POST", "http://api.github.com/graphql", bytes.NewReader(reqBody2))

	resp2, err := client.Do(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestLocalRoundTripper_RoundTrip(t *testing.T) {
	// Create a simple handler
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	roundTripper := localRoundTripper{handler: mux}

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := roundTripper.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "test response", string(body))
}

func TestConstructQuery_NoVariables(t *testing.T) {
	type Query struct {
		Viewer struct {
			Login githubv4.String
			Name  githubv4.String
		}
	}

	query := constructQuery(Query{}, nil)

	assert.Contains(t, query, "{viewer{login,name}}")
	assert.NotContains(t, query, "query(")
}

func TestConstructQuery_WithVariables(t *testing.T) {
	type Query struct {
		Repository struct {
			Name githubv4.String
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner": githubv4.String("github"),
		"name":  githubv4.String("test"),
	}

	query := constructQuery(Query{}, variables)

	assert.Contains(t, query, "query(")
	assert.Contains(t, query, "$owner")
	assert.Contains(t, query, "$name")
	assert.Contains(t, query, "repository(owner: $owner, name: $name)")
}

func TestConstructMutation_NoVariables(t *testing.T) {
	type Mutation struct {
		CloseIssue struct {
			Issue struct {
				ID githubv4.ID
			}
		} `graphql:"closeIssue(input: $input)"`
	}

	mutation := constructMutation(Mutation{}, nil)

	assert.Contains(t, mutation, "mutation")
	assert.Contains(t, mutation, "closeIssue")
}

func TestConstructMutation_WithVariables(t *testing.T) {
	type Mutation struct {
		UpdateIssue struct {
			Issue struct {
				ID githubv4.ID
			}
		} `graphql:"updateIssue(input: $input)"`
	}

	variables := map[string]any{
		"input": map[string]any{"issueId": "ISSUE_123"},
	}

	mutation := constructMutation(Mutation{}, variables)

	assert.Contains(t, mutation, "mutation(")
	assert.Contains(t, mutation, "$input")
}

func TestQueryArguments(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]any
		expected  string
	}{
		{
			name: "single string variable",
			variables: map[string]any{
				"name": githubv4.String("test"),
			},
			expected: "$name:String!",
		},
		{
			name: "single int variable",
			variables: map[string]any{
				"count": githubv4.Int(10),
			},
			expected: "$count:Int!",
		},
		{
			name: "multiple variables sorted",
			variables: map[string]any{
				"name":  githubv4.String("test"),
				"count": githubv4.Int(10),
			},
			// Should be sorted alphabetically
			expected: "$count:Int!$name:String!",
		},
		{
			name: "pointer variable (optional)",
			variables: map[string]any{
				"optional": (*githubv4.String)(nil),
			},
			expected: "$optional:String",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := queryArguments(tt.variables)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteArgumentType(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{
			name:     "required string",
			value:    githubv4.String("test"),
			expected: "String!",
		},
		{
			name:     "required int",
			value:    githubv4.Int(42),
			expected: "Int!",
		},
		{
			name:     "required boolean",
			value:    githubv4.Boolean(true),
			expected: "Boolean!",
		},
		{
			name:     "optional string",
			value:    (*githubv4.String)(nil),
			expected: "String",
		},
		{
			name:     "required ID",
			value:    githubv4.ID("test"),
			expected: "ID!",
		},
		{
			name:     "slice of strings",
			value:    []githubv4.String{},
			expected: "[String!]!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeArgumentType(&buf, reflect.TypeOf(tt.value), true)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestQuery(t *testing.T) {
	tests := []struct {
		name     string
		queryObj any
		expected string
	}{
		{
			name: "simple query",
			queryObj: struct {
				Viewer struct {
					Login githubv4.String
				}
			}{},
			expected: "{viewer{login}}",
		},
		{
			name: "nested query",
			queryObj: struct {
				Repository struct {
					Owner struct {
						Login githubv4.String
					}
					Name githubv4.String
				} `graphql:"repository(owner: $owner, name: $name)"`
			}{},
			expected: "{repository(owner: $owner, name: $name){owner{login},name}}",
		},
		{
			name: "multiple fields",
			queryObj: struct {
				Viewer struct {
					Login githubv4.String
					Name  githubv4.String
					Email githubv4.String
				}
			}{},
			expected: "{viewer{login,name,email}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := query(tt.queryObj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteQuery(t *testing.T) {
	tests := []struct {
		name     string
		queryObj any
		expected string
	}{
		{
			name: "struct with graphql tag",
			queryObj: struct {
				Repository struct {
					Name githubv4.String
				} `graphql:"repository(owner: \"github\")"`
			}{},
			expected: `{repository(owner: "github"){name}}`,
		},
		{
			name: "anonymous embedded field",
			queryObj: struct {
				Inner struct {
					Login githubv4.String
				}
			}{},
			expected: "{inner{login}}",
		},
		{
			name: "pointer field",
			queryObj: struct {
				Viewer *struct {
					Login githubv4.String
				}
			}{},
			expected: "{viewer{login}}",
		},
		{
			name: "slice field",
			queryObj: struct {
				Issues []struct {
					Title githubv4.String
				}
			}{},
			expected: "{issues{title}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeQuery(&buf, reflect.TypeOf(tt.queryObj), false)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}
