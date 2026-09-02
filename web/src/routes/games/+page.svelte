<script>
	import { goto } from '$app/navigation';
	import { SPORT_LABELS } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import CallRecord from '$lib/components/CallRecord.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import Listbox from '$lib/components/Listbox.svelte';
	import { reveal } from '$lib/actions/reveal.js';
	import { SKILL_LABELS } from '$lib/skills.js';

	let { data } = $props();

	const spots = $derived(data.calls.reduce((n, c) => n + c.spots_remaining, 0));

	const skillOptions = $derived([
		{ value: 'all', label: 'Any standard' },
		...data.skills.map((s) => ({ value: s, label: SKILL_LABELS[s] ?? s }))
	]);

	const areaOptions = $derived([
		{ value: 'all', label: 'Anywhere in the valley' },
		...data.areas.map((a) => ({ value: a, label: a }))
	]);

	function set(changes) {
		const next = { skill: data.skill, area: data.area, ...changes };
		goto(`/games?skill=${next.skill}&area=${encodeURIComponent(next.area)}`, {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}
</script>

<svelte:head>
	<title>Games looking for players | Khel Arena</title>
	<meta
		name="description"
		content="Pickup games around Kathmandu that are short of players, and courts already booked with spaces going spare."
	/>
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">The call sheet</p>
		<h1 class="display display-l"><TextAnimate text="Someone is a man short" /></h1>
		<p class="lede fade-up fade-up-1">
			{#if spots > 0}
				<strong class="num">{spots}</strong>
				{spots === 1 ? 'place' : 'places'} going in
				<strong class="num">{data.calls.length}</strong>
				{data.calls.length === 1 ? 'game' : 'games'}. Ask to join; the person who posted decides.
			{:else}
				Nothing on the sheet right now. Post a game and the valley will see it.
			{/if}
		</p>
		<div class="pitch-mark fade-up fade-up-2">
			<CentreMark wide />
		</div>
	</div>
</section>

<section class="board">
	<div class="shell">
		<div class="controls">
			<div class="filters">
				<Listbox
					value={data.skill}
					options={skillOptions}
					label="Standard"
					onselect={(skill) => set({ skill })}
				/>
				<Listbox
					value={data.area}
					options={areaOptions}
					label="Area"
					onselect={(area) => set({ area })}
				/>
			</div>

			{#if session.signedIn}
				<a class="btn btn-primary" href="/games/new">Post a game</a>
			{:else}
				<a class="btn btn-secondary" href="/login">Sign in to post</a>
			{/if}
		</div>

		{#if data.calls.length}
			<ul>
				{#each data.calls as call, i (call.id)}
					<li use:reveal={{ delay: Math.min(i, 6) * 60 }}><CallRecord {call} /></li>
				{/each}
			</ul>
		{:else}
			<p class="empty prose">
				No games are short of players in {data.area === 'all' ? 'the valley' : data.area} right now.
				<a class="link" href="/tonight">Book a court</a> and post the spare places.
			</p>
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

	.board {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	/* The filters and the one action sit on the same line until there is no
	   room, at which point the action drops beneath rather than shrinking. */
	.controls {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
		align-items: center;
		justify-content: space-between;
		margin-bottom: clamp(1.5rem, 3vw, 2.25rem);
	}

	.filters {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
	}

	ul {
		display: grid;
		gap: 1rem;
	}

	.empty {
		padding: clamp(2rem, 5vw, 3rem);
		border: 1px solid var(--line);
		border-radius: var(--r-lg);
		background: var(--surface);
		text-align: center;
		color: var(--muted);
	}
</style>
