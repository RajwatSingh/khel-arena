package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

type stubTournaments struct {
	tournament domain.Tournament
	entries    []domain.TournamentEntry
	writes     int
	registered uuid.UUID
}

func (s *stubTournaments) Create(_ context.Context, t domain.Tournament) (domain.Tournament, error) {
	s.writes++
	t.ID = uuid.New()
	s.tournament = t
	return t, nil
}
func (s *stubTournaments) ByID(context.Context, uuid.UUID) (domain.Tournament, error) {
	return s.tournament, nil
}
func (s *stubTournaments) BySlug(context.Context, string) (domain.Tournament, error) {
	return s.tournament, nil
}
func (s *stubTournaments) List(context.Context, int) ([]domain.Tournament, error) {
	return []domain.Tournament{s.tournament}, nil
}
func (s *stubTournaments) SetStatus(context.Context, uuid.UUID, domain.TournamentStatus) error {
	s.writes++
	return nil
}
func (s *stubTournaments) Entries(context.Context, uuid.UUID) ([]domain.TournamentEntry, error) {
	return s.entries, nil
}
func (s *stubTournaments) Register(_ context.Context, _, teamID, _ uuid.UUID) error {
	s.writes++
	s.registered = teamID
	return nil
}
func (s *stubTournaments) Withdraw(context.Context, uuid.UUID, uuid.UUID) error {
	s.writes++
	return nil
}
func (s *stubTournaments) SetEntryPaid(context.Context, uuid.UUID, uuid.UUID, bool) error {
	s.writes++
	return nil
}

type stubTournamentTeams struct{ team domain.Team }

func (s stubTournamentTeams) ByID(context.Context, uuid.UUID) (domain.Team, error) {
	return s.team, nil
}

func openTournament() domain.Tournament {
	return domain.Tournament{
		ID:          uuid.New(),
		OrganizerID: organiser,
		Name:        "Jhamsikhel Winter Cup",
		Slug:        "jhamsikhel-winter-cup",
		MaxTeams:    8,
		TeamCount:   3,
		Status:      domain.TournamentOpen,
		StartsOn:    now.AddDate(0, 0, 14),
		RegisterBy:  now.AddDate(0, 0, 7),
	}
}

var organiser = uuid.MustParse("bbbbbbbb-0000-4000-8000-000000000001")

func newTournamentService(ts *stubTournaments, captain uuid.UUID) *TournamentService {
	return NewTournamentService(ts,
		stubTournamentTeams{team: domain.Team{ID: uuid.New(), CaptainID: captain}},
		fixedClock{now})
}

// Entering a team commits it to a bracket, an entry fee and a Saturday.
// Nobody but the captain gets to do that.
func TestOnlyTheCaptainEntersATeam(t *testing.T) {
	ts := &stubTournaments{tournament: openTournament()}
	svc := newTournamentService(ts, captainID)

	err := svc.Register(context.Background(), ts.tournament.ID, uuid.New(), playerID)

	if domain.CodeOf(err) != domain.CodeForbidden {
		t.Errorf("code = %q, want forbidden", domain.CodeOf(err))
	}
	if ts.writes != 0 {
		t.Error("a non-captain entered a team")
	}
}

func TestCaptainEntersATeam(t *testing.T) {
	ts := &stubTournaments{tournament: openTournament()}
	svc := newTournamentService(ts, captainID)
	teamID := uuid.New()

	if err := svc.Register(context.Background(), ts.tournament.ID, teamID, captainID); err != nil {
		t.Fatalf("registering: %v", err)
	}
	if ts.registered != teamID {
		t.Errorf("registered = %v, want %v", ts.registered, teamID)
	}
}

func TestRegistrationRefusals(t *testing.T) {
	cases := map[string]func(domain.Tournament) domain.Tournament{
		"cancelled": func(t domain.Tournament) domain.Tournament {
			t.Status = domain.TournamentCancelled
			return t
		},
		"already started": func(t domain.Tournament) domain.Tournament {
			t.Status = domain.TournamentOngoing
			return t
		},
		"full by status": func(t domain.Tournament) domain.Tournament {
			t.Status = domain.TournamentFull
			return t
		},
		"full by count": func(t domain.Tournament) domain.Tournament {
			t.TeamCount = t.MaxTeams
			return t
		},
		"deadline passed": func(t domain.Tournament) domain.Tournament {
			t.RegisterBy = now.AddDate(0, 0, -2)
			return t
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t2 *testing.T) {
			ts := &stubTournaments{tournament: mutate(openTournament())}
			svc := newTournamentService(ts, captainID)

			err := svc.Register(context.Background(), ts.tournament.ID, uuid.New(), captainID)

			if domain.CodeOf(err) != domain.CodeConflict {
				t2.Errorf("code = %q, want conflict", domain.CodeOf(err))
			}
			if ts.writes != 0 {
				t2.Error("a team was entered anyway")
			}
		})
	}
}

// Registration is open through the end of the deadline day, not up to its
// midnight -- somebody registering on the closing date has not missed it.
func TestRegistrationIsOpenOnTheDeadlineDay(t *testing.T) {
	tournament := openTournament()
	tournament.RegisterBy = now.Truncate(24 * time.Hour)

	ts := &stubTournaments{tournament: tournament}
	svc := newTournamentService(ts, captainID)

	if err := svc.Register(context.Background(), tournament.ID, uuid.New(), captainID); err != nil {
		t.Errorf("registering on the closing date: %v", err)
	}
}

func TestWithdraw(t *testing.T) {
	t.Run("the captain may withdraw their team", func(t *testing.T) {
		ts := &stubTournaments{tournament: openTournament()}
		svc := newTournamentService(ts, captainID)

		if err := svc.Withdraw(context.Background(), ts.tournament.ID, uuid.New(), captainID); err != nil {
			t.Errorf("withdrawing: %v", err)
		}
	})

	// An organiser has to be able to remove a team that never paid or never
	// showed.
	t.Run("the organiser may too", func(t *testing.T) {
		ts := &stubTournaments{tournament: openTournament()}
		svc := newTournamentService(ts, captainID)

		if err := svc.Withdraw(context.Background(), ts.tournament.ID, uuid.New(), organiser); err != nil {
			t.Errorf("organiser withdrawing a team: %v", err)
		}
	})

	t.Run("nobody else may", func(t *testing.T) {
		ts := &stubTournaments{tournament: openTournament()}
		svc := newTournamentService(ts, captainID)

		err := svc.Withdraw(context.Background(), ts.tournament.ID, uuid.New(), outsider)

		if domain.CodeOf(err) != domain.CodeForbidden {
			t.Errorf("code = %q, want forbidden", domain.CodeOf(err))
		}
	})
}

func TestOnlyTheOrganiserRunsTheTournament(t *testing.T) {
	ts := &stubTournaments{tournament: openTournament()}
	svc := newTournamentService(ts, captainID)
	ctx := context.Background()

	for name, err := range map[string]error{
		"records an entry fee": svc.SetEntryPaid(ctx, ts.tournament.ID, uuid.New(), captainID, true),
		"changes the status":   svc.SetStatus(ctx, ts.tournament.ID, captainID, domain.TournamentOngoing),
	} {
		t.Run(name, func(t2 *testing.T) {
			if domain.CodeOf(err) != domain.CodeForbidden {
				t2.Errorf("code = %q, want forbidden", domain.CodeOf(err))
			}
		})
	}
	if ts.writes != 0 {
		t.Error("a non-organiser changed the tournament")
	}
}

func TestCreateTournamentTakesTheOrganiserFromTheCaller(t *testing.T) {
	ts := &stubTournaments{}
	svc := newTournamentService(ts, captainID)

	// A complete bracket, since Create validates: the point of this test is
	// who ends up owning it, not what Validate rejects.
	in := openTournament()
	in.OrganizerID = outsider
	in.SideCount = 5
	in.SquadCap = 10
	in.PrizeSplit = []int{60, 30, 10}

	if _, err := svc.Create(context.Background(), organiser, in); err != nil {
		t.Fatalf("creating: %v", err)
	}
	if ts.tournament.OrganizerID != organiser {
		t.Errorf("organiser = %v, want the caller %v", ts.tournament.OrganizerID, organiser)
	}
}
