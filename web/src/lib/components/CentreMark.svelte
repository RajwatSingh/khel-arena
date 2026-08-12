<script>
	/**
	 * The centre mark of a futsal court: halfway line, centre circle, spot.
	 * It is the identity glyph in the masthead and, drawn wide, the rule
	 * between sections. Nothing else in the interface is decorative.
	 */
	let { width = 20, wide = false, label = '' } = $props();
</script>

{#if wide}
	<div class="halfway">
		<svg class="wide" viewBox="0 0 1200 24" preserveAspectRatio="none" aria-hidden="true">
			<line x1="0" y1="12" x2="560" y2="12" />
			<line x1="640" y1="12" x2="1200" y2="12" />
		</svg>
		<svg class="badge" viewBox="0 0 40 24" aria-hidden="true">
			<circle cx="20" cy="12" r="10" />
			<line x1="20" y1="0" x2="20" y2="24" />
			<circle class="spot" cx="20" cy="12" r="1.6" />
		</svg>
	</div>
{:else}
	<svg
		style="width: {width}px"
		viewBox="0 0 24 24"
		role={label ? 'img' : 'presentation'}
		aria-label={label || undefined}
	>
		<circle cx="12" cy="12" r="9" />
		<line x1="12" y1="1" x2="12" y2="23" />
		<circle class="spot" cx="12" cy="12" r="1.5" />
	</svg>
{/if}

<style>
	svg {
		overflow: visible;
		fill: none;
		stroke: currentColor;
		stroke-width: 1;
		vector-effect: non-scaling-stroke;
	}

	.spot {
		fill: currentColor;
		stroke: none;
	}

	.halfway {
		position: relative;
		height: 24px;
	}

	.wide {
		width: 100%;
		height: 24px;
		color: inherit;
	}

	/* Marked out like a pitch would be: the halfway line first, then the
	   circle traced round, the spot dropped last — quick enough to read as
	   a flourish, not a loading state. */
	.wide line {
		stroke-dasharray: 560;
		stroke-dashoffset: 560;
		animation: draw 0.9s var(--ease) forwards;
	}

	.wide line:last-child {
		animation-delay: 0.12s;
	}

	.badge {
		position: absolute;
		left: 50%;
		top: 0;
		transform: translateX(-50%);
		width: 40px;
		height: 24px;
		color: inherit;
	}

	.badge circle:not(.spot) {
		stroke-dasharray: 63;
		stroke-dashoffset: 63;
		animation: draw 0.6s var(--ease) 0.45s forwards;
	}

	.badge line {
		stroke-dasharray: 24;
		stroke-dashoffset: 24;
		animation: draw 0.35s var(--ease) 0.85s forwards;
	}

	.badge .spot {
		opacity: 0;
		animation: dot-in 0.25s var(--ease) 1.05s forwards;
	}

	@keyframes draw {
		to {
			stroke-dashoffset: 0;
		}
	}

	@keyframes dot-in {
		to {
			opacity: 1;
		}
	}
</style>
