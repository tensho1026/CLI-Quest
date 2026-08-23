package workspace

import (
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
