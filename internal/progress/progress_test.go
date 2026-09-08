package progress

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	t.Setenv("CLIQUEST_HOME", root)
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	want := Data{
		Completed: []string{"linux-permission", "git-restore-file"},
		Active: &ActiveScene{
			SceneID: "linux-log-search", Workspace: "/tmp/example", HintIndex: 2,
			StartedAt: time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC),
			Resources: map[string]string{"pid": "123"},
		},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Save sorts completed IDs as part of its stable on-disk representation.
	want.Completed = []string{"git-restore-file", "linux-permission"}
	want.Stats = map[string]*SceneStats{}
	want.Settings.Language = "en"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestHistoryJournalRoundTripAndAppend(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	t.Setenv("CLIQUEST_HOME", root)
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	first := HistoryEntry{SceneID: "first", Result: "clear", EndedAt: time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)}
	second := HistoryEntry{SceneID: "second", Result: "cancelled", EndedAt: time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)}
	if err := store.Save(Data{History: []HistoryEntry{first}}); err != nil {
		t.Fatal(err)
	}
	progressData, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(progressData), `"history"`) {
		t.Fatal("progress metadata unexpectedly contains history")
	}
	historyData, err := os.ReadFile(store.historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(historyData, []byte("\n")); got != 1 {
		t.Fatalf("history journal has %d records, want 1", got)
	}
	if err := store.Update(func(data *Data) error {
		data.History = append(data.History, second)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	historyData, err = os.ReadFile(store.historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(historyData, []byte("\n")); got != 2 {
		t.Fatalf("history journal has %d records, want 2", got)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 2 || got.History[0].SceneID != first.SceneID || got.History[1].SceneID != second.SceneID {
		t.Fatalf("unexpected history: %#v", got.History)
	}
}

func TestLoadStateSkipsHistoryJournal(t *testing.T) {
	t.Setenv("CLIQUEST_HOME", filepath.Join(t.TempDir(), "state"))
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	entry := HistoryEntry{SceneID: "scene", Result: "clear", EndedAt: time.Now()}
	if err := store.Save(Data{History: []HistoryEntry{entry}}); err != nil {
		t.Fatal(err)
	}
	state, err := store.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.History != nil {
		t.Fatalf("LoadState decoded history: %#v", state.History)
	}
	full, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(full.History) != 1 || full.History[0].SceneID != entry.SceneID {
		t.Fatalf("Load did not restore history: %#v", full.History)
	}
}

func TestLegacyInlineHistoryMigratesOnStateUpdate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	t.Setenv("CLIQUEST_HOME", root)
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	entry := HistoryEntry{SceneID: "legacy", Result: "clear", EndedAt: time.Now()}
	legacy, err := json.Marshal(Data{History: []HistoryEntry{entry}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(func(data *Data) error {
		data.TotalXP = 10
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.History) != 1 || got.History[0].SceneID != entry.SceneID || got.TotalXP != 10 {
		t.Fatalf("legacy history was not preserved: %#v", got)
	}
	progressData, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(progressData), `"history"`) {
		t.Fatal("legacy history was not removed from progress metadata")
	}
}

func TestConcurrentUpdatesUseFileLock(t *testing.T) {
	t.Setenv("CLIQUEST_HOME", filepath.Join(t.TempDir(), "state"))
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if updateErr := store.Update(func(data *Data) error { data.TotalXP++; return nil }); updateErr != nil {
				t.Errorf("update: %v", updateErr)
			}
		}()
	}
	wait.Wait()
	data, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if data.TotalXP != workers {
		t.Fatalf("got TotalXP %d, want %d", data.TotalXP, workers)
	}
}

func TestMarkCompletedIsIdempotent(t *testing.T) {
	data := Data{}
	if !MarkCompleted(&data, "scene") {
		t.Fatal("first completion should be new")
	}
	if MarkCompleted(&data, "scene") {
		t.Fatal("second completion should not be new")
	}
	if len(data.Completed) != 1 {
		t.Fatalf("got %d completion entries", len(data.Completed))
	}
}

func TestXPAndCompletionHistory(t *testing.T) {
	started := time.Date(2026, 8, 23, 10, 0, 0, 0, time.Local)
	data := Data{Active: &ActiveScene{SceneID: "hard-scene", StartedAt: started, HintIndex: 2}}
	RecordStart(&data, "hard-scene", started)
	xp, first := Complete(&data, "hard-scene", "git", "hard", started.Add(90*time.Second))
	if !first || xp != 210 {
		t.Fatalf("got first=%v xp=%d; want true, 210", first, xp)
	}
	if data.TotalXP != 210 || len(data.History) != 1 || data.History[0].DurationSeconds != 90 {
		t.Fatalf("unexpected completion data: %#v", data)
	}
	if data.Stats["hard-scene"].Attempts != 1 || data.Stats["hard-scene"].BestSeconds != 90 {
		t.Fatalf("unexpected scene stats: %#v", data.Stats["hard-scene"])
	}
}

func TestStreakUsesConsecutiveCompletionDays(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.Local)
	data := Data{History: []HistoryEntry{
		{Result: "clear", EndedAt: now.AddDate(0, 0, -2)},
		{Result: "clear", EndedAt: now.AddDate(0, 0, -1)},
		{Result: "clear", EndedAt: now},
	}}
	if got := Streak(data, now); got != 3 {
		t.Fatalf("got streak %d, want 3", got)
	}
}
