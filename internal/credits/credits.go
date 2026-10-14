// Package credits remembers who made a track, by video ID, between runs.
//
// YouTube Music's lists often leave the artist out of a row, since the page
// the row is on already says it, but the same track in another list, or the
// track's own page, names it. Whatever names an artist is kept here, so the
// next time a track turns up without one it is not asked for again.
package credits

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"slices"
	"sync"

	"github.com/elianiva/meiro/internal/fsutil"
)

// MaxEntries bounds the cache; the oldest entries go first. An entry is a few
// dozen bytes, so this is well under a megabyte on disk.
const MaxEntries = 5000

// Credits is who made a track and the album it is on.
type Credits struct {
	Artist string `json:"artist"`
	Album  string `json:"album,omitempty"`
}

type entry struct {
	ID string `json:"id"`
	Credits
}

// Cache is the credits known, safe for concurrent use. Changes are in memory
// at once and written by a goroutine of the cache's own. The zero value keeps
// them in memory only.
type Cache struct {
	path string

	mu      sync.Mutex
	known   map[string]Credits
	order   []string // oldest first
	dirty   bool
	running bool
	done    chan struct{}
}

// Open reads the cache kept at path. A file that is missing or unreadable is
// an empty cache.
func Open(path string) *Cache {
	c := &Cache{path: path, known: map[string]Credits{}}
	if path == "" {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("credits: reading %s: %v", path, err)
		}
		return c
	}
	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Printf("credits: %s is not readable, starting fresh: %v", path, err)
		return c
	}
	for _, e := range entries {
		c.putLocked(e.ID, e.Credits)
	}
	return c
}

// Get is what is known of a track's credits.
func (c *Cache) Get(id string) (Credits, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	got, ok := c.known[id]
	return got, ok
}

// Put keeps a track's credits. Credits without an artist say nothing and are
// not kept.
func (c *Cache) Put(id string, credits Credits) {
	if id == "" || credits.Artist == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.known == nil {
		c.known = map[string]Credits{}
	}
	if c.known[id] == credits {
		return
	}
	c.putLocked(id, credits)
	c.changedLocked()
}

// Len is how many tracks are known.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.known)
}

func (c *Cache) putLocked(id string, credits Credits) {
	if _, ok := c.known[id]; ok {
		c.order = slices.DeleteFunc(c.order, func(have string) bool { return have == id })
	}
	c.known[id] = credits
	c.order = append(c.order, id)
	for len(c.order) > MaxEntries {
		delete(c.known, c.order[0])
		c.order = c.order[1:]
	}
}

func (c *Cache) changedLocked() {
	if c.path == "" {
		return
	}
	c.dirty = true
	if c.running {
		return
	}
	c.running, c.done = true, make(chan struct{})
	go c.drain(c.done)
}

func (c *Cache) drain(done chan struct{}) {
	defer close(done)
	for {
		c.mu.Lock()
		if !c.dirty {
			c.running = false
			c.mu.Unlock()
			return
		}
		entries := make([]entry, 0, len(c.order))
		for _, id := range c.order {
			entries = append(entries, entry{id, c.known[id]})
		}
		c.dirty = false
		c.mu.Unlock()
		data, err := json.Marshal(entries)
		if err == nil {
			err = fsutil.WriteFileAtomic(c.path, data, 0o600)
		}
		if err != nil {
			log.Printf("credits: saving: %v", err)
		}
	}
}

// Wait returns once what changed so far is on disk.
func (c *Cache) Wait() {
	c.mu.Lock()
	done, running := c.done, c.running
	c.mu.Unlock()
	if running {
		<-done
	}
}
