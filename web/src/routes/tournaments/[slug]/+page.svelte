<script>
	import { invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import Listbox from '$lib/components/Listbox.svelte';
	import { formatDateLong, formatNPR } from '$lib/time.js';
	import { SKILL_LABELS } from '$lib/skills.js';

	let { data } = $props();

	const t = $derived(data.tournament);
	const organiser = $derived(session.user?.id === t.organizer_id);

	// Only squads the viewer captains can be entered, and only ones not
	// already in the bracket — the server enforces both, and offering an
	// option it will refuse is a worse experience than not offering it.
	const entered = $derived(new Set(t.entries.map((e) => e.team_id)));
	const eligible = $derived(
		data.teams.filter((team) => team.captain_id === session.user?.id && !entered.has(team.id))
	);

	let teamId = $state('');
	let busy = $state(false);
	let error = $state(null);

	$effect(() => {
		if (!teamId && eligible.length) teamId = eligible[0].id;
	});

	async function act(fn) {
		busy = true;
		error = null;
		try {
			await fn();
			await invalidateAll();
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'That did not go through.';
		} finally {
			busy = false;
		}
	}

	const enter = () => act(() => api.registerTeam(t.id, teamId));
	const withdraw = (id) => act(() => api.withdrawTeam(t.id, id));
	const togglePaid = (entry) => act(() => api.setEntryPaid(t.id, entry.team_id, !entry.paid));
	const setStatus = (status) => act(() => api.setTournamentStatus(t.id, status));
</script>

<svelte:head>
	<title>{t.name} | Khel Arena</title>
	<meta name="description" content="{t.name} — {formatDateLong(t.starts_on)}, {t.side_count}-a-side." />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up"><a class="back" href="/tournaments">The fixture list</a></p>
		<h1 class="display display-l"><TextAnimate text={t.name} /></h1>
		<p class="lede fade-up fade-up-1">
			{formatDateLong(t.starts_on)} · {t.side_count}-a-side {t.format}
			{#if t.arena_name}· {t.arena_name}, {t.arena_area}{/if}
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="body">
	<div class="shell split">
		<div>
			{#if error}
				<p class="error small" role="alert">{error}</p>
			{/if}

			{#if t.description}<p class="prose">{t.description}</p>{/if}

			<dl class="facts">
				<div><dt class="label">Entry</dt><dd class="num">{t.entry_fee_npr ? formatNPR(t.entry_fee_npr) : 'Free'}</dd></div>
				<div><dt class="label">Prize pot</dt><dd class="num">{t.prize_pool_npr ? formatNPR(t.prize_pool_npr) : '—'}</dd></div>
				<div><dt class="label">Split</dt><dd class="num">{t.prize_split.join(' / ')}</dd></div>
				<div><dt class="label">Standard</dt><dd>{SKILL_LABELS[t.skill] ?? t.skill}</dd></div>
				<div><dt class="label">Squad cap</dt><dd class="num">{t.squad_cap}</dd></div>
				<div><dt class="label">Entries close</dt><dd>{formatDateLong(t.register_by)}</dd></div>
			</dl>

			<h2 class="display display-m">Teams in</h2>
			{#if t.entries.length}
				<ul class="entries">
					{#each t.entries as entry (entry.team_id)}
						<li>
							<span class="tag num">{entry.team_tag}</span>
							<span class="who"><strong>{entry.team_name}</strong></span>
							<span class="right">
								{#if entry.paid}
									<span class="chip">Paid</span>
								{:else}
									<span class="chip chip-quiet">Unpaid</span>
								{/if}
								{#if organiser}
									<button class="btn btn-quiet" disabled={busy} onclick={() => togglePaid(entry)}>
										{entry.paid ? 'Mark unpaid' : 'Mark paid'}
									</button>
									<button class="btn btn-quiet" disabled={busy} onclick={() => withdraw(entry.team_id)}>
										Remove
									</button>
								{:else if data.teams.some((x) => x.id === entry.team_id && x.captain_id === session.user?.id)}
									<button class="btn btn-quiet" disabled={busy} onclick={() => withdraw(entry.team_id)}>
										Withdraw
									</button>
								{/if}
							</span>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="small quiet">Nobody has entered yet.</p>
			{/if}

			{#if t.rules}
				<h2 class="display display-m">Rules</h2>
				<p class="prose rules">{t.rules}</p>
			{/if}
		</div>

		<aside class="panel card">
			<p class="slots num" class:full={t.slots_remaining === 0}>{t.slots_remaining}</p>
			<p class="label">of {t.max_teams} slots left</p>

			{#if organiser}
				<hr />
				<p class="label">You organise this</p>
				{#if t.status === 'open' || t.status === 'full'}
					<button class="btn btn-primary grow" disabled={busy} onclick={() => setStatus('ongoing')}>
						Start it
					</button>
				{:else if t.status === 'ongoing'}
					<button class="btn btn-primary grow" disabled={busy} onclick={() => setStatus('completed')}>
						Mark finished
					</button>
				{/if}
				<button class="btn btn-quiet grow danger" disabled={busy} onclick={() => setStatus('cancelled')}>
					Call it off
				</button>
			{:else if !session.signedIn}
				<a class="btn btn-primary grow" href="/login">Sign in to enter</a>
			{:else if eligible.length}
				{#if eligible.length > 1}
					<Listbox
						value={teamId}
						options={eligible.map((x) => ({ value: x.id, label: `${x.tag} · ${x.name}` }))}
						label="Team"
						fill
						onselect={(v) => (teamId = v)}
					/>
				{/if}
				<button class="btn btn-primary grow" class:loading={busy} disabled={busy} onclick={enter}>
					Enter {eligible.length === 1 ? eligible[0].name : 'this team'}
				</button>
			{:else}
				<p class="small quiet">
					Only a captain can enter a squad.
					<a class="link" href="/teams">Start a team</a> if you haven't got one.
				</p>
			{/if}
		</aside>
	</div>
</section>

<style>
	h1 { margin-top: 0.6rem; }
	.back { color: inherit; }

	.body { padding-block: clamp(2rem, 4vw, 3rem) var(--band); }

	.split {
		display: grid;
		gap: clamp(1.5rem, 4vw, 3rem);
		align-items: start;
	}

	@media (min-width: 60rem) {
		.split { grid-template-columns: minmax(0, 1fr) 20rem; }
	}

	.facts {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
		gap: 1rem 1.5rem;
		margin: clamp(1.5rem, 3vw, 2rem) 0;
		padding-top: 1.25rem;
		border-top: 1px solid var(--line);
	}

	dd { margin: 0.2rem 0 0; }

	h2 { margin: 2rem 0 1rem; }

	.entries { display: grid; gap: 0.6rem; }

	.entries li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
		padding: 0.75rem 1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.tag {
		display: grid;
		place-items: center;
		flex: none;
		width: 2.75rem;
		height: 2.25rem;
		border-radius: var(--r-sm);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.8rem;
		letter-spacing: 0.04em;
	}

	.who { flex: 1 1 8rem; min-width: 0; }

	.right {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.4rem;
		margin-left: auto;
	}

	.chip {
		padding: 0.2rem 0.7rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.78rem;
	}

	.chip-quiet { background: var(--surface-sunk); color: var(--faint); }

	.rules { white-space: pre-wrap; }

	.panel {
		display: grid;
		gap: 0.6rem;
		padding: clamp(1.25rem, 3vw, 1.75rem);
	}

	.slots {
		margin: 0;
		font-size: clamp(2.5rem, 6vw, 3.5rem);
		line-height: 1;
		color: var(--pine-deep);
	}

	.slots.full { color: var(--faint); }

	hr {
		width: 100%;
		height: 1px;
		margin: 0.5rem 0;
		border: 0;
		background: var(--line);
	}

	.grow { width: 100%; }
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
