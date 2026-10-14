package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/elianiva/meiro/internal/playstate"
)

func TestMiniPlayerShowsAndControlsWhatPlays(t *testing.T) {
	a := newTestApp()
	tt := ui.NewTester(a.miniPlayer, miniWidth, miniHeight)
	tt.Frame()
	if !tt.HasText("Nothing playing") {
		t.Fatalf("an idle player says nothing about it: %q", tt.Texts())
	}

	a.queue.restore(playstate.State{Items: tracks("a", "b"), Index: 0, RecStart: -1})
	a.current = a.queue.current()
	tt.Frame()
	if !tt.HasText("Track a") || tt.HasText("Nothing playing") {
		t.Fatalf("the playing track is not shown: %q", tt.Texts())
	}
	if err := tt.Click("Next"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.current.VideoID != "b" || !tt.HasText("Track b") {
		t.Fatalf("Next did not move on: current %q, shown %q", a.current.VideoID, tt.Texts())
	}
}

func TestMiniPlayerExpandsToTabs(t *testing.T) {
	a := newTestApp()
	a.queue.restore(playstate.State{Items: tracks("a", "b"), Index: 0, RecStart: -1})
	a.current = a.queue.current()
	tt := ui.NewTester(a.miniPlayer, miniWidth, miniTallHeight)
	tt.Frame()
	if tt.HasText("Lyrics") {
		t.Fatalf("tabs show before they are asked for: %q", tt.Texts())
	}
	if err := tt.Click("Show up next, lyrics and related"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Lyrics") || !tt.HasText("Related") || !tt.HasText("Track b") {
		t.Fatalf("the panel is not shown: %q", tt.Texts())
	}
	if err := tt.Click("Lyrics"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.nowPlaying.lyrics.videoID != "a" {
		t.Fatalf("lyrics were not loaded for the playing track: %q", a.nowPlaying.lyrics.videoID)
	}
}
