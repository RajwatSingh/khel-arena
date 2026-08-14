<script>
	/**
	 * Stands in for a native <select> wherever the dropdown itself needs to
	 * look like the rest of the site — a native listbox can't take a border,
	 * a radius, or a gap from the pill that opens it, so this draws all
	 * three itself, in the same bordered-surface language as every other
	 * panel here (--surface, --line, --r-md, --shadow).
	 */
	import { iconSwap } from '$lib/transitions/iconSwap.js';

	let { value, options, label, onselect, icon } = $props();

	let open = $state(false);
	let root;

	const selected = $derived(options.find((o) => o.value === value) ?? options[0]);

	function choose(v) {
		open = false;
		onselect(v);
	}

	function onDocClick(e) {
		if (open && root && !root.contains(e.target)) open = false;
	}

	function onKeydown(e) {
		if (e.key === 'Escape' && open) {
			open = false;
			root?.querySelector('.trigger')?.focus();
		}
	}

	/* The same blur-clears-as-it-lands language as the rest of the site's
	   motion (.reveal in app.css), just resolved on a dropdown's clock
	   instead of a scroll-in's — quick enough not to make someone wait on
	   a filter, silky enough not to just snap. Runs identically on the way
	   out, so closing reads as deliberate as opening instead of a cut. */
	function panelPop(node, { duration = 200 } = {}) {
		return {
			duration,
			easing: (t) => 1 - Math.pow(1 - t, 3),
			css: (t, u) => `
				opacity: ${t};
				transform: scale(${0.96 + 0.04 * t}) translateY(${u * -5}px);
				filter: blur(${u * 5}px);
			`
		};
	}
</script>

<svelte:window onclick={onDocClick} onkeydown={onKeydown} />

<div class="listbox" bind:this={root}>
	<button
		type="button"
		class="trigger"
		aria-haspopup="listbox"
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		{#if icon}
			<span class="trigger-icon">
				{#key selected.value}
					<span class="trigger-icon-inner" in:iconSwap out:iconSwap>
						{@render icon(selected.value)}
					</span>
				{/key}
			</span>
		{/if}
		<span class="sr-only">{label}</span>
		<span class="trigger-text">
			<span class="trigger-text-sizer" aria-hidden="true">
				{#each options as opt (opt.value)}
					<span>{opt.label}</span>
				{/each}
			</span>
			<span class="trigger-text-value">{selected.label}</span>
		</span>
	</button>

	{#if open}
		<ul class="panel" role="listbox" aria-label={label} transition:panelPop>
			{#each options as opt (opt.value)}
				<li>
					<button
						type="button"
						role="option"
						aria-selected={opt.value === value}
						class="option"
						class:on={opt.value === value}
						onclick={() => choose(opt.value)}
					>
						<span class="option-main">
							{#if icon}
								<span class="option-icon">{@render icon(opt.value)}</span>
							{/if}
							{opt.label}
						</span>
						{#if opt.value === value}
							<svg class="check" viewBox="0 0 16 16" aria-hidden="true">
								<path d="M3 8.5 6.2 12 13 4" />
							</svg>
						{/if}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.listbox {
		position: relative;
		display: inline-flex;
	}

	.trigger {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.5rem 2.25rem 0.5rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-pill);
		background: var(--surface);
		font-size: 0.9375rem;
		font-weight: 500;
		color: var(--muted);
		cursor: pointer;
		transition:
			border-color var(--dur-hover) var(--ease),
			color var(--dur-hover) var(--ease);
	}

	/* Reserves the width of the widest option, so the pill never resizes
	   as the selection changes ("Futsal" and "Cricket net" sit in the same
	   slot) — the invisible stack sets the box's true width, the value on
	   top just paints over it. */
	.trigger-text {
		position: relative;
		display: inline-block;
	}

	.trigger-text-sizer {
		display: grid;
		visibility: hidden;
	}

	.trigger-text-sizer span {
		grid-area: 1 / 1;
		white-space: nowrap;
	}

	.trigger-text-value {
		position: absolute;
		inset: 0;
		white-space: nowrap;
	}

	.trigger-icon {
		position: relative;
		display: inline-flex;
		width: 1.2em;
		height: 1.2em;
		color: var(--pine);
	}

	.trigger-icon-inner {
		position: absolute;
		inset: 0;
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}

	.trigger:hover {
		border-color: var(--line-strong);
		color: var(--ink);
	}

	.trigger::after {
		content: '';
		position: absolute;
		right: 1rem;
		top: 50%;
		width: 7px;
		height: 7px;
		border-right: 1.5px solid var(--muted);
		border-bottom: 1.5px solid var(--muted);
		transform: translateY(-65%) rotate(45deg);
		pointer-events: none;
	}

	/* The gap a native <select> can't give its own popup — the panel sits a
	   clear half-rem below the pill instead of sealed against it. */
	.panel {
		position: absolute;
		z-index: 30;
		top: 100%;
		right: 0;
		margin-top: 0.6rem;
		min-width: 100%;
		padding: 0.4rem;
		background: var(--surface);
		border-radius: var(--r-md);
		box-shadow: var(--shadow);
		transform-origin: top right;
	}

	/* Rows are told apart by a single hairline underneath, not a boxed
	   border — in the same soft pine tint the badges use, never a side. */
	li:not(:last-child) .option {
		border-bottom: 1px solid var(--pine-wash);
	}

	.option {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		width: 100%;
		padding: 0.55rem 0.75rem;
		/* Buttons carry a default UA border the site's other reset never
		   strips — without this, that default frames every row on all
		   four sides regardless of what the hairline rule below adds. */
		border: none;
		border-radius: var(--r-sm);
		background: none;
		text-align: left;
		font-size: 0.9375rem;
		color: var(--muted);
		white-space: nowrap;
		cursor: pointer;
		transition:
			background-color var(--dur-hover) var(--ease),
			color var(--dur-hover) var(--ease);
	}

	.option:hover {
		background: var(--surface-sunk);
		color: var(--ink);
	}

	.option.on {
		color: var(--pine);
		font-weight: 600;
	}

	.option-main {
		display: inline-flex;
		align-items: center;
		gap: 0.55rem;
	}

	.option-icon {
		display: inline-flex;
		flex-shrink: 0;
		color: var(--pine);
	}

	.check {
		width: 13px;
		height: 13px;
		flex-shrink: 0;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.8;
		stroke-linecap: round;
		stroke-linejoin: round;
	}
</style>
