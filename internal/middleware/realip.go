package middleware

import (
	"net"
	"net/http"
)

// RealIP rewrites r.RemoteAddr to the real visitor IP that Cloudflare reports in
// the CF-Connecting-IP header, so logging and any future rate-limiting/geo logic
// see the client rather than Cloudflare's (or Caddy's) address. Wrap the router
// with this as the outermost middleware so every handler sees the corrected addr.
//
// SECURITY: this trusts CF-Connecting-IP unconditionally, which is only safe
// because the origin is reachable solely through Cloudflare → Caddy (localhost).
// The header is client-spoofable, so the droplet's firewall MUST restrict :80/:443
// to Cloudflare's published IP ranges (or use Authenticated Origin Pulls). Without
// that, an attacker hitting the origin directly could forge any visitor IP.
func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
			// Keep RemoteAddr in its documented host:port form (and bracket IPv6)
			// by reusing the original connection port, so net.SplitHostPort still
			// works downstream — a bare IP would break it.
			_, port, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				port = "0"
			}
			r.RemoteAddr = net.JoinHostPort(ip, port)
		}
		next.ServeHTTP(w, r)
	})
}
