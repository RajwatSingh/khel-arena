package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/crypto"
)

// ArenaPaymentAccountRepo stores each venue's own gateway credentials.
//
// secret_key is a bytea holding AES-256-GCM ciphertext. It is sealed on the
// way in and opened only for the two reads that actually build a gateway
// adapter (EnabledForBooking, ForPayment). The owner-facing list never opens
// it — it returns a four-character hint instead.
//
// The repo is constructed only when PAYMENT_ENC_KEY is set. With no key there
// is nothing to seal secrets with, so online payments are simply off and every
// booking settles in cash.
type ArenaPaymentAccountRepo struct {
	pool *pgxpool.Pool
	box  *crypto.Box
}

func NewArenaPaymentAccountRepo(pool *pgxpool.Pool, box *crypto.Box) *ArenaPaymentAccountRepo {
	return &ArenaPaymentAccountRepo{pool: pool, box: box}
}

// UpsertOwnerScoped stores or replaces one arena's account for one provider.
//
// The ownership check is in the statement: the INSERT ... SELECT produces a
// row only when the caller owns the arena, so a stranger's request touches
// nothing and comes back as "not found" rather than silently writing.
func (r *ArenaPaymentAccountRepo) UpsertOwnerScoped(ctx context.Context, arenaID, ownerID uuid.UUID, acct domain.ArenaPaymentAccount) error {
	sealed, err := r.box.Seal([]byte(acct.SecretKey))
	if err != nil {
		return domain.Internal(err, "sealing payment secret for arena %s", arenaID)
	}

	const q = `
		insert into arena_payment_accounts
			(arena_id, provider, secret_key, merchant_code, live, enabled)
		select $1, $2, $3, $4, $5, $6
		where exists (select 1 from arenas where id = $1 and owner_id = $7)
		on conflict (arena_id, provider) do update
			set secret_key    = excluded.secret_key,
			    merchant_code = excluded.merchant_code,
			    live          = excluded.live,
			    enabled       = excluded.enabled`

	tag, err := r.pool.Exec(ctx, q,
		arenaID, acct.Provider, sealed, acct.MerchantCode, acct.Live, acct.Enabled, ownerID)
	if err != nil {
		return domain.Internal(err, "storing payment account for arena %s", arenaID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No venue of yours at that address.")
	}
	return nil
}

// DeleteOwnerScoped removes an arena's account for one provider.
func (r *ArenaPaymentAccountRepo) DeleteOwnerScoped(ctx context.Context, arenaID, ownerID uuid.UUID, provider domain.PaymentProvider) error {
	const q = `
		delete from arena_payment_accounts apa
		using arenas a
		where apa.arena_id = a.id
		  and apa.arena_id = $1
		  and a.owner_id   = $2
		  and apa.provider = $3`

	tag, err := r.pool.Exec(ctx, q, arenaID, ownerID, provider)
	if err != nil {
		return domain.Internal(err, "deleting payment account for arena %s", arenaID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No such payment account to remove.")
	}
	return nil
}

// ListOwnerScoped returns the state of every account on one of the caller's
// venues, secrets reduced to a hint.
func (r *ArenaPaymentAccountRepo) ListOwnerScoped(ctx context.Context, arenaID, ownerID uuid.UUID) ([]domain.ArenaPaymentAccountInfo, error) {
	const q = `
		select apa.provider, apa.merchant_code, apa.secret_key, apa.live, apa.enabled, apa.updated_at
		from arena_payment_accounts apa
		join arenas a on a.id = apa.arena_id
		where apa.arena_id = $1 and a.owner_id = $2
		order by apa.provider`

	rows, err := r.pool.Query(ctx, q, arenaID, ownerID)
	if err != nil {
		return nil, domain.Internal(err, "listing payment accounts for arena %s", arenaID)
	}
	defer rows.Close()

	out := make([]domain.ArenaPaymentAccountInfo, 0, len(onlineProviders))
	for rows.Next() {
		var (
			info   domain.ArenaPaymentAccountInfo
			sealed []byte
		)
		if err := rows.Scan(&info.Provider, &info.MerchantCode, &sealed, &info.Live, &info.Enabled, &info.UpdatedAt); err != nil {
			return nil, domain.Internal(err, "scanning payment account")
		}
		if plain, err := r.box.Open(sealed); err == nil {
			info.SecretHint = domain.HintFor(string(plain))
		}
		out = append(out, info)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "listing payment accounts for arena %s", arenaID)
	}
	return out, nil
}

// onlineProviders is only a capacity hint for ListOwnerScoped.
var onlineProviders = []domain.PaymentProvider{domain.ProviderEsewa, domain.ProviderKhalti}

const accountResolveColumns = `
	apa.arena_id, apa.provider, apa.secret_key, apa.merchant_code, apa.live, apa.enabled`

func (r *ArenaPaymentAccountRepo) scanAccount(row pgx.Row) (domain.ArenaPaymentAccount, error) {
	var (
		acct   domain.ArenaPaymentAccount
		sealed []byte
	)
	if err := row.Scan(&acct.ArenaID, &acct.Provider, &sealed, &acct.MerchantCode, &acct.Live, &acct.Enabled); err != nil {
		return domain.ArenaPaymentAccount{}, err
	}
	plain, err := r.box.Open(sealed)
	if err != nil {
		return domain.ArenaPaymentAccount{}, domain.Internal(err,
			"opening payment secret for arena %s", acct.ArenaID)
	}
	acct.SecretKey = string(plain)
	return acct, nil
}

// EnabledForBooking returns the accounts a booking's arena is currently taking
// payment through — used both to list the player's options and to pick one.
func (r *ArenaPaymentAccountRepo) EnabledForBooking(ctx context.Context, bookingID uuid.UUID) ([]domain.ArenaPaymentAccount, error) {
	const q = `
		select ` + accountResolveColumns + `
		from arena_payment_accounts apa
		join courts   c on c.arena_id = apa.arena_id
		join bookings b on b.court_id = c.id
		where b.id = $1 and apa.enabled
		order by apa.provider`

	rows, err := r.pool.Query(ctx, q, bookingID)
	if err != nil {
		return nil, domain.Internal(err, "loading payment accounts for booking %s", bookingID)
	}
	defer rows.Close()

	var out []domain.ArenaPaymentAccount
	for rows.Next() {
		acct, err := r.scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, acct)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "loading payment accounts for booking %s", bookingID)
	}
	return out, nil
}

// ForPayment returns the account a started payment must be verified through:
// the one its arena holds for the provider the payment was begun with.
//
// enabled is deliberately not required. A payment already in flight when the
// owner switches the provider off still has to be settled — the money has
// moved either way.
func (r *ArenaPaymentAccountRepo) ForPayment(ctx context.Context, paymentID uuid.UUID, provider domain.PaymentProvider) (domain.ArenaPaymentAccount, error) {
	const q = `
		select ` + accountResolveColumns + `
		from arena_payment_accounts apa
		join courts   c on c.arena_id = apa.arena_id
		join bookings b on b.court_id = c.id
		join payments p on p.booking_id = b.id
		where p.id = $1 and apa.provider = $2`

	acct, err := r.scanAccount(r.pool.QueryRow(ctx, q, paymentID, provider))
	if noRows(err) {
		return domain.ArenaPaymentAccount{}, domain.NotFound(
			"That venue no longer has a %s account to settle this through.", provider)
	}
	if err != nil {
		return domain.ArenaPaymentAccount{}, domain.Internal(err,
			"loading payment account for payment %s", paymentID)
	}
	return acct, nil
}

// ProvidersForArena lists the providers one arena is taking, for the public
// arena page.
func (r *ArenaPaymentAccountRepo) ProvidersForArena(ctx context.Context, arenaID uuid.UUID) ([]domain.PaymentProvider, error) {
	const q = `
		select provider from arena_payment_accounts
		where arena_id = $1 and enabled
		order by provider`

	rows, err := r.pool.Query(ctx, q, arenaID)
	if err != nil {
		return nil, domain.Internal(err, "loading payment providers for arena %s", arenaID)
	}
	defer rows.Close()

	out := []domain.PaymentProvider{}
	for rows.Next() {
		var p domain.PaymentProvider
		if err := rows.Scan(&p); err != nil {
			return nil, domain.Internal(err, "scanning payment provider")
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "loading payment providers for arena %s", arenaID)
	}
	return out, nil
}
