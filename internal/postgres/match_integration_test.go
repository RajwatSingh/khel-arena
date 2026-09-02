package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// The standings view is SQL — three points for a win, one for a draw, goal
// difference as the tiebreaker, verified results only — and none of that can
// be checked without a database.

// seedTeam creates a squad captained by one of the fixture's players.
func seedTeam(t *testing.T, f *fixture, captain uuid.UUID, name, tag string) uuid.UUID {
	t.Helper()

	repo := NewTeamRepo(f.pool)
	team, err := repo.Create(context.Background(), domain.Team{
		Name: name, Tag: tag, CaptainID: captain,
	})
	if err != nil {
		t.Fatalf("seeding team %q: %v", name, err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `delete from teams where id = $1`, team.ID)
	})
	return team.ID
}

func TestMatchLifecycleAndStandings(t *testing.T) {
	f := newFixture(t, 2)
	repo := NewMatchRepo(f.pool)
	ctx := context.Background()

	suffix := uuid.NewString()[:6]
	// Tags are uppercase in the schema, and this repository does not normalise
	// -- the service does that before it gets here.
	tag := strings.ToUpper(suffix[:3])
	home := seedTeam(t, f, f.players[0], "Home "+suffix, "H"+tag[:2])
	away := seedTeam(t, f, f.players[1], "Away "+suffix, "A"+tag[:2])

	created, err := repo.Create(ctx, domain.Match{
		HomeTeam: home, AwayTeam: away, HomeScore: 3, AwayScore: 1,
		PlayedAt: time.Now().Add(-time.Hour), ReportedBy: f.players[0],
	})
	if err != nil {
		t.Fatalf("creating match: %v", err)
	}
	if created.Verified {
		t.Error("a new result is already agreed")
	}
	if created.ReportedBy != f.players[0] {
		t.Errorf("reported_by = %v", created.ReportedBy)
	}
	// The join fills in both squads for display.
	if created.HomeName == "" || created.AwayTag == "" {
		t.Errorf("team context missing: %+v", created)
	}

	// An unagreed result must not reach the table.
	if inStandings(t, repo, home) {
		t.Fatal("an unverified result counted toward the standings")
	}

	if err := repo.Confirm(ctx, created.ID); err != nil {
		t.Fatalf("confirming: %v", err)
	}

	standings, err := repo.Standings(ctx, 100)
	if err != nil {
		t.Fatalf("loading standings: %v", err)
	}

	var homeRow, awayRow *domain.Standing
	for i := range standings {
		switch standings[i].TeamID {
		case home:
			homeRow = &standings[i]
		case away:
			awayRow = &standings[i]
		}
	}
	if homeRow == nil || awayRow == nil {
		t.Fatal("the agreed result did not reach the table")
	}

	// Three points for a win, goal difference from the score.
	if homeRow.Points != 3 || homeRow.Won != 1 || homeRow.GoalDiff != 2 {
		t.Errorf("winner = %+v, want 3 points and +2", homeRow)
	}
	if awayRow.Points != 0 || awayRow.Lost != 1 || awayRow.GoalDiff != -2 {
		t.Errorf("loser = %+v, want 0 points and -2", awayRow)
	}
	if homeRow.Rank >= awayRow.Rank {
		t.Errorf("ranks = %d vs %d; the winner should be above", homeRow.Rank, awayRow.Rank)
	}

	// Confirming again is a no-op rather than a second write.
	if err := repo.Confirm(ctx, created.ID); err != nil {
		t.Errorf("second confirm: %v", err)
	}
}

func TestDrawIsOnePointEach(t *testing.T) {
	f := newFixture(t, 2)
	repo := NewMatchRepo(f.pool)
	ctx := context.Background()

	suffix := uuid.NewString()[:6]
	tag := strings.ToUpper(suffix[:3])
	home := seedTeam(t, f, f.players[0], "Draw H "+suffix, "DH"+tag[:2])
	away := seedTeam(t, f, f.players[1], "Draw A "+suffix, "DA"+tag[:2])

	m, err := repo.Create(ctx, domain.Match{
		HomeTeam: home, AwayTeam: away, HomeScore: 2, AwayScore: 2,
		PlayedAt: time.Now().Add(-time.Hour), ReportedBy: f.players[0],
	})
	if err != nil {
		t.Fatalf("creating match: %v", err)
	}
	if err := repo.Confirm(ctx, m.ID); err != nil {
		t.Fatalf("confirming: %v", err)
	}

	standings, _ := repo.Standings(ctx, 200)
	for _, s := range standings {
		if s.TeamID != home && s.TeamID != away {
			continue
		}
		if s.Points != 1 || s.Drawn != 1 || s.GoalDiff != 0 {
			t.Errorf("%s = %+v, want 1 point from a draw", s.Name, s)
		}
	}
}

// A team cannot play itself: the schema says so, and the repository turns the
// violation into something a person can read.
func TestMatchAgainstItself(t *testing.T) {
	f := newFixture(t, 1)
	repo := NewMatchRepo(f.pool)

	team := seedTeam(t, f, f.players[0], "Solo "+uuid.NewString()[:6], "SL")

	_, err := repo.Create(context.Background(), domain.Match{
		HomeTeam: team, AwayTeam: team, PlayedAt: time.Now(), ReportedBy: f.players[0],
	})
	if domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid", domain.CodeOf(err))
	}
}

func inStandings(t *testing.T, repo *MatchRepo, teamID uuid.UUID) bool {
	t.Helper()

	standings, err := repo.Standings(context.Background(), 200)
	if err != nil {
		t.Fatalf("loading standings: %v", err)
	}
	for _, s := range standings {
		if s.TeamID == teamID && s.Played > 0 {
			return true
		}
	}
	return false
}
