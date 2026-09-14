package jsonparser

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// forbiddenCodecs are concrete JSON backends no package outside this one may
// import (E01-T07 codec seam).
var forbiddenCodecs = []string{
	"encoding/json",
	"github.com/bytedance/sonic",
	"github.com/json-iterator/go",
}

func findForbiddenImportsInFile(fset *token.FileSet, root, path string) ([]string, error) {
	node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, imp := range node.Imports {
		rawPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			rawPath = strings.Trim(imp.Path.Value, `"`)
		}
		for _, forbidden := range forbiddenCodecs {
			if rawPath == forbidden {
				rel, _ := filepath.Rel(root, path)
				found = append(found, rel+" imports "+forbidden)
			}
		}
	}
	return found, nil
}

func checkDirectoryForOffenders(fset *token.FileSet, root, subDir, jsonparserDir string) ([]string, error) {
	var offenders []string
	err := filepath.WalkDir(subDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == jsonparserDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found, parseErr := findForbiddenImportsInFile(fset, root, path)
		if parseErr != nil {
			return parseErr
		}
		offenders = append(offenders, found...)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return offenders, nil
}

// TestCodecSeam fails with the offending importers when the seam is breached.
func TestCodecSeam(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// This file lives at <root>/pkg/jsonparser/seam_test.go; root is 2 levels up.
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	jsonparserDir := filepath.Dir(thisFile)

	fset := token.NewFileSet()
	var offenders []string

	for _, sub := range []string{"cmd", "internal", "pkg", "api"} {
		subDir := filepath.Join(root, sub)
		found, err := checkDirectoryForOffenders(fset, root, subDir, jsonparserDir)
		if err != nil {
			t.Fatalf("walk %s: %v", sub, err)
		}
		offenders = append(offenders, found...)
	}

	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("codec seam breached (import pkg/jsonparser instead):\n%s", strings.Join(offenders, "\n"))
	}
}
