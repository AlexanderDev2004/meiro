package lastfm

import (
	"errors"
	"os"
	"strings"

	"github.com/elianiva/meiro/internal/fsutil"
)

// ErrNoSession reports that no session is saved.
var ErrNoSession = errors.New("not connected to Last.fm")

// SessionStore keeps the session key in a file only its user can read, apart
// from the settings, which are not private. Creating one does no work.
type SessionStore struct {
	Path string
}

// Load reads the saved session key.
func (s *SessionStore) Load() (string, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoSession
	}
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", ErrNoSession
	}
	return key, nil
}

// Save keeps a session key.
func (s *SessionStore) Save(key string) error {
	return fsutil.WriteFileAtomic(s.Path, []byte(key), 0o600)
}

// Delete forgets the saved session key.
func (s *SessionStore) Delete() error {
	if err := os.Remove(s.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
