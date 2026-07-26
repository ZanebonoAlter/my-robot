package httpclient

import (
	"net/http"
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
