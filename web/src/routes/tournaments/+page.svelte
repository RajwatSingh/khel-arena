<script>
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { reveal } from '$lib/actions/reveal.js';
	import { formatDateLong, formatNPR } from '$lib/time.js';
	import { SKILL_LABELS } from '$lib/skills.js';

	let { data } = $props();

	const open = $derived(data.tournaments.filter((t) => t.status === 'open').length);
</script>

<svelte:head>
	<title>Tournaments | Khel Arena</title>
	<meta
		name="description"
		content="Futsal tournaments around Kathmandu — entry fees, prize pools and how many slots are left."
	/>
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">The fixture list</p>
		<h1 class="display display-l"><TextAnimate text="Cups and leagues" /></h1>
		<p class="lede fade-up fade-up-1">
			{#if data.tournaments.length}
				<strong class="num">{open}</strong> still taking entries. A captain enters the squad; the
				organiser confirms the fee.
			{:else}
				Nothing running right now.
			{/if}
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="list">
	<div class="shell">
		{#if data.tournaments.length}
			<ul>
				{#each data.tournaments as t, i (t.id)}
					<li use:reveal={{ delay: Math.min(i, 6) * 60 }}>
						<a class="record" href="/tournaments/{t.slug}">
							<div class="line-1">
								<div class="who">
									<h3 class="display display-m">{t.name}</h3>
									<p class="where small">
										{formatDateLong(t.starts_on)}
										{#if t.arena_name}· {t.arena_name}, {t.arena_area}{/if}
									</p>
								</div>
								<span class="slots num" class:full={t.slots_remaining === 0}>
									{t.slots_remaining}
									<span class="slots-label label">
										{t.slots_remaining === 1 ? 'slot' : 'slots'}
									</span>
								</span>
							</div>

							<div class="line-2">
								<span class="chips">
									<span class="chip">{t.format}</span>
									<span class="chip">{t.side_count}-a-side</span>
									<span class="chip">{SKILL_LABELS[t.skill] ?? t.skill}</span>
									{#if t.status !== 'open'}<span class="chip chip-quiet">{t.status}</span>{/if}
								</span>
								<span class="money num">
									{#if t.entry_fee_npr}Entry {formatNPR(t.entry_fee_npr)}{:else}Free entry{/if}
									{#if t.prize_pool_npr}· Pot {formatNPR(t.prize_pool_npr)}{/if}
								</span>
							</div>
						</a>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="empty prose">No tournaments are listed yet.</p>
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

	.list {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	ul {
		display: grid;
	}

	.record {
		display: grid;
		gap: 0.85rem;
		padding: clamp(1.15rem, 2.5vw, 1.6rem) 0;
		border-top: 1px solid var(--line);
		color: inherit;
		text-decoration: none;
		transition: border-color var(--dur-hover) var(--ease);
	}

	.record:hover,
	.record:focus-visible {
		border-top-color: var(--pine);
	}

	.line-1 {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1.5rem;
	}

	h3 {
		margin: 0;
	}

	.record:hover h3 {
		color: var(--pine-deep);
	}

	.where {
		margin: 0.2rem 0 0;
		color: var(--faint);
	}

	.slots {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex: none;
		font-size: clamp(1.6rem, 3vw, 2.1rem);
		line-height: 1;
		color: var(--pine-deep);
	}

	.slots.full {
		color: var(--faint);
	}

	.slots-label {
		margin-top: 0.25rem;
		color: var(--faint);
	}

	.line-2 {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}

	.chip {
		padding: 0.2rem 0.6rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.78rem;
	}

	.chip-quiet {
		background: var(--surface-sunk);
		color: var(--faint);
	}

	.money {
		color: var(--muted);
		font-size: 0.9rem;
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
