package domain

import (
	"fmt"
	"time"
)

// Booking duration limits. A court is sold in whole hours, but a half-hour
// floor leaves room for arenas that sell 30-minute practice slots.
const (
	MinBookingDuration = 30 * time.Minute
	MaxBookingDuration = 4 * time.Hour

	// SlotGranularity is the step the availability grid is projected on.
	SlotGranularity = time.Hour

	// PastSlotGrace forgives a request whose start time has just slipped
	// past `now` while it was in flight. Without it, a player clicking the
	// 18:00 slot at 17:59:59.8 gets a confusing rejection.
	PastSlotGrace = time.Minute
)

// Slot is a half-open time interval [Start, End) on a single court.
//
// Half-open is what makes 13:00-14:00 and 14:00-15:00 adjacent rather than
// overlapping, and it is the same convention as the `tstzrange` the database
// stores, so the two can never disagree about whether two bookings collide.
type Slot struct {
	Start time.Time
	End   time.Time
}

// NewSlot builds a validated slot. Both instants are normalised to UTC so
// that equality and comparison never depend on a zone offset.
func NewSlot(start, end time.Time) (Slot, error) {
	s := Slot{Start: start.UTC(), End: end.UTC()}
	if err := s.Validate(); err != nil {
		return Slot{}, err
	}
	return s, nil
}

// Validate enforces the rules that hold for any slot, independent of court,
// arena hours or the current time.
func (s Slot) Validate() error {
	if s.Start.IsZero() || s.End.IsZero() {
		return Invalid("slot", "A start and end time are both required.")
	}
	if !s.End.After(s.Start) {
		return Invalid("slot", "The end time must be after the start time.")
	}

	d := s.Duration()
	if d < MinBookingDuration {
		return Invalid("slot", "Bookings must be at least %s long.", humanDuration(MinBookingDuration))
	}
	if d > MaxBookingDuration {
		return Invalid("slot", "Bookings are limited to %s.", humanDuration(MaxBookingDuration))
	}
	return nil
}

func (s Slot) Duration() time.Duration { return s.End.Sub(s.Start) }

// Hours is the number of billable hours, rounded up: pricing rules are quoted
// per hour, so a 90-minute booking is charged as two.
func (s Slot) Hours() int {
	h := int(s.Duration() / time.Hour)
	if s.Duration()%time.Hour != 0 {
		h++
	}
	return h
}

// Overlaps reports whether two slots share any instant. This mirrors the `&&`
// operator the database applies to the same pair of ranges.
func (s Slot) Overlaps(other Slot) bool {
	return s.Start.Before(other.End) && other.Start.Before(s.End)
}

// IsPast reports whether the slot has already started as of now, allowing
// PastSlotGrace for a request that was in flight across the boundary.
func (s Slot) IsPast(now time.Time) bool {
	return s.Start.Before(now.Add(-PastSlotGrace))
}

// WithinOperatingHours reports whether the slot fits inside an arena's opening
// hours, which are published as wall-clock times in the arena's local zone.
//
// The comparison is done in local time on purpose: an arena open "06:00 to
// 22:00" means those hours on the clock on the wall, and that is not a fixed
// offset from UTC everywhere or forever.
func (s Slot) WithinOperatingHours(opens, closes DayTime, loc *time.Location) bool {
	start := s.Start.In(loc)
	end := s.End.In(loc)

	// A slot that runs past midnight cannot sit inside a single day's hours.
	if !sameLocalDay(start, end) {
		// The sole exception is ending exactly at midnight, which is the
		// natural end of the previous day rather than the start of the next.
		if !(end.Hour() == 0 && end.Minute() == 0 && end.Second() == 0 && end.AddDate(0, 0, -1).YearDay() == start.YearDay()) {
			return false
		}
		return closes.Equal(DayTime{Hour: 24}) || closes.Equal(DayTime{})
	}

	return !DayTimeOf(start).Before(opens) && !DayTimeOf(end).After(closes)
}

func sameLocalDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func (s Slot) String() string {
	return fmt.Sprintf("[%s, %s)", s.Start.Format(time.RFC3339), s.End.Format(time.RFC3339))
}

// ---------------------------------------------------------------------------
// DayTime
// ---------------------------------------------------------------------------

// DayTime is a wall-clock time of day with no date attached -- what Postgres
// stores in a `time` column and what an arena means by "we open at six".
//
// Hour 24 is legal and means end-of-day, so an arena can close at midnight
// without the closing time sorting before the opening time.
type DayTime struct {
	Hour   int
	Minute int
}

func DayTimeOf(t time.Time) DayTime {
	return DayTime{Hour: t.Hour(), Minute: t.Minute()}
}

// ParseDayTime accepts "HH:MM" and "HH:MM:SS", the forms Postgres renders a
// `time` column in.
func ParseDayTime(s string) (DayTime, error) {
	var h, m, sec int
	switch n, _ := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec); n {
	case 2, 3:
	default:
		return DayTime{}, Invalid("time", "%q is not a time of day like 06:00.", s)
	}
	dt := DayTime{Hour: h, Minute: m}
	if err := dt.Validate(); err != nil {
		return DayTime{}, err
	}
	return dt, nil
}

func (d DayTime) Validate() error {
	if d.Hour < 0 || d.Hour > 24 {
		return Invalid("time", "Hour must be between 0 and 24.")
	}
	if d.Minute < 0 || d.Minute > 59 {
		return Invalid("time", "Minute must be between 0 and 59.")
	}
	if d.Hour == 24 && d.Minute != 0 {
		return Invalid("time", "24:00 is the latest possible time of day.")
	}
	return nil
}

func (d DayTime) Minutes() int          { return d.Hour*60 + d.Minute }
func (d DayTime) Before(o DayTime) bool { return d.Minutes() < o.Minutes() }
func (d DayTime) After(o DayTime) bool  { return d.Minutes() > o.Minutes() }
func (d DayTime) Equal(o DayTime) bool  { return d.Minutes() == o.Minutes() }
func (d DayTime) String() string        { return fmt.Sprintf("%02d:%02d", d.Hour, d.Minute) }

// OnDate resolves this time of day against a calendar date in a location,
// yielding the absolute instant the arena means.
func (d DayTime) OnDate(year int, month time.Month, day int, loc *time.Location) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, loc).
		Add(time.Duration(d.Minutes()) * time.Minute)
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d == time.Hour:
		return "1 hour"
	case d%time.Hour == 0:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	default:
		return d.String()
	}
}
