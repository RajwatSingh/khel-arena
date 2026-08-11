<script>
	/**
	 * Purely ambient. A row of hours draining and refilling — the same argument
	 * as the ledger below ("not discounted, gone"), made in motion instead of
	 * type, before the real board ever loads. No data in it; the sr-only label
	 * says so.
	 */
	const bars = Array.from({ length: 16 });
</script>

<div class="hourpulse" role="img" aria-label="An evening of court-hours, filling and running out">
	{#each bars as _, i (i)}
		<span class="bar" style="--i: {i}"></span>
	{/each}
</div>

<style>
	.hourpulse {
		display: flex;
		align-items: flex-end;
		gap: clamp(3px, 0.6vw, 6px);
		height: clamp(2rem, 4vw, 2.75rem);
	}

	.bar {
		flex: 1;
		min-width: 0;
		height: 100%;
		border-radius: 3px 3px 1px 1px;
		background: var(--accent-on-field);
		transform-origin: bottom;
		animation: drain 7.2s var(--ease) infinite;
		animation-delay: calc(var(--i) * -0.42s);
	}

	@keyframes drain {
		0%,
		44% {
			height: 100%;
			background: var(--accent-on-field);
		}
		56% {
			height: 100%;
			background: var(--sand-line);
		}
		70%,
		88% {
			height: 24%;
			background: var(--surface-sunk);
		}
		100% {
			height: 100%;
			background: var(--accent-on-field);
		}
	}
</style>
