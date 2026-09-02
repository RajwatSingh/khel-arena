package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidationCollectsEveryFailure(t *testing.T) {
	r := Registration{Email: "not-an-email", Username: "A", Password: "short"}

	err := r.Validate()
	if err == nil {
		t.Fatal("expected the registration to be rejected")
	}

	// The point of accumulating is that someone filling in a form learns
	// everything wrong with it in one response, not one field per attempt.
	var de *Error
	if !errors.As(err, &de) {
		t.Fatalf("expected a domain error, got %T", err)
	}

	got := make(map[string]string)
	for _, f := range de.FieldErrors() {
		got[f.Field] = f.Message
	}

	// full_name is included because it was left blank.
	for _, field := range []string{"email", "username", "password", "full_name"} {
		if _, ok := got[field]; !ok {
			t.Errorf("no failure reported for %q; got %v", field, got)
		}
	}
	if len(got) != 4 {
		t.Errorf("reported %d fields, want 4: %v", len(got), got)
	}
}

// A single-field failure reports itself through the same accessor, so a
// caller never has to branch on how many things went wrong.
func TestSingleFieldErrorExposesItself(t *testing.T) {
	err := ValidateUsername("ab")

	var de *Error
	if !errors.As(err, &de) {
		t.Fatalf("expected a domain error, got %T", err)
	}
	fields := de.FieldErrors()
	if len(fields) != 1 {
		t.Fatalf("got %d field errors, want 1", len(fields))
	}
	if fields[0].Field != "username" {
		t.Errorf("field = %q, want %q", fields[0].Field, "username")
	}
}

func TestRegistrationNormalisesInput(t *testing.T) {
	r := Registration{
		Email:    "  RajWat@Example.COM ",
		Username: "  RajWat  ",
		FullName: "  Rajwat Singh  ",
		Password: "a-long-enough-password",
	}

	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Email != "rajwat@example.com" {
		t.Errorf("email = %q, want it lowercased and trimmed", r.Email)
	}
	if r.Username != "rajwat" {
		t.Errorf("username = %q, want it lowercased and trimmed", r.Username)
	}
	if r.FullName != "Rajwat Singh" {
		t.Errorf("full name = %q, want it trimmed", r.FullName)
	}
	if r.AccountType != AccountPlayer {
		t.Errorf("account type = %q, want it to default to %q", r.AccountType, AccountPlayer)
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		in  string
		ok  bool
		why string
	}{
		{"rajwat", true, ""},
		{"raj_wat_99", true, ""},
		{"abc", true, "three characters is the minimum"},
		{"ab", false, "too short"},
		{"", false, "empty"},
		{strings.Repeat("a", 25), false, "too long"},
		{"Rajwat", false, "uppercase is normalised away, but a bare check must still reject it"},
		{"raj wat", false, "spaces"},
		{"raj-wat", false, "hyphens"},
		{"raj.wat", false, "dots"},
		{"admin", false, "reserved"},
		{"api", false, "reserved, and would collide with a route"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			err := ValidateUsername(tc.in)
			// ValidateUsername normalises first, so mixed case passes.
			if tc.in == "Rajwat" {
				if err != nil {
					t.Errorf("mixed case should normalise and pass, got %v", err)
				}
				return
			}
			if tc.ok && err != nil {
				t.Errorf("expected %q to be accepted: %v", tc.in, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("expected %q to be rejected (%s)", tc.in, tc.why)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	valid := []string{"a@b.co", "rajwat@example.com", "first.last+tag@sub.example.np"}
	invalid := []string{"", "notanemail", "@example.com", "a@", "a@b", "a b@example.com",
		"<script>@example.com", strings.Repeat("a", 250) + "@example.com"}

	for _, in := range valid {
		if err := ValidateEmail(in); err != nil {
			t.Errorf("expected %q to be accepted: %v", in, err)
		}
	}
	for _, in := range invalid {
		if err := ValidateEmail(in); err == nil {
			t.Errorf("expected %q to be rejected", in)
		}
	}
}

func TestValidatePhoneAcceptsNepaliMobiles(t *testing.T) {
	for _, in := range []string{"9801234567", "9741234567", ""} {
		if err := ValidatePhone(in); err != nil {
			t.Errorf("expected %q to be accepted: %v", in, err)
		}
	}
	for _, in := range []string{"1234567890", "980123456", "98012345678", "+9779801234567"} {
		if err := ValidatePhone(in); err == nil {
			t.Errorf("expected %q to be rejected", in)
		}
	}
}

func TestValidatePasswordEnforcesLengthOnly(t *testing.T) {
	// Length is what correlates with strength. Composition rules push people
	// toward predictable substitutions, so they are deliberately absent.
	if err := ValidatePassword("all lowercase but long enough"); err != nil {
		t.Errorf("a long passphrase should be accepted: %v", err)
	}
	if err := ValidatePassword("Sh0rt!"); err == nil {
		t.Error("a short password should be rejected however complex")
	}
	if err := ValidatePassword(strings.Repeat("a", MaxPasswordLength+1)); err == nil {
		t.Error("an unbounded password would let a caller spend our memory")
	}
	if err := ValidatePassword("          "); err == nil {
		t.Error("a password of only spaces should be rejected")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Dhuku Futsal", "dhuku-futsal"},
		{"Dhuku Futsal, Jhamsikhel", "dhuku-futsal-jhamsikhel"},
		{"  Spaces   Everywhere  ", "spaces-everywhere"},
		{"Already-Slugged", "already-slugged"},
		{"Under_scores", "under-scores"},
		{"Punctuation!!! Here???", "punctuation-here"},
		{"Multiple---Hyphens", "multiple-hyphens"},
		{"7A Side Arena", "7a-side-arena"},
		{"---leading and trailing---", "leading-and-trailing"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := Slugify(tc.in)
			if got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !ValidSlug(got) {
				t.Errorf("Slugify produced %q, which ValidSlug rejects", got)
			}
		})
	}
}

// Non-ASCII names slugify to nothing rather than to mojibake. Callers must
// notice, which is why ValidSlug rejects the empty string.
func TestSlugifyOnNonASCIIYieldsAnInvalidSlug(t *testing.T) {
	got := Slugify("धुकु फुटसल")
	if got != "" {
		t.Errorf("Slugify = %q, want an empty string for an all-Devanagari name", got)
	}
	if ValidSlug(got) {
		t.Error("an empty slug must not pass validation, or the arena gets an unreachable URL")
	}
}

func TestErrorCodeMatching(t *testing.T) {
	err := NotFound("That booking doesn't exist.")

	if !errors.Is(err, ErrNotFound) {
		t.Error("a NotFound error should match the ErrNotFound sentinel")
	}
	if errors.Is(err, ErrConflict) {
		t.Error("a NotFound error should not match a different sentinel")
	}
	if CodeOf(err) != CodeNotFound {
		t.Errorf("CodeOf = %q, want %q", CodeOf(err), CodeNotFound)
	}
}

// An unclassified error is one nobody decided was safe to show, so it must
// collapse to a generic message rather than leaking a driver's detail.
func TestUserMessageHidesInternalDetail(t *testing.T) {
	leaky := errors.New(`pq: password authentication failed for user "khel"`)

	if msg := UserMessage(leaky); strings.Contains(msg, "password") {
		t.Errorf("an unclassified error leaked detail: %q", msg)
	}
	if CodeOf(leaky) != CodeInternal {
		t.Errorf("CodeOf = %q, want unclassified errors treated as %q", CodeOf(leaky), CodeInternal)
	}

	wrapped := Internal(leaky, "loading booking %s", "abc")
	if msg := UserMessage(wrapped); strings.Contains(msg, "password") {
		t.Errorf("an internal error leaked its cause: %q", msg)
	}
	// The detail still has to reach the log.
	if !strings.Contains(wrapped.Error(), "password authentication failed") {
		t.Error("the cause should survive for the operator, even though the user never sees it")
	}
	if !errors.Is(wrapped, leaky) {
		t.Error("the cause should remain unwrappable")
	}

	// A classified error keeps its message: it was written to be read.
	shown := Conflict("Someone took this slot a moment ago.")
	if UserMessage(shown) != "Someone took this slot a moment ago." {
		t.Errorf("a classified message should be shown verbatim, got %q", UserMessage(shown))
	}
}

func TestEnumValidationRejectsUnknownValues(t *testing.T) {
	if err := BookingStatus("deleted").Validate(); err == nil {
		t.Error("an unknown booking status should be rejected")
	}
	if err := BookingConfirmed.Validate(); err != nil {
		t.Errorf("a known status should be accepted: %v", err)
	}
	// The message should tell the caller what is allowed.
	err := SkillTier("expert").Validate()
	if err == nil || !strings.Contains(err.Error(), "casual") {
		t.Errorf("the error should list the valid options, got %v", err)
	}
}

func TestBookingStatusPredicates(t *testing.T) {
	live := []BookingStatus{BookingPending, BookingConfirmed, BookingCompleted}
	dead := []BookingStatus{BookingCancelled, BookingNoShow}

	for _, s := range live {
		if !s.IsLive() {
			t.Errorf("%s should count as live", s)
		}
	}
	for _, s := range dead {
		if s.IsLive() {
			t.Errorf("%s should not hold its slot", s)
		}
	}

	for _, s := range []BookingStatus{BookingPending, BookingConfirmed} {
		if !s.CanCancel() {
			t.Errorf("%s should be cancellable", s)
		}
	}
	for _, s := range []BookingStatus{BookingCompleted, BookingCancelled, BookingNoShow} {
		if s.CanCancel() {
			t.Errorf("%s describes something already settled and should not be cancellable", s)
		}
	}
}

func TestTournamentPrizeSplit(t *testing.T) {
	base := func() Tournament {
		return Tournament{
			Name: "Kathmandu Cup", Format: FormatKnockout, SideCount: 5, SquadCap: 10,
			MaxTeams: 8, PrizePoolNPR: 100000, PrizeSplit: []int{60, 30, 10},
			StartsOn:   time.Date(2030, time.June, 1, 0, 0, 0, 0, time.UTC),
			RegisterBy: time.Date(2030, time.May, 20, 0, 0, 0, 0, time.UTC),
		}
	}

	tr := base()
	if err := tr.Validate(); err != nil {
		t.Fatalf("a well-formed tournament was rejected: %v", err)
	}
	if got := tr.PrizeFor(1); got != 60000 {
		t.Errorf("first prize = %d, want 60000", got)
	}
	if got := tr.PrizeFor(3); got != 10000 {
		t.Errorf("third prize = %d, want 10000", got)
	}
	if got := tr.PrizeFor(4); got != 0 {
		t.Errorf("fourth place is outside the split and should win %d, want 0", got)
	}

	// A split that does not total 100 either overpays or strands prize money.
	bad := base()
	bad.PrizeSplit = []int{60, 30}
	if err := bad.Validate(); err == nil {
		t.Error("a split totalling 90% should be rejected")
	}

	// A knockout bracket with a non-power-of-two field needs byes nothing models.
	odd := base()
	odd.MaxTeams = 12
	if err := odd.Validate(); err == nil {
		t.Error("a 12-team knockout bracket should be rejected")
	}
	// The same field is fine in a league.
	league := base()
	league.MaxTeams, league.Format = 12, FormatLeague
	if err := league.Validate(); err != nil {
		t.Errorf("a 12-team league is perfectly playable: %v", err)
	}
}

func TestTournamentRegistrationWindow(t *testing.T) {
	now := time.Date(2030, time.May, 15, 12, 0, 0, 0, time.UTC)
	base := Tournament{
		Status: TournamentOpen, MaxTeams: 8, TeamCount: 2,
		RegisterBy: time.Date(2030, time.May, 20, 0, 0, 0, 0, time.UTC),
	}

	if err := base.CanAcceptRegistration(now); err != nil {
		t.Errorf("registration should be open: %v", err)
	}

	// Registration runs through the end of the closing day.
	onDeadline := base
	if err := onDeadline.CanAcceptRegistration(time.Date(2030, time.May, 20, 23, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("the closing day should still accept entries: %v", err)
	}
	if err := onDeadline.CanAcceptRegistration(time.Date(2030, time.May, 21, 0, 1, 0, 0, time.UTC)); err == nil {
		t.Error("registration should be closed the day after the deadline")
	}

	full := base
	full.TeamCount = 8
	if err := full.CanAcceptRegistration(now); err == nil {
		t.Error("a full bracket should refuse another team")
	}

	for _, status := range []TournamentStatus{TournamentCancelled, TournamentOngoing, TournamentCompleted, TournamentFull} {
		tr := base
		tr.Status = status
		if err := tr.CanAcceptRegistration(now); err == nil {
			t.Errorf("a %s tournament should not accept registrations", status)
		}
	}
}
