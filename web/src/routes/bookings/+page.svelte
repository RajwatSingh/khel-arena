<script>
	import { api, ApiError } from '$lib/api/index.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { reveal } from '$lib/actions/reveal.js';
	import { clock, session } from '$lib/session.svelte.js';
	import { formatCountdown, formatDate, formatNPR, formatTime } from '$lib/time.js';

	const ticking = clock();

	let version = $state(0);
	let error = $state(null);
	let bookings = $state([]);

	// The gateways each venue takes, keyed by arena id. Payment credentials
	// are per arena now, so "what can I pay this with" is answered per booking,
	// not once for the whole site. Fetched for the arenas that actually have a
	// hold waiting on them.
	let providersByArena = $state({});

	$effect(() => {
		const pending = bookings.filter((b) => b.status === 'pending');
		const arenaIds = [...new Set(pending.map((b) => b.arena_id))].filter(
			(id) => !(id in providersByArena)
		);
		if (!arenaIds.length) return;

		let cancelled = false;
		for (const id of arenaIds) {
			api
				.arenaPaymentProviders(id)
				.then((next) => {
					if (!cancelled) providersByArena = { ...providersByArena, [id]: next };
				})
				.catch(() => {
					// Not worth an error banner: the pay button reports it if
					// the player actually reaches for it.
					if (!cancelled) providersByArena = { ...providersByArena, [id]: [] };
				});
		}
		return () => {
			cancelled = true;
		};
	});

	// Fetched, so this is an effect rather than a $derived: the list arrives
	// after the render that asked for it.
	//
	// It re-reads whenever the signed-in state changes, whenever an action
	// bumps `version`, and once a minute -- the last so a hold that lapsed
	// while the page sat open stops claiming to be live, which is the janitor's
	// rule showing through. Once a minute rather than once a second, which is
	// how often the countdown ticks: the countdown is arithmetic on a timestamp
	// already in hand and costs nothing, a re-read is a request.
	$effect(() => {
		void version;
		void Math.floor(ticking.now / 60_000);

		if (!session.signedIn) {
			bookings = [];
			return;
		}

		let cancelled = false;
		api
			.listBookings()
			.then((next) => {
				if (!cancelled) bookings = next;
			})
			.catch((err) => {
				if (cancelled) return;
				bookings = [];
				error = err instanceof ApiError ? err.message : 'We could not load your bookings.';
			});

		return () => {
			cancelled = true;
		};
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

	// Leaves the site for the gateway; see the note on the arena page.
	async function pay(booking, provider = (providersByArena[booking.arena_id] ?? [])[0]) {
		error = null;
		if (!provider) {
			error = 'This venue has no online payment set up. Settle at the arena.';
			return;
		}
		try {
			api.redirectToGateway(await api.startCheckout(booking.id, provider));
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'The payment did not go through.';
		}
	}
</script>

<svelte:head>
	<title>My bookings | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">Your hours</p>
		<h1 class="display display-l"><TextAnimate text="My bookings" /></h1>
		<div class="pitch-mark fade-up fade-up-1">
			<CentreMark wide />
		</div>
	</div>
</section>

<section class="body">
	<div class="shell">
		{#if !session.signedIn}
			<div class="notice fade-up">
				<h2 class="display display-m">Sign in to see your hours</h2>
				<p>Bookings are tied to your account, so we need to know whose name to look under.</p>
				<a class="btn btn-primary" href="/login">Sign in</a>
			</div>
		{:else if bookings.length === 0}
			<div class="notice fade-up">
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
				{#each bookings as booking, i (booking.id)}
					{@const remaining =
						booking.status === 'pending'
							? new Date(booking.hold_expires_at).getTime() - ticking.now
							: 0}
					{@const done = booking.status === 'cancelled' || booking.status === 'expired'}
					<li class="row" class:done use:reveal={{ delay: Math.min(i, 6) * 60 }}>
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
								<span
									class="note num"
									class:urgent={remaining < 120_000}
									class:pulse-urgent={remaining < 120_000}
								>
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
	h1 {
		margin-top: 0.6rem;
	}

	.body {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	.notice {
		display: grid;
		justify-items: start;
		gap: 0.75rem;
		padding-block: clamp(2.5rem, 6vw, 4rem);
		border-top: 1px solid var(--line);
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
		border-top: 1px solid var(--line);
	}

	.row {
		display: grid;
		grid-template-columns: 11rem minmax(0, 1fr) 13rem 8rem auto;
		align-items: center;
		gap: 1rem 1.75rem;
		padding: 1.35rem 0.25rem;
		border-bottom: 1px solid var(--line);
		transition: background-color var(--dur-hover) var(--ease);
	}

	.row:hover {
		background: var(--surface);
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
		color: #7a3b22;
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
