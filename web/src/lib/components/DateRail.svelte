<script>
	import { formatDate } from '$lib/time.js';

	/** Seven days forward. Arenas do not take bookings further out than that. */
	let { dates, selected, onselect } = $props();
</script>

<div class="rail" role="group" aria-label="Choose a date">
	{#each dates as date, i (date)}
		<button
			type="button"
			class="day"
			class:on={date === selected}
			aria-pressed={date === selected}
			onclick={() => onselect(date)}
		>
			<span class="when">{i === 0 ? 'Today' : i === 1 ? 'Tomorrow' : formatDate(date).slice(0, 3)}</span>
			<span class="num date">{formatDate(date).slice(4)}</span>
		</button>
	{/each}
</div>

<style>
	.rail {
		display: flex;
		gap: 0.5rem;
		overflow-x: auto;
		padding-bottom: 0.25rem;
		scrollbar-width: thin;
	}

	.day {
		flex: 1 0 auto;
		min-width: 6.5rem;
		padding: 0.7rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
		text-align: left;
		cursor: pointer;
		transition:
			border-color var(--dur-hover) var(--ease),
			background-color var(--dur-hover) var(--ease),
			color var(--dur-hover) var(--ease),
			transform calc(var(--dur-hover) * 0.7) var(--ease);
	}

	.day:hover:not(.on) {
		border-color: var(--line-strong);
	}

	.day:active {
		transform: scale(0.96);
	}

	.day.on {
		background: var(--pine);
		border-color: var(--pine);
		color: #ffffff;
		animation: settle-in 0.34s var(--ease-reveal);
	}

	@keyframes settle-in {
		from {
			transform: scale(0.94);
			filter: blur(4px);
		}
	}

	.when {
		display: block;
		font-size: 0.9375rem;
		font-weight: 600;
		letter-spacing: -0.01em;
	}

	.date {
		display: block;
		margin-top: 0.05rem;
		font-size: 0.8125rem;
		color: var(--faint);
	}

	.day.on .date {
		color: rgba(255, 255, 255, 0.75);
	}
</style>
