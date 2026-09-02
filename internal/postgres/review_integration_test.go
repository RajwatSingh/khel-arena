package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// The trigger that keeps an arena's rating in step with its reviews is SQL,
// and so is the one-review-per-player rule. Neither can be checked without a
// database.

func TestReviewUpsertsAndSyncsTheRating(t *testing.T) {
	f := newFixture(t, 2)
	repo := NewReviewRepo(f.pool)
	ctx := context.Background()

	first, err := repo.Upsert(ctx, domain.Review{
		ArenaID: f.arenaID, UserID: f.players[0], Rating: 5, Comment: "Excellent turf.",
	})
	if err != nil {
		t.Fatalf("first review: %v", err)
	}

	if rating, count := arenaRating(t, f); rating != 5.0 || count != 1 {
		t.Errorf("after one 5: rating = %v, count = %d", rating, count)
	}

	// Posting again edits rather than stacks: one review per player per arena.
	second, err := repo.Upsert(ctx, domain.Review{
		ArenaID: f.arenaID, UserID: f.players[0], Rating: 3, Comment: "Changed my mind.",
	})
	if err != nil {
		t.Fatalf("second review: %v", err)
	}
	if second.ID != first.ID {
		t.Error("posting again created a second review instead of editing the first")
	}

	if rating, count := arenaRating(t, f); rating != 3.0 || count != 1 {
		t.Errorf("after editing to 3: rating = %v, count = %d", rating, count)
	}

	// A different player adds a second opinion, and the average moves.
	if _, err := repo.Upsert(ctx, domain.Review{
		ArenaID: f.arenaID, UserID: f.players[1], Rating: 5,
	}); err != nil {
		t.Fatalf("second player's review: %v", err)
	}
	if rating, count := arenaRating(t, f); rating != 4.0 || count != 2 {
		t.Errorf("after a 3 and a 5: rating = %v, count = %d, want 4.0 and 2", rating, count)
	}

	reviews, err := repo.ListReviews(ctx, f.arenaID, 10)
	if err != nil {
		t.Fatalf("listing reviews: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("reviews = %d, want 2", len(reviews))
	}
	// The author comes back as a public summary, for rendering next to the
	// comment.
	if reviews[0].Author == nil || reviews[0].Author.Username == "" {
		t.Error("the review's author did not come back")
	}

	// Deleting takes the rating back down with it.
	if err := repo.DeleteReview(ctx, f.arenaID, f.players[1]); err != nil {
		t.Fatalf("deleting review: %v", err)
	}
	if rating, count := arenaRating(t, f); rating != 3.0 || count != 1 {
		t.Errorf("after deleting the 5: rating = %v, count = %d", rating, count)
	}

	// And deleting one you never wrote is a not-found, not a silent success.
	if err := repo.DeleteReview(ctx, f.arenaID, f.players[1]); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("code = %q, want not_found", domain.CodeOf(err))
	}
}

func TestPhotosAreOwnerScoped(t *testing.T) {
	f := newFixture(t, 1)
	repo := NewReviewRepo(f.pool)
	ctx := context.Background()

	stranger := f.players[0]

	if _, err := repo.AddPhotoOwnerScoped(ctx, stranger, Photo{
		ArenaID: f.arenaID, URL: "https://example.test/pitch.jpg",
	}); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("stranger add: code = %q, want not_found", domain.CodeOf(err))
	}

	photo, err := repo.AddPhotoOwnerScoped(ctx, f.owner, Photo{
		ArenaID: f.arenaID, URL: "https://example.test/pitch.jpg",
		Caption: "The covered court", SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("owner add: %v", err)
	}
	if photo.Caption != "The covered court" {
		t.Errorf("caption = %q", photo.Caption)
	}

	photos, err := repo.ListPhotos(ctx, f.arenaID)
	if err != nil || len(photos) != 1 {
		t.Fatalf("photos = %v, err = %v", photos, err)
	}

	if err := repo.DeletePhotoOwnerScoped(ctx, photo.ID, stranger); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("stranger delete: code = %q, want not_found", domain.CodeOf(err))
	}
	if err := repo.DeletePhotoOwnerScoped(ctx, photo.ID, f.owner); err != nil {
		t.Errorf("owner delete: %v", err)
	}
}

func TestHighlightsAreOwnedByTheirPlayer(t *testing.T) {
	f := newFixture(t, 2)
	repo := NewReviewRepo(f.pool)
	ctx := context.Background()

	h, err := repo.AddHighlight(ctx, Highlight{
		UserID: f.players[0], Title: "Hat-trick v Dhuku", URL: "https://youtube.test/watch?v=abc",
	})
	if err != nil {
		t.Fatalf("adding highlight: %v", err)
	}

	// The schema insists on a real web address.
	if _, err := repo.AddHighlight(ctx, Highlight{
		UserID: f.players[0], Title: "Bad link", URL: "not-a-url",
	}); domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("bad url: code = %q, want invalid", domain.CodeOf(err))
	}

	// Somebody else cannot remove it.
	if err := repo.DeleteHighlight(ctx, h.ID, f.players[1]); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("stranger delete: code = %q, want not_found", domain.CodeOf(err))
	}
	if err := repo.DeleteHighlight(ctx, h.ID, f.players[0]); err != nil {
		t.Errorf("owner delete: %v", err)
	}
}

// arenaRating reads the denormalised columns the trigger maintains.
func arenaRating(t *testing.T, f *fixture) (float64, int) {
	t.Helper()

	var (
		rating *float64
		count  int
	)
	err := f.pool.QueryRow(context.Background(),
		`select rating, review_count from arenas where id = $1`, f.arenaID).Scan(&rating, &count)
	if err != nil {
		t.Fatalf("reading arena rating: %v", err)
	}
	if rating == nil {
		return 0, count
	}
	return *rating, count
}

// The rule that a review has to be earned. Only a real database can say
// whether the booking history actually supports it.
func TestReviewRequiresHavingPlayed(t *testing.T) {
	f := newFixture(t, 1)
	repo := NewReviewRepo(f.pool)
	ctx := context.Background()
	player := f.players[0]

	// Nobody has played anywhere yet.
	if played, err := repo.HasPlayedAt(ctx, f.arenaID, player); err != nil || played {
		t.Fatalf("played = %v, err = %v, want false", played, err)
	}

	// A hold that was never paid for does not count: it may have lapsed.
	_, err := f.pool.Exec(ctx, `
		insert into bookings (court_id, user_id, slot, price_npr, status, hold_expires_at)
		values ($1, $2, $3, 1200, 'pending', now() - interval '1 day')`,
		f.courtID, player, tstzrange(f.pastSlotAt(t, 7, 8)))
	if err != nil {
		t.Fatalf("seeding lapsed hold: %v", err)
	}
	if played, _ := repo.HasPlayedAt(ctx, f.arenaID, player); played {
		t.Error("an unpaid hold counted as having played")
	}

	// Nor does a confirmed booking that has not happened yet.
	future := f.slotAt(t, 20, time.Hour)
	_, err = f.pool.Exec(ctx, `
		insert into bookings (court_id, user_id, slot, price_npr, status)
		values ($1, $2, $3, 1200, 'confirmed')`,
		f.courtID, player, tstzrange(future))
	if err != nil {
		t.Fatalf("seeding future booking: %v", err)
	}
	if played, _ := repo.HasPlayedAt(ctx, f.arenaID, player); played {
		t.Error("a booking that has not happened counted as having played")
	}

	// A paid booking whose hour has passed does.
	_, err = f.pool.Exec(ctx, `
		insert into bookings (court_id, user_id, slot, price_npr, status)
		values ($1, $2, $3, 1200, 'confirmed')`,
		f.courtID, player, tstzrange(f.pastSlotAt(t, 7, 10)))
	if err != nil {
		t.Fatalf("seeding played booking: %v", err)
	}
	if played, err := repo.HasPlayedAt(ctx, f.arenaID, player); err != nil || !played {
		t.Errorf("played = %v, err = %v, want true", played, err)
	}

	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `delete from bookings where court_id = $1`, f.courtID)
	})
}

// `completed` was a status the enum described and nothing ever reached.
func TestMarkPlayedPromotesPastBookings(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()

	for _, seed := range []struct {
		slot   domain.Slot
		status string
	}{
		{f.pastSlotAt(t, 3, 9), "confirmed"},
		{f.slotAt(t, 21, time.Hour), "confirmed"},
	} {
		_, err := f.pool.Exec(ctx, `
			insert into bookings (court_id, user_id, slot, price_npr, status)
			values ($1, $2, $3, 1200, $4)`,
			f.courtID, f.players[0], tstzrange(seed.slot), seed.status)
		if err != nil {
			t.Fatalf("seeding booking: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `delete from bookings where court_id = $1`, f.courtID)
	})

	played, err := f.repo.MarkPlayed(ctx)
	if err != nil {
		t.Fatalf("marking played: %v", err)
	}
	if played != 1 {
		t.Errorf("promoted %d, want just the one whose hour has passed", played)
	}

	var completed, confirmed int
	err = f.pool.QueryRow(ctx, `
		select count(*) filter (where status = 'completed'),
		       count(*) filter (where status = 'confirmed')
		from bookings where court_id = $1`, f.courtID).Scan(&completed, &confirmed)
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if completed != 1 || confirmed != 1 {
		t.Errorf("completed = %d, confirmed = %d, want 1 and 1", completed, confirmed)
	}
}
