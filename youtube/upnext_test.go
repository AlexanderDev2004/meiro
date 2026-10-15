package youtube

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetUpNextSeedsALoneTrackWithItsOwnMix(t *testing.T) {
	var payloads []map[string]any
	client := NewClient(Options{HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		payloads = append(payloads, payload)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
	})}})
	index := 3
	for _, options := range []UpNextOptions{
		{VideoID: "vid"},
		{VideoID: "vid", PlaylistID: "PLx", PlaylistIndex: &index},
	} {
		if _, err := client.GetUpNext(context.Background(), options); err != nil {
			t.Fatal(err)
		}
	}
	if payloads[0]["playlistId"] != "RDAMVMvid" {
		t.Errorf("a lone track is not seeded with its own mix: %v", payloads[0])
	}
	if payloads[1]["playlistId"] != "PLx" || payloads[1]["playlistIndex"] != float64(3) {
		t.Errorf("a playlist's own queue was replaced: %v", payloads[1])
	}
}

func TestLoudnessUsesTheSignedInWebPlayer(t *testing.T) {
	auth, err := NewCookieAuth("SAPISID=test", CookieOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	client := NewClient(Options{CookieAuth: auth, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return nil, err
		}
		body := `{"playabilityStatus":{"status":"OK"},"streamingData":{"formats":[{"audioTrack":{"loudnessDb":2.290}}]}}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	got, err := client.Loudness(context.Background(), "vid")
	if err != nil {
		t.Fatal(err)
	}
	if got.LoudnessDB != 2.290 {
		t.Errorf("loudness = %#v, want 2.290", got)
	}
	contextPayload, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("request context = %#v", payload["context"])
	}
	clientPayload, ok := contextPayload["client"].(map[string]any)
	if !ok || clientPayload["clientName"] != "WEB" || payload["videoId"] != "vid" {
		t.Errorf("web player request = %#v", payload)
	}
}

func TestReportPlaybackPingsTheTrackingURL(t *testing.T) {
	auth, err := NewCookieAuth("SAPISID=test", CookieOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var ping *http.Request
	client := NewClient(Options{CookieAuth: auth, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"playbackTracking":{"videostatsPlaybackUrl":{"baseUrl":"https://s.youtube.com/api/stats/playback?docid=vid"}}}`
		if r.Method == http.MethodGet {
			ping, body = r, ""
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	if err := client.ReportPlayback(context.Background(), "vid"); err != nil {
		t.Fatal(err)
	}
	q := ping.URL.Query()
	if q.Get("docid") != "vid" || q.Get("ver") != "2" || q.Get("c") != "WEB_REMIX" || len(q.Get("cpn")) != 16 || ping.Header.Get("Cookie") == "" {
		t.Errorf("ping %s, cookie %q", ping.URL, ping.Header.Get("Cookie"))
	}
	if err := NewClient(Options{}).ReportPlayback(context.Background(), "vid"); err != nil {
		t.Errorf("signed out must do nothing: %v", err)
	}
}
