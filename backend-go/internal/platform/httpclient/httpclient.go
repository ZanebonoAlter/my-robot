// Package httpclient provides a factory for outbound *http.Client instances
// whose transport is wrapped with OpenTelemetry (otelhttp) so every outbound
// call becomes a SpanKind=Client span with trace context (traceparent)
// propagated to downstream services. Business code constructs clients via
// httpclient.New instead of bare &http.Client{}.
package httpclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// instrumentHTTP toggles otelhttp transport wrapping. Set once at startup
// from main.go via SetInstrumentation based on config.Tracing.InstrumentHTTP.
var instrumentHTTP = true

// SetInstrumentation toggles otelhttp wrapping globally. When false, New
// returns a plain client (current behaviour, for troubleshooting).
func SetInstrumentation(enabled bool) {
	instrumentHTTP = enabled
}

// proxyTransport is a pre-built base RoundTripper carrying the globally
// configured outbound proxy (set via SetProxy). nil means no proxy is
// configured — New then falls back to http.DefaultTransport (which itself
// honours HTTP_PROXY/HTTPS_PROXY env vars via ProxyFromEnvironment). SetProxy
// swaps this atomically; clients already built keep their transport.
var (
	proxyMu        sync.RWMutex
	proxyTransport http.RoundTripper
)

// SetProxy configures the global outbound proxy applied to every client
// returned by New (unless the caller overrides Transport via WithTransport).
// An empty URL clears the proxy (restores direct / DefaultTransport behaviour).
// Accepted schemes: http, https, socks5. Returns an error for malformed or
// unsupported URLs so the settings API can surface bad input without mutating
// global state.
func SetProxy(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		proxyMu.Lock()
		proxyTransport = nil
		proxyMu.Unlock()
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL %q: %w", rawURL, err)
	}
	switch u.Scheme {
	case "http", "https", "socks5":
	default:
		return fmt.Errorf("unsupported proxy scheme %q: only http/https/socks5 are allowed", u.Scheme)
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return fmt.Errorf("http.DefaultTransport is not *http.Transport, cannot install proxy")
	}
	tr := base.Clone()
	tr.Proxy = http.ProxyURL(u)
	proxyMu.Lock()
	proxyTransport = tr
	proxyMu.Unlock()
	return nil
}

// currentProxyTransport returns the active proxy transport under a read lock.
func currentProxyTransport() http.RoundTripper {
	proxyMu.RLock()
	defer proxyMu.RUnlock()
	return proxyTransport
}

// Option configures the *http.Client returned by New.
type Option func(*http.Client)

// WithTimeout sets the client Timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *http.Client) {
		c.Timeout = d
	}
}

// WithTransport sets a custom base RoundTripper. When instrumentation is on,
// this is wrapped by otelhttp; otherwise used as-is. An explicit transport
// overrides the global proxy (caller takes responsibility for proxying).
func WithTransport(rt http.RoundTripper) Option {
	return func(c *http.Client) {
		if rt != nil {
			c.Transport = rt
		}
	}
}

// New returns an *http.Client. When instrumentation is enabled, its transport
// is wrapped with otelhttp.NewTransport so outbound calls produce
// SpanKind=Client spans and propagate traceparent. When no Transport option is
// supplied, the global proxy transport (SetProxy) is used if configured,
// otherwise http.DefaultTransport. Defaults match http.Client when no options
// are supplied and no proxy is set.
func New(opts ...Option) *http.Client {
	c := &http.Client{}
	for _, opt := range opts {
		opt(c)
	}
	base := c.Transport
	if base == nil {
		if pt := currentProxyTransport(); pt != nil {
			base = pt
		} else {
			base = http.DefaultTransport
		}
	}
	if instrumentHTTP {
		c.Transport = otelhttp.NewTransport(base)
	} else {
		c.Transport = base
	}
	return c
}
