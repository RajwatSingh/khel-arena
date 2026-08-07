package domain

import (
	"time"

	"github.com/google/uuid"
)

// Price is the resolved cost of a slot. It is always computed on the server
// from pricing rules; a price quoted by a client is advisory only, and is
// never trusted when the booking is written.
type Price struct {
	// PerHourNPR is the rate that applied, before duration.
	PerHourNPR int
	// TotalNPR is what the player owes: rate x billable hours.
	TotalNPR int
	IsPeak   bool
	// RuleID identifies the pricing rule that won, or nil if the court's
	// base price applied. Useful when an owner asks why a slot cost what it did.
	RuleID *uuid.UUID
}

// PricingRule sets a rate for a window of hours on given days of the week.
//
// A rule covers the half-open hour range [StartHour, EndHour) on each listed
// ISO weekday (1 = Monday ... 7 = Sunday). Where rules overlap, the highest
// Priority wins; ties break toward the more specific window, so a two-hour
// "Friday Night" rule beats an all-day "Weekend" rule laid over it.
type PricingRule struct {
	ID        uuid.UUID
	CourtID   uuid.UUID
	Label     string
	Days      []time.Weekday
	StartHour int
	EndHour   int
	PriceNPR  int
	IsPeak    bool
	Priority  int
}

func (r PricingRule) Validate() error {
	v := &Validation{}
	v.Check(r.Label != "", "label", "Give the rule a name, like \"Evening Peak\".")
	v.Check(len(r.Days) > 0, "days", "Choose at least one day of the week.")
	v.Check(r.StartHour >= 0 && r.StartHour <= 23, "start_hour", "The start hour must be between 0 and 23.")
	v.Check(r.EndHour >= 1 && r.EndHour <= 24, "end_hour", "The end hour must be between 1 and 24.")
	v.Check(r.EndHour > r.StartHour, "end_hour", "The window must end after it starts.")
	v.Check(r.PriceNPR > 0, "price_npr", "The price must be more than zero.")
	return v.Err()
}

// Covers reports whether this rule applies to an instant, evaluated in the
// arena's local zone: a rule written for "Friday 18:00" means Friday evening
// on the wall clock in Kathmandu, not in UTC.
func (r PricingRule) Covers(t time.Time, loc *time.Location) bool {
	local := t.In(loc)
	if !containsWeekday(r.Days, local.Weekday()) {
		return false
	}
	h := local.Hour()
	return h >= r.StartHour && h < r.EndHour
}

// span is the width of the rule's window, used to break priority ties in
// favour of the more specific rule.
func (r PricingRule) span() int { return r.EndHour - r.StartHour }

// ResolvePrice computes what a slot costs.
//
// The rate is chosen by the slot's *start* hour, then multiplied by the
// billable hours. That is deliberate and matches how arenas quote a booking:
// an 18:00-20:00 booking that begins in the evening peak is charged at the
// peak rate throughout, rather than being split across rate boundaries.
//
// With no matching rule, the court's base price applies.
func ResolvePrice(base int, rules []PricingRule, slot Slot, loc *time.Location) Price {
	best := pickRule(rules, slot.Start, loc)

	perHour, isPeak := base, false
	var ruleID *uuid.UUID
	if best != nil {
		perHour, isPeak = best.PriceNPR, best.IsPeak
		id := best.ID
		ruleID = &id
	}

	return Price{
		PerHourNPR: perHour,
		TotalNPR:   perHour * slot.Hours(),
		IsPeak:     isPeak,
		RuleID:     ruleID,
	}
}

// pickRule returns the winning rule for an instant, or nil for the base rate.
func pickRule(rules []PricingRule, at time.Time, loc *time.Location) *PricingRule {
	var best *PricingRule
	for i := range rules {
		r := &rules[i]
		if !r.Covers(at, loc) {
			continue
		}
		if best == nil || beats(*r, *best) {
			best = r
		}
	}
	return best
}

// beats reports whether candidate should win over incumbent: higher priority
// first, then the narrower window, then the higher price. The last two make
// the outcome deterministic rather than dependent on row order.
func beats(candidate, incumbent PricingRule) bool {
	if candidate.Priority != incumbent.Priority {
		return candidate.Priority > incumbent.Priority
	}
	if candidate.span() != incumbent.span() {
		return candidate.span() < incumbent.span()
	}
	return candidate.PriceNPR > incumbent.PriceNPR
}

func containsWeekday(days []time.Weekday, d time.Weekday) bool {
	for _, x := range days {
		if x == d {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ISO weekday conversion
//
// Postgres stores rule days as ISO numbers (1 = Monday ... 7 = Sunday) while
// Go's time.Weekday counts from Sunday = 0. Getting this wrong shifts every
// pricing rule by a day, so the conversion lives here and nowhere else.
// ---------------------------------------------------------------------------

// ISOWeekday converts a Go weekday to its ISO number.
func ISOWeekday(d time.Weekday) int {
	if d == time.Sunday {
		return 7
	}
	return int(d)
}

// WeekdayFromISO converts an ISO weekday number back to a Go weekday.
func WeekdayFromISO(iso int) (time.Weekday, error) {
	if iso < 1 || iso > 7 {
		return 0, Invalid("days", "%d is not a day of the week (expected 1 for Monday through 7 for Sunday).", iso)
	}
	if iso == 7 {
		return time.Sunday, nil
	}
	return time.Weekday(iso), nil
}
