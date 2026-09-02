<script>
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import Field from '$lib/components/Field.svelte';
	import Listbox from '$lib/components/Listbox.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { SKILL_LABELS } from '$lib/skills.js';

	let { data } = $props();

	let form = $state({
		title: '',
		description: '',
		needed_players: 2,
		skill: 'casual',
		booking_id: '',
		starts_at: ''
	});
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});

	const skillOptions = Object.entries(SKILL_LABELS).map(([value, label]) => ({ value, label }));

	// A booking already fixes the hour, so choosing one hides the time field:
	// the server takes kickoff from the booking and would ignore anything
	// typed here, and a form that asks for something it discards is a lie.
	const bookingOptions = $derived([
		{ value: '', label: "Not booked yet — we'll sort a court" },
		...data.bookings.map((b) => ({
			value: b.id,
			label: `${b.arena_name} · ${b.court_name} · ${new Date(b.starts_at).toLocaleString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}`
		}))
	]);

	const attached = $derived(form.booking_id !== '');

	async function submit(event) {
		event.preventDefault();
		busy = true;
		error = null;
		fieldErrors = {};

		try {
			const call = await api.createCall({
				title: form.title,
				description: form.description,
				needed_players: Number(form.needed_players),
				skill: form.skill,
				booking_id: form.booking_id || null,
				// The server overrides this when a booking is attached. Sent
				// as an instant either way, because that is what the wire
				// speaks.
				starts_at: attached
					? new Date().toISOString()
					: new Date(form.starts_at).toISOString()
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
				Put it on the call sheet. If you've already booked a court, say which one — the hour comes
				from the booking, so nobody turns up at the wrong time.
			</p>
			<div class="pitch-mark"><CentreMark wide /></div>
		</div>

		<form class="card" onsubmit={submit}>
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
				placeholder="Need two more at Dhuku"
			/>

			<label class="pick">
				<span class="label">Court</span>
				<Listbox
					value={form.booking_id}
					options={bookingOptions}
					label="Court"
					fill
					onselect={(v) => (form.booking_id = v)}
				/>
			</label>

			{#if !attached}
				<Field
					name="starts_at"
					label="Kick-off"
					type="datetime-local"
					bind:value={form.starts_at}
					error={fieldErrors.starts_at}
					required
				/>
			{/if}

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

			<label class="pick">
				<span class="label">Standard</span>
				<Listbox
					value={form.skill}
					options={skillOptions}
					label="Standard"
					fill
					onselect={(v) => (form.skill = v)}
				/>
			</label>

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
			grid-template-columns: minmax(0, 1fr) 26rem;
		}
	}

	.say .lede {
		margin-top: 1rem;
		max-width: 44ch;
	}

	form {
		display: grid;
		gap: 1rem;
		padding: clamp(1.5rem, 3vw, 2rem);
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
</style>
