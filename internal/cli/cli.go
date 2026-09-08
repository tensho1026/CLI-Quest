package cli

import (
	"fmt"
	"io"
	"os"
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

var Version = "v0.2.0-dev"

type App struct {
	in         io.Reader
	out        io.Writer
	errOut     io.Writer
	catalog    *scene.Catalog
	store      *progress.Store
	workspaces *workspace.Manager
	jsonOutput bool
	language   string
	color      bool
}

func New(out, errOut io.Writer) (*App, error) {
	return NewWithInput(os.Stdin, out, errOut)
}

func NewWithInput(in io.Reader, out, errOut io.Writer) (*App, error) {
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
	app := &App{in: in, out: out, errOut: errOut, catalog: catalog, store: store, workspaces: workspaces, language: "en"}
	app.color = supportsColor(out)
	return app, nil
}

func (a *App) Run(args []string) int {
	var err error
	args, err = a.applyGlobalOptions(args)
	if err != nil {
		fmt.Fprintf(a.errOut, "Error: %v\n", err)
		return 1
	}
	if len(args) == 0 {
		return a.home()
	}
	switch args[0] {
	case "list":
		err = a.list(args[1:])
	case "start":
		err = a.startCommand(args[1:])
	case "next":
		err = a.next(args[1:])
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
	case "cancel":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest cancel")
		} else {
			err = a.cancel()
		}
	case "cleanup":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest cleanup")
		} else {
			err = a.cleanup()
		}
	case "open":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest open")
		} else {
			err = a.openWorkspace()
		}
	case "solution":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest solution")
		} else {
			err = a.solution()
		}
	case "history":
		err = a.history(args[1:])
	case "stats":
		if len(args) != 1 {
			err = fmt.Errorf("usage: cliquest stats")
		} else {
			err = a.stats()
		}
	case "completion":
		if len(args) != 2 {
			err = fmt.Errorf("usage: cliquest completion <bash|zsh|fish>")
		} else {
			err = a.completion(args[1])
		}
	case "config":
		err = a.config(args[1:])
	case "scene":
		err = a.sceneCommand(args[1:])
	case "version", "--version", "-v":
		if a.jsonOutput {
			_ = a.writeJSON(map[string]string{"version": Version})
		} else {
			fmt.Fprintf(a.out, "cliquest %s\n", Version)
		}
		return 0
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
	data, err := a.store.LoadState()
	if err != nil {
		fmt.Fprintf(a.errOut, "Error: %v\n", err)
		return 1
	}
	streak, err := a.store.Streak(time.Now())
	if err != nil {
		fmt.Fprintf(a.errOut, "Error: %v\n", err)
		return 1
	}
	all := a.catalog.All()
	if a.jsonOutput {
		active := ""
		if data.Active != nil {
			active = data.Active.SceneID
		}
		if err := a.writeJSON(map[string]any{"completed": len(data.Completed), "total": len(all), "total_xp": data.TotalXP, "streak_days": streak, "active_scene": active}); err != nil {
			fmt.Fprintf(a.errOut, "Error: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(a.out, "CLI QUEST\n%s\n\n%s\n\n%s: %d / %d\n\n%s\n\n", rule, a.text("Practical terminal training for engineers", "エンジニア向け実践terminal training"), a.text("Progress", "進捗"), len(data.Completed), len(all), a.text("Categories", "分野"))
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
	for _, category := range []string{"Linux", "Git", "Process", "HTTP", "Shell", "Network", "Incident"} {
		key := strings.ToLower(category)
		if c := counts[key]; c != nil {
			fmt.Fprintf(a.out, "  %-12s %d / %d\n", category, c.completed, c.total)
		}
	}
	fmt.Fprintf(a.out, "\nXP: %d    %s: %d %s\n", data.TotalXP, a.text("Streak", "連続学習"), streak, a.text("day(s)", "日"))
	fmt.Fprintf(a.out, "\n%s\n\n  cliquest list\n  cliquest start\n  cliquest next\n  cliquest status\n\n%s\n", a.text("Commands", "コマンド"), rule)
	return 0
}

func (a *App) help() {
	if a.language == "ja" {
		fmt.Fprintln(a.out, `CLI Quest - 実践terminal training

使い方:
  cliquest                       Home画面
  cliquest list [filter]         Scene一覧と絞り込み
  cliquest start [scene-id]      Sceneを選択・開始
  cliquest next [filter]         次の未Clear Scene
  cliquest check                 現在状態を判定
  cliquest hint                  次のHint
  cliquest solution              解答例を表示
  cliquest reset                 Sceneを初期状態へ戻す
  cliquest cancel                Active Sceneを終了・削除
  cliquest cleanup               stale resourceを掃除
  cliquest open                  Workspaceのcd command
  cliquest status                進捗とActive Scene
  cliquest history               挑戦履歴
  cliquest stats                 XPと分野・難易度別成績
  cliquest doctor                dependency確認
  cliquest completion <shell>    shell completion生成
  cliquest config language ja    表示言語の保存
  cliquest scene validate <file> Scene JSON検証

Filter:
  --category <name>       分野
  --difficulty <level>   easy / normal / hard
  --status <state>       clear / active / uncleared

共通option:
  --json                  JSON出力
  --lang <en|ja>          今回だけ言語を変更
  --no-color              ANSI color無効`)
		return
	}
	fmt.Fprintln(a.out, `CLI Quest - practical terminal training

Usage:
  cliquest                       Show the home screen
  cliquest list [filters]        List scenes by category/difficulty/status
  cliquest start [scene-id]      Select or start a scene
  cliquest next [filters]        Start the next uncleared scene
  cliquest check                 Validate the current workspace state
  cliquest hint                  Reveal the next hint
  cliquest solution              Reveal explicit solution examples
  cliquest reset                 Rebuild the active scene from scratch
  cliquest cancel                Stop and remove the active scene
  cliquest cleanup               Clean stale CLI Quest resources
  cliquest open                  Print the active workspace cd command
  cliquest status                Show progress and the active scene
  cliquest history               Show attempt history
  cliquest stats                 Show XP and category/difficulty statistics
  cliquest doctor                Check optional command dependencies
  cliquest completion <shell>    Generate shell completion
  cliquest config language ja    Persist language (en or ja)
  cliquest scene validate <file> Validate an external scene JSON file
  cliquest help                  Show this help`)
	fmt.Fprintln(a.out, `
Filters:
  --category <name>       linux, git, process, http, shell, network, incident
  --difficulty <level>   easy, normal, hard
  --status <state>       clear, active, uncleared

Global options:
  --json                  Machine-readable JSON output
  --lang <en|ja>          Override language for this command
  --no-color              Disable ANSI colors`)
}

func (a *App) list(args []string) error {
	selected, positional, err := parseFilters(args, true)
	if err != nil {
		return err
	}
	if len(positional) != 0 {
		return fmt.Errorf("usage: cliquest list [filters]")
	}
	data, err := a.store.LoadState()
	if err != nil {
		return err
	}
	type row struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Category   string `json:"category"`
		Difficulty string `json:"difficulty"`
		Status     string `json:"status"`
	}
	var rows []row
	for _, original := range a.catalog.All() {
		if !a.matches(original, data, selected) {
			continue
		}
		def := original.Localize(a.language)
		status := "uncleared"
		if progress.IsCompleted(data, def.ID) {
			status = "clear"
		}
		if data.Active != nil && data.Active.SceneID == def.ID {
			status = "active"
		}
		rows = append(rows, row{def.ID, def.Title, def.Category, def.Difficulty, status})
	}
	if a.jsonOutput {
		return a.writeJSON(rows)
	}
	fmt.Fprintln(a.out, a.text("ID                       CATEGORY    DIFFICULTY  STATUS", "ID                       分野        難易度      状態"))
	fmt.Fprintln(a.out, rule+"────────────────────")
	for _, item := range rows {
		fmt.Fprintf(a.out, "%-24s %-11s %-11s %s\n", item.ID, title(item.Category), title(item.Difficulty), title(item.Status))
	}
	return nil
}

func (a *App) startScene(id string) error {
	baseDef, ok := a.catalog.Get(id)
	if !ok {
		return fmt.Errorf("scene %q was not found; run 'cliquest list'", id)
	}
	started := time.Now()
	var active *progress.ActiveScene
	var operationErr error
	err := a.store.Update(func(data *progress.Data) error {
		if data.Active != nil {
			if old, found := a.catalog.Get(data.Active.SceneID); found {
				progress.Cancel(data, old.ID, old.Category, old.Difficulty, started)
			}
			workspace.Cleanup(data.Active.Resources)
			if removeErr := a.workspaces.Remove(data.Active.SceneID); removeErr != nil {
				return removeErr
			}
			data.Active = nil
		}
		resources, setupErr := a.workspaces.Setup(baseDef)
		if setupErr != nil {
			operationErr = fmt.Errorf("prepare scene: %w", setupErr)
			return nil
		}
		active = &progress.ActiveScene{SceneID: baseDef.ID, Workspace: a.workspaces.Path(baseDef.ID), StartedAt: started, Resources: resources}
		data.Active = active
		progress.RecordStart(data, baseDef.ID, started)
		return nil
	})
	if err != nil {
		return err
	}
	if operationErr != nil {
		return operationErr
	}
	def := baseDef.Localize(a.language)
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"scene": def.ID, "title": def.Title, "category": def.Category, "difficulty": def.Difficulty, "workspace": active.Workspace, "mission": def.Mission})
	}
	if a.language == "ja" {
		fmt.Fprintf(a.out, "CLI QUEST\n%s\n\nScene: %s\n分野: %s\n難易度: %s\n\n%s\n\nMission:\n\n%s\n\n%s\n\nWorkspaceを作成しました。\n\nPath:\n%s\n\nWorkspaceへ移動して解決してください。\n\n  cd %s\n\n完了後:\n\n  cliquest check\n\nHintまたはやり直し:\n\n  cliquest hint\n  cliquest reset\n", rule, def.Title, title(def.Category), title(def.Difficulty), def.Description, def.Mission, rule, active.Workspace, shellDisplay(active.Workspace))
		return nil
	}
	fmt.Fprintf(a.out, "CLI QUEST\n%s\n\nScene: %s\nCategory: %s\nDifficulty: %s\n\n%s\n\nMission:\n\n%s\n\n%s\n\nWorkspace created.\n\nPath:\n%s\n\nEnter the workspace and solve the problem.\n\n  cd %s\n\nWhen finished:\n\n  cliquest check\n\nFor a hint or a fresh start:\n\n  cliquest hint\n  cliquest reset\n", rule, def.Title, title(def.Category), title(def.Difficulty), def.Description, def.Mission, rule, active.Workspace, shellDisplay(active.Workspace))
	return nil
}

func (a *App) check() error {
	var result validator.Result
	var baseDef scene.Definition
	var xp int
	var newlyCompleted bool
	err := a.store.Update(func(data *progress.Data) error {
		if data.Active == nil {
			return fmt.Errorf("no active scene; run 'cliquest start <scene-id>'")
		}
		var ok bool
		baseDef, ok = a.catalog.Get(data.Active.SceneID)
		if !ok {
			return fmt.Errorf("active scene definition %q is missing", data.Active.SceneID)
		}
		result = validator.Check(baseDef, data.Active)
		if !result.Clear {
			return nil
		}
		workspace.Cleanup(data.Active.Resources)
		xp, newlyCompleted = progress.Complete(data, baseDef.ID, baseDef.Category, baseDef.Difficulty, time.Now())
		data.Active = nil
		return nil
	})
	if err != nil {
		return err
	}
	def := baseDef.Localize(a.language)
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"scene": def.ID, "clear": result.Clear, "message": localizedResultMessage(result, def), "xp": xp, "first_clear": newlyCompleted, "lesson": def.Lesson})
	}
	fmt.Fprintln(a.out, a.text("Checking scene...", "Sceneを確認中..."))
	fmt.Fprintln(a.out)
	if !result.Clear {
		fmt.Fprintf(a.out, "%s\n\n%s\n\n%s\n\n  cliquest hint\n", a.text("Not cleared yet.", "まだクリアしていません。"), result.Message, a.text("Need a hint? Run:", "Hintを見るには:"))
		return nil
	}
	fmt.Fprintf(a.out, "%s\n\n%s\n\n%s\n", a.green("✓ CLEAR"), def.Title, def.Success)
	lesson := def.Lesson
	if lesson.WhatHappened == "" {
		lesson.WhatHappened = def.Success
	}
	if len(lesson.OtherSolutions) == 0 {
		lesson.OtherSolutions = def.Solution
	}
	fmt.Fprintf(a.out, "\n%s:\n%s\n", a.text("What happened", "何が起きていたか"), lesson.WhatHappened)
	if len(lesson.OtherSolutions) > 0 {
		fmt.Fprintf(a.out, "\n%s:\n", a.text("Other solutions", "別の解決方法"))
		for _, solution := range lesson.OtherSolutions {
			fmt.Fprintf(a.out, "  - %s\n", solution)
		}
	}
	if lesson.BeCareful != "" {
		fmt.Fprintf(a.out, "\n%s:\n%s\n", a.text("Be careful", "実務上の注意"), lesson.BeCareful)
	}
	if newlyCompleted {
		fmt.Fprintf(a.out, "\n+%d XP\n", xp)
	} else {
		fmt.Fprintf(a.out, "\n+%d XP (%s)\n", xp, a.text("replay", "再挑戦"))
	}
	return nil
}

func localizedResultMessage(result validator.Result, def scene.Definition) string {
	if result.Clear {
		return def.Success
	}
	return result.Message
}

func (a *App) hint() error {
	var def scene.Definition
	var index int
	err := a.store.Update(func(data *progress.Data) error {
		if data.Active == nil {
			return fmt.Errorf("no active scene")
		}
		base, ok := a.catalog.Get(data.Active.SceneID)
		if !ok {
			return fmt.Errorf("active scene definition %q is missing", data.Active.SceneID)
		}
		def = base.Localize(a.language)
		if len(def.Hints) == 0 {
			return fmt.Errorf("this scene has no hints")
		}
		index = data.Active.HintIndex
		if index >= len(def.Hints) {
			index = len(def.Hints) - 1
		}
		if data.Active.HintIndex < len(def.Hints) {
			data.Active.HintIndex++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"scene": def.ID, "hint_number": index + 1, "hint_count": len(def.Hints), "hint": def.Hints[index]})
	}
	fmt.Fprintf(a.out, "Hint %d / %d\n\n%s\n", index+1, len(def.Hints), def.Hints[index])
	return nil
}

func (a *App) reset() error {
	var active *progress.ActiveScene
	var baseDef scene.Definition
	var operationErr error
	err := a.store.Update(func(data *progress.Data) error {
		if data.Active == nil {
			return fmt.Errorf("no active scene")
		}
		var ok bool
		baseDef, ok = a.catalog.Get(data.Active.SceneID)
		if !ok {
			return fmt.Errorf("active scene definition %q is missing", data.Active.SceneID)
		}
		progress.Cancel(data, baseDef.ID, baseDef.Category, baseDef.Difficulty, time.Now())
		workspace.Cleanup(data.Active.Resources)
		data.Active = nil
		resources, setupErr := a.workspaces.Setup(baseDef)
		if setupErr != nil {
			operationErr = fmt.Errorf("reset scene: %w", setupErr)
			return nil
		}
		started := time.Now()
		active = &progress.ActiveScene{SceneID: baseDef.ID, Workspace: a.workspaces.Path(baseDef.ID), StartedAt: started, Resources: resources}
		data.Active = active
		progress.RecordStart(data, baseDef.ID, started)
		return nil
	})
	if err != nil {
		return err
	}
	if operationErr != nil {
		return operationErr
	}
	def := baseDef.Localize(a.language)
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"scene": def.ID, "reset": true, "workspace": active.Workspace})
	}
	fmt.Fprintf(a.out, "%s\n\n%s\n\nWorkspace:\n%s\n", a.text("Scene reset.", "Sceneをresetしました。"), fmt.Sprintf(a.text("%s has been restored to its initial state.", "%sを初期状態へ戻しました。"), def.Title), active.Workspace)
	return nil
}

func (a *App) status() error {
	data, err := a.store.LoadState()
	if err != nil {
		return err
	}
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"completed": data.Completed, "completed_count": len(data.Completed), "total_scenes": len(a.catalog.All()), "total_xp": data.TotalXP, "active": data.Active})
	}
	fmt.Fprintf(a.out, "CLI QUEST STATUS\n%s\n\n%s: %d / %d\nXP: %d\n", rule, a.text("Progress", "進捗"), len(data.Completed), len(a.catalog.All()), data.TotalXP)
	if data.Active == nil {
		fmt.Fprintf(a.out, "\n%s\n\n%s\n", a.text("Active scene: none", "Active Scene: なし"), a.text("Run 'cliquest list' to choose a scene.", "cliquest listからSceneを選んでください。"))
	} else {
		def, _ := a.catalog.Get(data.Active.SceneID)
		def = def.Localize(a.language)
		fmt.Fprintf(a.out, "\nActive Scene: %s (%s)\n%s: %s\n%s: %d / %d\nWorkspace: %s\n", def.Title, def.ID, a.text("Started", "開始"), data.Active.StartedAt.Local().Format("2006-01-02 15:04"), a.text("Hints used", "Hint使用数"), data.Active.HintIndex, len(def.Hints), data.Active.Workspace)
	}
	if len(data.Completed) > 0 {
		completed := append([]string(nil), data.Completed...)
		sort.Strings(completed)
		fmt.Fprintf(a.out, "\n%s:\n", a.text("Cleared", "Clear済み"))
		for _, id := range completed {
			fmt.Fprintf(a.out, "  ✓ %s\n", id)
		}
	}
	return nil
}

func (a *App) doctor() error {
	availability := map[string]bool{}
	for _, command := range []string{"git", "curl", "jq", "docker"} {
		_, err := exec.LookPath(command)
		availability[command] = err == nil
	}
	if a.jsonOutput {
		return a.writeJSON(availability)
	}
	fmt.Fprintln(a.out, a.text("Environment", "環境"))
	fmt.Fprintln(a.out)
	for _, command := range []string{"git", "curl", "jq", "docker"} {
		mark := "✗"
		if availability[command] {
			mark = "✓"
		}
		fmt.Fprintf(a.out, "%-10s %s\n", title(command), mark)
	}
	fmt.Fprintln(a.out, a.text("\nGit is required for Git scenes. curl is recommended for HTTP scenes.\njq and Docker are optional.", "\nGit SceneにはGit、HTTP Sceneにはcurlを推奨します。jqとDockerは任意です。"))
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
