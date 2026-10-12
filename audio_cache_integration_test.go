package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// The audio cache downloads real bytes from a media server, keeps the most
// recent tracks within its limit, survives a restart, and lets the app play a
// cached track without asking the resolver again.
func TestAudioCacheFromDownloadToReplay(t *testing.T) {
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gone":
			http.Error(w, "expired", http.StatusForbidden)
		default:
			w.Header().Set("Content-Type", "audio/mp4")
			_, _ = w.Write([]byte("audio for " + r.URL.Path))
		}
	}))
	defer media.Close()

	dir := t.TempDir()
	cache, err := newAudioCache(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	cached := func(id string) bool { _, ok := cache.get(id); return ok }
	waitCached := func(id string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !cached(id) {
			if time.Now().After(deadline) {
				t.Fatalf("%s was never cached", id)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	cache.enqueueStream("one", media.URL+"/one")
	waitCached("one")
	cache.enqueueStream("two", media.URL+"/two")
	waitCached("two")
	path, _ := cache.get("two")
	if !strings.HasSuffix(path, ".m4a") {
		t.Errorf("a track served as audio/mp4 was kept as %q", path)
	}
	if data, _ := os.ReadFile(path); string(data) != "audio for /two" {
		t.Errorf("cached bytes = %q", data)
	}

	// A third track pushes out the least recently used one, which is "one"
	// now that "two" was just read.
	cache.get("two")
	cache.enqueueStream("three", media.URL+"/three")
	waitCached("three")
	if cached("one") || !cached("two") {
		t.Errorf("eviction kept one=%v two=%v", cached("one"), cached("two"))
	}

	// An expired URL caches nothing and leaves nothing half written.
	cache.enqueueStream("gone", media.URL+"/gone")
	cache.close()
	if cached("gone") {
		t.Error("a refused download was cached")
	}

	// A restart finds the files that were kept.
	again, err := newAudioCache(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer again.close()
	if _, ok := again.get("three"); !ok {
		t.Error("the cache lost its files across a restart")
	}

	// The app plays a cached track without resolving it.
	a, r := playbackApp(t)
	a.audioCache = again
	a.play(tracks("three")[0], nil, 0)
	if r.asked("three") {
		t.Error("a cached track was resolved anyway")
	}
	if a.current.VideoID != "three" || a.resolving {
		t.Errorf("current=%q resolving=%v", a.current.VideoID, a.resolving)
	}
}
