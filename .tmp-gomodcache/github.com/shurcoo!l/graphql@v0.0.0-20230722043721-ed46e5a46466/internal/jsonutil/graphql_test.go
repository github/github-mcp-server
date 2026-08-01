package jsonutil_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/shurcooL/graphql"
	"github.com/shurcooL/graphql/internal/jsonutil"
)

func TestUnmarshalGraphQL(t *testing.T) {
	/*
		query {
			me {
				name
				height
			}
		}
	*/
	type query struct {
		Me struct {
			Name   graphql.String
			Height graphql.Float
		}
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"me": {
			"name": "Luke Skywalker",
			"height": 1.72
		}
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	var want query
	want.Me.Name = "Luke Skywalker"
	want.Me.Height = 1.72
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_graphqlTag(t *testing.T) {
	type query struct {
		Foo graphql.String `graphql:"baz"`
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"baz": "bar"
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	want := query{
		Foo: "bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_jsonTag(t *testing.T) {
	type query struct {
		Foo graphql.String `json:"baz"`
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"foo": "bar"
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	want := query{
		Foo: "bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_array(t *testing.T) {
	type query struct {
		Foo []graphql.String
		Bar []graphql.String
		Baz []graphql.String
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"foo": [
			"bar",
			"baz"
		],
		"bar": [],
		"baz": null
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	want := query{
		Foo: []graphql.String{"bar", "baz"},
		Bar: []graphql.String{},
		Baz: []graphql.String(nil),
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

// When unmarshaling into an array, its initial value should be overwritten
// (rather than appended to).
func TestUnmarshalGraphQL_arrayReset(t *testing.T) {
	var got = []string{"initial"}
	err := jsonutil.UnmarshalGraphQL([]byte(`["bar", "baz"]`), &got)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bar", "baz"}
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_objectArray(t *testing.T) {
	type query struct {
		Foo []struct {
			Name graphql.String
		}
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"foo": [
			{"name": "bar"},
			{"name": "baz"}
		]
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	want := query{
		Foo: []struct{ Name graphql.String }{
			{"bar"},
			{"baz"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_pointer(t *testing.T) {
	type query struct {
		Foo *graphql.String
		Bar *graphql.String
	}
	var got query
	got.Bar = new(graphql.String) // Test that got.Bar gets set to nil.
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"foo": "foo",
		"bar": null
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	want := query{
		Foo: graphql.NewString("foo"),
		Bar: nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_objectPointerArray(t *testing.T) {
	type query struct {
		Foo []*struct {
			Name graphql.String
		}
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"foo": [
			{"name": "bar"},
			null,
			{"name": "baz"}
		]
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	want := query{
		Foo: []*struct{ Name graphql.String }{
			{"bar"},
			nil,
			{"baz"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_pointerWithInlineFragment(t *testing.T) {
	type actor struct {
		User struct {
			DatabaseID uint64
		} `graphql:"... on User"`
		Login string
	}
	type query struct {
		Author actor
		Editor *actor
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"author": {
			"databaseId": 1,
			"login": "test1"
		},
		"editor": {
			"databaseId": 2,
			"login": "test2"
		}
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	var want query
	want.Author = actor{
		User:  struct{ DatabaseID uint64 }{1},
		Login: "test1",
	}
	want.Editor = &actor{
		User:  struct{ DatabaseID uint64 }{2},
		Login: "test2",
	}

	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_unexportedField(t *testing.T) {
	type query struct {
		foo graphql.String
	}
	err := jsonutil.UnmarshalGraphQL([]byte(`{"foo": "bar"}`), new(query))
	if err == nil {
		t.Fatal("got error: nil, want: non-nil")
	}
	if got, want := err.Error(), "struct field for \"foo\" doesn't exist in any of 1 places to unmarshal"; got != want {
		t.Errorf("got error: %v, want: %v", got, want)
	}
}

func TestUnmarshalGraphQL_multipleValues(t *testing.T) {
	type query struct {
		Foo graphql.String
	}
	err := jsonutil.UnmarshalGraphQL([]byte(`{"foo": "bar"}{"foo": "baz"}`), new(query))
	if err == nil {
		t.Fatal("got error: nil, want: non-nil")
	}
	if got, want := err.Error(), "invalid token '{' after top-level value"; got != want {
		t.Errorf("got error: %v, want: %v", got, want)
	}
}

func TestUnmarshalGraphQL_directives(t *testing.T) {
	/*
		query {
			me {
				name @include(if: true)
				height @skip(if: false)
			}
		}
	*/
	type query struct {
		Me struct {
			Name   graphql.String `graphql:"name @include(if: true)"`
			Height graphql.Float  `graphql:"height @skip(if: false)"`
		}
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"me": {
			"name": "Luke Skywalker",
			"height": 1.72
		}
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	var want query
	want.Me.Name = "Luke Skywalker"
	want.Me.Height = 1.72
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func TestUnmarshalGraphQL_union(t *testing.T) {
	/*
		{
			__typename
			... on ClosedEvent {
				createdAt
				actor {login}
			}
			... on ReopenedEvent {
				createdAt
				actor {login}
			}
		}
	*/
	type actor struct{ Login graphql.String }
	type closedEvent struct {
		Actor     actor
		CreatedAt time.Time
	}
	type reopenedEvent struct {
		Actor     actor
		CreatedAt time.Time
	}
	type issueTimelineItem struct {
		Typename      string        `graphql:"__typename"`
		ClosedEvent   closedEvent   `graphql:"... on ClosedEvent"`
		ReopenedEvent reopenedEvent `graphql:"... on ReopenedEvent"`
	}
	var got issueTimelineItem
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"__typename": "ClosedEvent",
		"createdAt": "2017-06-29T04:12:01Z",
		"actor": {
			"login": "shurcooL-test"
		}
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	want := issueTimelineItem{
		Typename: "ClosedEvent",
		ClosedEvent: closedEvent{
			Actor: actor{
				Login: "shurcooL-test",
			},
			CreatedAt: time.Unix(1498709521, 0).UTC(),
		},
		ReopenedEvent: reopenedEvent{
			Actor: actor{
				Login: "shurcooL-test",
			},
			CreatedAt: time.Unix(1498709521, 0).UTC(),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

// Issue https://github.com/shurcooL/githubv4/issues/18.
func TestUnmarshalGraphQL_arrayInsideInlineFragment(t *testing.T) {
	/*
		query {
			search(type: ISSUE, first: 1, query: "type:pr repo:owner/name") {
				nodes {
					... on PullRequest {
						commits(last: 1) {
							nodes {
								url
							}
						}
					}
				}
			}
		}
	*/
	type query struct {
		Search struct {
			Nodes []struct {
				PullRequest struct {
					Commits struct {
						Nodes []struct {
							URL string `graphql:"url"`
						}
					} `graphql:"commits(last: 1)"`
				} `graphql:"... on PullRequest"`
			}
		} `graphql:"search(type: ISSUE, first: 1, query: \"type:pr repo:owner/name\")"`
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"search": {
			"nodes": [
				{
					"commits": {
						"nodes": [
							{
								"url": "https://example.org/commit/49e1"
							}
						]
					}
				}
			]
		}
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	var want query
	want.Search.Nodes = make([]struct {
		PullRequest struct {
			Commits struct {
				Nodes []struct {
					URL string `graphql:"url"`
				}
			} `graphql:"commits(last: 1)"`
		} `graphql:"... on PullRequest"`
	}, 1)
	want.Search.Nodes[0].PullRequest.Commits.Nodes = make([]struct {
		URL string `graphql:"url"`
	}, 1)
	want.Search.Nodes[0].PullRequest.Commits.Nodes[0].URL = "https://example.org/commit/49e1"
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}
