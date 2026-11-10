package fixer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutoFixerCreateAndMerge(t *testing.T) {
	tmpDir := t.TempDir()
	fixer := NewAutoFixer(tmpDir)

	snippet1 := `
services:
  db:
    ports:
      - "5433:5432"
`
	msg, err := fixer.ApplyOverride(snippet1)
	if err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if !strings.Contains(msg, "Created") {
		t.Errorf("expected file creation message, got %s", msg)
	}

	overrideFile := filepath.Join(tmpDir, "docker-compose.override.yml")
	content, err := os.ReadFile(overrideFile)
	if err != nil {
		t.Fatalf("failed to read override file: %v", err)
	}
	if !strings.Contains(string(content), "5433:5432") {
		t.Errorf("expected 5433:5432 in created override, got %s", string(content))
	}

	// Apply second override snippet
	snippet2 := `
services:
  redis:
    user: "1000:1000"
`
	msg2, err := fixer.ApplyOverride(snippet2)
	if err != nil {
		t.Fatalf("second apply failed: %v", err)
	}
	if !strings.Contains(msg2, "Updated") {
		t.Errorf("expected update message, got %s", msg2)
	}

	// Check backup file existence
	backupFile := overrideFile + ".bak"
	if _, err := os.Stat(backupFile); err != nil {
		t.Errorf("expected backup file %s to exist", backupFile)
	}

	// Check merged content
	mergedContent, _ := os.ReadFile(overrideFile)
	mergedStr := string(mergedContent)
	if !strings.Contains(mergedStr, "5433:5432") || !strings.Contains(mergedStr, "1000:1000") {
		t.Errorf("expected both db and redis in merged override file, got:\n%s", mergedStr)
	}
}
