package headers

const (
	// AuthorizationHeader is a standard HTTP Header.
	AuthorizationHeader = "Authorization"
	// ContentTypeHeader is a standard HTTP Header.
	ContentTypeHeader = "Content-Type"
	// AcceptHeader is a standard HTTP Header.
	AcceptHeader = "Accept"
	// UserAgentHeader is a standard HTTP Header.
	UserAgentHeader = "User-Agent"

	// ContentTypeJSON is the standard MIME type for JSON.
	ContentTypeJSON = "application/json"
	// ContentTypeEventStream is the standard MIME type for Event Streams.
	ContentTypeEventStream = "text/event-stream"

	// ForwardedForHeader is a standard HTTP Header used to forward the originating IP address of a client.
	ForwardedForHeader = "X-Forwarded-For"

	// RealIPHeader is a standard HTTP Header used to indicate the real IP address of the client.
	RealIPHeader = "X-Real-IP"

	// RequestHmacHeader is used to authenticate requests to the Raw API.
	RequestHmacHeader = "Request-Hmac"
)
