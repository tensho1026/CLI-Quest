package progress

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type ActiveScene struct {
	SceneID   string            `json:"scene_id"`
	Workspace string            `json:"workspace"`
	HintIndex int               `json:"hint_index"`
	StartedAt time.Time         `json:"started_at"`
	Resources map[string]string `json:"resources,omitempty"`
}

type Data struct {
	Completed []string     `json:"completed"`
	Active    *ActiveScene `json:"active,omitempty"`
}

type Store struct {
	root string
	path string
}

func New() (*Store, error) {
	root := os.Getenv("CLIQUEST_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("find home directory: %w", err)
		}
		root = filepath.Join(home, ".cliquest")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve CLI Quest home: %w", err)
	}
	return &Store{root: root, path: filepath.Join(root, "progress.json")}, nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) Load() (Data, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Data{}, nil
	}
	if err != nil {
		return Data{}, fmt.Errorf("read progress: %w", err)
	}
	var progress Data
	if err := json.Unmarshal(data, &progress); err != nil {
		return Data{}, fmt.Errorf("parse %s: %w", s.path, err)
	}
	return progress, nil
}

func (s *Store) Save(progress Data) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create CLI Quest home: %w", err)
	}
	sort.Strings(progress.Completed)
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("encode progress: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(s.root, ".progress-*.json")
	if err != nil {
		return fmt.Errorf("create progress temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write progress: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close progress: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("save progress: %w", err)
	}
	return nil
}

func IsCompleted(data Data, id string) bool {
	for _, completed := range data.Completed {
		if completed == id {
			return true
		}
	}
	return false
}

func MarkCompleted(data *Data, id string) bool {
	if IsCompleted(*data, id) {
		return false
	}
	data.Completed = append(data.Completed, id)
	return true
}
