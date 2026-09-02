package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

// The fakes below satisfy AuthAPI, BookingAPI and ProfileAPI with func
// fields, so each test sets only the one method it exercises. A method a test
// did not set failing the test is the point: it says the handler called
// something the test did not expect it to.
//
// This is what the narrow interfaces in server.go buy -- none of these tests
// needs a database, a token issuer or a clock.

type fakeAuth struct {
	t *testing.T

	register              func(context.Context, domain.Registration, postgres.SessionContext) (service.Session, error)
	login                 func(context.Context, string, string, postgres.SessionContext) (service.Session, error)
	refresh               func(context.Context, string, postgres.SessionContext) (service.Session, error)
	logout                func(context.Context, string) error
	authenticate          func(string) (uuid.UUID, domain.AccountType, error)
	beginPasswordReset    func(context.Context, string) (string, domain.User, error)
	completePasswordReset func(context.Context, string, string) error
	changePassword        func(context.Context, uuid.UUID, string, string) error

	// Recorded inputs, for the assertions that care what the handler passed
	// on rather than what it returned.
	gotSessionContext postgres.SessionContext
	gotRefreshToken   string
	gotEmail          string
}

func (f *fakeAuth) unexpected(method string) {
	f.t.Helper()
	f.t.Fatalf("handler called AuthAPI.%s, which this test did not expect", method)
}

func (f *fakeAuth) Register(ctx context.Context, reg domain.Registration, sc postgres.SessionContext) (service.Session, error) {
	if f.register == nil {
		f.unexpected("Register")
	}
	f.gotSessionContext = sc
	return f.register(ctx, reg, sc)
}

func (f *fakeAuth) Login(ctx context.Context, email, password string, sc postgres.SessionContext) (service.Session, error) {
	if f.login == nil {
		f.unexpected("Login")
	}
	f.gotSessionContext = sc
	f.gotEmail = email
	return f.login(ctx, email, password, sc)
}

func (f *fakeAuth) Refresh(ctx context.Context, refreshToken string, sc postgres.SessionContext) (service.Session, error) {
	if f.refresh == nil {
		f.unexpected("Refresh")
	}
	f.gotRefreshToken = refreshToken
	return f.refresh(ctx, refreshToken, sc)
}

func (f *fakeAuth) Logout(ctx context.Context, refreshToken string) error {
	if f.logout == nil {
		f.unexpected("Logout")
	}
	f.gotRefreshToken = refreshToken
	return f.logout(ctx, refreshToken)
}

func (f *fakeAuth) Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error) {
	if f.authenticate == nil {
		f.unexpected("Authenticate")
	}
	return f.authenticate(accessToken)
}

func (f *fakeAuth) BeginPasswordReset(ctx context.Context, email string) (string, domain.User, error) {
	if f.beginPasswordReset == nil {
		f.unexpected("BeginPasswordReset")
	}
	f.gotEmail = email
	return f.beginPasswordReset(ctx, email)
}

func (f *fakeAuth) CompletePasswordReset(ctx context.Context, resetToken, newPassword string) error {
	if f.completePasswordReset == nil {
		f.unexpected("CompletePasswordReset")
	}
	return f.completePasswordReset(ctx, resetToken, newPassword)
}

func (f *fakeAuth) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	if f.changePassword == nil {
		f.unexpected("ChangePassword")
	}
	return f.changePassword(ctx, userID, current, next)
}

type fakeBookings struct {
	t *testing.T

	create       func(context.Context, service.CreateBookingInput) (domain.Booking, error)
	availability func(context.Context, uuid.UUID, time.Time) ([]domain.GridSlot, error)
	listMine     func(context.Context, uuid.UUID, int) ([]domain.BookingDetail, error)
	cancel       func(context.Context, uuid.UUID, uuid.UUID) error

	gotInput   service.CreateBookingInput
	gotDate    time.Time
	gotCourtID uuid.UUID
	gotLimit   int
	gotUserID  uuid.UUID
}

func (f *fakeBookings) Create(ctx context.Context, in service.CreateBookingInput) (domain.Booking, error) {
	if f.create == nil {
		f.t.Fatal("handler called BookingAPI.Create, which this test did not expect")
	}
	f.gotInput = in
	return f.create(ctx, in)
}

func (f *fakeBookings) Availability(ctx context.Context, courtID uuid.UUID, date time.Time) ([]domain.GridSlot, error) {
	if f.availability == nil {
		f.t.Fatal("handler called BookingAPI.Availability, which this test did not expect")
	}
	f.gotCourtID, f.gotDate = courtID, date
	return f.availability(ctx, courtID, date)
}

func (f *fakeBookings) ListMine(ctx context.Context, userID uuid.UUID, limit int) ([]domain.BookingDetail, error) {
	if f.listMine == nil {
		f.t.Fatal("handler called BookingAPI.ListMine, which this test did not expect")
	}
	f.gotUserID, f.gotLimit = userID, limit
	return f.listMine(ctx, userID, limit)
}

func (f *fakeBookings) Cancel(ctx context.Context, bookingID, userID uuid.UUID) error {
	if f.cancel == nil {
		f.t.Fatal("handler called BookingAPI.Cancel, which this test did not expect")
	}
	f.gotUserID = userID
	return f.cancel(ctx, bookingID, userID)
}

type fakeProfiles struct {
	t *testing.T

	me     func(context.Context, uuid.UUID) (domain.User, error)
	update func(context.Context, uuid.UUID, domain.ProfileUpdate) (domain.User, error)

	gotUpdate domain.ProfileUpdate
	updated   bool
}

func (f *fakeProfiles) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	if f.me == nil {
		f.t.Fatal("handler called ProfileAPI.Me, which this test did not expect")
	}
	return f.me(ctx, userID)
}

func (f *fakeProfiles) Update(ctx context.Context, userID uuid.UUID, p domain.ProfileUpdate) (domain.User, error) {
	if f.update == nil {
		f.t.Fatal("handler called ProfileAPI.Update, which this test did not expect")
	}
	f.gotUpdate, f.updated = p, true
	return f.update(ctx, userID, p)
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

// ------------------------------------------------------------------ harness -

// testUser is the account the fakes hand back, so assertions have one known
// shape to compare against.
var testUser = domain.User{
	ID:          uuid.MustParse("11111111-1111-4111-8111-111111111111"),
	Email:       "rajwat@khelarena.np",
	Username:    "rajwat",
	FullName:    "Rajwat Singh",
	AccountType: domain.AccountPlayer,
	Skill:       domain.SkillIntermediate,
}

const testAccessToken = "test.access.token"

func testSession() service.Session {
	return service.Session{
		User:                  testUser,
		AccessToken:           testAccessToken,
		AccessTokenExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshToken:          "test-refresh-token",
		RefreshTokenExpiresAt: time.Now().Add(720 * time.Hour),
	}
}

// signedIn is an Authenticate that accepts one token and rejects everything
// else, which is enough to exercise both sides of the auth middleware.
func signedIn(token string) func(string) (uuid.UUID, domain.AccountType, error) {
	return func(got string) (uuid.UUID, domain.AccountType, error) {
		if got != token {
			return uuid.Nil, "", domain.Unauthenticated("Please sign in.")
		}
		return testUser.ID, testUser.AccountType, nil
	}
}

// newTestServer wires a Server over the given fakes and returns the full
// handler -- routing and middleware included, so every test exercises the
// same stack a real request meets.
func newTestServer(t *testing.T, auth *fakeAuth, bookings *fakeBookings, profiles *fakeProfiles) http.Handler {
	t.Helper()

	if auth != nil {
		auth.t = t
	}
	if bookings != nil {
		bookings.t = t
	}
	if profiles != nil {
		profiles.t = t
	}

	return NewServer(Options{
		Auth:           auth,
		Bookings:       bookings,
		Profiles:       profiles,
		Pinger:         fakePinger{},
		AllowedOrigins: []string{"http://localhost:5173"},
	}).Handler()
}

// do sends one request through a handler and returns the recorded response.
func do(h http.Handler, method, target, body string, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	for _, opt := range opts {
		opt(r)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func bearer(token string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func withCookie(name, value string) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: name, Value: value}) }
}

// cookieNamed finds a Set-Cookie the response carries, so tests can assert on
// the refresh cookie without parsing headers by hand.
func cookieNamed(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}
