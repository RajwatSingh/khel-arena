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
	CreatedAt time.Time
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
