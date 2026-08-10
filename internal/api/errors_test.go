package api

import (
	"github.com/RajwatSingh/khel-arena/internal/domain"
	"testing"
	"net/http"
)

func TestCodeToStatus(t *testing.T) {
      cases := map[domain.Code]int{
              domain.CodeInvalid:         http.StatusBadRequest,
              domain.CodeUnauthenticated: http.StatusUnauthorized,
              domain.CodeForbidden:       http.StatusForbidden,
              domain.CodeNotFound:        http.StatusNotFound,
              domain.CodeConflict:        http.StatusConflict,
              domain.CodeRateLimited:     http.StatusTooManyRequests,
              domain.CodeUnavailable:     http.StatusServiceUnavailable,
              domain.CodeInternal:        http.StatusInternalServerError,
              domain.Code("bogus"):       http.StatusInternalServerError,
      }
      for code, want := range cases {
              if got := codeToStatus(code); got != want {
                      t.Errorf("codeToStatus(%q) = %d, want %d", code, got, want)
              }
      }
}
