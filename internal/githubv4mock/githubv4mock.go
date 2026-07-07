// githubv4mock package provides a mock GraphQL server used for testing queries produced via
// shurcooL/githubv4 or shurcooL/graphql modules.
package githubv4mock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Matcher struct {
	Request   string
	Variables map[string]any

	Response GQLResponse
}

// NewQueryMatcher constructs a new matcher for the provided query and variables.
// If the provided query is a string, it will be used-as-is, otherwise it will be
// converted to a string using the constructQuery function taken from shurcooL/graphql.
func NewQueryMatcher(query any, variables map[string]any, response GQLResponse) Matcher {
	queryString, ok := query.(string)
	if !ok {
		queryString = constructQuery(query, variables)
	}

	return Matcher{
		Request:   queryString,
		Variables: variables,
		Response:  response,
	}
}

// NewMutationMatcher constructs a new matcher for the provided mutation and variables.
// If the provided mutation is a string, it will be used-as-is, otherwise it will be
// converted to a string using the constructMutation function taken from shurcooL/graphql.
//
// The input parameter is a special form of variable, matching the usage in shurcooL/githubv4. It will be added
// to the query as a variable called `input`. Furthermore, it will be converted to a map[string]any
// to be used for later equality comparison, as when the http handler is called, the request body will no longer
// contain the input struct type information.
func NewMutationMatcher(mutation any, input any, variables map[string]any, response GQLResponse) Matcher {
	mutationString, ok := mutation.(string)
	if !ok {
		// Matching shurcooL/githubv4 mutation behaviour found in https://github.com/shurcooL/githubv4/blob/48295856cce734663ddbd790ff54800f784f3193/githubv4.go#L45-L56
		if variables == nil {
			variables = map[string]any{"input": input}
		} else {
			variables["input"] = input
		}

		mutationString = constructMutation(mutation, variables)
		m, _ := githubv4InputStructToMap(input)
		variables["input"] = m
	}

	return Matcher{
		Request:   mutationString,
		Variables: variables,
		Response:  response,
	}
}

type GQLResponse struct {
	Data   map[string]any `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// DataResponse is the happy path response constructor for a mocked GraphQL request.
func DataResponse(data map[string]any) GQLResponse {
	return GQLResponse{
		Data: data,
	}
}

// ErrorResponse is the unhappy path response constructor for a mocked GraphQL request.\
// Note that for the moment it is only possible to return a single error message.
func ErrorResponse(errorMsg string) GQLResponse {
	return GQLResponse{
		Errors: []struct {
			Message string `json:"message"`
		}{
			{
				Message: errorMsg,
			},
		},
	}
}

// githubv4InputStructToMap converts a struct to a map[string]any, it uses JSON marshalling rather than reflection
// to do so, because the json struct tags are used in the real implementation to produce the variable key names,
// and we need to ensure that when variable matching occurs in the http handler, the keys correctly match.
func githubv4InputStructToMap(s any) (map[string]any, error) {
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	err = json.Unmarshal(jsonBytes, &result)
	return result, err
}

// NewMockedHTTPClient creates a new HTTP client that registers a handler for /graphql POST requests.
// For each request, an attempt will be be made to match the request body against the provided matchers.
// If a match is found, the corresponding response will be returned with StatusOK.
//
// Note that query and variable matching can be slightly fickle. The client expects an EXACT match on the query,
// which in most cases will have been constructed from a type with graphql tags. The query construction code in
// shurcooL/githubv4 uses the field types to derive the query string, thus a go string is not the same as a graphql.ID,
// even though `type ID string`. It is therefore expected that matching variables have the right type for example:
//
//	githubv4mock.NewQueryMatcher(
//	    struct {
//	        Repository struct {
//	            PullRequest struct {
//	                 ID githubv4.ID
//	            } `graphql:"pullRequest(number: $prNum)"`
//	        } `graphql:"repository(owner: $owner, name: $repo)"`
//	    }{},
//	    map[string]any{
//	        "owner": githubv4.String("owner"),
//	        "repo":  githubv4.String("repo"),
//	        "prNum": githubv4.Int(42),
//	    },
//	    githubv4mock.DataResponse(
//	        map[string]any{
//	            "repository": map[string]any{
//	                "pullRequest": map[string]any{
//	                     "id": "PR_kwDODKw3uc6WYN1T",
//	                 },
//	            },
//	        },
//	    ),
//	)
//
// To aid in variable equality checks, values are considered equal if they approximate to the same type. This is
// required because when the http handler is called, the request body no longer has the type information. This manifests
// particularly when using the githubv4.Input types which have type deffed fields in their structs. For example:
//
//	type CloseIssueInput struct {
//	  IssueID ID `json:"issueId"`
//	  StateReason *IssueClosedStateReason `json:"stateReason,omitempty"`
//	}
//
// This client does not currently provide a mechanism for out-of-band errors e.g. returning a 500,
// and errors are constrained to GQL errors returned in the response body with a 200 status code.
func NewMockedHTTPClient(ms ...Matcher) *http.Client {
	matchers := make(map[string]Matcher, len(ms))
	for _, m := range ms {
		matchers[m.Request] = m
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		gqlRequest, err := parseBody(r.Body)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		defer func() { _ = r.Body.Close() }()

		matcher, ok := matchers[gqlRequest.Query]
		if !ok {
			http.Error(w, fmt.Sprintf("no matcher found for query %s", gqlRequest.Query), http.StatusNotFound)
			return
		}

		if len(gqlRequest.Variables) > 0 {
			if len(gqlRequest.Variables) != len(matcher.Variables) {
				http.Error(w, "variables do not have the same length", http.StatusBadRequest)
				return
			}

			for k, v := range matcher.Variables {
				if !objectsAreEqualValues(v, gqlRequest.Variables[k]) {
					http.Error(w, "variable does not match", http.StatusBadRequest)
					return
				}
			}
		}

		responseBody, err := json.Marshal(matcher.Response)
		if err != nil {
			http.Error(w, "error marshalling response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	})

	return &http.Client{Transport: &localRoundTripper{
		handler: mux,
	}}
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

func parseBody(r io.Reader) (gqlRequest, error) {
	var req gqlRequest
	err := json.NewDecoder(r).Decode(&req)
	return req, err
}

func Ptr[T any](v T) *T { return &v }
