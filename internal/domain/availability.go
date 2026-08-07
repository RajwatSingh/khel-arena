package domain

import (
	"time"
)

// GridRequest describes the availability matrix to project: one court, one
// calendar date, read in the arena's local zone.
type GridRequest struct {
	// Date is a calendar date in the arena's zone. Only the year, month and
	// day are read.
	Date     time.Time
	OpensAt  DayTime
	ClosesAt DayTime
	Location *time.Location

	BasePriceNPR int
	Rules        []PricingRule

	// Booked are the ranges already taken on this court. They need not be
	// aligned to the hour: a 90-minute booking blocks two grid cells.
	Booked []Slot

	// Now is the instant "past" is judged against, passed in rather than read
	// from the clock so the projection is a pure function of its inputs.
	Now time.Time
}

// BuildGrid projects the bookable hours of one court on one date.
//
// This is the TimeSlots entity, computed rather than stored. Materialising a
// row for every hour of every court forever would create millions of rows
// describing time, which passes whether or not it is written down. The grid
// is derived from operating hours, pricing rules and live bookings, so it is
// always current and there is nothing to keep in sync.
//
// The projection walks the arena's opening hours in one-hour steps. An arena
// open 06:00-22:00 yields sixteen cells; the last starts at 21:00 and ends at
// closing, so no cell ever runs past the arena's hours.
func BuildGrid(req GridRequest) []GridSlot {
	loc := req.Location
	if loc == nil {
		loc = time.UTC
	}

	year, month, day := req.Date.In(loc).Date()

	// Walk on whole hours from opening. A fractional opening time (05:30)
	// keeps its offset, so cells stay aligned to when the arena actually opens.
	openMinutes := req.OpensAt.Minutes()
	closeMinutes := req.ClosesAt.Minutes()
	if closeMinutes <= openMinutes {
		return nil
	}

	midnight := time.Date(year, month, day, 0, 0, 0, 0, loc)
	capacity := (closeMinutes - openMinutes) / 60
	if capacity < 0 {
		capacity = 0
	}
	grid := make([]GridSlot, 0, capacity)

	for offset := openMinutes; offset+60 <= closeMinutes; offset += 60 {
		start := midnight.Add(time.Duration(offset) * time.Minute)
		slot := Slot{
			Start: start.UTC(),
			End:   start.Add(SlotGranularity).UTC(),
		}

		price := ResolvePrice(req.BasePriceNPR, req.Rules, slot, loc)

		grid = append(grid, GridSlot{
			Slot:     slot,
			PriceNPR: price.TotalNPR,
			IsPeak:   price.IsPeak,
			IsBooked: overlapsAny(slot, req.Booked),
			IsPast:   slot.Start.Before(req.Now),
		})
	}

	return grid
}

func overlapsAny(slot Slot, taken []Slot) bool {
	for _, t := range taken {
		if slot.Overlaps(t) {
			return true
		}
	}
	return false
}

// GridWindow returns the absolute time range a date's grid spans, for querying
// which bookings could possibly intersect it.
//
// The window covers the whole local day rather than only the opening hours,
// so a booking that starts before opening -- one made when the arena kept
// different hours -- is still seen and still blocks its cells.
func GridWindow(date time.Time, loc *time.Location) Slot {
	if loc == nil {
		loc = time.UTC
	}
	year, month, day := date.In(loc).Date()
	midnight := time.Date(year, month, day, 0, 0, 0, 0, loc)

	return Slot{
		Start: midnight.UTC(),
		// AddDate rather than adding 24 hours: a day is not always 24 hours
		// long, and this must land on the next local midnight.
		End: midnight.AddDate(0, 0, 1).UTC(),
	}
}
