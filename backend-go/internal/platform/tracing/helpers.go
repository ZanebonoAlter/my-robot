package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func GoWithTrace(ctx context.Context, name string, fn func(ctx context.Context)) {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		go fn(ctx)
		return
	}

	parentTraceID := spanCtx.TraceID().String()

	go func() { //nolint:gosec // deliberate: new root span linked via parent_trace_id
		ctx, span := Tracer("async").Start(
			context.Background(),
			name,
			trace.WithNewRoot(),
			trace.WithAttributes(
				attribute.String("parent_trace_id", parentTraceID),
				attribute.String("async.operation", name),
			),
		)
		defer span.End()

		fn(ctx)
	}()
}

func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func TraceIDFromContext(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	return ""
}

func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

func RecordError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
	}
}

func SetStatus(ctx context.Context, code int, msg string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	switch code {
	case 1:
		span.SetStatus(1, msg) // Error
	case 2:
		span.SetStatus(2, msg) // Ok
	default:
		span.SetStatus(0, msg) // Unset
	}
}

