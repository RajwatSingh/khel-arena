package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxNeededPlayers is a full 7-a-side squad plus substitutes.
const MaxNeededPlayers = 15

// Call is a post on the find-a-player board.
//
// It comes in two shapes. With a BookingID it opens a court the author has
// already paid for ("we have Dhuku at 8, need 2 more"). Without one it is an
// open call for a pickup game nobody has booked yet.
type Call struct {
	ID            uuid.UUID
	AuthorID      uuid.UUID
	BookingID     *uuid.UUID
	ArenaID       *uuid.UUID
	Title         string
	Description   string
	NeededPlayers int
	FilledPlayers int
	Skill         SkillTier
	StartsAt      time.Time
	Status        MatchmakingStatus

	CreatedAt time.Time
	UpdatedAt time.Time

	// Populated when read for the feed.
	Author    *UserSummary
	ArenaName string
	ArenaArea string
}

// Response is a player asking to join a call. `Accepted` is the author's
// decision; until then it is a pending request.
type Response struct {
	CallID    uuid.UUID
	UserID    uuid.UUID
	Message   string
	Accepted  bool
	CreatedAt time.Time

	Responder *UserSummary
}

func (c Call) AuthoredBy(userID uuid.UUID) bool { return c.AuthorID == userID }
func (c Call) SpotsRemaining() int              { return max(0, c.NeededPlayers-c.FilledPlayers) }

// IsAttachedToBooking reports whether this call opens a real court.
func (c Call) IsAttachedToBooking() bool { return c.BookingID != nil }

func (c *Call) Validate() error {
	c.Title = strings.TrimSpace(c.Title)
	c.Description = strings.TrimSpace(c.Description)
	if c.Skill == "" {
		c.Skill = SkillCasual
	}

	v := &Validation{}
	titleLen := len([]rune(c.Title))
	v.Check(titleLen >= 3, "title", "Give the game a title.")
	v.Check(titleLen <= 120, "title", "Keep the title under 120 characters.")
	v.Check(len([]rune(c.Description)) <= 280, "description", "Keep the description under 280 characters.")
	v.Check(c.NeededPlayers >= 1 && c.NeededPlayers <= MaxNeededPlayers,
		"needed_players", "Ask for between 1 and %d players.", MaxNeededPlayers)
	v.Check(!c.StartsAt.IsZero(), "starts_at", "When does the game kick off?")

	if err := c.Skill.Validate(); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}
	return v.Err()
}

// CanBeJoinedBy states whether a player may ask to join, and why not.
func (c Call) CanBeJoinedBy(userID uuid.UUID, now time.Time) error {
	if c.AuthoredBy(userID) {
		return Conflict("You posted this game.")
	}
	if !c.Status.AcceptsResponses() {
		switch c.Status {
		case MatchmakingFilled:
			return Conflict("This game is already full.")
		case MatchmakingCancelled:
			return Conflict("This game was cancelled.")
		case MatchmakingExpired:
			return Conflict("This game has already kicked off.")
		default:
			return Conflict("This game is no longer taking players.")
		}
	}
	if !c.StartsAt.After(now) {
		return Conflict("This game has already kicked off.")
	}
	if c.SpotsRemaining() == 0 {
		return Conflict("This game is already full.")
	}
	return nil
}

// CanAcceptResponse states whether the author may accept another player.
func (c Call) CanAcceptResponse(actorID uuid.UUID, now time.Time) error {
	if !c.AuthoredBy(actorID) {
		return Forbidden("Only the player who posted this game can accept people.")
	}
	if c.Status == MatchmakingCancelled {
		return Conflict("This game was cancelled.")
	}
	if !c.StartsAt.After(now) {
		return Conflict("This game has already kicked off.")
	}
	if c.SpotsRemaining() == 0 {
		return Conflict("Every spot in this game is taken.")
	}
	return nil
}

// CanBeManagedBy states whether a user may edit or close this call.
func (c Call) CanBeManagedBy(actorID uuid.UUID) error {
	if !c.AuthoredBy(actorID) {
		return NotFound("That game doesn't exist.")
	}
	return nil
}

// Expired reports whether kickoff has passed while the call was still open.
// The board shows these as expired rather than letting them linger.
func (c Call) Expired(now time.Time) bool {
	return c.Status == MatchmakingOpen && !c.StartsAt.After(now)
}

func (r *Response) Validate() error {
	r.Message = strings.TrimSpace(r.Message)
	if len([]rune(r.Message)) > 200 {
		return Invalid("message", "Keep your message under 200 characters.")
	}
	return nil
}
