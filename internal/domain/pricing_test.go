package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func rule(label string, days []time.Weekday, startHour, endHour, price, priority int, peak bool) PricingRule {
	return PricingRule{
		ID:        uuid.New(),
		Label:     label,
		Days:      days,
		StartHour: startHour,
		EndHour:   endHour,
		PriceNPR:  price,
		IsPeak:    peak,
		Priority:  priority,
	}
}

var (
	weekdays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	weekend  = []time.Weekday{time.Saturday, time.Sunday}
	everyday = []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday, time.Thursday,
		time.Friday, time.Saturday, time.Sunday,
	}
)

func hourSlot(t *testing.T, y int, m time.Month, d, hour int) Slot {
	t.Helper()
	s, err := NewSlot(at(y, m, d, hour, 0), at(y, m, d, hour+1, 0))
	if err != nil {
		t.Fatalf("building slot: %v", err)
	}
	return s
}

func TestResolvePriceFallsBackToBase(t *testing.T) {
	// 2030-03-15 is a Friday.
	slot := hourSlot(t, 2030, time.March, 15, 10)

	got := ResolvePrice(1200, nil, slot, ktm)

	if got.PerHourNPR != 1200 {
		t.Errorf("rate = %d, want the court's base of 1200", got.PerHourNPR)
	}
	if got.TotalNPR != 1200 {
		t.Errorf("total = %d, want 1200 for one hour", got.TotalNPR)
	}
	if got.IsPeak {
		t.Error("the base rate is never a peak rate")
	}
	if got.RuleID != nil {
		t.Error("no rule applied, so RuleID should be nil")
	}
}

func TestResolvePriceAppliesMatchingRule(t *testing.T) {
	evening := rule("Evening Peak", everyday, 17, 21, 2000, 1, true)
	slot := hourSlot(t, 2030, time.March, 15, 18)

	got := ResolvePrice(1200, []PricingRule{evening}, slot, ktm)

	if got.PerHourNPR != 2000 {
		t.Errorf("rate = %d, want the evening rate of 2000", got.PerHourNPR)
	}
	if !got.IsPeak {
		t.Error("the evening rule is a peak rule; IsPeak should be true")
	}
	if got.RuleID == nil || *got.RuleID != evening.ID {
		t.Error("the winning rule should be reported back")
	}
}

func TestResolvePriceIgnoresRulesForOtherDays(t *testing.T) {
	weekendRule := rule("Weekend", weekend, 0, 24, 2500, 1, true)
	// 2030-03-15 is a Friday, so the weekend rule must not apply.
	slot := hourSlot(t, 2030, time.March, 15, 18)

	got := ResolvePrice(1200, []PricingRule{weekendRule}, slot, ktm)

	if got.PerHourNPR != 1200 {
		t.Errorf("rate = %d, want the base rate: a Friday is not the weekend", got.PerHourNPR)
	}
}

func TestResolvePriceWindowIsHalfOpen(t *testing.T) {
	evening := rule("Evening Peak", everyday, 17, 21, 2000, 1, true)

	tests := []struct {
		name string
		hour int
		want int
	}{
		{"the hour before the window", 16, 1200},
		{"the first hour of the window", 17, 2000},
		{"inside the window", 19, 2000},
		{"the last hour of the window", 20, 2000},
		{"the hour the window ends", 21, 1200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			slot := hourSlot(t, 2030, time.March, 15, tc.hour)
			got := ResolvePrice(1200, []PricingRule{evening}, slot, ktm)
			if got.PerHourNPR != tc.want {
				t.Errorf("%02d:00 priced at %d, want %d", tc.hour, got.PerHourNPR, tc.want)
			}
		})
	}
}

func TestResolvePriceHighestPriorityWins(t *testing.T) {
	broad := rule("All Day", everyday, 0, 24, 1500, 1, false)
	specific := rule("Friday Night", []time.Weekday{time.Friday}, 18, 22, 3000, 10, true)
	slot := hourSlot(t, 2030, time.March, 15, 19) // Friday 19:00

	// Order must not matter: the winner is decided by priority, not by which
	// row the database happened to return first.
	for _, rules := range [][]PricingRule{
		{broad, specific},
		{specific, broad},
	} {
		got := ResolvePrice(1200, rules, slot, ktm)
		if got.PerHourNPR != 3000 {
			t.Errorf("rate = %d, want the higher-priority 3000", got.PerHourNPR)
		}
		if got.RuleID == nil || *got.RuleID != specific.ID {
			t.Error("the higher-priority rule should have won")
		}
	}
}

// Two rules at the same priority must not resolve by row order, or the price
// of a slot could change between two identical requests.
func TestResolvePriceTiesBreakDeterministically(t *testing.T) {
	broad := rule("Weekday Evening", weekdays, 16, 22, 1800, 5, false)
	narrow := rule("Friday Prime", []time.Weekday{time.Friday}, 19, 20, 2200, 5, true)
	slot := hourSlot(t, 2030, time.March, 15, 19)

	for _, rules := range [][]PricingRule{
		{broad, narrow},
		{narrow, broad},
	} {
		got := ResolvePrice(1200, rules, slot, ktm)
		if got.PerHourNPR != 2200 {
			t.Errorf("rate = %d, want 2200: the narrower window should win a tie", got.PerHourNPR)
		}
	}
}

// The rate is chosen by the slot's start hour and applies for the whole
// booking, which is how arenas actually quote a court.
func TestResolvePriceScalesByDurationAtTheStartRate(t *testing.T) {
	evening := rule("Evening Peak", everyday, 17, 21, 2000, 1, true)

	// 18:00-21:00 starts inside the peak window and runs one hour past it.
	slot, err := NewSlot(at(2030, time.March, 15, 18, 0), at(2030, time.March, 15, 21, 0))
	if err != nil {
		t.Fatalf("building slot: %v", err)
	}

	got := ResolvePrice(1200, []PricingRule{evening}, slot, ktm)

	if got.PerHourNPR != 2000 {
		t.Errorf("rate = %d, want the 2000 rate in force at kick-off", got.PerHourNPR)
	}
	if got.TotalNPR != 6000 {
		t.Errorf("total = %d, want 6000 (2000 x 3 hours)", got.TotalNPR)
	}
}

func TestResolvePriceBillsPartHoursAsWholeOnes(t *testing.T) {
	slot, err := NewSlot(at(2030, time.March, 15, 10, 0), at(2030, time.March, 15, 11, 30))
	if err != nil {
		t.Fatalf("building slot: %v", err)
	}

	got := ResolvePrice(1000, nil, slot, ktm)

	if got.TotalNPR != 2000 {
		t.Errorf("total = %d, want 2000: 90 minutes bills as two hours", got.TotalNPR)
	}
}

// Rules are written in the arena's wall-clock time. At UTC+05:45, Friday
// 19:00 in Kathmandu is Friday 13:15 UTC -- and near midnight the two zones
// disagree about the day of the week entirely.
func TestPricingRulesEvaluateInArenaLocalTime(t *testing.T) {
	saturdayRule := rule("Saturday Premium", []time.Weekday{time.Saturday}, 0, 24, 2500, 1, true)

	// 2030-03-16 00:30 Kathmandu is a Saturday, but 18:45 UTC on Friday.
	slot := hourSlot(t, 2030, time.March, 16, 0)
	if slot.Start.In(time.UTC).Weekday() != time.Friday {
		t.Fatalf("precondition: expected this instant to be Friday in UTC, got %s",
			slot.Start.In(time.UTC).Weekday())
	}

	got := ResolvePrice(1200, []PricingRule{saturdayRule}, slot, ktm)
	if got.PerHourNPR != 2500 {
		t.Errorf("rate = %d, want 2500: it is Saturday where the arena is", got.PerHourNPR)
	}

	// Read in UTC the same instant is a Friday, so the rule must not apply.
	if utc := ResolvePrice(1200, []PricingRule{saturdayRule}, slot, time.UTC); utc.PerHourNPR != 1200 {
		t.Errorf("rate in UTC = %d, want the base 1200, confirming the zone decides", utc.PerHourNPR)
	}
}

func TestISOWeekdayRoundTrip(t *testing.T) {
	// Postgres counts Monday as 1 and Sunday as 7; Go counts Sunday as 0.
	// A slip here shifts every pricing rule by a day.
	want := map[time.Weekday]int{
		time.Monday: 1, time.Tuesday: 2, time.Wednesday: 3, time.Thursday: 4,
		time.Friday: 5, time.Saturday: 6, time.Sunday: 7,
	}

	for day, iso := range want {
		if got := ISOWeekday(day); got != iso {
			t.Errorf("ISOWeekday(%s) = %d, want %d", day, got, iso)
		}
		back, err := WeekdayFromISO(iso)
		if err != nil {
			t.Fatalf("WeekdayFromISO(%d): %v", iso, err)
		}
		if back != day {
			t.Errorf("WeekdayFromISO(%d) = %s, want %s", iso, back, day)
		}
	}

	for _, bad := range []int{0, 8, -1} {
		if _, err := WeekdayFromISO(bad); err == nil {
			t.Errorf("WeekdayFromISO(%d) should have been rejected", bad)
		}
	}
}

func TestPricingRuleValidation(t *testing.T) {
	valid := rule("Evening", everyday, 17, 21, 2000, 1, true)
	if err := valid.Validate(); err != nil {
		t.Fatalf("a well-formed rule was rejected: %v", err)
	}

	tests := []struct {
		name string
		mut  func(*PricingRule)
	}{
		{"no label", func(r *PricingRule) { r.Label = "" }},
		{"no days", func(r *PricingRule) { r.Days = nil }},
		{"end before start", func(r *PricingRule) { r.StartHour, r.EndHour = 20, 18 }},
		{"end equal to start", func(r *PricingRule) { r.StartHour, r.EndHour = 18, 18 }},
		{"start hour out of range", func(r *PricingRule) { r.StartHour = 24 }},
		{"end hour out of range", func(r *PricingRule) { r.EndHour = 25 }},
		{"zero price", func(r *PricingRule) { r.PriceNPR = 0 }},
		{"negative price", func(r *PricingRule) { r.PriceNPR = -100 }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := valid
			tc.mut(&r)
			if err := r.Validate(); err == nil {
				t.Error("expected the rule to be rejected")
			}
		})
	}
}
