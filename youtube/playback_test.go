package youtube

import (
	"encoding/json"
	"math"
	"testing"
)

func TestParseLoudnessFromWebPlayerResponse(t *testing.T) {
	raw := json.RawMessage(`{
		"playabilityStatus": {"status": "OK"},
		"streamingData": {"formats": [{
			"audioTrack": {"perceptualLoudnessDb": -11.710, "loudnessDb": 2.290}
		}]}
	}`)
	got, ok := parseLoudness(raw)
	if !ok || math.Abs(got.LoudnessDB-2.290) > 1e-9 {
		t.Fatalf("parseLoudness() = %#v, %t; want 2.290, true", got, ok)
	}
}

func TestParseLoudnessIgnoresUnavailableOrMalformedResponses(t *testing.T) {
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"playabilityStatus":{"status":"UNPLAYABLE"}}`),
		json.RawMessage(`{"streamingData":{}}`),
		json.RawMessage(`not json`),
	} {
		if got, ok := parseLoudness(raw); ok || got != (Loudness{}) {
			t.Fatalf("parseLoudness(%s) = %#v, %t; want empty, false", raw, got, ok)
		}
	}
}
