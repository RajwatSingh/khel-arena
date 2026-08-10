package domain

import (
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User is one account: the credentials that prove who someone is and the
// public player card everyone else sees.
//
// The previous system split these across a Supabase `auth.users` row and a
// `profiles` row joined on every read, which meant an account could exist
// with no profile. One row, one identity.
type User struct {
	ID              uuid.UUID
	Email           string
	EmailVerifiedAt *time.Time

	Username    string
	FullName    string
	AccountType AccountType
	AvatarURL   string
	Phone       string
	City        string

	Position       *Position
	JerseyNumber   *int
	PreferredFoot  *Foot
	Skill          SkillTier
	Bio            string

	MatchesPlayed  int
	MatchesWon     int
	CommunityScore int

	CreatedAt time.Time
	UpdatedAt time.Time
}

/** 
PasswordHash never appears on User. It is loaded only by the one repository
method that authenticates a login, so no handler can accidentally serialise
it. Credentials travel in this separate struct. 
**/
type Credentials struct {
	UserID       uuid.UUID
	Email        string
	PasswordHash string
}

func (u User) IsArenaOwner() bool  { return u.AccountType == AccountArenaOwner }
func (u User) EmailVerified() bool { return u.EmailVerifiedAt != nil }

// DisplayName is what to show when a full name is missing.
func (u User) DisplayName() string {
	if u.FullName != "" {
		return u.FullName
	}
	return "@" + u.Username
}

var (
	usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,24}$`)
	// Nepali mobile numbers: ten digits beginning 98 or 97.
	nepaliMobilePattern = regexp.MustCompile(`^(98|97)[0-9]{8}$`)
)

// Reserved names would collide with routes or impersonate the service.
var reservedUsernames = map[string]bool{
	"admin": true, "administrator": true, "root": true, "system": true,
	"khelarena": true, "khel_arena": true, "support": true, "help": true,
	"api": true, "auth": true, "login": true, "logout": true, "register": true,
	"me": true, "settings": true, "about": true, "null": true, "undefined": true,
}

// NormalizeEmail lowercases and trims an address so that the same mailbox
// cannot register twice under different capitalisation. The database column
// is citext, which enforces this independently.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizeUsername trims and lowercases, so that a user typing "RajWat"
// during signup gets the handle they expect rather than a rejection.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func ValidateEmail(email string) error {
	email = NormalizeEmail(email)
	if email == "" {
		return Invalid("email", "An email address is required.")
	}
	if len(email) > 254 {
		return Invalid("email", "That email address is too long.")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return Invalid("email", "That doesn't look like an email address.")
	}
	if !strings.Contains(strings.SplitN(email, "@", 2)[1], ".") {
		return Invalid("email", "That doesn't look like an email address.")
	}
	return nil
}

func ValidateUsername(username string) error {
	username = NormalizeUsername(username)
	switch {
	case username == "":
		return Invalid("username", "A username is required.")
	case len(username) < 3:
		return Invalid("username", "Usernames need at least 3 characters.")
	case len(username) > 24:
		return Invalid("username", "Usernames can be at most 24 characters.")
	case !usernamePattern.MatchString(username):
		return Invalid("username", "Usernames can use lowercase letters, numbers and underscores only.")
	case reservedUsernames[username]:
		return Invalid("username", "That username is reserved. Please pick another.")
	}
	return nil
}

func ValidatePhone(phone string) error {
	if phone == "" {
		return nil // optional
	}
	if !nepaliMobilePattern.MatchString(strings.TrimSpace(phone)) {
		return Invalid("phone", "Enter a 10-digit Nepali mobile number starting 98 or 97.")
	}
	return nil
}

// Registration is a validated signup request, ready to be persisted.
type Registration struct {
	Email       string
	Username    string
	FullName    string
	AccountType AccountType
	Password    string
}

// Validate checks everything about a signup at once, so the person filling in
// the form learns about all their mistakes in a single response.
func (r *Registration) Validate() error {
	r.Email = NormalizeEmail(r.Email)
	r.Username = NormalizeUsername(r.Username)
	r.FullName = strings.TrimSpace(r.FullName)
	if r.AccountType == "" {
		r.AccountType = AccountPlayer
	}

	v := &Validation{}
	if err := ValidateEmail(r.Email); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}
	if err := ValidateUsername(r.Username); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}
	if err := ValidatePassword(r.Password); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}
	if err := r.AccountType.Validate(); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}

	nameLen := len([]rune(r.FullName))
	v.Check(nameLen >= 1, "full_name", "Tell us your name.")
	v.Check(nameLen <= 80, "full_name", "That name is too long.")

	return v.Err()
}

// Password rules.
//
// Length is the requirement that actually correlates with strength, so it is
// the one enforced. Composition rules ("one uppercase, one symbol") push
// people toward predictable substitutions and are deliberately not imposed.
const (
	MinPasswordLength = 10
	// Bounded because Argon2 hashes the whole input; an unbounded password
	// is an invitation to spend the server's memory on someone else's behalf.
	MaxPasswordLength = 256
)

func ValidatePassword(password string) error {
	switch n := len(password); {
	case n == 0:
		return Invalid("password", "A password is required.")
	case n < MinPasswordLength:
		return Invalid("password", "Use at least %d characters.", MinPasswordLength)
	case n > MaxPasswordLength:
		return Invalid("password", "Passwords can be at most %d characters.", MaxPasswordLength)
	}
	if strings.TrimSpace(password) == "" {
		return Invalid("password", "A password can't be only spaces.")
	}
	return nil
}

// ProfileUpdate carries the editable parts of a player card. Pointer fields
// distinguish "leave this alone" (nil) from "clear this" (pointer to zero),
// so a partial update cannot silently blank a field it never mentioned.
type ProfileUpdate struct {
	FullName      *string
	AvatarURL     *string
	Phone         *string
	City          *string
	Position      **Position
	JerseyNumber  **int
	PreferredFoot **Foot
	Skill         *SkillTier
	Bio           *string
}

func (p *ProfileUpdate) Validate() error {
	v := &Validation{}

	if p.FullName != nil {
		*p.FullName = strings.TrimSpace(*p.FullName)
		n := len([]rune(*p.FullName))
		v.Check(n >= 1, "full_name", "Tell us your name.")
		v.Check(n <= 80, "full_name", "That name is too long.")
	}
	if p.Phone != nil {
		if err := ValidatePhone(*p.Phone); err != nil {
			v.fields = append(v.fields, err.(*Error))
		}
	}
	if p.Bio != nil && len([]rune(*p.Bio)) > 280 {
		v.Add("bio", "Keep your bio under 280 characters.")
	}
	if p.City != nil {
		*p.City = strings.TrimSpace(*p.City)
		v.Check(len([]rune(*p.City)) <= 60, "city", "That city name is too long.")
	}
	if p.Skill != nil {
		if err := p.Skill.Validate(); err != nil {
			v.fields = append(v.fields, err.(*Error))
		}
	}
	if p.Position != nil && *p.Position != nil {
		if err := (*p.Position).Validate(); err != nil {
			v.fields = append(v.fields, err.(*Error))
		}
	}
	if p.PreferredFoot != nil && *p.PreferredFoot != nil {
		if err := (*p.PreferredFoot).Validate(); err != nil {
			v.fields = append(v.fields, err.(*Error))
		}
	}
	if p.JerseyNumber != nil && *p.JerseyNumber != nil {
		n := **p.JerseyNumber
		v.Check(n >= 0 && n <= 99, "jersey_number", "Squad numbers run from 0 to 99.")
	}

	return v.Err()
}
