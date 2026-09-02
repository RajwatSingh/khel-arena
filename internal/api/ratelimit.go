package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// Rate limiting for the two endpoints an attacker can grind on: login, which
// is the online password-guessing target, and password/forgot, which sprays
// somebody's inbox on request.
//
// **This is per process.** A token bucket in memory does not survive a second
// instance of the API -- run three and an attacker gets three times the
// allowance. That is a real limitation and the reason to reach for a shared
// counter (Redis, or Postgres) when this runs on more than one box. It is
// still worth having now: a single instance is what runs today, and the
// difference between "five attempts a minute" and "as fast as the network
// allows" is the difference between a password that survives the night and
// one that does not.
//
// The service layer is deliberately not involved. A rate limit is a property
// of the transport -- who is allowed to knock, how often -- not of the use
// case, which should behave identically however the request arrived.

// RateLimit configures a bucket. The zero value means "use the defaults",
// so a caller that has no opinion does not have to express one.
type RateLimit struct {
	// Burst is how many requests may arrive at once before throttling starts.
	Burst int
	// Every is how long it takes to earn one request back.
	Every time.Duration
	// Disabled turns the limiter off entirely. Tests that fire a hundred
	// logins at a fake service want this; production does not.
	Disabled bool
}

// defaultLoginRate: five attempts up front, then one every twelve seconds.
//
// Chosen to be invisible to a person -- nobody types their password five times
// in a minute by accident, and a mistyped password plus a retry costs two --
// while making an online guessing attack pointless. At this rate a determined
// attacker gets about five thousand guesses a day from one address, against a
// password space that Argon2id already makes expensive to search offline.
var defaultLoginRate = RateLimit{Burst: 5, Every: 12 * time.Second}

func (rl RateLimit) orDefault() RateLimit {
	if rl.Disabled {
		return rl
	}
	if rl.Burst <= 0 {
		rl.Burst = defaultLoginRate.Burst
	}
	if rl.Every <= 0 {
		rl.Every = defaultLoginRate.Every
	}
	return rl
}

// limiter is a token bucket per client, swept of idle entries as it goes.
//
// The sweep is the part worth attention: without it this map is an unbounded
// allocation driven by whoever sends the most distinct addresses, which is a
// memory-exhaustion attack dressed as traffic. Buckets that have refilled
// completely are indistinguishable from absent ones, so dropping them changes
// no answer.
type limiter struct {
	rate RateLimit
	now  func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	lastGC  time.Time
}

type bucket struct {
	// tokens is fractional so that a partially-earned token is not lost each
	// time the bucket is read.
	tokens float64
	seen   time.Time
}

// gcInterval is how often idle buckets are swept. Rare enough that the sweep
// costs nothing amortised, frequent enough that a burst of unique addresses
// does not sit in memory for long.
const gcInterval = 5 * time.Minute

func newLimiter(rate RateLimit, now func() time.Time) *limiter {
	if now == nil {
		now = time.Now
	}
	return &limiter{
		rate:    rate.orDefault(),
		now:     now,
		buckets: map[string]*bucket{},
		lastGC:  now(),
	}
}

// allow reports whether a request from key may proceed, and if not, how long
// to wait. The wait is what goes in Retry-After: telling a client to come back
// without saying when invites it to retry immediately.
func (l *limiter) allow(key string) (bool, time.Duration) {
	if l == nil || l.rate.Disabled {
		return true, 0
	}

	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		// A new client starts full, minus the request it is making.
		l.buckets[key] = &bucket{tokens: float64(l.rate.Burst) - 1, seen: now}
		return true, 0
	}

	// Refill for the time that has passed, capped at the burst size: an
	// address that has been quiet for a week gets a full bucket, not a week's
	// worth of credit.
	earned := float64(now.Sub(b.seen)) / float64(l.rate.Every)
	b.tokens = min(b.tokens+earned, float64(l.rate.Burst))
	b.seen = now

	if b.tokens < 1 {
		// How long until the next whole token.
		wait := time.Duration((1 - b.tokens) * float64(l.rate.Every))
		return false, wait
	}

	b.tokens--
	return true, 0
}

// sweep drops buckets that have refilled to full, which are by definition
// identical to buckets that do not exist. Caller holds the lock.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastGC) < gcInterval {
		return
	}
	l.lastGC = now

	full := time.Duration(l.rate.Burst) * l.rate.Every
	for key, b := range l.buckets {
		if now.Sub(b.seen) >= full {
			delete(l.buckets, key)
		}
	}
}

// withRateLimit throttles a handler per client address.
//
// Keyed on the same address the session audit records -- see clientIP in
// auth.go, and the note there about why X-Forwarded-For is not read. That
// matters more here than there: if this trusted a client-supplied header, an
// attacker would lift their own limit by varying it, which is worse than
// having no limit at all because it would look like one.
func withRateLimit(l *limiter) func(http.HandlerFunc) http.Handler {
	return func(next http.HandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, wait := l.allow(clientIP(r))
			if !ok {
				seconds := int(wait.Seconds())
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				writeError(w, r, domain.RateLimited("Too many attempts. Try again in a moment."))
				return
			}

			next(w, r)
		})
	}
}
