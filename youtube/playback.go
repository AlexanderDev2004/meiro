package youtube

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// ReportPlayback tells YouTube Music that the account started playing a track,
// as its own player does. That is what puts the track in the listening
// history, and the history is what the recommendations are made from; Meiro
// plays through yt-dlp, so nothing else reports it. It needs a signed-in
// session, and does nothing without one.
func (c *Client) ReportPlayback(ctx context.Context, videoID string) error {
	if c.cookieAuth == nil || strings.TrimSpace(videoID) == "" {
		return nil
	}
	raw, err := c.execute(ctx, "player", map[string]any{
		"videoId":         videoID,
		"playbackContext": map[string]any{"contentPlaybackContext": map[string]any{}},
	})
	if err != nil {
		return err
	}
	tracking := findRenderer(decodeResponse(raw), "videostatsPlaybackUrl")
	base, _ := tracking.get("baseUrl").(string)
	if base == "" {
		return errors.New("youtube: player response has no playback tracking URL")
	}
	target, err := url.Parse(base)
	if err != nil {
		return err
	}
	if !c.allowInsecureCookieAuth && !isYouTubeURL(target) {
		return errors.New("youtube: refusing to send cookie authentication to a non-YouTube or non-HTTPS URL")
	}
	query := target.Query()
	query.Set("ver", "2")
	query.Set("c", defaultMusicContext)
	query.Set("cpn", clientPlaybackNonce())
	target.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Referer", "https://music.youtube.com/")
	c.setIdentity(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError("playback", resp)
	}
	return nil
}

// clientPlaybackNonce is the 16 characters the player sends to tell one
// playback from another.
func clientPlaybackNonce() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[b[i]&63]
	}
	return string(b)
}
