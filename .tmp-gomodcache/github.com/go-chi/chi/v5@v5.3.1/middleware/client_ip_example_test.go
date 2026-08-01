package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Example_clientIP shows how to choose a ClientIPFrom* middleware. There is
// no safe default — pick exactly ONE based on your network setup:
//
//   - [middleware.ClientIPFromRemoteAddr] — this server is directly on the
//     public internet, with no reverse proxy in front.
//
//   - [middleware.ClientIPFromHeader] — your reverse proxy sets a dedicated
//     single-IP header (X-Real-IP, CF-Connecting-IP, X-Client-IP) and
//     overwrites any client-supplied value for every request.
//
//   - [middleware.ClientIPFromXFF] — you sit behind one or more reverse
//     proxies whose IP ranges you can enumerate (your VPC CIDR, Cloudflare's
//     published IP list, etc.).
//
//   - [middleware.ClientIPFromXFFTrustedProxies] — you sit behind a known,
//     fixed number of reverse proxies whose IPs are dynamic (autoscaling
//     pools, ephemeral containers).
//
// Read the resulting IP with [middleware.GetClientIP] (string) or
// [middleware.GetClientIPAddr] ([net/netip.Addr]). These middlewares never
// mutate [net/http.Request.RemoteAddr].
//
// The legacy [middleware.RealIP] middleware is deprecated; it is vulnerable
// to IP spoofing (GHSA-3fxj-6jh8-hvhx, GHSA-rjr7-jggh-pgcp, GHSA-9g5q-2w5x-hmxf).
//
// Background:
// https://github.com/go-chi/chi/pull/967
// https://adam-p.ca/blog/2022/03/x-forwarded-for/
func Example_clientIP() {
	r := chi.NewRouter()

	// Pick ONE; the others are shown commented for reference.

	// (1) Behind a reverse proxy that sets a single-IP header for every
	// request and overwrites any client-supplied value (Cloudflare, Nginx
	// with ngx_http_realip_module, Apache with mod_remoteip, ...).
	r.Use(middleware.ClientIPFromHeader("CF-Connecting-IP"))

	// (2) Behind reverse proxies whose IP ranges you can enumerate as CIDRs.
	// The middleware walks X-Forwarded-For right-to-left, skipping trusted
	// entries; fail-closed on garbage.
	//
	// r.Use(middleware.ClientIPFromXFF(
	//     "13.32.0.0/15",   // CloudFront IPv4.
	//     "2600:9000::/28", // CloudFront IPv6.
	// ))

	// (3) Behind a known number of trusted reverse proxies whose IPs are
	// dynamic.
	//
	// r.Use(middleware.ClientIPFromXFFTrustedProxies(2))

	// (4) Directly on the public internet, no reverse proxy in front.
	//
	// r.Use(middleware.ClientIPFromRemoteAddr)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, middleware.GetClientIP(r.Context()))
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("CF-Connecting-IP", "198.51.100.42")
	req.Header.Set("X-Forwarded-For", "1.2.3.4") // attacker-supplied; ignored.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	fmt.Print(w.Body.String())
	// Output: 198.51.100.42
}
