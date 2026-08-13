<script>
	import { fly } from 'svelte/transition';
	import { clock } from '$lib/session.svelte.js';
	import { formatCountdown, formatDateLong, formatNPR, formatTime } from '$lib/time.js';

	/**
	 * The right-hand column of the booking page: what you picked, what it costs,
	 * and — once held — how long you have left to pay for it.
	 */
	let {
		arena,
		court,
		date,
		slot,
		hold,
		busy = false,
		error = null,
		signedIn = false,
		onhold,
		onpay,
		onrelease
	} = $props();

	const ticking = clock();

	const remaining = $derived(
		hold?.hold_expires_at ? new Date(hold.hold_expires_at).getTime() - ticking.now : 0
	);
	const lapsed = $derived(hold?.status === 'pending' && remaining <= 0);

	// What the panel is showing right now, not how it got there — used only
	// to key the transition below, so a status change slides the panel's
	// content rather than snapping it.
	const stateKey = $derived(
		hold && hold.status === 'confirmed'
			? 'confirmed'
			: hold && lapsed
				? 'lapsed'
				: hold
					? 'held'
					: slot
						? 'selected'
						: 'empty'
	);
</script>

<aside class="card panel" aria-live="polite">
{#key stateKey}
<div in:fly={{ y: 10, duration: 260 }}>
	{#if hold && hold.status === 'confirmed'}
		<p class="label">Confirmed</p>
		<p class="headline display num">{hold.reference}</p>
		<p class="body">
			{court.name} at {arena.name}, {formatDateLong(date)}, {formatTime(hold.starts_at)}–{formatTime(
				hold.ends_at
			)}. Read the reference out at the gate.
		</p>
		<a class="btn btn-secondary wide" href="/bookings">See my bookings</a>
	{:else if hold && lapsed}
		<p class="label">Hold lapsed</p>
		<p class="headline display">Time up</p>
		<p class="body">
			Fifteen minutes went by, so the hour went back on the board. Pick it again if it is still
			there.
		</p>
		<button class="btn btn-secondary wide" onclick={onrelease}>Start over</button>
	{:else if hold}
		<p class="label">Held for you</p>
		<p
			class="headline display num"
			class:urgent={remaining < 120_000}
			class:pulse-urgent={remaining < 120_000}
		>
			{formatCountdown(remaining)}
		</p>
		<p class="body">
			{court.name} · {formatTime(hold.starts_at)}–{formatTime(hold.ends_at)} · {formatDateLong(date)}
		</p>
		<p class="body dim">
			Pay before the clock runs out or the hour is released. Nobody else can take it until then.
		</p>

		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="actions">
			<button class="btn btn-primary grow" class:loading={busy} onclick={onpay} disabled={busy}>
				{busy ? 'Talking to eSewa…' : `Pay NPR ${formatNPR(hold.price_npr)}`}
			</button>
			<button class="btn btn-secondary" onclick={onrelease} disabled={busy}>Release</button>
		</div>
	{:else if slot}
		<p class="label">Selected</p>
		<p class="headline display num">{formatTime(slot.starts_at)}–{formatTime(slot.ends_at)}</p>
		<dl class="lines">
			<div><dt>Court</dt><dd>{court.name} · {court.format}</dd></div>
			<div><dt>Date</dt><dd>{formatDateLong(date)}</dd></div>
			<div><dt>Rate</dt><dd>{slot.rule}{slot.is_peak ? ' · peak' : ''}</dd></div>
			<div class="total">
				<dt>One hour</dt>
				<dd class="num">NPR {formatNPR(slot.price_npr)}</dd>
			</div>
		</dl>

		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		{#if signedIn}
			<button class="btn btn-primary wide" class:loading={busy} onclick={onhold} disabled={busy}>
				{busy ? 'Taking the hour…' : 'Hold this hour'}
			</button>
			<p class="body dim">
				Nothing is charged yet. You get fifteen minutes to pay before it goes back on the board.
			</p>
		{:else}
			<a class="btn btn-primary wide" href="/login">Sign in to hold it</a>
			<p class="body dim">
				Holding an hour puts it in your name, so we need to know whose name that is.
			</p>
		{/if}
	{:else}
		<p class="label">Nothing picked yet</p>
		<p class="headline display quiet">Choose an hour</p>
		<p class="body">
			Tap a free hour on the left and it appears here with the price. Struck-through hours are
			already taken; hatched ones have been and gone.
		</p>
	{/if}
</div>
{/key}
</aside>

<style>
	.panel {
		position: sticky;
		top: 100px;
		padding: 1.75rem;
	}

	.headline {
		margin-top: 0.5rem;
		font-size: clamp(2rem, 3vw, 2.5rem);
		line-height: 1.1;
		letter-spacing: -0.025em;
		font-variation-settings: 'opsz' 36;
	}

	.headline.urgent {
		color: var(--brick);
	}

	.headline.quiet {
		color: var(--muted);
	}

	.body {
		margin-top: 1.1rem;
		font-size: 0.9375rem;
		line-height: 1.6;
		color: var(--muted);
	}

	.dim {
		color: var(--faint);
	}

	.lines {
		margin-top: 1.35rem;
	}

	.lines div {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.65rem 0;
		border-bottom: 1px solid var(--line);
		font-size: 0.9375rem;
	}

	dt {
		color: var(--faint);
	}

	dd {
		margin: 0;
		text-align: right;
		color: var(--muted);
	}

	.total {
		border-bottom: 0;
		padding-top: 0.85rem;
	}

	.total dt,
	.total dd {
		color: var(--ink);
		font-weight: 600;
		font-size: 1.0625rem;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.6rem;
		margin-top: 1.35rem;
	}

	.grow {
		flex: 1 1 10rem;
	}

	.wide {
		width: 100%;
		margin-top: 1.35rem;
	}

	.error {
		margin-top: 1.2rem;
		padding: 0.8rem 1rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
		font-size: 0.9375rem;
		line-height: 1.5;
	}

	@media (max-width: 940px) {
		.panel {
			position: static;
		}
	}
</style>
