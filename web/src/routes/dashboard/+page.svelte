<script>
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { formatNPR } from '$lib/time.js';
	import { reveal } from '$lib/actions/reveal.js';

	let { data } = $props();

	const hasVenues = $derived(data.venues.length > 0);
</script>

<svelte:head>
	<title>Dashboard | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">Venue operator</p>
		<h1 class="display display-l"><TextAnimate text="Your dashboard" /></h1>
		<p class="lede fade-up fade-up-1">
			Every court you run, the bookings they've taken and the money that came in — across all your
			venues at once.
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="body">
	<div class="shell">
		{#if hasVenues}
			<ul class="stats">
				<li use:reveal>
					<span class="num big">{formatNPR(data.totals.earned_npr)}</span>
					<span class="label">Earnings, confirmed</span>
				</li>
				<li use:reveal={{ delay: 60 }}>
					<span class="num big">{data.totals.paid_bookings}</span>
					<span class="label">Bookings paid</span>
				</li>
				<li use:reveal={{ delay: 120 }}>
					<span class="num big">{data.totals.awaiting_cash_count}</span>
					<span class="label">
						Cash to confirm{#if data.totals.awaiting_cash_npr}
							· {formatNPR(data.totals.awaiting_cash_npr)}{/if}
					</span>
				</li>
			</ul>

			<h2 class="display display-m section-h">By venue</h2>
			<ul class="venues">
				{#each data.venues as venue, i (venue.id)}
					<li use:reveal={{ delay: Math.min(i, 6) * 60 }}>
						<a class="record" href="/manage/{venue.id}">
							<div class="who">
								<h3 class="display display-m">
									{venue.name}
									{#if !venue.is_active}<span class="closed label">closed</span>{/if}
								</h3>
								<p class="small">
									{venue.area}, {venue.city} · {venue.court_count}
									{venue.court_count === 1 ? 'court' : 'courts'}
								</p>
								{#if venue.awaiting_cash_count}
									<p class="small owed">
										{venue.awaiting_cash_count} cash booking{venue.awaiting_cash_count === 1
											? ''
											: 's'} waiting on you
									</p>
								{/if}
							</div>
							<div class="right">
								<span class="num earned">{formatNPR(venue.earned_npr)}</span>
								<span class="label">{venue.paid_bookings} paid</span>
							</div>
						</a>
					</li>
				{/each}
			</ul>

			<p class="small quiet more">
				Hours, courts, rates and the cash till for a venue live on
				<a class="link" href="/manage">its management page</a>.
			</p>
		{:else}
			<div class="empty">
				<p class="prose">
					No venue listed yet. Register your first futsal and this dashboard fills in as the
					bookings come.
				</p>
				<a class="btn btn-primary" href="/manage">List a venue</a>
			</div>
		{/if}
	</div>
</section>

<style>
	h1 {
		margin-top: 0.6rem;
	}
	.head .lede {
		margin-top: 1rem;
		max-width: 54ch;
	}
	.body {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(100%, 14rem), 1fr));
		gap: clamp(1rem, 3vw, 1.5rem);
		margin-bottom: clamp(2.5rem, 5vw, 3.5rem);
	}

	.stats li {
		display: grid;
		gap: 0.4rem;
		padding: clamp(1.25rem, 3vw, 1.75rem);
		border: 1px solid var(--line);
		border-radius: var(--r-lg);
		background: var(--surface);
	}

	.big {
		font-size: clamp(1.9rem, 4vw, 2.6rem);
		line-height: 1;
		color: var(--pine-deep);
	}

	.stats .label,
	.right .label {
		color: var(--faint);
	}

	.section-h {
		margin: 0 0 0.5rem;
	}

	.venues {
		display: grid;
	}

	.record {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1.5rem;
		padding: clamp(1rem, 2.5vw, 1.4rem) 0;
		border-top: 1px solid var(--line);
		color: inherit;
		text-decoration: none;
		transition: border-color var(--dur-hover) var(--ease);
	}

	.record:hover,
	.record:focus-visible {
		border-top-color: var(--pine);
	}

	h3 {
		margin: 0;
	}
	.record:hover h3 {
		color: var(--pine-deep);
	}
	.who .small {
		margin: 0.2rem 0 0;
		color: var(--faint);
	}
	.who .owed {
		color: var(--brick);
	}

	.closed {
		display: inline-block;
		margin-left: 0.5rem;
		padding: 0.15rem 0.5rem;
		border-radius: var(--r-pill);
		background: var(--surface-sunk);
		color: var(--faint);
		vertical-align: middle;
	}

	.right {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex: none;
	}

	.earned {
		font-size: clamp(1.4rem, 3vw, 1.9rem);
		line-height: 1;
		color: var(--pine-deep);
	}

	.right .label {
		margin-top: 0.2rem;
	}

	.more {
		margin-top: clamp(1.5rem, 4vw, 2.5rem);
	}
	.quiet {
		color: var(--muted);
	}

	.empty {
		display: grid;
		justify-items: center;
		gap: 1.25rem;
		padding: clamp(2rem, 5vw, 3rem);
		border: 1px solid var(--line);
		border-radius: var(--r-lg);
		background: var(--surface);
		text-align: center;
		color: var(--muted);
	}
</style>
