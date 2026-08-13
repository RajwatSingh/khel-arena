<script>
	import { page } from '$app/state';
	import { api, ApiError, SPORT_LABELS } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import DateRail from '$lib/components/DateRail.svelte';
	import HoldPanel from '$lib/components/HoldPanel.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import { reveal } from '$lib/actions/reveal.js';
	import { formatNPR, formatTime } from '$lib/time.js';

	let { data } = $props();

	const arena = $derived(data.arena);
	const dates = api.dates.rail(7);

	let courtId = $state(page.url.searchParams.get('court'));
	let date = $state(page.url.searchParams.get('date') ?? dates[0]);
	let picked = $state(page.url.searchParams.get('at'));
	let hold = $state(null);
	let busy = $state(false);
	let error = $state(null);

	// Falls back to the first court, which is also what makes an arena-to-arena
	// navigation safe: this component is reused, so `courtId` can still name a
	// court that belongs to the arena you just left.
	const court = $derived(arena.courts.find((c) => c.id === courtId) ?? arena.courts[0]);
	const grid = $derived(api.availability(court.id, date));
	const slot = $derived(grid.slots.find((s) => s.starts_at === picked && s.available) ?? null);
	const freeCount = $derived(grid.slots.filter((s) => s.available).length);

	let lastArenaId;
	$effect(() => {
		const id = arena.id;
		if (lastArenaId === undefined || id === lastArenaId) {
			lastArenaId = id;
			return;
		}
		lastArenaId = id;
		courtId = null;
		picked = null;
		hold = null;
		error = null;
	});

	function choose(next) {
		picked = next.starts_at;
		error = null;
	}

	function switchCourt(id) {
		courtId = id;
		picked = null;
		hold = null;
		error = null;
	}

	function switchDate(next) {
		date = next;
		picked = null;
		hold = null;
		error = null;
	}

	async function takeHold() {
		busy = true;
		error = null;
		try {
			hold = await api.createBooking({ court_id: court.id, starts_at: slot.starts_at });
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Something went wrong on our side.';
		} finally {
			busy = false;
		}
	}

	async function pay() {
		busy = true;
		error = null;
		try {
			hold = await api.payBooking(hold.id);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'The payment did not go through.';
		} finally {
			busy = false;
		}
	}

	async function release() {
		const id = hold?.id;
		hold = null;
		picked = null;
		error = null;
		if (id) await api.cancelBooking(id).catch(() => {});
	}
</script>

<svelte:head>
	<title>{arena.name}, {arena.area} — book by the hour | Khel Arena</title>
	<meta name="description" content={arena.description} />
</svelte:head>

<section class="masthead forest-band">
	<div class="shell">
		<nav class="crumbs small fade-up" aria-label="Breadcrumb">
			<a class="link" href="/arenas">Arenas</a>
			<span aria-hidden="true">›</span>
			<span>{arena.area}, {arena.city}</span>
		</nav>

		<div class="title fade-up">
			<h1 class="display display-xl">{arena.name}</h1>
			<span class="rating num">
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
				<ul class="hours">
					{#each grid.slots as s, i (s.starts_at)}
						<li style="--i: {Math.min(i, 10)}">
							{#if s.available}
								<button
									type="button"
									class="hour"
									class:peak={s.is_peak}
									class:on={picked === s.starts_at}
									aria-pressed={picked === s.starts_at}
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
				{error}
				signedIn={session.signedIn}
				onhold={takeHold}
				onpay={pay}
				onrelease={release}
			/>
		</div>
	</div>
</section>

<style>
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
