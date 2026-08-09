<script>
	import HourLedger from '$lib/components/HourLedger.svelte';
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
		<p class="label">Kathmandu &amp; Lalitpur · {formatDateLong(data.date)}</p>

		{#if freeTonight > 0}
			<h1 class="display display-xl">
				<span class="count num">{freeTonight}</span> court-hours are still free tonight.
			</h1>
		{:else}
			<h1 class="display display-xl">Tonight is played out.</h1>
		{/if}

		<p class="lede">
			{#if freeTonight > 0}
				They are not discounted at nine — they are gone. Pick an hour, hold it for fifteen minutes
				while your five confirm, then pay.
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
		<div class="promise card">
			<h2 class="display display-l">Two teams can never be sold the same hour</h2>
			<div class="prose">
				<p>
					Double-booking is not something we check for and apologise about afterwards. The database
					physically cannot hold two live bookings on one court at one time — not under load, not
					on a retry, not when twenty people tap the same eight o'clock slot in the same second.
				</p>
				<p>
					Exactly one of them gets it. The other nineteen are told straight away and can pick
					again, rather than finding out at the gate with nine people and a ball.
				</p>
			</div>
			<a class="btn btn-primary" href="/tonight">Find an hour</a>
		</div>
	</div>
</section>

<style>
	.hero {
		padding-block: clamp(3rem, 6vw, 5.5rem) clamp(2.5rem, 5vw, 4rem);
	}

	h1 {
		margin-top: 1.1rem;
		max-width: 18ch;
	}

	.count {
		color: var(--pine);
		font-weight: 600;
	}

	.hero .lede {
		margin-top: 1.4rem;
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

	.promise {
		padding: clamp(2rem, 5vw, 3.5rem);
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
		background: #ffffff;
		color: var(--pine);
	}

	.promise .btn:hover {
		background: var(--sand);
	}

	@media (max-width: 560px) {
		.arenas {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
