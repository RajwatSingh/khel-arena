<script>
	/**
	 * What a venue's saved eSewa or Khalti key is doing right now.
	 *
	 * Shown wherever a key has been submitted, so an owner can see at a glance
	 * that it took: activated or paused, sandbox or live, and the last four
	 * characters of the key in place. The key itself is never sent back.
	 */
	let { label, account, fresh = false } = $props();
</script>

<div class="status" class:paused={!account.enabled} role="status">
	<span class="tick" aria-hidden="true">{account.enabled ? '✓' : '‖'}</span>
	<div class="text">
		<p class="headline">
			{label} key {account.enabled ? 'activated' : 'saved, paused'}
		</p>
		<p class="detail">
			{#if account.enabled}
				Players can pay with {label} at this venue.
			{:else}
				Saved, but not offered to players until you switch it back on.
			{/if}
			{#if fresh}<strong>Saved just now.</strong>{/if}
		</p>
		<p class="chips">
			<span class="chip">{account.live ? 'Live account' : 'Test (sandbox) account'}</span>
			<span class="chip">Key ending {account.secret_hint}</span>
		</p>
	</div>
</div>

<style>
	.status {
		display: flex;
		gap: 0.85rem;
		align-items: flex-start;
		padding: 0.9rem 1rem;
		border-radius: var(--r-md);
		background: var(--pine-wash);
		color: var(--pine-deep);
	}

	.status.paused {
		background: var(--surface-sunk);
		color: var(--muted);
	}

	.tick {
		flex: none;
		display: grid;
		place-items: center;
		width: 1.8rem;
		height: 1.8rem;
		border-radius: var(--r-pill);
		background: var(--pine);
		color: var(--surface);
		font-weight: 700;
	}

	.paused .tick {
		background: var(--faint);
	}

	.text {
		display: grid;
		gap: 0.3rem;
	}

	.headline {
		margin: 0;
		font-weight: 700;
	}

	.detail {
		margin: 0;
		font-size: 0.9rem;
		color: var(--muted);
	}

	.detail strong {
		margin-left: 0.35rem;
		color: var(--pine-deep);
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
		margin: 0.2rem 0 0;
	}

	.chip {
		padding: 0.15rem 0.6rem;
		border-radius: var(--r-pill);
		background: var(--surface);
		color: var(--muted);
		font-size: 0.78rem;
	}
</style>
