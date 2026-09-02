<script>
	/**
	 * A team's results, and the captain's way of filing one.
	 *
	 * A score counts only once both captains have agreed it, so each row says
	 * whose turn it is: the captain who filed sees "waiting on them", the
	 * other sees a confirm button.
	 */
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import Field from './Field.svelte';
	import Listbox from './Listbox.svelte';
	import { formatDate } from '$lib/time.js';

	let { team, captain } = $props();

	let matches = $state([]);
	let opponents = $state([]);
	let form = $state({ opponent: '', home_score: 0, away_score: 0, at_home: true });
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});
	let version = $state(0);

	$effect(() => {
		void version;
		const id = team.id;
		let cancelled = false;

		api
			.teamMatches(id)
			.then((next) => {
				if (!cancelled) matches = next;
			})
			.catch(() => {
				if (!cancelled) matches = [];
			});

		// Opponents come from the table: every team with a squad is a possible
		// fixture, and the standings list is the only public roll of teams.
		api
			.standings()
			.then((next) => {
				if (!cancelled) opponents = next.filter((s) => s.team_id !== id);
			})
			.catch(() => {
				if (!cancelled) opponents = [];
			});

		return () => {
			cancelled = true;
		};
	});

	async function act(fn) {
		busy = true;
		error = null;
		fieldErrors = {};
		try {
			await fn();
			version++;
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

	async function report(event) {
		event.preventDefault();
		if (!form.opponent) {
			error = 'Pick who you played.';
			return;
		}

		const ok = await act(() =>
			api.reportMatch({
				home_team: form.at_home ? team.id : form.opponent,
				away_team: form.at_home ? form.opponent : team.id,
				home_score: Number(form.home_score),
				away_score: Number(form.away_score),
				// Now, because a result is filed after the game. The server
				// refuses anything in the future.
				played_at: new Date().toISOString()
			})
		);
		if (ok) form = { ...form, home_score: 0, away_score: 0 };
	}

	const confirm = (id) => act(() => api.confirmMatch(id));
	const withdraw = (id) => act(() => api.withdrawMatch(id));

	// Disputing is countering with your version, not rejecting theirs: the
	// result goes back to the other captain to agree or counter again.
	let disputing = $state(null);
	let counter = $state({ home_score: 0, away_score: 0 });

	function openDispute(match) {
		disputing = match.id;
		counter = { home_score: match.home_score, away_score: match.away_score };
	}

	async function sendDispute(event) {
		event.preventDefault();
		const ok = await act(() =>
			api.disputeMatch(disputing, Number(counter.home_score), Number(counter.away_score))
		);
		if (ok) disputing = null;
	}

	/** Whose turn it is on an unagreed result. */
	function awaiting(match) {
		if (match.verified) return null;
		return match.reported_by === session.user?.id ? 'them' : 'you';
	}
</script>

<section class="results">
	<h2 class="display display-m">Results</h2>

	{#if error}
		<p class="error small" role="alert">{error}</p>
	{/if}

	{#if matches.length}
		<ul>
			{#each matches as match (match.id)}
				{@const turn = awaiting(match)}
				<li class:pending={!match.verified}>
					<span class="score">
						<span class="side" class:won={match.home_score > match.away_score}>
							<span class="tag num">{match.home_tag}</span>
							{match.home_name}
						</span>
						<span class="num line">{match.home_score}–{match.away_score}</span>
						<span class="side away" class:won={match.away_score > match.home_score}>
							{match.away_name}
							<span class="tag num">{match.away_tag}</span>
						</span>
					</span>

					<span class="meta">
						<span class="small">{formatDate(match.played_at)}</span>
						{#if match.verified}
							<span class="chip">Agreed</span>
						{:else if turn === 'you' && captain}
							<button class="btn btn-secondary" disabled={busy} onclick={() => confirm(match.id)}>
								Confirm
							</button>
							<button class="btn btn-quiet" disabled={busy} onclick={() => openDispute(match)}>
								Wrong score
							</button>
						{:else if turn === 'them'}
							<span class="chip chip-quiet">Waiting on them</span>
							{#if captain}
								<button class="btn btn-quiet" disabled={busy} onclick={() => withdraw(match.id)}>
									Withdraw
								</button>
							{/if}
						{:else}
							<span class="chip chip-quiet">Unconfirmed</span>
						{/if}
					</span>
					{#if disputing === match.id}
						<form class="counter" onsubmit={sendDispute}>
							<p class="small quiet">
								What was the score? It goes back to them to agree.
							</p>
							<div class="counter-scores">
								<label>
									<span class="label">{match.home_tag}</span>
									<input type="number" min="0" bind:value={counter.home_score} required />
								</label>
								<span class="dash">–</span>
								<label>
									<span class="label">{match.away_tag}</span>
									<input type="number" min="0" bind:value={counter.away_score} required />
								</label>
								<button class="btn btn-secondary" disabled={busy}>Send</button>
								<button type="button" class="btn btn-quiet" onclick={() => (disputing = null)}>
									Cancel
								</button>
							</div>
						</form>
					{/if}
				</li>
			{/each}
		</ul>
	{:else}
		<p class="small quiet">No results yet.</p>
	{/if}

	{#if captain}
		<form class="card" onsubmit={report}>
			<h3 class="label">File a result</h3>
			<p class="small quiet">
				The other captain confirms it. Until they do it doesn't reach
				<a class="link" href="/standings">the table</a>.
			</p>

			{#if opponents.length}
				<label class="pick">
					<span class="label">Who did you play?</span>
					<Listbox
						value={form.opponent}
						options={[
							{ value: '', label: 'Pick a team' },
							...opponents.map((o) => ({ value: o.team_id, label: `${o.tag} · ${o.name}` }))
						]}
						label="Opponent"
						fill
						onselect={(v) => (form.opponent = v)}
					/>
				</label>

				<label class="where">
					<input type="checkbox" bind:checked={form.at_home} />
					<span>We were at home</span>
				</label>

				<div class="pair">
					<Field
						name="home_score"
						label={form.at_home ? 'Us' : 'Them'}
						type="number"
						bind:value={form.home_score}
						error={fieldErrors.home_score}
						min="0"
						required
					/>
					<Field
						name="away_score"
						label={form.at_home ? 'Them' : 'Us'}
						type="number"
						bind:value={form.away_score}
						error={fieldErrors.away_score}
						min="0"
						required
					/>
				</div>

				<button class="btn btn-primary" class:loading={busy} disabled={busy}>File it</button>
			{:else}
				<p class="small quiet">
					There's nobody to play yet — another squad has to exist before a result can be filed.
				</p>
			{/if}
		</form>
	{/if}
</section>

<style>
	.results { margin-top: clamp(2rem, 4vw, 3rem); }
	h2 { margin: 0 0 1rem; }
	h3 { margin: 0; }

	ul { display: grid; gap: 0.6rem; margin-bottom: 1.5rem; }

	li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem 1rem;
		padding: 0.85rem 1.1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	/* An unagreed result is provisional, and looks it. */
	li.pending {
		border-style: dashed;
		background: var(--field);
	}

	.score {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex: 1 1 16rem;
		min-width: 0;
	}

	.side {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		min-width: 0;
		color: var(--muted);
	}

	.side.away { justify-content: flex-end; }
	.side.won { color: var(--ink); font-weight: 600; }

	.line { flex: none; font-size: 1.1rem; }

	.tag {
		display: grid;
		place-items: center;
		flex: none;
		width: 2.4rem;
		padding: 0.1rem 0;
		border-radius: 4px;
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.7rem;
		letter-spacing: 0.04em;
	}

	.meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.4rem 0.75rem;
	}

	.meta .small { color: var(--faint); }

	.chip {
		padding: 0.2rem 0.7rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.78rem;
	}

	.chip-quiet { background: var(--surface-sunk); color: var(--faint); }

	/* The counter-form drops beneath the row it belongs to rather than
	   replacing it: you want to see the score you are arguing with. */
	.counter {
		flex-basis: 100%;
		display: grid;
		gap: 0.5rem;
		padding-top: 0.75rem;
		margin-top: 0.25rem;
		border-top: 1px dashed var(--line-strong);
	}

	.counter-scores {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		gap: 0.5rem;
	}

	.counter-scores label { display: grid; gap: 0.2rem; }

	.counter-scores input {
		width: 4rem;
		padding: 0.4rem 0.5rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-sm);
		background: var(--surface);
		font: inherit;
		text-align: center;
	}

	.dash { align-self: center; color: var(--faint); }

	form {
		display: grid;
		gap: 0.85rem;
		max-width: 30rem;
		padding: clamp(1.25rem, 3vw, 1.75rem);
	}

	.pick { display: grid; gap: 0.35rem; }
	.pair { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }

	.where {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.9rem;
		color: var(--muted);
	}

	.quiet { color: var(--muted); }

	.error {
		padding: 0.6rem 0.75rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}
</style>
