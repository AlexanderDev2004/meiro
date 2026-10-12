package main

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/elianiva/meiro/youtube"
)

// pageBench runs b.N frames of a page at each size, counting the artwork
// lookups a frame makes. setup shows the page with n entries.
func pageBench(b *testing.B, sizes []int, setup func(a *app, n int)) {
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			a := newTestApp()
			a.signedIn = true
			asked := 0
			art := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 8, 8)))
			a.thumbs.synth = func(string, int) *ui.Bitmap { asked++; return art }
			tt := ui.NewTester(a.view, 1200, 760)
			setup(a, n)
			for range 40 { // let a page's enter transition finish
				tt.Frame()
			}
			asked = 0
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tt.Frame()
			}
			b.ReportMetric(float64(asked)/float64(b.N), "art-asks/frame")
		})
	}
}

func openPlayer(a *app, tab int) {
	a.current = benchTracks(1)[0]
	a.npOpen, a.npTab = true, tab
}

func BenchmarkQueuePanelFrame(b *testing.B) {
	pageBench(b, []int{100, 1000, 5000}, func(a *app, n int) {
		openPlayer(a, 0)
		a.queue = benchTracks(n)
		a.current = a.queue[0]
	})
}

func BenchmarkRelatedPanelFrame(b *testing.B) {
	pageBench(b, []int{50, 500, 2000}, func(a *app, n int) {
		openPlayer(a, 2)
		a.related = relatedState{videoID: "v0", items: benchTracks(n)}
	})
}

func BenchmarkLyricsPanelFrame(b *testing.B) {
	pageBench(b, []int{40, 400, 4000}, func(a *app, n int) {
		openPlayer(a, 1)
		a.lyrics.text = strings.Repeat("a line of the lyrics that goes on a little\n", n)
	})
}

func BenchmarkPlaylistPageFrame(b *testing.B) {
	pageBench(b, []int{100, 1000, 5000}, func(a *app, n int) {
		a.router.Push("/playlist/PL1")
		a.location = "/playlist/PL1"
		a.detail = detail{title: "Mix", kind: pagePlaylist}
		a.feed = pageState{sections: []youtube.MusicSection{{Items: benchTracks(n)}}}
	})
}

func BenchmarkSearchPageFrame(b *testing.B) {
	pageBench(b, []int{50, 500, 2000}, func(a *app, n int) {
		a.router.Push("/search")
		a.location = "/search"
		a.search.submitted = "x"
		a.search.query = "x"
		items := benchTracks(n)
		for i := 0; i < n; i += 3 {
			items[i] = benchAlbum(i)
		}
		a.search.items = items
	})
}

func BenchmarkAddToPlaylistFrame(b *testing.B) {
	pageBench(b, []int{20, 200, 1000}, func(a *app, n int) {
		a.router.Push("/home")
		a.location = "/home"
		playlists := make([]youtube.MusicItem, n)
		for i := range playlists {
			playlists[i] = benchAlbum(i)
			playlists[i].Kind = "playlist"
			playlists[i].PlaylistID = fmt.Sprint("PL", i)
		}
		a.addToPlaylist = addToPlaylistState{open: true, item: benchTracks(1)[0], playlists: playlists}
	})
}

func BenchmarkStatsPageFrame(b *testing.B) {
	pageBench(b, []int{500, 5000}, func(a *app, n int) {
		a.router.Push("/stats")
		a.location = "/stats"
		a.historyLoaded = true
		a.history = benchHistory(n, time.Now())
	})
}

func BenchmarkRecentSearchesFrame(b *testing.B) {
	pageBench(b, []int{10, 100}, func(a *app, n int) {
		a.router.Push("/search")
		a.location = "/search"
		for i := 0; i < n; i++ {
			a.settings.Recent = append(a.settings.Recent, fmt.Sprint("query ", i))
		}
	})
}
