package validator

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tensho1026/CLI-Quest/internal/progress"
	"github.com/tensho1026/CLI-Quest/internal/scene"
)

func TestCompositeAllValidator(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "result.txt"), []byte("ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	def := scene.Definition{ID: "composite", Success: "done", Validation: scene.Validation{Type: "all", Validators: []scene.Validation{
		{Type: "file_exists", Path: "result.txt"}, {Type: "file_contains", Path: "result.txt", Value: "ready"}, {Type: "file_mode", Path: "result.txt", Value: "0600"},
	}}}
	result := Check(def, &progress.ActiveScene{SceneID: "composite", Workspace: workspace})
	if !result.Clear {
		t.Fatalf("composite should clear: %s", result.Message)
	}
	if err := os.WriteFile(filepath.Join(workspace, "result.txt"), []byte("wrong\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result = Check(def, &progress.ActiveScene{SceneID: "composite", Workspace: workspace})
	if result.Clear {
		t.Fatal("composite cleared with a failing nested validator")
	}
}

func TestValidatorRejectsEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires Unix permissions")
	}
	workspace := t.TempDir()
	external := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(external, []byte("expected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(workspace, "answer.txt")); err != nil {
		t.Fatal(err)
	}
	def := scene.Definition{ID: "safe", Validation: scene.Validation{Type: "file_contains", Path: "answer.txt", Value: "expected"}}
	if result := Check(def, &progress.ActiveScene{SceneID: "safe", Workspace: workspace}); result.Clear {
		t.Fatal("validator followed a symlink outside the workspace")
	}
}

func TestGitValidatorRejectsSymlinkedWorkspaceRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires Unix permissions")
	}
	realWorkspace := t.TempDir()
	link := filepath.Join(t.TempDir(), "workspace")
	if err := os.Symlink(realWorkspace, link); err != nil {
		t.Fatal(err)
	}
	def := scene.Definition{ID: "git-safe", Validation: scene.Validation{Type: "git_clean"}}
	result := Check(def, &progress.ActiveScene{SceneID: "git-safe", Workspace: link})
	if result.Clear || !strings.Contains(result.Message, "unsafe") {
		t.Fatalf("unexpected result: %#v", result)
	}
}
