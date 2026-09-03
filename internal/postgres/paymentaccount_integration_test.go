package postgres

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/crypto"
)

// The per-arena credential store is where "whose money is it" lives, so the
// properties worth a real database are: the owner predicate is in the SQL, the
// stored secret is ciphertext, and the resolve queries walk
// booking → court → arena → account correctly.

func testAccountRepo(t *testing.T, f *fixture) *ArenaPaymentAccountRepo {
	t.Helper()
	key := make([]byte, crypto.KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("key: %v", err)
	}
	box, err := crypto.NewBox(key)
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	return NewArenaPaymentAccountRepo(f.pool, box)
}

func esewaAccount(secret string) domain.ArenaPaymentAccount {
	return domain.ArenaPaymentAccount{
		Provider: domain.ProviderEsewa, SecretKey: secret, MerchantCode: "EPAYTEST",
		Live: false, Enabled: true,
	}
}

func TestPaymentAccountUpsertIsOwnerScopedAndSecretIsSealed(t *testing.T) {
	f := newFixture(t, 0)
	repo := testAccountRepo(t, f)
	ctx := context.Background()

	if err := repo.UpsertOwnerScoped(ctx, f.arenaID, f.owner, esewaAccount("first-secret")); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// A stranger cannot write.
	stranger := seedUser(t, f, "stranger")
	err := repo.UpsertOwnerScoped(ctx, f.arenaID, stranger, esewaAccount("theirs"))
	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Fatalf("stranger upsert: code = %q, want not_found", domain.CodeOf(err))
	}

	// The column holds ciphertext, not the key.
	var stored []byte
	if err := f.pool.QueryRow(ctx,
		`select secret_key from arena_payment_accounts where arena_id = $1 and provider = 'esewa'`,
		f.arenaID).Scan(&stored); err != nil {
		t.Fatalf("reading stored secret: %v", err)
	}
	if string(stored) == "first-secret" {
		t.Fatal("the secret is stored in plaintext")
	}

	// Upsert replaces rather than duplicating.
	if err := repo.UpsertOwnerScoped(ctx, f.arenaID, f.owner, esewaAccount("second-secret")); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	infos, err := repo.ListOwnerScoped(ctx, f.arenaID, f.owner)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("accounts = %d, want 1 after upsert", len(infos))
	}
	if infos[0].SecretHint != domain.HintFor("second-secret") {
		t.Errorf("hint = %q, want the new key's", infos[0].SecretHint)
	}
}

func TestPaymentAccountResolvesThroughBookingAndPayment(t *testing.T) {
	f := newFixture(t, 1)
	repo := testAccountRepo(t, f)
	ctx := context.Background()

	if err := repo.UpsertOwnerScoped(ctx, f.arenaID, f.owner, esewaAccount("live-key")); err != nil {
		t.Fatalf("upsert esewa: %v", err)
	}
	// A disabled Khalti account: resolvable for settlement, absent from the
	// player's options.
	if err := repo.UpsertOwnerScoped(ctx, f.arenaID, f.owner, domain.ArenaPaymentAccount{
		Provider: domain.ProviderKhalti, SecretKey: "k", Enabled: false,
	}); err != nil {
		t.Fatalf("upsert khalti: %v", err)
	}

	bookingID := seedBooking(t, f, f.players[0])
	paymentID := seedPayment(t, f, bookingID, domain.ProviderEsewa)

	enabled, err := repo.EnabledForBooking(ctx, bookingID)
	if err != nil {
		t.Fatalf("EnabledForBooking: %v", err)
	}
	if len(enabled) != 1 || enabled[0].Provider != domain.ProviderEsewa {
		t.Fatalf("enabled = %+v, want just the enabled eSewa account", enabled)
	}
	if enabled[0].SecretKey != "live-key" {
		t.Errorf("secret came back as %q, not decrypted", enabled[0].SecretKey)
	}

	acct, err := repo.ForPayment(ctx, paymentID, domain.ProviderEsewa)
	if err != nil {
		t.Fatalf("ForPayment: %v", err)
	}
	if acct.SecretKey != "live-key" || acct.ArenaID != f.arenaID {
		t.Errorf("ForPayment = %+v", acct)
	}

	providers, err := repo.ProvidersForArena(ctx, f.arenaID)
	if err != nil {
		t.Fatalf("ProvidersForArena: %v", err)
	}
	if len(providers) != 1 || providers[0] != domain.ProviderEsewa {
		t.Errorf("providers = %v, want [esewa] (khalti is disabled)", providers)
	}

	if err := repo.DeleteOwnerScoped(ctx, f.arenaID, f.owner, domain.ProviderEsewa); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if left, _ := repo.ProvidersForArena(ctx, f.arenaID); len(left) != 0 {
		t.Errorf("providers after delete = %v, want none", left)
	}
}

// --- small seed helpers, local to this file ---------------------------------

func seedUser(t *testing.T, f *fixture, tag string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	suffix := uuid.NewString()[:8]
	if err := f.pool.QueryRow(context.Background(), `
		insert into users (email, password_hash, username, full_name)
		values ($1, 'x', $2, 'Seed') returning id`,
		tag+"-"+suffix+"@test.np", tag+"_"+suffix).Scan(&id); err != nil {
		t.Fatalf("seeding user %s: %v", tag, err)
	}
	t.Cleanup(func() { _, _ = f.pool.Exec(context.Background(), `delete from users where id = $1`, id) })
	return id
}

func seedBooking(t *testing.T, f *fixture, player uuid.UUID) uuid.UUID {
	t.Helper()
	start := time.Date(2031, 3, 15, 18, 0, 0, 0, time.UTC)
	var id uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `
		insert into bookings (court_id, user_id, slot, price_npr, status, hold_expires_at)
		values ($1, $2, tstzrange($3, $4, '[)'), 1800, 'pending', now() + interval '15 minutes')
		returning id`,
		f.courtID, player, start, start.Add(time.Hour)).Scan(&id); err != nil {
		t.Fatalf("seeding booking: %v", err)
	}
	return id
}

func seedPayment(t *testing.T, f *fixture, bookingID uuid.UUID, provider domain.PaymentProvider) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `
		insert into payments (booking_id, provider, amount_npr, status, transaction_uuid)
		values ($1, $2, 1800, 'initiated', $3)
		returning id`,
		bookingID, provider, uuid.NewString()).Scan(&id); err != nil {
		t.Fatalf("seeding payment: %v", err)
	}
	return id
}
