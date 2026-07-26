// Package httpclient provides a factory for outbound *http.Client instances
// whose transport is wrapped with OpenTelemetry (otelhttp) so every outbound
// call becomes a SpanKind=Client span with trace context (traceparent)
// propagated to downstream services. Business code constructs clients via
// httpclient.New instead of bare &http.Client{}.
package httpclient

import (
	"net/http"
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

// Option configures the *http.Client returned by New.
type Option func(*http.Client)

// WithTimeout sets the client Timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *http.Client) {
		c.Timeout = d
	}
}

// WithTransport sets a custom base RoundTripper. When instrumentation is on,
// this is wrapped by otelhttp; otherwise used as-is.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *http.Client) {
		if rt != nil {
			c.Transport = rt
		}
	}
}

// New returns an *http.Client. When instrumentation is enabled, its transport
// is wrapped with otelhttp.NewTransport so outbound calls produce
// SpanKind=Client spans and propagate traceparent. Defaults match http.Client
// when no options are supplied.
func New(opts ...Option) *http.Client {
	c := &http.Client{}
	for _, opt := range opts {
		opt(c)
	}
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	if instrumentHTTP {
		c.Transport = otelhttp.NewTransport(base)
	} else {
		c.Transport = base
	}
	return c
}
