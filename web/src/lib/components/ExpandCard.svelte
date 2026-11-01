<script>
	/**
	 * A horizontal card that opens into whatever it holds, usually a form.
	 *
	 * The closed card is a one-line summary (an optional picture or icon on
	 * the left, a title and a line of detail) so a long back-office page reads
	 * as a list of things you can do rather than a wall of empty fields.
	 *
	 * A native <details>: keyboard and screen-reader support come with it, and
	 * `open` is bindable so the page can close the card after a save.
	 */
	let {
		open = $bindable(false),
		/** Snippet for the left edge: a cover image, an icon. Optional. */
		lead,
		/** Snippet for the summary text. */
		summary,
		/** Words on the right while closed and open. */
		openLabel = 'Edit',
		closeLabel = 'Close',
		children
	} = $props();
</script>

<details class="xcard" bind:open>
	<summary>
		{#if lead}
			<span class="lead">{@render lead()}</span>
		{/if}
		<span class="what">{@render summary()}</span>
		<span class="toggle small">{open ? closeLabel : openLabel}</span>
		<span class="chev" aria-hidden="true"></span>
	</summary>

	<div class="body">
		{@render children()}
	</div>
</details>

<style>
	.xcard {
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
		transition: border-color var(--dur-hover) var(--ease);
		/* Lets the open/close below animate to and from `auto` where the
		   browser supports it; elsewhere it simply snaps open. */
		interpolate-size: allow-keywords;
	}

	.xcard:hover,
	.xcard[open] {
		border-color: var(--line-strong);
	}

	.xcard::details-content {
		block-size: 0;
		overflow: hidden;
		transition:
			block-size 280ms var(--ease),
			content-visibility 280ms allow-discrete;
	}

	.xcard[open]::details-content {
		block-size: auto;
	}

	@media (prefers-reduced-motion: reduce) {
		.xcard::details-content {
			transition: none;
		}
	}

	summary {
		display: flex;
		align-items: center;
		gap: 1rem 1.25rem;
		padding: 0.9rem 1.1rem;
		cursor: pointer;
		list-style: none;
	}

	summary::-webkit-details-marker {
		display: none;
	}

	summary:focus-visible {
		outline: 2px solid var(--pine);
		outline-offset: 2px;
		border-radius: var(--r-md);
	}

	.lead {
		flex: none;
		display: flex;
	}

	.what {
		display: grid;
		flex: 1;
		gap: 0.3rem;
		min-width: 0;
	}

	.toggle {
		flex: none;
		color: var(--pine);
		font-weight: 600;
	}

	/* The same drawn chevron as the court panels, turning with the card. */
	.chev {
		flex: none;
		width: 0.5rem;
		height: 0.5rem;
		border-right: 2px solid var(--faint);
		border-bottom: 2px solid var(--faint);
		transform: rotate(45deg) translate(-2px, -2px);
		transition: transform var(--dur-hover) var(--ease);
	}

	.xcard[open] .chev {
		transform: rotate(-135deg) translate(-2px, -2px);
	}

	.body {
		padding: 1.25rem 1.1rem 1.4rem;
		border-top: 1px solid var(--line);
	}

	@media (max-width: 34rem) {
		.toggle {
			display: none;
		}
	}
</style>
