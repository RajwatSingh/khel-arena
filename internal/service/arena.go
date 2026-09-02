package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
)

// ArenaStore is the venue storage this service needs. Reads only: owner-facing
// writes are separate work with an authorization story of their own, and this
// interface should not quietly acquire the ability to do them.
type ArenaStore interface {
	List(ctx context.Context) ([]postgres.ArenaListing, error)
	BySlug(ctx context.Context, slug string) (postgres.ArenaDetail, error)
	ListAreas(ctx context.Context) ([]string, error)
	LedgerCourts(ctx context.Context, sport domain.Sport, area string) ([]postgres.LedgerCourt, error)
}

// LedgerBookings supplies what is already taken across many courts at once.
//
// Declared separately from ArenaStore because it is the booking repository
// that answers it: the ledger is the one read that spans both, and naming the
// two halves apart keeps either from growing methods that belong to the other.
type LedgerBookings interface {
	BookedRangesForCourts(ctx context.Context, courtIDs []uuid.UUID, window domain.Slot) (map[uuid.UUID][]domain.Slot, error)
}

// ArenaService answers the browsing questions: what venues exist, what one of
// them looks like, and what the whole city has free on a given day.
type ArenaService struct {
	arenas   ArenaStore
	bookings LedgerBookings
	clock    Clock
	loc      *time.Location
}

func NewArenaService(arenas ArenaStore, bookings LedgerBookings, clock Clock, loc *time.Location) *ArenaService {
	if clock == nil {
		clock = SystemClock{}
	}
	if loc == nil {
		loc = time.UTC
	}
	return &ArenaService{arenas: arenas, bookings: bookings, clock: clock, loc: loc}
}

func (s *ArenaService) List(ctx context.Context) ([]postgres.ArenaListing, error) {
	return s.arenas.List(ctx)
}

func (s *ArenaService) BySlug(ctx context.Context, slug string) (postgres.ArenaDetail, error) {
	if slug == "" {
		return postgres.ArenaDetail{}, domain.Invalid("slug", "Which arena?")
	}
	return s.arenas.BySlug(ctx, slug)
}

func (s *ArenaService) ListAreas(ctx context.Context) ([]string, error) {
	return s.arenas.ListAreas(ctx)
}

// LedgerRow is one court's day: the court, its venue, and the projected grid.
type LedgerRow struct {
	postgres.LedgerCourt
	Slots []domain.GridSlot
}

// Ledger is the city-wide view of one date.
type Ledger struct {
	Date      time.Time
	Rows      []LedgerRow
	OpenHours int
	// CheapestNPR is the lowest price among slots that can actually be
	// booked, or nil when nothing is free. A "from" price quoting an hour
	// nobody can take is worse than no price at all.
	CheapestNPR *int
}

// CityLedger projects every matching court's day in one pass.
//
// The shape of this is the whole point. A naive version asks the database
// which hours are taken once per court, so a city with forty courts costs
// forty queries to render one page. Here the courts and their rules come back
// in one batch, the taken ranges for all of them come back in one more, and
// the projection itself is pure Go over data already in hand -- two queries,
// whatever the size of the city.
//
// An empty sport or area means "everything", which is what the filter's
// default position means.
func (s *ArenaService) CityLedger(ctx context.Context, date time.Time, sport domain.Sport, area string) (Ledger, error) {
	if sport != "" && !sport.Valid() {
		return Ledger{}, domain.Invalid("sport", "We don't list that sport.")
	}

	courts, err := s.arenas.LedgerCourts(ctx, sport, area)
	if err != nil {
		return Ledger{}, err
	}
	if len(courts) == 0 {
		return Ledger{Date: date, Rows: []LedgerRow{}}, nil
	}

	ids := make([]uuid.UUID, 0, len(courts))
	for _, c := range courts {
		ids = append(ids, c.ID)
	}

	window := domain.GridWindow(date, s.loc)
	booked, err := s.bookings.BookedRangesForCourts(ctx, ids, window)
	if err != nil {
		return Ledger{}, err
	}

	now := s.clock.Now()
	rows := make([]LedgerRow, 0, len(courts))
	openHours := 0
	var cheapest *int

	for _, court := range courts {
		slots := domain.BuildGrid(domain.GridRequest{
			Date:         date,
			OpensAt:      court.OpensAt,
			ClosesAt:     court.ClosesAt,
			Location:     s.loc,
			BasePriceNPR: court.BasePriceNPR,
			Rules:        court.PricingRules,
			Booked:       booked[court.ID],
			Now:          now,
		})

		for _, slot := range slots {
			if !slot.Available() {
				continue
			}
			openHours++
			if cheapest == nil || slot.PriceNPR < *cheapest {
				price := slot.PriceNPR
				cheapest = &price
			}
		}

		rows = append(rows, LedgerRow{LedgerCourt: court, Slots: slots})
	}

	return Ledger{Date: date, Rows: rows, OpenHours: openHours, CheapestNPR: cheapest}, nil
}
