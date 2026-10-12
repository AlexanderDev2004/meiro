package player

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// These tests run the real ffmpeg on real audio and feed its PCM to a fake
// speaker that listens at twenty times real time, so a whole track plays in
// a fraction of a second and no sound card is needed.

// newTestPlayer returns a player on the fake speaker, and a three second
// sine wave made by ffmpeg to play.
func newTestPlayer(t *testing.T) (*Player, string) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	wav := filepath.Join(t.TempDir(), "tone.wav")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=3", wav).CombinedOutput(); err != nil {
		t.Fatalf("making a tone: %v: %s", err, out)
	}
	p := New(WithFFmpeg(ffmpeg), WithNullAudio())
	t.Cleanup(p.Stop)
	return p, wav
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func near(got, want time.Duration) bool {
	d := got - want
	return d > -300*time.Millisecond && d < 300*time.Millisecond
}

func TestPlaysAFileToItsEnd(t *testing.T) {
	p, wav := newTestPlayer(t)
	if err := p.Play(wav); err != nil {
		t.Fatal(err)
	}
	if !p.Active() {
		t.Fatal("a started stream is not active")
	}
	waitFor(t, "the track to end", p.Ended)
	if !near(p.Position(), 3*time.Second) {
		t.Errorf("position at the end = %v, want about 3s", p.Position())
	}
	if p.Playing() || p.Failure() != "" {
		t.Errorf("an ended track: playing=%v failure=%q", p.Playing(), p.Failure())
	}
}

func TestPauseHoldsAndResumeFinishes(t *testing.T) {
	p, wav := newTestPlayer(t)
	if err := p.Play(wav); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "sound", func() bool { return p.Position() > 0 })
	p.Pause()
	if !p.Paused() || p.Playing() {
		t.Fatalf("pause left paused=%v playing=%v", p.Paused(), p.Playing())
	}
	time.Sleep(50 * time.Millisecond) // let a chunk in flight land
	held := p.Position()
	time.Sleep(150 * time.Millisecond)
	if p.Position() != held || p.Ended() {
		t.Errorf("a paused stream moved: %v -> %v, ended=%v", held, p.Position(), p.Ended())
	}
	p.Toggle()
	waitFor(t, "the resumed track to end", p.Ended)
}

func TestSeekRestartsTheDecodeAtThatPoint(t *testing.T) {
	p, wav := newTestPlayer(t)
	if err := p.Play(wav); err != nil {
		t.Fatal(err)
	}
	p.Pause()
	p.Seek(2 * time.Second)
	if !p.Paused() {
		t.Error("seeking a paused stream resumed it")
	}
	waitFor(t, "the seek to land", func() bool { return p.Position() >= 2*time.Second })
	p.Resume()
	waitFor(t, "the track to end", p.Ended)
	if !near(p.Position(), 3*time.Second) {
		t.Errorf("position after seeking = %v, want about 3s", p.Position())
	}
}

func TestPlaysOverHTTP(t *testing.T) {
	p, wav := newTestPlayer(t)
	server := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir(wav))))
	defer server.Close()
	if err := p.Play(server.URL + "/tone.wav"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the streamed track to end", p.Ended)
	if !near(p.Position(), 3*time.Second) {
		t.Errorf("streamed position = %v, want about 3s", p.Position())
	}
}

func TestAnUnreachableStreamFails(t *testing.T) {
	p, _ := newTestPlayer(t)
	if err := p.Play(filepath.Join(t.TempDir(), "missing.wav")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the failure", func() bool { return p.Failure() != "" })
	if !p.StreamFailed() {
		t.Error("an unreadable stream was not reported as a stream failure")
	}
}

func TestStopForgetsTheStream(t *testing.T) {
	p, wav := newTestPlayer(t)
	if err := p.Play(wav); err != nil {
		t.Fatal(err)
	}
	p.Stop()
	if p.Active() || p.Position() != 0 {
		t.Errorf("a stopped player: active=%v position=%v", p.Active(), p.Position())
	}
}
