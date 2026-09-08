package validator

import (
	"os"
	"os/exec"
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

func TestGitCommitContainsReadsReachableHistory(t *testing.T) {
	workspace := t.TempDir()
	notePath := "recovery note.txt"
	runGitTestCommand(t, workspace, "init", "-b", "main")
	runGitTestCommand(t, workspace, "config", "user.name", "CLI Quest Test")
	runGitTestCommand(t, workspace, "config", "user.email", "cliquest-test@example.invalid")
	if err := os.WriteFile(filepath.Join(workspace, notePath), []byte("CLIQUEST_RECOVERED_TEST\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitTestCommand(t, workspace, "add", notePath)
	runGitTestCommand(t, workspace, "commit", "-m", "add recovery note")
	if err := os.Remove(filepath.Join(workspace, notePath)); err != nil {
		t.Fatal(err)
	}
	runGitTestCommand(t, workspace, "add", "-A")
	runGitTestCommand(t, workspace, "commit", "-m", "remove recovery note")

	def := scene.Definition{
		ID:         "git-recovery",
		Success:    "recovered",
		Validation: scene.Validation{Type: "git_commit_contains", Path: notePath, Value: "CLIQUEST_RECOVERED_TEST"},
	}
	result := Check(def, &progress.ActiveScene{SceneID: def.ID, Workspace: workspace})
	if !result.Clear {
		t.Fatalf("reachable history should clear: %#v", result)
	}
}

func runGitTestCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
