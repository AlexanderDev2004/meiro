package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/elianiva/meiro/youtube"
)

// resolved records the tracks the app asked its stream resolver for, in place
// of yt-dlp.
type resolved struct {
	mu  sync.Mutex
	ids []string
	err map[string]error
	// file, when set, is the audio every track resolves to.
	file string
}

func (r *resolved) resolve(_ context.Context, videoID, _ string) (string, time.Duration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, videoID)
	if err := r.err[videoID]; err != nil {
		return "", 0, err
	}
	if r.file != "" {
		return r.file, 3 * time.Second, nil
	}
	return "/nonexistent/" + videoID + ".m4a", 3 * time.Minute, nil
}

// asked reports whether the resolver was asked for videoID. The app also
// resolves the next track ahead of time, so the last ask is not the track
// playing.
func (r *resolved) count(videoID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, id := range r.ids {
		if id == videoID {
			n++
		}
	}
	return n
}

func (r *resolved) asked(videoID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range r.ids {
		if id == videoID {
			return true
		}
	}
	return false
}

// playbackApp is an app whose tracks resolve to fake files, for the tests of
// what the app decides. TestPlayingATrackThroughToTheEnd plays real audio.
func playbackApp(t *testing.T) (*app, *resolved) {
	t.Helper()
	a := newTestApp()
	a.settings.AutoPlay = false
	r := &resolved{err: map[string]error{}}
	a.streams = newStreamCache(r.resolve)
	a.audioCache = nil
	t.Cleanup(a.player.Stop)
	return a, r
}

func tracks(ids ...string) []youtube.MusicItem {
	items := make([]youtube.MusicItem, len(ids))
	for i, id := range ids {
		items[i] = youtube.MusicItem{VideoID: id, Title: "Track " + id}
	}
	return items
}

func ids(items []youtube.MusicItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.VideoID
	}
	return out
}

func TestClickingATrackPlaysIt(t *testing.T) {
	a, r := playbackApp(t)
	a.router.Push("/album/MPREb_test")
	tt := ui.NewTester(a.view, 1000, 700)
	tt.Frame()
	if err := tt.Click("Album Track One"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.current.VideoID != "vid-3" || !r.asked("vid-3") {
		t.Fatalf("current %q, resolver asked for vid-3: %v", a.current.VideoID, r.asked("vid-3"))
	}
	if a.total != 3*time.Minute {
		t.Errorf("the track length was %v, want the resolver's 3m", a.total)
	}
	if !tt.HasText("Track") && !tt.HasText("Album Track One") {
		t.Errorf("the player bar does not show the track: %q", tt.Texts())
	}
}

func TestQueueAdvancesPreviousAndStops(t *testing.T) {
	a, r := playbackApp(t)
	q := tracks("a", "b", "c")
	a.play(q[0], q, 0)

	a.trackEnded()
	if a.current.VideoID != "b" || !r.asked("b") {
		t.Fatalf("after one track current=%q", a.current.VideoID)
	}
	a.trackEnded()
	a.previous() // just started: goes back a track
	if a.current.VideoID != "b" || a.index != 1 {
		t.Fatalf("previous left index %d on %q", a.index, a.current.VideoID)
	}
	a.trackEnded()
	if a.current.VideoID != "c" {
		t.Fatalf("current = %q", a.current.VideoID)
	}
	a.trackEnded() // the end of the queue, nothing repeats
	if a.current.VideoID != "c" || a.resolving {
		t.Errorf("the queue end: current=%q resolving=%v", a.current.VideoID, a.resolving)
	}
}

func TestRepeatModes(t *testing.T) {
	a, _ := playbackApp(t)
	q := tracks("a", "b")
	a.play(q[0], q, 0)

	a.cycleRepeat() // queue
	a.trackEnded()
	a.trackEnded()
	if a.current.VideoID != "a" {
		t.Errorf("repeating the queue ended on %q, want a", a.current.VideoID)
	}
	a.cycleRepeat() // track
	before := a.streamGen
	a.trackEnded()
	if a.current.VideoID != "a" || a.streamGen == before {
		t.Errorf("repeating the track: current=%q, restarted=%v", a.current.VideoID, a.streamGen != before)
	}
	a.cycleRepeat() // off
	if a.repeat != repeatOff {
		t.Errorf("a third press left repeat %d", a.repeat)
	}
}

func TestShuffleKeepsTheTrackPlayingAndEveryTrack(t *testing.T) {
	a, _ := playbackApp(t)
	q := tracks("a", "b", "c", "d", "e", "f", "g", "h")
	a.play(q[0], q, 0)
	a.setShuffle(true)
	if a.queue[0].VideoID != "a" || a.index != 0 {
		t.Errorf("shuffle moved the track playing: index %d", a.index)
	}
	if len(a.queue) != len(q) {
		t.Fatalf("shuffle changed the queue length: %v", ids(a.queue))
	}
	seen := map[string]bool{}
	for _, id := range ids(a.queue) {
		seen[id] = true
	}
	if len(seen) != len(q) {
		t.Errorf("shuffle lost or repeated tracks: %v", ids(a.queue))
	}
}

func TestQueueingATrack(t *testing.T) {
	a, _ := playbackApp(t)
	q := tracks("a", "b", "c")
	a.play(q[0], q, 0)
	extra := tracks("x", "y")

	a.enqueue(extra[0], false)
	a.enqueue(extra[1], true)
	if got := ids(a.queue); len(got) != 5 || got[1] != "y" || got[4] != "x" {
		t.Fatalf("queue = %v, want y next and x last", got)
	}
	a.trackEnded()
	if a.current.VideoID != "y" {
		t.Errorf("the track queued to play next played %q", a.current.VideoID)
	}
}

func TestEnqueueOnAnIdleAppStartsPlaying(t *testing.T) {
	a, r := playbackApp(t)
	a.enqueue(tracks("solo")[0], false)
	if a.current.VideoID != "solo" || !r.asked("solo") {
		t.Errorf("current=%q, asked=%v", a.current.VideoID, r.asked("solo"))
	}
}

func TestAnUnplayableTrackSaysWhyAndRetries(t *testing.T) {
	a, r := playbackApp(t)
	r.err["bad"] = errors.New("video unavailable")
	a.play(tracks("bad")[0], nil, 0)
	if a.playErr != "video unavailable" || a.resolving {
		t.Fatalf("playErr=%q resolving=%v", a.playErr, a.resolving)
	}
	// Pressing play asks for the same track again.
	delete(r.err, "bad")
	a.togglePlay()
	if r.count("bad") != 2 || a.playErr == "video unavailable" {
		t.Errorf("play did not resolve the track again: %d resolves, playErr %q", r.count("bad"), a.playErr)
	}
}

func TestVolumeAndMutePersistToSettings(t *testing.T) {
	a, _ := playbackApp(t)
	a.setVolume(30)
	if a.settings.Volume != 30 || a.player.Volume() != 0.3 {
		t.Errorf("volume: settings %v, player %v", a.settings.Volume, a.player.Volume())
	}
	a.toggleMute()
	if a.player.Volume() != 0 {
		t.Errorf("mute left the player at %v", a.player.Volume())
	}
	a.toggleMute()
	if a.settings.Volume != 40 { // unmuting never comes back quieter than 40
		t.Errorf("unmuting restored %v, want 40", a.settings.Volume)
	}
	a.setVolume(500)
	if a.settings.Volume != 100 {
		t.Errorf("volume is not clamped: %v", a.settings.Volume)
	}
}

func TestAutoPlayAsksYouTubeForWhatFollows(t *testing.T) {
	a, _ := playbackApp(t)
	a.settings.AutoPlay = true
	q := tracks("a")
	a.play(q[0], q, 0)
	if !a.upNextFetched && !a.upNextLoading {
		t.Error("autoplay never asked YouTube Music for an up-next queue")
	}
}

// A queue played the way the app runs it: real ffmpeg decoding real audio,
// the ticker's poll moving on from each track to the next.
func TestPlayingATrackThroughToTheEnd(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	tone := filepath.Join(t.TempDir(), "tone.wav")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=duration=1", tone).CombinedOutput(); err != nil {
		t.Fatalf("making a tone: %v: %s", err, out)
	}
	a, r := playbackApp(t)
	r.file = tone
	q := tracks("a", "b")
	a.play(q[0], q, 0)

	deadline := time.Now().Add(15 * time.Second)
	heard := map[string]bool{}
	for time.Now().Before(deadline) {
		a.poll()
		if a.player.Position() > 0 {
			heard[a.current.VideoID] = true
		}
		if a.index == 1 && heard["b"] && !a.player.Active() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.index != 1 || !heard["a"] || !heard["b"] || a.player.Active() {
		t.Fatalf("heard %v, index %d, active %v: want both tracks played and the queue stopped", heard, a.index, a.player.Active())
	}
	if a.playErr != "" {
		t.Errorf("playErr = %q", a.playErr)
	}

	a.togglePlay() // pressing play on an ended track starts it again
	waitPlaying := time.Now().Add(5 * time.Second)
	for !a.player.Playing() && time.Now().Before(waitPlaying) {
		a.poll()
		time.Sleep(10 * time.Millisecond)
	}
	if !a.player.Playing() {
		t.Error("play did not restart the ended track")
	}
	a.togglePlay()
	if !a.player.Paused() {
		t.Error("play did not pause a playing track")
	}
}
