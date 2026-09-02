<script>
	import { untrack } from 'svelte';
	import { invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import Field from '$lib/components/Field.svelte';
	import CourtPanel from '$lib/components/CourtPanel.svelte';
	import { formatDate, formatNPR, formatTime } from '$lib/time.js';

	let { data } = $props();

	const arena = $derived(data.arena);
	const courts = $derived(arena.courts ?? []);

	// The till: cash intents nobody has confirmed yet are the whole reason
	// this page exists, so they come first and everything else follows.
	const outstanding = $derived(
		data.payments.filter((p) => p.provider === 'cash' && p.status === 'initiated')
	);
	const settled = $derived(data.payments.filter((p) => !outstanding.includes(p)));

	let newCourt = $state({ name: '', format: '', side_count: 5, base_price_npr: 1200 });
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});
	let savedVenue = $state(false);

	/**
	 * The venue form is seeded from the arena as it stands.
	 *
	 * The endpoint replaces every field it owns, so a form that started empty
	 * would blank the description and the amenities of anyone who only came to
	 * fix the closing time.
	 *
	 * `untrack` because reading it once is the point: a reload landing while
	 * somebody is halfway through typing must not throw their edit away.
	 */
	let venue = $state(untrack(() => seedVenue(data.arena)));

	function seedVenue(a) {
		return {
			name: a.name,
			area: a.area,
			city: a.city,
			description: a.description ?? '',
			cover_url: a.cover_url ?? '',
			// Amenities are a list on the wire and a comma-separated line here:
			// a chip editor is a lot of machinery for a field most venues fill
			// in once.
			amenities: (a.amenities ?? []).join(', '),
			phone: a.phone ?? '',
			opens_at: a.opens_at,
			closes_at: a.closes_at,
			lat: a.lat ?? '',
			lng: a.lng ?? ''
		};
	}

	async function act(fn) {
		busy = true;
		error = null;
		fieldErrors = {};
		try {
			await fn();
			await invalidateAll();
			return true;
		} catch (err) {
			if (err instanceof ApiError) {
				error = err.fields.length ? null : err.message;
				fieldErrors = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
			} else {
				error = 'That did not go through.';
			}
			return false;
		} finally {
			busy = false;
		}
	}

	const received = (payment) => act(() => api.markCashReceived(payment.id));
	const setActive = (active) => act(() => api.setArenaActive(arena.id, active));

	async function saveVenue(event) {
		event.preventDefault();
		const ok = await act(() =>
			api.updateArena(arena.id, {
				name: venue.name,
				area: venue.area,
				city: venue.city,
				description: venue.description,
				cover_url: venue.cover_url,
				amenities: venue.amenities
					.split(',')
					.map((a) => a.trim())
					.filter(Boolean),
				phone: venue.phone,
				opens_at: venue.opens_at,
				closes_at: venue.closes_at,
				// Empty means "no coordinate", which is null on the wire — not
				// zero, which is a real place in the Gulf of Guinea.
				lat: venue.lat === '' ? null : Number(venue.lat),
				lng: venue.lng === '' ? null : Number(venue.lng)
			})
		);
		if (ok) {
			savedVenue = true;
			setTimeout(() => (savedVenue = false), 2000);
		}
	}

	async function addCourt(event) {
		event.preventDefault();
		const ok = await act(() =>
			api.createCourt(arena.id, {
				...newCourt,
				side_count: Number(newCourt.side_count),
				base_price_npr: Number(newCourt.base_price_npr)
			})
		);
		if (ok) newCourt = { name: '', format: '', side_count: 5, base_price_npr: 1200 };
	}
</script>

<svelte:head>
	<title>{arena.name} · manage | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up"><a class="back" href="/manage">The back office</a></p>
		<h1 class="display display-l"><TextAnimate text={arena.name} /></h1>
		<p class="lede fade-up fade-up-1">
			{arena.area}, {arena.city} · open {arena.opens_at}–{arena.closes_at}
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="body">
	<div class="shell">
		{#if error}
			<p class="error small" role="alert">{error}</p>
		{/if}

		<h2 class="display display-m">The till</h2>
		{#if outstanding.length}
			<p class="small quiet">
				Cash bookings waiting on you. Confirming one is what turns a hold into a confirmed
				booking — until then it lapses like any unpaid hold.
			</p>
			<ul class="till">
				{#each outstanding as payment (payment.id)}
					<li>
						<div class="what">
							<strong>{payment.player_name || payment.player_username}</strong>
							<span class="small">
								{payment.court_name} · {formatDate(payment.starts_at)}
								{formatTime(payment.starts_at)}
							</span>
						</div>
						<span class="amount num">{formatNPR(payment.amount_npr)}</span>
						<button class="btn btn-primary" disabled={busy} onclick={() => received(payment)}>
							Cash received
						</button>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="small quiet">Nothing waiting to be settled.</p>
		{/if}

		{#if settled.length}
			<details class="ledger">
				<summary class="label">Everything else ({settled.length})</summary>
				<ul class="till quiet-list">
					{#each settled as payment (payment.id)}
						<li>
							<div class="what">
								<strong>{payment.player_name || payment.player_username}</strong>
								<span class="small">
									{payment.court_name} · {formatDate(payment.starts_at)} · {payment.provider}
								</span>
							</div>
							<span class="amount num">{formatNPR(payment.amount_npr)}</span>
							<span class="chip" class:paid={payment.status === 'verified'}>{payment.status}</span>
						</li>
					{/each}
				</ul>
			</details>
		{/if}

		<h2 class="display display-m">Courts and rates</h2>
		<p class="small quiet">
			Open a court to change its details or set what it charges at different hours.
		</p>
		<ul class="courts">
			{#each courts as court (court.id)}
				<li><CourtPanel {court} onchange={invalidateAll} /></li>
			{:else}
				<li class="none small quiet">No courts yet. Add one below.</li>
			{/each}
		</ul>

		<form class="card" onsubmit={addCourt}>
			<h3 class="display display-m">Add a court</h3>
			<Field name="name" label="Name" bind:value={newCourt.name} error={fieldErrors.label}
				required maxlength="40" placeholder="Court A" />
			<Field name="format" label="Format" bind:value={newCourt.format}
				hint="What you call it — “5-a-side”, “Full court”. Left blank, it's derived from the side count."
				maxlength="40" placeholder="5-a-side" />
			<div class="pair">
				<Field name="side_count" label="A side" type="number" bind:value={newCourt.side_count}
					error={fieldErrors.side_count} min="3" max="11" required />
				<Field name="base_price_npr" label="Base rate (NPR/hr)" type="number"
					bind:value={newCourt.base_price_npr} error={fieldErrors.base_price} min="1" required />
			</div>
			<button class="btn btn-primary" class:loading={busy} disabled={busy}>Add</button>
		</form>

		<h2 class="display display-m">Venue details</h2>
		<form class="card venue" onsubmit={saveVenue}>
			<Field name="v-name" label="Name" bind:value={venue.name} error={fieldErrors.name}
				required maxlength="80" />
			<div class="pair">
				<Field name="v-area" label="Area" bind:value={venue.area} error={fieldErrors.area} required />
				<Field name="v-city" label="City" bind:value={venue.city} error={fieldErrors.city} />
			</div>
			<div class="pair">
				<Field name="v-opens" label="Opens" type="time" bind:value={venue.opens_at}
					error={fieldErrors.opens_at} required />
				<Field name="v-closes" label="Closes" type="time" bind:value={venue.closes_at}
					error={fieldErrors.closes_at} required />
			</div>
			<Field name="v-phone" label="Phone" bind:value={venue.phone} error={fieldErrors.phone}
				hint="Ten digits starting 98 or 97, or a landline." placeholder="9851000000" />
			<Field name="v-amenities" label="Amenities" bind:value={venue.amenities}
				error={fieldErrors.amenities} hint="Separated by commas."
				placeholder="Covered, Floodlit, Parking" />
			<Field name="v-cover" label="Cover image URL" bind:value={venue.cover_url} />
			<label class="pick" for="v-description">
				<span class="label">Description</span>
				<textarea id="v-description" bind:value={venue.description} maxlength="2000" rows="4"
					placeholder="Two covered courts behind the Jhamsikhel bus stop, laid with 40mm turf."
				></textarea>
			</label>
			<div class="pair">
				<Field name="v-lat" label="Latitude" type="number" step="any" bind:value={venue.lat}
					error={fieldErrors.lat} />
				<Field name="v-lng" label="Longitude" type="number" step="any" bind:value={venue.lng}
					error={fieldErrors.lng} />
			</div>
			<button class="btn btn-primary" class:loading={busy} disabled={busy}>
				{savedVenue ? 'Saved' : 'Save details'}
			</button>
			<p class="small quiet">
				The web address stays <code>{arena.slug}</code> — it's in every link anyone has shared.
			</p>
		</form>

		<hr />
		{#if !data.listing.is_active}
			<button class="btn btn-secondary" disabled={busy} onclick={() => setActive(true)}>
				Reopen this venue
			</button>
			<p class="small quiet">It's currently hidden from every listing.</p>
		{:else}
			<button class="btn btn-quiet danger" disabled={busy} onclick={() => setActive(false)}>
				Close this venue
			</button>
			<p class="small quiet">
				Hides it from every listing and stops new bookings. Hours already booked stand — people
				have plans.
			</p>
		{/if}
	</div>
</section>

<style>
	h1 { margin-top: 0.6rem; }
	.back { color: inherit; }
	.body { padding-block: clamp(2rem, 4vw, 3rem) var(--band); }

	h2 { margin: 0 0 0.75rem; }
	h2 ~ h2 { margin-top: 2.5rem; }
	h3 { margin: 0; }

	.till, .courts { display: grid; gap: 0.6rem; margin: 1rem 0 0; }

	.till li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem 1rem;
		padding: 0.9rem 1.1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.what { flex: 1 1 14rem; min-width: 0; }
	.what span { display: block; color: var(--faint); }

	.amount {
		font-size: 1.1rem;
		color: var(--ink);
	}

	.chip {
		padding: 0.2rem 0.7rem;
		border-radius: var(--r-pill);
		background: var(--surface-sunk);
		color: var(--faint);
		font-size: 0.78rem;
	}

	.chip.paid { background: var(--pine-wash); color: var(--pine-deep); }

	.ledger { margin-top: 1.25rem; }
	.ledger summary { cursor: pointer; color: var(--faint); }
	.quiet-list li { background: var(--field); }


	form {
		display: grid;
		gap: 1rem;
		max-width: 34rem;
		margin-top: 1.5rem;
		padding: clamp(1.5rem, 3vw, 2rem);
	}

	.venue { max-width: 40rem; }

	.pick { display: grid; gap: 0.35rem; }

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

	code {
		padding: 0.1rem 0.35rem;
		border-radius: 4px;
		background: var(--surface-sunk);
		font-size: 0.85em;
	}

	.none {
		padding: 0.9rem 1.1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
		text-align: center;
	}

	.pair { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }

	hr {
		height: 1px;
		margin: 2.5rem 0 1rem;
		border: 0;
		background: var(--line);
	}

	.danger { color: var(--brick); }
	.quiet { color: var(--muted); }

	.error {
		padding: 0.6rem 0.75rem;
		margin-bottom: 1.5rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}
</style>
