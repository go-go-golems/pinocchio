package chatapp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSchemaRegistrationsAvoidGenericStructPayloads(t *testing.T) {
	root := moduleRoot(t)

	var violations []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "pkg" && strings.Contains(filepath.ToSlash(path), "/cmd/web-chat/web/") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".pb.go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		if !strings.Contains(text, "&structpb.Struct{}") {
			return nil
		}
		if !strings.Contains(text, "RegisterEvent(") && !strings.Contains(text, "RegisterUIEvent(") && !strings.Contains(text, "RegisterTimelineEntity(") {
			return nil
		}
		violations = append(violations, filepath.ToSlash(path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(violations) > 0 {
		t.Fatalf("sessionstream chatapp event/UI/timeline payloads must use concrete protobuf messages, not google.protobuf.Struct; violations: %s", strings.Join(violations, ", "))
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find module root from %s", file)
		}
		dir = parent
	}
}
