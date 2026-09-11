package progress

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"time"
)

type ActiveScene struct {
	SceneID   string            `json:"scene_id"`
	Workspace string            `json:"workspace"`
	HintIndex int               `json:"hint_index"`
	StartedAt time.Time         `json:"started_at"`
	Resources map[string]string `json:"resources,omitempty"`
}

type SceneStats struct {
	Attempts      int       `json:"attempts"`
	Clears        int       `json:"clears"`
	TotalHints    int       `json:"total_hints"`
	TotalXP       int       `json:"total_xp"`
	BestSeconds   int64     `json:"best_seconds,omitempty"`
	LastStarted   time.Time `json:"last_started,omitempty"`
	LastCompleted time.Time `json:"last_completed,omitempty"`
}

type HistoryEntry struct {
	SceneID         string    `json:"scene_id"`
	Category        string    `json:"category"`
	Difficulty      string    `json:"difficulty"`
	Result          string    `json:"result"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
	DurationSeconds int64     `json:"duration_seconds"`
	HintsUsed       int       `json:"hints_used"`
	XP              int       `json:"xp"`
}

type Settings struct {
	Language string `json:"language,omitempty"`
}

type Data struct {
	Completed []string               `json:"completed"`
	Active    *ActiveScene           `json:"active,omitempty"`
	Stats     map[string]*SceneStats `json:"stats,omitempty"`
	History   []HistoryEntry         `json:"history,omitempty"`
	TotalXP   int                    `json:"total_xp,omitempty"`
	Settings  Settings               `json:"settings,omitempty"`
}

type Store struct {
	root        string
	path        string
	historyPath string
}

type persistedData struct {
	Completed []string               `json:"completed"`
	Active    *ActiveScene           `json:"active,omitempty"`
	Stats     map[string]*SceneStats `json:"stats,omitempty"`
	TotalXP   int                    `json:"total_xp,omitempty"`
	Settings  Settings               `json:"settings,omitempty"`
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
	return &Store{
		root:        root,
		path:        filepath.Join(root, "progress.json"),
		historyPath: filepath.Join(root, "history.jsonl"),
	}, nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) Load() (Data, error) {
	lock, err := s.lock(false)
	if err != nil {
		return Data{}, err
	}
	defer unlock(lock)
	return s.loadUnlocked()
}

func (s *Store) loadUnlocked() (Data, error) {
	progress, err := s.readProgressUnlocked()
	if err != nil {
		return Data{}, err
	}
	hasHistory, err := s.historyExistsUnlocked()
	if err != nil {
		return Data{}, err
	}
	if hasHistory {
		progress.History, err = s.readHistoryUnlocked()
		if err != nil {
			return Data{}, err
		}
	}
	normalize(&progress)
	return progress, nil
}

// LoadState loads progress metadata without decoding the history journal.
// Legacy progress files that still contain inline history are kept intact so
// the next state update can migrate them safely.
func (s *Store) LoadState() (Data, error) {
	lock, err := s.lock(false)
	if err != nil {
		return Data{}, err
	}
	defer unlock(lock)
	return s.loadStateUnlocked()
}

func (s *Store) loadStateUnlocked() (Data, error) {
	progress, err := s.readProgressUnlocked()
	if err != nil {
		return Data{}, err
	}
	hasHistory, err := s.historyExistsUnlocked()
	if err != nil {
		return Data{}, err
	}
	if hasHistory {
		progress.History = nil
	}
	normalize(&progress)
	return progress, nil
}

func (s *Store) readProgressUnlocked() (Data, error) {
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
	lock, err := s.lock(true)
	if err != nil {
		return err
	}
	defer unlock(lock)
	return s.saveUnlocked(progress)
}

func (s *Store) saveUnlocked(progress Data) error {
	if err := s.replaceHistoryUnlocked(progress.History); err != nil {
		return err
	}
	return s.saveStateUnlocked(progress)
}

func (s *Store) saveStateUnlocked(progress Data) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create CLI Quest home: %w", err)
	}
	sort.Strings(progress.Completed)
	data, err := json.MarshalIndent(persistedData{
		Completed: progress.Completed,
		Active:    progress.Active,
		Stats:     progress.Stats,
		TotalXP:   progress.TotalXP,
		Settings:  progress.Settings,
	}, "", "  ")
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

func (s *Store) Update(change func(*Data) error) error {
	lock, err := s.lock(true)
	if err != nil {
		return err
	}
	defer unlock(lock)
	hadHistory, err := s.historyExistsUnlocked()
	if err != nil {
		return err
	}
	data, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	previousHistory := append([]HistoryEntry(nil), data.History...)
	if err := change(&data); err != nil {
		return err
	}
	if err := s.persistHistoryChangeUnlocked(hadHistory, previousHistory, data.History); err != nil {
		return err
	}
	return s.saveStateUnlocked(data)
}

// UpdateState updates metadata without loading or rewriting the history
// journal. It is intended for operations such as hint and settings changes.
func (s *Store) UpdateState(change func(*Data) error) error {
	lock, err := s.lock(true)
	if err != nil {
		return err
	}
	defer unlock(lock)
	hadHistory, err := s.historyExistsUnlocked()
	if err != nil {
		return err
	}
	data, err := s.loadStateUnlocked()
	if err != nil {
		return err
	}
	if err := change(&data); err != nil {
		return err
	}
	if !hadHistory && len(data.History) > 0 {
		if err := s.replaceHistoryUnlocked(data.History); err != nil {
			return err
		}
	}
	return s.saveStateUnlocked(data)
}

func (s *Store) historyExistsUnlocked() (bool, error) {
	_, err := os.Stat(s.historyPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect history journal: %w", err)
	}
	return true, nil
}

func (s *Store) readHistoryUnlocked() ([]HistoryEntry, error) {
	file, err := os.Open(s.historyPath)
	if err != nil {
		return nil, fmt.Errorf("read history journal: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var history []HistoryEntry
	for {
		var entry HistoryEntry
		err := decoder.Decode(&entry)
		if errors.Is(err, io.EOF) {
			return history, nil
		}
		if err != nil {
			return nil, fmt.Errorf("parse history journal: %w", err)
		}
		history = append(history, entry)
	}
}

func (s *Store) persistHistoryChangeUnlocked(hadHistory bool, previous, current []HistoryEntry) error {
	if !hadHistory {
		if len(current) == 0 {
			return nil
		}
		return s.replaceHistoryUnlocked(current)
	}
	if !historyPrefix(previous, current) {
		return s.replaceHistoryUnlocked(current)
	}
	if len(current) == len(previous) {
		return nil
	}
	return s.appendHistoryUnlocked(current[len(previous):])
}

func historyPrefix(prefix, values []HistoryEntry) bool {
	if len(prefix) > len(values) {
		return false
	}
	return reflect.DeepEqual(prefix, values[:len(prefix)])
}

func (s *Store) appendHistoryUnlocked(entries []HistoryEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create CLI Quest home: %w", err)
	}
	file, err := os.OpenFile(s.historyPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open history journal: %w", err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			_ = file.Close()
			return fmt.Errorf("append history: %w", err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close history journal: %w", err)
	}
	return nil
}

func (s *Store) replaceHistoryUnlocked(history []HistoryEntry) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create CLI Quest home: %w", err)
	}
	if len(history) == 0 {
		if err := os.Remove(s.historyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove empty history journal: %w", err)
		}
		return nil
	}
	tmp, err := os.CreateTemp(s.root, ".history-*.jsonl")
	if err != nil {
		return fmt.Errorf("create history temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	encoder := json.NewEncoder(tmp)
	for _, entry := range history {
		if err := encoder.Encode(entry); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("write history: %w", err)
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close history: %w", err)
	}
	if err := os.Rename(tmpName, s.historyPath); err != nil {
		return fmt.Errorf("save history: %w", err)
	}
	return nil
}

// Streak calculates the current streak without decoding full HistoryEntry
// values when the journal format is available.
func (s *Store) Streak(now time.Time) (int, error) {
	lock, err := s.lock(false)
	if err != nil {
		return 0, err
	}
	defer unlock(lock)

	hasHistory, err := s.historyExistsUnlocked()
	if err != nil {
		return 0, err
	}
	if !hasHistory {
		data, err := s.loadUnlocked()
		if err != nil {
			return 0, err
		}
		return Streak(data, now), nil
	}

	file, err := os.Open(s.historyPath)
	if err != nil {
		return 0, fmt.Errorf("read history journal: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	days := map[string]bool{}
	for {
		var entry struct {
			Result  string    `json:"result"`
			EndedAt time.Time `json:"ended_at"`
		}
		err := decoder.Decode(&entry)
		if errors.Is(err, io.EOF) {
			return streakFromDays(days, now), nil
		}
		if err != nil {
			return 0, fmt.Errorf("parse history journal: %w", err)
		}
		if entry.Result == "clear" {
			days[entry.EndedAt.Local().Format("2006-01-02")] = true
		}
	}
}

func (s *Store) lock(exclusive bool) (*os.File, error) {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return nil, fmt.Errorf("create CLI Quest home: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(s.root, "progress.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open progress lock: %w", err)
	}
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	if err := syscall.Flock(int(file.Fd()), how); err != nil {
		file.Close()
		return nil, fmt.Errorf("lock progress: %w", err)
	}
	return file, nil
}

func unlock(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}

func normalize(data *Data) {
	if data.Stats == nil {
		data.Stats = map[string]*SceneStats{}
	}
	if data.Settings.Language == "" {
		data.Settings.Language = "en"
	}
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

func RecordStart(data *Data, id string, started time.Time) {
	normalize(data)
	stats := data.Stats[id]
	if stats == nil {
		stats = &SceneStats{}
		data.Stats[id] = stats
	}
	stats.Attempts++
	stats.LastStarted = started
}

func Complete(data *Data, id, category, difficulty string, ended time.Time) (int, bool) {
	if data.Active == nil {
		return 0, false
	}
	normalize(data)
	duration := ended.Sub(data.Active.StartedAt)
	if duration < 0 {
		duration = 0
	}
	seconds := int64(duration.Round(time.Second) / time.Second)
	xp := CalculateXP(difficulty, data.Active.HintIndex)
	stats := data.Stats[id]
	if stats == nil {
		stats = &SceneStats{}
		data.Stats[id] = stats
	}
	stats.Clears++
	stats.TotalHints += data.Active.HintIndex
	stats.TotalXP += xp
	stats.LastCompleted = ended
	if stats.BestSeconds == 0 || seconds < stats.BestSeconds {
		stats.BestSeconds = seconds
	}
	data.TotalXP += xp
	data.History = append(data.History, HistoryEntry{
		SceneID: id, Category: category, Difficulty: difficulty, Result: "clear",
		StartedAt: data.Active.StartedAt, EndedAt: ended, DurationSeconds: seconds,
		HintsUsed: data.Active.HintIndex, XP: xp,
	})
	return xp, MarkCompleted(data, id)
}

func Cancel(data *Data, id, category, difficulty string, ended time.Time) {
	if data.Active == nil {
		return
	}
	duration := ended.Sub(data.Active.StartedAt)
	if duration < 0 {
		duration = 0
	}
	data.History = append(data.History, HistoryEntry{
		SceneID: id, Category: category, Difficulty: difficulty, Result: "cancelled",
		StartedAt: data.Active.StartedAt, EndedAt: ended,
		DurationSeconds: int64(duration.Round(time.Second) / time.Second),
		HintsUsed:       data.Active.HintIndex,
	})
}

func CalculateXP(difficulty string, hints int) int {
	base := 150
	switch strings.ToLower(difficulty) {
	case "easy":
		base = 100
	case "hard":
		base = 250
	}
	xp := base - hints*20
	minimum := base * 40 / 100
	if xp < minimum {
		xp = minimum
	}
	return xp
}

func Streak(data Data, now time.Time) int {
	days := map[string]bool{}
	for _, entry := range data.History {
		if entry.Result == "clear" {
			days[entry.EndedAt.Local().Format("2006-01-02")] = true
		}
	}
	return streakFromDays(days, now)
}

func streakFromDays(days map[string]bool, now time.Time) int {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !days[day.Format("2006-01-02")] {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for days[day.Format("2006-01-02")] {
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return streak
}
