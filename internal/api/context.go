package api

import (
	"context"
)

type ctxKey struct {name string}

var (
	requestIDKey = ctxKey{"requestID"}
	userIDKey = ctxKey{"userID"}
	accountTypeKey = ctxKey{"accountType"}
)

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func userIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

func accountTypeFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(accountTypeKey).(string)
	return id, ok
}
