package lastfm

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func play(title string, started time.Time) Scrobble {
	return Scrobble{Track: Track{Artist: "A", Title: title, Album: "Al", Duration: 200 * time.Second}, Started: started, Chosen: title == "keep"}
}

func TestOutboxSurvivesRestartWithOriginalTimestamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.json")
	now := time.Unix(1_700_000_000, 0)
	o := OpenOutbox(path)
	first, second := now.Add(-3*time.Hour), now.Add(-2*time.Hour)
	o.Add(play("keep", first))
	o.Add(play("two", second))
	o.Wait()

	reopened := OpenOutbox(path)
	batch := reopened.Batch(now)
	if len(batch) != 2 {
		t.Fatalf("after a restart: %d pending, want 2", len(batch))
	}
	if !batch[0].Started.Equal(first) || !batch[1].Started.Equal(second) {
		t.Errorf("timestamps changed: %v %v", batch[0].Started, batch[1].Started)
	}
	if batch[0].Title != "keep" || !batch[0].Chosen || batch[0].Album != "Al" || batch[0].Duration != 200*time.Second || batch[1].Chosen {
		t.Errorf("fields lost: %+v %+v", batch[0], batch[1])
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("stat = %v, %v; want mode 0600", info, err)
	}

	reopened.Done(1)
	reopened.Wait()
	if left := OpenOutbox(path).Batch(now); len(left) != 1 || left[0].Title != "two" {
		t.Errorf("after delivering the first: %+v", left)
	}
}

func TestOutboxDropsPlaysLastFmWouldIgnore(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	o := &Outbox{}
	o.Add(play("old", now.Add(-14*24*time.Hour)))
	o.Add(play("new", now.Add(-time.Hour)))
	batch := o.Batch(now)
	if len(batch) != 1 || batch[0].Title != "new" || o.Len() != 1 {
		t.Errorf("batch = %+v, len %d", batch, o.Len())
	}
}

func TestOutboxBatchesAtLastFmsLimitAndCapsItself(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	o := &Outbox{}
	for i := range MaxPending + 5 {
		o.Add(play("t", now.Add(time.Duration(i-MaxPending-10)*time.Second)))
	}
	if o.Len() != MaxPending {
		t.Fatalf("Len = %d, want the cap %d", o.Len(), MaxPending)
	}
	batch := o.Batch(now)
	if len(batch) != MaxBatch {
		t.Fatalf("batch = %d, want %d", len(batch), MaxBatch)
	}
	if want := now.Add(time.Duration(5-MaxPending-10) * time.Second); !batch[0].Started.Equal(want) {
		t.Errorf("oldest kept = %v, want %v: the oldest plays go first", batch[0].Started, want)
	}
}

func TestOutboxToleratesBadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	o := OpenOutbox(path)
	if o.Len() != 0 {
		t.Error("an unreadable file is an empty outbox")
	}
	o.Add(play("x", time.Now()))
	o.Wait()
	if OpenOutbox(path).Len() != 1 {
		t.Error("the outbox did not replace the bad file")
	}
}

func TestOutboxClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.json")
	o := OpenOutbox(path)
	o.Add(play("x", time.Now()))
	o.Clear()
	o.Wait()
	if OpenOutbox(path).Len() != 0 {
		t.Error("Clear left plays on disk")
	}
}

func TestSessionStoreIsPrivate(t *testing.T) {
	s := &SessionStore{Path: filepath.Join(t.TempDir(), "lastfm.txt")}
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Load before save: %v", err)
	}
	if err := s.Save("KEY"); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(s.Path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	if key, err := s.Load(); err != nil || key != "KEY" {
		t.Errorf("Load = %q, %v", key, err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(); err != nil {
		t.Errorf("deleting twice: %v", err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrNoSession) {
		t.Errorf("Load after delete: %v", err)
	}
}
