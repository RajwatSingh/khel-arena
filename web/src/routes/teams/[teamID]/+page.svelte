<script>
	import { goto, invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import ResultsPanel from '$lib/components/ResultsPanel.svelte';
	import { formatDate } from '$lib/time.js';

	let { data } = $props();

	const team = $derived(data.team);
	const captain = $derived(session.user?.id === team.captain_id);

	let busy = $state(false);
	let error = $state(null);
	let copied = $state(false);

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

	const remove = (userId) => act(() => api.removeTeamMember(team.id, userId));
	const promote = (userId) => act(() => api.transferCaptaincy(team.id, userId));
	const rotate = () => act(() => api.rotateJoinCode(team.id));

	async function leave() {
		await act(() => api.removeTeamMember(team.id, session.user.id));
		if (!error) goto('/teams');
	}

	async function disband() {
		await act(() => api.disbandTeam(team.id));
		if (!error) goto('/teams');
	}

	async function copyCode() {
		try {
			await navigator.clipboard.writeText(team.join_code);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			// Clipboard access can be refused; the code is on screen to read.
			copied = false;
		}
	}
</script>

<svelte:head>
	<title>{team.name} | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up"><a class="back" href="/teams">The squad book</a></p>
		<h1 class="display display-l"><TextAnimate text={team.name} /></h1>
		<p class="lede fade-up fade-up-1">
			<span class="tag num">{team.tag}</span>
			{team.member_count}
			{team.member_count === 1 ? 'player' : 'players'} on the roster.
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

			<h2 class="display display-m">Roster</h2>
			<ul class="roster">
				{#each team.members as member (member.user_id)}
					<li>
						<div class="player">
							<strong>{member.full_name || member.username}</strong>
							<span class="small">@{member.username} · joined {formatDate(member.joined_at)}</span>
						</div>

						<div class="actions">
							{#if member.role === 'captain'}
								<span class="chip">Captain</span>
							{:else if captain}
								<button class="btn btn-quiet" disabled={busy} onclick={() => promote(member.user_id)}>
									Hand over armband
								</button>
								<button class="btn btn-quiet" disabled={busy} onclick={() => remove(member.user_id)}>
									Remove
								</button>
							{:else if member.user_id === session.user?.id}
								<button class="btn btn-quiet" disabled={busy} onclick={leave}>Leave</button>
							{/if}
						</div>
					</li>
				{/each}
			</ul>

			<ResultsPanel {team} {captain} />
		</div>

		<aside class="panel card">
			{#if team.join_code}
				<h2 class="display display-m">Invite</h2>
				<p class="small quiet">
					Anyone with this code can join the squad. Rotate it if it gets out.
				</p>
				<button class="code num" onclick={copyCode} title="Copy">
					{team.join_code}
					<span class="label">{copied ? 'copied' : 'tap to copy'}</span>
				</button>
				{#if captain}
					<button class="btn btn-secondary grow" disabled={busy} onclick={rotate}>
						New code
					</button>
				{/if}
			{/if}

			{#if captain}
				<hr />
				<button class="btn btn-quiet grow danger" disabled={busy} onclick={disband}>
					Disband the team
				</button>
				<p class="small quiet">
					Bookings the squad made stay booked — somebody still has to turn up.
				</p>
			{/if}
		</aside>
	</div>
</section>

<style>
	h1 {
		margin-top: 0.6rem;
	}

	.back {
		color: inherit;
	}

	.head .lede {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		margin-top: 1rem;
	}

	.tag {
		padding: 0.2rem 0.6rem;
		border-radius: var(--r-sm);
		background: rgba(255, 255, 255, 0.12);
		color: var(--accent-on-field);
		font-size: 0.85rem;
		letter-spacing: 0.04em;
	}

	.body {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	.split {
		display: grid;
		gap: clamp(1.5rem, 4vw, 3rem);
		align-items: start;
	}

	@media (min-width: 60rem) {
		.split {
			grid-template-columns: minmax(0, 1fr) 20rem;
		}
	}

	h2 {
		margin: 0 0 1rem;
	}

	.roster {
		display: grid;
		gap: 0.6rem;
	}

	.roster li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 0.9rem 1.1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.player span {
		display: block;
		color: var(--faint);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}

	.chip {
		padding: 0.2rem 0.7rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.78rem;
	}

	.panel {
		display: grid;
		gap: 0.75rem;
		padding: clamp(1.25rem, 3vw, 1.75rem);
	}

	/* The code is the thing you read out or paste, so it gets the room and
	   the tracking to be read at a glance. */
	.code {
		display: grid;
		gap: 0.2rem;
		width: 100%;
		padding: 0.85rem;
		border: 1px dashed var(--line-strong);
		border-radius: var(--r-md);
		background: var(--field);
		font-family: var(--sans);
		font-size: 1.35rem;
		letter-spacing: 0.16em;
		color: var(--ink);
		cursor: pointer;
	}

	.code .label {
		color: var(--faint);
		letter-spacing: 0.08em;
	}

	.code:hover {
		border-color: var(--pine);
	}

	hr {
		width: 100%;
		height: 1px;
		margin: 0.5rem 0;
		border: 0;
		background: var(--line);
	}

	.grow {
		width: 100%;
	}

	.danger {
		color: var(--brick);
	}

	.quiet {
		color: var(--muted);
	}

	.error {
		padding: 0.6rem 0.75rem;
		margin-bottom: 1.5rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}
</style>
