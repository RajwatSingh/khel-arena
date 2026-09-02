package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
)

// ReviewStore is the storage for reviews, photos and highlights.
type ReviewStore interface {
	HasPlayedAt(ctx context.Context, arenaID, userID uuid.UUID) (bool, error)
	ReviewByUser(ctx context.Context, arenaID, userID uuid.UUID) (domain.Review, error)
	Upsert(ctx context.Context, review domain.Review) (domain.Review, error)
	DeleteReview(ctx context.Context, arenaID, userID uuid.UUID) error
	ListReviews(ctx context.Context, arenaID uuid.UUID, limit int) ([]domain.Review, error)

	AddPhotoOwnerScoped(ctx context.Context, ownerID uuid.UUID, p postgres.Photo) (postgres.Photo, error)
	DeletePhotoOwnerScoped(ctx context.Context, photoID, ownerID uuid.UUID) error
	ListPhotos(ctx context.Context, arenaID uuid.UUID) ([]postgres.Photo, error)

	AddHighlight(ctx context.Context, h postgres.Highlight) (postgres.Highlight, error)
	DeleteHighlight(ctx context.Context, highlightID, userID uuid.UUID) error
	ListHighlights(ctx context.Context, userID uuid.UUID) ([]postgres.Highlight, error)
}

// ReviewService covers what people attach to a venue and to themselves.
type ReviewService struct {
	reviews ReviewStore
}

func NewReviewService(reviews ReviewStore) *ReviewService {
	return &ReviewService{reviews: reviews}
}

// Review records a player's opinion of an arena.
//
// **You have to have played there.** A rating from somebody who has never set
// foot on the pitch is worth nothing to the next person reading it, and an
// arena's rating is a number the listing shows and a booking decision turns
// on. The booking history is the evidence, and it is already here.
//
// The cost is real and deliberate: a venue's first review can only come from
// its first paying customer, after they have played. That is the right way
// round.
//
// The author is the caller, never a field in the request, and posting again
// edits rather than stacks: one review per player per arena is the schema's
// rule and the repository upserts on it.
func (s *ReviewService) Review(ctx context.Context, arenaID, userID uuid.UUID, rating int, comment string) (domain.Review, error) {
	if userID == uuid.Nil {
		return domain.Review{}, domain.Unauthenticated("Sign in to review an arena.")
	}

	review := domain.Review{ArenaID: arenaID, UserID: userID, Rating: rating, Comment: comment}
	if err := review.Validate(); err != nil {
		return domain.Review{}, err
	}

	played, err := s.reviews.HasPlayedAt(ctx, arenaID, userID)
	if err != nil {
		return domain.Review{}, err
	}
	if !played {
		return domain.Review{}, domain.Forbidden(
			"You can review an arena once you've played there.")
	}

	return s.reviews.Upsert(ctx, review)
}

// MyReview reports what the caller has said about a venue and whether they
// are allowed to say anything at all.
//
// Both in one call because the page needs both to decide what to render: a
// form, an edit, or an explanation. Asking twice would be two round trips to
// answer one question.
func (s *ReviewService) MyReview(ctx context.Context, arenaID, userID uuid.UUID) (domain.Review, bool, error) {
	if userID == uuid.Nil {
		return domain.Review{}, false, domain.Unauthenticated("Sign in to review an arena.")
	}

	canReview, err := s.reviews.HasPlayedAt(ctx, arenaID, userID)
	if err != nil {
		return domain.Review{}, false, err
	}

	review, err := s.reviews.ReviewByUser(ctx, arenaID, userID)
	if domain.CodeOf(err) == domain.CodeNotFound {
		// Nothing said yet, which is not an error -- it is the answer.
		return domain.Review{}, canReview, nil
	}
	if err != nil {
		return domain.Review{}, false, err
	}
	return review, canReview, nil
}

// DeleteReview removes the caller's own review. Scoped by user id in the SQL,
// so there is no way to delete somebody else's.
func (s *ReviewService) DeleteReview(ctx context.Context, arenaID, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage your reviews.")
	}
	return s.reviews.DeleteReview(ctx, arenaID, userID)
}

func (s *ReviewService) ListReviews(ctx context.Context, arenaID uuid.UUID, limit int) ([]domain.Review, error) {
	return s.reviews.ListReviews(ctx, arenaID, limit)
}

func (s *ReviewService) ListPhotos(ctx context.Context, arenaID uuid.UUID) ([]postgres.Photo, error) {
	return s.reviews.ListPhotos(ctx, arenaID)
}

// AddPhoto puts an image in a venue's gallery. Owner-scoped in the SQL.
func (s *ReviewService) AddPhoto(ctx context.Context, ownerID uuid.UUID, p postgres.Photo) (postgres.Photo, error) {
	if ownerID == uuid.Nil {
		return postgres.Photo{}, domain.Unauthenticated("Sign in to manage a venue.")
	}
	if p.URL == "" {
		return postgres.Photo{}, domain.Invalid("url", "Give the image's web address.")
	}
	return s.reviews.AddPhotoOwnerScoped(ctx, ownerID, p)
}

func (s *ReviewService) DeletePhoto(ctx context.Context, photoID, ownerID uuid.UUID) error {
	if ownerID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage a venue.")
	}
	return s.reviews.DeletePhotoOwnerScoped(ctx, photoID, ownerID)
}

func (s *ReviewService) ListHighlights(ctx context.Context, userID uuid.UUID) ([]postgres.Highlight, error) {
	return s.reviews.ListHighlights(ctx, userID)
}

// AddHighlight puts a link on the caller's own card.
func (s *ReviewService) AddHighlight(ctx context.Context, userID uuid.UUID, title, url string) (postgres.Highlight, error) {
	if userID == uuid.Nil {
		return postgres.Highlight{}, domain.Unauthenticated("Sign in to edit your card.")
	}

	v := &domain.Validation{}
	v.Check(len(title) >= 2 && len(title) <= 80, "title", "Give the clip a name.")
	v.Check(url != "", "url", "Give the clip's web address.")
	if err := v.Err(); err != nil {
		return postgres.Highlight{}, err
	}

	return s.reviews.AddHighlight(ctx, postgres.Highlight{UserID: userID, Title: title, URL: url})
}

func (s *ReviewService) DeleteHighlight(ctx context.Context, highlightID, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return domain.Unauthenticated("Sign in to edit your card.")
	}
	return s.reviews.DeleteHighlight(ctx, highlightID, userID)
}
