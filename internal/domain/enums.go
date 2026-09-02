package domain

import (
	"fmt"
	"slices"
)

// The enums below mirror the Postgres types of the same name. Each is a
// distinct string type so the compiler refuses to pass a BookingStatus where
// a PaymentStatus belongs, and each validates itself so an unknown value is
// rejected at the edge rather than reaching a query.

type AccountType string

const (
	AccountPlayer     AccountType = "player"
	AccountArenaOwner AccountType = "arena_owner"
)

var accountTypes = []AccountType{AccountPlayer, AccountArenaOwner}

func (a AccountType) Valid() bool { return slices.Contains(accountTypes, a) }
func (a AccountType) Validate() error {
	return validateEnum("account_type", string(a), accountTypes)
}

// ---------------------------------------------------------------------------

type BookingStatus string

const (
	// BookingPending is an unpaid hold. It blocks the slot only until its
	// hold window expires.
	BookingPending BookingStatus = "pending"
	// BookingConfirmed is paid. It blocks the slot unconditionally.
	BookingConfirmed BookingStatus = "confirmed"
	BookingCompleted BookingStatus = "completed"
	BookingCancelled BookingStatus = "cancelled"
	BookingNoShow    BookingStatus = "no_show"
)

var bookingStatuses = []BookingStatus{
	BookingPending, BookingConfirmed, BookingCompleted, BookingCancelled, BookingNoShow,
}

func (b BookingStatus) Valid() bool { return slices.Contains(bookingStatuses, b) }
func (b BookingStatus) Validate() error {
	return validateEnum("status", string(b), bookingStatuses)
}

// IsLive reports whether a booking in this status still occupies its slot.
// This is the Go half of the `booking_blocks_slot` SQL function; a pending
// booking additionally needs its hold to be unexpired, which callers check
// via Booking.BlocksSlot.
func (b BookingStatus) IsLive() bool {
	return b != BookingCancelled && b != BookingNoShow
}

// CanCancel reports whether a booking in this status may still be cancelled.
// A completed or no-show booking describes something that already happened.
func (b BookingStatus) CanCancel() bool {
	return b == BookingPending || b == BookingConfirmed
}

// ---------------------------------------------------------------------------

type PaymentProvider string

const (
	ProviderEsewa  PaymentProvider = "esewa"
	ProviderKhalti PaymentProvider = "khalti"
	ProviderCash   PaymentProvider = "cash"
)

var paymentProviders = []PaymentProvider{ProviderEsewa, ProviderKhalti, ProviderCash}

func (p PaymentProvider) Valid() bool { return slices.Contains(paymentProviders, p) }
func (p PaymentProvider) Validate() error {
	return validateEnum("provider", string(p), paymentProviders)
}

// IsOnline reports whether the provider settles through a gateway callback.
// Cash is reconciled by the arena owner instead.
func (p PaymentProvider) IsOnline() bool { return p == ProviderEsewa || p == ProviderKhalti }

type PaymentStatus string

const (
	PaymentInitiated PaymentStatus = "initiated"
	PaymentVerified  PaymentStatus = "verified"
	PaymentFailed    PaymentStatus = "failed"
	PaymentRefunded  PaymentStatus = "refunded"
)

var paymentStatuses = []PaymentStatus{
	PaymentInitiated, PaymentVerified, PaymentFailed, PaymentRefunded,
}

func (p PaymentStatus) Valid() bool { return slices.Contains(paymentStatuses, p) }
func (p PaymentStatus) Validate() error {
	return validateEnum("status", string(p), paymentStatuses)
}

// IsSettled reports whether the payment has reached a state the gateway will
// not move it out of. A settled payment ignores late duplicate callbacks.
func (p PaymentStatus) IsSettled() bool {
	return p == PaymentVerified || p == PaymentFailed || p == PaymentRefunded
}

// ---------------------------------------------------------------------------

type Sport string

const (
	SportFutsal     Sport = "futsal"
	SportBasketball Sport = "basketball"
	SportBadminton  Sport = "badminton"
	SportCricketNet Sport = "cricket_net"
	SportTennis     Sport = "tennis"
)

var sports = []Sport{SportFutsal, SportBasketball, SportBadminton, SportCricketNet, SportTennis}

func (s Sport) Valid() bool     { return slices.Contains(sports, s) }
func (s Sport) Validate() error { return validateEnum("sport", string(s), sports) }

// ---------------------------------------------------------------------------

type SkillTier string

const (
	SkillCasual       SkillTier = "casual"
	SkillIntermediate SkillTier = "intermediate"
	SkillCompetitive  SkillTier = "competitive"
	SkillSemiPro      SkillTier = "semi_pro"
)

var skillTiers = []SkillTier{SkillCasual, SkillIntermediate, SkillCompetitive, SkillSemiPro}

func (s SkillTier) Valid() bool { return slices.Contains(skillTiers, s) }
func (s SkillTier) Validate() error {
	return validateEnum("skill", string(s), skillTiers)
}

// ---------------------------------------------------------------------------

type MatchmakingStatus string

const (
	MatchmakingOpen      MatchmakingStatus = "open"
	MatchmakingFilled    MatchmakingStatus = "filled"
	MatchmakingExpired   MatchmakingStatus = "expired"
	MatchmakingCancelled MatchmakingStatus = "cancelled"
)

var matchmakingStatuses = []MatchmakingStatus{
	MatchmakingOpen, MatchmakingFilled, MatchmakingExpired, MatchmakingCancelled,
}

func (m MatchmakingStatus) Valid() bool { return slices.Contains(matchmakingStatuses, m) }
func (m MatchmakingStatus) Validate() error {
	return validateEnum("status", string(m), matchmakingStatuses)
}

// AcceptsResponses reports whether players may still ask to join.
func (m MatchmakingStatus) AcceptsResponses() bool { return m == MatchmakingOpen }

// ---------------------------------------------------------------------------

// Position uses futsal vocabulary: Goleiro (keeper), Fixo (last man),
// Ala (winger), Pivo (target man), Universal (plays everywhere).
type Position string

const (
	PositionGoleiro   Position = "Goleiro"
	PositionFixo      Position = "Fixo"
	PositionAla       Position = "Ala"
	PositionPivo      Position = "Pivo"
	PositionUniversal Position = "Universal"
)

var positions = []Position{
	PositionGoleiro, PositionFixo, PositionAla, PositionPivo, PositionUniversal,
}

func (p Position) Valid() bool { return slices.Contains(positions, p) }
func (p Position) Validate() error {
	return validateEnum("position", string(p), positions)
}

type Foot string

const (
	FootLeft  Foot = "left"
	FootRight Foot = "right"
	FootBoth  Foot = "both"
)

var feet = []Foot{FootLeft, FootRight, FootBoth}

func (f Foot) Valid() bool     { return slices.Contains(feet, f) }
func (f Foot) Validate() error { return validateEnum("preferred_foot", string(f), feet) }

// ---------------------------------------------------------------------------

type TeamRole string

const (
	RoleCaptain TeamRole = "captain"
	RolePlayer  TeamRole = "player"
)

var teamRoles = []TeamRole{RoleCaptain, RolePlayer}

func (r TeamRole) Valid() bool     { return slices.Contains(teamRoles, r) }
func (r TeamRole) Validate() error { return validateEnum("role", string(r), teamRoles) }

// ---------------------------------------------------------------------------

type TournamentFormat string

const (
	FormatKnockout      TournamentFormat = "knockout"
	FormatLeague        TournamentFormat = "league"
	FormatGroupKnockout TournamentFormat = "group_knockout"
)

var tournamentFormats = []TournamentFormat{FormatKnockout, FormatLeague, FormatGroupKnockout}

func (f TournamentFormat) Valid() bool { return slices.Contains(tournamentFormats, f) }
func (f TournamentFormat) Validate() error {
	return validateEnum("format", string(f), tournamentFormats)
}

type TournamentStatus string

const (
	TournamentOpen      TournamentStatus = "open"
	TournamentFull      TournamentStatus = "full"
	TournamentOngoing   TournamentStatus = "ongoing"
	TournamentCompleted TournamentStatus = "completed"
	TournamentCancelled TournamentStatus = "cancelled"
)

var tournamentStatuses = []TournamentStatus{
	TournamentOpen, TournamentFull, TournamentOngoing, TournamentCompleted, TournamentCancelled,
}

func (t TournamentStatus) Valid() bool { return slices.Contains(tournamentStatuses, t) }
func (t TournamentStatus) Validate() error {
	return validateEnum("status", string(t), tournamentStatuses)
}

// AcceptsRegistrations reports whether a team may still enter. A full bracket
// is deliberately excluded: capacity is the database's call, and the service
// should not invite a registration it knows will be rejected.
func (t TournamentStatus) AcceptsRegistrations() bool { return t == TournamentOpen }

// ---------------------------------------------------------------------------

func validateEnum[T ~string](field string, value string, allowed []T) error {
	if value == "" {
		return Invalid(field, "A %s is required.", field)
	}
	if slices.Contains(allowed, T(value)) {
		return nil
	}
	names := make([]string, len(allowed))
	for i, a := range allowed {
		names[i] = string(a)
	}
	return Invalid(field, "%q is not a valid %s. Expected one of: %s.",
		value, field, joinOr(names))
}

func joinOr(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		out := ""
		for i, s := range items[:len(items)-1] {
			if i > 0 {
				out += ", "
			}
			out += s
		}
		return fmt.Sprintf("%s or %s", out, items[len(items)-1])
	}
}
