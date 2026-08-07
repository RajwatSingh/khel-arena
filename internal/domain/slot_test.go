package domain

import (
	"errors"
	"testing"
	"time"
)

// ktm is the zone every arena publishes its hours in. Kathmandu sits at
// UTC+05:45, an offset that is not a whole number of hours -- which is
// exactly why nothing in this package reasons about local time by adding a
// fixed number of hours to UTC.
var ktm = mustLoadKathmandu()

func mustLoadKathmandu() *time.Location {
	loc, err := time.LoadLocation("Asia/Kathmandu")
	if err != nil {
		panic("Asia/Kathmandu unavailable: " + err.Error())
	}
	return loc
}

// at builds an instant from Kathmandu wall-clock time, the way an arena
// owner would describe it.
func at(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, ktm)
}

func TestNewSlotRejectsInvalidRanges(t *testing.T) {
	base := at(2030, time.March, 15, 18, 0)

	tests := []struct {
		name       string
		start, end time.Time
		wantField  string
	}{
		{"end before start", base, base.Add(-time.Hour), "slot"},
		{"zero length", base, base, "slot"},
		{"under the minimum", base, base.Add(29 * time.Minute), "slot"},
		{"over the maximum", base, base.Add(4*time.Hour + time.Minute), "slot"},
		{"missing start", time.Time{}, base, "slot"},
		{"missing end", base, time.Time{}, "slot"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSlot(tc.start, tc.end)
			if err == nil {
				t.Fatal("expected the slot to be rejected, but it was accepted")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("code = %q, want %q", CodeOf(err), CodeInvalid)
			}
			var de *Error
			if errors.As(err, &de) && de.Field != tc.wantField {
				t.Errorf("field = %q, want %q", de.Field, tc.wantField)
			}
		})
	}
}

func TestNewSlotAcceptsBoundaryDurations(t *testing.T) {
	base := at(2030, time.March, 15, 18, 0)

	for _, d := range []time.Duration{MinBookingDuration, time.Hour, MaxBookingDuration} {
		if _, err := NewSlot(base, base.Add(d)); err != nil {
			t.Errorf("duration %s should be allowed: %v", d, err)
		}
	}
}

func TestNewSlotNormalisesToUTC(t *testing.T) {
	start := at(2030, time.March, 15, 18, 0)
	s, err := NewSlot(start, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Start.Location() != time.UTC || s.End.Location() != time.UTC {
		t.Errorf("slot retained a non-UTC zone: %s", s)
	}
	if !s.Start.Equal(start) {
		t.Errorf("normalisation moved the instant: got %s, want %s", s.Start, start)
	}
}

// Half-open ranges are what make back-to-back bookings legal. This is the
// same convention the database's tstzrange uses; if the two ever disagree,
// adjacent slots start colliding.
func TestSlotOverlapIsHalfOpen(t *testing.T) {
	slot := func(fromHour, toHour int) Slot {
		s, err := NewSlot(at(2030, time.March, 15, fromHour, 0), at(2030, time.March, 15, toHour, 0))
		if err != nil {
			t.Fatalf("building slot %d-%d: %v", fromHour, toHour, err)
		}
		return s
	}

	tests := []struct {
		name string
		a, b Slot
		want bool
	}{
		{"identical", slot(18, 19), slot(18, 19), true},
		{"partial, later start", slot(18, 20), slot(19, 21), true},
		{"partial, earlier start", slot(19, 21), slot(18, 20), true},
		{"fully contained", slot(18, 22), slot(19, 20), true},
		{"adjacent after", slot(18, 19), slot(19, 20), false},
		{"adjacent before", slot(19, 20), slot(18, 19), false},
		{"disjoint", slot(18, 19), slot(21, 22), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Overlaps(tc.b); got != tc.want {
				t.Errorf("%s overlaps %s = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			// Overlap is symmetric; a rule that only holds one way would
			// let ordering decide whether a double-booking is caught.
			if got := tc.b.Overlaps(tc.a); got != tc.want {
				t.Errorf("not symmetric: %s overlaps %s = %v, want %v", tc.b, tc.a, got, tc.want)
			}
		})
	}
}

// Pricing rules are quoted per hour, so a part-hour is billed as a whole one.
func TestSlotHoursRoundsUp(t *testing.T) {
	base := at(2030, time.March, 15, 18, 0)
	tests := []struct {
		d    time.Duration
		want int
	}{
		{30 * time.Minute, 1},
		{time.Hour, 1},
		{90 * time.Minute, 2},
		{2 * time.Hour, 2},
		{150 * time.Minute, 3},
		{4 * time.Hour, 4},
	}
	for _, tc := range tests {
		s, err := NewSlot(base, base.Add(tc.d))
		if err != nil {
			t.Fatalf("duration %s: %v", tc.d, err)
		}
		if got := s.Hours(); got != tc.want {
			t.Errorf("%s billed as %d hours, want %d", tc.d, got, tc.want)
		}
	}
}

func TestSlotIsPastAllowsInFlightGrace(t *testing.T) {
	now := at(2030, time.March, 15, 18, 0)
	slotAt := func(offset time.Duration) Slot {
		s, err := NewSlot(now.Add(offset), now.Add(offset+time.Hour))
		if err != nil {
			t.Fatalf("offset %s: %v", offset, err)
		}
		return s
	}

	tests := []struct {
		name   string
		offset time.Duration
		want   bool
	}{
		{"well in the future", time.Hour, false},
		{"starting now", 0, false},
		{"just slipped past, within grace", -30 * time.Second, false},
		{"past the grace window", -2 * time.Minute, true},
		{"yesterday", -24 * time.Hour, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := slotAt(tc.offset).IsPast(now); got != tc.want {
				t.Errorf("IsPast = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSlotWithinOperatingHours(t *testing.T) {
	opens := DayTime{Hour: 6}
	closes := DayTime{Hour: 22}

	mk := func(fromHour, fromMin, toHour, toMin int) Slot {
		return Slot{
			Start: at(2030, time.March, 15, fromHour, fromMin).UTC(),
			End:   at(2030, time.March, 15, toHour, toMin).UTC(),
		}
	}

	tests := []struct {
		name string
		slot Slot
		want bool
	}{
		{"mid-afternoon", mk(14, 0, 15, 0), true},
		{"exactly at opening", mk(6, 0, 7, 0), true},
		{"ending exactly at closing", mk(21, 0, 22, 0), true},
		{"starts before opening", mk(5, 0, 6, 0), false},
		{"runs past closing", mk(21, 30, 22, 30), false},
		{"entirely outside", mk(2, 0, 3, 0), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.slot.WithinOperatingHours(opens, closes, ktm); got != tc.want {
				t.Errorf("WithinOperatingHours = %v, want %v", got, tc.want)
			}
		})
	}
}

// A slot is stored in UTC but judged against wall-clock opening hours. At
// UTC+05:45 an 06:00 Kathmandu start is 00:15 UTC the same day, and a 21:00
// start is 15:15 UTC -- so a naive UTC comparison would reject both.
func TestOperatingHoursUseLocalWallClock(t *testing.T) {
	opens, closes := DayTime{Hour: 6}, DayTime{Hour: 22}

	earliest := Slot{
		Start: at(2030, time.March, 15, 6, 0).UTC(),
		End:   at(2030, time.March, 15, 7, 0).UTC(),
	}
	if earliest.Start.In(time.UTC).Hour() != 0 {
		t.Fatalf("precondition: expected 06:00 Kathmandu to be 00:15 UTC, got %s", earliest.Start)
	}
	if !earliest.WithinOperatingHours(opens, closes, ktm) {
		t.Error("the first slot of the day was rejected; hours are being compared in the wrong zone")
	}

	// The same instant, judged in UTC, falls before a 06:00 opening.
	if earliest.WithinOperatingHours(opens, closes, time.UTC) {
		t.Error("expected the UTC reading to reject this slot, confirming the zone matters")
	}
}

func TestParseDayTime(t *testing.T) {
	tests := []struct {
		in      string
		want    DayTime
		wantErr bool
	}{
		{"06:00", DayTime{Hour: 6}, false},
		{"06:00:00", DayTime{Hour: 6}, false},
		{"22:30", DayTime{Hour: 22, Minute: 30}, false},
		{"24:00", DayTime{Hour: 24}, false},
		{"00:00", DayTime{}, false},
		{"25:00", DayTime{}, true},
		{"24:30", DayTime{}, true},
		{"06:60", DayTime{}, true},
		{"nonsense", DayTime{}, true},
		{"", DayTime{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseDayTime(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected %q to be rejected, got %v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDayTimeOrdering(t *testing.T) {
	six := DayTime{Hour: 6}
	ten := DayTime{Hour: 22}

	if !six.Before(ten) {
		t.Error("06:00 should sort before 22:00")
	}
	if !ten.After(six) {
		t.Error("22:00 should sort after 06:00")
	}
	if !six.Equal(DayTime{Hour: 6, Minute: 0}) {
		t.Error("equal times should compare equal")
	}
}
