package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/mail"
)

// Notifier composes and sends the transactional email this service owns.
//
// It sits between the auth service and the mail sender so that neither has to
// know about the other: `AuthService` produces a reset token and knows nothing
// about URLs or wording, and `mail.Sender` moves bytes and knows nothing about
// password resets.
type Notifier struct {
	sender mail.Sender
	// AppURL is the interface's own origin, used to build the link a person
	// clicks. It is configuration rather than anything derived from a request:
	// building it from a Host header would let whoever sent the request choose
	// the domain in the email.
	appURL string
	log    *slog.Logger
}

func NewNotifier(sender mail.Sender, appURL string, log *slog.Logger) *Notifier {
	if log == nil {
		log = slog.Default()
	}
	return &Notifier{sender: sender, appURL: appURL, log: log}
}

// PasswordReset sends the reset link.
//
// The token is a bearer credential for the account, so the only place it goes
// is into the message body addressed to the account's own address. It is
// deliberately never logged here and never returned to the caller -- the
// handler that triggers this answers 202 either way.
func (n *Notifier) PasswordReset(ctx context.Context, user domain.User, token string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", n.appURL, url.QueryEscape(token))

	body := fmt.Sprintf(`Hello %s,

Someone asked to reset the password on your Khel Arena account.

Open this link to choose a new one:

%s

The link works once and expires in an hour.

If this wasn't you, nothing has changed and you can ignore this message.

— Khel Arena
`, user.DisplayName(), link)

	return n.sender.Send(ctx, mail.Message{
		To:      user.Email,
		Subject: "Reset your Khel Arena password",
		Body:    body,
	})
}

// SendPasswordReset delivers the link and reports nothing to the caller.
//
// Failure is logged rather than returned on purpose. The endpoint behind this
// answers 202 whether or not the address is registered -- that is what stops
// it being used to discover who has an account -- and returning an error when
// delivery fails would reintroduce exactly that difference: a registered
// address that bounced would answer differently from an unknown one.
//
// The cost is that a genuine mail outage is invisible to the person waiting
// for the link. That is what the log is for, and it is the right trade: the
// alternative leaks account existence to anyone who can read a status code.
func (n *Notifier) SendPasswordReset(ctx context.Context, user domain.User, token string) {
	if token == "" {
		// No such address. The caller cannot be told that.
		return
	}
	if err := n.PasswordReset(ctx, user, token); err != nil {
		n.log.ErrorContext(ctx, "sending password reset email",
			"error", err, "user_id", user.ID)
	}
}
