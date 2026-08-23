package cli

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tensho1026/CLI-Quest/internal/progress"
	"github.com/tensho1026/CLI-Quest/internal/scene"
	"github.com/tensho1026/CLI-Quest/internal/workspace"
)

func (a *App) cancel() error {
	var def scene.Definition
	var workspacePath string
	err := a.store.Update(func(data *progress.Data) error {
		if data.Active == nil {
			return fmt.Errorf("no active scene")
		}
		base, ok := a.catalog.Get(data.Active.SceneID)
		if !ok {
			return fmt.Errorf("active scene definition %q is missing", data.Active.SceneID)
		}
		def = base.Localize(a.language)
		workspacePath = data.Active.Workspace
		progress.Cancel(data, base.ID, base.Category, base.Difficulty, time.Now())
		workspace.Cleanup(data.Active.Resources)
		if err := a.workspaces.Remove(data.Active.SceneID); err != nil {
			return err
		}
		data.Active = nil
		return nil
	})
	if err != nil {
		return err
	}
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"cancelled": true, "scene": def.ID, "workspace_removed": workspacePath})
	}
	fmt.Fprintf(a.out, "%s\n\n%s (%s)\n%s\n", a.text("Scene cancelled.", "Sceneをcancelしました。"), def.Title, def.ID, a.text("Owned resources were stopped and the workspace was removed.", "所有resourceを停止しWorkspaceを削除しました。"))
	return nil
}

func (a *App) cleanup() error {
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	activeWorkspace := ""
	if data.Active != nil {
		activeWorkspace = data.Active.Workspace
	}
	report, err := a.workspaces.CleanupStale(activeWorkspace)
	if err != nil {
		return err
	}
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"orphan_processes_stopped": report.Stopped, "stale_records_removed": report.Stale, "active_scene_preserved": data.Active != nil})
	}
	fmt.Fprintf(a.out, "%s\n\n%s: %d\n%s: %d\n", a.text("Cleanup complete.", "Cleanupが完了しました。"), a.text("Orphan processes stopped", "孤立processの停止数"), report.Stopped, a.text("Stale records removed", "stale recordの削除数"), report.Stale)
	if data.Active != nil {
		fmt.Fprintln(a.out, a.text("\nThe active scene and its live resources were preserved.", "\nActive Sceneと稼働中resourceは維持しました。"))
	}
	return nil
}

func (a *App) openWorkspace() error {
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	if data.Active == nil {
		return fmt.Errorf("no active scene")
	}
	command := "cd " + shellDisplay(data.Active.Workspace)
	if a.jsonOutput {
		return a.writeJSON(map[string]string{"workspace": data.Active.Workspace, "command": command})
	}
	fmt.Fprintln(a.out, command)
	fmt.Fprintln(a.out, a.text("\nRun this command in your shell; a child CLI cannot change its parent shell directory.", "\nこのcommandをshellで実行してください。child processからparent shellのdirectoryは変更できません。"))
	return nil
}

func (a *App) solution() error {
	var def scene.Definition
	err := a.store.Update(func(data *progress.Data) error {
		if data.Active == nil {
			return fmt.Errorf("no active scene")
		}
		base, ok := a.catalog.Get(data.Active.SceneID)
		if !ok {
			return fmt.Errorf("active scene definition %q is missing", data.Active.SceneID)
		}
		def = base.Localize(a.language)
		if data.Active.HintIndex < len(def.Hints) {
			data.Active.HintIndex = len(def.Hints)
		}
		return nil
	})
	if err != nil {
		return err
	}
	solutions := def.Solution
	if len(solutions) == 0 && len(def.Hints) > 0 {
		solutions = []string{def.Hints[len(def.Hints)-1]}
	}
	if len(solutions) == 0 {
		return fmt.Errorf("this scene has no explicit solution")
	}
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"scene": def.ID, "solutions": solutions, "xp_penalty_applied": true})
	}
	fmt.Fprintf(a.out, "%s — %s\n", a.text("Solution examples", "解答例"), def.Title)
	for _, item := range solutions {
		fmt.Fprintf(a.out, "\n  %s\n", item)
	}
	fmt.Fprintln(a.out, a.text("\nThe examples are not the only valid solutions. Revealing them applies the maximum hint penalty.", "\nこれだけが正解ではありません。解答表示には最大Hint penaltyが適用されます。"))
	return nil
}

func (a *App) history(args []string) error {
	limit := 20
	if len(args) == 2 && args[0] == "--limit" {
		parsed, err := strconv.Atoi(args[1])
		if err != nil || parsed < 1 {
			return fmt.Errorf("--limit must be a positive integer")
		}
		limit = parsed
	} else if len(args) != 0 {
		return fmt.Errorf("usage: cliquest history [--limit N]")
	}
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	entries := append([]progress.HistoryEntry(nil), data.History...)
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
		entries[left], entries[right] = entries[right], entries[left]
	}
	if a.jsonOutput {
		return a.writeJSON(entries)
	}
	if len(entries) == 0 {
		fmt.Fprintln(a.out, a.text("No attempt history yet.", "まだ挑戦履歴がありません。"))
		return nil
	}
	fmt.Fprintln(a.out, a.text("WHEN              RESULT     SCENE                    TIME    HINTS  XP", "日時              結果       SCENE                    時間    HINT   XP"))
	fmt.Fprintln(a.out, rule+"────────────────────────────────────")
	for _, entry := range entries {
		fmt.Fprintf(a.out, "%-17s %-10s %-24s %-7s %-6d %d\n", entry.EndedAt.Local().Format("2006-01-02 15:04"), entry.Result, entry.SceneID, formatDuration(entry.DurationSeconds), entry.HintsUsed, entry.XP)
	}
	return nil
}

type statsGroup struct {
	Attempts  int `json:"attempts"`
	Clears    int `json:"clears"`
	XP        int `json:"xp"`
	Completed int `json:"completed"`
	Total     int `json:"total"`
}

func (a *App) stats() error {
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	categories := map[string]*statsGroup{}
	difficulties := map[string]*statsGroup{}
	for _, def := range a.catalog.All() {
		if categories[def.Category] == nil {
			categories[def.Category] = &statsGroup{}
		}
		if difficulties[def.Difficulty] == nil {
			difficulties[def.Difficulty] = &statsGroup{}
		}
		categories[def.Category].Total++
		difficulties[def.Difficulty].Total++
		if progress.IsCompleted(data, def.ID) {
			categories[def.Category].Completed++
			difficulties[def.Difficulty].Completed++
		}
		if item := data.Stats[def.ID]; item != nil {
			categories[def.Category].Attempts += item.Attempts
			categories[def.Category].Clears += item.Clears
			categories[def.Category].XP += item.TotalXP
			difficulties[def.Difficulty].Attempts += item.Attempts
			difficulties[def.Difficulty].Clears += item.Clears
			difficulties[def.Difficulty].XP += item.TotalXP
		}
	}
	result := map[string]any{"total_xp": data.TotalXP, "streak_days": progress.Streak(data, time.Now()), "completed": len(data.Completed), "total_scenes": len(a.catalog.All()), "categories": categories, "difficulties": difficulties}
	if a.jsonOutput {
		return a.writeJSON(result)
	}
	fmt.Fprintf(a.out, "CLI QUEST STATS\n%s\n\nXP: %d\n%s: %d %s\n%s: %d / %d\n\n%s\n", rule, data.TotalXP, a.text("Streak", "連続学習"), progress.Streak(data, time.Now()), a.text("day(s)", "日"), a.text("Progress", "進捗"), len(data.Completed), len(a.catalog.All()), a.text("Categories", "分野"))
	printStatsGroups(a.out, categories)
	fmt.Fprintf(a.out, "\n%s\n", a.text("Difficulties", "難易度"))
	printStatsGroups(a.out, difficulties)
	return nil
}

func printStatsGroups(out interface{ Write([]byte) (int, error) }, groups map[string]*statsGroup) {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		fmt.Fprintf(out, "  %-12s %2d / %-2d  attempts=%-3d clears=%-3d xp=%d\n", title(key), group.Completed, group.Total, group.Attempts, group.Clears, group.XP)
	}
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm%02ds", seconds/60, seconds%60)
}

func (a *App) config(args []string) error {
	if len(args) == 0 {
		data, err := a.store.Load()
		if err != nil {
			return err
		}
		if a.jsonOutput {
			return a.writeJSON(data.Settings)
		}
		fmt.Fprintf(a.out, "language=%s\n", data.Settings.Language)
		return nil
	}
	if len(args) != 2 || args[0] != "language" || !validLanguage(args[1]) {
		return fmt.Errorf("usage: cliquest config language <en|ja>")
	}
	language := strings.ToLower(args[1])
	if err := a.store.Update(func(data *progress.Data) error { data.Settings.Language = language; return nil }); err != nil {
		return err
	}
	a.language = language
	if a.jsonOutput {
		return a.writeJSON(map[string]string{"language": language})
	}
	fmt.Fprintf(a.out, "%s: %s\n", a.text("Language saved", "言語設定を保存しました"), language)
	return nil
}

func (a *App) sceneCommand(args []string) error {
	if len(args) != 2 || args[0] != "validate" {
		return fmt.Errorf("usage: cliquest scene validate <scene.json>")
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	def, err := scene.Decode(data)
	if err != nil {
		return fmt.Errorf("invalid scene: %w", err)
	}
	if a.jsonOutput {
		return a.writeJSON(map[string]any{"valid": true, "id": def.ID, "category": def.Category, "difficulty": def.Difficulty})
	}
	fmt.Fprintf(a.out, "%s: %s (%s / %s)\n", a.green("✓ VALID"), def.ID, def.Category, def.Difficulty)
	return nil
}

func (a *App) completion(shell string) error {
	ids := make([]string, 0, len(a.catalog.All()))
	for _, def := range a.catalog.All() {
		ids = append(ids, def.ID)
	}
	commands := "list start next check hint solution reset cancel cleanup open status history stats doctor completion config scene help version"
	switch shell {
	case "bash":
		fmt.Fprintf(a.out, "_cliquest() { local cur=\"${COMP_WORDS[COMP_CWORD]}\"; if [ \"${COMP_WORDS[1]}\" = start ]; then COMPREPLY=( $(compgen -W '%s' -- \"$cur\") ); else COMPREPLY=( $(compgen -W '%s' -- \"$cur\") ); fi; }; complete -F _cliquest cliquest\n", strings.Join(ids, " "), commands)
	case "zsh":
		fmt.Fprintf(a.out, "#compdef cliquest\n_cliquest() { local -a commands scenes; commands=(%s); scenes=(%s); if [[ $words[2] == start ]]; then _describe 'scene' scenes; else _describe 'command' commands; fi }; _cliquest\n", strings.ReplaceAll(commands, " ", "\n  "), strings.Join(ids, "\n  "))
	case "fish":
		fmt.Fprintf(a.out, "complete -c cliquest -f -n '__fish_use_subcommand' -a '%s'\ncomplete -c cliquest -f -n '__fish_seen_subcommand_from start' -a '%s'\n", commands, strings.Join(ids, " "))
	default:
		return fmt.Errorf("unsupported shell %q; choose bash, zsh, or fish", shell)
	}
	return nil
}
