// Package lastfm scrobbles to Last.fm: the signed API client, the desktop
// authorization flow, the outbox that keeps plays until they are delivered,
// and the tracker that decides when a listen qualifies.
//
// Nothing here knows about the app. Callers do the work off the main thread.
package lastfm

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// APIRoot is the Last.fm web service.
	APIRoot = "https://ws.audioscrobbler.com/2.0/"
	// authRoot is where the user approves a token.
	authRoot = "https://www.last.fm/api/auth/"
	// MaxBatch is how many scrobbles one track.scrobble call takes.
	MaxBatch = 50
	// maxResponse bounds what is read of an answer.
	maxResponse = 1 << 20
)

// Client talks to Last.fm for one app key. The secret of a client that ships
// in a desktop app cannot be kept: signing only keeps requests honest, it
// does not authenticate the app.
type Client struct {
	Key, Secret string
	// Root is the API address; empty is APIRoot.
	Root string
	// HTTP is the client to send with; nil is a client with a timeout.
	HTTP *http.Client
}

// Configured reports whether the client has the app key and secret.
func (c *Client) Configured() bool { return c != nil && c.Key != "" && c.Secret != "" }

// Track is what a play says about the track.
type Track struct {
	Artist, Title, Album string
	// Duration is the track's length; zero leaves it out.
	Duration time.Duration
}

// Valid reports whether the track names a title and an artist, which Last.fm
// needs and which must not be guessed.
func (t Track) Valid() bool {
	return strings.TrimSpace(t.Artist) != "" && strings.TrimSpace(t.Title) != ""
}

// Scrobble is a play to submit: the track, when it started, and whether the
// user chose it.
type Scrobble struct {
	Track
	// Started is when playback began, which Last.fm keeps as the play's time.
	Started time.Time
	// Chosen is Last.fm's chosenByUser: the listener picked this track, rather
	// than the queue or a recommendation moving on to it.
	Chosen bool
}

// Session is what approving a token gives: the key that signs a user's calls.
type Session struct {
	Name, Key string
}

// Error is an error Last.fm answered with.
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("last.fm error %d: %s", e.Code, e.Message) }

// Last.fm's codes that the app acts on; see the API's error list.
const (
	codeAuthFailed       = 4
	codeOperationFailed  = 8
	codeInvalidSession   = 9
	codeServiceOffline   = 11
	codeTemporary        = 16
	codeRateLimited      = 29
	codeTokenNotApproved = 14
)

// InvalidSession reports whether err says the saved session no longer works,
// so the user must connect again.
func InvalidSession(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.Code == codeInvalidSession || e.Code == codeAuthFailed)
}

// NotApproved reports whether err says the token is not approved yet.
func NotApproved(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == codeTokenNotApproved
}

// Transient reports whether a request that failed with err may work when it
// is sent again: the network or the service failed, rather than the request
// being wrong. A session that is invalid is not transient: sending it again
// cannot help until the user connects.
func Transient(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var e *Error
	if errors.As(err, &e) {
		switch e.Code {
		case codeOperationFailed, codeServiceOffline, codeTemporary, codeRateLimited:
			return true
		}
		return false
	}
	var status *statusError
	if errors.As(err, &status) {
		return status.code >= 500 || status.code == http.StatusTooManyRequests
	}
	return true // the network: no answer at all
}

type statusError struct{ code int }

func (e *statusError) Error() string { return "last.fm answered HTTP " + strconv.Itoa(e.code) }

// Sign is the api_sig of a call: the MD5 of its parameters in name order,
// each as name then value, followed by the secret. format and callback stay
// out of it.
func Sign(params url.Values, secret string) string {
	names := make([]string, 0, len(params))
	for name := range params {
		if name != "format" && name != "callback" && name != "api_sig" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteString(params.Get(name))
	}
	b.WriteString(secret)
	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// AuthURL is the page where the user approves token.
func (c *Client) AuthURL(token string) string {
	return authRoot + "?api_key=" + url.QueryEscape(c.Key) + "&token=" + url.QueryEscape(token)
}

// Token asks for a request token to send the user to approve.
func (c *Client) Token(ctx context.Context) (string, error) {
	var out struct {
		Token string `json:"token"`
	}
	if err := c.call(ctx, "auth.getToken", url.Values{}, "", &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("last.fm sent no token")
	}
	return out.Token, nil
}

// Session exchanges an approved token for the user's session. It fails with
// an error NotApproved says yes to until the user has approved the token.
func (c *Client) Session(ctx context.Context, token string) (Session, error) {
	var out struct {
		Session struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"session"`
	}
	if err := c.call(ctx, "auth.getSession", url.Values{"token": {token}}, "", &out); err != nil {
		return Session{}, err
	}
	if out.Session.Key == "" {
		return Session{}, errors.New("last.fm sent no session")
	}
	return Session{Name: out.Session.Name, Key: out.Session.Key}, nil
}

// UpdateNowPlaying tells Last.fm what the user is listening to now.
func (c *Client) UpdateNowPlaying(ctx context.Context, sessionKey string, t Track) error {
	params := url.Values{}
	setTrack(params, "", t)
	return c.call(ctx, "track.updateNowPlaying", params, sessionKey, nil)
}

// Scrobble submits plays, at most MaxBatch at a time.
func (c *Client) Scrobble(ctx context.Context, sessionKey string, plays []Scrobble) error {
	if len(plays) == 0 {
		return nil
	}
	if len(plays) > MaxBatch {
		return fmt.Errorf("last.fm takes %d scrobbles at a time, not %d", MaxBatch, len(plays))
	}
	params := url.Values{}
	for i, play := range plays {
		suffix := "[" + strconv.Itoa(i) + "]"
		setTrack(params, suffix, play.Track)
		params.Set("timestamp"+suffix, strconv.FormatInt(play.Started.Unix(), 10))
		chosen := "0"
		if play.Chosen {
			chosen = "1"
		}
		params.Set("chosenByUser"+suffix, chosen)
	}
	return c.call(ctx, "track.scrobble", params, sessionKey, nil)
}

func setTrack(params url.Values, suffix string, t Track) {
	params.Set("artist"+suffix, t.Artist)
	params.Set("track"+suffix, t.Title)
	if t.Album != "" {
		params.Set("album"+suffix, t.Album)
	}
	if seconds := int(t.Duration.Round(time.Second) / time.Second); seconds > 0 {
		params.Set("duration"+suffix, strconv.Itoa(seconds))
	}
}

// call sends one signed call and decodes the answer into out, if it is not
// nil. A call with a session key is a write, which Last.fm takes by POST.
func (c *Client) call(ctx context.Context, method string, params url.Values, sessionKey string, out any) error {
	if !c.Configured() {
		return errors.New("last.fm is not set up in this build")
	}
	form := url.Values{}
	for name, values := range params {
		form[name] = values
	}
	form.Set("method", method)
	form.Set("api_key", c.Key)
	if sessionKey != "" {
		form.Set("sk", sessionKey)
	}
	form.Set("api_sig", Sign(form, c.Secret))
	form.Set("format", "json")

	root := c.Root
	if root == "" {
		root = APIRoot
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, root, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponse))
	if err != nil {
		return err
	}
	// Last.fm reports its errors in the body, often with a 4xx status, so the
	// body is read first.
	var failure struct {
		Code    int    `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &failure) == nil && failure.Code != 0 {
		return &Error{Code: failure.Code, Message: failure.Message}
	}
	if response.StatusCode != http.StatusOK {
		return &statusError{response.StatusCode}
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("reading last.fm's answer: %w", err)
		}
	}
	return nil
}
