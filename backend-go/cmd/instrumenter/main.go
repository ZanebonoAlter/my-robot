package main

// Command instrumenter scans service-layer packages and rewrites their Go
// source files to inject OpenTelemetry spans into eligible methods.
//
// Usage:
//
//	go run ./cmd/instrumenter [<dir>...]
//
// With no args it instruments the default target set (the service packages
// plus platform/airouter). With dir args it instruments exactly those
// directories (each resolved as a package). Files are rewritten in place;
// the command is idempotent.
//
// It is meant to be wired into `go generate ./...` (see the //go:generate
// directives placed in each target package).

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// defaultTargets are the package directories instrumented when no args given.
// tagmanagement/service/board is intentionally excluded (out of scope).
var defaultTargets = []string{
	"./internal/admin/service",
	"./internal/dataenrichment/service",
	"./internal/reader/service",
	"./internal/tagmanagement/service",
	"./internal/tagmanagement/service/core",
	"./internal/topicgraph/service",
	"./internal/platform/airouter",
}

func main() {
	log.SetFlags(0)
	targets := os.Args[1:]
	if len(targets) == 0 {
		targets = defaultTargets
	}

	changed, skipped, err := run(targets)
	if err != nil {
		log.Fatalf("instrumenter: %v", err)
	}
	log.Printf("instrumenter: %d file(s) rewritten, %d file(s) unchanged", changed, skipped)
}

// run loads each target package, instruments its non-test Go files, and writes
// back the ones that changed. Returns counts of rewritten / unchanged files.
func run(targets []string) (int, int, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax,
	}
	pkgs, err := packages.Load(cfg, targets...)
	if err != nil {
		return 0, 0, fmt.Errorf("load packages: %w", err)
	}

	var loadErrs []string
	for _, p := range pkgs {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, e.Error())
		}
	}
	// Loading may emit type-check warnings for files we are about to rewrite;
	// only hard fail on errors that prevent file enumeration.
	if len(loadErrs) > 0 && allPackagesEmpty(pkgs) {
		return 0, 0, fmt.Errorf("package load errors: %s", strings.Join(loadErrs, "; "))
	}

	changed, unchanged := 0, 0
	for _, p := range pkgs {
		for _, f := range p.GoFiles {
			info, err := os.Stat(f) //nolint:gosec // f is a trusted path enumerated by go/packages
			if err != nil {
				return changed, unchanged, fmt.Errorf("stat %s: %w", f, err)
			}
			rel := repoRelativePath(f)
			src, err := os.ReadFile(f) //nolint:gosec // f is a trusted path enumerated by go/packages
			if err != nil {
				return changed, unchanged, fmt.Errorf("read %s: %w", f, err)
			}
			out, err := instrumentSource(rel, src)
			if err != nil {
				return changed, unchanged, fmt.Errorf("instrument %s: %w", f, err)
			}
			if string(out) == string(src) {
				unchanged++
				continue
			}
			if err := os.WriteFile(f, out, info.Mode().Perm()); err != nil { //nolint:gosec // f is a trusted path enumerated by go/packages
				return changed, unchanged, fmt.Errorf("write %s: %w", f, err)
			}
			changed++
			fmt.Println("instrumented:", rel)
		}
	}
	return changed, unchanged, nil
}

func allPackagesEmpty(pkgs []*packages.Package) bool {
	for _, p := range pkgs {
		if len(p.GoFiles) > 0 {
			return false
		}
	}
	return true
}

// repoRelativePath returns a backend-go/...-rooted path for /*line*/ directives.
func repoRelativePath(abs string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return abs
	}
	rel, err := filepath.Rel(cwd, abs)
	if err != nil {
		return abs
	}
	// Express relative to the repo root (backend-go/...).
	return filepath.Join("backend-go", rel)
}
