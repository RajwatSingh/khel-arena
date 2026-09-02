package api

import (
	"context"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/google/uuid"
)

// ctxKey is an unexported struct type so that no other package can construct
// one, and so these keys cannot collide with a key some other package put in
// the same context. A string key can collide with any other string key --
// this is exactly the case the context package documentation warns about.
type ctxKey struct{ name string }

var (
	requestIDKey   = ctxKey{"requestID"}
	userIDKey      = ctxKey{"userID"}
	accountTypeKey = ctxKey{"accountType"}
	callerKey      = ctxKey{"caller"}
)

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func userIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func accountTypeFromContext(ctx context.Context) (domain.AccountType, bool) {
	at, ok := ctx.Value(accountTypeKey).(domain.AccountType)
	return at, ok
}

// caller is how the auth middleware reports who signed in back out to the
// logging middleware wrapped around it.
//
// Context values flow inward only: withAuth derives a new context, and the
// outer withLogging still holds the old one, so it cannot see the user id by
// reading its own request. A pointer placed on the way in and filled on the
// way through is the way across that boundary, and it is why the field is
// written exactly once, by one middleware, before the handler runs.
type caller struct {
	userID      uuid.UUID
	accountType domain.AccountType
}

func withCaller(ctx context.Context) (context.Context, *caller) {
	c := &caller{}
	return context.WithValue(ctx, callerKey, c), c
}

func callerFromContext(ctx context.Context) (*caller, bool) {
	c, ok := ctx.Value(callerKey).(*caller)
	return c, ok
}
