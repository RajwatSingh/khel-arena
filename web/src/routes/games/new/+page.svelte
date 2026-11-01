<script>
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import Field from '$lib/components/Field.svelte';
	import Listbox from '$lib/components/Listbox.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { SKILL_LABELS } from '$lib/skills.js';
	import { formatDate, formatTime, localDate } from '$lib/time.js';

	let { data } = $props();

	// Step one is picking the booking; step two is the details. A game always
	// fills a court that's already booked, so there is no "not booked yet".
	// Read once: a ?booking= link picks the starting booking, nothing more.
	const preselected = untrack(() =>
		data.bookings.find((b) => b.id === data.preselect && !b.game_id)
	);
	let bookingId = $state(preselected?.id ?? null);
	const booking = $derived(data.bookings.find((b) => b.id === bookingId) ?? null);

	let form = $state({
		title: preselected ? `Need players at ${preselected.arena_name}` : '',
		description: '',
		needed_players: 2,
		skill: 'casual'
	});
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});

	const skillOptions = Object.entries(SKILL_LABELS).map(([value, label]) => ({ value, label }));
	const available = $derived(data.bookings.filter((b) => !b.game_id));

	const day = (b) => formatDate(localDate(new Date(b.starts_at)));
	const hours = (b) => `${formatTime(b.starts_at)}–${formatTime(b.ends_at)}`;
	// Court, format and area, skipping whichever a venue left blank.
	const details = (...parts) => parts.filter(Boolean).join(' · ');

	function pick(b) {
		bookingId = b.id;
		error = null;
		fieldErrors = {};
		if (!form.title.trim()) form.title = `Need players at ${b.arena_name}`;
	}

	async function submit(event) {
		event.preventDefault();
		if (!booking) return;
		busy = true;
		error = null;
		fieldErrors = {};

		try {
			const call = await api.createCall({
				title: form.title,
				description: form.description,
				needed_players: Number(form.needed_players),
				skill: form.skill,
				booking_id: booking.id,
				// The server takes kickoff from the booking; this is only here
				// because the wire format expects an instant.
				starts_at: booking.starts_at
			});
			goto(`/games/${call.id}`);
		} catch (err) {
			if (err instanceof ApiError) {
				error = err.fields.length ? null : err.message;
				fieldErrors = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
			} else {
				error = 'Something went wrong on our side.';
			}
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Post a game | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="auth forest-band">
	<div class="shell split">
		<div class="say">
			<h1 class="display display-l"><TextAnimate text="Who's missing?" /></h1>
			<p class="lede">
				Pick the court you've booked, say how many you're short, and it goes on the call sheet. The
				hour comes from the booking, so nobody turns up at the wrong time.
			</p>
			<div class="pitch-mark"><CentreMark wide /></div>
		</div>

		<div class="card panel">
			{#if !booking}
				<p class="label">Step 1 of 2</p>
				<h2 class="step-title">Which booking?</h2>

				{#if data.bookings.length === 0}
					<div class="empty">
						<p>
							You need a confirmed booking to post a game. Book a court first, then come back to
							find players for it.
						</p>
						<a class="btn btn-primary" href="/tonight">Find a court</a>
					</div>
				{:else}
					{#if available.length === 0}
						<p class="small quiet">
							Every booking you have already has a game. Book another court to post a new one.
						</p>
					{/if}
					<ul class="bookings" role="list">
						{#each data.bookings as b (b.id)}
							<li>
								{#if b.game_id}
									<div class="booking taken">
										<span class="when">
											<span class="time num">{hours(b)}</span>
											<span class="date small">{day(b)}</span>
										</span>
										<span class="where">
											<strong>{b.arena_name}</strong>
											<span class="small">{details(b.court_name, b.format)}</span>
										</span>
										<a class="small has-game" href="/games/{b.game_id}">View game</a>
									</div>
								{:else}
									<button type="button" class="booking" onclick={() => pick(b)}>
										<span class="when">
											<span class="time num">{hours(b)}</span>
											<span class="date small">{day(b)}</span>
										</span>
										<span class="where">
											<strong>{b.arena_name}</strong>
											<span class="small">{details(b.court_name, b.format, b.arena_area)}</span>
										</span>
										<span class="go" aria-hidden="true">→</span>
									</button>
								{/if}
							</li>
						{/each}
					</ul>
					<p class="small quiet foot">
						Only confirmed bookings show here. A hold you haven't paid for yet will appear once it's
						confirmed.
					</p>
				{/if}
			{:else}
				<form onsubmit={submit}>
					<p class="label">Step 2 of 2</p>
					<h2 class="step-title">The details</h2>

					<div class="chosen">
						<span class="where">
							<strong>{booking.arena_name}</strong>
							<span class="small num">{day(booking)} · {hours(booking)}</span>
							<span class="small">{details(booking.court_name, booking.format)}</span>
						</span>
						<button type="button" class="btn btn-quiet" onclick={() => (bookingId = null)}>
							Change
						</button>
					</div>

					{#if error}
						<p class="error small" role="alert">{error}</p>
					{/if}

					<Field
						name="title"
						label="What's the game?"
						bind:value={form.title}
						error={fieldErrors.title}
						required
						maxlength="120"
						placeholder="Need two more at {booking.arena_name}"
					/>

					<Field
						name="needed_players"
						label="How many are you short?"
						type="number"
						bind:value={form.needed_players}
						error={fieldErrors.needed_players}
						min="1"
						max="15"
						required
					/>

					<div class="pick">
						<span class="label">Standard</span>
						<Listbox
							value={form.skill}
							options={skillOptions}
							label="Standard"
							fill
							onselect={(v) => (form.skill = v)}
						/>
					</div>

					<label class="pick" for="description">
						<span class="label">Anything else (optional)</span>
						<textarea
							id="description"
							bind:value={form.description}
							maxlength="280"
							rows="3"
							placeholder="Friendly pace, bibs provided, we play through."
						></textarea>
					</label>

					<button class="btn btn-primary" class:loading={busy} disabled={busy || !session.signedIn}>
						Post it
					</button>
				</form>
			{/if}
		</div>
	</div>
</section>

<style>
	h1 {
		margin-top: 0.6rem;
	}

	.split {
		display: grid;
		gap: clamp(2rem, 5vw, 4rem);
		align-items: start;
	}

	@media (min-width: 60rem) {
		.split {
			grid-template-columns: minmax(0, 1fr) 28rem;
		}
	}

	.say .lede {
		margin-top: 1rem;
		max-width: 44ch;
	}

	.panel {
		padding: clamp(1.5rem, 3vw, 2rem);
	}

	.step-title {
		margin: 0.35rem 0 1rem;
		font-family: var(--display);
		font-size: 1.5rem;
		font-weight: 800;
		letter-spacing: -0.02em;
		color: var(--ink);
	}

	form {
		display: grid;
		gap: 1rem;
	}

	form .step-title {
		margin-bottom: 0;
	}

	.bookings {
		display: grid;
		gap: 0.5rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.booking {
		display: grid;
		grid-template-columns: 7.5rem minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.85rem;
		width: 100%;
		padding: 0.8rem 0.9rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
		font: inherit;
		text-align: left;
		color: var(--ink);
		cursor: pointer;
		transition:
			border-color var(--dur-hover) var(--ease),
			background-color var(--dur-hover) var(--ease);
	}

	button.booking:hover {
		border-color: var(--pine);
		background: var(--pine-wash);
	}

	.booking.taken {
		cursor: default;
		background: var(--field);
	}

	.booking.taken .when,
	.booking.taken .where {
		opacity: 0.6;
	}

	.when,
	.where {
		display: grid;
		gap: 0.1rem;
		min-width: 0;
	}

	.time {
		font-weight: 700;
		white-space: nowrap;
	}

	.date,
	.where .small {
		color: var(--muted);
	}

	.where strong,
	.where .small {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.go {
		color: var(--pine);
		font-weight: 700;
	}

	.has-game {
		white-space: nowrap;
		font-weight: 600;
	}

	.chosen {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 0.8rem 0.9rem;
		border: 1px solid var(--pine);
		border-radius: var(--r-md);
		background: var(--pine-wash);
	}

	.chosen .btn {
		flex-shrink: 0;
		padding-block: 0.45em;
	}

	.empty {
		display: grid;
		gap: 1rem;
		justify-items: start;
		color: var(--muted);
	}

	.quiet {
		color: var(--muted);
	}

	.foot {
		margin-top: 1rem;
	}

	.pick {
		display: grid;
		gap: 0.35rem;
	}

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
		padding: 0.6rem 0.75rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}

	@media (max-width: 30rem) {
		.booking {
			grid-template-columns: minmax(0, 1fr) auto;
		}

		.booking .when {
			grid-column: 1 / -1;
			grid-auto-flow: column;
			justify-content: start;
			gap: 0.5rem;
			align-items: baseline;
		}
	}
</style>
