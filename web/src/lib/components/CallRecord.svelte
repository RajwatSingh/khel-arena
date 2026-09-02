<script>
	/**
	 * One game on the call sheet.
	 *
	 * Built as a record rather than a card, matching ArenaRecord: a hairline
	 * rule, the thing itself on the left, the number that matters on the
	 * right. What matters here is how many places are left, so that is the
	 * figure the eye lands on.
	 */
	import { formatDateLong, formatTime } from '$lib/time.js';
	import { SKILL_LABELS } from '$lib/skills.js';
	import MiddleTruncation from './MiddleTruncation.svelte';

	let { call } = $props();

	const where = $derived(
		call.arena_name ? `${call.arena_name}${call.arena_area ? `, ${call.arena_area}` : ''}` : 'Venue to be agreed'
	);

	// A booked court is a different proposition from a pickup game nobody has
	// paid for yet, and the difference is worth saying on the row.
	const booked = $derived(Boolean(call.booking_id));
</script>

<a class="record" href="/games/{call.id}">
	<div class="line-1">
		<div class="who">
			<h3 class="display display-m">{call.title}</h3>
			<p class="where small"><MiddleTruncation class="venue" text={where} /></p>
		</div>

		<span class="spots num" class:last={call.spots_remaining === 1}>
			{call.spots_remaining}
			<span class="spots-label label">{call.spots_remaining === 1 ? 'place' : 'places'}</span>
		</span>
	</div>

	<div class="line-2">
		<span class="when num">{formatDateLong(call.starts_at)} · {formatTime(call.starts_at)}</span>
		<span class="chips">
			<span class="chip">{SKILL_LABELS[call.skill] ?? call.skill}</span>
			{#if booked}
				<span class="chip chip-court">Court booked</span>
			{/if}
			<span class="chip chip-quiet">{call.author.username}</span>
		</span>
	</div>
</a>

<style>
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

	.who {
		min-width: 0;
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

	/* The number is the point of the row: how many places are left. */
	.spots {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex: none;
		font-size: clamp(1.6rem, 3vw, 2.1rem);
		line-height: 1;
		color: var(--pine-deep);
	}

	.spots.last {
		color: var(--brick);
	}

	.spots-label {
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

	.when {
		color: var(--muted);
		font-size: 0.9rem;
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

	.chip-court {
		background: var(--sand);
		color: #7a3f22;
	}

	.chip-quiet {
		background: var(--surface-sunk);
		color: var(--faint);
	}
</style>
