package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

func TestHandleRegister(t *testing.T) {
	t.Run("creates an account and signs in", func(t *testing.T) {
		var gotReg domain.Registration
		auth := &fakeAuth{register: func(_ context.Context, reg domain.Registration, _ postgres.SessionContext) (service.Session, error) {
			gotReg = reg
			return testSession(), nil
		}}
		profiles := &fakeProfiles{update: func(_ context.Context, _ uuid.UUID, _ domain.ProfileUpdate) (domain.User, error) {
			return testUser, nil
		}}

		w := do(newTestServer(t, auth, nil, profiles), http.MethodPost, "/v1/auth/register", `{
			"full_name": "Rajwat Singh",
			"username": "rajwat",
			"email": "rajwat@khelarena.np",
			"password": "kathmandu2026",
			"skill": "intermediate",
			"position": "Ala"
		}`)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s), want 201", w.Code, w.Body.String())
		}
		if gotReg.Email != "rajwat@khelarena.np" || gotReg.Username != "rajwat" {
			t.Errorf("registration passed to the service = %+v", gotReg)
		}

		var got sessionDTO
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding session: %v", err)
		}
		if got.AccessToken != testAccessToken {
			t.Errorf("access_token = %q, want %q", got.AccessToken, testAccessToken)
		}
		if got.User.Username != testUser.Username {
			t.Errorf("user.username = %q, want %q", got.User.Username, testUser.Username)
		}

		// The skill and position the signup form collects are player-card
		// fields, applied through the profile service after the account
		// exists -- and, crucially, not rejected as unknown JSON.
		if !profiles.updated {
			t.Error("signup profile fields were dropped instead of applied")
		}
		if profiles.gotUpdate.Skill == nil || *profiles.gotUpdate.Skill != domain.SkillIntermediate {
			t.Errorf("skill in profile update = %v, want intermediate", profiles.gotUpdate.Skill)
		}
	})

	// The refresh token is the long-lived credential. It leaves in an httpOnly
	// cookie so a script cannot read it; putting it in the body as well would
	// give that protection straight back.
	t.Run("refresh token goes in the cookie, never the body", func(t *testing.T) {
		auth := &fakeAuth{register: func(context.Context, domain.Registration, postgres.SessionContext) (service.Session, error) {
			return testSession(), nil
		}}

		w := do(newTestServer(t, auth, nil, &fakeProfiles{}), http.MethodPost, "/v1/auth/register",
			`{"full_name":"R","username":"rajwat","email":"r@k.np","password":"kathmandu2026"}`)

		if strings.Contains(w.Body.String(), "test-refresh-token") {
			t.Errorf("refresh token in the response body: %s", w.Body.String())
		}

		cookie := cookieNamed(t, w, refreshCookieName)
		if cookie == nil {
			t.Fatal("no refresh cookie was set")
		}
		if cookie.Value != "test-refresh-token" {
			t.Errorf("cookie value = %q", cookie.Value)
		}
		if !cookie.HttpOnly {
			t.Error("refresh cookie is not HttpOnly, so a script can read it")
		}
	})

	t.Run("validation failures come back per field", func(t *testing.T) {
		v := &domain.Validation{}
		v.Add("username", "Usernames need at least 3 characters.")
		auth := &fakeAuth{register: func(context.Context, domain.Registration, postgres.SessionContext) (service.Session, error) {
			return service.Session{}, v.Err()
		}}

		w := do(newTestServer(t, auth, nil, &fakeProfiles{}), http.MethodPost, "/v1/auth/register",
			`{"full_name":"R","username":"ab","email":"r@k.np","password":"kathmandu2026"}`)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
		var env errorEnvelope
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if len(env.Error.Fields) != 1 || env.Error.Fields[0].Field != "username" {
			t.Errorf("fields = %+v, want one for username", env.Error.Fields)
		}
	})

	t.Run("a typo'd field is a 400, not a silent drop", func(t *testing.T) {
		w := do(newTestServer(t, &fakeAuth{}, nil, &fakeProfiles{}), http.MethodPost, "/v1/auth/register",
			`{"user_name":"rajwat","email":"r@k.np","password":"kathmandu2026"}`)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400 for an unknown field", w.Code)
		}
	})
}

func TestHandleLogin(t *testing.T) {
	t.Run("signs in", func(t *testing.T) {
		auth := &fakeAuth{login: func(_ context.Context, email, password string, _ postgres.SessionContext) (service.Session, error) {
			if email != "rajwat@khelarena.np" || password != "kathmandu2026" {
				t.Errorf("credentials passed through = %q / %q", email, password)
			}
			return testSession(), nil
		}}

		w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/login",
			`{"email":"rajwat@khelarena.np","password":"kathmandu2026"}`)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
		}
		if cookieNamed(t, w, refreshCookieName) == nil {
			t.Error("no refresh cookie was set")
		}
	})

	t.Run("bad credentials are a 401 with one message", func(t *testing.T) {
		auth := &fakeAuth{login: func(context.Context, string, string, postgres.SessionContext) (service.Session, error) {
			return service.Session{}, domain.Unauthenticated("That email and password don't match.")
		}}

		w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/login",
			`{"email":"nobody@khelarena.np","password":"wrong"}`)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
		if cookieNamed(t, w, refreshCookieName) != nil {
			t.Error("a failed login set a refresh cookie")
		}
	})

	// The session audit row is only worth writing if it describes the caller,
	// so the handler must actually build a SessionContext rather than pass a
	// zero value along.
	t.Run("records the caller for the session audit", func(t *testing.T) {
		auth := &fakeAuth{login: func(context.Context, string, string, postgres.SessionContext) (service.Session, error) {
			return testSession(), nil
		}}

		do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/login",
			`{"email":"r@k.np","password":"kathmandu2026"}`,
			func(r *http.Request) {
				r.RemoteAddr = "203.0.113.7:54321"
				r.Header.Set("User-Agent", "khel-test/1.0")
			})

		if auth.gotSessionContext.IP != "203.0.113.7" {
			t.Errorf("session IP = %q, want the host part of RemoteAddr", auth.gotSessionContext.IP)
		}
		if auth.gotSessionContext.UserAgent != "khel-test/1.0" {
			t.Errorf("session user agent = %q", auth.gotSessionContext.UserAgent)
		}
	})

	// X-Forwarded-For is set by the client and can say anything. Reading it
	// without knowing the trusted hop count lets anyone forge the address
	// recorded against their own session.
	t.Run("ignores a spoofable X-Forwarded-For", func(t *testing.T) {
		auth := &fakeAuth{login: func(context.Context, string, string, postgres.SessionContext) (service.Session, error) {
			return testSession(), nil
		}}

		do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/login",
			`{"email":"r@k.np","password":"kathmandu2026"}`,
			func(r *http.Request) {
				r.RemoteAddr = "203.0.113.7:54321"
				r.Header.Set("X-Forwarded-For", "10.0.0.1")
			})

		if auth.gotSessionContext.IP != "203.0.113.7" {
			t.Errorf("session IP = %q, want RemoteAddr and not the header", auth.gotSessionContext.IP)
		}
	})
}

func TestHandleRefresh(t *testing.T) {
	t.Run("rotates the pair", func(t *testing.T) {
		auth := &fakeAuth{refresh: func(context.Context, string, postgres.SessionContext) (service.Session, error) {
			return testSession(), nil
		}}

		w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/refresh", "",
			withCookie(refreshCookieName, "old-refresh-token"))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
		}
		if auth.gotRefreshToken != "old-refresh-token" {
			t.Errorf("token passed to the service = %q, want the one in the cookie", auth.gotRefreshToken)
		}
	})

	// A rejected token can never work again, so leaving it in the browser
	// only guarantees the next refresh fails the same way.
	t.Run("clears the cookie when the token is refused", func(t *testing.T) {
		auth := &fakeAuth{refresh: func(context.Context, string, postgres.SessionContext) (service.Session, error) {
			return service.Session{}, domain.Unauthenticated("Your session has expired. Please sign in again.")
		}}

		w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/refresh", "",
			withCookie(refreshCookieName, "stale"))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
		cookie := cookieNamed(t, w, refreshCookieName)
		if cookie == nil || cookie.Value != "" || cookie.MaxAge >= 0 {
			t.Errorf("refresh cookie = %+v, want it cleared", cookie)
		}
	})

	t.Run("no cookie still reaches the service", func(t *testing.T) {
		called := false
		auth := &fakeAuth{refresh: func(_ context.Context, token string, _ postgres.SessionContext) (service.Session, error) {
			called = true
			if token != "" {
				t.Errorf("token = %q, want empty", token)
			}
			return service.Session{}, domain.Unauthenticated("Your session has expired. Please sign in again.")
		}}

		do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/refresh", "")

		if !called {
			t.Error("the handler decided what an empty token means instead of asking the service")
		}
	})
}

// Logout is idempotent: signing out twice, or without a session at all, is
// not an error a client should have to handle.
func TestHandleLogoutIsIdempotent(t *testing.T) {
	cases := map[string][]func(*http.Request){
		"with a session":     {withCookie(refreshCookieName, "a-token")},
		"already signed out": nil,
	}

	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			auth := &fakeAuth{logout: func(context.Context, string) error { return nil }}

			w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/logout", "", opts...)

			if w.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", w.Code)
			}
			cookie := cookieNamed(t, w, refreshCookieName)
			if cookie == nil || cookie.Value != "" {
				t.Errorf("refresh cookie = %+v, want it cleared", cookie)
			}
		})
	}
}

func TestHandlePasswordForgot(t *testing.T) {
	// The reset token is a bearer credential: whoever holds it owns the
	// account. Returning it here would let anyone reset anyone's password by
	// calling this endpoint and reading the reply.
	t.Run("never returns the reset token", func(t *testing.T) {
		auth := &fakeAuth{beginPasswordReset: func(context.Context, string) (string, domain.User, error) {
			return "the-secret-reset-token", testUser, nil
		}}

		w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/password/forgot",
			`{"email":"rajwat@khelarena.np"}`)

		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202", w.Code)
		}
		if strings.Contains(w.Body.String(), "the-secret-reset-token") {
			t.Fatalf("reset token in the response: %s", w.Body.String())
		}
		if w.Body.Len() != 0 {
			t.Errorf("body = %q, want it empty", w.Body.String())
		}
	})

	// Answering differently for an unknown address turns this endpoint into a
	// way to discover which addresses are registered.
	t.Run("an unknown address is indistinguishable from a known one", func(t *testing.T) {
		known := &fakeAuth{beginPasswordReset: func(context.Context, string) (string, domain.User, error) {
			return "a-token", testUser, nil
		}}
		unknown := &fakeAuth{beginPasswordReset: func(context.Context, string) (string, domain.User, error) {
			return "", domain.User{}, nil // what the service returns for an address it has never seen
		}}

		a := do(newTestServer(t, known, nil, nil), http.MethodPost, "/v1/auth/password/forgot", `{"email":"rajwat@khelarena.np"}`)
		b := do(newTestServer(t, unknown, nil, nil), http.MethodPost, "/v1/auth/password/forgot", `{"email":"nobody@khelarena.np"}`)

		if a.Code != b.Code {
			t.Errorf("status differs by whether the address exists: %d vs %d", a.Code, b.Code)
		}
		if a.Body.String() != b.Body.String() {
			t.Errorf("body differs by whether the address exists: %q vs %q", a.Body.String(), b.Body.String())
		}
	})
}

func TestHandlePasswordReset(t *testing.T) {
	auth := &fakeAuth{completePasswordReset: func(_ context.Context, token, next string) error {
		if token != "reset-token" || next != "newpassword2026" {
			t.Errorf("service got %q / %q", token, next)
		}
		return nil
	}}

	w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/password/reset",
		`{"token":"reset-token","new_password":"newpassword2026"}`)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d (%s), want 204", w.Code, w.Body.String())
	}
}

func TestHandlePasswordChange(t *testing.T) {
	t.Run("changes the signed-in user's password", func(t *testing.T) {
		auth := &fakeAuth{
			authenticate: signedIn(testAccessToken),
			changePassword: func(_ context.Context, userID uuid.UUID, current, next string) error {
				// The id comes from the token, not the body: otherwise one
				// valid session could change any account's password.
				if userID != testUser.ID {
					t.Errorf("user id = %v, want the one from the token %v", userID, testUser.ID)
				}
				if current != "kathmandu2026" || next != "newpassword2026" {
					t.Errorf("passwords passed through = %q / %q", current, next)
				}
				return nil
			},
		}

		w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/password/change",
			`{"current_password":"kathmandu2026","new_password":"newpassword2026"}`,
			bearer(testAccessToken))

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d (%s), want 204", w.Code, w.Body.String())
		}
	})

	t.Run("requires a session", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, nil, nil), http.MethodPost, "/v1/auth/password/change",
			`{"current_password":"a","new_password":"b"}`)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})
}

func TestHandleMe(t *testing.T) {
	t.Run("returns the caller's own account", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		profiles := &fakeProfiles{me: func(_ context.Context, userID uuid.UUID) (domain.User, error) {
			if userID != testUser.ID {
				t.Errorf("user id = %v, want %v", userID, testUser.ID)
			}
			return testUser, nil
		}}

		w := do(newTestServer(t, auth, nil, profiles), http.MethodGet, "/v1/me", "", bearer(testAccessToken))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
		}

		var got userDTO
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding user: %v", err)
		}
		if got.ID != testUser.ID || got.Email != testUser.Email {
			t.Errorf("user = %+v", got)
		}
	})

	t.Run("without a token", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, nil, &fakeProfiles{}), http.MethodGet, "/v1/me", "")

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})
}
