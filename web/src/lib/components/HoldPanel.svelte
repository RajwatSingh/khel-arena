<script>
	import { fly } from 'svelte/transition';
	import { clock } from '$lib/session.svelte.js';
	import { formatCountdown, formatDateLong, formatNPR, formatTime } from '$lib/time.js';

	/**
	 * The right-hand column of the booking page: what you picked, what it costs,
	 * and — once held — how long you have left to pay for it.
	 *
	 * `summary` describes the selection before anything is held (total hours,
	 * how many separate bookings, the price). `holds` is what came back from
	 * "Hold" — one booking, or more when the picked hours weren't all in a row.
	 */
	let {
		arena,
		court,
		date,
		summary = null,
		holds = [],
		busy = false,
		error = null,
		signedIn = false,
		onhold,
		onpay,
		onrelease
	} = $props();

	const ticking = clock();

	const pending = $derived(holds.filter((h) => h.status === 'pending'));
	const soonest = $derived(
		pending.length ? Math.min(...pending.map((h) => new Date(h.hold_expires_at).getTime())) : 0
	);
	const remaining = $derived(soonest ? soonest - ticking.now : 0);

	const allConfirmed = $derived(holds.length > 0 && holds.every((h) => h.status === 'confirmed'));
	const lapsed = $derived(pending.length > 0 && remaining <= 0);
	const single = $derived(holds.length === 1 ? holds[0] : null);
	const heldHours = $derived(
		holds.reduce(
			(n, h) => n + Math.round((new Date(h.ends_at).getTime() - new Date(h.starts_at).getTime()) / 3_600_000),
			0
		)
	);

	// Keys the slide transition to the panel's current shape, not how it got
	// there.
	const stateKey = $derived(
		allConfirmed ? 'confirmed' : lapsed ? 'lapsed' : holds.length ? 'held' : summary ? 'selected' : 'empty'
	);
</script>

<aside class="card panel" aria-live="polite">
{#key stateKey}
<div in:fly={{ y: 10, duration: 260 }}>
	{#if allConfirmed}
		<p class="label">Confirmed</p>
		{#if single}
			<p class="headline display num">{single.reference}</p>
			<p class="body">
				{court.name} at {arena.name}, {formatDateLong(date)}, {formatTime(single.starts_at)}–{formatTime(
					single.ends_at
				)}. Read the reference out at the gate.
			</p>
		{:else}
			<p class="headline display">{holds.length} bookings</p>
			<p class="body">
				All {holds.length} are paid and confirmed for {formatDateLong(date)}. The references are on
				your bookings page.
			</p>
		{/if}
		<a class="btn btn-secondary wide" href="/bookings">See my bookings</a>
	{:else if lapsed}
		<p class="label">Hold lapsed</p>
		<p class="headline display">Time up</p>
		<p class="body">
			Fifteen minutes went by, so the {pending.length === 1 ? 'hour' : 'hours'} went back on the
			board. Pick again if still free.
		</p>
		<button class="btn btn-secondary wide" onclick={onrelease}>Start over</button>
	{:else if holds.length}
		<p class="label">Held for you</p>
		<p
			class="headline display num"
			class:urgent={remaining < 120_000}
			class:pulse-urgent={remaining < 120_000}
		>
			{formatCountdown(remaining)}
		</p>
		{#if single}
			<p class="body">
				{court.name} · {formatTime(single.starts_at)}–{formatTime(single.ends_at)} · {formatDateLong(
					date
				)}
			</p>
		{:else}
			<p class="body">
				{heldHours} hours across {holds.length} bookings · {formatDateLong(date)}
			</p>
		{/if}
		<p class="body dim">
			Pay before the clock runs out or {single ? 'the hour is' : 'the hours are'} released. Nobody
			else can take {single ? 'it' : 'them'} until then.
		</p>

		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="actions">
			<button
				class="btn btn-primary grow"
				class:loading={busy}
				onclick={() => onpay()}
				disabled={busy}
			>
				{#if busy && single}Talking to the gateway…{:else if single}Pay NPR {formatNPR(
						single.price_npr
					)}{:else}Pay in My bookings{/if}
			</button>
			<button class="btn btn-secondary" onclick={onrelease} disabled={busy}>
				{single ? 'Cancel hold' : 'Cancel all'}
			</button>
		</div>
	{:else if summary}
		<p class="label">Selected</p>
		{#if summary.ends_at}
			<p class="headline display num">
				{formatTime(summary.starts_at)}–{formatTime(summary.ends_at)}
			</p>
		{:else}
			<p class="headline display num">{summary.hours} hours</p>
		{/if}
		<dl class="lines">
			<div><dt>Court</dt><dd>{court.name} · {court.format}</dd></div>
			<div><dt>Date</dt><dd>{formatDateLong(date)}</dd></div>
			<div><dt>Rate</dt><dd>{summary.rule}{summary.is_peak ? ' · peak' : ''}</dd></div>
			{#if summary.blocks > 1}
				<div><dt>Bookings</dt><dd>{summary.blocks} separate holds</dd></div>
			{/if}
			<div class="total">
				<dt>{summary.hours === 1 ? 'One hour' : `${summary.hours} hours`}</dt>
				<dd class="num">NPR {formatNPR(summary.price_npr)}</dd>
			</div>
		</dl>

		{#if summary.overlong}
			<p class="error" role="alert">
				One block runs over four hours. Drop an hour, or leave a gap so it splits into two
				bookings.
			</p>
		{:else if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		{#if signedIn}
			<button
				class="btn btn-primary wide"
				class:loading={busy}
				onclick={onhold}
				disabled={busy || summary.overlong}
			>
				{#if busy}Taking the {summary.hours === 1 ? 'hour' : 'hours'}…{:else if summary.hours === 1}Hold
					this hour{:else}Hold {summary.hours} hours{/if}
			</button>
			<p class="body dim">
				Nothing is charged yet. You get fifteen minutes to pay before
				{summary.hours === 1 ? 'it goes' : 'they go'} back on the board.
			</p>
		{:else}
			<a class="btn btn-primary wide" href="/login">Sign in to hold {summary.hours === 1 ? 'it' : 'them'}</a>
			<p class="body dim">
				Holding {summary.hours === 1 ? 'an hour' : 'hours'} puts {summary.hours === 1 ? 'it' : 'them'}
				in your name, so we need to know whose name that is.
			</p>
		{/if}
	{:else}
		<p class="label">Nothing picked yet</p>
		<p class="headline display quiet">Choose your hours</p>
		<p class="body">
			Tap any free hours on the left — back to back or not — and they appear here with the price.
			Struck-through hours are taken; hatched ones have been and gone.
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
