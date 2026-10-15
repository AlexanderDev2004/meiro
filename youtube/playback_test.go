package youtube

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestParseLoudnessFromWebPlayerResponse(t *testing.T) {
	raw := json.RawMessage(`{
		"playabilityStatus": {"status": "OK"},
		"playerConfig": {"audioConfig": {
			"perceptualLoudnessDb": -11.710,
			"loudnessDb": 2.290
		}},
		"streamingData": {"formats": [{
			"audioTrack": {"id": "original", "displayName": "Original"}
		}]}
	}`)
	got, err := parseLoudness(raw)
	if err != nil || math.Abs(got.LoudnessDB-2.290) > 1e-9 {
		t.Fatalf("parseLoudness() = %#v, %v; want 2.290, nil", got, err)
	}
}

func TestParseLoudnessPrefersPerceptualWhenIntegratedLoudnessIsZero(t *testing.T) {
	raw := json.RawMessage(`{"playerConfig":{"audioConfig":{"loudnessDb":0,"perceptualLoudnessDb":-12.5}}}`)
	got, err := parseLoudness(raw)
	if err != nil || got.LoudnessDB != -12.5 {
		t.Fatalf("parseLoudness() = %#v, %v; want -12.5, nil", got, err)
	}
}

func TestLoudnessPropagatesMissingMetadataError(t *testing.T) {
	auth, err := NewCookieAuth("SAPISID=test", CookieOptions{})
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(Options{CookieAuth: auth, HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"playabilityStatus":{"status":"OK"},"playerConfig":{"audioConfig":{}}}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}})
	if got, err := client.Loudness(context.Background(), "vid"); err == nil || got != (Loudness{}) {
		t.Fatalf("Loudness() without metadata = %#v, %v; want empty and an error", got, err)
	}
}

func TestLoudnessRequiresSignedInSession(t *testing.T) {
	if got, err := NewClient(Options{}).Loudness(context.Background(), "vid"); err == nil || got != (Loudness{}) {
		t.Fatalf("Loudness() without auth = %#v, %v; want empty and an error", got, err)
	}
}

func TestParseLoudnessRejectsUnavailableOrMalformedResponses(t *testing.T) {
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"playabilityStatus":{"status":"UNPLAYABLE"}}`),
		json.RawMessage(`{"playerConfig":{"audioConfig":{}}}`),
		json.RawMessage(`{"streamingData":{"formats":[{"audioTrack":{"loudnessDb":9}}]}}`),
		json.RawMessage(`not json`),
	} {
		if got, err := parseLoudness(raw); err == nil || got != (Loudness{}) {
			t.Errorf("parseLoudness(%s) = %#v, %v; want empty and an error", raw, got, err)
		}
	}
}
