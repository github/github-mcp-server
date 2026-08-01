package jsonutil_test

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shurcooL/graphql"
	"github.com/shurcooL/graphql/internal/jsonutil"
)

func TestUnmarshalGraphQL_benchmark(t *testing.T) {
	/*
		query {
			viewer {
				login
				createdAt
			}
		}
	*/
	type query struct {
		Viewer struct {
			Login     graphql.String
			CreatedAt time.Time
		}
	}
	var got query
	err := jsonutil.UnmarshalGraphQL([]byte(`{
		"viewer": {
			"login": "shurcooL-test",
			"createdAt": "2017-06-29T04:12:01Z"
		}
	}`), &got)
	if err != nil {
		t.Fatal(err)
	}
	var want query
	want.Viewer.Login = "shurcooL-test"
	want.Viewer.CreatedAt = time.Unix(1498709521, 0).UTC()
	if !reflect.DeepEqual(got, want) {
		t.Error("not equal")
	}
}

func BenchmarkUnmarshalGraphQL(b *testing.B) {
	type query struct {
		Viewer struct {
			Login     graphql.String
			CreatedAt time.Time
		}
	}
	for i := 0; i < b.N; i++ {
		now := time.Now().UTC()
		var got query
		err := jsonutil.UnmarshalGraphQL([]byte(`{
			"viewer": {
				"login": "shurcooL-test",
				"createdAt": "`+now.Format(time.RFC3339Nano)+`"
			}
		}`), &got)
		if err != nil {
			b.Fatal(err)
		}
		var want query
		want.Viewer.Login = "shurcooL-test"
		want.Viewer.CreatedAt = now
		if !reflect.DeepEqual(got, want) {
			b.Error("not equal")
		}
	}
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	type query struct {
		Viewer struct {
			Login     graphql.String
			CreatedAt time.Time
		}
	}
	for i := 0; i < b.N; i++ {
		now := time.Now().UTC()
		var got query
		err := json.Unmarshal([]byte(`{
			"viewer": {
				"login": "shurcooL-test",
				"createdAt": "`+now.Format(time.RFC3339Nano)+`"
			}
		}`), &got)
		if err != nil {
			b.Fatal(err)
		}
		var want query
		want.Viewer.Login = "shurcooL-test"
		want.Viewer.CreatedAt = now
		if !reflect.DeepEqual(got, want) {
			b.Error("not equal")
		}
	}
}

func BenchmarkJSONTokenize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		now := time.Now().UTC()
		dec := json.NewDecoder(strings.NewReader(`{
			"viewer": {
				"login": "shurcooL-test",
				"createdAt": "` + now.Format(time.RFC3339Nano) + `"
			}
		}`))
		var tokens int
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			} else if err != nil {
				b.Error(err)
			}
			tokens++
		}
		if tokens != 9 {
			b.Error("not 9 tokens")
		}
	}
}
