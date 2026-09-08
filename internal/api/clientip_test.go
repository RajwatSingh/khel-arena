package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// clientIP decides two things at once: what goes in the session audit trail,
// and what the login limiter counts against. Both are wrong in a different
// way if the hop count is wrong, so the cases below are grouped by what an
// operator would have misconfigured rather than by input shape.

func request(remote string, forwarded ...string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	r.RemoteAddr = remote
	for _, f := range forwarded {
		r.Header.Add("X-Forwarded-For", f)
	}
	return r
}

func TestClientIPTrustsNothingByDefault(t *testing.T) {
	// The direct-exposure deployment. A caller that sends the header gets no
	// say in the answer at all -- this is the case that keeps a forged
	// address out of the audit row and out of the limiter's key.
	r := request("203.0.113.9:51000", "1.2.3.4")

	if got := clientIP(r, 0); got != "203.0.113.9" {
		t.Errorf("clientIP = %q, want the peer address; a forged header was believed", got)
	}
}

func TestClientIPBehindOneProxy(t *testing.T) {
	// Caddy or nginx on the same host: it appends the real client and hands
	// the request over, so the chain is [client] and the peer is the proxy.
	r := request("127.0.0.1:44321", "203.0.113.9")

	if got := clientIP(r, 1); got != "203.0.113.9" {
		t.Errorf("clientIP = %q, want 203.0.113.9", got)
	}
}

func TestClientIPIgnoresForgedPrefix(t *testing.T) {
	// The attack the hop count exists to stop. A client sends its own
	// X-Forwarded-For; the proxy appends the address it actually saw rather
	// than replacing the header, so the forgery ends up to the left of the
	// entry we count back to.
	//
	// If this ever returns the forged value, the login limiter can be stepped
	// around one header at a time, which is worse than having no limiter.
	r := request("127.0.0.1:44321", "9.9.9.9, 203.0.113.9")

	if got := clientIP(r, 1); got != "203.0.113.9" {
		t.Errorf("clientIP = %q, want 203.0.113.9; the caller chose its own address", got)
	}
}

func TestClientIPSplitAcrossHeaders(t *testing.T) {
	// A list split across repeated headers is legal, and Header.Get would
	// read only the first -- which would make the chain look one hop shorter
	// than it is and land on the wrong entry.
	r := request("127.0.0.1:44321", "9.9.9.9", "203.0.113.9")

	if got := clientIP(r, 1); got != "203.0.113.9" {
		t.Errorf("clientIP = %q, want 203.0.113.9", got)
	}
}

func TestClientIPTwoProxies(t *testing.T) {
	// A CDN in front of the local proxy. Each hop appended one entry.
	r := request("127.0.0.1:44321", "203.0.113.9, 198.51.100.7")

	if got := clientIP(r, 2); got != "203.0.113.9" {
		t.Errorf("clientIP = %q, want 203.0.113.9", got)
	}
}

func TestClientIPChainShorterThanHopCount(t *testing.T) {
	// Configured for a proxy that is not there, or one that replaced the
	// header instead of appending. Counting back past the start of the chain
	// must not wrap around to an entry a caller supplied -- the peer is the
	// only thing still worth believing.
	r := request("198.51.100.7:44321")

	if got := clientIP(r, 2); got != "198.51.100.7" {
		t.Errorf("clientIP = %q, want the peer address when the chain runs out", got)
	}
}

func TestClientIPRejectsUnparseableEntry(t *testing.T) {
	// A forged entry can be any string. Letting one through would give an
	// attacker an unbounded supply of distinct limiter keys -- a fresh burst
	// allowance per request -- which is exactly what the bucket sweep is
	// there to prevent.
	r := request("127.0.0.1:44321", "not-an-address")

	if got := clientIP(r, 1); got != "127.0.0.1" {
		t.Errorf("clientIP = %q, want the peer address for an unparseable entry", got)
	}
}

func TestClientIPHandlesIPv6(t *testing.T) {
	r := request("[::1]:44321", "2001:db8::1")

	if got := clientIP(r, 1); got != "2001:db8::1" {
		t.Errorf("clientIP = %q, want 2001:db8::1", got)
	}
	if got := clientIP(request("[2001:db8::2]:44321"), 0); got != "2001:db8::2" {
		t.Errorf("clientIP = %q, want the peer with its port stripped", got)
	}
}
