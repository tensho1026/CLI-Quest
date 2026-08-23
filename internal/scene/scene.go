package scene

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	definitions "github.com/tensho1026/CLI-Quest/scenes"
)

type FileSpec struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Mode    uint32 `json:"mode,omitempty"`
	Size    int64  `json:"size,omitempty"`
}

type Setup struct {
	Type   string            `json:"type"`
	Files  []FileSpec        `json:"files,omitempty"`
	Port   int               `json:"port,omitempty"`
	Mode   string            `json:"mode,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

type Validation struct {
	Type       string       `json:"type"`
	Path       string       `json:"path,omitempty"`
	Value      string       `json:"value,omitempty"`
	Name       string       `json:"name,omitempty"`
	Validators []Validation `json:"validators,omitempty"`
}

type Lesson struct {
	WhatHappened   string   `json:"what_happened"`
	OtherSolutions []string `json:"other_solutions,omitempty"`
	BeCareful      string   `json:"be_careful,omitempty"`
}

type LocalizedText struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Mission     string   `json:"mission,omitempty"`
	Success     string   `json:"success,omitempty"`
	Hints       []string `json:"hints,omitempty"`
	Solution    []string `json:"solution,omitempty"`
	Lesson      Lesson   `json:"lesson,omitempty"`
}

type Definition struct {
	ID           string                   `json:"id"`
	Title        string                   `json:"title"`
	Category     string                   `json:"category"`
	Difficulty   string                   `json:"difficulty"`
	Description  string                   `json:"description"`
	Mission      string                   `json:"mission"`
	Success      string                   `json:"success"`
	Solution     []string                 `json:"solution,omitempty"`
	Lesson       Lesson                   `json:"lesson,omitempty"`
	Translations map[string]LocalizedText `json:"translations,omitempty"`
	Setup        Setup                    `json:"setup"`
	Validation   Validation               `json:"validation"`
	Hints        []string                 `json:"hints"`
}

type Catalog struct {
	byID map[string]Definition
	all  []Definition
}

func Load() (*Catalog, error) {
	var loaded []Definition
	err := fs.WalkDir(definitions.Files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := definitions.Files.ReadFile(path)
		if err != nil {
			return err
		}
		def, err := Decode(data)
		if err != nil {
			return fmt.Errorf("read scene %s: %w", path, err)
		}
		loaded = append(loaded, def)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(loaded, func(i, j int) bool {
		if loaded[i].Category == loaded[j].Category {
			return loaded[i].ID < loaded[j].ID
		}
		return categoryOrder(loaded[i].Category) < categoryOrder(loaded[j].Category)
	})
	byID := make(map[string]Definition, len(loaded))
	for _, def := range loaded {
		if _, exists := byID[def.ID]; exists {
			return nil, fmt.Errorf("duplicate scene id %q", def.ID)
		}
		byID[def.ID] = def
	}
	return &Catalog{byID: byID, all: loaded}, nil
}

func Decode(data []byte) (Definition, error) {
	var def Definition
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&def); err != nil {
		return Definition{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Definition{}, fmt.Errorf("scene must contain exactly one JSON object")
	}
	if err := Validate(def); err != nil {
		return Definition{}, err
	}
	return def, nil
}

func Validate(def Definition) error {
	if def.ID == "" || def.Title == "" || def.Category == "" || def.Difficulty == "" || def.Description == "" || def.Mission == "" || def.Success == "" {
		return fmt.Errorf("id, title, category, difficulty, description, mission and success are required")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(def.ID) {
		return fmt.Errorf("id must use lowercase letters, digits and hyphens")
	}
	if def.Difficulty != "easy" && def.Difficulty != "normal" && def.Difficulty != "hard" {
		return fmt.Errorf("difficulty must be easy, normal or hard")
	}
	if len(def.Hints) == 0 {
		return fmt.Errorf("at least one hint is required")
	}
	if def.Setup.Type == "" || def.Validation.Type == "" {
		return fmt.Errorf("setup.type and validation.type are required")
	}
	knownSetup := map[string]bool{
		"filesystem": true, "git_wrong_branch": true, "git_restore_file": true,
		"git_lost_commit": true, "git_conflict": true, "git_stash_recovery": true,
		"git_bisect": true, "port_conflict": true, "runaway_process": true,
		"http_server": true,
	}
	if !knownSetup[def.Setup.Type] {
		return fmt.Errorf("unsupported setup type %q", def.Setup.Type)
	}
	if (def.Setup.Type == "port_conflict" || def.Setup.Type == "http_server") && (def.Setup.Port < 1 || def.Setup.Port > 65535) {
		return fmt.Errorf("setup port must be between 1 and 65535")
	}
	if def.Setup.Type == "http_server" {
		knownMode := map[string]bool{"health": true, "auth": true, "json": true, "debug": true, "incident": true}
		if !knownMode[def.Setup.Mode] {
			return fmt.Errorf("unsupported HTTP mode %q", def.Setup.Mode)
		}
	}
	if err := validateRule(def.Validation); err != nil {
		return fmt.Errorf("validation: %w", err)
	}
	for _, file := range def.Setup.Files {
		clean := filepath.ToSlash(filepath.Clean(file.Path))
		if file.Path == "" || filepath.IsAbs(file.Path) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("unsafe setup path %q", file.Path)
		}
	}
	return nil
}

func validateRule(rule Validation) error {
	known := map[string]bool{
		"all": true, "any": true, "executable": true, "file_exists": true, "file_mode": true,
		"file_contains": true, "file_not_contains": true, "git_branch": true,
		"git_clean": true, "git_no_conflicts": true, "git_file_contains": true,
		"git_wrong_branch": true, "git_commit_contains": true, "git_bisect_answer": true,
		"process_stopped": true, "port_available": true, "marker_exists": true,
	}
	if !known[rule.Type] {
		return fmt.Errorf("unsupported type %q", rule.Type)
	}
	pathRequired := map[string]bool{"executable": true, "file_exists": true, "file_mode": true, "file_contains": true, "file_not_contains": true, "git_file_contains": true, "git_commit_contains": true, "git_bisect_answer": true, "marker_exists": true}
	if pathRequired[rule.Type] && rule.Path == "" {
		return fmt.Errorf("type %s requires path", rule.Type)
	}
	if rule.Path != "" {
		clean := filepath.ToSlash(filepath.Clean(rule.Path))
		if filepath.IsAbs(rule.Path) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("unsafe path %q", rule.Path)
		}
	}
	if rule.Type == "git_branch" && rule.Name == "" {
		return fmt.Errorf("git_branch requires name")
	}
	if rule.Type == "all" || rule.Type == "any" {
		if len(rule.Validators) == 0 {
			return fmt.Errorf("%s requires at least one nested validator", rule.Type)
		}
		for _, nested := range rule.Validators {
			if err := validateRule(nested); err != nil {
				return err
			}
		}
	} else if len(rule.Validators) != 0 {
		return fmt.Errorf("type %s cannot contain nested validators", rule.Type)
	}
	return nil
}

func categoryOrder(category string) int {
	switch strings.ToLower(category) {
	case "linux":
		return 0
	case "git":
		return 1
	case "process":
		return 2
	case "http":
		return 3
	case "shell":
		return 4
	case "network":
		return 5
	case "incident":
		return 6
	default:
		return 100
	}
}

func (d Definition) Localize(language string) Definition {
	localized, ok := d.Translations[strings.ToLower(language)]
	if !ok {
		return d
	}
	if localized.Title != "" {
		d.Title = localized.Title
	}
	if localized.Description != "" {
		d.Description = localized.Description
	}
	if localized.Mission != "" {
		d.Mission = localized.Mission
	}
	if localized.Success != "" {
		d.Success = localized.Success
	}
	if len(localized.Hints) > 0 {
		d.Hints = localized.Hints
	}
	if len(localized.Solution) > 0 {
		d.Solution = localized.Solution
	}
	if localized.Lesson.WhatHappened != "" {
		d.Lesson = localized.Lesson
	}
	return d
}

func (c *Catalog) Filter(category, difficulty string, includeCompleted func(string) bool) []Definition {
	var filtered []Definition
	for _, def := range c.all {
		if category != "" && !strings.EqualFold(def.Category, category) {
			continue
		}
		if difficulty != "" && !strings.EqualFold(def.Difficulty, difficulty) {
			continue
		}
		if includeCompleted != nil && !includeCompleted(def.ID) {
			continue
		}
		filtered = append(filtered, def)
	}
	return filtered
}

func (c *Catalog) All() []Definition {
	return append([]Definition(nil), c.all...)
}

func (c *Catalog) Get(id string) (Definition, bool) {
	def, ok := c.byID[id]
	return def, ok
}
