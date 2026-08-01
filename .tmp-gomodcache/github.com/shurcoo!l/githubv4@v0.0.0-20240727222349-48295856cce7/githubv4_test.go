package githubv4_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/shurcooL/githubv4"
)

func TestNewClient_nil(t *testing.T) {
	// Shouldn't panic with nil parameter.
	client := githubv4.NewClient(nil)
	_ = client
}

func TestClient_Query(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		if got, want := req.Method, http.MethodPost; got != want {
			t.Errorf("got request method: %v, want: %v", got, want)
		}
		body := mustRead(req.Body)
		if got, want := body, `{"query":"{viewer{login,bio}}"}`+"\n"; got != want {
			t.Errorf("got body: %v, want %v", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(w, `{"data": {"viewer": {"login": "gopher", "bio": "The Go gopher."}}}`)
	})
	client := githubv4.NewClient(&http.Client{Transport: localRoundTripper{handler: mux}})

	type query struct {
		Viewer struct {
			Login     githubv4.String
			Biography githubv4.String `graphql:"bio"` // GraphQL alias.
		}
	}

	var q query
	err := client.Query(context.Background(), &q, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := q

	var want query
	want.Viewer.Login = "gopher"
	want.Viewer.Biography = "The Go gopher."
	if !reflect.DeepEqual(got, want) {
		t.Errorf("client.Query got: %v, want: %v", got, want)
	}
}

func TestClient_Query_errorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(w, `{
			"data": null,
			"errors": [
				{
					"message": "Field 'bad' doesn't exist on type 'Query'",
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
	client := githubv4.NewClient(&http.Client{Transport: localRoundTripper{handler: mux}})

	var q struct {
		Bad githubv4.String
	}
	err := client.Query(context.Background(), &q, nil)
	if err == nil {
		t.Fatal("got error: nil, want: non-nil")
	}
	if got, want := err.Error(), "Field 'bad' doesn't exist on type 'Query'"; got != want {
		t.Errorf("got error: %v, want: %v", got, want)
	}
}

func TestClient_Query_errorStatusCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "404 Not Found", http.StatusNotFound)
	})
	client := githubv4.NewClient(&http.Client{Transport: localRoundTripper{handler: mux}})

	var q struct {
		Viewer struct {
			Login githubv4.String
		}
	}
	err := client.Query(context.Background(), &q, nil)
	if err == nil {
		t.Fatal("got error: nil, want: non-nil")
	}
	if got, want := err.Error(), `non-200 OK status code: 404 Not Found body: "404 Not Found\n"`; got != want {
		t.Errorf("got error: %v, want: %v", got, want)
	}
}

func TestClient_Query_union(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		if got, want := req.Method, http.MethodPost; got != want {
			t.Errorf("got request method: %v, want: %v", got, want)
		}
		body := mustRead(req.Body)
		if got, want := body, `{"query":"query($issueNumber:Int!$repositoryName:String!$repositoryOwner:String!){repository(owner: $repositoryOwner, name: $repositoryName){issue(number: $issueNumber){timeline(first: 10){nodes{__typename,...on ClosedEvent{actor{login},createdAt},...on ReopenedEvent{actor{login},createdAt},...on RenamedTitleEvent{actor{login},createdAt,currentTitle,previousTitle}}}}}}","variables":{"issueNumber":1,"repositoryName":"go","repositoryOwner":"golang"}}`+"\n"; got != want {
			t.Errorf("got body: %v, want %v", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(w, `{"data": {
			"repository": {
				"issue": {
					"timeline": {
						"nodes": [
							{
								"__typename": "RenamedTitleEvent",
								"createdAt": "2017-06-29T04:12:01Z",
								"actor": {
									"login": "gopher"
								},
								"currentTitle": "new",
								"previousTitle": "old"
							}
						]
					}
				}
			}
		}}`)
	})
	client := githubv4.NewClient(&http.Client{Transport: localRoundTripper{handler: mux}})

	type event struct { // Common fields for all events.
		Actor     struct{ Login githubv4.String }
		CreatedAt githubv4.DateTime
	}
	type issueTimelineItem struct {
		Typename    string `graphql:"__typename"`
		ClosedEvent struct {
			event
		} `graphql:"...on ClosedEvent"`
		ReopenedEvent struct {
			event
		} `graphql:"...on ReopenedEvent"`
		RenamedTitleEvent struct {
			event
			CurrentTitle  string
			PreviousTitle string
		} `graphql:"...on RenamedTitleEvent"`
	}
	type query struct {
		Repository struct {
			Issue struct {
				Timeline struct {
					Nodes []issueTimelineItem
				} `graphql:"timeline(first: 10)"`
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $repositoryOwner, name: $repositoryName)"`
	}

	var q query
	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String("golang"),
		"repositoryName":  githubv4.String("go"),
		"issueNumber":     githubv4.Int(1),
	}
	err := client.Query(context.Background(), &q, variables)
	if err != nil {
		t.Fatal(err)
	}
	got := q

	var want query
	want.Repository.Issue.Timeline.Nodes = make([]issueTimelineItem, 1)
	want.Repository.Issue.Timeline.Nodes[0].Typename = "RenamedTitleEvent"
	want.Repository.Issue.Timeline.Nodes[0].RenamedTitleEvent.Actor.Login = "gopher"
	want.Repository.Issue.Timeline.Nodes[0].RenamedTitleEvent.CreatedAt.Time = time.Unix(1498709521, 0).UTC()
	want.Repository.Issue.Timeline.Nodes[0].RenamedTitleEvent.CurrentTitle = "new"
	want.Repository.Issue.Timeline.Nodes[0].RenamedTitleEvent.PreviousTitle = "old"
	want.Repository.Issue.Timeline.Nodes[0].ClosedEvent.event = want.Repository.Issue.Timeline.Nodes[0].RenamedTitleEvent.event
	want.Repository.Issue.Timeline.Nodes[0].ReopenedEvent.event = want.Repository.Issue.Timeline.Nodes[0].RenamedTitleEvent.event
	if !reflect.DeepEqual(got, want) {
		t.Errorf("client.Query:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestClient_Mutate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, req *http.Request) {
		if got, want := req.Method, http.MethodPost; got != want {
			t.Errorf("got request method: %v, want: %v", got, want)
		}
		body := mustRead(req.Body)
		if got, want := body, `{"query":"mutation($input:AddReactionInput!){addReaction(input:$input){reaction{content},subject{id,reactionGroups{users{totalCount}}}}}","variables":{"input":{"subjectId":"MDU6SXNzdWUyMTc5NTQ0OTc=","content":"HOORAY"}}}`+"\n"; got != want {
			t.Errorf("got body: %v, want %v", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(w, `{"data": {
			"addReaction": {
				"reaction": {
					"content": "HOORAY"
				},
				"subject": {
					"id": "MDU6SXNzdWUyMTc5NTQ0OTc=",
					"reactionGroups": [
						{
							"users": {"totalCount": 3}
						}
					]
				}
			}
		}}`)
	})
	client := githubv4.NewClient(&http.Client{Transport: localRoundTripper{handler: mux}})

	type reactionGroup struct {
		Users struct {
			TotalCount githubv4.Int
		}
	}
	type mutation struct {
		AddReaction struct {
			Reaction struct {
				Content githubv4.ReactionContent
			}
			Subject struct {
				ID             githubv4.ID
				ReactionGroups []reactionGroup
			}
		} `graphql:"addReaction(input:$input)"`
	}

	var m mutation
	input := githubv4.AddReactionInput{
		SubjectID: "MDU6SXNzdWUyMTc5NTQ0OTc=",
		Content:   githubv4.ReactionContentHooray,
	}
	err := client.Mutate(context.Background(), &m, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := m

	var want mutation
	want.AddReaction.Reaction.Content = githubv4.ReactionContentHooray
	want.AddReaction.Subject.ID = "MDU6SXNzdWUyMTc5NTQ0OTc="
	var rg reactionGroup
	rg.Users.TotalCount = 3
	want.AddReaction.Subject.ReactionGroups = []reactionGroup{rg}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("client.Query got: %v, want: %v", got, want)
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
