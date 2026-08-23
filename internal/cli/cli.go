package cli

import (
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/tensho1026/CLI-Quest/internal/progress"
	"github.com/tensho1026/CLI-Quest/internal/scene"
	"github.com/tensho1026/CLI-Quest/internal/validator"
	"github.com/tensho1026/CLI-Quest/internal/workspace"
)

const rule = "────────────────────────────────"

type App struct {
	out        io.Writer
	errOut     io.Writer
	catalog    *scene.Catalog
	store      *progress.Store
	workspaces *workspace.Manager
}

func New(out, errOut io.Writer) (*App, error) {
	catalog, err := scene.Load()
	if err != nil {
		return nil, err
	}
	store, err := progress.New()
	if err != nil {
		return nil, err
	}
	workspaces, err := workspace.New(store.Root())
	if err != nil {
		return nil, err
	}
	return &App{out: out, errOut: errOut, catalog: catalog, store: store, workspaces: workspaces}, nil
}

func (a *App) Run(args []string) int {
	if len(args) == 0 {
		return a.home()
	}
	var err error
	switch args[0] {
	case "list":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest list")
		} else {
			err = a.list()
		}
	case "start":
		if len(args) != 2 {
			err = fmt.Errorf("usage: cliquest start <scene-id>")
		} else {
			err = a.start(args[1])
		}
	case "check":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest check")
		} else {
			err = a.check()
		}
	case "hint":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest hint")
		} else {
			err = a.hint()
		}
	case "reset":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest reset")
		} else {
			err = a.reset()
		}
	case "status":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest status")
		} else {
			err = a.status()
		}
	case "doctor":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest doctor")
		} else {
			err = a.doctor()
		}
	case "help", "--help", "-h":
		a.help()
		return 0
	default:
		err = fmt.Errorf("unknown command %q\n\nRun 'cliquest help' to see available commands", args[0])
	}
	if err != nil {
		fmt.Fprintf(a.errOut, "Error: %v\n", err)
		return 1
	}
	return 0
}

func (a *App) home() int {
	data, err := a.store.Load()
	if err != nil {
		fmt.Fprintf(a.errOut, "Error: %v\n", err)
		return 1
	}
	all := a.catalog.All()
	fmt.Fprintf(a.out, "CLI QUEST\n%s\n\nPractical terminal training for engineers\n\nProgress: %d / %d\n\nCategories\n\n", rule, len(data.Completed), len(all))
	type count struct{ total, completed int }
	counts := map[string]*count{}
	for _, def := range all {
		if counts[def.Category] == nil {
			counts[def.Category] = &count{}
		}
		counts[def.Category].total++
		if progress.IsCompleted(data, def.ID) {
			counts[def.Category].completed++
		}
	}
	for _, category := range []string{"Linux", "Git", "Process", "HTTP"} {
		key := strings.ToLower(category)
		if c := counts[key]; c != nil {
			fmt.Fprintf(a.out, "  %-12s %d / %d\n", category, c.completed, c.total)
		}
	}
	fmt.Fprintln(a.out, "\nCommands\n\n  cliquest list\n  cliquest start <scene-id>\n  cliquest status\n  cliquest doctor\n\n"+rule)
	return 0
}

func (a *App) help() {
	fmt.Fprintln(a.out, `CLI Quest - practical terminal training

Usage:
  cliquest                       Show the home screen
  cliquest list                  List all scenes
  cliquest start <scene-id>      Create and start a scene
  cliquest check                 Validate the current workspace state
  cliquest hint                  Reveal the next hint
  cliquest reset                 Rebuild the active scene from scratch
  cliquest status                Show progress and the active scene
  cliquest doctor                Check optional command dependencies
  cliquest help                  Show this help`)
}

func (a *App) list() error {
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, "ID                       CATEGORY    DIFFICULTY  STATUS")
	fmt.Fprintln(a.out, rule+"────────────────────")
	for _, def := range a.catalog.All() {
		status := "-"
		if progress.IsCompleted(data, def.ID) {
			status = "Clear"
		}
		if data.Active != nil && data.Active.SceneID == def.ID {
			status = "Active"
		}
		fmt.Fprintf(a.out, "%-24s %-11s %-11s %s\n", def.ID, title(def.Category), title(def.Difficulty), status)
	}
	return nil
}

func (a *App) start(id string) error {
	def, ok := a.catalog.Get(id)
	if !ok {
		return fmt.Errorf("scene %q was not found; run 'cliquest list'", id)
	}
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	if data.Active != nil {
		workspace.Cleanup(data.Active.Resources)
		if err := a.workspaces.Remove(data.Active.SceneID); err != nil {
			return err
		}
		data.Active = nil
		if err := a.store.Save(data); err != nil {
			return err
		}
	}
	resources, err := a.workspaces.Setup(def)
	if err != nil {
		return fmt.Errorf("prepare scene: %w", err)
	}
	active := &progress.ActiveScene{
		SceneID: def.ID, Workspace: a.workspaces.Path(def.ID), StartedAt: time.Now(), Resources: resources,
	}
	data.Active = active
	if err := a.store.Save(data); err != nil {
		workspace.Cleanup(resources)
		return err
	}
	fmt.Fprintf(a.out, "CLI QUEST\n%s\n\nScene: %s\nCategory: %s\nDifficulty: %s\n\n%s\n\nMission:\n\n%s\n\n%s\n\nWorkspace created.\n\nPath:\n%s\n\nEnter the workspace and solve the problem.\n\n  cd %s\n\nWhen finished:\n\n  cliquest check\n\nFor a hint or a fresh start:\n\n  cliquest hint\n  cliquest reset\n", rule, def.Title, title(def.Category), title(def.Difficulty), def.Description, def.Mission, rule, active.Workspace, shellDisplay(active.Workspace))
	return nil
}

func (a *App) check() error {
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	if data.Active == nil {
		return fmt.Errorf("no active scene; run 'cliquest start <scene-id>'")
	}
	def, ok := a.catalog.Get(data.Active.SceneID)
	if !ok {
		return fmt.Errorf("active scene definition %q is missing", data.Active.SceneID)
	}
	fmt.Fprintln(a.out, "Checking scene...")
	fmt.Fprintln(a.out)
	result := validator.Check(def, data.Active)
	if !result.Clear {
		fmt.Fprintf(a.out, "Not cleared yet.\n\n%s\n\nNeed a hint? Run:\n\n  cliquest hint\n", result.Message)
		return nil
	}
	workspace.Cleanup(data.Active.Resources)
	newlyCompleted := progress.MarkCompleted(&data, def.ID)
	data.Active = nil
	if err := a.store.Save(data); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "✓ CLEAR\n\n%s\n\n%s\n", def.Title, result.Message)
	if newlyCompleted {
		fmt.Fprintln(a.out, "\n+100 XP")
	}
	return nil
}

func (a *App) hint() error {
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	if data.Active == nil {
		return fmt.Errorf("no active scene")
	}
	def, ok := a.catalog.Get(data.Active.SceneID)
	if !ok {
		return fmt.Errorf("active scene definition %q is missing", data.Active.SceneID)
	}
	if len(def.Hints) == 0 {
		return fmt.Errorf("this scene has no hints")
	}
	index := data.Active.HintIndex
	if index >= len(def.Hints) {
		index = len(def.Hints) - 1
	}
	fmt.Fprintf(a.out, "Hint %d / %d\n\n%s\n", index+1, len(def.Hints), def.Hints[index])
	if data.Active.HintIndex < len(def.Hints) {
		data.Active.HintIndex++
	}
	return a.store.Save(data)
}

func (a *App) reset() error {
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	if data.Active == nil {
		return fmt.Errorf("no active scene")
	}
	def, ok := a.catalog.Get(data.Active.SceneID)
	if !ok {
		return fmt.Errorf("active scene definition %q is missing", data.Active.SceneID)
	}
	workspace.Cleanup(data.Active.Resources)
	resources, err := a.workspaces.Setup(def)
	if err != nil {
		return fmt.Errorf("reset scene: %w", err)
	}
	data.Active = &progress.ActiveScene{SceneID: def.ID, Workspace: a.workspaces.Path(def.ID), StartedAt: time.Now(), Resources: resources}
	if err := a.store.Save(data); err != nil {
		workspace.Cleanup(resources)
		return err
	}
	fmt.Fprintf(a.out, "Scene reset.\n\n%s has been restored to its initial state.\n\nWorkspace:\n%s\n", def.Title, data.Active.Workspace)
	return nil
}

func (a *App) status() error {
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "CLI QUEST STATUS\n%s\n\nProgress: %d / %d scenes cleared\n", rule, len(data.Completed), len(a.catalog.All()))
	if data.Active == nil {
		fmt.Fprintln(a.out, "\nActive scene: none\n\nRun 'cliquest list' to choose a scene.")
	} else {
		def, _ := a.catalog.Get(data.Active.SceneID)
		fmt.Fprintf(a.out, "\nActive scene: %s (%s)\nStarted: %s\nHints used: %d / %d\nWorkspace: %s\n", def.Title, def.ID, data.Active.StartedAt.Local().Format("2006-01-02 15:04"), data.Active.HintIndex, len(def.Hints), data.Active.Workspace)
	}
	if len(data.Completed) > 0 {
		completed := append([]string(nil), data.Completed...)
		sort.Strings(completed)
		fmt.Fprintln(a.out, "\nCleared:")
		for _, id := range completed {
			fmt.Fprintf(a.out, "  ✓ %s\n", id)
		}
	}
	return nil
}

func (a *App) doctor() error {
	fmt.Fprintln(a.out, "Environment")
	fmt.Fprintln(a.out)
	for _, command := range []string{"git", "curl", "jq", "docker"} {
		mark := "✗"
		if _, err := exec.LookPath(command); err == nil {
			mark = "✓"
		}
		fmt.Fprintf(a.out, "%-10s %s\n", title(command), mark)
	}
	fmt.Fprintln(a.out, "\nGit is required for Git scenes. curl is recommended for HTTP scenes.\njq and Docker are optional in this MVP.")
	return nil
}

func title(value string) string {
	if value == "" {
		return value
	}
	if strings.EqualFold(value, "http") {
		return "HTTP"
	}
	return strings.ToUpper(value[:1]) + strings.ToLower(value[1:])
}

func shellDisplay(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}
