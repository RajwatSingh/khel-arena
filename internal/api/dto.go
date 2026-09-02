package api

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

// This file is the whole domain -> wire mapping, in one place so it can be
// reviewed as a unit. Domain types are never marshalled directly: domain.User
// carries an email and a verification timestamp that are fine in /v1/me and
// wrong the instant a user appears inside somebody else's view, and
// domain.Slot is nested where the frontend wants starts_at/ends_at flat.
//
// Money stays an int all the way out. PriceNPR is an int in the domain and in
// the schema; making it a float on the wire would introduce rounding that
// neither of the other two layers has.
//
// Times need no formatting help. domain.Slot normalises to UTC when it is
// constructed, so a plain time.Time marshals as RFC 3339 already.

// dateLayout is the calendar-date format the API speaks, on ?date= in and on
// availability responses out.
const dateLayout = "2006-01-02"

// ---------------------------------------------------------------- requests -

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// registerRequest is the signup form, which is domain.Registration plus the
// two player-card fields the form also collects.
//
// Registration itself has no room for them -- it is the set of things
// required to create an account -- so they are applied as a profile update
// once the account exists. They are named here rather than dropped because
// decode() rejects unknown fields: a field the form sends and this struct
// does not name is a 400, not a silent no-op.
type registerRequest struct {
	Email       string             `json:"email"`
	Username    string             `json:"username"`
	FullName    string             `json:"full_name"`
	Password    string             `json:"password"`
	AccountType domain.AccountType `json:"account_type"`

	Skill    *domain.SkillTier `json:"skill"`
	Position *domain.Position  `json:"position"`
}

// registration is the part of a signup that creates the account.
func (r registerRequest) registration() domain.Registration {
	return domain.Registration{
		Email:       r.Email,
		Username:    r.Username,
		FullName:    r.FullName,
		Password:    r.Password,
		AccountType: r.AccountType,
	}
}

// profile is the part of a signup that decorates the player card, and
// whether there is any of it at all.
func (r registerRequest) profile() (domain.ProfileUpdate, bool) {
	var p domain.ProfileUpdate
	touched := false

	if r.Skill != nil {
		p.Skill = r.Skill
		touched = true
	}
	if r.Position != nil {
		// **Position, because "clear my position" and "leave it alone" are
		// different requests and ProfileUpdate distinguishes them by the
		// outer pointer. The form sends null for "none", which reaches here
		// as a non-nil outer and a nil inner: set it to nothing.
		pos := *r.Position
		inner := &pos
		if pos == "" {
			inner = nil
		}
		p.Position = &inner
		touched = true
	}
	return p, touched
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// createBookingRequest is what a client may say about a booking it wants.
//
// Note what is missing: a price. The service resolves that from pricing rules
// server-side, and a user id -- that comes from the access token, never from
// the body, or one client could book as another.
type createBookingRequest struct {
	CourtID  uuid.UUID  `json:"court_id"`
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   time.Time  `json:"ends_at"`
	TeamID   *uuid.UUID `json:"team_id"`
	Note     string     `json:"note"`
}

// --------------------------------------------------------------- responses -

type userDTO struct {
	ID            uuid.UUID          `json:"id"`
	FullName      string             `json:"full_name"`
	Username      string             `json:"username"`
	Email         string             `json:"email"`
	AccountType   domain.AccountType `json:"account_type"`
	Skill         domain.SkillTier   `json:"skill"`
	Position      *domain.Position   `json:"position"`
	JerseyNumber  *int               `json:"jersey_number"`
	PreferredFoot *domain.Foot       `json:"preferred_foot"`
}

type sessionDTO struct {
	User        userDTO `json:"user"`
	AccessToken string  `json:"access_token"`
}

type gridSlotDTO struct {
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	PriceNPR  int       `json:"price_npr"`
	IsPeak    bool      `json:"is_peak"`
	Rule      string    `json:"rule"`
	IsBooked  bool      `json:"is_booked"`
	IsPast    bool      `json:"is_past"`
	Available bool      `json:"available"`
}

type availabilityDTO struct {
	CourtID uuid.UUID     `json:"court_id"`
	Date    string        `json:"date"`
	Slots   []gridSlotDTO `json:"slots"`
}

type bookingDTO struct {
	ID            uuid.UUID            `json:"id"`
	Reference     string               `json:"reference"`
	CourtID       uuid.UUID            `json:"court_id"`
	StartsAt      time.Time            `json:"starts_at"`
	EndsAt        time.Time            `json:"ends_at"`
	PriceNPR      int                  `json:"price_npr"`
	IsPeak        bool                 `json:"is_peak"`
	Status        domain.BookingStatus `json:"status"`
	Note          string               `json:"note"`
	HoldExpiresAt *time.Time           `json:"hold_expires_at"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// bookingDetailDTO embeds bookingDTO without a json tag, which is what makes
// encoding/json flatten it rather than nest it under a key. That is a
// deliberate choice, not the default falling out a convenient way.
type bookingDetailDTO struct {
	bookingDTO
	CourtName  string    `json:"court_name"`
	ArenaID    uuid.UUID `json:"arena_id"`
	ArenaName  string    `json:"arena_name"`
	ArenaSlug  string    `json:"arena_slug"`
	ArenaArea  string    `json:"arena_area"`
	OpenToJoin bool      `json:"open_to_join"`
}

// ------------------------------------------------------------ constructors -

func userDTOFromDomain(user domain.User) userDTO {
	return userDTO{
		ID:            user.ID,
		FullName:      user.FullName,
		Username:      user.Username,
		Email:         user.Email,
		AccountType:   user.AccountType,
		Skill:         user.Skill,
		Position:      user.Position,
		JerseyNumber:  user.JerseyNumber,
		PreferredFoot: user.PreferredFoot,
	}
}

// sessionDTOFromDomain deliberately drops the refresh token and both expiry
// timestamps. The refresh token leaves in an httpOnly cookie instead (see
// writeSession in auth.go), so it must not also be in a body that JavaScript
// can read -- that would give back exactly the XSS exposure the cookie buys.
func sessionDTOFromDomain(s service.Session) sessionDTO {
	return sessionDTO{
		User:        userDTOFromDomain(s.User),
		AccessToken: s.AccessToken,
	}
}

func gridSlotDTOFromDomain(slot domain.GridSlot) gridSlotDTO {
	return gridSlotDTO{
		StartsAt: slot.Slot.Start,
		EndsAt:   slot.Slot.End,
		PriceNPR: slot.PriceNPR,
		IsPeak:   slot.IsPeak,
		Rule:     rateLabel(slot.IsPeak),
		IsBooked: slot.IsBooked,
		IsPast:   slot.IsPast,
		// Computed once, here, from the domain's own rule. The client should
		// not have to re-derive "neither booked nor past" and risk deriving
		// it differently.
		Available: slot.Available(),
	}
}

func availabilityDTOFromDomain(courtID uuid.UUID, date time.Time, slots []domain.GridSlot) availabilityDTO {
	// Length 0 rather than nil: an arena closed on this date should serialize
	// as "slots": [], which the frontend can map over, not as null.
	out := make([]gridSlotDTO, 0, len(slots))
	for _, s := range slots {
		out = append(out, gridSlotDTOFromDomain(s))
	}
	return availabilityDTO{
		CourtID: courtID,
		Date:    date.Format(dateLayout),
		Slots:   out,
	}
}

func bookingDTOFromDomain(b domain.Booking) bookingDTO {
	return bookingDTO{
		ID:            b.ID,
		Reference:     bookingReference(b.ID),
		CourtID:       b.CourtID,
		StartsAt:      b.Slot.Start,
		EndsAt:        b.Slot.End,
		PriceNPR:      b.PriceNPR,
		IsPeak:        b.IsPeak,
		Status:        b.Status,
		Note:          b.Note,
		HoldExpiresAt: b.HoldExpiresAt,
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
	}
}

func bookingDetailDTOFromDomain(b domain.BookingDetail) bookingDetailDTO {
	return bookingDetailDTO{
		bookingDTO: bookingDTOFromDomain(b.Booking),
		CourtName:  b.CourtLabel,
		ArenaID:    b.ArenaID,
		ArenaName:  b.ArenaName,
		ArenaSlug:  b.ArenaSlug,
		ArenaArea:  b.ArenaArea,
		OpenToJoin: b.OpenToJoin,
	}
}

func bookingDetailDTOsFromDomain(bs []domain.BookingDetail) []bookingDetailDTO {
	out := make([]bookingDetailDTO, 0, len(bs))
	for _, b := range bs {
		out = append(out, bookingDetailDTOFromDomain(b))
	}
	return out
}

// bookingReference is a short, human-sayable handle for a booking -- what a
// player reads out at the gate instead of a UUID.
//
// It is derived from the id rather than stored, so it needs no column and no
// uniqueness machinery, at the cost of not being unique itself: four bytes
// collide at roughly one pair in 65,000 bookings. That is fine for reading
// aloud next to a name and a time, and is not an identifier -- every lookup
// still goes through the id.
func bookingReference(id uuid.UUID) string {
	return "KA-" + strings.ToUpper(hex.EncodeToString(id[:4]))
}

// rateLabel names the rate that applied, for the line the booking panel shows
// under the price.
//
// It is a stand-in. The real label lives on the winning domain.PricingRule
// ("Evening Peak"), but domain.GridSlot does not carry it out of BuildGrid,
// and adding a field there is a domain change this layer should not make on
// its own. See the note in README.
func rateLabel(isPeak bool) string {
	if isPeak {
		return "Peak rate"
	}
	return "Base rate"
}
