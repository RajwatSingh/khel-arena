package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// MatchStore is the results storage this service needs.
type MatchStore interface {
	Create(ctx context.Context, m domain.Match) (domain.Match, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Match, error)
	Confirm(ctx context.Context, id uuid.UUID) error
	Recount(ctx context.Context, id uuid.UUID, homeScore, awayScore int, byUserID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListForTeam(ctx context.Context, teamID uuid.UUID, limit int) ([]domain.Match, error)
	Standings(ctx context.Context, limit int) ([]domain.Standing, error)
}

// MatchService records results.
//
// The shape of it is one rule: a captain reports, the other captain agrees,
// and only then does the result count. Everything else here exists to make
// that rule enforceable -- which team the actor captains, whether they are the
// one who filed it, and whether it is still open to being changed.
type MatchService struct {
	matches MatchStore
	teams   TournamentTeams
	clock   Clock
}

func NewMatchService(matches MatchStore, teams TournamentTeams, clock Clock) *MatchService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &MatchService{matches: matches, teams: teams, clock: clock}
}

// Report files a result.
//
// The reporter must captain one of the two teams. Which one decides nothing
// about the score -- home and away are given, not inferred -- but it is what
// makes them a party to the match rather than a passer-by filing a scoreline
// between two squads they have nothing to do with.
func (s *MatchService) Report(ctx context.Context, actorID uuid.UUID, m domain.Match) (domain.Match, error) {
	if actorID == uuid.Nil {
		return domain.Match{}, domain.Unauthenticated("Sign in to report a result.")
	}
	if err := m.Validate(); err != nil {
		return domain.Match{}, err
	}

	if _, err := s.captainedTeam(ctx, actorID, m); err != nil {
		return domain.Match{}, err
	}

	m.ReportedBy = actorID
	if m.PlayedAt.IsZero() {
		m.PlayedAt = s.clock.Now()
	}
	// A result cannot be filed for a game that has not happened.
	if m.PlayedAt.After(s.clock.Now()) {
		return domain.Match{}, domain.Invalid("played_at", "That match hasn't been played yet.")
	}

	return s.matches.Create(ctx, m)
}

// Confirm is the other captain agreeing.
func (s *MatchService) Confirm(ctx context.Context, matchID, actorID uuid.UUID) (domain.Match, error) {
	if actorID == uuid.Nil {
		return domain.Match{}, domain.Unauthenticated("Sign in to confirm a result.")
	}

	match, err := s.matches.ByID(ctx, matchID)
	if err != nil {
		return domain.Match{}, err
	}

	teamID, err := s.captainedTeam(ctx, actorID, match)
	if err != nil {
		return domain.Match{}, err
	}
	if err := match.CanBeConfirmedBy(actorID, teamID); err != nil {
		return domain.Match{}, err
	}

	if err := s.matches.Confirm(ctx, matchID); err != nil {
		return domain.Match{}, err
	}
	return s.matches.ByID(ctx, matchID)
}

// Dispute counters a result with a different score.
//
// Not a rejection: the disputer says what they think happened, and the result
// goes back to the other captain to agree or counter again. `reported_by`
// moves with it, so the next confirmation has to come from the other side --
// the same rule as before, applied to the new score.
//
// This is what a captain does when the score is wrong. Withdrawing is for
// when the whole result is: a fixture that never happened, or the wrong
// opponent.
func (s *MatchService) Dispute(ctx context.Context, matchID, actorID uuid.UUID, homeScore, awayScore int) (domain.Match, error) {
	if actorID == uuid.Nil {
		return domain.Match{}, domain.Unauthenticated("Sign in to dispute a result.")
	}
	if homeScore < 0 || awayScore < 0 {
		return domain.Match{}, domain.Invalid("home_score", "Scores can't be negative.")
	}

	match, err := s.matches.ByID(ctx, matchID)
	if err != nil {
		return domain.Match{}, err
	}

	teamID, err := s.captainedTeam(ctx, actorID, match)
	if err != nil {
		return domain.Match{}, err
	}
	if err := match.CanBeDisputedBy(actorID, teamID); err != nil {
		return domain.Match{}, err
	}
	if match.HomeScore == homeScore && match.AwayScore == awayScore {
		// Agreeing with the score is confirming it, not disputing it.
		return domain.Match{}, domain.Invalid("home_score",
			"That's the score already filed. Confirm it instead.")
	}

	if err := s.matches.Recount(ctx, matchID, homeScore, awayScore, actorID); err != nil {
		return domain.Match{}, err
	}
	return s.matches.ByID(ctx, matchID)
}

// Withdraw removes a result nobody has agreed yet.
func (s *MatchService) Withdraw(ctx context.Context, matchID, actorID uuid.UUID) error {
	if actorID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage results.")
	}

	match, err := s.matches.ByID(ctx, matchID)
	if err != nil {
		return err
	}

	teamID, err := s.captainedTeam(ctx, actorID, match)
	if err != nil {
		return err
	}
	if err := match.CanBeWithdrawnBy(teamID); err != nil {
		return err
	}

	return s.matches.Delete(ctx, matchID)
}

func (s *MatchService) ListForTeam(ctx context.Context, teamID uuid.UUID, limit int) ([]domain.Match, error) {
	return s.matches.ListForTeam(ctx, teamID, limit)
}

func (s *MatchService) Standings(ctx context.Context, limit int) ([]domain.Standing, error) {
	return s.matches.Standings(ctx, limit)
}

// captainedTeam reports which of the match's two teams the actor captains.
//
// Not-found rather than forbidden when they captain neither: a stranger asking
// about a result between two squads they have nothing to do with should not
// learn from the answer that the result exists.
func (s *MatchService) captainedTeam(ctx context.Context, actorID uuid.UUID, m domain.Match) (uuid.UUID, error) {
	for _, teamID := range []uuid.UUID{m.HomeTeam, m.AwayTeam} {
		if teamID == uuid.Nil {
			continue
		}
		team, err := s.teams.ByID(ctx, teamID)
		if err != nil {
			return uuid.Nil, err
		}
		if team.IsCaptain(actorID) {
			return teamID, nil
		}
	}
	return uuid.Nil, domain.NotFound("No result of yours with that id.")
}
