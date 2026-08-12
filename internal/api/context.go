package api

import (
	"context"
	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/google/uuid"
)

type ctxKey struct{ name string }

var (
	requestIDKey   = ctxKey{"requestID"}
	userIDKey      = ctxKey{"userID"}
	accountTypeKey = ctxKey{"accountType"}
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
	id, ok := ctx.Value(accountTypeKey).(domain.AccountType)
	return id, ok
}
