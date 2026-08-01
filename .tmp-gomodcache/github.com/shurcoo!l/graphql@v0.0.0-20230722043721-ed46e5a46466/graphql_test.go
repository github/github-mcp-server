package graphql_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shurcooL/graphql"
)

func TestClient_Query_partialDataWithErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(w, `{
			"data": {
				"node1": {
					"id": "MDEyOklzc3VlQ29tbWVudDE2OTQwNzk0Ng=="
				},
				"node2": null
			},
			"errors": [
				{
					"message": "Could not resolve to a node with the global id of 'NotExist'",
					"type": "NOT_FOUND",
					"path": [
						"node2"
					],
					"locations": [
						{
							"line": 10,
							"column": 4
						}
					]
				}
			]
		}`)
	})
	client := graphql.NewClient("/graphql", &http.Client{Transport: localRoundTripper{handler: mux}})

	var q struct {
		Node1 *struct {
			ID graphql.ID
		} `graphql:"node1: node(id: \"MDEyOklzc3VlQ29tbWVudDE2OTQwNzk0Ng==\")"`
		Node2 *struct {
			ID graphql.ID
		} `graphql:"node2: node(id: \"NotExist\")"`
	}
	err := client.Query(context.Background(), &q, nil)
	if err == nil {
		t.Fatal("got error: nil, want: non-nil")
	}
	if got, want := err.Error(), "Could not resolve to a node with the global id of 'NotExist'"; got != want {
		t.Errorf("got error: %v, want: %v", got, want)
	}
	if q.Node1 == nil || q.Node1.ID != "MDEyOklzc3VlQ29tbWVudDE2OTQwNzk0Ng==" {
		t.Errorf("got wrong q.Node1: %v", q.Node1)
	}
	if q.Node2 != nil {
		t.Errorf("got non-nil q.Node2: %v, want: nil", *q.Node2)
	}
}

func TestClient_Query_noDataWithErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(w, `{
			"errors": [
				{
					"message": "Field 'user' is missing required arguments: login",
					"locations": [
						{
							"line": 7,
							"column": 3
						}
					]
				}
			]
		}`)
	})
	client := graphql.NewClient("/graphql", &http.Client{Transport: localRoundTripper{handler: mux}})

	var q struct {
		User struct {
			Name graphql.String
		}
	}
	err := client.Query(context.Background(), &q, nil)
	if err == nil {
		t.Fatal("got error: nil, want: non-nil")
	}
	if got, want := err.Error(), "Field 'user' is missing required arguments: login"; got != want {
		t.Errorf("got error: %v, want: %v", got, want)
	}
	if q.User.Name != "" {
		t.Errorf("got non-empty q.User.Name: %v", q.User.Name)
	}
}

func TestClient_Query_errorStatusCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "important message", http.StatusInternalServerError)
	})
	client := graphql.NewClient("/graphql", &http.Client{Transport: localRoundTripper{handler: mux}})

	var q struct {
		User struct {
			Name graphql.String
		}
	}
	err := client.Query(context.Background(), &q, nil)
	if err == nil {
		t.Fatal("got error: nil, want: non-nil")
	}
	if got, want := err.Error(), `non-200 OK status code: 500 Internal Server Error body: "important message\n"`; got != want {
		t.Errorf("got error: %v, want: %v", got, want)
	}
	if q.User.Name != "" {
		t.Errorf("got non-empty q.User.Name: %v", q.User.Name)
	}
}

// Test that an empty (but non-nil) variables map is
// handled no differently than a nil variables map.
func TestClient_Query_emptyVariables(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		body := mustRead(req.Body)
		if got, want := body, `{"query":"{user{name}}"}`+"\n"; got != want {
			t.Errorf("got body: %v, want %v", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(w, `{"data": {"user": {"name": "Gopher"}}}`)
	})
	client := graphql.NewClient("/graphql", &http.Client{Transport: localRoundTripper{handler: mux}})

	var q struct {
		User struct {
			Name string
		}
	}
	err := client.Query(context.Background(), &q, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := q.User.Name, "Gopher"; got != want {
		t.Errorf("got q.User.Name: %q, want: %q", got, want)
	}
}

// localRoundTripper is an http.RoundTripper that executes HTTP transactions
// by using handler directly, instead of going over an HTTP connection.
type localRoundTripper struct {
	handler http.Handler
}

func (l localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

func mustRead(r io.Reader) string {
	b, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func mustWrite(w io.Writer, s string) {
	_, err := io.WriteString(w, s)
	if err != nil {
		panic(err)
	}
}
