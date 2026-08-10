package api

import (
	"time"
	"strings"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

type gridSlotDTO struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt time.Time `json:"ends_at"`
	PriceNPR int `json:"price_npr"`
	IsPeak bool `json:"is_peak"`
	IsBooked bool `json:"is_booked"`
	IsPast bool `json:"is_past"`
	Available bool `json:"available"`
}

type userDTO struct {
	Id uuid.UUID `json:"id"`
	FullName string `json:"full_name"`
	UserName string `json:"username"`
	Email string `json:"email"`
	AccountType domain.AccountType `json:"account_type"`
	Skill domain.SkillTier `json:"skill"`
	Position *domain.Position `json:"position"`
	JerseyNumber *int `json:"jersey_number"`
	PreferredFoot *domain.Foot `json:"preferred_foot"`
}

type sessionDTO struct {
	User userDTO `json:"user"`
	AccessToken string `json:"access_token"`
}

type bookingDTO struct {
	ID uuid.UUID `json:"id"`
	Reference string `json:"reference"`
	CourtID uuid.UUID `json:"court_id"`
	CourtName string `json:"court_name"`

}

func gridSlotDTOFromDomain(slot domain.GridSlot) gridSlotDTO {
	return gridSlotDTO{
		StartsAt: slot.Slot.Start,
		EndsAt: slot.Slot.End,
		PriceNPR: slot.PriceNPR,
		IsPeak: slot.IsPeak,
		IsBooked: slot.IsBooked,
		IsPast: slot.IsPast,
		Available: slot.Available(),
	}
}

func userDTOFromDomain(user domain.User) userDTO {
	return userDTO{
		Id: user.ID, 
		FullName: user.FullName,
		UserName: user.Username,
		Email: user.Email,
		AccountType: user.AccountType,
		Skill: user.Skill,
		Position: user.Position,
		JerseyNumber: user.JerseyNumber,
		PreferredFoot: user.PreferredFoot,
	}
}

func sessionDTOFromDomain(s service.Session) sessionDTO {
	return sessionDTO{
		User: userDTOFromDomain(s.User),
		AccessToken: s.AccessToken,
	}
}

func bookingReference(id uuid.UUID) string {
    return "KA-" + strings.ToUpper(hex.EncodeToString(id[:4]))
}
