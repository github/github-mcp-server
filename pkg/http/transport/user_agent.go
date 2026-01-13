package transport

import "net/http"

type UserAgentTransport struct {
	Transport http.RoundTripper
	Agent     string
}

func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.Agent)
	return t.Transport.RoundTrip(req)
}
