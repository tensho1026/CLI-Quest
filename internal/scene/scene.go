package scene

import (
	"encoding/json"
	"fmt"
	"io/fs"
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
	Type  string `json:"type"`
	Path  string `json:"path,omitempty"`
	Value string `json:"value,omitempty"`
}

type Definition struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Category    string     `json:"category"`
	Difficulty  string     `json:"difficulty"`
	Description string     `json:"description"`
	Mission     string     `json:"mission"`
	Success     string     `json:"success"`
	Setup       Setup      `json:"setup"`
	Validation  Validation `json:"validation"`
	Hints       []string   `json:"hints"`
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
		var def Definition
		if err := json.Unmarshal(data, &def); err != nil {
			return fmt.Errorf("read scene %s: %w", path, err)
		}
		if err := validate(def); err != nil {
			return fmt.Errorf("scene %s: %w", path, err)
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

func validate(def Definition) error {
	if def.ID == "" || def.Title == "" || def.Category == "" || def.Difficulty == "" {
		return fmt.Errorf("id, title, category and difficulty are required")
	}
	if def.Setup.Type == "" || def.Validation.Type == "" {
		return fmt.Errorf("setup.type and validation.type are required")
	}
	if strings.Contains(def.ID, "/") || strings.Contains(def.ID, "..") {
		return fmt.Errorf("unsafe id %q", def.ID)
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
	default:
		return 100
	}
}

func (c *Catalog) All() []Definition {
	return append([]Definition(nil), c.all...)
}

func (c *Catalog) Get(id string) (Definition, bool) {
	def, ok := c.byID[id]
	return def, ok
}
