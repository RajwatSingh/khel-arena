<script>
	import { untrack } from 'svelte';
	import { invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import Field from '$lib/components/Field.svelte';
	import CourtPanel from '$lib/components/CourtPanel.svelte';
	import ExpandCard from '$lib/components/ExpandCard.svelte';
	import PaymentKeyForm from '$lib/components/PaymentKeyForm.svelte';
	import PaymentKeyStatus from '$lib/components/PaymentKeyStatus.svelte';
	import { formatDate, formatNPR, formatTime, localDate } from '$lib/time.js';

	let { data } = $props();

	const arena = $derived(data.arena);
	const courts = $derived(arena.courts ?? []);

	// The till. Cash bookings are confirmed the moment the player books, so
	// the venue's only job is collecting the cash (booking confirmed or
	// played). Requests still pending date from when venues approved cash
	// bookings by hand; they are shown only while any are left.
	const openCash = (p) => p.provider === 'cash' && p.status === 'initiated';
	const requests = $derived(
		data.payments
			.filter((p) => openCash(p) && p.booking_status === 'pending')
			.sort((a, b) => a.starts_at.localeCompare(b.starts_at))
	);
	const toCollect = $derived(
		data.payments
			.filter(
				(p) =>
					openCash(p) && (p.booking_status === 'confirmed' || p.booking_status === 'completed')
			)
			.sort((a, b) => a.starts_at.localeCompare(b.starts_at))
	);
	const settled = $derived(
		data.payments.filter((p) => !requests.includes(p) && !toCollect.includes(p))
	);

	let newCourt = $state({ name: '', format: '', side_count: 5, base_price_npr: 1200 });
	let newPhoto = $state({ caption: '' });
	let linkPhoto = $state({ url: '', caption: '', sort_order: 0 });
	// Each "add" form is a card that opens on click, and closes after a save.
	let addCourtOpen = $state(false);
	let uploadOpen = $state(false);
	let linkOpen = $state(false);
	let file = $state(null);

	function pickFile(event) {
		file = event.currentTarget.files?.[0] ?? null;
	}
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});
	let savedVenue = $state(false);
	// The venue card opens into its edit form. Closed by default: most visits
	// are for the till and the courts, not for renaming the place.
	let venueOpen = $state(false);

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
	const approve = (payment) => act(() => api.approveCashBooking(payment.id));
	const decline = (payment) => act(() => api.declineCashBooking(payment.id));
	const removePhoto = (id) => act(() => api.deletePhoto(id));

	// Per-venue gateway credentials. The money for a booking settles into the
	// account set here, not a platform one — that is the whole point of this
	// being on the venue's own page.
	const PROVIDERS = [
		['esewa', 'eSewa'],
		['khalti', 'Khalti']
	];

	const accountByProvider = $derived(
		Object.fromEntries((data.paymentAccounts ?? []).map((a) => [a.provider, a]))
	);

	// The provider whose key was saved in this visit, so its card can say so.
	// Changing or removing a key afterwards happens on the Payment keys page.
	let justActivated = $state(null);

	async function upload(event) {
		event.preventDefault();
		if (!file) return;

		const ok = await act(() =>
			api.uploadPhoto(arena.id, file, { caption: newPhoto.caption })
		);
		if (ok) {
			newPhoto = { caption: '' };
			uploadOpen = false;
			file = null;
			// The input keeps its selection otherwise, which reads as though
			// the upload had not happened.
			const input = document.getElementById('photo-file');
			if (input) input.value = '';
		}
	}

	async function addPhoto(event) {
		event.preventDefault();
		const ok = await act(() =>
			api.addPhoto(arena.id, { ...linkPhoto, sort_order: Number(linkPhoto.sort_order) })
		);
		if (ok) {
			linkPhoto = { url: '', caption: '', sort_order: 0 };
			linkOpen = false;
		}
	}
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
		if (ok) {
			newCourt = { name: '', format: '', side_count: 5, base_price_npr: 1200 };
			addCourtOpen = false;
		}
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

		{#if requests.length}
			<h2 class="display display-m">Older booking requests</h2>
			<p class="small quiet">
				Cash requests made before cash bookings were confirmed automatically. Approve to confirm
				the hour; one nobody answers lapses when the game was due to start.
			</p>
			<ul class="till">
				{#each requests as payment (payment.id)}
					<li>
						<div class="what">
							<strong>{payment.player_name || payment.player_username}</strong>
							<span class="small">
								{payment.court_name} · {formatDate(localDate(new Date(payment.starts_at)))}
								{formatTime(payment.starts_at)}–{formatTime(payment.ends_at)}
							</span>
						</div>
						<span class="amount num">{formatNPR(payment.amount_npr)}</span>
						<div class="decide">
							<button class="btn btn-primary" disabled={busy} onclick={() => approve(payment)}>
								Approve
							</button>
							<button class="btn btn-secondary" disabled={busy} onclick={() => decline(payment)}>
								Decline
							</button>
						</div>
					</li>
				{/each}
			</ul>
		{/if}

		<h2 class="display display-m">Cash to collect</h2>
		{#if toCollect.length}
			<p class="small quiet">
				Players who booked to pay cash at the venue. Mark each one once the player has paid.
			</p>
			<ul class="till">
				{#each toCollect as payment (payment.id)}
					<li>
						<div class="what">
							<strong>{payment.player_name || payment.player_username}</strong>
							<span class="small">
								{payment.court_name} · {formatDate(localDate(new Date(payment.starts_at)))}
								{formatTime(payment.starts_at)}{payment.booking_status === 'completed'
									? ' · played'
									: ''}
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
			<p class="small quiet">Nothing to collect.</p>
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
									{payment.court_name} · {formatDate(localDate(new Date(payment.starts_at)))} · {payment.provider}
								</span>
							</div>
							<span class="amount num">{formatNPR(payment.amount_npr)}</span>
							<span class="chip" class:paid={payment.status === 'verified'}>{payment.status}</span>
						</li>
					{/each}
				</ul>
			</details>
		{/if}

		<h2 class="display display-m">Taking payment</h2>
		<p class="small quiet">
			Your own eSewa or Khalti merchant keys. What a player pays for a court lands in the account
			you set here — nothing passes through Khel Arena. Leave both unset and every booking settles
			in cash, confirmed above.
		</p>
		<ul class="accounts">
			{#each PROVIDERS as [id, label] (id)}
				{@const saved = accountByProvider[id]}
				<li class="account card">
					<div class="account-head">
						<h3 class="display display-m">{label}</h3>
						{#if !saved}<span class="chip">not set up</span>{/if}
					</div>

					{#if saved}
						<PaymentKeyStatus {label} account={saved} fresh={justActivated === id} />
						<p class="small quiet change-note">
							To change or remove this key, open
							<a class="link" href="/payment-keys#venue-{arena.id}">Payment keys</a> from your
							profile menu.
						</p>
					{:else}
						<PaymentKeyForm
							arenaId={arena.id}
							provider={id}
							{label}
							submitLabel="Activate {label}"
							onsaved={(p) => (justActivated = p)}
						/>
					{/if}
				</li>
			{/each}
		</ul>

		<h2 class="display display-m">Courts and rates</h2>
		<p class="small quiet">
			Open a court to change its details or set what it charges at different hours.
		</p>
		<ul class="courts">
			{#each courts as court (court.id)}
				<li>
					<CourtPanel
						{court}
						onchange={invalidateAll}
						siblings={courts.filter((c) => c.id !== court.id)}
					/>
				</li>
			{:else}
				<li class="none small quiet">No courts yet. Add one below.</li>
			{/each}
		</ul>

		<div class="add-card">
			<ExpandCard bind:open={addCourtOpen} openLabel="Add">
				{#snippet lead()}<span class="plus" aria-hidden="true"></span>{/snippet}
				{#snippet summary()}
					<strong class="card-title">Add a court</strong>
					<span class="small card-meta">Name it, say how many a side, and set its base rate.</span>
				{/snippet}
				<form class="in-card" onsubmit={addCourt}>
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
					<button class="btn btn-primary" class:loading={busy} disabled={busy}>Add court</button>
				</form>
			</ExpandCard>
		</div>

		<h2 class="display display-m">Gallery</h2>
		<p class="small quiet">
			Photographs of the place, shown on the venue's page. Upload one, or link one you already
			host.
		</p>

		{#if data.photos.length}
			<ul class="gallery">
				{#each data.photos as photo (photo.id)}
					<li>
						<img src={photo.url} alt={photo.caption || 'The arena'} loading="lazy" />
						<span class="cap small">{photo.caption || '—'}</span>
						<button class="btn btn-quiet" disabled={busy} onclick={() => removePhoto(photo.id)}>
							Remove
						</button>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="small quiet none-inline">No photographs yet.</p>
		{/if}

		<div class="add-cards">
			<ExpandCard bind:open={uploadOpen} openLabel="Upload">
				{#snippet lead()}<span class="plus" aria-hidden="true"></span>{/snippet}
				{#snippet summary()}
					<strong class="card-title">Upload a photograph</strong>
					<span class="small card-meta">JPEG, PNG, WebP or GIF, up to 5 MB.</span>
				{/snippet}
				<form class="in-card" onsubmit={upload}>
					<label class="pick" for="photo-file">
						<span class="label">Image file</span>
						<input id="photo-file" type="file" accept="image/*" onchange={pickFile} />
						<span class="small quiet">JPEG, PNG, WebP or GIF, up to 5 MB.</span>
					</label>
					<Field name="upload-caption" label="Caption" bind:value={newPhoto.caption}
						maxlength="120" placeholder="The covered court, looking east" />
					<button class="btn btn-primary" class:loading={busy} disabled={busy || !file}>Upload</button>
				</form>
			</ExpandCard>

			<ExpandCard bind:open={linkOpen} openLabel="Add">
				{#snippet lead()}<span class="plus" aria-hidden="true"></span>{/snippet}
				{#snippet summary()}
					<strong class="card-title">Link a photo you host</strong>
					<span class="small card-meta">Paste the address of an image that's already online.</span>
				{/snippet}
				<form class="in-card" onsubmit={addPhoto}>
					<Field name="photo-url" label="Image address" bind:value={linkPhoto.url}
						error={fieldErrors.url} required placeholder="https://…/dhuku-court-a.jpg" />
					<Field name="photo-caption" label="Caption" bind:value={linkPhoto.caption}
						maxlength="120" placeholder="The covered court, looking east" />
					<Field name="photo-order" label="Position" type="number" bind:value={linkPhoto.sort_order}
						hint="Lower numbers come first." min="0" />
					<button class="btn btn-primary" class:loading={busy} disabled={busy}>Add photo</button>
				</form>
			</ExpandCard>
		</div>

		<h2 class="display display-m">Venue details</h2>
		<div class="add-card">
			<ExpandCard bind:open={venueOpen}>
				{#snippet lead()}
					<span class="cover" class:empty={!arena.cover_url}>
						{#if arena.cover_url}
							<img src={arena.cover_url} alt="" loading="lazy" />
						{:else}
							<span class="initial" aria-hidden="true">{arena.name.slice(0, 1)}</span>
						{/if}
					</span>
				{/snippet}
				{#snippet summary()}
					<strong class="card-title">{arena.name}</strong>
					<span class="small card-meta">
						{arena.area}{arena.city ? `, ${arena.city}` : ''} · open {arena.opens_at}–{arena.closes_at}
						{#if arena.phone}· {arena.phone}{/if}
					</span>
					{#if arena.amenities?.length}
						<span class="amenities">
							{#each arena.amenities as amenity, i (i)}
								<span class="chip">{amenity}</span>
							{/each}
						</span>
					{/if}
				{/snippet}

				<form class="venue-form in-card" onsubmit={saveVenue}>
					<div class="venue-col">
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
						<div class="pair">
							<Field name="v-lat" label="Latitude" type="number" step="any" bind:value={venue.lat}
								error={fieldErrors.lat} />
							<Field name="v-lng" label="Longitude" type="number" step="any" bind:value={venue.lng}
								error={fieldErrors.lng} />
						</div>
					</div>

					<div class="venue-col">
						<Field name="v-amenities" label="Amenities" bind:value={venue.amenities}
							error={fieldErrors.amenities} hint="Separated by commas."
							placeholder="Covered, Floodlit, Parking" />
						<Field name="v-cover" label="Cover image URL" bind:value={venue.cover_url} />
						<label class="pick" for="v-description">
							<span class="field-label">Description</span>
							<textarea id="v-description" bind:value={venue.description} maxlength="2000" rows="6"
								placeholder="Two covered courts behind the Jhamsikhel bus stop, laid with 40mm turf."
							></textarea>
						</label>
					</div>

					<div class="venue-actions">
						<button class="btn btn-primary" class:loading={busy} disabled={busy}>
							{savedVenue ? 'Saved' : 'Save details'}
						</button>
						<p class="small quiet">
							The web address stays <code>{arena.slug}</code> — it's in every link anyone has shared.
						</p>
					</div>
				</form>
			</ExpandCard>
		</div>

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
	.decide { display: flex; gap: 0.5rem; }
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

	.accounts {
		display: grid;
		gap: 1rem;
		margin: 1rem 0 2.5rem;
	}

	.account {
		display: grid;
		gap: 1rem;
		padding: clamp(1.25rem, 3vw, 1.6rem);
		max-width: 34rem;
	}

	.account-head {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.5rem 0.75rem;
	}

	.account-head h3 { margin: 0; }

	.change-note { margin: 0; }

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

	/* The expandable cards (ExpandCard) and what goes inside them. */
	.add-card { margin-top: 1rem; }

	.add-cards {
		display: grid;
		gap: 0.75rem;
		margin-top: 1rem;
	}

	.card-title { font-size: 1.0625rem; }
	.card-meta { color: var(--faint); }

	/* A drawn plus in a soft square, the "add" counterpart to a cover photo. */
	.plus {
		position: relative;
		width: 2.75rem;
		height: 2.75rem;
		border-radius: var(--r-sm);
		background: var(--pine-wash);
	}

	.plus::before,
	.plus::after {
		content: '';
		position: absolute;
		inset: 50% auto auto 50%;
		width: 1rem;
		height: 2px;
		border-radius: 1px;
		background: var(--pine);
		transform: translate(-50%, -50%);
	}

	.plus::after { transform: translate(-50%, -50%) rotate(90deg); }

	/* Forms inside a card drop the page's standalone-card padding and width. */
	form.in-card {
		max-width: 34rem;
		margin: 0;
		padding: 0;
	}

	.cover {
		flex: none;
		width: 7rem;
		aspect-ratio: 4 / 3;
		overflow: hidden;
		border-radius: var(--r-sm);
		background: var(--surface-sunk);
	}

	.cover.empty {
		display: grid;
		place-items: center;
	}

	.initial {
		font-family: var(--display);
		font-size: 1.75rem;
		font-weight: 700;
		color: var(--faint);
	}

	/* Matches the labels Field draws, so the textarea sits in with them. */
	.field-label {
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--ink);
	}

	.cover img {
		display: block;
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.amenities {
		display: flex;
		flex-wrap: wrap;
		gap: 0.35rem;
		margin-top: 0.15rem;
	}

	/* The venue form uses the card's full width, in two columns when it can. */
	form.venue-form {
		max-width: none;
		gap: 1rem 2rem;
	}

	.venue-col {
		display: grid;
		gap: 1rem;
		align-content: start;
		min-width: 0;
	}

	.venue-actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem 1rem;
	}

	.venue-actions p { margin: 0; }

	@media (min-width: 52rem) {
		form.venue-form { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
		.venue-actions { grid-column: 1 / -1; }
	}

	@media (max-width: 34rem) {
		.cover { width: 4.5rem; }
	}

	.gallery {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
		gap: 0.75rem;
		margin: 1rem 0 0;
	}

	.gallery li {
		display: grid;
		gap: 0.4rem;
		padding: 0.6rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.gallery img {
		width: 100%;
		aspect-ratio: 4 / 3;
		object-fit: cover;
		border-radius: var(--r-sm);
		background: var(--surface-sunk);
	}

	.cap { color: var(--faint); }
	.none-inline { margin-top: 0.75rem; }

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
