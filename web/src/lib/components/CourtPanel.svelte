<script>
	/**
	 * One court in the back office: its details, editable, and the rate
	 * windows that price it.
	 *
	 * A `<details>` rather than a separate page. A venue has a handful of
	 * courts and an owner changing a rate wants to see the others while they
	 * do it — a route per court would lose that for no gain.
	 */
	import { untrack } from 'svelte';
	import { api, ApiError } from '$lib/api/index.js';
	import Field from './Field.svelte';
	import Listbox from './Listbox.svelte';
	import { formatNPR } from '$lib/time.js';

	let { court, onchange, siblings = [] } = $props();

	// ISO weekdays, which is what the wire speaks: Monday is 1, Sunday is 7.
	const DAYS = [
		{ iso: 1, short: 'Mon' },
		{ iso: 2, short: 'Tue' },
		{ iso: 3, short: 'Wed' },
		{ iso: 4, short: 'Thu' },
		{ iso: 5, short: 'Fri' },
		{ iso: 6, short: 'Sat' },
		{ iso: 7, short: 'Sun' }
	];

	// Seeded from the court as it stands, so the form is an edit rather than a
	// blank slate — the endpoint replaces every field, and a form that started
	// empty would blank the ones nobody touched.
	//
	// `untrack` because reading the prop once is the point: a reload landing
	// while somebody is halfway through typing must not throw their edit away.
	// The `{#each}` keys panels by court id, so a different court gets a fresh
	// component rather than a stale form.
	let edit = $state(
		untrack(() => ({
			name: court.name,
			format: court.format ?? '',
			surface: court.surface ?? '',
			sport: court.sport,
			side_count: court.side_count,
			base_price_npr: court.base_price_npr
		}))
	);

	let rule = $state(
		untrack(() => ({
			label: '',
			days: [1, 2, 3, 4, 5],
			start_hour: 17,
			end_hour: 21,
			price_npr: court.base_price_npr,
			is_peak: true,
			priority: 10
		}))
	);

	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});
	let saved = $state(false);

	async function act(fn) {
		busy = true;
		error = null;
		fieldErrors = {};
		try {
			await fn();
			await onchange();
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

	async function save(event) {
		event.preventDefault();
		const ok = await act(() =>
			api.updateCourt(court.id, {
				...edit,
				side_count: Number(edit.side_count),
				base_price_npr: Number(edit.base_price_npr)
			})
		);
		if (ok) {
			saved = true;
			setTimeout(() => (saved = false), 2000);
		}
	}

	function toggleDay(iso) {
		rule.days = rule.days.includes(iso)
			? rule.days.filter((d) => d !== iso)
			: [...rule.days, iso].sort((a, b) => a - b);
	}

	async function addRule(event) {
		event.preventDefault();
		const ok = await act(() =>
			api.createPricingRule(court.id, {
				...rule,
				start_hour: Number(rule.start_hour),
				end_hour: Number(rule.end_hour),
				price_npr: Number(rule.price_npr),
				priority: Number(rule.priority)
			})
		);
		if (ok) rule = { ...rule, label: '' };
	}

	const dropRule = (id) => act(() => api.deletePricingRule(id));

	// Setting the same four windows on five identical courts is the tedious
	// part of running a venue. This appends rather than replaces — overlapping
	// windows are already resolved by priority, and silently deleting what was
	// there would be the worse surprise.
	let copyFrom = $state('');
	let copied = $state(0);

	async function copyCard(event) {
		event.preventDefault();
		if (!copyFrom) return;

		const before = copyFrom;
		const ok = await act(async () => {
			const result = await api.copyPricingRules(court.id, before);
			copied = result.copied;
		});
		if (ok) {
			copyFrom = '';
			setTimeout(() => (copied = 0), 3000);
		}
	}

	/** "Mon–Fri" where the days run together, "Mon, Wed, Sat" where they don't. */
	function describeDays(days) {
		if (!days?.length) return '';
		const sorted = [...days].sort((a, b) => a - b);
		const contiguous = sorted.every((d, i) => i === 0 || d === sorted[i - 1] + 1);
		const name = (iso) => DAYS.find((d) => d.iso === iso)?.short ?? iso;

		if (sorted.length === 7) return 'Every day';
		if (contiguous && sorted.length > 2) return `${name(sorted[0])}–${name(sorted.at(-1))}`;
		return sorted.map(name).join(', ');
	}

	const hour = (h) => String(h).padStart(2, '0') + ':00';
</script>

<details class="court">
	<summary>
		<span class="what">
			<strong>{court.name}</strong>
			<span class="small">
				{court.format} · {court.surface} · base {formatNPR(court.base_price_npr)}/hr
				{#if court.rules?.length}
					· {court.rules.length}
					{court.rules.length === 1 ? 'rate window' : 'rate windows'}
				{/if}
			</span>
		</span>
		<span class="chev" aria-hidden="true"></span>
	</summary>

	<div class="body">
		{#if error}
			<p class="error small" role="alert">{error}</p>
		{/if}

		<form onsubmit={save}>
			<h4 class="label">Details</h4>
			<Field name="court-name-{court.id}" label="Name" bind:value={edit.name}
				error={fieldErrors.label} required maxlength="40" />
			<div class="pair">
				<Field name="court-format-{court.id}" label="Format" bind:value={edit.format}
					maxlength="40" placeholder="5-a-side" />
				<Field name="court-surface-{court.id}" label="Surface" bind:value={edit.surface}
					placeholder="40mm turf" />
			</div>
			<div class="pair">
				<Field name="court-side-{court.id}" label="A side" type="number"
					bind:value={edit.side_count} error={fieldErrors.side_count} min="3" max="11" required />
				<Field name="court-price-{court.id}" label="Base rate (NPR/hr)" type="number"
					bind:value={edit.base_price_npr} error={fieldErrors.base_price} min="1" required />
			</div>
			<button class="btn btn-secondary" class:loading={busy} disabled={busy}>
				{saved ? 'Saved' : 'Save court'}
			</button>
		</form>

		<div class="rates">
			<h4 class="label">Rate windows</h4>
			<p class="small quiet">
				Where two overlap the higher priority wins, and a tie goes to the narrower window. Hours
				already booked keep the price they were booked at.
			</p>

			{#if court.rules?.length}
				<ul>
					{#each court.rules as r (r.id)}
						<li>
							<span class="rule-what">
								<strong>{r.label}</strong>
								<span class="small">
									{describeDays(r.days)} · {hour(r.start_hour)}–{hour(r.end_hour)}
									{#if r.is_peak}· peak{/if}
									· priority {r.priority}
								</span>
							</span>
							<span class="rule-price num">{formatNPR(r.price_npr)}</span>
							<button class="btn btn-quiet" disabled={busy} onclick={() => dropRule(r.id)}>
								Remove
							</button>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="small quiet none">
					No rate windows. Every hour is charged at the base rate.
				</p>
			{/if}

			{#if siblings.length}
				<form class="copy" onsubmit={copyCard}>
					<label class="pick">
						<span class="label">Copy a rate card from</span>
						<Listbox
							value={copyFrom}
							options={[
								{ value: '', label: 'Another court…' },
								...siblings.map((c) => ({
									value: c.id,
									label: `${c.name} (${c.rules?.length ?? 0} window${(c.rules?.length ?? 0) === 1 ? '' : 's'})`
								}))
							]}
							label="Source court"
							fill
							onselect={(v) => (copyFrom = v)}
						/>
					</label>
					<button class="btn btn-quiet" disabled={busy || !copyFrom}>
						{copied ? `Copied ${copied}` : 'Copy'}
					</button>
				</form>
			{/if}

			<form class="new-rule" onsubmit={addRule}>
				<Field name="rule-label-{court.id}" label="What is it called?" bind:value={rule.label}
					error={fieldErrors.label} required placeholder="Evening peak" />

				<fieldset class="days">
					<legend class="label">Days</legend>
					{#each DAYS as day (day.iso)}
						<button
							type="button"
							class="day"
							class:on={rule.days.includes(day.iso)}
							aria-pressed={rule.days.includes(day.iso)}
							onclick={() => toggleDay(day.iso)}
						>
							{day.short}
						</button>
					{/each}
					{#if fieldErrors.days}<p class="small err">{fieldErrors.days}</p>{/if}
				</fieldset>

				<div class="quad">
					<Field name="rule-start-{court.id}" label="From (hour)" type="number"
						bind:value={rule.start_hour} error={fieldErrors.start_hour} min="0" max="23" required />
					<Field name="rule-end-{court.id}" label="To (hour)" type="number"
						bind:value={rule.end_hour} error={fieldErrors.end_hour} min="1" max="24" required />
					<Field name="rule-price-{court.id}" label="Rate (NPR/hr)" type="number"
						bind:value={rule.price_npr} error={fieldErrors.price_npr} min="1" required />
					<Field name="rule-priority-{court.id}" label="Priority" type="number"
						bind:value={rule.priority} min="0" required />
				</div>

				<label class="peak">
					<input type="checkbox" bind:checked={rule.is_peak} />
					<span>Show this as a peak rate</span>
				</label>

				<button class="btn btn-secondary" class:loading={busy} disabled={busy}>
					Add rate window
				</button>
			</form>
		</div>
	</div>
</details>

<style>
	.court {
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.court[open] {
		border-color: var(--line-strong);
	}

	summary {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 0.9rem 1.1rem;
		cursor: pointer;
		list-style: none;
	}

	summary::-webkit-details-marker {
		display: none;
	}

	.what {
		flex: 1;
		min-width: 0;
	}

	.what span {
		display: block;
		color: var(--faint);
	}

	/* A chevron drawn rather than typed, so it turns with the disclosure
	   instead of swapping glyph. */
	.chev {
		flex: none;
		width: 0.5rem;
		height: 0.5rem;
		border-right: 2px solid var(--faint);
		border-bottom: 2px solid var(--faint);
		transform: rotate(45deg) translate(-2px, -2px);
		transition: transform var(--dur-hover) var(--ease);
	}

	.court[open] .chev {
		transform: rotate(-135deg) translate(-2px, -2px);
	}

	.body {
		display: grid;
		gap: 1.5rem;
		padding: 0 1.1rem 1.25rem;
		border-top: 1px solid var(--line);
		padding-top: 1.25rem;
	}

	@media (min-width: 52rem) {
		.body {
			grid-template-columns: minmax(0, 20rem) minmax(0, 1fr);
			gap: 2rem;
		}
	}

	form {
		display: grid;
		gap: 0.85rem;
		align-content: start;
	}

	h4 {
		margin: 0;
	}

	.pair {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.75rem;
	}

	.quad {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(6.5rem, 1fr));
		gap: 0.75rem;
	}

	.rates {
		display: grid;
		gap: 0.75rem;
		align-content: start;
	}

	ul {
		display: grid;
		gap: 0.5rem;
	}

	li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem 0.9rem;
		padding: 0.65rem 0.85rem;
		border: 1px solid var(--line);
		border-radius: var(--r-sm);
		background: var(--field);
	}

	.rule-what {
		flex: 1 1 12rem;
		min-width: 0;
	}

	.rule-what span {
		display: block;
		color: var(--faint);
	}

	.rule-price {
		color: var(--ink);
	}

	.none {
		padding: 0.65rem 0;
	}

	.new-rule {
		gap: 0.85rem;
		padding-top: 1rem;
		border-top: 1px dashed var(--line);
	}

	.copy {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		gap: 0.6rem;
		padding-top: 1rem;
		border-top: 1px dashed var(--line);
	}

	.copy .pick { flex: 1 1 12rem; }
	.pick { display: grid; gap: 0.35rem; }

	.days {
		display: flex;
		flex-wrap: wrap;
		gap: 0.35rem;
		padding: 0;
		border: 0;
		margin: 0;
	}

	legend {
		width: 100%;
		margin-bottom: 0.35rem;
		padding: 0;
	}

	.day {
		padding: 0.35rem 0.6rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-pill);
		background: var(--surface);
		font: inherit;
		font-size: 0.82rem;
		color: var(--muted);
		cursor: pointer;
		transition:
			background var(--dur-hover) var(--ease),
			color var(--dur-hover) var(--ease),
			border-color var(--dur-hover) var(--ease);
	}

	.day.on {
		border-color: var(--pine);
		background: var(--pine-wash);
		color: var(--pine-deep);
	}

	.peak {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.9rem;
		color: var(--muted);
	}

	.err {
		width: 100%;
		margin: 0.35rem 0 0;
		color: var(--brick);
	}

	.quiet {
		color: var(--muted);
	}

	.error {
		padding: 0.6rem 0.75rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}
</style>
