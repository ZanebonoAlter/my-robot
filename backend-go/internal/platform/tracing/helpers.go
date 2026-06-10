package tracing

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
