package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/tensho1026/CLI-Quest/internal/progress"
	"github.com/tensho1026/CLI-Quest/internal/scene"
)

type filters struct {
	Category   string
	Difficulty string
	Status     string
}

func (a *App) applyGlobalOptions(args []string) ([]string, error) {
	if data, err := a.store.Load(); err == nil && data.Settings.Language != "" {
		a.language = data.Settings.Language
	}
	var rest []string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			a.jsonOutput = true
		case "--no-color":
			a.color = false
		case "--lang":
			if index+1 >= len(args) {
				return nil, fmt.Errorf("--lang requires en or ja")
			}
			index++
			if !validLanguage(args[index]) {
				return nil, fmt.Errorf("unsupported language %q", args[index])
			}
			a.language = strings.ToLower(args[index])
		default:
			if strings.HasPrefix(args[index], "--lang=") {
				value := strings.TrimPrefix(args[index], "--lang=")
				if !validLanguage(value) {
					return nil, fmt.Errorf("unsupported language %q", value)
				}
				a.language = strings.ToLower(value)
			} else {
				rest = append(rest, args[index])
			}
		}
	}
	if a.jsonOutput {
		a.color = false
	}
	return rest, nil
}

func validLanguage(value string) bool {
	return strings.EqualFold(value, "en") || strings.EqualFold(value, "ja")
}

func parseFilters(args []string, allowStatus bool) (filters, []string, error) {
	var result filters
	var positional []string
	for index := 0; index < len(args); index++ {
		name := args[index]
		if name != "--category" && name != "--difficulty" && name != "--status" {
			positional = append(positional, name)
			continue
		}
		if index+1 >= len(args) {
			return result, nil, fmt.Errorf("%s requires a value", name)
		}
		index++
		switch name {
		case "--category":
			result.Category = strings.ToLower(args[index])
		case "--difficulty":
			result.Difficulty = strings.ToLower(args[index])
		case "--status":
			if !allowStatus {
				return result, nil, fmt.Errorf("--status is not valid here")
			}
			result.Status = strings.ToLower(args[index])
		}
	}
	if result.Difficulty != "" && result.Difficulty != "easy" && result.Difficulty != "normal" && result.Difficulty != "hard" {
		return result, nil, fmt.Errorf("unknown difficulty %q", result.Difficulty)
	}
	if result.Status != "" && result.Status != "clear" && result.Status != "active" && result.Status != "uncleared" {
		return result, nil, fmt.Errorf("unknown status %q", result.Status)
	}
	return result, positional, nil
}

func (a *App) matches(def scene.Definition, data progress.Data, selected filters) bool {
	if selected.Category != "" && !strings.EqualFold(def.Category, selected.Category) {
		return false
	}
	if selected.Difficulty != "" && !strings.EqualFold(def.Difficulty, selected.Difficulty) {
		return false
	}
	switch selected.Status {
	case "clear":
		return progress.IsCompleted(data, def.ID)
	case "active":
		return data.Active != nil && data.Active.SceneID == def.ID
	case "uncleared":
		return !progress.IsCompleted(data, def.ID)
	default:
		return true
	}
}

func (a *App) startCommand(args []string) error {
	selected, positional, err := parseFilters(args, false)
	if err != nil {
		return err
	}
	if len(positional) > 1 {
		return fmt.Errorf("usage: cliquest start [scene-id] [filters]")
	}
	if len(positional) == 1 {
		return a.startScene(positional[0])
	}
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	var candidates []scene.Definition
	for _, def := range a.catalog.All() {
		if a.matches(def, data, selected) {
			candidates = append(candidates, def)
		}
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no scenes match the selected category and difficulty")
	}
	if len(candidates) == 1 {
		return a.startScene(candidates[0].ID)
	}
	chosen, err := a.chooseScene(candidates, selected)
	if err != nil {
		return err
	}
	return a.startScene(chosen.ID)
}

func (a *App) chooseScene(candidates []scene.Definition, selected filters) (scene.Definition, error) {
	reader := bufio.NewReader(a.in)
	if selected.Category == "" {
		categories := distinct(candidates, func(def scene.Definition) string { return def.Category })
		value, err := a.chooseValue(reader, a.text("Choose a category", "分野を選択"), categories)
		if err != nil {
			return scene.Definition{}, err
		}
		if value != "" {
			selected.Category = value
		}
	}
	if selected.Difficulty == "" {
		var available []scene.Definition
		for _, def := range candidates {
			if selected.Category == "" || strings.EqualFold(def.Category, selected.Category) {
				available = append(available, def)
			}
		}
		difficulties := distinct(available, func(def scene.Definition) string { return def.Difficulty })
		value, err := a.chooseValue(reader, a.text("Choose a difficulty", "難易度を選択"), difficulties)
		if err != nil {
			return scene.Definition{}, err
		}
		if value != "" {
			selected.Difficulty = value
		}
	}
	var available []scene.Definition
	for _, def := range candidates {
		if (selected.Category == "" || strings.EqualFold(def.Category, selected.Category)) && (selected.Difficulty == "" || strings.EqualFold(def.Difficulty, selected.Difficulty)) {
			available = append(available, def)
		}
	}
	if len(available) == 1 {
		return available[0], nil
	}
	fmt.Fprintf(a.out, "\n%s\n", a.text("Choose a scene", "Sceneを選択"))
	for index, def := range available {
		localized := def.Localize(a.language)
		fmt.Fprintf(a.out, "  %d) %-24s %s / %s\n", index+1, localized.ID, title(localized.Category), title(localized.Difficulty))
	}
	fmt.Fprintf(a.out, "> ")
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return scene.Definition{}, err
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(available) {
		return scene.Definition{}, fmt.Errorf("invalid scene selection")
	}
	return available[choice-1], nil
}

func (a *App) chooseValue(reader *bufio.Reader, prompt string, values []string) (string, error) {
	fmt.Fprintf(a.out, "\n%s\n  0) %s\n", prompt, a.text("All", "すべて"))
	for index, value := range values {
		fmt.Fprintf(a.out, "  %d) %s\n", index+1, title(value))
	}
	fmt.Fprint(a.out, "> ")
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 0 || choice > len(values) {
		return "", fmt.Errorf("invalid selection")
	}
	if choice == 0 {
		return "", nil
	}
	return values[choice-1], nil
}

func distinct(defs []scene.Definition, value func(scene.Definition) string) []string {
	seen := map[string]bool{}
	var values []string
	for _, def := range defs {
		key := value(def)
		if !seen[key] {
			seen[key] = true
			values = append(values, key)
		}
	}
	sort.Strings(values)
	return values
}

func (a *App) next(args []string) error {
	selected, positional, err := parseFilters(args, false)
	if err != nil {
		return err
	}
	if len(positional) != 0 {
		return fmt.Errorf("usage: cliquest next [filters]")
	}
	data, err := a.store.Load()
	if err != nil {
		return err
	}
	for _, def := range a.catalog.All() {
		if !progress.IsCompleted(data, def.ID) && a.matches(def, data, selected) {
			return a.startScene(def.ID)
		}
	}
	return fmt.Errorf("all matching scenes are already clear")
}

func (a *App) writeJSON(value any) error {
	encoder := json.NewEncoder(a.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func (a *App) text(english, japanese string) string {
	if a.language == "ja" {
		return japanese
	}
	return english
}

func supportsColor(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") != "" {
		return true
	}
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func (a *App) green(value string) string {
	if !a.color {
		return value
	}
	return "\x1b[32m" + value + "\x1b[0m"
}
