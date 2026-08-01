package httpclient

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TestNew_DefaultClientHasTransport 验证默认构造的 client 有非空 transport。
func TestNew_DefaultClientHasTransport(t *testing.T) {
	SetInstrumentation(true)
	c := New()
	if c == nil {
		t.Fatal("New returned nil client")
	}
	if c.Transport == nil {
		t.Fatal("default transport is nil")
	}
}

// TestNew_InstrumentedWrapsWithOtelHTTP 验证 InstrumentHTTP=true 时
// transport 被 otelhttp 包装（对应 spec: LLM 调用出现在 trace）。
func TestNew_InstrumentedWrapsWithOtelHTTP(t *testing.T) {
	SetInstrumentation(true)
	defer SetInstrumentation(true)
	c := New()
	if _, ok := c.Transport.(*otelhttp.Transport); !ok {
		t.Fatalf("expected *otelhttp.Transport when instrumented, got %T", c.Transport)
	}
}

// TestNew_DisabledReturnsPlainTransport 验证 InstrumentHTTP=false 时
// transport 不含 otelhttp 包装（对应 spec: 可按配置关闭）。
func TestNew_DisabledReturnsPlainTransport(t *testing.T) {
	SetInstrumentation(false)
	defer SetInstrumentation(true)
	c := New()
	if _, ok := c.Transport.(*otelhttp.Transport); ok {
		t.Fatalf("expected plain transport when disabled, got *otelhttp.Transport")
	}
}

// TestWithTimeout 验证 Timeout option 生效（对应 spec: 保留各调用点自定义）。
func TestWithTimeout(t *testing.T) {
	c := New(WithTimeout(5 * time.Second))
	if c.Timeout != 5*time.Second {
		t.Fatalf("expected timeout 5s, got %v", c.Timeout)
	}
}

// TestWithTransport 验证自定义 transport 被保留并（启用时）被 otelhttp 包装。
func TestWithTransport(t *testing.T) {
	SetInstrumentation(true)
	defer SetInstrumentation(true)
	custom := &http.Transport{}
	c := New(WithTransport(custom))
	if _, ok := c.Transport.(*otelhttp.Transport); !ok {
		t.Fatalf("expected *otelhttp.Transport wrapping custom base, got %T", c.Transport)
	}
}

// TestSetInstrumentation_TogglesGlobally 验证开关全局生效。
func TestSetInstrumentation_TogglesGlobally(t *testing.T) {
	SetInstrumentation(false)
	c1 := New()
	if _, ok := c1.Transport.(*otelhttp.Transport); ok {
		t.Fatal("disabled should not wrap")
	}
	SetInstrumentation(true)
	defer SetInstrumentation(true)
	c2 := New()
	if _, ok := c2.Transport.(*otelhttp.Transport); !ok {
		t.Fatal("enabled should wrap")
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

// TestSetProxy_AppliesToNewClients 验证 SetProxy 后 New 构造的 client 走代理。
func TestSetProxy_AppliesToNewClients(t *testing.T) {
	SetInstrumentation(false) // 裸 transport，方便断言 base
	t.Cleanup(func() {
		SetProxy("")
		SetInstrumentation(true)
	})
	if err := SetProxy("http://proxy.example.com:8080"); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}
	c := New()
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport base, got %T", c.Transport)
	}
	got, err := tr.Proxy(&http.Request{URL: mustParseURL(t, "http://target.example.com/")})
	if err != nil {
		t.Fatalf("Proxy() error: %v", err)
	}
	if got == nil || got.Host != "proxy.example.com:8080" {
		t.Fatalf("expected proxy host proxy.example.com:8080, got %v", got)
	}
}

// TestSetProxy_EmptyClearsProxy 验证空串清除代理（恢复 DefaultTransport）。
func TestSetProxy_EmptyClearsProxy(t *testing.T) {
	SetInstrumentation(false)
	t.Cleanup(func() {
		SetProxy("")
		SetInstrumentation(true)
	})
	if err := SetProxy("http://proxy.example.com:8080"); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}
	_ = New()
	if err := SetProxy(""); err != nil {
		t.Fatalf("SetProxy empty: %v", err)
	}
	c := New()
	if c.Transport != http.DefaultTransport {
		t.Fatalf("expected http.DefaultTransport after clearing proxy, got %T", c.Transport)
	}
}

// TestSetProxy_RejectsUnsupportedScheme 验证非法 scheme 报错且不污染全局状态。
func TestSetProxy_RejectsUnsupportedScheme(t *testing.T) {
	t.Cleanup(func() { SetProxy("") })
	if err := SetProxy("ftp://proxy.example.com:21"); err == nil {
		t.Fatal("expected error for ftp scheme, got nil")
	}
	if currentProxyTransport() != nil {
		t.Fatal("global proxy transport should remain nil after rejected scheme")
	}
}

// TestNew_WithTransportOverridesProxy 验证显式 WithTransport 覆盖全局代理。
func TestNew_WithTransportOverridesProxy(t *testing.T) {
	SetInstrumentation(false)
	t.Cleanup(func() {
		SetProxy("")
		SetInstrumentation(true)
	})
	if err := SetProxy("http://proxy.example.com:8080"); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}
	custom := &http.Transport{}
	c := New(WithTransport(custom))
	if c.Transport != custom {
		t.Fatalf("expected custom transport to override proxy, got %T", c.Transport)
	}
}
