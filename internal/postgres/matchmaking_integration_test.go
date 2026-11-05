package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// The filled count is kept by a trigger on matchmaking_responses. These pin
// that each accept and withdraw moves it by exactly one, which is what broke
// when the repository also moved it by hand.

func TestAcceptAndWithdrawCountOncePerPlayer(t *testing.T) {
	f := newFixture(t, 3)
	repo := NewMatchmakingRepo(f.pool)
	ctx := context.Background()

	author, a, b := f.players[0], f.players[1], f.players[2]
	call, err := repo.Create(ctx, domain.Call{
		AuthorID: author, ArenaID: &f.arenaID, Title: "Need two",
		NeededPlayers: 2, Skill: domain.SkillCasual,
		StartsAt: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("creating call: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `delete from matchmaking_posts where id = $1`, call.ID)
	})

	state := func(wantFilled int, wantStatus domain.MatchmakingStatus) {
		t.Helper()
		got, err := repo.ByID(ctx, call.ID)
		if err != nil {
			t.Fatalf("reading call: %v", err)
		}
		if got.FilledPlayers != wantFilled || got.Status != wantStatus {
			t.Fatalf("filled=%d status=%s, want filled=%d status=%s",
				got.FilledPlayers, got.Status, wantFilled, wantStatus)
		}
	}

	for _, p := range []uuid.UUID{a, b} {
		if err := repo.Respond(ctx, call.ID, p, ""); err != nil {
			t.Fatalf("responding: %v", err)
		}
	}

	if err := repo.Accept(ctx, call.ID, a); err != nil {
		t.Fatalf("accepting first player: %v", err)
	}
	state(1, domain.MatchmakingOpen)

	// A double click is a no-op, not a second place.
	if err := repo.Accept(ctx, call.ID, a); err != nil {
		t.Fatalf("accepting again: %v", err)
	}
	state(1, domain.MatchmakingOpen)

	// The last place: this is the one that used to trip not_overfilled.
	if err := repo.Accept(ctx, call.ID, b); err != nil {
		t.Fatalf("accepting last player: %v", err)
	}
	state(2, domain.MatchmakingFilled)

	// One player leaving frees exactly one place and reopens the game.
	if err := repo.Withdraw(ctx, call.ID, a); err != nil {
		t.Fatalf("withdrawing: %v", err)
	}
	state(1, domain.MatchmakingOpen)
}

func TestPendingRequestsAreTheAuthorsUnansweredOnes(t *testing.T) {
	f := newFixture(t, 3)
	repo := NewMatchmakingRepo(f.pool)
	ctx := context.Background()
	author, a, b := f.players[0], f.players[1], f.players[2]

	newCall := func(title string, by uuid.UUID, starts time.Time) uuid.UUID {
		t.Helper()
		c, err := repo.Create(ctx, domain.Call{
			AuthorID: by, ArenaID: &f.arenaID, Title: title,
			NeededPlayers: 3, Skill: domain.SkillCasual, StartsAt: starts,
		})
		if err != nil {
			t.Fatalf("creating %q: %v", title, err)
		}
		t.Cleanup(func() {
			_, _ = f.pool.Exec(context.Background(), `delete from matchmaking_posts where id = $1`, c.ID)
		})
		return c.ID
	}
	respond := func(call, who uuid.UUID) {
		t.Helper()
		if err := repo.Respond(ctx, call, who, ""); err != nil {
			t.Fatalf("responding: %v", err)
		}
	}

	mine := newCall("Mine", author, time.Now().Add(24*time.Hour))
	theirs := newCall("Theirs", a, time.Now().Add(24*time.Hour))
	respond(mine, a)
	respond(mine, b)
	respond(theirs, b)
	if err := repo.Accept(ctx, mine, a); err != nil {
		t.Fatalf("accepting: %v", err)
	}

	got, err := repo.PendingRequests(ctx, author, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].CallID != mine || got[0].UserID != b {
		t.Fatalf("got %+v, want only b's request on the author's game", got)
	}
	if got[0].Responder == nil || got[0].Responder.ID != b {
		t.Fatal("responder card missing")
	}

	// A game that has already kicked off is no longer asking for anyone.
	if got, _ := repo.PendingRequests(ctx, author, time.Now().Add(48*time.Hour)); len(got) != 0 {
		t.Fatalf("past game still shows %d requests", len(got))
	}
}
