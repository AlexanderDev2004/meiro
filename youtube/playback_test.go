package youtube

import (
	"encoding/json"
	"math"
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
	got, ok := parseLoudness(raw)
	if !ok || math.Abs(got.LoudnessDB-2.290) > 1e-9 {
		t.Fatalf("parseLoudness() = %#v, %t; want 2.290, true", got, ok)
	}
}

func TestParseLoudnessPrefersPerceptualWhenIntegratedLoudnessIsZero(t *testing.T) {
	raw := json.RawMessage(`{"playerConfig":{"audioConfig":{"loudnessDb":0,"perceptualLoudnessDb":-12.5}}}`)
	got, ok := parseLoudness(raw)
	if !ok || got.LoudnessDB != -12.5 {
		t.Fatalf("parseLoudness() = %#v, %t; want -12.5, true", got, ok)
	}
}

func TestParseLoudnessIgnoresUnavailableOrMalformedResponses(t *testing.T) {
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"playabilityStatus":{"status":"UNPLAYABLE"}}`),
		json.RawMessage(`{"playerConfig":{"audioConfig":{}}}`),
		json.RawMessage(`{"streamingData":{"formats":[{"audioTrack":{"loudnessDb":9}}]}}`),
		json.RawMessage(`not json`),
	} {
		if got, ok := parseLoudness(raw); ok || got != (Loudness{}) {
			t.Fatalf("parseLoudness(%s) = %#v, %t; want empty, false", raw, got, ok)
		}
	}
}
