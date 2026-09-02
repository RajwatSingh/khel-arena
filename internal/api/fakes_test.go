package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
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

type fakeArenas struct {
	t *testing.T

	list       func(context.Context) ([]postgres.ArenaListing, error)
	bySlug     func(context.Context, string) (postgres.ArenaDetail, error)
	listAreas  func(context.Context) ([]string, error)
	cityLedger func(context.Context, time.Time, domain.Sport, string) (service.Ledger, error)

	gotSlug  string
	gotSport domain.Sport
	gotArea  string
	gotDate  time.Time
}

func (f *fakeArenas) List(ctx context.Context) ([]postgres.ArenaListing, error) {
	if f.list == nil {
		f.t.Fatal("handler called ArenaAPI.List, which this test did not expect")
	}
	return f.list(ctx)
}

func (f *fakeArenas) BySlug(ctx context.Context, slug string) (postgres.ArenaDetail, error) {
	if f.bySlug == nil {
		f.t.Fatal("handler called ArenaAPI.BySlug, which this test did not expect")
	}
	f.gotSlug = slug
	return f.bySlug(ctx, slug)
}

func (f *fakeArenas) ListAreas(ctx context.Context) ([]string, error) {
	if f.listAreas == nil {
		f.t.Fatal("handler called ArenaAPI.ListAreas, which this test did not expect")
	}
	return f.listAreas(ctx)
}

func (f *fakeArenas) CityLedger(ctx context.Context, date time.Time, sport domain.Sport, area string) (service.Ledger, error) {
	if f.cityLedger == nil {
		f.t.Fatal("handler called ArenaAPI.CityLedger, which this test did not expect")
	}
	f.gotDate, f.gotSport, f.gotArea = date, sport, area
	return f.cityLedger(ctx, date, sport, area)
}

type fakePayments struct {
	t *testing.T

	providers func() []domain.PaymentProvider
	checkout  func(context.Context, uuid.UUID, uuid.UUID, domain.PaymentProvider) (payment.Checkout, domain.Payment, error)
	settle    func(context.Context, domain.PaymentProvider, payment.CallbackRef) (domain.Payment, error)
	status    func(context.Context, uuid.UUID, uuid.UUID) (domain.Payment, error)

	gotProvider domain.PaymentProvider
	gotRef      payment.CallbackRef
	gotUserID   uuid.UUID
}

func (f *fakePayments) Providers() []domain.PaymentProvider {
	if f.providers == nil {
		return []domain.PaymentProvider{domain.ProviderEsewa}
	}
	return f.providers()
}

func (f *fakePayments) Checkout(ctx context.Context, bookingID, userID uuid.UUID, provider domain.PaymentProvider) (payment.Checkout, domain.Payment, error) {
	if f.checkout == nil {
		f.t.Fatal("handler called PaymentAPI.Checkout, which this test did not expect")
	}
	f.gotProvider, f.gotUserID = provider, userID
	return f.checkout(ctx, bookingID, userID, provider)
}

func (f *fakePayments) Settle(ctx context.Context, provider domain.PaymentProvider, ref payment.CallbackRef) (domain.Payment, error) {
	if f.settle == nil {
		f.t.Fatal("handler called PaymentAPI.Settle, which this test did not expect")
	}
	f.gotProvider, f.gotRef = provider, ref
	return f.settle(ctx, provider, ref)
}

func (f *fakePayments) Status(ctx context.Context, bookingID, userID uuid.UUID) (domain.Payment, error) {
	if f.status == nil {
		f.t.Fatal("handler called PaymentAPI.Status, which this test did not expect")
	}
	f.gotUserID = userID
	return f.status(ctx, bookingID, userID)
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
//
// The optional dependencies are functional options rather than more
// positional arguments: most tests have an opinion about one of them, and a
// five- or six-argument helper full of nils says nothing at a call site.
func withArenas(a *fakeArenas) func(*Options) {
	return func(o *Options) { o.Arenas = a }
}

func withPayments(p *fakePayments) func(*Options) {
	return func(o *Options) { o.Payments = p }
}

func newTestServer(t *testing.T, auth *fakeAuth, bookings *fakeBookings, profiles *fakeProfiles, extras ...func(*Options)) http.Handler {
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

	opts := Options{
		Auth:           auth,
		Bookings:       bookings,
		Profiles:       profiles,
		Pinger:         fakePinger{},
		AllowedOrigins: []string{"http://localhost:5173"},
		// Off by default. A handler test that signs in six times is exercising
		// the handler, not the limiter, and should not start failing on the
		// sixth because of a policy it never asked about. TestRateLimit turns
		// it on deliberately.
		LoginRateLimit: RateLimit{Disabled: true},
	}
	for _, extra := range extras {
		extra(&opts)
	}

	// Give every fake the testing handle it reports unexpected calls through.
	if a, ok := opts.Arenas.(*fakeArenas); ok && a != nil {
		a.t = t
	}
	if p, ok := opts.Payments.(*fakePayments); ok && p != nil {
		p.t = t
	}
	if o, ok := opts.Owner.(*fakeOwner); ok && o != nil {
		o.t = t
	}
	if m, ok := opts.Matches.(*fakeMatches); ok && m != nil {
		m.t = t
	}

	return NewServer(opts).Handler()
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

type fakeOwner struct {
	t *testing.T

	myArenas          func(context.Context, uuid.UUID) ([]postgres.ArenaListing, error)
	createArena       func(context.Context, uuid.UUID, domain.Arena) (domain.Arena, error)
	updateArena       func(context.Context, uuid.UUID, uuid.UUID, domain.Arena) (domain.Arena, error)
	setArenaActive    func(context.Context, uuid.UUID, uuid.UUID, bool) error
	createCourt       func(context.Context, uuid.UUID, domain.Court, string) (postgres.CourtWithRules, error)
	updateCourt       func(context.Context, uuid.UUID, uuid.UUID, domain.Court, string) (postgres.CourtWithRules, error)
	setCourtActive    func(context.Context, uuid.UUID, uuid.UUID, bool) error
	createPricingRule func(context.Context, uuid.UUID, domain.PricingRule) (domain.PricingRule, error)
	copyPricingRules  func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (int, error)
	deletePricingRule func(context.Context, uuid.UUID, uuid.UUID) error
	payments          func(context.Context, uuid.UUID, uuid.UUID, int) ([]postgres.OwnerPayment, error)
	markCashReceived  func(context.Context, uuid.UUID, uuid.UUID) (domain.Payment, error)
}

func (f *fakeOwner) unexpected(method string) {
	f.t.Helper()
	f.t.Fatalf("handler called OwnerAPI.%s, which this test did not expect", method)
}

func (f *fakeOwner) MyArenas(ctx context.Context, ownerID uuid.UUID) ([]postgres.ArenaListing, error) {
	if f.myArenas == nil {
		f.unexpected("MyArenas")
	}
	return f.myArenas(ctx, ownerID)
}

func (f *fakeOwner) CreateArena(ctx context.Context, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error) {
	if f.createArena == nil {
		f.unexpected("CreateArena")
	}
	return f.createArena(ctx, ownerID, a)
}

func (f *fakeOwner) UpdateArena(ctx context.Context, arenaID, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error) {
	if f.updateArena == nil {
		f.unexpected("UpdateArena")
	}
	return f.updateArena(ctx, arenaID, ownerID, a)
}

func (f *fakeOwner) SetArenaActive(ctx context.Context, arenaID, ownerID uuid.UUID, active bool) error {
	if f.setArenaActive == nil {
		f.unexpected("SetArenaActive")
	}
	return f.setArenaActive(ctx, arenaID, ownerID, active)
}

func (f *fakeOwner) CreateCourt(ctx context.Context, ownerID uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error) {
	if f.createCourt == nil {
		f.unexpected("CreateCourt")
	}
	return f.createCourt(ctx, ownerID, c, format)
}

func (f *fakeOwner) UpdateCourt(ctx context.Context, courtID, ownerID uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error) {
	if f.updateCourt == nil {
		f.unexpected("UpdateCourt")
	}
	return f.updateCourt(ctx, courtID, ownerID, c, format)
}

func (f *fakeOwner) SetCourtActive(ctx context.Context, courtID, ownerID uuid.UUID, active bool) error {
	if f.setCourtActive == nil {
		f.unexpected("SetCourtActive")
	}
	return f.setCourtActive(ctx, courtID, ownerID, active)
}

func (f *fakeOwner) CreatePricingRule(ctx context.Context, ownerID uuid.UUID, rule domain.PricingRule) (domain.PricingRule, error) {
	if f.createPricingRule == nil {
		f.unexpected("CreatePricingRule")
	}
	return f.createPricingRule(ctx, ownerID, rule)
}

func (f *fakeOwner) CopyPricingRules(ctx context.Context, fromCourtID, toCourtID, ownerID uuid.UUID) (int, error) {
	if f.copyPricingRules == nil {
		f.unexpected("CopyPricingRules")
	}
	return f.copyPricingRules(ctx, fromCourtID, toCourtID, ownerID)
}

func (f *fakeOwner) DeletePricingRule(ctx context.Context, ruleID, ownerID uuid.UUID) error {
	if f.deletePricingRule == nil {
		f.unexpected("DeletePricingRule")
	}
	return f.deletePricingRule(ctx, ruleID, ownerID)
}

func (f *fakeOwner) Payments(ctx context.Context, arenaID, ownerID uuid.UUID, limit int) ([]postgres.OwnerPayment, error) {
	if f.payments == nil {
		f.unexpected("Payments")
	}
	return f.payments(ctx, arenaID, ownerID, limit)
}

func (f *fakeOwner) MarkCashReceived(ctx context.Context, paymentID, ownerID uuid.UUID) (domain.Payment, error) {
	if f.markCashReceived == nil {
		f.unexpected("MarkCashReceived")
	}
	return f.markCashReceived(ctx, paymentID, ownerID)
}

func withOwner(o *fakeOwner) func(*Options) {
	return func(opts *Options) { opts.Owner = o }
}

type fakeMatches struct {
	t *testing.T

	report      func(context.Context, uuid.UUID, domain.Match) (domain.Match, error)
	confirm     func(context.Context, uuid.UUID, uuid.UUID) (domain.Match, error)
	withdraw    func(context.Context, uuid.UUID, uuid.UUID) error
	listForTeam func(context.Context, uuid.UUID, int) ([]domain.Match, error)
	standings   func(context.Context, int) ([]domain.Standing, error)

	gotActor uuid.UUID
}

func (f *fakeMatches) Report(ctx context.Context, actorID uuid.UUID, m domain.Match) (domain.Match, error) {
	if f.report == nil {
		f.t.Fatal("handler called MatchAPI.Report, which this test did not expect")
	}
	f.gotActor = actorID
	return f.report(ctx, actorID, m)
}

func (f *fakeMatches) Confirm(ctx context.Context, matchID, actorID uuid.UUID) (domain.Match, error) {
	if f.confirm == nil {
		f.t.Fatal("handler called MatchAPI.Confirm, which this test did not expect")
	}
	f.gotActor = actorID
	return f.confirm(ctx, matchID, actorID)
}

func (f *fakeMatches) Withdraw(ctx context.Context, matchID, actorID uuid.UUID) error {
	if f.withdraw == nil {
		f.t.Fatal("handler called MatchAPI.Withdraw, which this test did not expect")
	}
	return f.withdraw(ctx, matchID, actorID)
}

func (f *fakeMatches) ListForTeam(ctx context.Context, teamID uuid.UUID, limit int) ([]domain.Match, error) {
	if f.listForTeam == nil {
		f.t.Fatal("handler called MatchAPI.ListForTeam, which this test did not expect")
	}
	return f.listForTeam(ctx, teamID, limit)
}

func (f *fakeMatches) Standings(ctx context.Context, limit int) ([]domain.Standing, error) {
	if f.standings == nil {
		f.t.Fatal("handler called MatchAPI.Standings, which this test did not expect")
	}
	return f.standings(ctx, limit)
}

func withMatches(m *fakeMatches) func(*Options) {
	return func(o *Options) { o.Matches = m }
}
