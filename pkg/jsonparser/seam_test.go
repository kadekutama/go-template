package jsonparser

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// forbiddenCodecs are concrete JSON backends no package outside this one may
// import (E01-T07 codec seam). The check runs `go list` over the module, so it
// tracks refactors automatically; adding a legitimate exception requires
// editing allowedOffenders with a recorded reason (none today).
var forbiddenCodecs = []string{
	"encoding/json",
	"github.com/bytedance/sonic",
	"github.com/json-iterator/go",
}

// TestCodecSeam fails with the offending importers when the seam is breached.
func TestCodecSeam(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// This file lives at <root>/pkg/jsonparser; list from <root>.
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	cmd := exec.CommandContext(t.Context(), "go", "list", "-f", `{{.ImportPath}}|{{join .Imports " "}}`, "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var offenders []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		importer, imports := parts[0], " "+parts[1]+" "
		if strings.HasPrefix(importer, "example.com/go-template/pkg/jsonparser") {
			continue
		}
		for _, codec := range forbiddenCodecs {
			if strings.Contains(imports, " "+codec+" ") {
				offenders = append(offenders, importer+" imports "+codec)
			}
		}
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("codec seam breached (import pkg/jsonparser instead):\n%s", strings.Join(offenders, "\n"))
	}
}
