<script>
	import { api, ApiError } from '$lib/api/index.js';
	import { clock, session } from '$lib/session.svelte.js';
	import { formatCountdown, formatDate, formatNPR, formatTime } from '$lib/time.js';

	const ticking = clock();

	let version = $state(0);
	let error = $state(null);

	// The janitor's rule runs here too: re-read on every tick so a lapsed hold
	// stops claiming to be live.
	const bookings = $derived.by(() => {
		void version;
		void ticking.now;
		if (!session.signedIn) return [];
		try {
			return api.listBookings();
		} catch {
			return [];
		}
	});

	const STATUS = {
		pending: 'Held, unpaid',
		confirmed: 'Confirmed',
		completed: 'Played',
		cancelled: 'Cancelled',
		expired: 'Hold lapsed',
		no_show: 'No show'
	};

	async function cancel(booking) {
		error = null;
		try {
			await api.cancelBooking(booking.id);
			version++;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'That did not go through.';
		}
	}

	async function pay(booking) {
		error = null;
		try {
			await api.payBooking(booking.id);
			version++;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'The payment did not go through.';
		}
	}
</script>

<svelte:head>
	<title>My bookings | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="head">
	<div class="shell">
		<p class="label">Your hours</p>
		<h1 class="display display-l">My bookings</h1>
	</div>
</section>

<section class="body">
	<div class="shell">
		{#if !session.signedIn}
			<div class="card notice">
				<h2 class="display display-m">Sign in to see your hours</h2>
				<p>Bookings are tied to your account, so we need to know whose name to look under.</p>
				<a class="btn btn-primary" href="/login">Sign in</a>
			</div>
		{:else if bookings.length === 0}
			<div class="card notice">
				<h2 class="display display-m">Nothing booked yet</h2>
				<p>
					The board shows every free hour in the valley. Pick one and it is yours for fifteen
					minutes while you pay.
				</p>
				<a class="btn btn-primary" href="/tonight">Find a court</a>
			</div>
		{:else}
			{#if error}
				<p class="error" role="alert">{error}</p>
			{/if}
			<ul class="rows">
				{#each bookings as booking (booking.id)}
					{@const remaining =
						booking.status === 'pending'
							? new Date(booking.hold_expires_at).getTime() - ticking.now
							: 0}
					{@const done = booking.status === 'cancelled' || booking.status === 'expired'}
					<li class="card row" class:done>
						<div class="when">
							<span class="time num">
								{formatTime(booking.starts_at)}–{formatTime(booking.ends_at)}
							</span>
							<span class="date small">{formatDate(booking.starts_at.slice(0, 10))}</span>
						</div>

						<div class="what">
							<a class="venue" href="/arenas/{booking.arena_slug}">{booking.arena_name}</a>
							<span class="court small">
								{booking.court_name} · {booking.format} · {booking.arena_area}
							</span>
						</div>

						<div class="state">
							<span
								class="badge"
								class:live={booking.status === 'confirmed'}
								class:waiting={booking.status === 'pending' && remaining > 0}
							>
								{STATUS[booking.status] ?? booking.status}
							</span>
							{#if booking.status === 'pending' && remaining > 0}
								<span class="note num" class:urgent={remaining < 120_000}>
									{formatCountdown(remaining)} left to pay
								</span>
							{:else if booking.status === 'confirmed'}
								<span class="note num">{booking.reference}</span>
							{/if}
						</div>

						<p class="cost num">NPR {formatNPR(booking.price_npr)}</p>

						<div class="acts">
							{#if booking.status === 'pending' && remaining > 0}
								<button class="btn btn-primary" onclick={() => pay(booking)}>Pay</button>
								<button class="btn btn-secondary" onclick={() => cancel(booking)}>Release</button>
							{:else if booking.status === 'confirmed'}
								<button class="btn btn-secondary" onclick={() => cancel(booking)}>Cancel</button>
							{/if}
						</div>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</section>

<style>
	.head {
		padding-block: clamp(2.5rem, 5vw, 4rem) 1.75rem;
	}

	h1 {
		margin-top: 0.6rem;
	}

	.body {
		padding-bottom: var(--band);
	}

	.notice {
		display: grid;
		justify-items: start;
		gap: 0.75rem;
		padding: clamp(2rem, 5vw, 3.5rem);
	}

	.notice p {
		color: var(--muted);
		max-width: 46ch;
	}

	.notice .btn {
		margin-top: 0.75rem;
	}

	.rows {
		display: grid;
		gap: 0.75rem;
	}

	.row {
		display: grid;
		grid-template-columns: 11rem minmax(0, 1fr) 13rem 8rem auto;
		align-items: center;
		gap: 1rem 1.75rem;
		padding: 1.25rem 1.5rem;
	}

	.row.done {
		opacity: 0.55;
	}

	.time {
		display: block;
		font-size: 1.125rem;
		font-weight: 600;
		letter-spacing: -0.015em;
	}

	.date,
	.court {
		display: block;
		margin-top: 0.1rem;
		color: var(--faint);
	}

	.venue {
		font-size: 1.0625rem;
		font-weight: 600;
		letter-spacing: -0.01em;
	}

	.venue:hover {
		color: var(--pine);
	}

	.badge {
		display: inline-block;
		padding: 0.2rem 0.65rem;
		border-radius: var(--r-pill);
		background: var(--surface-sunk);
		color: var(--muted);
		font-size: 0.8125rem;
		font-weight: 600;
	}

	.badge.live {
		background: var(--pine-wash);
		color: var(--pine);
	}

	.badge.waiting {
		background: var(--sand);
		color: #7a5a1a;
	}

	.note {
		display: block;
		margin-top: 0.3rem;
		font-size: 0.875rem;
		color: var(--faint);
	}

	.note.urgent {
		color: var(--brick);
		font-weight: 600;
	}

	.cost {
		font-size: 1.0625rem;
		font-weight: 600;
	}

	.acts {
		display: flex;
		gap: 0.5rem;
		justify-self: end;
	}

	.acts :global(.btn) {
		padding: 0.55em 1.1em;
		font-size: 0.875rem;
	}

	.error {
		margin-bottom: 1rem;
		padding: 0.85rem 1.1rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
		font-size: 0.9375rem;
	}

	@media (max-width: 1040px) {
		.row {
			grid-template-columns: 10rem minmax(0, 1fr);
		}

		.state,
		.cost,
		.acts {
			grid-column: 2;
			justify-self: start;
		}
	}

	@media (max-width: 560px) {
		.row {
			grid-template-columns: minmax(0, 1fr);
		}

		.state,
		.cost,
		.acts {
			grid-column: 1;
		}
	}
</style>
