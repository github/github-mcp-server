//go:build ignore

// graphqldev is a test program currently being used for developing graphql package.
// It performs queries against a local test GraphQL server instance.
//
// It's not meant to be a clean or readable example. But it's functional.
// Better, actual examples will be created in the future.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	graphqlserver "github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/example/starwars"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/shurcooL/graphql"
)

func main() {
	flag.Parse()

	err := run()
	if err != nil {
		log.Println(err)
	}
}

func run() error {
	// Set up a GraphQL server.
	schema, err := graphqlserver.ParseSchema(starwars.Schema, &starwars.Resolver{})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/query", &relay.Handler{Schema: schema})

	client := graphql.NewClient("/query", &http.Client{Transport: localRoundTripper{handler: mux}})

	/*
		query {
			hero {
				id
				name
			}
			character(id: "1003") {
				name
				friends {
					name
					__typename
				}
				appearsIn
			}
		}
	*/
	var q struct {
		Hero struct {
			ID   graphql.ID
			Name graphql.String
		}
		Character struct {
			Name    graphql.String
			Friends []struct {
				Name     graphql.String
				Typename graphql.String `graphql:"__typename"`
			}
			AppearsIn []graphql.String
		} `graphql:"character(id: $characterID)"`
	}
	variables := map[string]any{
		"characterID": graphql.ID("1003"),
	}
	err = client.Query(context.Background(), &q, variables)
	if err != nil {
		return err
	}
	print(q)

	return nil
}

// print pretty prints v to stdout. It panics on any error.
func print(v any) {
	w := json.NewEncoder(os.Stdout)
	w.SetIndent("", "\t")
	err := w.Encode(v)
	if err != nil {
		panic(err)
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
