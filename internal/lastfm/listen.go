package lastfm

import "time"

const (
	// MinTrack is the shortest track Last.fm counts: it must be longer.
	MinTrack = 30 * time.Second
	// MaxThreshold is the listening that qualifies a track however long it is.
	MaxThreshold = 4 * time.Minute

	// maxGap is the longest wait between two samples that still counts as
	// listening through them. A longer one is a pause, a stall, or the machine
	// asleep, and the time in it is not credited.
	maxGap = 2 * time.Second
	// slack is how much further than the wait the position may move and still
	// be playing: the position is read a few tens of milliseconds apart from
	// the clock.
	slack = 500 * time.Millisecond
)

// Threshold is how long a track of the given length must be listened to
// before it is scrobbled, and false when its length is too short or not known:
// Last.fm counts only tracks longer than 30 seconds, and a play that qualifies
// has been listened to for half the track or four minutes, whichever comes
// first.
func Threshold(total time.Duration) (time.Duration, bool) {
	if total <= MinTrack {
		return 0, false
	}
	return min(total/2, MaxThreshold), true
}

// Event is what a sample of the playing track made true.
type Event uint8

const (
	// Nothing is no news.
	Nothing Event = iota
	// Started says playback of the track is confirmed: now playing.
	Started
	// Qualified says the track has been listened to for long enough: scrobble.
	Qualified
)

// Listen follows one play of one track. It counts what was listened to from
// the positions samples report, not from the clock: a pause, buffering or a
// seek forward must not make a track qualify.
//
// One Listen is one play. Repeating a track, or moving to another, starts a
// new one. The zero value follows nothing.
type Listen struct {
	// Track and Chosen are what the play is scrobbled as.
	Track  Track
	Chosen bool

	confirmed bool
	started   time.Time
	listened  time.Duration
	scrobbled bool

	haveLast bool
	lastAt   time.Time
	lastPos  time.Duration
}

// NewListen begins following a play. A track without a clear title and artist
// is followed to nothing: no event ever comes of it.
func NewListen(track Track, chosen bool) *Listen {
	return &Listen{Track: track, Chosen: chosen}
}

// Break says the sound stopped flowing between samples, as it does while the
// player is paused, buffering or moving to another position. The next sample
// only sets where counting resumes from.
func (l *Listen) Break() {
	if l != nil {
		l.haveLast = false
	}
}

// Listened is how long the track has been listened to.
func (l *Listen) Listened() time.Duration { return l.listened }

// Observe takes a sample of the track playing: when it was taken, the player's
// position, and the track's length, if known. It reports what became true, if
// anything.
func (l *Listen) Observe(at time.Time, position, total time.Duration) Event {
	if l == nil || !l.Track.Valid() {
		return Nothing
	}
	if !l.confirmed {
		l.confirmed, l.started = true, at
		l.setLast(at, position)
		return Started
	}
	if l.haveLast {
		wait, moved := at.Sub(l.lastAt), position-l.lastPos
		if wait > 0 && wait <= maxGap && moved >= 0 && moved <= wait+slack {
			l.listened += moved
		}
	}
	l.setLast(at, position)
	if l.scrobbled {
		return Nothing
	}
	if need, ok := Threshold(total); ok && l.listened >= need {
		l.scrobbled = true
		return Qualified
	}
	return Nothing
}

func (l *Listen) setLast(at time.Time, position time.Duration) {
	l.haveLast, l.lastAt, l.lastPos = true, at, position
}

// Started is when playback was confirmed.
func (l *Listen) Started() time.Time { return l.started }

// Scrobble is the play as it is submitted. Total is the track's length.
func (l *Listen) Scrobble(total time.Duration) Scrobble {
	track := l.Track
	track.Duration = total
	return Scrobble{Track: track, Started: l.started, Chosen: l.Chosen}
}
