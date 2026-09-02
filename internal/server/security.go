package server

import "net/http"

// defaultContentSecurityPolicy is the browser policy for m2h's own pages.
//
//	script-src 'self'         — every script (app bundle, Mermaid, KaTeX,
//	                            ZenUML ESM, the Vega trio) loads from this
//	                            origin; no CDN, no inline script, no inline
//	                            event handlers, and no 'unsafe-eval': Vega
//	                            charts run their expressions through the AST
//	                            interpreter Vega-Embed bundles (embed option
//	                            ast: true) instead of the runtime's
//	                            generated-code path, so raw HTML in a
//	                            Markdown document — or a chart spec — can
//	                            never gain code evaluation.
//	style-src 'unsafe-inline' — React, Mermaid and KaTeX all emit element
//	                            styles, so inline styles stay allowed for now.
//	img/media-src http(s:)    — Markdown legitimately embeds remote media.
//	frame-src 'none'          — m2h publishes no iframe embeds.
//
// HSTS is deliberately absent: m2h never sees the TLS connection. A reverse
// proxy terminating TLS owns that header.
const defaultContentSecurityPolicy = "default-src 'self'; " +
	"base-uri 'none'; " +
	"object-src 'none'; " +
	"frame-ancestors 'self'; " +
	"form-action 'none'; " +
	"frame-src 'none'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: http: https:; " +
	"media-src 'self' blob: http: https:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"worker-src 'self' blob:"

// securityHeaders wraps the whole mux so every response — pages, JSON APIs,
// attachments, runtime scripts, 404s — carries the same browser hardening
// baseline. A handler may still override an individual header afterwards
// (the assets route replaces the CSP with its stricter sandbox policy).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		headers := response.Header()
		headers.Set("Content-Security-Policy", defaultContentSecurityPolicy)
		headers.Set("X-Content-Type-Options", "nosniff")
		// Private-document service: never leak the document URL to another
		// origin through the Referer header.
		headers.Set("Referrer-Policy", "same-origin")
		headers.Set("X-Frame-Options", "SAMEORIGIN")
		headers.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		next.ServeHTTP(response, request)
	})
}
