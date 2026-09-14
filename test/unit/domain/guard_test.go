// Package domain mirrors the in-package domain suites with cross-cutting
// proofs: a hermetic external-import guard, a full-slice composition test,
// and allocation exactness properties.
package domain_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func collectDomainFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
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
		if strings.Contains(p, ".") && !strings.HasPrefix(p, "github.com/kadekutama/go-template/internal/domain/") {
			t.Errorf("%s imports external module %s", path, p)
		}
	}
}

// TestDomainHasNoExternalImports parses every non-test Go file under
// internal/domain and fails on any import outside the standard library and
// the module domain tree. Hermetic: no toolchain execution.
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
