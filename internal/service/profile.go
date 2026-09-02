package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// ProfileStore is the account storage this service needs. Narrow on purpose:
// the profile use cases read one user and write their player card, and
// nothing here should be able to touch credentials.
type ProfileStore interface {
	ByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, p domain.ProfileUpdate) (domain.User, error)
}

// ProfileService answers "who am I" and "change my player card".
//
// It exists so the transport layer never reaches for UserRepo directly. A
// handler that calls a repository has skipped the layer where authorization
// and business rules live, and the moment that read needs one -- a
// permission check, a derived field -- there is nowhere to put it.
type ProfileService struct {
	users ProfileStore
}

func NewProfileService(users ProfileStore) *ProfileService {
	return &ProfileService{users: users}
}

// Me loads the signed-in user's own account.
//
// The nil check is defence in depth: the auth middleware cannot produce a nil
// user ID, but a handler wired without it could, and "look up the zero UUID"
// must never be allowed to mean "look up somebody".
func (s *ProfileService) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	if userID == uuid.Nil {
		return domain.User{}, domain.Unauthenticated("Please sign in.")
	}
	return s.users.ByID(ctx, userID)
}

// Update changes the caller's own player card and returns the account as it
// now stands.
//
// Validation runs here rather than in the handler so that every caller of
// this use case gets it, and so the field names in the resulting
// domain.Error match the JSON the client sent.
func (s *ProfileService) Update(ctx context.Context, userID uuid.UUID, p domain.ProfileUpdate) (domain.User, error) {
	if userID == uuid.Nil {
		return domain.User{}, domain.Unauthenticated("Please sign in.")
	}
	if err := p.Validate(); err != nil {
		return domain.User{}, err
	}
	return s.users.UpdateProfile(ctx, userID, p)
}
