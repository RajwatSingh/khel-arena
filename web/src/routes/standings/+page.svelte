<script>
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { reveal } from '$lib/actions/reveal.js';

	let { data } = $props();

	const played = $derived(data.standings.reduce((n, s) => n + s.played, 0) / 2);
</script>

<svelte:head>
	<title>The table | Khel Arena</title>
	<meta
		name="description"
		content="Team standings from results both captains agreed: three points for a win, one for a draw."
	/>
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">The table</p>
		<h1 class="display display-l"><TextAnimate text="Who is actually any good" /></h1>
		<p class="lede fade-up fade-up-1">
			{#if data.standings.length}
				<strong class="num">{played}</strong>
				{played === 1 ? 'match' : 'matches'}, counted only where both captains agreed the score.
				Three points for a win, one for a draw, goal difference splits the rest.
			{:else}
				Nothing here yet. A result counts once both captains have confirmed it.
			{/if}
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="table-wrap">
	<div class="shell">
		{#if data.standings.length}
			<div class="scroller" use:reveal>
				<table>
					<caption class="sr-only">Team standings</caption>
					<thead>
						<tr>
							<th scope="col" class="pos">#</th>
							<th scope="col">Team</th>
							<th scope="col" class="num">P</th>
							<th scope="col" class="num">W</th>
							<th scope="col" class="num">D</th>
							<th scope="col" class="num">L</th>
							<th scope="col" class="num">GF</th>
							<th scope="col" class="num">GA</th>
							<th scope="col" class="num">GD</th>
							<th scope="col" class="num pts">Pts</th>
						</tr>
					</thead>
					<tbody>
						{#each data.standings as row (row.team_id)}
							<tr>
								<td class="pos num">{row.rank}</td>
								<th scope="row">
									<a href="/teams/{row.team_id}">
										<span class="tag num">{row.tag}</span>
										{row.name}
									</a>
								</th>
								<td class="num">{row.played}</td>
								<td class="num">{row.won}</td>
								<td class="num">{row.drawn}</td>
								<td class="num">{row.lost}</td>
								<td class="num">{row.goals_for}</td>
								<td class="num">{row.goals_against}</td>
								<td class="num">{row.goal_diff > 0 ? '+' : ''}{row.goal_diff}</td>
								<td class="num pts">{row.points}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<p class="empty prose">
				No agreed results yet. Report one from your team's page — the other captain confirms it,
				and then it counts.
			</p>
		{/if}
	</div>
</section>

<style>
	h1 { margin-top: 0.6rem; }
	.head .lede { margin-top: 1rem; max-width: 54ch; }
	.table-wrap { padding-block: clamp(2rem, 4vw, 3rem) var(--band); }

	/* A league table is wide and a phone is not: the table scrolls inside its
	   own box rather than making the page scroll sideways. */
	.scroller {
		overflow-x: auto;
		border: 1px solid var(--line);
		border-radius: var(--r-lg);
		background: var(--surface);
	}

	table { width: 100%; border-collapse: collapse; }

	th, td {
		padding: 0.7rem 0.75rem;
		text-align: right;
		white-space: nowrap;
	}

	thead th {
		border-bottom: 1px solid var(--line);
		font-size: 0.78rem;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--faint);
	}

	tbody th { text-align: left; font-weight: 500; }
	tbody tr + tr th, tbody tr + tr td { border-top: 1px solid var(--line); }
	tbody tr:hover { background: var(--field); }

	tbody th a {
		display: inline-flex;
		align-items: center;
		gap: 0.6rem;
		color: inherit;
		text-decoration: none;
	}

	tbody th a:hover { color: var(--pine-deep); }

	.tag {
		display: grid;
		place-items: center;
		width: 2.6rem;
		padding: 0.15rem 0;
		border-radius: var(--r-sm);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.72rem;
		letter-spacing: 0.04em;
	}

	.pos { width: 2.5rem; text-align: left; color: var(--faint); }
	.pts { font-weight: 600; color: var(--ink); }

	.empty {
		padding: clamp(2rem, 5vw, 3rem);
		border: 1px solid var(--line);
		border-radius: var(--r-lg);
		background: var(--surface);
		text-align: center;
		color: var(--muted);
	}
</style>
