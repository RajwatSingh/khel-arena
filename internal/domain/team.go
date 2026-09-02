package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Team is a squad with a captain and a roster.
type Team struct {
	ID          uuid.UUID
	Name        string
	Tag         string // 2-5 uppercase alphanumerics, e.g. "KTM"
	CrestURL    string
	CaptainID   uuid.UUID
	HomeArena   *uuid.UUID
	JoinCode    string
	MemberCount int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Member is one player's place on a roster.
type Member struct {
	TeamID   uuid.UUID
	UserID   uuid.UUID
	Role     TeamRole
	JoinedAt time.Time

	// User is populated when a roster is read for display.
	User *UserSummary
}

func (t Team) IsCaptain(userID uuid.UUID) bool { return t.CaptainID == userID }

// MaxSquadSize caps a roster. Futsal plays 5 a side; the rest is substitutes
// and the people who say they will definitely make it next week.
const MaxSquadSize = 20

var teamTagPattern = regexp.MustCompile(`^[A-Z0-9]{2,5}$`)

func (t *Team) Validate() error {
	t.Name = strings.TrimSpace(t.Name)
	t.Tag = strings.ToUpper(strings.TrimSpace(t.Tag))

	v := &Validation{}
	nameLen := len([]rune(t.Name))
	v.Check(nameLen >= 2, "name", "Give the team a name.")
	v.Check(nameLen <= 40, "name", "That team name is too long.")
	v.Check(teamTagPattern.MatchString(t.Tag), "tag",
		"The tag should be 2 to 5 letters or numbers, like KTM.")
	return v.Err()
}

// CanAddMember states whether the captain may add another player, and why not.
func (t Team) CanAddMember(actorID uuid.UUID) error {
	if !t.IsCaptain(actorID) {
		return Forbidden("Only the captain can change the squad.")
	}
	if t.MemberCount >= MaxSquadSize {
		return Conflict("This squad is full at %d players.", MaxSquadSize)
	}
	return nil
}

// CanRemoveMember states whether actor may remove target from the roster.
// A captain can remove anyone but themselves; a player can only leave.
func (t Team) CanRemoveMember(actorID, targetID uuid.UUID) error {
	if targetID == t.CaptainID {
		return Conflict("The captain can't leave the team. Hand the armband to someone else first.")
	}
	if actorID == targetID || t.IsCaptain(actorID) {
		return nil
	}
	return Forbidden("Only the captain can remove other players.")
}

// CanTransferCaptaincy states whether actor may hand the armband to target.
func (t Team) CanTransferCaptaincy(actorID, targetID uuid.UUID) error {
	if !t.IsCaptain(actorID) {
		return Forbidden("Only the captain can hand over the armband.")
	}
	if targetID == t.CaptainID {
		return Invalid("user_id", "They're already the captain.")
	}
	return nil
}

// JoinCodeLength is the length of a team invite code.
const JoinCodeLength = 8

var joinCodePattern = regexp.MustCompile(`^[A-Z0-9]{8}$`)

// NormalizeJoinCode makes codes forgiving to type: case is ignored, and the
// spaces and hyphens people insert when reading a code aloud are dropped.
func NormalizeJoinCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.NewReplacer(" ", "", "-", "", "_", "").Replace(code)
	return code
}

func ValidateJoinCode(code string) error {
	if !joinCodePattern.MatchString(NormalizeJoinCode(code)) {
		return Invalid("join_code", "That doesn't look like a team code.")
	}
	return nil
}

// ---------------------------------------------------------------------------

// Match is a recorded result between two teams. Only verified matches, where
// both captains confirmed the score, count toward standings.
type Match struct {
	ID        uuid.UUID
	BookingID *uuid.UUID
	HomeTeam  uuid.UUID
	AwayTeam  uuid.UUID
	HomeScore int
	AwayScore int
	PlayedAt  time.Time
	Verified  bool
	// ReportedBy is the captain who filed the score. Confirmation has to come
	// from the other side, so this is what makes `Verified` mean "both agreed"
	// rather than "somebody said so twice".
	ReportedBy uuid.UUID
	CreatedAt  time.Time

	// Populated when a result is read for display.
	HomeName string
	HomeTag  string
	AwayName string
	AwayTag  string
}

// Involves reports whether a team played in this match.
func (m Match) Involves(teamID uuid.UUID) bool {
	return m.HomeTeam == teamID || m.AwayTeam == teamID
}

// CanBeConfirmedBy states whether the captain of teamID may confirm this
// result, and why not.
//
// The rule is the point of the whole flow: one captain reports, the other
// agrees. A captain confirming their own report would make `verified` a
// synonym for `reported`, and the standings are built on verified results
// alone.
func (m Match) CanBeConfirmedBy(actorID, teamID uuid.UUID) error {
	if m.Verified {
		return Conflict("That result is already agreed.")
	}
	if !m.Involves(teamID) {
		return Forbidden("Only the two captains can confirm a result.")
	}
	if m.ReportedBy == uuid.Nil {
		// A result with no reporter -- one filed before this was recorded --
		// cannot be confirmed by anybody. Safer than letting either side
		// wave it through.
		return Conflict("We can't tell who filed that result. Report it again.")
	}
	if m.ReportedBy == actorID {
		return Conflict("The other captain has to agree the score.")
	}
	return nil
}

// CanBeDisputedBy states whether the captain of teamID may counter this
// result with a different score.
//
// A dispute is a re-report, not a rejection: you say what you think the score
// was, and the ball goes back to the other captain to agree or counter again.
// That is why the rule is the same as confirming -- the other side, on an
// unagreed result -- rather than a separate permission. Two captains who keep
// disagreeing keep passing it back, which is the honest model of an argument
// about a score.
func (m Match) CanBeDisputedBy(actorID, teamID uuid.UUID) error {
	return m.CanBeConfirmedBy(actorID, teamID)
}

// CanBeWithdrawnBy states whether a captain may delete a result.
//
// Either side, and only while it is unagreed: once both captains have said the
// same thing it is a record of what happened, not a draft.
func (m Match) CanBeWithdrawnBy(teamID uuid.UUID) error {
	if !m.Involves(teamID) {
		return Forbidden("Only the two captains can withdraw a result.")
	}
	if m.Verified {
		return Conflict("Both captains agreed that result. It stands.")
	}
	return nil
}

func (m *Match) Validate() error {
	v := &Validation{}
	v.Check(m.HomeTeam != m.AwayTeam, "away_team", "A team can't play itself.")
	v.Check(m.HomeScore >= 0, "home_score", "Scores can't be negative.")
	v.Check(m.AwayScore >= 0, "away_score", "Scores can't be negative.")
	v.Check(m.HomeTeam != uuid.Nil, "home_team", "Which team was at home?")
	v.Check(m.AwayTeam != uuid.Nil, "away_team", "Who were the visitors?")
	return v.Err()
}

// Standing is one row of the leaderboard.
type Standing struct {
	TeamID       uuid.UUID
	Name         string
	Tag          string
	CrestURL     string
	Played       int
	Won          int
	Drawn        int
	Lost         int
	GoalsFor     int
	GoalsAgainst int
	GoalDiff     int
	Points       int
	Rank         int
}
