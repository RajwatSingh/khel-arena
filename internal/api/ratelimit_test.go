package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

// The clock is injected throughout: a rate limiter tested against the wall
// clock either sleeps (slow, and flaky on a loaded machine) or cannot test
// refill at all.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time      { return c.t }
func (c *fakeClock) add(d time.Duration) { c.t = c.t.Add(d) }

func newTestClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
}

func TestLimiterBurstThenThrottle(t *testing.T) {
	clock := newTestClock()
	l := newLimiter(RateLimit{Burst: 3, Every: time.Second}, clock.now)

	for i := 1; i <= 3; i++ {
		if ok, _ := l.allow("1.1.1.1"); !ok {
			t.Fatalf("request %d was refused inside the burst", i)
		}
	}

	ok, wait := l.allow("1.1.1.1")
	if ok {
		t.Fatal("a fourth request slipped through a burst of three")
	}
	// A refusal with no wait tells a client to retry immediately, which is
	// how a limiter turns into a busy loop.
	if wait <= 0 {
		t.Errorf("wait = %v, want a positive duration for Retry-After", wait)
	}
}

func TestLimiterRefills(t *testing.T) {
	clock := newTestClock()
	l := newLimiter(RateLimit{Burst: 2, Every: 10 * time.Second}, clock.now)

	l.allow("1.1.1.1")
	l.allow("1.1.1.1")
	if ok, _ := l.allow("1.1.1.1"); ok {
		t.Fatal("the bucket should be empty")
	}

	clock.add(10 * time.Second)
	if ok, _ := l.allow("1.1.1.1"); !ok {
		t.Error("one token should have been earned back after one interval")
	}
}

// A bucket that kept earning while nobody was knocking would hand a quiet
// address a week's worth of credit in one burst.
func TestLimiterRefillIsCappedAtBurst(t *testing.T) {
	clock := newTestClock()
	l := newLimiter(RateLimit{Burst: 2, Every: time.Second}, clock.now)

	l.allow("1.1.1.1")
	l.allow("1.1.1.1")

	clock.add(24 * time.Hour)

	if ok, _ := l.allow("1.1.1.1"); !ok {
		t.Fatal("first request after a long quiet period was refused")
	}
	if ok, _ := l.allow("1.1.1.1"); !ok {
		t.Fatal("second request was refused")
	}
	if ok, _ := l.allow("1.1.1.1"); ok {
		t.Error("a day of quiet earned more than the burst")
	}
}

func TestLimiterIsPerClient(t *testing.T) {
	clock := newTestClock()
	l := newLimiter(RateLimit{Burst: 1, Every: time.Minute}, clock.now)

	if ok, _ := l.allow("1.1.1.1"); !ok {
		t.Fatal("first client refused")
	}
	if ok, _ := l.allow("1.1.1.1"); ok {
		t.Error("first client got a second token")
	}
	// One noisy address must not lock everybody else out.
	if ok, _ := l.allow("2.2.2.2"); !ok {
		t.Error("a different client was throttled by the first one's traffic")
	}
}

// Without a sweep this map grows with every distinct address seen, which is a
// memory-exhaustion attack wearing traffic's clothes.
func TestLimiterSweepsIdleBuckets(t *testing.T) {
	clock := newTestClock()
	l := newLimiter(RateLimit{Burst: 2, Every: time.Second}, clock.now)

	for _, ip := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		l.allow(ip)
	}
	if got := len(l.buckets); got != 3 {
		t.Fatalf("buckets = %d, want 3", got)
	}

	// Past both the GC interval and the time it takes a bucket to refill.
	clock.add(gcInterval + time.Minute)
	l.allow("4.4.4.4")

	// The three idle ones are gone; only the caller that just knocked remains.
	if got := len(l.buckets); got != 1 {
		t.Errorf("buckets = %d after the sweep, want 1", got)
	}
}

func TestLimiterDisabled(t *testing.T) {
	l := newLimiter(RateLimit{Disabled: true}, newTestClock().now)

	for i := 0; i < 100; i++ {
		if ok, _ := l.allow("1.1.1.1"); !ok {
			t.Fatalf("request %d was throttled with the limiter off", i)
		}
	}
}

func TestLimiterZeroValueUsesDefaults(t *testing.T) {
	l := newLimiter(RateLimit{}, newTestClock().now)

	if l.rate.Burst != defaultLoginRate.Burst || l.rate.Every != defaultLoginRate.Every {
		t.Errorf("rate = %+v, want the defaults %+v", l.rate, defaultLoginRate)
	}
}

// Login is the online password-guessing target; this is the endpoint the
// limiter exists for.
func TestLoginIsRateLimited(t *testing.T) {
	auth := &fakeAuth{
		t: t,
		login: func(context.Context, string, string, postgres.SessionContext) (service.Session, error) {
			return service.Session{}, domain.Unauthenticated("That email and password don't match.")
		},
	}

	h := NewServer(Options{
		Auth:           auth,
		LoginRateLimit: RateLimit{Burst: 3, Every: time.Minute},
	}).Handler()

	body := `{"email":"victim@khelarena.np","password":"guess"}`

	// Three wrong guesses are answered as wrong guesses.
	for i := 1; i <= 3; i++ {
		if w := do(h, http.MethodPost, "/v1/auth/login", body); w.Code != http.StatusUnauthorized {
			t.Fatalf("guess %d: status = %d, want 401", i, w.Code)
		}
	}

	// The fourth is refused before it reaches the service, so guessing costs
	// the attacker time rather than costing us an Argon2id hash.
	w := do(h, http.MethodPost, "/v1/auth/login", body)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d (%s), want 429", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After on a 429, so a client has nothing to wait for")
	}
}

// The other grindable endpoint: unthrottled it sprays somebody's inbox.
func TestPasswordForgotIsRateLimited(t *testing.T) {
	auth := &fakeAuth{
		t: t,
		beginPasswordReset: func(context.Context, string) (string, domain.User, error) {
			return "", domain.User{}, nil
		},
	}

	h := NewServer(Options{
		Auth:           auth,
		LoginRateLimit: RateLimit{Burst: 2, Every: time.Minute},
	}).Handler()

	body := `{"email":"victim@khelarena.np"}`
	for i := 1; i <= 2; i++ {
		if w := do(h, http.MethodPost, "/v1/auth/password/forgot", body); w.Code != http.StatusAccepted {
			t.Fatalf("attempt %d: status = %d, want 202", i, w.Code)
		}
	}

	if w := do(h, http.MethodPost, "/v1/auth/password/forgot", body); w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
}

// Only the two endpoints that need it. Throttling a read would break the
// interface's own grid, which fetches a day per court.
func TestOtherEndpointsAreNotRateLimited(t *testing.T) {
	bookings := &fakeBookings{
		t:            t,
		availability: func(context.Context, uuid.UUID, time.Time) ([]domain.GridSlot, error) { return nil, nil },
	}

	h := NewServer(Options{
		Bookings:       bookings,
		LoginRateLimit: RateLimit{Burst: 1, Every: time.Hour},
	}).Handler()

	target := "/v1/courts/" + testCourtID.String() + "/availability?date=2026-08-14"
	for i := 1; i <= 10; i++ {
		if w := do(h, http.MethodGet, target, ""); w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, w.Code)
		}
	}
}
