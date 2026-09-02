package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// The one rule worth testing here: a captain reports, the *other* captain
// agrees. A captain confirming their own report would make `verified` a
// synonym for `reported`, and the standings are built on verified results.

type stubMatches struct {
	match    domain.Match
	confirms int
	deletes  int
	creates  int
}

func (s *stubMatches) Create(_ context.Context, m domain.Match) (domain.Match, error) {
	s.creates++
	m.ID = uuid.New()
	s.match = m
	return m, nil
}
func (s *stubMatches) ByID(context.Context, uuid.UUID) (domain.Match, error) { return s.match, nil }
func (s *stubMatches) Confirm(context.Context, uuid.UUID) error {
	s.confirms++
	s.match.Verified = true
	return nil
}
func (s *stubMatches) Delete(context.Context, uuid.UUID) error { s.deletes++; return nil }
func (s *stubMatches) ListForTeam(context.Context, uuid.UUID, int) ([]domain.Match, error) {
	return []domain.Match{s.match}, nil
}
func (s *stubMatches) Standings(context.Context, int) ([]domain.Standing, error) {
	return []domain.Standing{{Rank: 1, Points: 9}}, nil
}

// twoTeams answers with whichever team was asked for, each with its own
// captain — which is what lets a test say "the other captain".
type twoTeams struct {
	home, away domain.Team
}

func (t twoTeams) ByID(_ context.Context, id uuid.UUID) (domain.Team, error) {
	switch id {
	case t.home.ID:
		return t.home, nil
	case t.away.ID:
		return t.away, nil
	}
	return domain.Team{}, domain.NotFound("No team with that id.")
}

var (
	homeCaptain = uuid.MustParse("cccccccc-0000-4000-8000-000000000001")
	awayCaptain = uuid.MustParse("cccccccc-0000-4000-8000-000000000002")
)

func matchFixture() (*stubMatches, twoTeams, domain.Match) {
	home := domain.Team{ID: uuid.New(), Name: "Yeti FC", Tag: "YETI", CaptainID: homeCaptain}
	away := domain.Team{ID: uuid.New(), Name: "Dhuku XI", Tag: "DHK", CaptainID: awayCaptain}

	return &stubMatches{}, twoTeams{home: home, away: away}, domain.Match{
		HomeTeam:  home.ID,
		AwayTeam:  away.ID,
		HomeScore: 3,
		AwayScore: 2,
		PlayedAt:  now.Add(-2 * time.Hour),
	}
}

func TestReportRecordsWhoFiledIt(t *testing.T) {
	matches, teams, m := matchFixture()
	svc := NewMatchService(matches, teams, fixedClock{now})

	filed, err := svc.Report(context.Background(), homeCaptain, m)
	if err != nil {
		t.Fatalf("reporting: %v", err)
	}
	if filed.ReportedBy != homeCaptain {
		t.Errorf("reported_by = %v, want the caller", filed.ReportedBy)
	}
	if filed.Verified {
		t.Error("a freshly filed result is already agreed")
	}
}

// Somebody who captains neither team has no business filing a scoreline
// between two squads they have nothing to do with.
func TestOnlyACaptainReports(t *testing.T) {
	matches, teams, m := matchFixture()
	svc := NewMatchService(matches, teams, fixedClock{now})

	_, err := svc.Report(context.Background(), outsider, m)

	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("code = %q, want not_found", domain.CodeOf(err))
	}
	if matches.creates != 0 {
		t.Error("a stranger filed a result")
	}
}

func TestConfirm(t *testing.T) {
	t.Run("the other captain agrees", func(t *testing.T) {
		matches, teams, m := matchFixture()
		svc := NewMatchService(matches, teams, fixedClock{now})

		if _, err := svc.Report(context.Background(), homeCaptain, m); err != nil {
			t.Fatalf("reporting: %v", err)
		}

		confirmed, err := svc.Confirm(context.Background(), matches.match.ID, awayCaptain)
		if err != nil {
			t.Fatalf("confirming: %v", err)
		}
		if !confirmed.Verified {
			t.Error("the result is not agreed")
		}
	})

	// The whole point. Without this, `verified` means nothing.
	t.Run("the reporter cannot confirm their own", func(t *testing.T) {
		matches, teams, m := matchFixture()
		svc := NewMatchService(matches, teams, fixedClock{now})

		if _, err := svc.Report(context.Background(), homeCaptain, m); err != nil {
			t.Fatalf("reporting: %v", err)
		}

		_, err := svc.Confirm(context.Background(), matches.match.ID, homeCaptain)

		if domain.CodeOf(err) != domain.CodeConflict {
			t.Errorf("code = %q, want conflict", domain.CodeOf(err))
		}
		if matches.confirms != 0 {
			t.Error("a captain agreed their own scoreline")
		}
	})

	t.Run("a stranger cannot confirm", func(t *testing.T) {
		matches, teams, m := matchFixture()
		svc := NewMatchService(matches, teams, fixedClock{now})
		_, _ = svc.Report(context.Background(), homeCaptain, m)

		_, err := svc.Confirm(context.Background(), matches.match.ID, outsider)

		if domain.CodeOf(err) != domain.CodeNotFound {
			t.Errorf("code = %q, want not_found", domain.CodeOf(err))
		}
	})

	t.Run("confirming twice is a conflict, not a second write", func(t *testing.T) {
		matches, teams, m := matchFixture()
		svc := NewMatchService(matches, teams, fixedClock{now})
		_, _ = svc.Report(context.Background(), homeCaptain, m)
		_, _ = svc.Confirm(context.Background(), matches.match.ID, awayCaptain)

		_, err := svc.Confirm(context.Background(), matches.match.ID, awayCaptain)

		if domain.CodeOf(err) != domain.CodeConflict {
			t.Errorf("code = %q, want conflict", domain.CodeOf(err))
		}
		if matches.confirms != 1 {
			t.Errorf("confirms = %d, want 1", matches.confirms)
		}
	})

	// A row filed before the reporter was recorded cannot be agreed by
	// anybody: safer than letting either side wave it through.
	t.Run("a result with no reporter cannot be confirmed", func(t *testing.T) {
		matches, teams, m := matchFixture()
		matches.match = m
		matches.match.ID = uuid.New()
		matches.match.ReportedBy = uuid.Nil
		svc := NewMatchService(matches, teams, fixedClock{now})

		_, err := svc.Confirm(context.Background(), matches.match.ID, awayCaptain)

		if domain.CodeOf(err) != domain.CodeConflict {
			t.Errorf("code = %q, want conflict", domain.CodeOf(err))
		}
		if matches.confirms != 0 {
			t.Error("an unattributable result was agreed")
		}
	})
}

func TestWithdrawMatch(t *testing.T) {
	t.Run("either captain, while it is unagreed", func(t *testing.T) {
		for name, actor := range map[string]uuid.UUID{"reporter": homeCaptain, "the other": awayCaptain} {
			t.Run(name, func(t *testing.T) {
				matches, teams, m := matchFixture()
				svc := NewMatchService(matches, teams, fixedClock{now})
				_, _ = svc.Report(context.Background(), homeCaptain, m)

				if err := svc.Withdraw(context.Background(), matches.match.ID, actor); err != nil {
					t.Errorf("withdrawing: %v", err)
				}
			})
		}
	})

	// Once both captains have said the same thing it is a record of what
	// happened, not a draft.
	t.Run("not once it is agreed", func(t *testing.T) {
		matches, teams, m := matchFixture()
		svc := NewMatchService(matches, teams, fixedClock{now})
		_, _ = svc.Report(context.Background(), homeCaptain, m)
		_, _ = svc.Confirm(context.Background(), matches.match.ID, awayCaptain)

		err := svc.Withdraw(context.Background(), matches.match.ID, homeCaptain)

		if domain.CodeOf(err) != domain.CodeConflict {
			t.Errorf("code = %q, want conflict", domain.CodeOf(err))
		}
		if matches.deletes != 0 {
			t.Error("an agreed result was withdrawn")
		}
	})
}

func TestCannotReportAMatchThatHasNotHappened(t *testing.T) {
	matches, teams, m := matchFixture()
	m.PlayedAt = now.Add(2 * time.Hour)
	svc := NewMatchService(matches, teams, fixedClock{now})

	_, err := svc.Report(context.Background(), homeCaptain, m)

	if domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid", domain.CodeOf(err))
	}
	if matches.creates != 0 {
		t.Error("a result was filed for a game in the future")
	}
}

func TestATeamCannotPlayItself(t *testing.T) {
	matches, teams, m := matchFixture()
	m.AwayTeam = m.HomeTeam
	svc := NewMatchService(matches, teams, fixedClock{now})

	_, err := svc.Report(context.Background(), homeCaptain, m)

	if domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid", domain.CodeOf(err))
	}
}
