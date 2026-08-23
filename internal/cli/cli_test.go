package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensho1026/CLI-Quest/internal/progress"
)

func newTestApp(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	home := filepath.Join(t.TempDir(), "cliquest-home")
	t.Setenv("CLIQUEST_HOME", home)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app, err := New(out, errOut)
	if err != nil {
		t.Fatal(err)
	}
	return app, out, errOut, home
}

func TestLinuxPermissionFlow(t *testing.T) {
	app, out, errOut, home := newTestApp(t)
	if code := app.Run([]string{"start", "linux-permission"}); code != 0 {
		t.Fatalf("start returned %d: %s", code, errOut.String())
	}
	path := filepath.Join(home, "workspaces", "linux-permission", "deploy.sh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 != 0 {
		t.Fatal("deploy.sh unexpectedly starts executable")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := app.Run([]string{"check"}); code != 0 {
		t.Fatalf("check returned %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "✓ CLEAR") {
		t.Fatalf("clear output missing: %s", out.String())
	}
	store, _ := progress.New()
	data, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if data.Active != nil || !progress.IsCompleted(data, "linux-permission") {
		t.Fatalf("unexpected progress after clear: %#v", data)
	}
}

func TestFailedCheckAndHintPersistence(t *testing.T) {
	app, out, errOut, _ := newTestApp(t)
	if code := app.Run([]string{"start", "linux-permission"}); code != 0 {
		t.Fatalf("start returned %d: %s", code, errOut.String())
	}
	out.Reset()
	if code := app.Run([]string{"check"}); code != 0 {
		t.Fatalf("check returned %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Not cleared yet") {
		t.Fatalf("failure output missing: %s", out.String())
	}
	out.Reset()
	if code := app.Run([]string{"hint"}); code != 0 {
		t.Fatalf("hint returned %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Hint 1 / 3") {
		t.Fatalf("hint output missing: %s", out.String())
	}
	store, _ := progress.New()
	data, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if data.Active == nil || data.Active.HintIndex != 1 {
		t.Fatalf("hint index was not persisted: %#v", data.Active)
	}
}

func TestResetRestoresInitialFilesystem(t *testing.T) {
	app, _, errOut, home := newTestApp(t)
	if code := app.Run([]string{"start", "linux-permission"}); code != 0 {
		t.Fatalf("start returned %d: %s", code, errOut.String())
	}
	path := filepath.Join(home, "workspaces", "linux-permission", "deploy.sh")
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if code := app.Run([]string{"hint"}); code != 0 {
		t.Fatalf("hint returned %d", code)
	}
	if code := app.Run([]string{"reset"}); code != 0 {
		t.Fatalf("reset returned %d: %s", code, errOut.String())
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 != 0 {
		t.Fatal("reset did not restore non-executable mode")
	}
	store, _ := progress.New()
	data, _ := store.Load()
	if data.Active == nil || data.Active.HintIndex != 0 {
		t.Fatalf("reset did not reset hint index: %#v", data.Active)
	}
}

func TestListFiltersAndJSON(t *testing.T) {
	app, out, errOut, _ := newTestApp(t)
	if code := app.Run([]string{"--json", "list", "--category", "git", "--difficulty", "hard"}); code != 0 {
		t.Fatalf("list returned %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"id": "git-bisect"`) || strings.Contains(out.String(), `"id": "linux-permission"`) {
		t.Fatalf("unexpected filtered JSON: %s", out.String())
	}
}

func TestInteractiveDifficultyAndCategorySelection(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("CLIQUEST_HOME", home)
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	// Categories are sorted: Git, HTTP, Incident, Linux. Linux easy has one scene.
	app, err := NewWithInput(strings.NewReader("4\n1\n"), out, errOut)
	if err != nil {
		t.Fatal(err)
	}
	if code := app.Run([]string{"start"}); code != 0 {
		t.Fatalf("start returned %d: %s", code, errOut.String())
	}
	store, _ := progress.New()
	data, _ := store.Load()
	if data.Active == nil || data.Active.SceneID != "linux-permission" {
		t.Fatalf("unexpected selected scene: %#v\n%s", data.Active, out.String())
	}
}

func TestCancelRemovesWorkspaceAndRecordsHistory(t *testing.T) {
	app, _, errOut, home := newTestApp(t)
	if code := app.Run([]string{"start", "linux-permission"}); code != 0 {
		t.Fatalf("start: %s", errOut.String())
	}
	workspacePath := filepath.Join(home, "workspaces", "linux-permission")
	if code := app.Run([]string{"cancel"}); code != 0 {
		t.Fatalf("cancel: %s", errOut.String())
	}
	if _, err := os.Stat(workspacePath); !os.IsNotExist(err) {
		t.Fatalf("workspace still exists: %v", err)
	}
	store, _ := progress.New()
	data, _ := store.Load()
	if data.Active != nil || len(data.History) != 1 || data.History[0].Result != "cancelled" {
		t.Fatalf("unexpected progress: %#v", data)
	}
}

func TestLanguageConfigurationPersists(t *testing.T) {
	app, out, errOut, _ := newTestApp(t)
	if code := app.Run([]string{"config", "language", "ja"}); code != 0 {
		t.Fatalf("config: %s", errOut.String())
	}
	out.Reset()
	if code := app.Run([]string{"start", "linux-permission"}); code != 0 {
		t.Fatalf("start: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "deploy.shの内容を変えず") {
		t.Fatalf("Japanese scene text missing: %s", out.String())
	}
}
