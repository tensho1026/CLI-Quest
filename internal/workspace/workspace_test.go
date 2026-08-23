package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoinRejectsPathsOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	unsafe := []string{"", "../secret", filepath.Join("nested", "..", "..", "secret")}
	if filepath.IsAbs(root) {
		unsafe = append(unsafe, root)
	}
	for _, path := range unsafe {
		if _, err := safeJoin(root, path); err == nil {
			t.Errorf("safeJoin accepted unsafe path %q", path)
		}
	}
}

func TestCleanupStaleRemovesDeadResourceRecord(t *testing.T) {
	manager := &Manager{root: filepath.Join(t.TempDir(), "workspaces")}
	workspacePath := filepath.Join(manager.root, "old-scene")
	if err := os.MkdirAll(workspacePath, 0o700); err != nil {
		t.Fatal(err)
	}
	resources := map[string]string{"pid": "99999999", "token": "dead-token", "workspace": workspacePath}
	data, _ := json.Marshal(resources)
	registry := filepath.Join(workspacePath, ".cliquest-resource.json")
	if err := os.WriteFile(registry, data, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := manager.CleanupStale("")
	if err != nil {
		t.Fatal(err)
	}
	if report.Stale != 1 {
		t.Fatalf("got report %#v", report)
	}
	if _, err := os.Stat(registry); !os.IsNotExist(err) {
		t.Fatalf("stale registry remains: %v", err)
	}
}

func TestSafeJoinAcceptsNestedRelativePath(t *testing.T) {
	root := t.TempDir()
	got, err := safeJoin(root, "logs/api.log")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "logs", "api.log")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
