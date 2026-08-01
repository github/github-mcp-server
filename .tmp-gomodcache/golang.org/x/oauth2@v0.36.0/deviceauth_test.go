package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeviceAuthResponseMarshalJson(t *testing.T) {
	tests := []struct {
		name     string
		response DeviceAuthResponse
		want     string
	}{
		{
			name:     "empty",
			response: DeviceAuthResponse{},
			want:     `{"device_code":"","user_code":"","verification_uri":""}`,
		},
		{
			name: "soon",
			response: DeviceAuthResponse{
				Expiry: time.Now().Add(100*time.Second + 999*time.Millisecond),
			},
			want: `{"expires_in":100,"device_code":"","user_code":"","verification_uri":""}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			begin := time.Now()
			gotBytes, err := json.Marshal(tc.response)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(tc.want, "expires_in") && time.Since(begin) > 999*time.Millisecond {
				t.Skip("test ran too slowly to compare `expires_in`")
			}
			got := string(gotBytes)
			if got != tc.want {
				t.Errorf("want=%s, got=%s", tc.want, got)
			}
		})
	}
}

func TestDeviceAuthResponseUnmarshalJson(t *testing.T) {
	tests := []struct {
		name string
		data string
		want DeviceAuthResponse
	}{
		{
			name: "empty",
			data: `{}`,
			want: DeviceAuthResponse{},
		},
		{
			name: "soon",
			data: `{"expires_in":100}`,
			want: DeviceAuthResponse{Expiry: time.Now().UTC().Add(100 * time.Second)},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			begin := time.Now()
			got := DeviceAuthResponse{}
			err := json.Unmarshal([]byte(tc.data), &got)
			if err != nil {
				t.Fatal(err)
			}
			margin := time.Second + time.Since(begin)
			timeDiff := got.Expiry.Sub(tc.want.Expiry)
			if timeDiff < 0 {
				timeDiff *= -1
			}
			if timeDiff > margin {
				t.Errorf("expiry time difference too large, got=%v, want=%v margin=%v", got.Expiry, tc.want.Expiry, margin)
			}
			got.Expiry, tc.want.Expiry = time.Time{}, time.Time{}
			if got != tc.want {
				t.Errorf("want=%#v, got=%#v", tc.want, got)
			}
		})
	}
}

func ExampleConfig_DeviceAuth() {
	var config Config
	ctx := context.Background()
	response, err := config.DeviceAuth(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("please enter code %s at %s\n", response.UserCode, response.VerificationURI)
	token, err := config.DeviceAccessToken(ctx, response)
	if err != nil {
		panic(err)
	}
	fmt.Println(token)
}

func TestDeviceAuthTokenRetrieveError(t *testing.T) {
	runner := func(responseFun func(w http.ResponseWriter)) func(t *testing.T) {
		return func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.String() != "/device" {
					t.Errorf("Unexpected device auth request URL, %v is found.", r.URL)
				}
				responseFun(w)
			}))
			defer ts.Close()
			conf := newConf(ts.URL)
			_, err := conf.DeviceAuth(context.Background())
			if err == nil {
				t.Fatalf("got no error, expected one")
			}
			re, ok := err.(*RetrieveError)
			if !ok {
				t.Fatalf("got %T error, expected *RetrieveError; error was: %v", err, err)
			}
			expected := `oauth2: "invalid_grant" "sometext"`
			if errStr := err.Error(); errStr != expected {
				t.Fatalf("got %#v, expected %#v", errStr, expected)
			}
			expected = "invalid_grant"
			if re.ErrorCode != expected {
				t.Fatalf("got %#v, expected %#v", re.ErrorCode, expected)
			}
			expected = "sometext"
			if re.ErrorDescription != expected {
				t.Fatalf("got %#v, expected %#v", re.ErrorDescription, expected)
			}
		}
	}

	t.Run("UrlEncoding", runner(func(w http.ResponseWriter) {
		w.Header().Set("Content-type", "application/x-www-form-urlencoded")
		// "The authorization server responds with an HTTP 400 (Bad Request)" https://www.rfc-editor.org/rfc/rfc6749#section-5.2
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`error=invalid_grant&error_description=sometext`))
	}))

	t.Run("JSON", runner(func(w http.ResponseWriter) {
		w.Header().Set("Content-type", "application/json")
		// "The authorization server responds with an HTTP 400 (Bad Request)" https://www.rfc-editor.org/rfc/rfc6749#section-5.2
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid_grant", "error_description": "sometext"}`))
	}))
}
