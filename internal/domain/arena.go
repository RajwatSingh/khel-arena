package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Arena is a venue ("Dhuku Futsal, Jhamsikhel") that owns one or more courts.
type Arena struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Name        string
	Slug        string
	Area        string
	City        string
	Lat         *float64
	Lng         *float64
	Description string
	CoverURL    string
	Amenities   []string
	Phone       string

	// OpensAt and ClosesAt are wall-clock times in the arena's local zone.
	OpensAt  DayTime
	ClosesAt DayTime

	Rating      *float64
	ReviewCount int
	IsActive    bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// OwnedBy reports whether a user may administer this arena.
func (a Arena) OwnedBy(userID uuid.UUID) bool { return a.OwnerID == userID }

// OperatingHours returns the arena's opening window, for slot validation.
func (a Arena) OperatingHours() (opens, closes DayTime) { return a.OpensAt, a.ClosesAt }

// IsOpenFor reports whether a slot fits entirely within the arena's hours.
func (a Arena) IsOpenFor(slot Slot, loc *time.Location) bool {
	return slot.WithinOperatingHours(a.OpensAt, a.ClosesAt, loc)
}

func (a *Arena) Validate() error {
	a.Name = strings.TrimSpace(a.Name)
	a.Area = strings.TrimSpace(a.Area)
	a.City = strings.TrimSpace(a.City)
	if a.City == "" {
		a.City = "Kathmandu"
	}
	if a.Slug == "" {
		a.Slug = Slugify(a.Name)
	}

	v := &Validation{}
	nameLen := len([]rune(a.Name))
	v.Check(nameLen >= 2, "name", "Give the arena a name.")
	v.Check(nameLen <= 80, "name", "That name is too long.")
	v.Check(a.Area != "", "area", "Which area is the arena in?")
	v.Check(ValidSlug(a.Slug), "slug", "The web address may use lowercase letters, numbers and hyphens only.")
	v.Check(len([]rune(a.Description)) <= 2000, "description", "Keep the description under 2000 characters.")
	v.Check(a.OpensAt.Before(a.ClosesAt), "closes_at", "Closing time must be after opening time.")

	if err := a.OpensAt.Validate(); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}
	if err := a.ClosesAt.Validate(); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}
	if a.Lat != nil {
		v.Check(*a.Lat >= -90 && *a.Lat <= 90, "lat", "Latitude must be between -90 and 90.")
	}
	if a.Lng != nil {
		v.Check(*a.Lng >= -180 && *a.Lng <= 180, "lng", "Longitude must be between -180 and 180.")
	}
	for _, amenity := range a.Amenities {
		if strings.TrimSpace(amenity) == "" {
			v.Add("amenities", "Amenities can't be blank.")
			break
		}
	}

	return v.Err()
}

// ---------------------------------------------------------------------------

// Court is one playable surface within an arena.
type Court struct {
	ID        uuid.UUID
	ArenaID   uuid.UUID
	Label     string
	Sport     Sport
	Surface   string
	SideCount int
	// BasePriceNPR is the hourly fallback when no pricing rule matches.
	BasePriceNPR int
	IsActive     bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (c *Court) Validate() error {
	c.Label = strings.TrimSpace(c.Label)
	if c.Sport == "" {
		c.Sport = SportFutsal
	}
	if c.Surface == "" {
		c.Surface = "artificial_turf"
	}

	v := &Validation{}
	v.Check(c.Label != "", "label", "Name the court, like \"Court A\".")
	v.Check(len([]rune(c.Label)) <= 40, "label", "That court name is too long.")
	v.Check(c.SideCount >= 3 && c.SideCount <= 11, "side_count", "Team size must be between 3 and 11 a side.")
	v.Check(c.BasePriceNPR > 0, "base_price", "Set an hourly price above zero.")
	if err := c.Sport.Validate(); err != nil {
		v.fields = append(v.fields, err.(*Error))
	}
	return v.Err()
}

// ---------------------------------------------------------------------------

// Review is a player's rating of an arena. One per player per arena; posting
// again edits the existing review rather than stacking a second one.
type Review struct {
	ID        uuid.UUID
	ArenaID   uuid.UUID
	UserID    uuid.UUID
	Rating    int
	Comment   string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Author is populated when a review is read as part of a feed.
	Author *UserSummary
}

func (r *Review) Validate() error {
	r.Comment = strings.TrimSpace(r.Comment)
	v := &Validation{}
	v.Check(r.Rating >= 1 && r.Rating <= 5, "rating", "Rate the arena from 1 to 5 stars.")
	v.Check(len([]rune(r.Comment)) <= 500, "comment", "Keep the review under 500 characters.")
	return v.Err()
}

// Photo is an image in an arena's gallery.
type Photo struct {
	ID        uuid.UUID
	ArenaID   uuid.UUID
	URL       string
	Caption   string
	SortOrder int
	CreatedAt time.Time
}

// UserSummary is the slice of a user shown alongside their contributions:
// enough to render an avatar and a name, and nothing more.
type UserSummary struct {
	ID             uuid.UUID
	Username       string
	FullName       string
	AvatarURL      string
	CommunityScore int
}

// ---------------------------------------------------------------------------
// Slugs
// ---------------------------------------------------------------------------

var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	nonSlugRunes    = regexp.MustCompile(`[^a-z0-9]+`)
	repeatedHyphens = regexp.MustCompile(`-{2,}`)
)

func ValidSlug(s string) bool { return s != "" && len(s) <= 80 && slugPattern.MatchString(s) }

// Slugify derives a URL-safe identifier from a display name.
//
// Non-ASCII letters are dropped rather than transliterated, so a name written
// only in Devanagari slugifies to nothing. Callers must handle the empty
// result -- ValidSlug rejects it -- rather than publish an unreachable URL.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	for _, r := range s {
		switch {
		case r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}

	out := nonSlugRunes.ReplaceAllString(b.String(), "-")
	out = repeatedHyphens.ReplaceAllString(out, "-")
	out = strings.Trim(out, "-")
	if len(out) > 80 {
		out = strings.Trim(out[:80], "-")
	}
	return out
}
