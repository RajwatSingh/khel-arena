package api

import (
	"errors"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"net/http"
)

func readRefreshToken(r *http.Request) string {
	cookie, err := r.Cookie("refresh_token")

	if errors.Is(err, http.ErrNoCookie) {
		return ""
	}

	return cookie.Value
}

func (s *Server) writeSession(w http.ResponseWriter, session service.Session, status int) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    session.RefreshToken,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		Expires:  session.RefreshTokenExpiresAt,
	})

	encode(w, status, sessionDTOFromDomain(session))
}
