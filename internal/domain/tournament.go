package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Tournament is a bracket that teams register into.
//
// Capacity is enforced by the database (a CHECK on a trigger-maintained
// counter), not here: two captains claiming the last slot at the same instant
// is a race only the database can settle. The rules on this type are the ones
// that can be judged from a single tournament in isolation.
type Tournament struct {
	ID           uuid.UUID
	OrganizerID  uuid.UUID
	ArenaID      *uuid.UUID
	Name         string
	Slug         string
	Format       TournamentFormat
	SideCount    int
	SquadCap     int
	MaxTeams     int
	TeamCount    int
	EntryFeeNPR  int
	PrizePoolNPR int
	// PrizeSplit is the percentage to 1st, 2nd, 3rd... and must total 100.
	PrizeSplit  []int
	Skill       SkillTier
	Description string
	Rules       string
	StartsOn    time.Time // date only
	RegisterBy  time.Time // date only
	Status      TournamentStatus

	CreatedAt time.Time
	UpdatedAt time.Time

	// Populated when read for a listing.
	ArenaName         string
	ArenaArea         string
	OrganizerUsername string
}

func (t Tournament) OrganizedBy(userID uuid.UUID) bool { return t.OrganizerID == userID }
func (t Tournament) SlotsRemaining() int               { return max(0, t.MaxTeams-t.TeamCount) }

func (t *Tournament) Validate() error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Slug == "" {
		t.Slug = Slugify(t.Name)
	}
	if t.Format == "" {
		t.Format = FormatKnockout
	}
	if t.Skill == "" {
		t.Skill = SkillCasual
	}
	if len(t.PrizeSplit) == 0 {
		t.PrizeSplit = []int{60, 30, 10}
	}

	v := &Validation{}
	nameLen := len([]rune(t.Name))
	v.Check(nameLen >= 4, "name", "Give the tournament a name.")
	v.Check(nameLen <= 80, "name", "That name is too long.")
	v.Check(ValidSlug(t.Slug), "slug", "The web address may use lowercase letters, numbers and hyphens only.")
	v.Check(t.SideCount >= 4 && t.SideCount <= 7, "side_count", "Tournaments run from 4 to 7 a side.")
	v.Check(t.SquadCap >= 5 && t.SquadCap <= 15, "squad_cap", "Squad size must be between 5 and 15.")
	v.Check(t.MaxTeams >= 4 && t.MaxTeams <= 32, "max_teams", "A bracket takes between 4 and 32 teams.")
	v.Check(t.SquadCap >= t.SideCount, "squad_cap", "The squad must be at least as large as a team on the pitch.")
	v.Check(t.EntryFeeNPR >= 0, "entry_fee_npr", "The entry fee can't be negative.")
	v.Check(t.PrizePoolNPR >= 0, "prize_pool_npr", "The prize pool can't be negative.")
	v.Check(len([]rune(t.Description)) <= 500, "description", "Keep the description under 500 characters.")
	v.Check(len([]rune(t.Rules)) <= 2000, "rules", "Keep the rules under 2000 characters.")

	if err := t.Format.Validate(); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}
	if err := t.Skill.Validate(); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}

	if t.StartsOn.IsZero() {
		v.Add("starts_on", "When does the tournament start?")
	}
	if t.RegisterBy.IsZero() {
		v.Add("register_by", "When does registration close?")
	}
	if !t.StartsOn.IsZero() && !t.RegisterBy.IsZero() {
		v.Check(!t.RegisterBy.After(t.StartsOn), "register_by",
			"Registration has to close on or before the first match.")
	}

	if err := validatePrizeSplit(t.PrizeSplit); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}

	// A knockout bracket needs a power-of-two field to avoid byes that the
	// scheduler does not model.
	if t.Format == FormatKnockout && t.MaxTeams > 0 && t.MaxTeams&(t.MaxTeams-1) != 0 {
		v.Add("max_teams", "A knockout bracket needs 4, 8, 16 or 32 teams.")
	}

	return v.Err()
}

func validatePrizeSplit(split []int) error {
	if len(split) == 0 {
		return Invalid("prize_split", "Say how the prize money is shared.")
	}
	if len(split) > 8 {
		return Invalid("prize_split", "Split the prize between at most 8 places.")
	}
	total := 0
	for i, share := range split {
		if share < 0 {
			return Invalid("prize_split", "Prize shares can't be negative.")
		}
		if i > 0 && share > split[i-1] {
			return Invalid("prize_split", "Each place should win no more than the one above it.")
		}
		total += share
	}
	if total != 100 {
		return Invalid("prize_split", "The prize shares add up to %d%%, not 100%%.", total)
	}
	return nil
}

// PrizeFor returns the payout in NPR for a finishing position, 1-indexed.
// Positions outside the split win nothing.
//
// Rounding is toward zero per place, so the pool is never over-committed;
// any remainder from the rounding stays with the organizer.
func (t Tournament) PrizeFor(place int) int {
	if place < 1 || place > len(t.PrizeSplit) {
		return 0
	}
	return t.PrizePoolNPR * t.PrizeSplit[place-1] / 100
}

// CanAcceptRegistration states whether a team may still enter, judged from
// this tournament alone. The database has the final word on capacity.
func (t Tournament) CanAcceptRegistration(now time.Time) error {
	switch t.Status {
	case TournamentCancelled:
		return Conflict("This tournament was cancelled.")
	case TournamentOngoing, TournamentCompleted:
		return Conflict("This tournament has already started.")
	case TournamentFull:
		return Conflict("This tournament is full.")
	}
	// RegisterBy is a date: registration is open through the end of that day
	// in the tournament's local reckoning.
	deadline := t.RegisterBy.AddDate(0, 0, 1)
	if !now.Before(deadline) {
		return Conflict("Registration for this tournament has closed.")
	}
	if t.TeamCount >= t.MaxTeams {
		return Conflict("This tournament is full.")
	}
	return nil
}

// Registration of a team in a tournament.
type TournamentEntry struct {
	TournamentID uuid.UUID
	TeamID       uuid.UUID
	RegisteredBy uuid.UUID
	Paid         bool
	RegisteredAt time.Time

	// Populated when a bracket is read for display.
	TeamName string
	TeamTag  string
	CrestURL string
}
