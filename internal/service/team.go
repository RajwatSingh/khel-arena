package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// TeamStore is the squad storage this service needs.
type TeamStore interface {
	Create(ctx context.Context, t domain.Team) (domain.Team, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Team, error)
	ByJoinCode(ctx context.Context, code string) (domain.Team, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Team, error)
	Roster(ctx context.Context, teamID uuid.UUID) ([]domain.Member, error)
	Update(ctx context.Context, teamID uuid.UUID, t domain.Team) (domain.Team, error)
	AddMember(ctx context.Context, teamID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
	IsMember(ctx context.Context, teamID, userID uuid.UUID) (bool, error)
	TransferCaptaincy(ctx context.Context, teamID, fromID, toID uuid.UUID) error
	RotateJoinCode(ctx context.Context, teamID uuid.UUID) (string, error)
	Disband(ctx context.Context, teamID uuid.UUID) error
}

// TeamService is squad management.
//
// Every authorization decision here is a call into the domain --
// `CanAddMember`, `CanRemoveMember`, `CanTransferCaptaincy` -- rather than an
// `if` written at this layer. Those predicates already exist and already
// return the right refusal; restating them here would be a second opinion
// that can drift from the first.
type TeamService struct {
	teams TeamStore
}

func NewTeamService(teams TeamStore) *TeamService { return &TeamService{teams: teams} }

// TeamWithRoster is a squad and its players.
type TeamWithRoster struct {
	domain.Team
	Members []domain.Member
	// JoinCode is blank for anyone but a member: it is an invite credential,
	// and a team page readable by strangers must not hand it out.
	JoinCode string
}

func (s *TeamService) Create(ctx context.Context, captainID uuid.UUID, t domain.Team) (domain.Team, error) {
	if captainID == uuid.Nil {
		return domain.Team{}, domain.Unauthenticated("Sign in to start a team.")
	}

	// The captain is the caller, never a field in the request.
	t.CaptainID = captainID
	if err := t.Validate(); err != nil {
		return domain.Team{}, err
	}
	return s.teams.Create(ctx, t)
}

func (s *TeamService) MyTeams(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	if userID == uuid.Nil {
		return nil, domain.Unauthenticated("Sign in to see your teams.")
	}
	return s.teams.ListForUser(ctx, userID)
}

// Get returns a team with its roster.
//
// Readable by anyone signed in -- a squad page is something you share -- but
// the join code is stripped for non-members. That code is how somebody gets
// onto the roster, so publishing it on a page strangers can read would make
// every team open to anyone who looked.
func (s *TeamService) Get(ctx context.Context, teamID, viewerID uuid.UUID) (TeamWithRoster, error) {
	team, err := s.teams.ByID(ctx, teamID)
	if err != nil {
		return TeamWithRoster{}, err
	}

	members, err := s.teams.Roster(ctx, teamID)
	if err != nil {
		return TeamWithRoster{}, err
	}

	out := TeamWithRoster{Team: team, Members: members}
	// Blank it on the copy that leaves, so a caller cannot forget to.
	out.Team.JoinCode = ""

	for _, m := range members {
		if m.UserID == viewerID {
			out.JoinCode = team.JoinCode
			break
		}
	}
	return out, nil
}

func (s *TeamService) Update(ctx context.Context, teamID, actorID uuid.UUID, in domain.Team) (domain.Team, error) {
	team, err := s.load(ctx, teamID, actorID)
	if err != nil {
		return domain.Team{}, err
	}
	if !team.IsCaptain(actorID) {
		return domain.Team{}, domain.Forbidden("Only the captain can change the team.")
	}

	in.CaptainID = team.CaptainID
	if err := in.Validate(); err != nil {
		return domain.Team{}, err
	}
	return s.teams.Update(ctx, teamID, in)
}

// Join adds the caller to a team using an invite code.
//
// The code identifies the team; there is no separate "which team" parameter,
// so a code cannot be redeemed against a different squad than the one it
// belongs to.
func (s *TeamService) Join(ctx context.Context, userID uuid.UUID, code string) (domain.Team, error) {
	if userID == uuid.Nil {
		return domain.Team{}, domain.Unauthenticated("Sign in to join a team.")
	}

	code = domain.NormalizeJoinCode(code)
	if err := domain.ValidateJoinCode(code); err != nil {
		return domain.Team{}, err
	}

	team, err := s.teams.ByJoinCode(ctx, code)
	if err != nil {
		return domain.Team{}, err
	}

	// The squad-size rule is the captain's, and a code holder is exercising it
	// on the captain's behalf -- so the check is made as the captain.
	if err := team.CanAddMember(team.CaptainID); err != nil {
		return domain.Team{}, err
	}
	if err := s.teams.AddMember(ctx, team.ID, userID); err != nil {
		return domain.Team{}, err
	}

	team.JoinCode = ""
	return team, nil
}

// AddMember seats a player the captain names.
func (s *TeamService) AddMember(ctx context.Context, teamID, actorID, userID uuid.UUID) error {
	team, err := s.load(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if err := team.CanAddMember(actorID); err != nil {
		return err
	}
	return s.teams.AddMember(ctx, teamID, userID)
}

// RemoveMember drops a player, or lets one leave.
func (s *TeamService) RemoveMember(ctx context.Context, teamID, actorID, targetID uuid.UUID) error {
	team, err := s.load(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if err := team.CanRemoveMember(actorID, targetID); err != nil {
		return err
	}
	return s.teams.RemoveMember(ctx, teamID, targetID)
}

// TransferCaptaincy hands the armband to another member.
func (s *TeamService) TransferCaptaincy(ctx context.Context, teamID, actorID, targetID uuid.UUID) error {
	team, err := s.load(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if err := team.CanTransferCaptaincy(actorID, targetID); err != nil {
		return err
	}

	// The armband can only go to somebody on the roster: a captain who is not
	// a member is a team nobody can manage.
	member, err := s.teams.IsMember(ctx, teamID, targetID)
	if err != nil {
		return err
	}
	if !member {
		return domain.Invalid("user_id", "They aren't on this team.")
	}

	return s.teams.TransferCaptaincy(ctx, teamID, actorID, targetID)
}

// RotateJoinCode retires a leaked invite without disbanding the squad.
func (s *TeamService) RotateJoinCode(ctx context.Context, teamID, actorID uuid.UUID) (string, error) {
	team, err := s.load(ctx, teamID, actorID)
	if err != nil {
		return "", err
	}
	if !team.IsCaptain(actorID) {
		return "", domain.Forbidden("Only the captain can change the invite code.")
	}
	return s.teams.RotateJoinCode(ctx, teamID)
}

func (s *TeamService) Disband(ctx context.Context, teamID, actorID uuid.UUID) error {
	team, err := s.load(ctx, teamID, actorID)
	if err != nil {
		return err
	}
	if !team.IsCaptain(actorID) {
		return domain.Forbidden("Only the captain can disband the team.")
	}
	return s.teams.Disband(ctx, teamID)
}

// load fetches a team for an action, refusing an unauthenticated caller first.
func (s *TeamService) load(ctx context.Context, teamID, actorID uuid.UUID) (domain.Team, error) {
	if actorID == uuid.Nil {
		return domain.Team{}, domain.Unauthenticated("Sign in to manage a team.")
	}
	return s.teams.ByID(ctx, teamID)
}
