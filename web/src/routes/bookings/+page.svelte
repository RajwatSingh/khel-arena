<script>
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { reveal } from '$lib/actions/reveal.js';
	import { clock, session } from '$lib/session.svelte.js';
	import { formatCountdown, formatDate, formatNPR, formatTime } from '$lib/time.js';
	import { ONLINE_PROVIDERS, notSetUpMessage, providerLabel } from '$lib/payments.js';
	import CancelDialog from '$lib/components/CancelDialog.svelte';
	import { isBanned, isLateCancel, BOOKING_BAN_FLOOR, LATE_CANCEL_PENALTY } from '$lib/userScore.js';

	const ticking = clock();

	// Where a gateway sends the player back: the API checks the payment with
	// eSewa or Khalti, then redirects here with ?payment=paid|failed and, when
	// it knows it, ?booking=<id>. Read once, then dropped from the address bar
	// so a refresh doesn't announce the same payment again.
	const PAYMENT_OUTCOMES = {
		paid: { tone: 'ok', text: 'Payment received. Your booking is confirmed.' },
		failed: {
			tone: 'bad',
			// Also what a cancel looks like: eSewa returns a cancelled and a
			// failed payment the same way, so this can't tell them apart.
			text: "Payment cancelled or didn't go through. If your hold still has time on it, you can try again."
		}
	};
	const paymentOutcome = PAYMENT_OUTCOMES[page.url.searchParams.get('payment')] ?? null;
	const paidBookingId =
		page.url.searchParams.get('payment') === 'paid' ? page.url.searchParams.get('booking') : null;

	onMount(() => {
		if (page.url.searchParams.has('payment')) replaceState('/bookings', {});
	});

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
					// Not worth an error banner. Null means "unknown": the pay
					// buttons then go straight to the server, whose answer is
					// shown if the venue can't take that gateway.
					if (!cancelled) providersByArena = { ...providersByArena, [id]: null };
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

	// Cash bookings are told apart by their latest payment: on a confirmed or
	// played booking a cash intent is money still to hand over at the venue.
	// On a pending one it is an older request still waiting on the venue, from
	// before cash bookings were confirmed straight away.
	const isCash = (b) => b.payment_provider === 'cash' && b.payment_status === 'initiated';
	const awaitingVenue = (b) => b.status === 'pending' && isCash(b);
	const cashDue = (b) => (b.status === 'confirmed' || b.status === 'completed') && isCash(b);

	function statusLabel(b) {
		if (awaitingVenue(b)) return 'Waiting for venue';
		return STATUS[b.status] ?? b.status;
	}

	async function payCash(booking) {
		error = null;
		try {
			await api.startCheckout(booking.id, 'cash');
			version++;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'The booking did not go through.';
		}
	}

	// The player's user score and any booking ban. Read from /me, then kept
	// up to date from what each cancellation reports.
	let standing = $state(null);
	$effect(() => {
		if (!session.signedIn) return;
		api
			.me()
			.then((me) => {
				standing = { score: me.user_score, bannedUntil: me.booking_banned_until };
			})
			.catch(() => {});
	});
	const banned = $derived(standing ? isBanned(standing.bannedUntil, ticking.now) : false);

	// Every cancel goes through the popup, which asks why first.
	let cancelling = $state(null);
	let cancelOpen = $state(false);
	let cancelNotice = $state(null);

	function cancel(booking) {
		error = null;
		cancelNotice = null;
		cancelling = booking;
		cancelOpen = true;
	}

	// Throws on failure so the popup stays open and shows why.
	async function confirmCancel(reason) {
		const outcome = await api.cancelBooking(cancelling.id, reason);
		standing = { score: outcome.user_score, bannedUntil: outcome.booking_banned_until };
		if (outcome.late) {
			cancelNotice = isBanned(outcome.booking_banned_until)
				? `Cancelled. Your user score is now ${outcome.user_score}/100, so you can't book until ${formatDate(outcome.booking_banned_until.slice(0, 10))}.`
				: `Cancelled. Your user score is now ${outcome.user_score}/100.`;
		}
		version++;
	}

	function cancelTitle(b) {
		if (!b) return 'Cancel this booking?';
		if (b.status === 'pending' && !awaitingVenue(b)) return 'Let go of this hold?';
		if (awaitingVenue(b)) return 'Withdraw this request?';
		return 'Cancel this booking?';
	}

	// A message under one booking — e.g. the venue hasn't set up that gateway —
	// kept next to the button that caused it rather than at the top of a long list.
	let rowNotice = $state({ id: null, text: '' });
	let payingWith = $state({ id: null, provider: null });

	// Leaves the site for the gateway; see the note on the arena page.
	async function pay(booking, provider) {
		error = null;
		rowNotice = { id: null, text: '' };
		const available = providersByArena[booking.arena_id];
		if (available && !available.includes(provider)) {
			rowNotice = { id: booking.id, text: notSetUpMessage(provider, booking.arena_name, available) };
			return;
		}
		payingWith = { id: booking.id, provider };
		try {
			api.redirectToGateway(await api.startCheckout(booking.id, provider));
		} catch (err) {
			rowNotice = {
				id: booking.id,
				text: err instanceof ApiError ? err.message : 'The payment did not go through.'
			};
			payingWith = { id: null, provider: null };
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
		{#if paymentOutcome}
			<p class="outcome {paymentOutcome.tone}" role="status">{paymentOutcome.text}</p>
		{/if}

		{#if cancelNotice}
			<p class="outcome bad" role="status">{cancelNotice}</p>
		{/if}

		{#if standing}
			<div class="standing" class:banned>
				<div class="score">
					<span class="label">User score</span>
					<span class="num score-value">{standing.score}<small>/100</small></span>
				</div>
				<p class="small">
					{#if banned}
						<strong>You can't book any venue until {formatDate(standing.bannedUntil.slice(0, 10))}.</strong>
						Your score fell to {BOOKING_BAN_FLOOR} or below after late cancellations. It goes back to
						50 when the ban ends.
					{:else}
						Cancelling a confirmed booking less than 5 hours before kick-off costs
						{LATE_CANCEL_PENALTY} points. At {BOOKING_BAN_FLOOR} or below, you can't book for a month.
					{/if}
				</p>
			</div>
		{/if}

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
					{@const requested = awaitingVenue(booking)}
					{@const available = providersByArena[booking.arena_id]}
					{@const done = booking.status === 'cancelled' || booking.status === 'expired'}
					<li
						class="row"
						class:done
						class:just-paid={booking.id === paidBookingId}
						use:reveal={{ delay: Math.min(i, 6) * 60 }}>
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
								{statusLabel(booking)}
							</span>
							{#if requested && remaining > 0}
								<span class="note">The venue will approve it · pay cash after the game</span>
							{:else if cashDue(booking)}
								<span class="note num">{booking.reference} · pay cash at the venue</span>
							{:else if booking.status === 'pending' && remaining > 0}
								<span
									class="note num"
									class:urgent={remaining < 120_000}
									class:pulse-urgent={remaining < 120_000}
								>
									{formatCountdown(remaining)} left to pay
								</span>
							{:else if booking.status === 'confirmed' && booking.paid_online}
								<span class="note num">
									{booking.reference} · paid with {providerLabel(booking.paid_online)}
								</span>
							{:else if booking.status === 'confirmed'}
								<span class="note num">{booking.reference}</span>
							{:else if booking.status === 'cancelled' && booking.cancel_reason}
								<span class="note reason">“{booking.cancel_reason}”</span>
							{/if}
						</div>

						<p class="cost num">NPR {formatNPR(booking.price_npr)}</p>

						<div class="acts">
							{#if requested && remaining > 0}
								<button class="btn btn-secondary" onclick={() => cancel(booking)}>Cancel request</button>
							{:else if booking.status === 'pending' && remaining > 0}
								{#each ONLINE_PROVIDERS as p (p.id)}
									{@const paying = payingWith.id === booking.id && payingWith.provider === p.id}
									<button
										class="btn pay-{p.id}"
										class:unavailable={available && !available.includes(p.id)}
										class:loading={paying}
										disabled={payingWith.id === booking.id}
										onclick={() => pay(booking, p.id)}
									>
										{paying ? `Opening ${p.label}…` : p.label}
									</button>
								{/each}
								<button
									class="btn btn-secondary"
									disabled={payingWith.id === booking.id}
									onclick={() => payCash(booking)}
								>
									Pay cash
								</button>
								<button class="btn btn-secondary" onclick={() => cancel(booking)}>Cancel hold</button>
							{:else if booking.status === 'confirmed'}
								{#if !booking.open_to_join && new Date(booking.starts_at).getTime() > ticking.now}
									<a class="btn btn-secondary" href="/games/new?booking={booking.id}">Find players</a>
								{/if}
								{#if booking.paid_online}
									{#if booking.arena_phone}
										<a class="btn btn-secondary" href="tel:{booking.arena_phone}">Call venue to cancel</a>
									{/if}
								{:else}
									<button class="btn btn-secondary" onclick={() => cancel(booking)}>Cancel</button>
								{/if}
							{/if}
						</div>

						{#if booking.status === 'confirmed' && booking.paid_online}
							<p class="row-notice paid-note">
								You paid online, so only {booking.arena_name} can refund you. To cancel, contact them{booking.arena_phone
									? ` on ${booking.arena_phone}`
									: ''}.
							</p>
						{/if}

						{#if rowNotice.id === booking.id}
							<p class="row-notice" role="alert">{rowNotice.text}</p>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</section>

<CancelDialog
	bind:open={cancelOpen}
	title={cancelTitle(cancelling)}
	confirmLabel={cancelling?.status === 'pending' ? 'Cancel hold' : 'Cancel booking'}
	what={cancelling
		? `${cancelling.arena_name} · ${cancelling.court_name} · ${formatDate(cancelling.starts_at.slice(0, 10))}, ${formatTime(cancelling.starts_at)}–${formatTime(cancelling.ends_at)}`
		: ''}
	late={isLateCancel(cancelling, ticking.now)}
	userScore={standing?.score ?? null}
	onconfirm={confirmCancel}
/>

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
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 0.5rem;
		justify-self: end;
	}

	.acts :global(.btn) {
		padding: 0.55em 1.1em;
		font-size: 0.875rem;
	}

	.row-notice {
		grid-column: 1 / -1;
		margin: 0;
		padding: 0.75rem 1rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
		font-size: 0.9375rem;
		line-height: 1.5;
	}

	.outcome {
		margin-bottom: 1.25rem;
		padding: 0.85rem 1.1rem;
		border-radius: var(--r-sm);
		font-size: 0.9375rem;
		font-weight: 600;
	}

	.outcome.ok {
		background: var(--pine-wash);
		color: var(--pine);
	}

	.outcome.bad {
		background: var(--brick-wash);
		color: var(--brick);
	}

	.standing {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem 1.5rem;
		margin-bottom: 1.5rem;
		padding: 0.9rem 1.1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.standing p {
		flex: 1 1 22rem;
		margin: 0;
		color: var(--muted);
	}

	.standing.banned {
		border-color: var(--brick);
		background: var(--brick-wash);
	}

	.standing.banned p { color: var(--brick); }

	.score {
		display: grid;
		gap: 0.1rem;
	}

	.score-value {
		font-size: 1.6rem;
		font-weight: 700;
		line-height: 1;
	}

	.score-value small {
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--faint);
	}

	.note.reason { font-style: italic; }

	/* Information, not an error: quieter than the red row notice. */
	.row-notice.paid-note {
		background: var(--surface-sunk);
		color: var(--muted);
	}

	.row.just-paid {
		background: var(--pine-wash);
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

		.acts {
			justify-content: flex-start;
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
