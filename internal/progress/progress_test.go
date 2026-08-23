package progress

import (
	"path/filepath"
	"reflect"
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
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch\ngot:  %#v\nwant: %#v", got, want)
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
