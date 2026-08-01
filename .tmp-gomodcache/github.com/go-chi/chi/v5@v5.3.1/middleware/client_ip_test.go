package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestClientIPFromHeader(t *testing.T) {
	tt := []struct {
		name string
		in   string
		out  string
	}{
		{"empty", "", ""},
		{"ipv4", "100.100.100.100", "100.100.100.100"},
		{"ipv6_canonical", "2345:425:2ca1::567:5673:23b5", "2345:425:2ca1::567:5673:23b5"},
		{"ipv6_uncompressed_normalized", "2345:0425:2CA1:0000:0000:0567:5673:23B5", "2345:425:2ca1::567:5673:23b5"},

		// netip.ParseAddr rejects non-IPs and embedded ports.
		{"invalid", "invalid", ""},
		{"ipv4_with_port", "100.100.100.100:80", ""},
		{"multiple_ips", "100.100.100.100,200.200.200.200", ""},

		// Per the blog and the three GHSAs: we trust the user's choice of header.
		// If their infra writes a private/loopback IP here, that IS the answer.
		{"loopback_accepted", "127.0.0.1", "127.0.0.1"},
		{"private_v4_accepted", "10.0.1.10", "10.0.1.10"},
		{"private_v6_accepted", "fc00::1", "fc00::1"},
		{"unspecified_accepted", "0.0.0.0", "0.0.0.0"},

		// Normalization: v4-mapped IPv6 folds to plain v4, IPv6 zone is stripped.
		// Same logical client → same canonical string regardless of upstream notation.
		{"v4_mapped_ipv6_folded_to_v4", "::ffff:1.2.3.4", "1.2.3.4"},
		{"ipv6_zone_stripped", "2001:db8::1%eth0", "2001:db8::1"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromHeader("X-Real-IP"), func(r *http.Request) {
				if tc.in != "" {
					r.Header.Set("X-Real-IP", tc.in)
				}
			})
			if got != tc.out {
				t.Errorf("want %q, got %q", tc.out, got)
			}
		})
	}
}

func TestClientIPFromXFF_NoTrustedPrefixes(t *testing.T) {
	tt := []struct {
		name string
		xff  []string
		out  string
	}{
		{"missing", nil, ""},
		{"empty", []string{""}, ""},
		{"single", []string{"100.100.100.100"}, "100.100.100.100"},
		{"comma_space", []string{"1.1.1.1, 2.2.2.2"}, "2.2.2.2"},
		{"comma_no_space", []string{"1.1.1.1,2.2.2.2"}, "2.2.2.2"},
		{"multi_header_merged", []string{"1.1.1.1", "2.2.2.2"}, "2.2.2.2"},
		{"multi_header_with_commas", []string{"5.5.5.5, 6.6.6.6", "7.7.7.7, 4.4.4.4"}, "4.4.4.4"},
		{"ipv6", []string{"2001:db8::1"}, "2001:db8::1"},
		{"mixed_v4_v6_rightmost_wins", []string{"203.0.113.1, 2001:db8::1"}, "2001:db8::1"},

		// Normalization at the entry point: v4-mapped IPv6 folds to v4, zone stripped.
		{"v4_mapped_rightmost_folded", []string{"::ffff:1.2.3.4"}, "1.2.3.4"},
		{"zone_stripped_rightmost", []string{"2001:db8::1%eth0"}, "2001:db8::1"},

		// A header like "oh, hi,,127.0.0.1,,,," can be injected by the client.
		// The walker reaches 127.0.0.1 from the right before encountering the
		// unparseable tokens, so fail-closed never trips here.
		// See https://adam-p.ca/blog/2022/03/x-forwarded-for/ for more details.
		{"weird_with_empties_then_valid_rightmost", []string{"oh, hi,,127.0.0.1,,,,"}, "127.0.0.1"},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromXFF(), func(r *http.Request) {
				for _, v := range tc.xff {
					r.Header.Add("X-Forwarded-For", v)
				}
			})
			if got != tc.out {
				t.Errorf("want %q, got %q", tc.out, got)
			}
		})
	}
}

func TestClientIPFromXFF_TrustedPrefixes(t *testing.T) {
	tt := []struct {
		name     string
		prefixes []string
		xff      []string
		out      string
	}{
		{
			name:     "single_trusted_proxy_skipped",
			prefixes: []string{"203.0.113.0/24"},
			xff:      []string{"100.100.100.100, 203.0.113.50"},
			out:      "100.100.100.100",
		},
		{
			name:     "multiple_trusted_proxies_skipped",
			prefixes: []string{"203.0.113.0/24", "198.51.100.0/24"},
			xff:      []string{"1.1.1.1, 203.0.113.10, 198.51.100.5"},
			out:      "1.1.1.1",
		},
		{
			name:     "all_trusted_returns_empty",
			prefixes: []string{"203.0.113.0/24"},
			xff:      []string{"203.0.113.10, 203.0.113.20"},
			out:      "",
		},
		{
			name:     "ipv6_trusted_range",
			prefixes: []string{"2606:4700::/32"},
			xff:      []string{"2001:db8::1, 2606:4700::1"},
			out:      "2001:db8::1",
		},
		{
			name:     "mixed_v4_v6_trust_list",
			prefixes: []string{"2606:4700::/32", "203.0.113.0/24"},
			xff:      []string{"8.8.8.8, 2606:4700::1, 203.0.113.5"},
			out:      "8.8.8.8",
		},
		// Adam-P's blog and rezmoss's advisory: we must NOT filter private/loopback
		// values when they sit between trusted proxies. K8s nginx-ingress legitimately
		// produces this shape.
		{
			name:     "private_between_trusted_is_client",
			prefixes: []string{"10.244.0.0/24"},
			xff:      []string{"10.244.1.50, 10.244.0.10"},
			out:      "10.244.1.50",
		},
		// /24 boundary tests (Saku0512's PR #1087).
		{
			name:     "boundary_first_addr_in_prefix",
			prefixes: []string{"203.0.113.0/24"},
			xff:      []string{"100.100.100.100, 203.0.113.0"},
			out:      "100.100.100.100",
		},
		{
			name:     "boundary_last_addr_in_prefix",
			prefixes: []string{"203.0.113.0/24"},
			xff:      []string{"100.100.100.100, 203.0.113.255"},
			out:      "100.100.100.100",
		},
		{
			name:     "ip_just_outside_prefix_is_client",
			prefixes: []string{"203.0.113.0/24"},
			xff:      []string{"203.0.114.1, 203.0.113.1"},
			out:      "203.0.114.1",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromXFF(tc.prefixes...), func(r *http.Request) {
				for _, v := range tc.xff {
					r.Header.Add("X-Forwarded-For", v)
				}
			})
			if got != tc.out {
				t.Errorf("want %q, got %q", tc.out, got)
			}
		})
	}
}

func TestClientIPFromXFF_PanicsOnBadPrefix(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on invalid CIDR")
		}
	}()
	ClientIPFromXFF("not-a-cidr")
}

func TestClientIPFromXFFTrustedProxies(t *testing.T) {
	tt := []struct {
		name string
		n    int
		xff  []string
		out  string
	}{
		// N=1 ⇒ rightmost (equivalent to ClientIPFromXFF() with no prefixes).
		{"n1_rightmost", 1, []string{"1.1.1.1, 2.2.2.2"}, "2.2.2.2"},
		{"n2_second_from_right", 2, []string{"1.1.1.1, 2.2.2.2, 3.3.3.3"}, "2.2.2.2"},
		{"n3_third_from_right", 3, []string{"1.1.1.1, 2.2.2.2, 3.3.3.3, 4.4.4.4"}, "2.2.2.2"},
		{"n2_exactly_matches_len", 2, []string{"1.1.1.1, 2.2.2.2"}, "1.1.1.1"},

		// XFF shorter than N: no IP set. This is intentionally fail-closed so a
		// proxy-count mismatch doesn't silently fall through to attacker-controlled
		// values.
		{"shorter_than_n", 3, []string{"1.1.1.1, 2.2.2.2"}, ""},
		{"missing_header", 1, nil, ""},

		// Spoofing: prepended attacker values are to the LEFT of the chosen
		// position, so they're ignored.
		{"spoof_prepend_ignored", 2, []string{"6.6.6.6, 1.1.1.1, 2.2.2.2, 3.3.3.3"}, "2.2.2.2"},

		// Bad parse at the target position: no IP set (don't accept garbage as client IP).
		{"bad_parse_at_position", 2, []string{"garbage, 2.2.2.2"}, ""},

		// Merging across multiple header instances.
		{"multi_header", 2, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, "2.2.2.2"},

		// Empty entries dropped by the walker must not shift the chosen
		// position. The position is computed from the right, where trusted
		// proxies append, so an attacker prepending commas can't slide their
		// chosen value into the trusted position.
		{"empties_dont_shift_position", 2, []string{",,, 3.3.3.3, 1.1.1.1, 2.2.2.2"}, "1.1.1.1"},

		// Normalization at the chosen position: v4-mapped IPv6 folds to v4, zone stripped.
		{"v4_mapped_at_position_folded", 1, []string{"::ffff:1.2.3.4"}, "1.2.3.4"},
		{"zone_stripped_at_position", 1, []string{"2001:db8::1%eth0"}, "2001:db8::1"},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromXFFTrustedProxies(tc.n), func(r *http.Request) {
				for _, v := range tc.xff {
					r.Header.Add("X-Forwarded-For", v)
				}
			})
			if got != tc.out {
				t.Errorf("want %q, got %q", tc.out, got)
			}
		})
	}
}

func TestClientIPFromXFFTrustedProxies_PanicsOnZero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on numTrustedProxies < 1")
		}
	}()
	ClientIPFromXFFTrustedProxies(0)
}

func TestClientIPFromRemoteAddr(t *testing.T) {
	tt := []struct {
		name       string
		remoteAddr string
		out        string
	}{
		{"ipv4_with_port", "192.0.2.1:1234", "192.0.2.1"},
		{"ipv6_with_port", "[2001:db8::1]:1234", "2001:db8::1"},
		{"bare_ipv4", "192.0.2.1", "192.0.2.1"},
		{"empty", "", ""},
		{"garbage", "not-an-ip", ""},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromRemoteAddr, func(r *http.Request) {
				r.RemoteAddr = tc.remoteAddr
			})
			if got != tc.out {
				t.Errorf("want %q, got %q", tc.out, got)
			}
		})
	}
}

// TestClientIPLastWriteWins locks down the implementation property that each
// ClientIPFrom* middleware unconditionally overwrites the context value when
// it finds a valid IP, while leaving any previously stored value alone when
// it doesn't. Application code should pick exactly one ClientIPFrom*
// middleware; this test only guards against an accidental return to
// first-write-wins semantics.
func TestClientIPLastWriteWins(t *testing.T) {
	t.Run("later_overrides_earlier", func(t *testing.T) {
		got := runChain(t,
			[]func(http.Handler) http.Handler{
				ClientIPFromXFF(),
				ClientIPFromHeader("CF-Connecting-IP"),
			},
			func(r *http.Request) {
				r.Header.Set("CF-Connecting-IP", "1.1.1.1")
				r.Header.Set("X-Forwarded-For", "2.2.2.2")
			})
		if got != "1.1.1.1" {
			t.Errorf("want 1.1.1.1, got %q", got)
		}
	})

	t.Run("earlier_persists_when_later_finds_nothing", func(t *testing.T) {
		got := runChain(t,
			[]func(http.Handler) http.Handler{
				ClientIPFromXFF(),
				ClientIPFromHeader("CF-Connecting-IP"), // missing → no-op.
			},
			func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "8.8.8.8")
			})
		if got != "8.8.8.8" {
			t.Errorf("want 8.8.8.8, got %q", got)
		}
	})

	t.Run("earlier_persists_when_later_xff_all_trusted", func(t *testing.T) {
		got := runChain(t,
			[]func(http.Handler) http.Handler{
				ClientIPFromRemoteAddr,
				ClientIPFromXFF("10.0.0.0/8"), // all trusted → no-op.
			},
			func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
				r.RemoteAddr = "192.0.2.1:1234"
			})
		if got != "192.0.2.1" {
			t.Errorf("want 192.0.2.1, got %q", got)
		}
	})
}

func TestGetClientIPAddr_Unset(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if got := GetClientIP(req.Context()); got != "" {
		t.Errorf("GetClientIP on empty ctx: want %q, got %q", "", got)
	}
	if got := GetClientIPAddr(req.Context()); got.IsValid() {
		t.Errorf("GetClientIPAddr on empty ctx: want zero, got %v", got)
	}
}

func TestGetClientIPAddr_RoundTrip(t *testing.T) {
	var gotStr string
	var gotAddr netip.Addr
	r := chi.NewRouter()
	r.Use(ClientIPFromHeader("X-Real-IP"))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		gotStr = GetClientIP(r.Context())
		gotAddr = GetClientIPAddr(r.Context())
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "203.0.113.42")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if gotStr != "203.0.113.42" {
		t.Errorf("GetClientIP: want 203.0.113.42, got %q", gotStr)
	}
	if !gotAddr.IsValid() || gotAddr.String() != "203.0.113.42" {
		t.Errorf("GetClientIPAddr: want 203.0.113.42, got %v", gotAddr)
	}
	if gotAddr.Is6() {
		t.Errorf("GetClientIPAddr: want IPv4, got IPv6")
	}
}

// GHSA-3fxj-6jh8-hvhx (Saku0512): an admin endpoint gates on the client IP,
// and an attacker prepends a spoofed IP to X-Forwarded-For. middleware.RealIP
// would set r.RemoteAddr from the leftmost XFF value, bypassing the gate.
// The new ClientIPFrom* middlewares defeat this attack in both deployment
// shapes the application might choose.
func TestAdvisory_GHSA_3fxj_6jh8_hvhx(t *testing.T) {
	// (a) Server directly on the internet: use ClientIPFromRemoteAddr.
	// XFF is never consulted; the spoofed admin IP is ignored.
	t.Run("direct_internet_uses_remoteaddr", func(t *testing.T) {
		var clientIP string
		r := chi.NewRouter()
		r.Use(ClientIPFromRemoteAddr)
		r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
			clientIP = GetClientIP(r.Context())
		})

		req := httptest.NewRequest("GET", "/admin", nil)
		req.RemoteAddr = "99.99.99.99:1234"
		req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8") // attacker's spoof.
		r.ServeHTTP(httptest.NewRecorder(), req)

		if clientIP == "1.2.3.4" {
			t.Fatal("VULNERABLE: returned attacker-supplied admin IP")
		}
		if clientIP != "99.99.99.99" {
			t.Errorf("want 99.99.99.99 (real TCP peer), got %q", clientIP)
		}
	})

	// (b) Server behind a trusted proxy: use ClientIPFromXFF with the proxy's
	// CIDR. In a real deployment the trusted proxy APPENDS the attacker's
	// real IP, so by the time the request reaches us the rightmost-untrusted
	// entry is the genuine attacker IP, never the prepended spoof.
	t.Run("behind_proxy_uses_xff", func(t *testing.T) {
		got := run(t, ClientIPFromXFF("10.0.0.0/8"), func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", "1.2.3.4, 99.99.99.99")
		})
		if got == "1.2.3.4" {
			t.Fatal("VULNERABLE: returned attacker-supplied admin IP")
		}
		if got != "99.99.99.99" {
			t.Errorf("want 99.99.99.99 (rightmost-untrusted), got %q", got)
		}
	})
}

// GHSA-9g5q-2w5x-hmxf (convto): RealIP set r.RemoteAddr from the leftmost XFF
// IP. With ClientIPFromXFF the rightmost-untrusted IP is selected, and we
// never mutate r.RemoteAddr.
func TestAdvisory_GHSA_9g5q_2w5x_hmxf(t *testing.T) {
	originalRemoteAddr := "203.0.113.99:1234"

	var capturedClientIP, capturedRemoteAddr string
	r := chi.NewRouter()
	r.Use(ClientIPFromXFF())
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		capturedClientIP = GetClientIP(r.Context())
		capturedRemoteAddr = r.RemoteAddr
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "192.0.2.2, 192.0.2.1") // convto's PoC values.
	req.RemoteAddr = originalRemoteAddr
	r.ServeHTTP(httptest.NewRecorder(), req)

	// The rightmost value is the one added by our (single) trusted hop.
	if capturedClientIP != "192.0.2.1" {
		t.Errorf("client IP: want 192.0.2.1 (rightmost), got %q", capturedClientIP)
	}
	if capturedClientIP == "192.0.2.2" {
		t.Errorf("VULNERABLE: returned attacker-supplied leftmost IP")
	}
	// Critical: r.RemoteAddr is never mutated, unlike middleware.RealIP.
	if capturedRemoteAddr != originalRemoteAddr {
		t.Errorf("r.RemoteAddr was mutated: want %q, got %q", originalRemoteAddr, capturedRemoteAddr)
	}
}

// GHSA-rjr7-jggh-pgcp (rezmoss): RealIP trusted X-Real-IP / True-Client-IP /
// X-Forwarded-For unconditionally and used the leftmost XFF value, allowing
// "X-Forwarded-For: 127.0.0.1" to impersonate loopback. The new middleware
// (a) requires the user to opt into a specific source and (b) uses
// rightmost-untrusted for XFF.
func TestAdvisory_GHSA_rjr7_jggh_pgcp(t *testing.T) {
	// Attacker sends "X-Forwarded-For: 127.0.0.1" to spoof loopback. In a real
	// deployment the trusted proxy appends the attacker's actual IP, so the
	// rightmost-untrusted entry is never the loopback spoof.
	t.Run("xff_loopback_spoof_rejected", func(t *testing.T) {
		got := run(t, ClientIPFromXFF("10.0.0.0/8"), func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", "127.0.0.1, 99.99.99.99")
		})
		if got == "127.0.0.1" {
			t.Fatal("VULNERABLE: returned attacker-supplied loopback")
		}
		if got != "99.99.99.99" {
			t.Errorf("want 99.99.99.99 (rightmost-untrusted), got %q", got)
		}
	})

	// Unlike RealIP, ClientIPFromHeader reads ONLY the header the user opted
	// into. Attacker-supplied True-Client-IP / X-Forwarded-For are ignored.
	t.Run("only_opted_in_header_is_read", func(t *testing.T) {
		got := run(t, ClientIPFromHeader("X-Real-IP"), func(r *http.Request) {
			r.Header.Set("True-Client-IP", "1.1.1.1")  // attacker-supplied; ignored.
			r.Header.Set("X-Forwarded-For", "2.2.2.2") // attacker-supplied; ignored.
			r.Header.Set("X-Real-IP", "203.0.113.7")   // set by the trusted proxy.
		})
		if got != "203.0.113.7" {
			t.Errorf("want 203.0.113.7 (the only trusted header), got %q", got)
		}
	})
}

// "Multiple XFF headers" attack — Go's http.Header.Get returns only the first
// value, but the security-relevant rightmost IP is in the LAST instance after
// the proxy appends. We must merge all values; otherwise an attacker can pick
// which header the server sees by sending a duplicate.
//
// See https://adam-p.ca/blog/2022/03/x-forwarded-for/ "Multiple headers".
func TestXFF_MultipleHeadersMerged(t *testing.T) {
	// Attacker sends two XFF headers; the trusted proxy appends to the second
	// one (per RFC 2616). The genuine client is at the right of the merged
	// chain. A naive http.Header.Get() implementation would only see the first
	// header and return the attacker's prepended value.
	got := run(t, ClientIPFromXFF("198.51.100.0/24"), func(r *http.Request) {
		r.Header.Add("X-Forwarded-For", "127.0.0.1")             // attacker-injected.
		r.Header.Add("X-Forwarded-For", "8.8.8.8, 198.51.100.1") // appended by trusted proxy.
	})
	if got != "8.8.8.8" {
		t.Errorf("want 8.8.8.8, got %q", got)
	}
}

// TestClientIPFromHeader_MultiValueLastWins is a regression pin for the
// defense-in-depth gap fixed in a8e3dbf. The pre-fix implementation read
// the trusted single-IP header with r.Header.Get(), which returns only
// the first header value Go saw on the wire — so a misconfigured proxy
// that appended (instead of overwriting) the trusted header, or any
// chain where another proxy already set it, could surface an attacker-
// controlled values[0].
//
// Pinned post-fix contract: ClientIPFromHeader reads r.Header.Values()
// and uses the LAST entry (closest hop = most trusted, same
// rightmost-untrusted spirit as ClientIPFromXFF). Fail-closed on garbage
// at the last position; we do NOT fall back to earlier values — those
// came from hops further from us and are less trustworthy by definition.
func TestClientIPFromHeader_MultiValueLastWins(t *testing.T) {
	tt := []struct {
		name   string
		values []string // appended in order via Header.Add
		out    string
	}{
		// Baseline: single value (the well-configured case) still works.
		{"single_value_unchanged", []string{"100.1.1.1"}, "100.1.1.1"},

		// Attack: attacker sets X-Real-IP, then proxy appends (instead of
		// overwriting) its own value. Last value (the proxy's) must win.
		{"attacker_then_proxy", []string{"6.6.6.6", "100.2.2.2"}, "100.2.2.2"},

		// Three hops; only the last is trusted.
		{"three_values_last_wins", []string{"6.6.6.6", "5.5.5.5", "100.3.3.3"}, "100.3.3.3"},

		// Fail-closed: last value is garbage. We do NOT fall back to the
		// previous value — that value was set by someone further from us in
		// the chain and is by definition less trustworthy than what our
		// nearest hop produced (even if what they produced is garbage).
		{"last_unparseable_no_fallback", []string{"100.4.4.4", "garbage"}, ""},

		// Fail-closed: last value is empty. Same rationale.
		{"last_empty_no_fallback", []string{"100.5.5.5", ""}, ""},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromHeader("X-Real-IP"), func(r *http.Request) {
				for _, v := range tc.values {
					r.Header.Add("X-Real-IP", v)
				}
			})
			if got != tc.out {
				t.Errorf("VULNERABLE or wrong: want %q, got %q", tc.out, got)
			}
		})
	}
}

// TestXFF_V4MappedIPv6BypassesTrustedV4Prefix demonstrates a gap in the
// rightmost-untrusted walker: netip.Prefix.Contains returns false when an
// IPv4-mapped IPv6 address (::ffff:a.b.c.d) is checked against a plain v4
// prefix. So an attacker who can inject the v4-mapped form of an internal
// address into XFF — and whose own connecting IP happens to be in the
// trusted range (e.g. a compromised host inside an internal VPC) — gets a
// trusted-looking value returned as "the client IP". Any downstream code
// that treats internal IPs as privileged (rate-limit exemptions, internal
// admin allowlists, audit pipelines) then mis-classifies the request.
//
// Pinned post-fix behavior: parseHeaderAddr calls Unmap() before the prefix
// check, so ::ffff:10.0.0.5 is recognized as 10.0.0.5 and skipped along with
// the rest of the trusted chain — leaving no untrusted IP and an empty
// GetClientIP.
func TestXFF_V4MappedIPv6BypassesTrustedV4Prefix(t *testing.T) {
	tt := []struct {
		name     string
		prefixes []string
		xff      string
		out      string
	}{
		{
			name:     "internal_address_via_v4_mapped",
			prefixes: []string{"10.0.0.0/8"},
			xff:      "::ffff:10.0.0.5, 10.0.0.1",
			out:      "",
		},
		{
			name:     "loopback_via_v4_mapped",
			prefixes: []string{"127.0.0.0/8", "10.0.0.0/8"},
			xff:      "::ffff:127.0.0.1, 10.0.0.1",
			out:      "",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromXFF(tc.prefixes...), func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", tc.xff)
			})
			if got != tc.out {
				t.Errorf("VULNERABLE: v4-mapped IPv6 escaped v4 trusted prefix — want %q, got %q", tc.out, got)
			}
		})
	}
}

// TestXFF_IPv6ZoneIDBypassesTrustedV6Prefix demonstrates the same shape of
// gap for IPv6 trust prefixes: netip.Prefix.Contains returns false for any
// zoned IPv6 address, so an attacker-injected zone suffix escapes the trust
// check and the value is returned as "the client IP". Zone IDs are
// link-local concepts; they have no meaning on the wire and should never
// appear in a real XFF chain.
//
// Pinned post-fix behavior: parseHeaderAddr strips the zone before the
// prefix check, so 2606:4700::1%attacker is recognized as in 2606:4700::/32
// and skipped.
func TestXFF_IPv6ZoneIDBypassesTrustedV6Prefix(t *testing.T) {
	got := run(t, ClientIPFromXFF("2606:4700::/32"), func(r *http.Request) {
		r.Header.Set("X-Forwarded-For", "2606:4700::1%attacker, 2606:4700::5")
	})
	if got != "" {
		t.Errorf("VULNERABLE: zoned IPv6 escaped v6 trusted prefix — want %q, got %q", "", got)
	}
}

// TestClientIPFromRemoteAddr_V4MappedIsUnmapped pins the desired
// normalization for v4 connections accepted on a dual-stack listener:
// Go's net/http surfaces those as ::ffff:a.b.c.d in r.RemoteAddr, and
// returning them in that v6 form splits one logical client across two
// rate-limit buckets, log keys, and prefix-check buckets depending on
// listener configuration. Pinned post-fix behavior: ClientIPFromRemoteAddr
// calls Unmap() before storing.
func TestClientIPFromRemoteAddr_V4MappedIsUnmapped(t *testing.T) {
	tt := []struct {
		name       string
		remoteAddr string
		out        string
	}{
		{"v4_mapped_with_port", "[::ffff:1.2.3.4]:1234", "1.2.3.4"},
		{"v4_mapped_bare", "::ffff:1.2.3.4", "1.2.3.4"},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromRemoteAddr, func(r *http.Request) {
				r.RemoteAddr = tc.remoteAddr
			})
			if got != tc.out {
				t.Errorf("want %q (unmapped v4 form), got %q", tc.out, got)
			}
		})
	}
}

// TestXFF_FailClosedOnUnparseable pins the fail-closed contract for the
// rightmost-untrusted XFF walk (PR #967 review issue #6, adam-p): if the
// walker encounters an entry that does not parse as an IP, it MUST abandon
// the walk and leave no client IP set. Rightmost-untrusted relies on being
// able to read every hop to the right of the client; an unparseable hop is
// indistinguishable from a hostile or missing one, so we cannot safely keep
// walking past it.
//
// Empty / whitespace-only entries are NOT parse failures — they are dropped
// by trim before parsing, and do not trip fail-closed.
func TestXFF_FailClosedOnUnparseable(t *testing.T) {
	tt := []struct {
		name     string
		prefixes []string
		xff      []string
		out      string
	}{
		// Rightmost entry is garbage — walker fails closed before reaching the
		// otherwise-valid 1.1.1.1 to the left. Without fail-closed, garbage
		// to the right of a real client could be silently elided.
		{
			name: "garbage_rightmost_no_prefixes",
			xff:  []string{"1.1.1.1, garbage"},
			out:  "",
		},
		// Garbage between the client and a trusted proxy hop. Without fail-
		// closed, the walker would skip the trusted hop, silently skip the
		// garbage, and return 203.0.113.7 — a spoofable result.
		{
			name:     "garbage_between_client_and_trusted_proxy",
			prefixes: []string{"10.0.0.0/8"},
			xff:      []string{"203.0.113.7, garbage, 10.0.0.1"},
			out:      "",
		},
		// Garbage past a multi-hop trusted chain. Same shape with more
		// trusted hops in front; behaviour must be identical.
		{
			name:     "garbage_past_trusted_chain",
			prefixes: []string{"10.0.0.0/8"},
			xff:      []string{"203.0.113.7, garbage, 10.0.0.1, 10.0.0.2"},
			out:      "",
		},
		// Negative case: garbage sits in a left-hand header that the walker
		// never reaches because it returns at a valid untrusted entry first.
		// Fail-closed must NOT over-trigger here.
		{
			name:     "garbage_in_unreachable_left_header",
			prefixes: []string{"10.0.0.0/8"},
			xff:      []string{"garbage", "203.0.113.7, 10.0.0.1"},
			out:      "203.0.113.7",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, ClientIPFromXFF(tc.prefixes...), func(r *http.Request) {
				for _, v := range tc.xff {
					r.Header.Add("X-Forwarded-For", v)
				}
			})
			if got != tc.out {
				t.Errorf("want %q, got %q", tc.out, got)
			}
		})
	}
}

// run invokes mw with the request constructed by buildReq and returns the
// client IP captured inside the handler (or "" if none was set).
func run(t *testing.T, mw func(http.Handler) http.Handler, buildReq func(*http.Request)) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	buildReq(req)

	var got string
	r := chi.NewRouter()
	r.Use(mw)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		got = GetClientIP(r.Context())
	})
	r.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

// runChain is like run but applies multiple middlewares in order.
func runChain(t *testing.T, mws []func(http.Handler) http.Handler, buildReq func(*http.Request)) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	buildReq(req)

	var got string
	r := chi.NewRouter()
	for _, mw := range mws {
		r.Use(mw)
	}
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		got = GetClientIP(r.Context())
	})
	r.ServeHTTP(httptest.NewRecorder(), req)
	return got
}
