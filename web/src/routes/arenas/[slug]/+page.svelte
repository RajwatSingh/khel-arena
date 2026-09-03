<script>
	import { page } from '$app/state';
	import { api, ApiError, SPORT_LABELS } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import DateRail from '$lib/components/DateRail.svelte';
	import HoldPanel from '$lib/components/HoldPanel.svelte';
	import PhotoStrip from '$lib/components/PhotoStrip.svelte';
	import ReviewPanel from '$lib/components/ReviewPanel.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { reveal } from '$lib/actions/reveal.js';
	import { dates as dateHelper, formatNPR, formatTime } from '$lib/time.js';

	let { data } = $props();

	const arena = $derived(data.arena);
	const dates = dateHelper.rail(7);

	let courtId = $state(page.url.searchParams.get('court'));
	let date = $state(page.url.searchParams.get('date') ?? dates[0]);

	// A booking can span up to four consecutive free hours. `anchor` is the
	// first hour tapped; `head` is the far end of the block (null while only
	// one hour is picked). The deep link from the search pages sets the
	// anchor.
	let anchor = $state(page.url.searchParams.get('at'));
	let head = $state(null);

	let hold = $state(null);
	let busy = $state(false);
	let error = $state(null);

	// The gateways *this venue* takes. Each arena holds its own eSewa/Khalti
	// credentials, so this is per arena, not per deployment. Asked for rather
	// than hardcoded, so a provider the owner has not set up is never offered.
	let providers = $state([]);

	$effect(() => {
		const id = arena.id;
		let cancelled = false;
		api
			.arenaPaymentProviders(id)
			.then((next) => {
				if (!cancelled) providers = next;
			})
			.catch(() => {
				if (!cancelled) providers = [];
			});
		return () => {
			cancelled = true;
		};
	});

	// Falls back to the first court, which is also what makes an arena-to-arena
	// navigation safe: this component is reused, so `courtId` can still name a
	// court that belongs to the arena you just left.
	const court = $derived(arena.courts.find((c) => c.id === courtId) ?? arena.courts[0]);

	// The grid is fetched, not computed, so it cannot be a $derived: it arrives
	// later than the court and date that ask for it. The effect re-runs when
	// either changes, and the cancelled flag drops a reply that lost the race
	// -- without it, switching courts quickly can leave the slower response
	// painting the grid of the court you just left.
	let grid = $state({ slots: [] });
	let gridError = $state(null);

	$effect(() => {
		const courtID = court.id;
		const on = date;
		let cancelled = false;

		api
			.availability(courtID, on)
			.then((next) => {
				if (!cancelled) {
					grid = next;
					gridError = null;
				}
			})
			.catch((err) => {
				if (cancelled) return;
				grid = { slots: [] };
				gridError = err instanceof ApiError ? err.message : 'We could not load that day.';
			});

		return () => {
			cancelled = true;
		};
	});

	const freeCount = $derived(grid.slots.filter((s) => s.available).length);

	// The most consecutive free hours the domain allows in one booking.
	const MAX_HOURS = 4;

	/**
	 * The run of grid slots from one start time to another, in order — or an
	 * empty array if that run isn't a bookable block: a gap, a taken hour, or
	 * longer than the four-hour limit.
	 */
	function runBetween(aStart, bStart) {
		const slots = grid.slots;
		let i = slots.findIndex((x) => x.starts_at === aStart);
		let j = slots.findIndex((x) => x.starts_at === bStart);
		if (i < 0 || j < 0) return [];
		if (i > j) [i, j] = [j, i];
		const run = slots.slice(i, j + 1);
		if (run.length > MAX_HOURS) return [];
		for (let k = 0; k < run.length; k++) {
			if (!run[k].available) return [];
			if (k > 0 && run[k - 1].ends_at !== run[k].starts_at) return [];
		}
		return run;
	}

	// The hours currently selected, as grid slots.
	const selection = $derived(anchor ? runBetween(anchor, head ?? anchor) : []);
	const selectedStarts = $derived(new Set(selection.map((s) => s.starts_at)));

	/**
	 * One synthetic "slot" standing for the whole block, so the panel and the
	 * hold request work the same for one hour or four.
	 *
	 * The price follows the service's rule: the rate at the *start* hour,
	 * charged for every hour of the block — an evening block that starts in
	 * peak is peak throughout, not split at the boundary.
	 */
	const slot = $derived(
		selection.length
			? {
					starts_at: selection[0].starts_at,
					ends_at: selection[selection.length - 1].ends_at,
					hours: selection.length,
					price_npr: selection[0].price_npr * selection.length,
					rule: selection[0].rule,
					is_peak: selection[0].is_peak
				}
			: null
	);

	let lastArenaId;
	$effect(() => {
		const id = arena.id;
		if (lastArenaId === undefined || id === lastArenaId) {
			lastArenaId = id;
			return;
		}
		lastArenaId = id;
		courtId = null;
		clearSelection();
		hold = null;
		error = null;
	});

	const shownError = $derived(error ?? gridError);

	function clearSelection() {
		anchor = null;
		head = null;
	}

	/**
	 * Tap once to pick an hour; tap another free hour to book the block
	 * between them. Tapping the sole picked hour again clears it; tapping
	 * somewhere a block can't reach starts a new selection there.
	 */
	function choose(next) {
		error = null;
		if (hold) return;

		if (!anchor || (next.starts_at === anchor && head === null)) {
			const sameHour = anchor === next.starts_at && head === null;
			anchor = sameHour ? null : next.starts_at;
			head = null;
			return;
		}

		const run = runBetween(anchor, next.starts_at);
		if (run.length) {
			head = next.starts_at;
		} else {
			// A gap, a taken hour, or over four hours — restart from here.
			anchor = next.starts_at;
			head = null;
		}
	}

	function switchCourt(id) {
		courtId = id;
		clearSelection();
		hold = null;
		error = null;
	}

	function switchDate(next) {
		date = next;
		clearSelection();
		hold = null;
		error = null;
	}

	async function takeHold() {
		busy = true;
		error = null;
		try {
			hold = await api.createBooking({
				court_id: court.id,
				// The whole block, start to finish. One booking covers one to
				// four consecutive hours; the service prices and locks the span.
				starts_at: slot.starts_at,
				ends_at: slot.ends_at
			});
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Something went wrong on our side.';
		} finally {
			busy = false;
		}
	}

	// Paying leaves the site: the gateway takes over, and the player comes back
	// through /v1/payments/{provider}/callback, which confirms the booking
	// server-side before redirecting them on to /bookings.
	//
	// `busy` is never cleared on the success path because there is no success
	// path here -- redirectToGateway navigates away.
	async function pay(provider = providers[0]) {
		if (!provider) {
			error = 'No payment method is available right now. Settle at the arena.';
			return;
		}

		busy = true;
		error = null;
		try {
			const checkout = await api.startCheckout(hold.id, provider);
			api.redirectToGateway(checkout);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'The payment did not go through.';
			busy = false;
		}
	}

	async function release() {
		const id = hold?.id;
		hold = null;
		clearSelection();
		error = null;
		if (id) await api.cancelBooking(id).catch(() => {});
	}
</script>

<svelte:head>
	<title>{arena.name}, {arena.area} · Book by the hour | Khel Arena</title>
	<meta name="description" content={arena.description} />
</svelte:head>

<section class="masthead forest-band">
	<div class="shell">
		<nav class="crumbs small fade-up" aria-label="Breadcrumb">
			<a class="link" href="/arenas">Arenas</a>
			<span aria-hidden="true">›</span>
			<span>{arena.area}, {arena.city}</span>
		</nav>

		<div class="title">
			<h1 class="display display-xl">{#key arena.slug}<TextAnimate text={arena.name} />{/key}</h1>
			<span class="rating num fade-up fade-up-1">
				<svg viewBox="0 0 12 12" aria-hidden="true"
					><path d="M6 .8l1.6 3.3 3.6.5-2.6 2.5.6 3.6L6 9l-3.2 1.7.6-3.6L.8 4.6l3.6-.5z" /></svg
				>
				{arena.rating}
				<span class="reviews">· {arena.review_count} reviews</span>
			</span>
		</div>

		<div class="pitch-mark fade-up fade-up-1">
			<CentreMark wide />
		</div>

		<div class="intro fade-up fade-up-2">
			<p class="prose">{arena.description}</p>

			<dl class="vitals">
				<div>
					<dt class="label">Open daily</dt>
					<dd class="num">{arena.opens_at} – {arena.closes_at}</dd>
				</div>
				<div>
					<dt class="label">Phone</dt>
					<dd class="num">{arena.phone}</dd>
				</div>
				<div class="wide">
					<dt class="label">On site</dt>
					<dd>
						<ul class="amenities">
							{#each arena.amenities as amenity (amenity)}
								<li>{amenity}</li>
							{/each}
						</ul>
					</dd>
				</div>
			</dl>
		</div>
	</div>
</section>

<section class="booking">
	<div class="shell">
		<h2 class="label section-label" use:reveal>Choose a court</h2>
		<div class="courts" role="group" aria-label="Choose a court" use:reveal={{ delay: 60 }}>
			{#each arena.courts as c (c.id)}
				<button
					type="button"
					class="court"
					class:on={c.id === court.id}
					aria-pressed={c.id === court.id}
					onclick={() => switchCourt(c.id)}
				>
					<span class="who">
						<span class="court-name display display-m">{c.name}</span>
						<span class="court-meta small">{SPORT_LABELS[c.sport]} · {c.format} · {c.surface}</span>
					</span>
					<span class="court-price small">
						from <strong class="num">NPR {formatNPR(c.base_price_npr)}</strong> / hour
					</span>
				</button>
			{/each}
		</div>

		<h2 class="label section-label" use:reveal>Choose a day</h2>
		<div use:reveal={{ delay: 60 }}>
			<DateRail {dates} selected={date} onselect={switchDate} />
		</div>

		<div class="split">
			<div>
				<h2 class="label section-label">
					{court.name} · {freeCount}
					{freeCount === 1 ? 'hour' : 'hours'} free
				</h2>
				<p class="hours-hint small">
					{#if selection.length > 1}
						{selection.length} hours selected — {formatTime(slot.starts_at)}–{formatTime(
							slot.ends_at
						)}. Tap another hour to change the block, or tap the first one to clear it.
					{:else}
						Tap an hour, then tap another to book a block of up to four.
					{/if}
				</p>
				<ul class="hours">
					{#each grid.slots as s, i (s.starts_at)}
						<li style="--i: {Math.min(i, 10)}">
							{#if s.available}
								<button
									type="button"
									class="hour"
									class:peak={s.is_peak}
									class:on={selectedStarts.has(s.starts_at)}
									aria-pressed={selectedStarts.has(s.starts_at)}
									onclick={() => choose(s)}
								>
									<span class="at num">{formatTime(s.starts_at)}</span>
									<span class="price num">NPR {formatNPR(s.price_npr)}</span>
									<span class="rule">{s.rule}</span>
								</button>
							{:else}
								<div class="hour off" aria-disabled="true">
									<span class="at num">{formatTime(s.starts_at)}</span>
									<span class="price">{s.is_past ? 'Gone' : 'Taken'}</span>
									<span class="rule">{s.is_past ? 'Already played' : 'Someone booked it'}</span>
								</div>
							{/if}
						</li>
					{/each}
				</ul>
			</div>

			<HoldPanel
				{arena}
				{court}
				{date}
				{slot}
				{hold}
				{busy}
				error={shownError}
				signedIn={session.signedIn}
				onhold={takeHold}
				onpay={pay}
				onrelease={release}
			/>
		</div>
	</div>
</section>

<section class="extras">
	<div class="shell">
		<PhotoStrip photos={data.photos} />
		<ReviewPanel arenaId={arena.id} />
	</div>
</section>

<style>
	.extras {
		padding-bottom: var(--band);
	}

	.crumbs {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		color: var(--on-field-faint);
	}

	.crumbs .link {
		color: var(--accent-on-field);
		text-decoration-color: color-mix(in srgb, var(--accent-on-field) 35%, transparent);
	}

	.crumbs .link:hover,
	.crumbs .link:focus-visible {
		text-decoration-color: var(--accent-on-field);
	}

	.title {
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: 0.75rem 1.25rem;
		margin-top: 0.9rem;
	}

	.rating {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		padding: 0.35rem 0.8rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine);
		font-size: 0.9375rem;
		font-weight: 600;
	}

	.rating svg {
		width: 13px;
		height: 13px;
		fill: currentColor;
	}

	.reviews {
		font-weight: 500;
		opacity: 0.75;
	}

	.intro {
		display: grid;
		grid-template-columns: minmax(0, 1.05fr) minmax(0, 1fr);
		gap: clamp(1.5rem, 4vw, 3.5rem);
		margin-top: 1.75rem;
		align-items: start;
	}

	.intro .prose {
		font-size: 1.125rem;
		line-height: 1.65;
	}

	/* Read straight off the dark band, like a scoreboard readout — a hairline
	   rule at top stands in for the box the numbers used to sit in. */
	.vitals {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1.25rem 1.5rem;
		padding-top: 1.25rem;
		border-top: 1px solid color-mix(in srgb, var(--on-field) 20%, transparent);
	}

	.vitals .wide {
		grid-column: 1 / -1;
	}

	.vitals dt.label {
		color: var(--on-field-faint);
	}

	dd {
		margin: 0;
		margin-top: 0.3rem;
		font-size: 1rem;
		color: var(--on-field);
	}

	.amenities {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
		margin-top: 0.5rem;
	}

	.amenities li {
		padding: 0.25rem 0.7rem;
		border: 1px solid color-mix(in srgb, var(--on-field) 24%, transparent);
		border-radius: var(--r-pill);
		font-size: 0.8125rem;
		color: var(--on-field-muted);
	}

	/* ----------------------------------------------------------- booking --- */

	.booking {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	.section-label {
		margin-bottom: 0.8rem;
	}

	.hours-hint {
		margin: -0.4rem 0 1rem;
		color: var(--muted);
		max-width: 40ch;
	}

	/* A pick-list, not a rack of tiles: rows in a register, told apart by a
	   left rule that lights up pine when a court is picked. */
	.courts {
		display: grid;
		margin-bottom: 2rem;
		border-top: 1px solid var(--line);
	}

	.court {
		display: flex;
		align-items: center;
		justify-content: space-between;
		flex-wrap: wrap;
		gap: 0.4rem 2rem;
		width: 100%;
		padding: 1.1rem 0.9rem 1.1rem 1.1rem;
		border-bottom: 1px solid var(--line);
		border-left: 3px solid transparent;
		text-align: left;
		cursor: pointer;
		transition:
			border-color 0.18s var(--ease),
			background-color 0.18s var(--ease);
	}

	.court:hover:not(.on) {
		border-left-color: var(--line-strong);
		background: var(--surface);
	}

	.court:active {
		background: var(--surface-sunk);
	}

	.court.on {
		border-left-color: var(--pine);
		background: var(--pine-wash);
	}

	.court-name {
		display: block;
		transition: color 0.18s var(--ease);
	}

	.court.on .court-name {
		color: var(--pine);
	}

	.court-meta {
		display: block;
		margin-top: 0.15rem;
		color: var(--muted);
	}

	.court-price {
		flex-shrink: 0;
		color: var(--muted);
	}

	.court-price strong {
		color: var(--ink);
		font-weight: 600;
	}

	.split {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 23rem;
		gap: clamp(1.5rem, 3vw, 2.5rem);
		margin-top: 2rem;
		align-items: start;
	}

	.hours {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(10.5rem, 1fr));
		gap: 0.6rem;
	}

	.hours li {
		animation: hour-in 0.3s var(--ease) backwards;
		animation-delay: calc(var(--i) * 30ms);
	}

	@keyframes hour-in {
		from {
			opacity: 0;
			transform: translateY(6px);
		}
	}

	.hour {
		display: grid;
		gap: 0.1rem;
		width: 100%;
		padding: 0.9rem 1rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
		text-align: left;
		cursor: pointer;
		transition:
			border-color 0.15s var(--ease),
			background-color 0.15s var(--ease),
			color 0.15s var(--ease),
			transform 0.12s var(--ease);
	}

	.hour:hover:not(.on):not(.off) {
		border-color: var(--pine);
		transform: translateY(-2px);
	}

	.hour:active:not(.off) {
		transform: scale(0.97);
	}

	.hour.peak {
		background: var(--sand);
		border-color: var(--sand-line);
	}

	.hour.on {
		background: var(--pine);
		border-color: var(--pine);
		color: #ffffff;
		animation: settle-in 0.2s var(--ease);
	}

	@keyframes settle-in {
		from {
			transform: scale(0.95);
		}
	}

	.at {
		font-size: 1.125rem;
		font-weight: 600;
		letter-spacing: -0.015em;
	}

	.price {
		font-size: 0.9375rem;
		color: var(--muted);
	}

	.rule {
		font-size: 0.8125rem;
		color: var(--faint);
	}

	.hour.on .price,
	.hour.on .rule {
		color: rgba(255, 255, 255, 0.8);
	}

	.off {
		cursor: default;
		background: var(--surface-sunk);
		border-color: transparent;
	}

	.off .at,
	.off .price,
	.off .rule {
		color: var(--faint);
	}

	@media (max-width: 940px) {
		.intro,
		.split {
			grid-template-columns: minmax(0, 1fr);
		}
	}

	@media (max-width: 480px) {
		.vitals {
			grid-template-columns: 1fr;
		}
	}
</style>
