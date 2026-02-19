package thinkingmode

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}

func walkFiles(root string, exts []string, fn func(path string, rel string)) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "dist" || base == "static" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !slices.Contains(exts, ext) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fn(path, filepath.ToSlash(rel))
		return nil
	})
}

func fileContainsAny(path string, markers []string) (bool, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "", err
	}
	text := string(data)
	for _, m := range markers {
		if strings.Contains(text, m) {
			return true, m, nil
		}
	}
	return false, "", nil
}

func TestThinkingModeBackendProjectionIsIsolatedToCmdWebChatModule(t *testing.T) {
	root := repoRoot(t)
	markers := []string{
		"thinking.mode.started",
		"thinking.mode.update",
		"thinking.mode.completed",
		"EventThinkingModeStarted",
		"EventThinkingModeUpdate",
		"EventThinkingModeCompleted",
	}

	var violations []string
	targets := []string{
		filepath.Join(root, "pkg", "webchat"),
		filepath.Join(root, "cmd", "web-chat"),
	}
	for _, target := range targets {
		err := walkFiles(target, []string{".go"}, func(path string, rel string) {
			normalized := filepath.ToSlash(path)
			if strings.Contains(normalized, "/cmd/web-chat/thinkingmode/") {
				return
			}
			found, marker, err := fileContainsAny(path, markers)
			if err != nil {
				violations = append(violations, rel+": read error: "+err.Error())
				return
			}
			if found {
				violations = append(violations, rel+": contains marker "+marker)
			}
		})
		if err != nil {
			t.Fatalf("walk failed for %s: %v", target, err)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("thinking-mode backend markers leaked outside cmd/web-chat/thinkingmode:\n%s", strings.Join(violations, "\n"))
	}
}

func TestThinkingModeFrontendRegistrationIsIsolatedToFeatureModule(t *testing.T) {
	root := repoRoot(t)
	webRoot := filepath.Join(root, "cmd", "web-chat", "web", "src")
	markers := []string{
		"registerSem('thinking.mode.started'",
		"registerSem('thinking.mode.update'",
		"registerSem('thinking.mode.completed'",
		"registerTimelineRenderer('thinking_mode'",
		"registerTimelinePropsNormalizer('thinking_mode'",
	}

	var violations []string
	err := walkFiles(webRoot, []string{".ts", ".tsx"}, func(path string, rel string) {
		normalized := filepath.ToSlash(path)
		if strings.Contains(normalized, "/features/thinkingMode/") {
			return
		}
		found, marker, err := fileContainsAny(path, markers)
		if err != nil {
			violations = append(violations, rel+": read error: "+err.Error())
			return
		}
		if found {
			violations = append(violations, rel+": contains marker "+marker)
		}
	})
	if err != nil {
		t.Fatalf("walk failed: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("thinking-mode frontend registrations leaked outside feature module:\n%s", strings.Join(violations, "\n"))
	}
}
