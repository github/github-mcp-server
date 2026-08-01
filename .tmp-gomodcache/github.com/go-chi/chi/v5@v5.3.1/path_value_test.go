package chi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPathValue(t *testing.T) {
	testCases := []struct {
		name         string
		pattern      string
		method       string
		requestPath  string
		expectedBody string
		pathKeys     []string
	}{
		{
			name:         "Basic path value",
			pattern:      "/hubs/{hubID}",
			method:       "GET",
			pathKeys:     []string{"hubID"},
			requestPath:  "/hubs/392",
			expectedBody: "392",
		},
		{
			name:         "Two path values",
			pattern:      "/users/{userID}/conversations/{conversationID}",
			method:       "POST",
			pathKeys:     []string{"userID", "conversationID"},
			requestPath:  "/users/Gojo/conversations/2948",
			expectedBody: "Gojo 2948",
		},
		{
			name:         "Wildcard path",
			pattern:      "/users/{userID}/friends/*",
			method:       "POST",
			pathKeys:     []string{"userID", "*"},
			requestPath:  "/users/Gojo/friends/all-of-them/and/more",
			expectedBody: "Gojo all-of-them/and/more",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRouter()

			r.Handle(tc.method+" "+tc.pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pathValues := []string{}
				for _, pathKey := range tc.pathKeys {
					pathValue := r.PathValue(pathKey)
					if pathValue == "" {
						pathValue = "NOT_FOUND:" + pathKey
					}

					pathValues = append(pathValues, pathValue)
				}

				body := strings.Join(pathValues, " ")

				w.Write([]byte(body))
			}))

			ts := httptest.NewServer(r)
			defer ts.Close()

			_, body := testRequest(t, ts, tc.method, tc.requestPath, nil)
			if body != tc.expectedBody {
				t.Fatalf("expecting %q, got %q", tc.expectedBody, body)
			}
		})
	}
}
