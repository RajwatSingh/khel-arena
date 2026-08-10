package api

import (
	"net/http"
	"errors"
	"github.com/RajwatSingh/khel-arena/internal/domain"
	"log/slog"
)

func codeToStatus(code domain.Code) int {
	switch code {
	case domain.CodeInvalid:
		return http.StatusBadRequest
	case domain.CodeUnauthenticated:
		return http.StatusUnauthorized
	case domain.CodeForbidden:
		return http.StatusForbidden
	case domain.CodeNotFound:
		return http.StatusNotFound
	case domain.CodeConflict:
		return http.StatusConflict
	case domain.CodeRateLimited:
		return http.StatusTooManyRequests
	case domain.CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

type Resp struct {
	Err Error `json:"error"`
}

type Error struct {
	Code string `json:"code"`
	Message string `json:"message"`
	Fields []Field `json:"fields,omitempty"`
}

type Field struct {
	F string `json:"field"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	code := domain.CodeOf(err)
	status := codeToStatus(code)

	if code == domain.CodeInternal {

		var de *domain.Error
		cause := error(err)
		if errors.As(err, &de) {
			if de.Unwrap() != nil {
				cause = de.Unwrap()
			}
		}
		slog.ErrorContext(r.Context(), "internal error",
			"error", cause,
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
        )
	}
	
	body := Error{
		Code: string(code),
		Message: domain.UserMessage(err),
		Fields: fieldsFromError(err),
	}

	encode(w, status, Resp{Err: body})
}

func fieldsFromError(err error) []Field {
	var de *domain.Error
	if !errors.As(err, &de) {
		return nil
	}
	fes := de.FieldErrors()
	if len(fes) == 0 {
		return nil
	}

	out := make([]Field, len(fes))

	for i, fe := range fes {
		out[i] = Field{F: fe.Field, Message: fe.Message}
	}
	 return out
}
