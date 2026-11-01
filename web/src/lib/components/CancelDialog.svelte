<script>
	/**
	 * The popup that asks why before a booking is cancelled.
	 *
	 * A reason is required: the venue sees it, and an hour that comes back on
	 * the board unexplained tells them nothing. When cancelling would cost
	 * user score points (a confirmed booking less than five hours away), the
	 * popup says how many, and whether it would stop the player booking.
	 *
	 * A native <dialog> opened with showModal(): focus is trapped and Escape
	 * closes it without any code here.
	 */
	import { penaltyPreview, LATE_CANCEL_PENALTY } from '$lib/userScore.js';

	let {
		open = $bindable(false),
		/** One line saying which booking this is. */
		what = '',
		/** The heading, e.g. "Cancel this booking?" */
		title = 'Cancel this booking?',
		confirmLabel = 'Cancel booking',
		/** True when this cancellation costs points. */
		late = false,
		/** The player's score now, for the warning. Null if unknown. */
		userScore = null,
		/** Called with the reason. Throw to keep the popup open with an error. */
		onconfirm
	} = $props();

	const REASONS = [
		'Plans changed',
		'Not enough players',
		'Weather',
		'Booked the wrong time',
		'Found another venue',
		'Other'
	];

	let dialog = $state();
	let choice = $state('');
	let details = $state('');
	let busy = $state(false);
	let error = $state(null);

	const preview = $derived(late && userScore !== null ? penaltyPreview(userScore) : null);
	const reason = $derived(
		choice === 'Other' ? details.trim() : details.trim() ? `${choice}: ${details.trim()}` : choice
	);
	const ready = $derived(choice !== '' && (choice !== 'Other' || details.trim().length >= 3));

	// The prop drives the element: showModal() to open, close() to shut.
	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) {
			choice = '';
			details = '';
			error = null;
			dialog.showModal();
		} else if (!open && dialog.open) {
			dialog.close();
		}
	});

	async function submit(event) {
		event.preventDefault();
		if (!ready || busy) return;
		busy = true;
		error = null;
		try {
			await onconfirm?.(reason);
			open = false;
		} catch (err) {
			error = err?.message || 'That did not go through.';
		} finally {
			busy = false;
		}
	}

	// A click on the backdrop lands on the <dialog> itself, outside the panel.
	function onBackdrop(event) {
		if (event.target === dialog && !busy) open = false;
	}
</script>

<dialog
	bind:this={dialog}
	class="cancel"
	aria-labelledby="cancel-title"
	onclose={() => (open = false)}
	onclick={onBackdrop}
>
	<form class="panel" onsubmit={submit}>
		<h2 id="cancel-title" class="display display-m">{title}</h2>
		{#if what}<p class="what small">{what}</p>{/if}

		{#if late}
			<div class="warn" role="note">
				<strong>This is less than 5 hours before kick-off.</strong>
				{#if preview}
					Your user score drops by {LATE_CANCEL_PENALTY}, from {preview.before} to
					{preview.after}/100.
					{#if preview.bans}
						That's 10 or below, so <strong>you won't be able to book any venue for a month.</strong>
					{/if}
				{:else}
					Your user score drops by {LATE_CANCEL_PENALTY} points.
				{/if}
			</div>
		{/if}

		<fieldset>
			<legend class="field-label">Why are you cancelling?</legend>
			<div class="reasons">
				{#each REASONS as r (r)}
					<label class="reason" class:on={choice === r}>
						<input type="radio" name="cancel-reason" value={r} bind:group={choice} />
						{r}
					</label>
				{/each}
			</div>
		</fieldset>

		<label class="details" for="cancel-details">
			<span class="field-label">
				{choice === 'Other' ? 'Tell the venue why' : 'Anything to add? (optional)'}
			</span>
			<textarea
				id="cancel-details"
				bind:value={details}
				rows="3"
				maxlength="250"
				required={choice === 'Other'}
				placeholder="Two of our players are ill."
			></textarea>
		</label>

		{#if error}<p class="error small" role="alert">{error}</p>{/if}

		<div class="actions">
			<button type="button" class="btn btn-secondary" disabled={busy} onclick={() => (open = false)}>
				Keep it
			</button>
			<button class="btn btn-primary danger" class:loading={busy} disabled={!ready || busy}>
				{confirmLabel}
			</button>
		</div>
	</form>
</dialog>

<style>
	.cancel {
		width: min(32rem, calc(100vw - 2rem));
		max-height: calc(100dvh - 2rem);
		padding: 0;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-md);
		background: var(--surface);
		color: var(--ink);
	}

	.cancel::backdrop {
		background: rgb(20 28 18 / 0.45);
	}

	.panel {
		display: grid;
		gap: 1rem;
		padding: clamp(1.25rem, 4vw, 1.75rem);
	}

	h2 { margin: 0; }
	.what { margin: -0.5rem 0 0; color: var(--faint); }

	.warn {
		padding: 0.75rem 0.9rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
		font-size: 0.9rem;
		line-height: 1.45;
	}

	fieldset {
		display: grid;
		gap: 0.5rem;
		margin: 0;
		padding: 0;
		border: 0;
	}

	.field-label {
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--ink);
	}

	.reasons {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}

	.reason {
		padding: 0.4rem 0.8rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-pill);
		background: var(--field);
		font-size: 0.875rem;
		cursor: pointer;
		transition:
			background-color var(--dur-hover) var(--ease),
			border-color var(--dur-hover) var(--ease);
	}

	.reason:hover { border-color: var(--pine); }

	.reason.on {
		border-color: var(--pine);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-weight: 600;
	}

	/* The radio itself is hidden; the pill is the control. Still focusable. */
	.reason input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	.reason:has(input:focus-visible) {
		outline: 2px solid var(--pine);
		outline-offset: 2px;
	}

	.details { display: grid; gap: 0.35rem; }

	textarea {
		width: 100%;
		padding: 0.6rem 0.75rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-sm);
		background: var(--field);
		font: inherit;
		font-size: 0.92rem;
		resize: vertical;
	}

	textarea:focus-visible {
		outline: 2px solid var(--pine);
		outline-offset: 1px;
	}

	.error {
		margin: 0;
		padding: 0.6rem 0.75rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		justify-content: flex-end;
		gap: 0.5rem;
	}

	.danger {
		background: var(--brick);
		border-color: var(--brick);
	}

	.danger:hover:not(:disabled) {
		background: #7f2f21;
		border-color: #7f2f21;
	}
</style>
