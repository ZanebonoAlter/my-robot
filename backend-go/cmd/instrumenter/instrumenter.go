package main

// Method-level OTel span auto-instrumenter.
//
// instrumentSource rewrites a single Go source file: for every exported
// method or package-level function whose first parameter is a named
// `context.Context`, it injects an OpenTelemetry span at the top of the body.
//
// Rules (see openspec/changes/add-method-auto-instrumentation):
//   - span name: "TypeName.Method" for methods, "pkg.Func" for functions
//   - skip when the first param is not a named context.Context
//   - skip when the body already contains a span-creating call
//     (otel.Tracer / tracer.Start / tracing.Tracer) -> idempotent + coexists
//     with hand-written spans
//   - when the result list has a named `err error`, inject an error-recording
//     defer (span.SetStatus + span.RecordError)
//   - add missing imports (otel, otel/codes, tracing); reuse existing aliases
//   - a /*line*/ directive is emitted before the first original statement to
//     keep debug positions anchored to the pre-rewrite source

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"

	"golang.org/x/tools/go/ast/astutil"
)

const (
	otelImportPath    = "go.opentelemetry.io/otel"
	codesImportPath   = "go.opentelemetry.io/otel/codes"
	tracingImportPath = "syntopica-backend/internal/platform/tracing"
)

// spliceOp describes raw text to insert at a byte offset of the original src.
type spliceOp struct {
	offset int
	text   string
}

// instrumentSource rewrites src (a single Go file) and returns the new bytes.
// It is idempotent: a second run over its own output returns it unchanged.
func instrumentSource(filename string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	aliases := resolveAliases(file)
	splices, anyNamedErr := analyze(fset, file, filename, aliases)
	if len(splices) == 0 {
		// Nothing to inject: leave the file byte-identical (idempotent no-op).
		return src, nil
	}

	rewritten := applySplices(src, splices)
	return finalize(rewritten, filename, anyNamedErr, aliases)
}

// analyze walks the file's declarations, collecting text splices for every
// eligible function/method. It also reports whether any injected method needs
// the error-recording defer (so finalize knows whether to add the codes import).
func analyze(fset *token.FileSet, file *ast.File, filename string, a aliases) ([]spliceOp, bool) {
	var ops []spliceOp
	anyNamedErr := false
	pkgName := file.Name.Name

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !fn.Name.IsExported() {
			continue
		}
		if fn.Body == nil {
			continue // external (body-less) declaration
		}
		paramName, ok := firstCtxParam(fn.Type)
		if !ok {
			continue
		}
		if hasExistingSpan(fn.Body) {
			continue // idempotency / coexistence with hand-written spans
		}

		typeName := ""
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			typeName = baseTypeName(fn.Recv.List[0].Type)
		}
		var spanName string
		if typeName != "" {
			spanName = typeName + "." + fn.Name.Name
		} else {
			spanName = pkgName + "." + fn.Name.Name
		}
		namedErr := hasNamedError(fn.Type)
		if namedErr {
			anyNamedErr = true
		}

		bodyOpenOff := fset.Position(fn.Body.Lbrace).Offset + 1 // byte after '{'
		ops = append(ops, spliceOp{
			offset: bodyOpenOff,
			text:   injectionBlock(paramName, spanName, namedErr, a),
		})

		if len(fn.Body.List) > 0 {
			first := fn.Body.List[0]
			stmtOff := fset.Position(first.Pos()).Offset
			pos := fset.Position(first.Pos())
			directive := fmt.Sprintf("/*line %s:%d:%d*/ ", filename, pos.Line, pos.Column)
			ops = append(ops, spliceOp{offset: stmtOff, text: directive})
		}
	}
	return ops, anyNamedErr
}

// applySplices inserts every op's text at its offset. Ops are applied from the
// largest offset down so that earlier offsets stay valid as the buffer grows.
func applySplices(src []byte, ops []spliceOp) []byte {
	sorted := make([]spliceOp, len(ops))
	copy(sorted, ops)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].offset > sorted[j].offset })

	var out []byte
	out = append(out, src...)
	for _, op := range sorted {
		out = append(out[:op.offset], append([]byte(op.text), out[op.offset:]...)...)
	}
	return out
}

// finalize re-parses the spliced source, adds any missing imports, and emits
// gofmt-formatted bytes.
func finalize(src []byte, filename string, anyNamedErr bool, a aliases) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("re-parse after injection: %w", err)
	}

	if a.addOtel {
		astutil.AddImport(fset, file, otelImportPath)
	}
	if anyNamedErr && a.addCodes {
		astutil.AddNamedImport(fset, file, a.codesName, codesImportPath)
	}
	if a.addTracing {
		astutil.AddImport(fset, file, tracingImportPath)
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, file); err != nil {
		return nil, fmt.Errorf("print: %w", err)
	}
	out, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt after injection: %w", err)
	}
	return out, nil
}

// injectionBlock builds the text inserted right after the body's opening brace.
func injectionBlock(paramName, spanName string, namedErr bool, a aliases) string {
	head := "\n\t" + paramName + ", span := " + a.otelName + ".Tracer(" + a.tracingName +
		".ServiceName).Start(" + paramName + ", \"" + spanName + "\")\n\tdefer span.End()"
	if !namedErr {
		return head
	}
	tail := "\n\tdefer func() {\n\t\tif err != nil {\n\t\t\tspan.SetStatus(" + a.codesName +
		".Error, \"error\")\n\t\t\tspan.RecordError(err)\n\t\t}\n\t}()"
	return head + tail
}

// aliases captures the selector names to use for the three OTel imports, based
// on what the file already imports. Missing imports get default names and are
// flagged for addition by finalize.
type aliases struct {
	otelName    string
	codesName   string
	tracingName string
	addOtel     bool
	addCodes    bool
	addTracing  bool
}

func resolveAliases(file *ast.File) aliases {
	a := aliases{
		otelName:    "otel",
		codesName:   "otelCodes",
		tracingName: "tracing",
		addOtel:     true,
		addCodes:    true,
		addTracing:  true,
	}
	for _, imp := range file.Imports {
		switch imp.Path.Value {
		case `"` + otelImportPath + `"`:
			a.otelName = importAlias(imp, "otel")
			a.addOtel = false
		case `"` + codesImportPath + `"`:
			a.codesName = importAlias(imp, "codes")
			a.addCodes = false
		case `"` + tracingImportPath + `"`:
			a.tracingName = importAlias(imp, "tracing")
			a.addTracing = false
		}
	}
	return a
}

func importAlias(imp *ast.ImportSpec, def string) string {
	if imp.Name != nil && imp.Name.Name != "" {
		return imp.Name.Name
	}
	return def
}

// firstCtxParam returns the name of the first parameter iff it is a named
// `context.Context`. Unnamed context params are rejected (cannot be referenced).
func firstCtxParam(ft *ast.FuncType) (string, bool) {
	if ft.Params == nil || len(ft.Params.List) == 0 {
		return "", false
	}
	field := ft.Params.List[0]
	if len(field.Names) == 0 {
		return "", false // unnamed, e.g. func(context.Context)
	}
	name := field.Names[0].Name
	if name == "_" {
		return "", false // discarded ctx: caller does not propagate it, skip
	}
	if !isContextType(field.Type) {
		return "", false
	}
	return name, true
}

func isContextType(t ast.Expr) bool {
	sel, ok := t.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "context" && sel.Sel.Name == "Context"
}

// hasNamedError reports whether the result list declares a named `err error`.
func hasNamedError(ft *ast.FuncType) bool {
	if ft.Results == nil {
		return false
	}
	for _, field := range ft.Results.List {
		id, ok := field.Type.(*ast.Ident)
		if !ok || id.Name != "error" {
			continue
		}
		for _, n := range field.Names {
			if n.Name == "err" {
				return true
			}
		}
	}
	return false
}

// baseTypeName strips pointer/generic wrappers to get the bare receiver type.
func baseTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return baseTypeName(e.X)
	case *ast.IndexExpr:
		return baseTypeName(e.X)
	case *ast.IndexListExpr:
		return baseTypeName(e.X)
	}
	return ""
}

// hasExistingSpan reports whether the body already creates a span via one of
// the three recognized call shapes. This drives idempotency and coexistence.
func hasExistingSpan(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// otel.Tracer(...) or tracing.Tracer(...)
		if sel.Sel.Name == "Tracer" {
			if ident, ok := sel.X.(*ast.Ident); ok && (ident.Name == "otel" || ident.Name == "tracing") {
				found = true
				return false
			}
		}
		// tracer.Start(...) or any <x>.Start where <x> is a tracer-named ident
		if sel.Sel.Name == "Start" {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "tracer" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
