<script>
	import HourLedger from '$lib/components/HourLedger.svelte';
	import HourPulse from '$lib/components/HourPulse.svelte';
	import ArenaRecord from '$lib/components/ArenaRecord.svelte';
	import AvailabilitySearch from '$lib/components/AvailabilitySearch.svelte';
	import { formatDateLong, formatNPR, formatTime } from '$lib/time.js';

	let { data } = $props();

	const freeTonight = $derived(
		data.ledger.rows.reduce(
			(total, row) =>
				total +
				row.slots.filter((s) => s.available && Number(formatTime(s.starts_at).slice(0, 2)) >= 17)
					.length,
			0
		)
	);

	// Counts up from zero on mount, once, as the header's own small piece of
	// motion. Falls back to the real number whenever there's no animated
	// value yet — SSR, no JS, or reduced motion never show a zero.
	let animatedCount = $state(null);

	$effect(() => {
		const target = freeTonight;
		const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

		if (reduceMotion) {
			animatedCount = null;
			return;
		}

		animatedCount = 0;
		let raf;
		const start = performance.now();
		const duration = 800;

		function tick(now) {
			const t = Math.min(1, (now - start) / duration);
			const eased = 1 - Math.pow(1 - t, 3);
			animatedCount = t < 1 ? Math.round(target * eased) : null;
			if (t < 1) raf = requestAnimationFrame(tick);
		}

		raf = requestAnimationFrame(tick);
		return () => cancelAnimationFrame(raf);
	});

	const steps = [
		{
			at: 'Right away',
			title: 'Hold the hour',
			body: 'One tap puts the hour in your name. Nothing is charged yet, and nobody else can take it while you hold it.'
		},
		{
			at: 'Within 15 minutes',
			title: 'Pay, or lose it',
			body: 'eSewa, Khalti, or cash reserved at the arena. Miss the window and the hour goes back on the board for someone else.'
		},
		{
			at: 'At kick-off',
			title: 'Show the reference',
			body: 'Read the four digits out at the gate. Cancel any time before you are due on and the hour is released.'
		}
	];
</script>

<svelte:head>
	<title>Khel Arena — book a court by the hour in Kathmandu</title>
	<meta
		name="description"
		content="Live futsal, basketball and badminton availability across Kathmandu and Lalitpur. Hold an hour for fifteen minutes, pay with eSewa or Khalti, play."
	/>
</svelte:head>

<section class="hero">
	<div class="shell">
		<p class="label live">Kathmandu &amp; Lalitpur · {formatDateLong(data.date)}</p>

		{#if freeTonight > 0}
			<h1 class="display display-xl">
				<span class="count num">{animatedCount ?? freeTonight}</span> court-hours are still free
				tonight.
			</h1>
		{:else}
			<h1 class="display display-xl">Tonight is played out.</h1>
		{/if}

		<HourPulse />

		<p class="lede">
			{#if freeTonight > 0}
				An hour nobody claims doesn't go on sale — it just goes away. Tap one, give your five
				fifteen minutes to say yes, then pay to make it yours.
			{:else}
				Every evening hour in the valley is taken or done. Tomorrow is one tap away, and the dawn
				slots are still open.
			{/if}
		</p>

		<div class="search-wrap">
			<AvailabilitySearch
				dates={data.dates}
				areas={data.areas}
				sports={['futsal', 'basketball', 'badminton', 'cricket_net']}
			/>
		</div>

		{#if data.ledger.cheapest_npr}
			<p class="from small">
				Cheapest free hour today is <strong class="num"
					>NPR {formatNPR(data.ledger.cheapest_npr)}</strong
				>. No booking fee — the arena's price is the price.
			</p>
		{/if}
	</div>
</section>

<section class="board">
	<div class="shell">
		<header class="section-head">
			<div>
				<h2 class="display display-l">Every free hour today</h2>
				<p class="lede">
					Futsal courts across the valley, hour by hour. Tap a price to hold that hour.
				</p>
			</div>
			<a class="btn btn-secondary" href="/tonight">See the whole week</a>
		</header>

		<HourLedger ledger={data.ledger} />
	</div>
</section>

<section class="band how">
	<div class="shell">
		<header class="section-head">
			<div>
				<h2 class="display display-l">Fifteen minutes to get your five to answer</h2>
				<p class="lede">
					A held hour is real — the court leaves the board the moment you tap it. It is also
					temporary, which is the only reason anyone can trust the board at all.
				</p>
			</div>
		</header>

		<ol class="steps">
			{#each steps as step (step.at)}
				<li class="card">
					<p class="when">{step.at}</p>
					<h3 class="display display-m">{step.title}</h3>
					<p class="small">{step.body}</p>
				</li>
			{/each}
		</ol>
	</div>
</section>

<section class="band arenas-band">
	<div class="shell">
		<header class="section-head">
			<div>
				<h2 class="display display-l">Four arenas, nine courts</h2>
				<p class="lede">
					Every venue you can book here, with its own hours and its own rates.
				</p>
			</div>
			<a class="btn btn-secondary" href="/arenas">All arenas</a>
		</header>

		<ul class="arenas">
			{#each data.arenas as arena (arena.id)}
				<li><ArenaRecord {arena} /></li>
			{/each}
		</ul>
	</div>
</section>

<section class="band">
	<div class="shell">
		<div class="promises">
			<div class="promise card">
				<h2 class="display display-l">Two teams can never be sold the same hour</h2>
				<div class="prose">
					<p>
						Double-booking is not something we check for and apologise about afterwards. The
						database physically cannot hold two live bookings on one court at one time — not under
						load, not on a retry, not when twenty people tap the same eight o'clock slot in the
						same second.
					</p>
					<p>
						Exactly one of them gets it. The other nineteen are told straight away and can pick
						again, rather than finding out at the gate with nine people and a ball.
					</p>
				</div>
				<a class="btn btn-primary" href="/tonight">Find an hour</a>
			</div>

			<div class="promise-light card">
				<h2 class="display display-m">The arena's price is the price</h2>
				<div class="prose">
					<p>
						No booking fee, and nothing added for being the last hour left on the board. eSewa,
						Khalti, or cash at the gate — the number on the cell is the number you pay, whether you
						book at noon or with four minutes left on your hold.
					</p>
				</div>
			</div>
		</div>
	</div>
</section>

<style>
	.hero {
		padding-block: clamp(3rem, 6vw, 5.5rem) clamp(2.5rem, 5vw, 4rem);
	}

	.label.live {
		display: inline-flex;
		align-items: center;
		gap: 0.55rem;
	}

	.label.live::before {
		content: '';
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--pine);
		animation: pulse 2.4s ease-in-out infinite;
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
			box-shadow: 0 0 0 0 color-mix(in srgb, var(--pine) 35%, transparent);
		}
		50% {
			opacity: 0.55;
			box-shadow: 0 0 0 4px color-mix(in srgb, var(--pine) 0%, transparent);
		}
	}

	h1 {
		margin-top: 1.1rem;
		max-width: 18ch;
	}

	.count {
		color: var(--pine);
		font-weight: 600;
	}

	:global(.hero .hourpulse) {
		margin-top: 1.6rem;
		max-width: 34rem;
	}

	.hero .lede {
		margin-top: 1.6rem;
		max-width: 52ch;
	}

	.search-wrap {
		margin-top: 2.25rem;
		max-width: 60rem;
	}

	.from {
		margin-top: 1.1rem;
		color: var(--muted);
	}

	.from strong {
		color: var(--ink);
		font-weight: 600;
	}

	.board {
		padding-bottom: var(--band);
	}

	.section-head {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1.5rem 2rem;
		margin-bottom: 2rem;
		flex-wrap: wrap;
	}

	.section-head .lede {
		margin-top: 0.75rem;
	}

	.section-head .btn {
		flex-shrink: 0;
	}

	/* --------------------------------------------------------------- how --- */

	.how {
		background: var(--surface);
		border-block: 1px solid var(--line);
	}

	.steps {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(17rem, 1fr));
		gap: 1rem;
	}

	.steps li {
		padding: 1.6rem;
		background: var(--field);
		border-color: transparent;
	}

	/* The markers are the clock, not 01/02/03 — the hold window is the content. */
	.when {
		display: inline-block;
		margin-bottom: 0.9rem;
		padding: 0.25rem 0.7rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine);
		font-size: 0.8125rem;
		font-weight: 600;
	}

	.steps h3 {
		margin-bottom: 0.5rem;
	}

	.steps p {
		color: var(--muted);
	}

	/* ------------------------------------------------------------ arenas --- */

	.arenas {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(23rem, 1fr));
		gap: 1rem;
	}

	/* ----------------------------------------------------------- promise --- */

	.promises {
		display: grid;
		grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr);
		gap: 1rem;
		align-items: stretch;
	}

	.promise,
	.promise-light {
		display: flex;
		flex-direction: column;
		padding: clamp(1.75rem, 4vw, 3.5rem);
	}

	.promise {
		background: var(--pine);
		border-color: var(--pine);
		color: #ffffff;
	}

	.promise h2 {
		color: #ffffff;
		max-width: 18ch;
	}

	.promise .prose {
		margin-top: 1.5rem;
		color: rgba(255, 255, 255, 0.82);
		font-size: 1.0625rem;
	}

	.promise .btn {
		margin-top: 2rem;
		align-self: flex-start;
		background: #ffffff;
		color: var(--pine);
	}

	.promise .btn:hover {
		background: var(--sand);
	}

	/* The pricing card borrows the ledger's peak-rate tint — sand already
	   means "money" on this site, so reusing it here is not decoration. */
	.promise-light {
		justify-content: center;
		background: var(--sand);
		border-color: var(--sand-line);
	}

	.promise-light h2 {
		max-width: 16ch;
	}

	.promise-light .prose {
		margin-top: 1rem;
	}

	@media (max-width: 780px) {
		.promises {
			grid-template-columns: minmax(0, 1fr);
		}
	}

	@media (max-width: 560px) {
		.arenas {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
