package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// TournamentStore is the bracket storage this service needs.
type TournamentStore interface {
	Create(ctx context.Context, t domain.Tournament) (domain.Tournament, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Tournament, error)
	BySlug(ctx context.Context, slug string) (domain.Tournament, error)
	List(ctx context.Context, limit int) ([]domain.Tournament, error)
	SetStatus(ctx context.Context, id uuid.UUID, status domain.TournamentStatus) error
	Entries(ctx context.Context, tournamentID uuid.UUID) ([]domain.TournamentEntry, error)
	Register(ctx context.Context, tournamentID, teamID, byUserID uuid.UUID) error
	Withdraw(ctx context.Context, tournamentID, teamID uuid.UUID) error
	SetEntryPaid(ctx context.Context, tournamentID, teamID uuid.UUID, paid bool) error
}

// TournamentTeams is the slice of team storage this service reads: entering a
// bracket is something a captain does on behalf of a squad.
type TournamentTeams interface {
	ByID(ctx context.Context, id uuid.UUID) (domain.Team, error)
}

type TournamentService struct {
	tournaments TournamentStore
	teams       TournamentTeams
	clock       Clock
}

func NewTournamentService(tournaments TournamentStore, teams TournamentTeams, clock Clock) *TournamentService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &TournamentService{tournaments: tournaments, teams: teams, clock: clock}
}

// TournamentWithEntries is a bracket and the teams in it.
type TournamentWithEntries struct {
	domain.Tournament
	Entries []domain.TournamentEntry
}

func (s *TournamentService) List(ctx context.Context, limit int) ([]domain.Tournament, error) {
	return s.tournaments.List(ctx, limit)
}

func (s *TournamentService) Get(ctx context.Context, slug string) (TournamentWithEntries, error) {
	if slug == "" {
		return TournamentWithEntries{}, domain.Invalid("slug", "Which tournament?")
	}

	t, err := s.tournaments.BySlug(ctx, slug)
	if err != nil {
		return TournamentWithEntries{}, err
	}

	entries, err := s.tournaments.Entries(ctx, t.ID)
	if err != nil {
		return TournamentWithEntries{}, err
	}
	return TournamentWithEntries{Tournament: t, Entries: entries}, nil
}

func (s *TournamentService) Create(ctx context.Context, organizerID uuid.UUID, t domain.Tournament) (domain.Tournament, error) {
	if organizerID == uuid.Nil {
		return domain.Tournament{}, domain.Unauthenticated("Sign in to run a tournament.")
	}

	// The organiser is the caller, never a field in the request.
	t.OrganizerID = organizerID
	if err := t.Validate(); err != nil {
		return domain.Tournament{}, err
	}
	return s.tournaments.Create(ctx, t)
}

// Register enters a team.
//
// Two authorizations, and both are needed. The captain check is the squad's:
// nobody else may commit a team to a bracket, an entry fee and a Saturday.
// `CanAcceptRegistration` is the tournament's: it may be cancelled, started,
// full or past its deadline. Capacity is checked here for the message and
// enforced in the database for the truth -- two captains can claim the last
// slot at the same instant, and only the constraint settles that.
func (s *TournamentService) Register(ctx context.Context, tournamentID, teamID, actorID uuid.UUID) error {
	if actorID == uuid.Nil {
		return domain.Unauthenticated("Sign in to enter a tournament.")
	}

	team, err := s.teams.ByID(ctx, teamID)
	if err != nil {
		return err
	}
	if !team.IsCaptain(actorID) {
		return domain.Forbidden("Only the captain can enter the team in a tournament.")
	}

	tournament, err := s.tournaments.ByID(ctx, tournamentID)
	if err != nil {
		return err
	}
	if err := tournament.CanAcceptRegistration(s.clock.Now()); err != nil {
		return err
	}

	return s.tournaments.Register(ctx, tournamentID, teamID, actorID)
}

// Withdraw pulls a team out. The captain may do it, and so may the organiser
// -- who has to be able to remove a team that never paid or never showed.
func (s *TournamentService) Withdraw(ctx context.Context, tournamentID, teamID, actorID uuid.UUID) error {
	if actorID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage a tournament entry.")
	}

	tournament, err := s.tournaments.ByID(ctx, tournamentID)
	if err != nil {
		return err
	}

	if !tournament.OrganizedBy(actorID) {
		team, err := s.teams.ByID(ctx, teamID)
		if err != nil {
			return err
		}
		if !team.IsCaptain(actorID) {
			return domain.Forbidden("Only the captain or the organiser can withdraw a team.")
		}
	}

	return s.tournaments.Withdraw(ctx, tournamentID, teamID)
}

// SetEntryPaid is the organiser recording an entry fee. Same shape as an
// arena confirming cash: the person who took the money is the one who says so.
func (s *TournamentService) SetEntryPaid(ctx context.Context, tournamentID, teamID, actorID uuid.UUID, paid bool) error {
	tournament, err := s.load(ctx, tournamentID, actorID)
	if err != nil {
		return err
	}
	if !tournament.OrganizedBy(actorID) {
		return domain.Forbidden("Only the organiser can record entry fees.")
	}
	return s.tournaments.SetEntryPaid(ctx, tournamentID, teamID, paid)
}

// SetStatus moves a tournament through its life. Open and full are maintained
// by the database as teams come and go; this is for the transitions only a
// person can make -- starting it, finishing it, calling it off.
func (s *TournamentService) SetStatus(ctx context.Context, tournamentID, actorID uuid.UUID, status domain.TournamentStatus) error {
	tournament, err := s.load(ctx, tournamentID, actorID)
	if err != nil {
		return err
	}
	if !tournament.OrganizedBy(actorID) {
		return domain.Forbidden("Only the organiser can change a tournament.")
	}
	if err := status.Validate(); err != nil {
		return err
	}
	return s.tournaments.SetStatus(ctx, tournamentID, status)
}

func (s *TournamentService) load(ctx context.Context, tournamentID, actorID uuid.UUID) (domain.Tournament, error) {
	if actorID == uuid.Nil {
		return domain.Tournament{}, domain.Unauthenticated("Sign in to manage a tournament.")
	}
	return s.tournaments.ByID(ctx, tournamentID)
}
