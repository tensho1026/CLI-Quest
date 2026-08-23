package progress

import (
	"path/filepath"
	"reflect"
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
