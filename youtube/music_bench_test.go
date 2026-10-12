package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func benchResponse() []byte {
	var b strings.Builder
	b.WriteString(`{"contents":{"sectionListRenderer":{"contents":[`)
	for s := 0; s < 12; s++ {
		if s > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"musicCarouselShelfRenderer":{"header":{"musicCarouselShelfBasicHeaderRenderer":{"title":{"runs":[{"text":"Shelf %d"}]}}},"contents":[`, s)
		for i := 0; i < 20; i++ {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"musicTwoRowItemRenderer":{"title":{"runs":[{"text":"Title %d-%d","navigationEndpoint":{"browseEndpoint":{"browseId":"MPREb_%d_%d"}}}]},"subtitle":{"runs":[{"text":"Artist"}]},"thumbnailRenderer":{"musicThumbnailRenderer":{"thumbnail":{"thumbnails":[{"url":"https://x/a=w60-h60","width":60},{"url":"https://x/a=w544-h544","width":544}]}}},"menu":{"menuRenderer":{"items":[{"menuServiceItemRenderer":{"text":{"runs":[{"text":"Add to queue"}]},"serviceEndpoint":{"queueAddEndpoint":{"queueTarget":{"videoId":"v%d%d"}}}}}]}},"navigationEndpoint":{"browseEndpoint":{"browseId":"MPREb_%d_%d"}}}}`, s, i, s, i, s, i, s, i)
		}
		b.WriteString(`]}}`)
	}
	b.WriteString(`]}}}`)
	return []byte(b.String())
}

// BenchmarkNewBrowseResult is the cost of reading a home page: twelve shelves of
// twenty cards.
func BenchmarkNewBrowseResult(b *testing.B) {
	raw := benchResponse()
	c := NewClient(Options{APIKey: "k"})
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.newBrowseResult(raw)
	}
}

// benchUpNextResponse is a playback queue: one panel of sixty entries, each
// naming its track, artwork, and the mix it belongs to.
func benchUpNextResponse() []byte {
	var b strings.Builder
	b.WriteString(`{"contents":{"playlistPanelRenderer":{"playlistId":"RDAMVMbench","contents":[`)
	for i := 0; i < 60; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"playlistPanelVideoRenderer":{"videoId":"v%[1]d","title":{"runs":[{"text":"Track %[1]d"}]},"longBylineText":{"runs":[{"text":"Artist %[1]d"}]},"thumbnail":{"thumbnails":[{"url":"https://x/v%[1]d=w60-h60","width":60},{"url":"https://x/v%[1]d=w544-h544","width":544}]},"lengthText":{"runs":[{"text":"3:45"}]},"navigationEndpoint":{"watchEndpoint":{"videoId":"v%[1]d","playlistId":"RDAMVMbench"}},"menu":{"menuRenderer":{"items":[{"menuServiceItemRenderer":{"text":{"runs":[{"text":"Save"}]},"serviceEndpoint":{"queueAddEndpoint":{"queueTarget":{"videoId":"v%[1]d"}}}}}]}}}}`, i)
	}
	b.WriteString(`],"continuations":[{"nextRadioContinuationData":{"continuation":"RADIO_MORE"},"nextContinuationData":{"continuation":"STANDARD_MORE"}}]}}}`)
	return []byte(b.String())
}

// BenchmarkNewUpNextResult is the cost of reading a playback queue: one panel of
// sixty entries.
func BenchmarkNewUpNextResult(b *testing.B) {
	raw := benchUpNextResponse()
	c := NewClient(Options{APIKey: "k"})
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.newUpNextResult(raw)
	}
}

// BenchmarkVideoIDs is what a write naming a whole library's worth of songs
// costs to clean up.
func BenchmarkVideoIDs(b *testing.B) {
	videos := make([]string, 5000)
	for i := range videos {
		videos[i] = fmt.Sprintf(" video%d ", i%4500)
	}
	b.ReportAllocs()
	for b.Loop() {
		videoIDs(videos)
	}
}

// BenchmarkExecute is the cost of one API round trip on the client's side,
// over a loopback server that answers with a home page.
func BenchmarkExecute(b *testing.B) {
	raw := benchResponse()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer server.Close()
	c := NewClient(Options{APIKey: "k", BaseURL: server.URL})
	ctx := context.Background()
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := c.execute(ctx, "browse", map[string]any{"browseId": "FEmusic_home"}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetAllLibrary is the wait for a whole library over a connection
// that takes 20ms a request: six shelves, each followed by four more pages.
func BenchmarkGetAllLibrary(b *testing.B) {
	const shelves, pages = 6, 4
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		token, _ := request["continuation"].(string)
		if token == "" {
			var sb strings.Builder
			sb.WriteString(`{"contents":{"sectionListRenderer":{"contents":[`)
			for s := 0; s < shelves; s++ {
				if s > 0 {
					sb.WriteString(",")
				}
				fmt.Fprintf(&sb, `{"musicShelfRenderer":{"title":{"simpleText":"Shelf %d"},"contents":[{"musicResponsiveListItemRenderer":{"videoId":"s%d-0","title":{"simpleText":"T"}}}],"continuations":[{"nextContinuationData":{"continuation":"%d.1"}}]}}`, s, s, s)
			}
			sb.WriteString(`]}}}`)
			_, _ = w.Write([]byte(sb.String()))
			return
		}
		var s, p int
		_, _ = fmt.Sscanf(token, "%d.%d", &s, &p)
		next := ""
		if p < pages {
			next = fmt.Sprintf(`,"continuations":[{"nextContinuationData":{"continuation":"%d.%d"}}]`, s, p+1)
		}
		fmt.Fprintf(w, `{"continuationContents":{"musicShelfContinuation":{"contents":[{"musicResponsiveListItemRenderer":{"videoId":"s%d-%d","title":{"simpleText":"T"}}}]%s}}}`, s, p, next)
	}))
	defer server.Close()
	c := NewClient(Options{APIKey: "k", BaseURL: server.URL})
	var firstPaint time.Duration
	for b.Loop() {
		start := time.Now()
		library, err := c.StreamLibrary(context.Background(), func(*BrowseResult) {
			if firstPaint == 0 || time.Since(start) < firstPaint {
				firstPaint = time.Since(start)
			}
		})
		if err != nil || len(library.Items) != shelves*(pages+1) {
			b.Fatalf("library = %d items, %v", len(library.Items), err)
		}
	}
	b.ReportMetric(float64(firstPaint.Milliseconds()), "ms-to-first-paint")
}
