package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// instrumentSource is the unit under test (defined in instrumenter.go).
// It takes a filename (used for /*line*/ directives) and raw source bytes,
// rewrites eligible methods to inject OTel spans, and returns the new source.
// It must be idempotent.

// mustParse is a test helper: ensures the rewritten source is syntactically
// valid Go (catches injection bugs that break the AST).
func mustParse(t *testing.T, name string, src []byte) {
	t.Helper()
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, name, src, parser.ParseComments); err != nil {
		t.Fatalf("output of %s is not valid Go: %v\n--- source ---\n%s", name, err, src)
	}
}

// contains is a small helper to keep assertions readable.
func contains(t *testing.T, src []byte, want string) {
	t.Helper()
	if !strings.Contains(string(src), want) {
		t.Fatalf("expected output to contain:\n  %q\n--- actual ---\n%s", want, src)
	}
}

func notContains(t *testing.T, src []byte, avoid string) {
	t.Helper()
	if strings.Contains(string(src), avoid) {
		t.Fatalf("expected output to NOT contain %q\n--- actual ---\n%s", avoid, src)
	}
}

// countOccurrence counts non-overlapping occurrences of s in src.
func countOccurrence(src []byte, s string) int {
	return strings.Count(string(src), s)
}

// ---------------------------------------------------------------------------
// Test 1: exported method with ctx first param is instrumented; span name
// is "TypeName.Method" (pointer receiver dereferenced).
// ---------------------------------------------------------------------------

func TestInstrument_ExportedMethodWithCtx(t *testing.T) {
	src := []byte(`package svc

import "context"

type S struct{}

func (s *S) DoThing(ctx context.Context, id int) error {
	return nil
}
`)
	out, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	mustParse(t, "svc.go", out)
	contains(t, out, `otel.Tracer(tracing.ServiceName).Start(ctx, "S.DoThing")`)
	contains(t, out, `defer span.End()`)
	// unnamed error return -> no error-recording defer
	notContains(t, out, `span.RecordError(`)
	mustParse(t, "svc.go", out)
}

// ---------------------------------------------------------------------------
// Test 2: non-exported method is skipped.
// ---------------------------------------------------------------------------

func TestInstrument_NonExportedMethodSkipped(t *testing.T) {
	src := []byte(`package svc

import "context"

type S struct{}

func (s *S) doThing(ctx context.Context) error {
	return nil
}
`)
	out, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	if string(out) != string(src) {
		t.Fatalf("non-exported method should be untouched;\n--- got ---\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Test 3: exported method whose first param is NOT context.Context is skipped.
// ---------------------------------------------------------------------------

func TestInstrument_NoCtxFirstParamSkipped(t *testing.T) {
	src := []byte(`package svc

type S struct{}

func (s *S) NoCtx(id int) error {
	return nil
}

func (s *S) CtxSecond(id int, ctx _ctx) error {
	return nil
}
`)
	out, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	if string(out) != string(src) {
		t.Fatalf("method without ctx first param should be untouched;\n--- got ---\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Test 4: exclusion — method body already contains a span-creating call is
// skipped (covers existing go-instrument methods + hand-written spans).
// ---------------------------------------------------------------------------

func TestInstrument_ExcludesExistingSpan(t *testing.T) {
	cases := []string{
		`otel.Tracer("x").Start(ctx, "x")`,
		`tracer.Start(ctx, "x")`,
		`tracing.Tracer("x").Start(ctx, "x")`,
	}
	for _, existing := range cases {
		src := []byte(`package svc

import "context"

type S struct{}

func (s *S) Already(ctx context.Context) error {
	_, span := ` + existing + `
	defer span.End()
	return nil
}
`)
		out, err := instrumentSource("svc.go", src)
		if err != nil {
			t.Fatalf("instrumentSource (%s): %v", existing, err)
		}
		if got := countOccurrence(out, `Start(ctx`); got != 1 {
			t.Fatalf("exclusion failed for %q: expected exactly 1 Start call, got %d\n--- out ---\n%s", existing, got, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 5: idempotency — running twice yields identical output.
// ---------------------------------------------------------------------------

func TestInstrument_Idempotent(t *testing.T) {
	src := []byte(`package svc

import "context"

type S struct{}

func (s *S) DoThing(ctx context.Context, id int) (err error) {
	return nil
}

func (s *S) Plain(ctx context.Context) error {
	return nil
}
`)
	first, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := instrumentSource("svc.go", first)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("not idempotent: second run changed output\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// ---------------------------------------------------------------------------
// Test 6: named err — method returning named `err error` gets an error-
// recording defer; method returning unnamed error does not.
// ---------------------------------------------------------------------------

func TestInstrument_NamedErrDefer(t *testing.T) {
	src := []byte(`package svc

import "context"

type S struct{}

func (s *S) WithErr(ctx context.Context) (err error) {
	return nil
}

func (s *S) WithoutErr(ctx context.Context) error {
	return nil
}
`)
	out, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	mustParse(t, "svc.go", out)
	contains(t, out, `if err != nil {`)
	contains(t, out, `span.RecordError(err)`)
	contains(t, out, `span.SetStatus(`)
	// exactly one error-recording defer (only WithErr)
	if got := countOccurrence(out, `span.RecordError(err)`); got != 1 {
		t.Fatalf("expected 1 RecordError, got %d\n--- out ---\n%s", got, out)
	}
}

// ---------------------------------------------------------------------------
// Test 7: imports are added when missing (otel, codes aliased otelCodes
// when named err present, tracing).
// ---------------------------------------------------------------------------

func TestInstrument_AddsMissingImports(t *testing.T) {
	src := []byte(`package svc

import "context"

type S struct{}

func (s *S) WithErr(ctx context.Context) (err error) {
	return nil
}
`)
	out, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	mustParse(t, "svc.go", out)
	contains(t, out, `"go.opentelemetry.io/otel"`)
	contains(t, out, `"go.opentelemetry.io/otel/codes"`)
	contains(t, out, `"syntopica-backend/internal/platform/tracing"`)
}

// ---------------------------------------------------------------------------
// Test 8: package-level exported func with ctx is instrumented with span
// name "pkg.Func".
// ---------------------------------------------------------------------------

func TestInstrument_PackageLevelFunc(t *testing.T) {
	src := []byte(`package svc

import "context"

func DoGlobal(ctx context.Context) error {
	return nil
}
`)
	out, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	mustParse(t, "svc.go", out)
	contains(t, out, `Start(ctx, "svc.DoGlobal")`)
}

// ---------------------------------------------------------------------------
// Test 9: value receiver uses the bare type name.
// ---------------------------------------------------------------------------

func TestInstrument_ValueReceiver(t *testing.T) {
	src := []byte(`package svc

import "context"

type S struct{}

func (s S) ValueRecv(ctx context.Context) error {
	return nil
}
`)
	out, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	mustParse(t, "svc.go", out)
	contains(t, out, `Start(ctx, "S.ValueRecv")`)
}

// ---------------------------------------------------------------------------
// Test 10: file with no eligible methods is returned byte-identical.
// ---------------------------------------------------------------------------

func TestInstrument_NoEligibleMethodsUnchanged(t *testing.T) {
	src := []byte(`package svc

import "context"

type S struct{}

func (s *S) private(ctx context.Context) error {
	return nil
}
`)
	out, err := instrumentSource("svc.go", src)
	if err != nil {
		t.Fatalf("instrumentSource: %v", err)
	}
	if string(out) != string(src) {
		t.Fatalf("file with no eligible methods should be unchanged;\n--- got ---\n%s", out)
	}
}
