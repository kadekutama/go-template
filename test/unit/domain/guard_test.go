// Package domain mirrors the in-package domain suites with cross-cutting
// proofs: hermetic architectural guards (external-import boundaries and the
// ADR-026 banned `samber/lo` helper surface), a full-slice composition test,
// and allocation exactness properties.
package domain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// loRootImport is the only approved `samber/lo` import path (ADR-026).
const loRootImport = "github.com/samber/lo"

// collectGoFiles walks every root and returns all Go files, skipping testdata
// trees (which may hold deliberate lint fixtures).
func collectGoFiles(t *testing.T, roots ...string) []string {
	t.Helper()

	var files []string

	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				if info.Name() == "testdata" {
					return filepath.SkipDir
				}

				return nil
			}

			if strings.HasSuffix(path, ".go") {
				files = append(files, path)
			}

			return nil
		})
		if err != nil {
			t.Fatalf("Walk(%s): %v", root, err)
		}
	}

	if len(files) == 0 {
		t.Fatal("no Go files found")
	}

	return files
}

func collectDomainFiles(t *testing.T, root string) []string {
	t.Helper()

	var files []string

	for _, path := range collectGoFiles(t, root) {
		if !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
	}

	if len(files) == 0 {
		t.Fatal("no domain files found")
	}

	return files
}

func verifyDomainImports(t *testing.T, fset *token.FileSet, path string) {
	t.Helper()

	parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}

	for _, imp := range parsed.Imports {
		p := strings.Trim(imp.Path.Value, `"`)

		if !strings.Contains(p, ".") {
			continue // Standard library.
		}

		if strings.HasPrefix(p, "github.com/kadekutama/go-template/internal/domain/") {
			continue // Module-internal domain package.
		}

		// ADR-026: only the pure root combinators; lo/mutable and lo/parallel stay forbidden.
		if p == loRootImport {
			continue
		}

		t.Errorf("%s imports external module %s", path, p)
	}
}

// isBannedLoHelper mirrors the ADR-026 Decision 2 ban list enforced by
// `.golangci.yml` `forbidigo`; keep both in sync when the ban list changes.
func isBannedLoHelper(name string) bool {
	switch name {
	case "Attempt", "AttemptWithDelay", "AttemptWhile", "AttemptWhileWithDelay",
		"Async", "Async0", "Async1", "Async2", "Async3", "Async4", "Async5", "Async6",
		"NewDebounce", "NewDebounceBy", "Synchronize", "WaitFor", "WaitForWithContext",
		"Transaction", "NewTransaction", "ChannelDispatcher", "SliceToChannel", "ChannelToSlice",
		"Generator", "Buffer", "BufferWithContext", "BufferWithTimeout", "FanIn", "FanOut",
		"Must", "Must0", "Must1", "Must2", "Must3", "Must4", "Must5", "Must6":
		return true
	default:
		return false
	}
}

// verifyLoUsage parses one Go file and fails on forbidden `samber/lo` usage.
// Resolving the import alias keeps aliased calls (`mylo.Attempt`) and dot
// imports visible, independently of `golangci-lint`.
func verifyLoUsage(t *testing.T, fset *token.FileSet, path string) {
	t.Helper()

	parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}

	alias := ""

	for _, imp := range parsed.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		if strings.HasPrefix(importPath, loRootImport+"/") {
			t.Errorf("%s imports forbidden lo subpackage %s (ADR-026: only the pure root combinators are approved)", path, importPath)

			continue
		}

		if importPath != loRootImport {
			continue
		}

		switch {
		case imp.Name == nil:
			alias = "lo"
		case imp.Name.Name == "_":
			// Blank import anchor (internal/shared/deps/deps.go); allowed.
		case imp.Name.Name == ".":
			t.Errorf("%s dot-imports %s (ADR-026); dot imports hide helper usage from qualified checks", path, loRootImport)
		default:
			alias = imp.Name.Name
		}
	}

	if alias == "" {
		return
	}

	ast.Inspect(parsed, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name != alias {
			return true
		}

		if isBannedLoHelper(selector.Sel.Name) {
			t.Errorf("%s uses banned lo helper %s.%s (ADR-026)", path, alias, selector.Sel.Name)
		}

		return true
	})
}

// TestImplementationTreeAvoidsBannedLoHelpers proves the ADR-026 ban list
// mechanically and independently of `golangci-lint` (whose `issues.uniq-by-line`
// default can hide a finding on a line that already reports another issue): it
// walks the implementation and test trees, resolves the `github.com/samber/lo`
// import alias, and fails on any banned helper usage or subpackage import
// (including `lo/it`). Hermetic: parses source only.
func TestImplementationTreeAvoidsBannedLoHelpers(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	files := collectGoFiles(
		t,
		filepath.Join(root, "internal"),
		filepath.Join(root, "pkg"),
		filepath.Join(root, "cmd"),
		filepath.Join(root, "test"),
	)

	fset := token.NewFileSet()
	for _, path := range files {
		verifyLoUsage(t, fset, path)
	}
}

// TestDomainHasNoExternalImports parses every non-test Go file under
// internal/domain and fails on any import outside the standard library, the
// module domain tree, and the ADR-026 pure root collection combinators.
// Hermetic: no toolchain execution.
func TestDomainHasNoExternalImports(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "internal", "domain")
	files := collectDomainFiles(t, root)

	fset := token.NewFileSet()
	for _, path := range files {
		verifyDomainImports(t, fset, path)
	}
}
