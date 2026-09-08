package api

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
)

// refreshCookieName is where the refresh token lives.
//
// The choice between a cookie and a JSON body is made once, here, and is
// reached only through readRefreshToken and writeSession below -- so changing
// it later means editing those two functions, not four handlers.
//
// A cookie, because the only client today is a browser and an httpOnly cookie
// is unreachable from JavaScript: an XSS that can read localStorage cannot
// read this. The costs are real but small here -- a native client would find
// it awkward, and cross-site requests need SameSite handling -- and the
// frontend already sends credentials: 'include' on every call.
const refreshCookieName = "refresh_token"

// readRefreshToken pulls the refresh token off the request, or returns "" if
// there is none. Every "no token" case is one value, because every caller
// treats them identically: Refresh rejects an empty token, Logout treats it
// as already signed out.
func readRefreshToken(r *http.Request) string {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// writeSession sets the refresh cookie and writes the session body.
//
// The access token goes in the body because the client has to attach it to
// each request as a header; the refresh token goes only in the cookie, so it
// is never in a place a script can read.
func (s *Server) writeSession(w http.ResponseWriter, session service.Session, status int) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    session.RefreshToken,
		Path:     "/",
		Expires:  session.RefreshTokenExpiresAt,
		HttpOnly: true,
		Secure:   s.secureCookies,
		// Lax, not Strict: the cookie must survive a top-level navigation
		// back into the app (a password-reset link in an email), and the
		// frontend is served from the same site as the API -- through the
		// vite proxy in development, one host in production -- so no
		// cross-site request needs it.
		SameSite: http.SameSiteLaxMode,
	})

	encode(w, status, sessionDTOFromDomain(session))
}

// clearRefreshCookie expires the cookie in the client. Value and expiry are
// both cleared: MaxAge alone is ignored by some older clients, and a stale
// value left behind is a credential sitting in a browser for no reason.
func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// sessionContext describes the caller for the session audit trail.
func (s *Server) sessionContext(r *http.Request) postgres.SessionContext {
	return postgres.SessionContext{
		UserAgent: r.UserAgent(),
		IP:        clientIP(r, s.trustedProxyHops),
	}
}

// clientIP is the address recorded against a session and keyed on by the rate
// limiter. Both uses want the same answer, and both are wrong in a different
// way if it is wrong, so there is one function.
//
// The hop count is the whole design. X-Forwarded-For is written by whoever
// sent the request, so reading it blindly lets anyone forge the address in
// their own audit row and, worse, mint a fresh rate-limit bucket per attempt
// -- a limit that can be stepped around is more dangerous than no limit,
// because it reads as protection. Ignoring it entirely is just as wrong once
// a proxy is in front: every request then carries the proxy's address, the
// audit trail records nothing, and the login limiter has one bucket for the
// whole internet, so one attacker locks out every user.
//
// What makes the header safe to read is knowing how many hops to discard.
// With hops == 0 nothing is trusted and RemoteAddr is the answer. With
// hops == N the last N entries of the chain were appended by infrastructure
// we run, and the entry before them is the furthest address we have evidence
// for. Anything an attacker prepends sits to the left of that and is skipped.
//
// This depends on every proxy in front *appending* to X-Forwarded-For rather
// than replacing it: Caddy's reverse_proxy and nginx's
// $proxy_add_x_forwarded_for both do. A proxy that overwrites the header
// makes the chain shorter than the count says, which lands on the fallback
// below rather than on anything a caller chose.
func clientIP(r *http.Request, hops int) string {
	remote := remoteHost(r)
	if hops <= 0 {
		return remote
	}

	// The chain as the last proxy saw it: what was forwarded, then the peer
	// that handed us the request. Header.Values covers a client that split
	// the list across several X-Forwarded-For headers, which is legal and
	// which Get would silently truncate to the first.
	chain := make([]string, 0, 8)
	for _, header := range r.Header.Values("X-Forwarded-For") {
		for _, addr := range strings.Split(header, ",") {
			if addr = strings.TrimSpace(addr); addr != "" {
				chain = append(chain, addr)
			}
		}
	}
	chain = append(chain, remote)

	// Count back past the proxies we run. A chain shorter than the count is a
	// misconfiguration or a proxy that replaced the header rather than
	// appending; either way the honest answer is the peer we can actually
	// see, not a guess taken from further left where a caller could have put
	// it.
	i := len(chain) - 1 - hops
	if i < 0 {
		return remote
	}

	// A forged entry can be any string at all. An unparseable one must not
	// become a rate-limit key -- that is the unbounded-distinct-keys hole the
	// limiter's own sweep exists to avoid -- so it falls back to the peer.
	if net.ParseIP(chain[i]) == nil {
		return remote
	}
	return chain[i]
}

// remoteHost is the peer address with any port stripped.
func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr is not always host:port -- a unix socket, or a test
		// server. postgres.SessionContext stores nil for anything that does
		// not parse as an IP, so a wrong guess here writes an empty audit
		// column rather than failing the request.
		return r.RemoteAddr
	}
	return host
}

// handleRegister — POST /v1/auth/register
//
// Registration is not validated here: AuthService.Register calls
// reg.Validate() itself, so a second copy of those rules in the transport
// layer could only ever drift out of step with the first.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	req, err := decode[registerRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	session, err := s.auth.Register(r.Context(), req.registration(), s.sessionContext(r))
	if err != nil {
		writeError(w, r, err)
		return
	}

	// The signup form also collects a skill tier and a position, which are
	// player-card fields rather than account fields, so they are applied
	// after the account exists. Best effort on purpose: the account and the
	// session are already real, and failing the whole signup over a decorative
	// field would leave the caller with an account they were told they did
	// not get. Worst case they set it again on their profile.
	if p, ok := req.profile(); ok {
		if user, err := s.profiles.Update(r.Context(), session.User.ID, p); err != nil {
			slog.WarnContext(r.Context(), "applying signup profile fields",
				"error", err, "user_id", session.User.ID,
				"request_id", requestIDFromContext(r.Context()))
		} else {
			session.User = user
		}
	}

	s.writeSession(w, session, http.StatusCreated)
}

// handleLogin — POST /v1/auth/login
//
// Rate limited in routes(): this is the online password-guessing target, and
// an unthrottled one is only as strong as the weakest password anybody chose.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	req, err := decode[loginRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	session, err := s.auth.Login(r.Context(), req.Email, req.Password, s.sessionContext(r))
	if err != nil {
		writeError(w, r, err)
		return
	}

	s.writeSession(w, session, http.StatusOK)
}

// handleRefresh — POST /v1/auth/refresh
//
// There is no request body: the token is in the cookie. An absent token still
// goes to the service, which answers "sign in again" -- the handler does not
// get its own opinion about what an empty token means.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	session, err := s.auth.Refresh(r.Context(), readRefreshToken(r), s.sessionContext(r))
	if err != nil {
		// The presented token is now either invalid or rotated away. Either
		// way it will never work again, so clear it rather than leaving the
		// client to retry with a credential that cannot succeed.
		s.clearRefreshCookie(w)
		writeError(w, r, err)
		return
	}

	s.writeSession(w, session, http.StatusOK)
}

// handleLogout — POST /v1/auth/logout
//
// Idempotent: 204 whether or not there was a session to end. AuthService.Logout
// already treats an empty token as "already signed out", so there is no check
// in front of it here.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.Logout(r.Context(), readRefreshToken(r)); err != nil {
		writeError(w, r, err)
		return
	}

	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handlePasswordForgot — POST /v1/auth/password/forgot
//
// Two things this must not do, both of which the obvious implementation does:
//
// It must not return the reset token. BeginPasswordReset hands the plaintext
// back to its caller so the caller can mail it; putting it in the response
// would let anyone reset anyone's password by calling this endpoint and
// reading the reply.
//
// It must not branch on whether an account was found. BeginPasswordReset
// returns an empty token and no error for an unknown address precisely so
// this endpoint cannot be used to discover which addresses are registered.
// The answer is 202 either way.
//
// Rate limited in routes(): unthrottled, this endpoint sprays an inbox on
// request, and now that it actually sends mail that is no longer theoretical.
func (s *Server) handlePasswordForgot(w http.ResponseWriter, r *http.Request) {
	req, err := decode[forgotPasswordRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	resetToken, user, err := s.auth.BeginPasswordReset(r.Context(), req.Email)
	if err != nil {
		// A real failure -- the database is down, say. Reporting it leaks
		// nothing about whether the address exists, and pretending it
		// succeeded would leave someone waiting for a mail nobody will send.
		writeError(w, r, err)
		return
	}

	// Mail it. The notifier swallows its own failures on purpose: answering
	// differently when delivery fails would tell a caller that the address was
	// registered, which is the property this endpoint exists to protect.
	if s.mailer != nil {
		s.mailer.SendPasswordReset(r.Context(), user, resetToken)
	}

	// Outside production the token is also logged, so the flow can be
	// completed on a laptop with no mail server. The gate is what makes this
	// structurally unreachable in production rather than merely unlikely.
	if s.logResetTokens && resetToken != "" {
		slog.InfoContext(r.Context(), "password reset token (development only)",
			"user_id", user.ID, "username", user.Username, "token", resetToken)
	}

	w.WriteHeader(http.StatusAccepted)
}

// handlePasswordReset — POST /v1/auth/password/reset
//
// 204: the token is burnt as a side effect and there is nothing to hand back.
// The caller signs in with the new password like anyone else.
func (s *Server) handlePasswordReset(w http.ResponseWriter, r *http.Request) {
	req, err := decode[resetPasswordRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.auth.CompletePasswordReset(r.Context(), req.Token, req.NewPassword); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handlePasswordChange — POST /v1/auth/password/change (authenticated)
//
// The user id comes from the access token, never the body: otherwise anyone
// holding one valid session could change any account's password by naming a
// different id.
func (s *Server) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[changePasswordRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.auth.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		writeError(w, r, err)
		return
	}

	// AuthService.ChangePassword revokes every session the account holds, so
	// the cookie this request arrived with is already dead. Clear it rather
	// than leaving the client to discover that on its next refresh.
	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handlePlayer — GET /v1/players/{username}
//
// Somebody's public card, with their highlight reel. Deliberately a different
// shape from /v1/me: that one returns the account, this one returns the
// player. Email and verification state are on the account and stop here.
//
// The clips come back with the card rather than behind a second request: the
// page wants both, and the id-addressed highlights endpoint exists for callers
// that already know who they are asking about.
func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	user, err := s.profiles.ByUsername(r.Context(), r.PathValue("username"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	dto := playerDTOFromDomain(user)

	if s.reviews != nil {
		highlights, err := s.reviews.ListHighlights(r.Context(), user.ID)
		if err != nil {
			writeError(w, r, err)
			return
		}
		dto.Highlights = highlightDTOsFromDomain(highlights)
	}

	encode(w, http.StatusOK, dto)
}

// handleMe — GET /v1/me (authenticated)
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	user, err := s.profiles.Me(r.Context(), userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, userDTOFromDomain(user))
}
