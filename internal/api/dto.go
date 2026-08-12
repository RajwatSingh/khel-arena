package api

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type bookingInput struct {
	CourtID  uuid.UUID  `json:"court_id"`
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   time.Time  `json:"ends_at"`
	TeamID   *uuid.UUID `json:"team_id"`
	Note     string     `json:"note"`
}

type gridSlotDTO struct {
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	PriceNPR  int       `json:"price_npr"`
	IsPeak    bool      `json:"is_peak"`
	IsBooked  bool      `json:"is_booked"`
	IsPast    bool      `json:"is_past"`
	Available bool      `json:"available"`
}

type userDTO struct {
	Id            uuid.UUID          `json:"id"`
	FullName      string             `json:"full_name"`
	UserName      string             `json:"username"`
	Email         string             `json:"email"`
	AccountType   domain.AccountType `json:"account_type"`
	Skill         domain.SkillTier   `json:"skill"`
	Position      *domain.Position   `json:"position"`
	JerseyNumber  *int               `json:"jersey_number"`
	PreferredFoot *domain.Foot       `json:"preferred_foot"`
}

type sessionDTO struct {
	User        userDTO `json:"user"`
	AccessToken string  `json:"access_token"`
}

type bookingDTO struct {
	ID            uuid.UUID            `json:"id"`
	Reference     string               `json:"reference"`
	CourtID       uuid.UUID            `json:"court_id"`
	StartsAt      time.Time            `json:"starts_at"`
	EndsAt        time.Time            `json:"ends_at"`
	PriceNPR      int                  `json:"price_npr"`
	Status        domain.BookingStatus `json:"status"`
	Note          string               `json:"note"`
	HoldExpiresAt *time.Time           `json:"hold_expires_at"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

type bookingDetailDTO struct {
	bookingDTO
	CourtName string    `json:"court_name"`
	ArenaID   uuid.UUID `json:"arena_id"`
	ArenaName string    `json:"arena_name"`
	ArenaSlug string    `json:"arena_slug"`
	ArenaArea string    `json:"arena_area"`
}

func gridSlotDTOFromDomain(slot domain.GridSlot) gridSlotDTO {
	return gridSlotDTO{
		StartsAt:  slot.Slot.Start,
		EndsAt:    slot.Slot.End,
		PriceNPR:  slot.PriceNPR,
		IsPeak:    slot.IsPeak,
		IsBooked:  slot.IsBooked,
		IsPast:    slot.IsPast,
		Available: slot.Available(),
	}
}

func userDTOFromDomain(user domain.User) userDTO {
	return userDTO{
		Id:            user.ID,
		FullName:      user.FullName,
		UserName:      user.Username,
		Email:         user.Email,
		AccountType:   user.AccountType,
		Skill:         user.Skill,
		Position:      user.Position,
		JerseyNumber:  user.JerseyNumber,
		PreferredFoot: user.PreferredFoot,
	}
}

func sessionDTOFromDomain(s service.Session) sessionDTO {
	return sessionDTO{
		User:        userDTOFromDomain(s.User),
		AccessToken: s.AccessToken,
	}
}

func bookingDTOFromDomain(b domain.Booking) bookingDTO {
	return bookingDTO{
		ID:            b.ID,
		Reference:     bookingReference(b.ID),
		CourtID:       b.CourtID,
		StartsAt:      b.Slot.Start,
		EndsAt:        b.Slot.End,
		PriceNPR:      b.PriceNPR,
		Status:        b.Status,
		Note:          b.Note,
		HoldExpiresAt: b.HoldExpiresAt,
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
	}
}

func bookingDetailDTOFromDomain(b domain.BookingDetail) bookingDetailDTO {
	return bookingDetailDTO{
		bookingDTO: bookingDTOFromDomain(b.Booking),
		CourtName:  b.CourtLabel,
		ArenaID:    b.ArenaID,
		ArenaName:  b.ArenaName,
		ArenaSlug:  b.ArenaSlug,
		ArenaArea:  b.ArenaArea,
	}
}

func bookingReference(id uuid.UUID) string {
	return "KA-" + strings.ToUpper(hex.EncodeToString(id[:4]))
}
