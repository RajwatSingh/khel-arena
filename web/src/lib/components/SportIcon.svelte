<script>
	/**
	 * One mark per sport, drawn in the same stroke-only, no-fill language as
	 * the centre-mark glyph — a seamed ball, a basketball's panel lines, a
	 * shuttlecock's skirt, a set of stumps.
	 */
	let { sport, size = 18 } = $props();
</script>

<svg style="width: {size}px; height: {size}px" viewBox="0 0 24 24" aria-hidden="true">
	{#if sport === 'basketball'}
		<!-- Ball over a backboard and net, the way the hoop reads from courtside. -->
		<rect x="5" y="3" width="14" height="8" rx="1.2" />
		<circle cx="12" cy="6.5" r="3" />
		<path d="M12 3.5v6M9.3 6.5Q12 4.7 14.7 6.5" />
		<path d="M6.5 11 9 18M17.5 11 15 18M9 18 15 11M15 18 9 11M9 18h6" />
	{:else if sport === 'badminton'}
		<!-- A cork and the flared skirt of feathers above it. -->
		<circle class="dot" cx="12" cy="19.3" r="1.4" />
		<path d="M12 18 5.5 4M12 18 8.5 3M12 18 12 2.6M12 18 15.5 3M12 18 18.5 4" />
		<path d="M5.5 4Q12 1.4 18.5 4" />
	{:else if sport === 'cricket_net'}
		<!-- Two bats crossed, the ball resting where they meet. -->
		<g transform="rotate(-32 12 12)">
			<rect x="9.6" y="2" width="4.8" height="7.2" rx="1.6" />
			<line x1="12" y1="9.2" x2="12" y2="20" />
		</g>
		<g transform="rotate(32 12 12)">
			<rect x="9.6" y="2" width="4.8" height="7.2" rx="1.6" />
			<line x1="12" y1="9.2" x2="12" y2="20" />
		</g>
		<circle class="dot" cx="12" cy="16.6" r="1.7" />
	{:else}
		<circle cx="12" cy="12" r="9" />
		<path d="M12 6.5 8.2 9.2l1.4 4.5h4.8l1.4-4.5z" />
		<path d="M12 6.5V3.7M8.2 9.2 5.7 7.4M9.6 13.7 8 18M14.4 13.7 16 18M15.8 9.2l2.5-1.8" />
	{/if}
</svg>

<style>
	svg {
		display: block;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.4;
		stroke-linecap: round;
		stroke-linejoin: round;
		vector-effect: non-scaling-stroke;
	}

	.dot {
		fill: currentColor;
		stroke: none;
	}
</style>
