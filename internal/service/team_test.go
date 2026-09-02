package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// The interesting cases here are all refusals, and all of them are the
// domain's own predicates being reached: the service must not restate
// `CanRemoveMember` in its own words, because a second opinion drifts.

type stubTeams struct {
	team    domain.Team
	roster  []domain.Member
	byCode  map[string]domain.Team
	member  bool
	added   uuid.UUID
	removed uuid.UUID
	from    uuid.UUID
	to      uuid.UUID
	writes  int
}

func (s *stubTeams) Create(_ context.Context, t domain.Team) (domain.Team, error) {
	s.writes++
	t.ID = uuid.New()
	s.team = t
	return t, nil
}

func (s *stubTeams) ByID(context.Context, uuid.UUID) (domain.Team, error) { return s.team, nil }

func (s *stubTeams) ByJoinCode(_ context.Context, code string) (domain.Team, error) {
	t, ok := s.byCode[code]
	if !ok {
		return domain.Team{}, domain.NotFound("That invite code doesn't match a team.")
	}
	return t, nil
}

func (s *stubTeams) ListForUser(context.Context, uuid.UUID) ([]domain.Team, error) {
	return []domain.Team{s.team}, nil
}

func (s *stubTeams) Roster(context.Context, uuid.UUID) ([]domain.Member, error) {
	return s.roster, nil
}

func (s *stubTeams) Update(_ context.Context, _ uuid.UUID, t domain.Team) (domain.Team, error) {
	s.writes++
	return t, nil
}

func (s *stubTeams) AddMember(_ context.Context, _, userID uuid.UUID) error {
	s.writes++
	s.added = userID
	return nil
}

func (s *stubTeams) RemoveMember(_ context.Context, _, userID uuid.UUID) error {
	s.writes++
	s.removed = userID
	return nil
}

func (s *stubTeams) IsMember(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.member, nil
}

func (s *stubTeams) TransferCaptaincy(_ context.Context, _, from, to uuid.UUID) error {
	s.writes++
	s.from, s.to = from, to
	return nil
}

func (s *stubTeams) RotateJoinCode(context.Context, uuid.UUID) (string, error) {
	s.writes++
	return "NEWCODE1", nil
}

func (s *stubTeams) Disband(context.Context, uuid.UUID) error {
	s.writes++
	return nil
}

var (
	captainID = uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000001")
	playerID  = uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000002")
	outsider  = uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000003")
)

func squad(memberCount int) domain.Team {
	return domain.Team{
		ID:          uuid.New(),
		Name:        "Yeti FC",
		Tag:         "YETI",
		CaptainID:   captainID,
		JoinCode:    "ABCD1234",
		MemberCount: memberCount,
	}
}

func TestCreateTeamTakesTheCaptainFromTheCaller(t *testing.T) {
	teams := &stubTeams{}
	svc := NewTeamService(teams)

	_, err := svc.Create(context.Background(), captainID,
		domain.Team{Name: "Yeti FC", Tag: "YETI", CaptainID: outsider})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}

	if teams.team.CaptainID != captainID {
		t.Errorf("captain = %v, want the caller %v -- a body must not be able to name one",
			teams.team.CaptainID, captainID)
	}
}

// The join code is an invite credential. A team page readable by anyone signed
// in must not hand it to a stranger.
func TestGetTeamHidesTheJoinCodeFromNonMembers(t *testing.T) {
	teams := &stubTeams{
		team:   squad(2),
		roster: []domain.Member{{UserID: captainID, Role: domain.RoleCaptain}},
	}
	svc := NewTeamService(teams)

	t.Run("a member sees it", func(t *testing.T) {
		got, err := svc.Get(context.Background(), teams.team.ID, captainID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.JoinCode != "ABCD1234" {
			t.Errorf("join code = %q, want it visible to a member", got.JoinCode)
		}
	})

	t.Run("a stranger does not", func(t *testing.T) {
		got, err := svc.Get(context.Background(), teams.team.ID, outsider)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.JoinCode != "" {
			t.Errorf("join code = %q, want it hidden", got.JoinCode)
		}
		// Also stripped from the embedded team, so a caller reading either
		// field gets the same answer.
		if got.Team.JoinCode != "" {
			t.Errorf("the embedded team still carries the code: %q", got.Team.JoinCode)
		}
	})
}

func TestOnlyTheCaptainCanChangeTheSquad(t *testing.T) {
	teams := &stubTeams{team: squad(3)}
	svc := NewTeamService(teams)

	err := svc.AddMember(context.Background(), teams.team.ID, playerID, outsider)

	if domain.CodeOf(err) != domain.CodeForbidden {
		t.Errorf("code = %q, want forbidden", domain.CodeOf(err))
	}
	if teams.writes != 0 {
		t.Error("a non-captain's change was written")
	}
}

func TestSquadSizeIsCapped(t *testing.T) {
	teams := &stubTeams{team: squad(domain.MaxSquadSize)}
	svc := NewTeamService(teams)

	err := svc.AddMember(context.Background(), teams.team.ID, captainID, outsider)

	if domain.CodeOf(err) != domain.CodeConflict {
		t.Errorf("code = %q, want conflict", domain.CodeOf(err))
	}
	if teams.writes != 0 {
		t.Error("a player was added to a full squad")
	}
}

func TestRemoveMember(t *testing.T) {
	t.Run("a player may leave", func(t *testing.T) {
		teams := &stubTeams{team: squad(3)}
		svc := NewTeamService(teams)

		if err := svc.RemoveMember(context.Background(), teams.team.ID, playerID, playerID); err != nil {
			t.Fatalf("leaving: %v", err)
		}
		if teams.removed != playerID {
			t.Errorf("removed = %v", teams.removed)
		}
	})

	t.Run("a player may not remove somebody else", func(t *testing.T) {
		teams := &stubTeams{team: squad(3)}
		svc := NewTeamService(teams)

		err := svc.RemoveMember(context.Background(), teams.team.ID, playerID, outsider)

		if domain.CodeOf(err) != domain.CodeForbidden {
			t.Errorf("code = %q, want forbidden", domain.CodeOf(err))
		}
	})

	// A captain who left would be a team nobody can manage: the armband has to
	// move first.
	t.Run("the captain cannot leave", func(t *testing.T) {
		teams := &stubTeams{team: squad(3)}
		svc := NewTeamService(teams)

		err := svc.RemoveMember(context.Background(), teams.team.ID, captainID, captainID)

		if domain.CodeOf(err) != domain.CodeConflict {
			t.Errorf("code = %q, want conflict", domain.CodeOf(err))
		}
		if teams.writes != 0 {
			t.Error("the captain was removed")
		}
	})
}

func TestTransferCaptaincy(t *testing.T) {
	t.Run("hands the armband to a member", func(t *testing.T) {
		teams := &stubTeams{team: squad(3), member: true}
		svc := NewTeamService(teams)

		if err := svc.TransferCaptaincy(context.Background(), teams.team.ID, captainID, playerID); err != nil {
			t.Fatalf("transferring: %v", err)
		}
		if teams.from != captainID || teams.to != playerID {
			t.Errorf("transfer = %v -> %v", teams.from, teams.to)
		}
	})

	// A captain who is not on the roster is a team nobody can leave and
	// nobody can manage.
	t.Run("refuses somebody who is not on the roster", func(t *testing.T) {
		teams := &stubTeams{team: squad(3), member: false}
		svc := NewTeamService(teams)

		err := svc.TransferCaptaincy(context.Background(), teams.team.ID, captainID, outsider)

		if domain.CodeOf(err) != domain.CodeInvalid {
			t.Errorf("code = %q, want invalid", domain.CodeOf(err))
		}
		if teams.writes != 0 {
			t.Error("the armband went to a non-member")
		}
	})

	t.Run("only the captain may hand it over", func(t *testing.T) {
		teams := &stubTeams{team: squad(3), member: true}
		svc := NewTeamService(teams)

		err := svc.TransferCaptaincy(context.Background(), teams.team.ID, playerID, outsider)

		if domain.CodeOf(err) != domain.CodeForbidden {
			t.Errorf("code = %q, want forbidden", domain.CodeOf(err))
		}
	})
}

func TestJoinByCode(t *testing.T) {
	full := squad(domain.MaxSquadSize)
	full.JoinCode = "FULLCODE"

	teams := &stubTeams{byCode: map[string]domain.Team{
		"ABCD1234": squad(3),
		"FULLCODE": full,
	}}
	svc := NewTeamService(teams)

	t.Run("joins the team the code belongs to", func(t *testing.T) {
		team, err := svc.Join(context.Background(), outsider, "ABCD1234")
		if err != nil {
			t.Fatalf("joining: %v", err)
		}
		if teams.added != outsider {
			t.Errorf("added = %v", teams.added)
		}
		// Not handed back on the way out either: the joiner is a member now,
		// but this response is about the team they joined, not their invite.
		if team.JoinCode != "" {
			t.Errorf("join code = %q, want it stripped", team.JoinCode)
		}
	})

	// Codes are read aloud and typed by hand.
	t.Run("is forgiving about how a code is typed", func(t *testing.T) {
		teams.added = uuid.Nil
		if _, err := svc.Join(context.Background(), outsider, " abcd-1234 "); err != nil {
			t.Fatalf("joining with a hand-typed code: %v", err)
		}
		if teams.added != outsider {
			t.Error("a normalised code did not resolve")
		}
	})

	t.Run("a full squad refuses", func(t *testing.T) {
		if _, err := svc.Join(context.Background(), outsider, "FULLCODE"); domain.CodeOf(err) != domain.CodeConflict {
			t.Errorf("code = %q, want conflict", domain.CodeOf(err))
		}
	})

	t.Run("an unknown code is not found", func(t *testing.T) {
		if _, err := svc.Join(context.Background(), outsider, "ZZZZ9999"); domain.CodeOf(err) != domain.CodeNotFound {
			t.Errorf("code = %q, want not_found", domain.CodeOf(err))
		}
	})

	t.Run("a malformed code never reaches storage", func(t *testing.T) {
		if _, err := svc.Join(context.Background(), outsider, "nope"); domain.CodeOf(err) != domain.CodeInvalid {
			t.Errorf("code = %q, want invalid", domain.CodeOf(err))
		}
	})
}

func TestTeamActionsRequireASession(t *testing.T) {
	teams := &stubTeams{team: squad(3)}
	svc := NewTeamService(teams)
	ctx := context.Background()
	id := teams.team.ID

	cases := map[string]error{
		"create":   firstErr(svc.Create(ctx, uuid.Nil, domain.Team{Name: "X", Tag: "XX"})),
		"my teams": secondErr(svc.MyTeams(ctx, uuid.Nil)),
		"join":     firstErr(svc.Join(ctx, uuid.Nil, "ABCD1234")),
		"add":      svc.AddMember(ctx, id, uuid.Nil, playerID),
		"remove":   svc.RemoveMember(ctx, id, uuid.Nil, playerID),
		"transfer": svc.TransferCaptaincy(ctx, id, uuid.Nil, playerID),
		"disband":  svc.Disband(ctx, id, uuid.Nil),
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if domain.CodeOf(err) != domain.CodeUnauthenticated {
				t.Errorf("code = %q, want unauthenticated", domain.CodeOf(err))
			}
		})
	}
}

// The two helpers exist because Go has no way to say "just the error" of a
// multi-value call inside a composite literal.
func firstErr(_ domain.Team, err error) error    { return err }
func secondErr(_ []domain.Team, err error) error { return err }
