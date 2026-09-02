package api

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
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
		Rule:     slot.RuleLabel,
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

// ----------------------------------------------------------------- arenas --

type arenaDTO struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Area        string    `json:"area"`
	City        string    `json:"city"`
	Description string    `json:"description"`
	CoverURL    string    `json:"cover_url"`
	Amenities   []string  `json:"amenities"`
	Phone       string    `json:"phone"`
	OpensAt     string    `json:"opens_at"`
	ClosesAt    string    `json:"closes_at"`
	Rating      *float64  `json:"rating"`
	ReviewCount int       `json:"review_count"`
	Lat         *float64  `json:"lat"`
	Lng         *float64  `json:"lng"`
}

// arenaListingDTO is a row of the index: the venue plus the two figures the
// listing shows. Both are computed in SQL, not by counting courts here.
type arenaListingDTO struct {
	arenaDTO
	CourtCount   int            `json:"court_count"`
	FromPriceNPR int            `json:"from_price_npr"`
	Sports       []domain.Sport `json:"sports"`
	// IsActive is always true on the public index, which lists nothing else.
	// It carries the answer for the owner view, which does include closed
	// venues -- and without it an owner who closed one could never tell, or
	// reopen it.
	IsActive bool `json:"is_active"`
}

type arenaDetailDTO struct {
	arenaDTO
	Courts []courtDTO `json:"courts"`
}

type courtDTO struct {
	ID           uuid.UUID        `json:"id"`
	Name         string           `json:"name"`
	Sport        domain.Sport     `json:"sport"`
	Format       string           `json:"format"`
	Surface      string           `json:"surface"`
	SideCount    int              `json:"side_count"`
	BasePriceNPR int              `json:"base_price_npr"`
	Rules        []pricingRuleDTO `json:"rules"`
}

type pricingRuleDTO struct {
	// The id is what DELETE /v1/owner/pricing/{ruleID} is addressed by, and
	// reading the rules is the only way a client can learn it. Without this
	// the delete endpoint is unreachable.
	ID        uuid.UUID `json:"id"`
	Label     string    `json:"label"`
	Days      []int     `json:"days"`
	StartHour int       `json:"start_hour"`
	EndHour   int       `json:"end_hour"`
	PriceNPR  int       `json:"price_npr"`
	IsPeak    bool      `json:"is_peak"`
	Priority  int       `json:"priority"`
}

// ledgerDTO is the city-wide grid for one date.
type ledgerDTO struct {
	Date        string         `json:"date"`
	Rows        []ledgerRowDTO `json:"rows"`
	OpenHours   int            `json:"open_hours"`
	CheapestNPR *int           `json:"cheapest_npr"`
}

type ledgerRowDTO struct {
	CourtID   uuid.UUID     `json:"court_id"`
	CourtName string        `json:"court_name"`
	Sport     domain.Sport  `json:"sport"`
	Format    string        `json:"format"`
	ArenaID   uuid.UUID     `json:"arena_id"`
	ArenaName string        `json:"arena_name"`
	ArenaSlug string        `json:"arena_slug"`
	ArenaArea string        `json:"arena_area"`
	Slots     []gridSlotDTO `json:"slots"`
}

func arenaDTOFromDomain(a domain.Arena) arenaDTO {
	// Amenities is a text[] that can arrive nil. An empty slice serializes as
	// [], which a client can iterate; null forces every caller to guard.
	amenities := a.Amenities
	if amenities == nil {
		amenities = []string{}
	}

	return arenaDTO{
		ID:          a.ID,
		Slug:        a.Slug,
		Name:        a.Name,
		Area:        a.Area,
		City:        a.City,
		Description: a.Description,
		CoverURL:    a.CoverURL,
		Amenities:   amenities,
		Phone:       a.Phone,
		// DayTime already knows how to render itself; reformatting the clock
		// by hand here would be a second opinion about what "18:00" looks like.
		OpensAt:     a.OpensAt.String(),
		ClosesAt:    a.ClosesAt.String(),
		Rating:      a.Rating,
		ReviewCount: a.ReviewCount,
		Lat:         a.Lat,
		Lng:         a.Lng,
	}
}

func arenaListingDTOsFromDomain(listings []postgres.ArenaListing) []arenaListingDTO {
	out := make([]arenaListingDTO, 0, len(listings))
	for _, l := range listings {
		sports := l.Sports
		if sports == nil {
			sports = []domain.Sport{}
		}
		out = append(out, arenaListingDTO{
			arenaDTO:     arenaDTOFromDomain(l.Arena),
			CourtCount:   l.CourtCount,
			FromPriceNPR: l.FromPriceNPR,
			Sports:       sports,
			IsActive:     l.IsActive,
		})
	}
	return out
}

func arenaDetailDTOFromDomain(d postgres.ArenaDetail) arenaDetailDTO {
	courts := make([]courtDTO, 0, len(d.Courts))
	for _, c := range d.Courts {
		courts = append(courts, courtDTOFromDomain(c))
	}
	return arenaDetailDTO{arenaDTO: arenaDTOFromDomain(d.Arena), Courts: courts}
}

func courtDTOFromDomain(c postgres.CourtWithRules) courtDTO {
	rules := make([]pricingRuleDTO, 0, len(c.PricingRules))
	for _, r := range c.PricingRules {
		rules = append(rules, pricingRuleDTOFromDomain(r))
	}

	return courtDTO{
		ID:           c.ID,
		Name:         c.Label,
		Sport:        c.Sport,
		Format:       courtFormat(c),
		Surface:      c.Surface,
		SideCount:    c.SideCount,
		BasePriceNPR: c.BasePriceNPR,
		Rules:        rules,
	}
}

func pricingRuleDTOFromDomain(r domain.PricingRule) pricingRuleDTO {
	// ISO weekday numbers on the wire, matching what the schema stores and
	// what the interface's own date helpers speak. Go's Sunday-is-zero
	// counting is a Go detail and stops at this boundary.
	days := make([]int, 0, len(r.Days))
	for _, d := range r.Days {
		days = append(days, domain.ISOWeekday(d))
	}

	return pricingRuleDTO{
		ID:        r.ID,
		Label:     r.Label,
		Days:      days,
		StartHour: r.StartHour,
		EndHour:   r.EndHour,
		PriceNPR:  r.PriceNPR,
		IsPeak:    r.IsPeak,
		Priority:  r.Priority,
	}
}

// courtFormat is the venue's name for the pitch, falling back to what the
// side count implies.
//
// The stored label wins because only the owner knows a court is called "Full
// court" rather than "5-a-side". The fallback keeps every court readable
// until one is set, which matters because the column was added after the
// courts were.
func courtFormat(c postgres.CourtWithRules) string {
	if c.Format != "" {
		return c.Format
	}
	if c.SideCount > 0 {
		return strconv.Itoa(c.SideCount) + "-a-side"
	}
	return ""
}

func ledgerDTOFromDomain(l service.Ledger) ledgerDTO {
	rows := make([]ledgerRowDTO, 0, len(l.Rows))
	for _, r := range l.Rows {
		slots := make([]gridSlotDTO, 0, len(r.Slots))
		for _, s := range r.Slots {
			slots = append(slots, gridSlotDTOFromDomain(s))
		}

		rows = append(rows, ledgerRowDTO{
			CourtID:   r.ID,
			CourtName: r.Label,
			Sport:     r.Sport,
			Format:    courtFormat(r.CourtWithRules),
			// Carried on every row because the ledger links each cell to its
			// arena and labels it with the venue. A row without these renders
			// a nameless link to nowhere.
			ArenaID:   r.ArenaID,
			ArenaName: r.ArenaName,
			ArenaSlug: r.ArenaSlug,
			ArenaArea: r.ArenaArea,
			Slots:     slots,
		})
	}

	return ledgerDTO{
		Date:        l.Date.Format(dateLayout),
		Rows:        rows,
		OpenHours:   l.OpenHours,
		CheapestNPR: l.CheapestNPR,
	}
}

// --------------------------------------------------------------- payments --

type checkoutRequest struct {
	Provider domain.PaymentProvider `json:"provider"`
}

// checkoutDTO tells a client how to hand the player to a gateway.
//
// Method and Fields exist because providers differ: Khalti returns a URL to
// redirect to, eSewa wants a signed form POSTed to it. Expressing both lets
// the interface hand either to a browser without a per-provider branch.
type checkoutDTO struct {
	PaymentID uuid.UUID              `json:"payment_id"`
	Provider  domain.PaymentProvider `json:"provider"`
	AmountNPR int                    `json:"amount_npr"`
	Method    string                 `json:"method"`
	URL       string                 `json:"url"`
	Fields    map[string]string      `json:"fields,omitempty"`
}

// paymentDTO is the state of one payment attempt.
//
// TransactionUUID and RawResponse are deliberately absent. The first is the
// identifier a gateway callback is addressed by -- handing it to a client
// invites someone to go and settle a payment out of band -- and the second is
// the gateway's own reply, kept for an operator to read in an incident, not
// for a browser.
type paymentDTO struct {
	ID         uuid.UUID              `json:"id"`
	BookingID  uuid.UUID              `json:"booking_id"`
	Provider   domain.PaymentProvider `json:"provider"`
	AmountNPR  int                    `json:"amount_npr"`
	Status     domain.PaymentStatus   `json:"status"`
	CreatedAt  time.Time              `json:"created_at"`
	VerifiedAt *time.Time             `json:"verified_at"`
}

func checkoutDTOFromDomain(c payment.Checkout, p domain.Payment) checkoutDTO {
	return checkoutDTO{
		PaymentID: p.ID,
		Provider:  p.Provider,
		AmountNPR: p.AmountNPR,
		Method:    c.Method,
		URL:       c.URL,
		Fields:    c.Fields,
	}
}

func paymentDTOFromDomain(p domain.Payment) paymentDTO {
	return paymentDTO{
		ID:         p.ID,
		BookingID:  p.BookingID,
		Provider:   p.Provider,
		AmountNPR:  p.AmountNPR,
		Status:     p.Status,
		CreatedAt:  p.CreatedAt,
		VerifiedAt: p.VerifiedAt,
	}
}

// ------------------------------------------------------------------ owner --

type activeRequest struct {
	Active bool `json:"active"`
}

// arenaWriteRequest is the venue form.
//
// No owner_id and no slug. The owner is the caller, and the slug is in every
// link anyone has shared -- changing it silently breaks all of them, which
// wants a redirect table and a deliberate decision, not a form field.
type arenaWriteRequest struct {
	Name        string   `json:"name"`
	Area        string   `json:"area"`
	City        string   `json:"city"`
	Description string   `json:"description"`
	CoverURL    string   `json:"cover_url"`
	Amenities   []string `json:"amenities"`
	Phone       string   `json:"phone"`
	OpensAt     string   `json:"opens_at"`
	ClosesAt    string   `json:"closes_at"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
}

func (r arenaWriteRequest) arena() domain.Arena {
	// A malformed clock time becomes the zero DayTime, which Validate then
	// rejects with a message about the field. Parsing here and reporting the
	// error separately would say the same thing twice, in two voices.
	opens, _ := domain.ParseDayTime(r.OpensAt)
	closes, _ := domain.ParseDayTime(r.ClosesAt)

	return domain.Arena{
		Name:        r.Name,
		Area:        r.Area,
		City:        r.City,
		Description: r.Description,
		CoverURL:    r.CoverURL,
		Amenities:   r.Amenities,
		Phone:       r.Phone,
		OpensAt:     opens,
		ClosesAt:    closes,
		Lat:         r.Lat,
		Lng:         r.Lng,
	}
}

// courtWriteRequest is the court form. The arena comes from the path.
type courtWriteRequest struct {
	Name         string       `json:"name"`
	Sport        domain.Sport `json:"sport"`
	Format       string       `json:"format"`
	Surface      string       `json:"surface"`
	SideCount    int          `json:"side_count"`
	BasePriceNPR int          `json:"base_price_npr"`
}

func (r courtWriteRequest) court() domain.Court {
	return domain.Court{
		Label:        r.Name,
		Sport:        r.Sport,
		Surface:      r.Surface,
		SideCount:    r.SideCount,
		BasePriceNPR: r.BasePriceNPR,
	}
}

// pricingRuleWriteRequest is a rate window. Days are ISO numbers on the wire
// (1 = Monday), matching the schema and the interface's own date helpers.
type pricingRuleWriteRequest struct {
	Label     string `json:"label"`
	Days      []int  `json:"days"`
	StartHour int    `json:"start_hour"`
	EndHour   int    `json:"end_hour"`
	PriceNPR  int    `json:"price_npr"`
	IsPeak    bool   `json:"is_peak"`
	Priority  int    `json:"priority"`
}

func (r pricingRuleWriteRequest) rule() (domain.PricingRule, error) {
	days := make([]time.Weekday, 0, len(r.Days))
	for _, iso := range r.Days {
		day, err := domain.WeekdayFromISO(iso)
		if err != nil {
			return domain.PricingRule{}, err
		}
		days = append(days, day)
	}

	return domain.PricingRule{
		Label:     r.Label,
		Days:      days,
		StartHour: r.StartHour,
		EndHour:   r.EndHour,
		PriceNPR:  r.PriceNPR,
		IsPeak:    r.IsPeak,
		Priority:  r.Priority,
	}, nil
}

// ownerPaymentDTO is a payment with the context a venue needs to reconcile it.
//
// It carries the player's name, which the player-facing paymentDTO does not:
// an owner reconciling the till has to know who to ask about, and the booking
// is theirs to see. TransactionUUID and RawResponse stay out for the same
// reason as always -- one addresses a gateway callback, the other is the
// gateway's own reply.
type ownerPaymentDTO struct {
	paymentDTO
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	CourtName      string    `json:"court_name"`
	PlayerName     string    `json:"player_name"`
	PlayerUsername string    `json:"player_username"`
}

func ownerPaymentDTOsFromDomain(payments []postgres.OwnerPayment) []ownerPaymentDTO {
	out := make([]ownerPaymentDTO, 0, len(payments))
	for _, p := range payments {
		out = append(out, ownerPaymentDTO{
			paymentDTO:     paymentDTOFromDomain(p.Payment),
			StartsAt:       p.Slot.Start,
			EndsAt:         p.Slot.End,
			CourtName:      p.CourtLabel,
			PlayerName:     p.PlayerName,
			PlayerUsername: p.PlayerUsername,
		})
	}
	return out
}

// ------------------------------------------------------------------ teams --

type teamWriteRequest struct {
	Name      string     `json:"name"`
	Tag       string     `json:"tag"`
	CrestURL  string     `json:"crest_url"`
	HomeArena *uuid.UUID `json:"home_arena"`
}

func (r teamWriteRequest) team() domain.Team {
	return domain.Team{
		Name:      r.Name,
		Tag:       r.Tag,
		CrestURL:  r.CrestURL,
		HomeArena: r.HomeArena,
	}
}

type joinTeamRequest struct {
	Code string `json:"code"`
}

type memberRequest struct {
	UserID uuid.UUID `json:"user_id"`
}

type joinCodeDTO struct {
	JoinCode string `json:"join_code"`
}

// teamDTO carries no join code. That code is how somebody gets onto the
// roster, so it appears only on teamDetailDTO and only for members -- a squad
// listing that leaked it would make every team open to anyone who saw one.
type teamDTO struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Tag         string     `json:"tag"`
	CrestURL    string     `json:"crest_url"`
	CaptainID   uuid.UUID  `json:"captain_id"`
	HomeArena   *uuid.UUID `json:"home_arena"`
	MemberCount int        `json:"member_count"`
	CreatedAt   time.Time  `json:"created_at"`
}

type teamDetailDTO struct {
	teamDTO
	Members []memberDTO `json:"members"`
	// JoinCode is present only for members; blank otherwise.
	JoinCode string `json:"join_code,omitempty"`
}

// memberDTO is a roster entry. The player is a UserSummary -- the public card
// -- not a userDTO: an email address has no business appearing in somebody
// else's squad list.
type memberDTO struct {
	UserID         uuid.UUID       `json:"user_id"`
	Username       string          `json:"username"`
	FullName       string          `json:"full_name"`
	AvatarURL      string          `json:"avatar_url"`
	CommunityScore int             `json:"community_score"`
	Role           domain.TeamRole `json:"role"`
	JoinedAt       time.Time       `json:"joined_at"`
}

func teamDTOFromDomain(t domain.Team) teamDTO {
	return teamDTO{
		ID:          t.ID,
		Name:        t.Name,
		Tag:         t.Tag,
		CrestURL:    t.CrestURL,
		CaptainID:   t.CaptainID,
		HomeArena:   t.HomeArena,
		MemberCount: t.MemberCount,
		CreatedAt:   t.CreatedAt,
	}
}

func teamDTOsFromDomain(teams []domain.Team) []teamDTO {
	out := make([]teamDTO, 0, len(teams))
	for _, t := range teams {
		out = append(out, teamDTOFromDomain(t))
	}
	return out
}

func memberDTOFromDomain(m domain.Member) memberDTO {
	dto := memberDTO{UserID: m.UserID, Role: m.Role, JoinedAt: m.JoinedAt}
	if m.User != nil {
		dto.Username = m.User.Username
		dto.FullName = m.User.FullName
		dto.AvatarURL = m.User.AvatarURL
		dto.CommunityScore = m.User.CommunityScore
	}
	return dto
}

func teamDetailDTOFromDomain(t service.TeamWithRoster) teamDetailDTO {
	members := make([]memberDTO, 0, len(t.Members))
	for _, m := range t.Members {
		members = append(members, memberDTOFromDomain(m))
	}
	return teamDetailDTO{
		teamDTO:  teamDTOFromDomain(t.Team),
		Members:  members,
		JoinCode: t.JoinCode,
	}
}

// ------------------------------------------------------------ matchmaking --

type callWriteRequest struct {
	BookingID     *uuid.UUID       `json:"booking_id"`
	ArenaID       *uuid.UUID       `json:"arena_id"`
	Title         string           `json:"title"`
	Description   string           `json:"description"`
	NeededPlayers int              `json:"needed_players"`
	Skill         domain.SkillTier `json:"skill"`
	StartsAt      time.Time        `json:"starts_at"`
}

func (r callWriteRequest) call() domain.Call {
	return domain.Call{
		BookingID:     r.BookingID,
		ArenaID:       r.ArenaID,
		Title:         r.Title,
		Description:   r.Description,
		NeededPlayers: r.NeededPlayers,
		Skill:         r.Skill,
		StartsAt:      r.StartsAt,
	}
}

type respondRequest struct {
	Message string `json:"message"`
}

type callDTO struct {
	ID             uuid.UUID                `json:"id"`
	AuthorID       uuid.UUID                `json:"author_id"`
	Author         userSummaryDTO           `json:"author"`
	BookingID      *uuid.UUID               `json:"booking_id"`
	ArenaID        *uuid.UUID               `json:"arena_id"`
	ArenaName      string                   `json:"arena_name"`
	ArenaArea      string                   `json:"arena_area"`
	Title          string                   `json:"title"`
	Description    string                   `json:"description"`
	NeededPlayers  int                      `json:"needed_players"`
	FilledPlayers  int                      `json:"filled_players"`
	SpotsRemaining int                      `json:"spots_remaining"`
	Skill          domain.SkillTier         `json:"skill"`
	StartsAt       time.Time                `json:"starts_at"`
	Status         domain.MatchmakingStatus `json:"status"`
	CreatedAt      time.Time                `json:"created_at"`
}

// callDetailDTO adds the responses, which are present only for the author:
// who has asked to join is a list of people's plans, not a public roster.
type callDetailDTO struct {
	callDTO
	Responses    []responseDTO `json:"responses,omitempty"`
	YouResponded bool          `json:"you_responded"`
}

type responseDTO struct {
	UserID    uuid.UUID      `json:"user_id"`
	Responder userSummaryDTO `json:"responder"`
	Message   string         `json:"message"`
	Accepted  bool           `json:"accepted"`
	CreatedAt time.Time      `json:"created_at"`
}

// userSummaryDTO is the public player card: what may appear inside somebody
// else's view. No email, no verification state, no account type.
type userSummaryDTO struct {
	ID             uuid.UUID `json:"id"`
	Username       string    `json:"username"`
	FullName       string    `json:"full_name"`
	AvatarURL      string    `json:"avatar_url"`
	CommunityScore int       `json:"community_score"`
}

func userSummaryDTOFromDomain(u *domain.UserSummary) userSummaryDTO {
	if u == nil {
		return userSummaryDTO{}
	}
	return userSummaryDTO{
		ID:             u.ID,
		Username:       u.Username,
		FullName:       u.FullName,
		AvatarURL:      u.AvatarURL,
		CommunityScore: u.CommunityScore,
	}
}

func callDTOFromDomain(c domain.Call) callDTO {
	return callDTO{
		ID:             c.ID,
		AuthorID:       c.AuthorID,
		Author:         userSummaryDTOFromDomain(c.Author),
		BookingID:      c.BookingID,
		ArenaID:        c.ArenaID,
		ArenaName:      c.ArenaName,
		ArenaArea:      c.ArenaArea,
		Title:          c.Title,
		Description:    c.Description,
		NeededPlayers:  c.NeededPlayers,
		FilledPlayers:  c.FilledPlayers,
		SpotsRemaining: c.SpotsRemaining(),
		Skill:          c.Skill,
		StartsAt:       c.StartsAt,
		Status:         c.Status,
		CreatedAt:      c.CreatedAt,
	}
}

func callDTOsFromDomain(calls []domain.Call) []callDTO {
	out := make([]callDTO, 0, len(calls))
	for _, c := range calls {
		out = append(out, callDTOFromDomain(c))
	}
	return out
}

func callDetailDTOFromDomain(c service.CallWithResponses) callDetailDTO {
	var responses []responseDTO
	for _, r := range c.Responses {
		responses = append(responses, responseDTO{
			UserID:    r.UserID,
			Responder: userSummaryDTOFromDomain(r.Responder),
			Message:   r.Message,
			Accepted:  r.Accepted,
			CreatedAt: r.CreatedAt,
		})
	}

	return callDetailDTO{
		callDTO:      callDTOFromDomain(c.Call),
		Responses:    responses,
		YouResponded: c.YouResponded,
	}
}

// ------------------------------------------------------------ tournaments --

type tournamentWriteRequest struct {
	ArenaID      *uuid.UUID              `json:"arena_id"`
	Name         string                  `json:"name"`
	Slug         string                  `json:"slug"`
	Format       domain.TournamentFormat `json:"format"`
	SideCount    int                     `json:"side_count"`
	SquadCap     int                     `json:"squad_cap"`
	MaxTeams     int                     `json:"max_teams"`
	EntryFeeNPR  int                     `json:"entry_fee_npr"`
	PrizePoolNPR int                     `json:"prize_pool_npr"`
	PrizeSplit   []int                   `json:"prize_split"`
	Skill        domain.SkillTier        `json:"skill"`
	Description  string                  `json:"description"`
	Rules        string                  `json:"rules"`
	StartsOn     string                  `json:"starts_on"`
	RegisterBy   string                  `json:"register_by"`
}

func (r tournamentWriteRequest) tournament() (domain.Tournament, error) {
	// Dates, not instants: a tournament starts on a day, and the hour it
	// kicks off is a fixture detail rather than a property of the bracket.
	startsOn, err := time.Parse(dateLayout, r.StartsOn)
	if err != nil {
		return domain.Tournament{}, domain.Invalid("starts_on", "Dates look like 2026-08-14.")
	}
	registerBy, err := time.Parse(dateLayout, r.RegisterBy)
	if err != nil {
		return domain.Tournament{}, domain.Invalid("register_by", "Dates look like 2026-08-14.")
	}

	return domain.Tournament{
		ArenaID:      r.ArenaID,
		Name:         r.Name,
		Slug:         r.Slug,
		Format:       r.Format,
		SideCount:    r.SideCount,
		SquadCap:     r.SquadCap,
		MaxTeams:     r.MaxTeams,
		EntryFeeNPR:  r.EntryFeeNPR,
		PrizePoolNPR: r.PrizePoolNPR,
		PrizeSplit:   r.PrizeSplit,
		Skill:        r.Skill,
		Description:  r.Description,
		Rules:        r.Rules,
		StartsOn:     startsOn,
		RegisterBy:   registerBy,
	}, nil
}

type registerTeamRequest struct {
	TeamID uuid.UUID `json:"team_id"`
}

type paidRequest struct {
	Paid bool `json:"paid"`
}

type tournamentStatusRequest struct {
	Status domain.TournamentStatus `json:"status"`
}

type tournamentDTO struct {
	ID                uuid.UUID               `json:"id"`
	Slug              string                  `json:"slug"`
	Name              string                  `json:"name"`
	OrganizerID       uuid.UUID               `json:"organizer_id"`
	OrganizerUsername string                  `json:"organizer_username"`
	ArenaID           *uuid.UUID              `json:"arena_id"`
	ArenaName         string                  `json:"arena_name"`
	ArenaArea         string                  `json:"arena_area"`
	Format            domain.TournamentFormat `json:"format"`
	SideCount         int                     `json:"side_count"`
	SquadCap          int                     `json:"squad_cap"`
	MaxTeams          int                     `json:"max_teams"`
	TeamCount         int                     `json:"team_count"`
	SlotsRemaining    int                     `json:"slots_remaining"`
	EntryFeeNPR       int                     `json:"entry_fee_npr"`
	PrizePoolNPR      int                     `json:"prize_pool_npr"`
	PrizeSplit        []int                   `json:"prize_split"`
	Skill             domain.SkillTier        `json:"skill"`
	Description       string                  `json:"description"`
	Rules             string                  `json:"rules"`
	StartsOn          string                  `json:"starts_on"`
	RegisterBy        string                  `json:"register_by"`
	Status            domain.TournamentStatus `json:"status"`
}

type tournamentDetailDTO struct {
	tournamentDTO
	Entries []tournamentEntryDTO `json:"entries"`
}

// tournamentEntryDTO carries `paid`, which is between the organiser and the
// captain but is not a secret from the other teams: everyone in a bracket can
// see who has actually entered.
type tournamentEntryDTO struct {
	TeamID       uuid.UUID `json:"team_id"`
	TeamName     string    `json:"team_name"`
	TeamTag      string    `json:"team_tag"`
	CrestURL     string    `json:"crest_url"`
	Paid         bool      `json:"paid"`
	RegisteredAt time.Time `json:"registered_at"`
}

func tournamentDTOFromDomain(t domain.Tournament) tournamentDTO {
	split := t.PrizeSplit
	if split == nil {
		split = []int{}
	}

	return tournamentDTO{
		ID:                t.ID,
		Slug:              t.Slug,
		Name:              t.Name,
		OrganizerID:       t.OrganizerID,
		OrganizerUsername: t.OrganizerUsername,
		ArenaID:           t.ArenaID,
		ArenaName:         t.ArenaName,
		ArenaArea:         t.ArenaArea,
		Format:            t.Format,
		SideCount:         t.SideCount,
		SquadCap:          t.SquadCap,
		MaxTeams:          t.MaxTeams,
		TeamCount:         t.TeamCount,
		SlotsRemaining:    t.SlotsRemaining(),
		EntryFeeNPR:       t.EntryFeeNPR,
		PrizePoolNPR:      t.PrizePoolNPR,
		PrizeSplit:        split,
		Skill:             t.Skill,
		Description:       t.Description,
		Rules:             t.Rules,
		StartsOn:          t.StartsOn.Format(dateLayout),
		RegisterBy:        t.RegisterBy.Format(dateLayout),
		Status:            t.Status,
	}
}

func tournamentDTOsFromDomain(ts []domain.Tournament) []tournamentDTO {
	out := make([]tournamentDTO, 0, len(ts))
	for _, t := range ts {
		out = append(out, tournamentDTOFromDomain(t))
	}
	return out
}

func tournamentDetailDTOFromDomain(t service.TournamentWithEntries) tournamentDetailDTO {
	entries := make([]tournamentEntryDTO, 0, len(t.Entries))
	for _, e := range t.Entries {
		entries = append(entries, tournamentEntryDTO{
			TeamID:       e.TeamID,
			TeamName:     e.TeamName,
			TeamTag:      e.TeamTag,
			CrestURL:     e.CrestURL,
			Paid:         e.Paid,
			RegisteredAt: e.RegisteredAt,
		})
	}
	return tournamentDetailDTO{
		tournamentDTO: tournamentDTOFromDomain(t.Tournament),
		Entries:       entries,
	}
}
