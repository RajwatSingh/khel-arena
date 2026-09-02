<script>
	import { goto, invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import { formatDateLong, formatTime } from '$lib/time.js';
	import { SKILL_LABELS } from '$lib/skills.js';

	let { data } = $props();

	const call = $derived(data.call);
	const mine = $derived(session.signedIn && session.user?.id === call.author_id);
	const full = $derived(call.spots_remaining === 0);
	const over = $derived(new Date(call.starts_at).getTime() <= Date.now());
	const closed = $derived(call.status !== 'open' || full || over);

	let message = $state('');
	let busy = $state(false);
	let error = $state(null);

	// Every action re-reads the page rather than patching state by hand: the
	// server owns the spot count and the status, and guessing at them here is
	// how a screen ends up disagreeing with the database.
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

	const join = () => act(async () => {
		await api.respondToCall(call.id, message);
		message = '';
	});

	const withdraw = () => act(() => api.withdrawFromCall(call.id));
	const accept = (userId) => act(() => api.acceptResponder(call.id, userId));
	const cancel = () => act(() => api.cancelCall(call.id));

	async function remove() {
		await act(() => api.deleteCall(call.id));
		if (!error) goto('/games');
	}
</script>

<svelte:head>
	<title>{call.title} | Khel Arena</title>
	<meta name="description" content="{call.spots_remaining} places going, {formatDateLong(call.starts_at)}." />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">
			<a class="back" href="/games">The call sheet</a>
		</p>
		<h1 class="display display-l"><TextAnimate text={call.title} /></h1>
		<p class="lede fade-up fade-up-1">
			{formatDateLong(call.starts_at)} at {formatTime(call.starts_at)}
			{#if call.arena_name}· {call.arena_name}, {call.arena_area}{/if}
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="body">
	<div class="shell split">
		<div class="detail">
			{#if call.description}
				<p class="prose">{call.description}</p>
			{/if}

			<dl class="facts">
				<div><dt class="label">Standard</dt><dd>{SKILL_LABELS[call.skill] ?? call.skill}</dd></div>
				<div>
					<dt class="label">Posted by</dt>
					<dd>
						<a href="/players/{call.author.username}">
							{call.author.full_name || call.author.username}
						</a>
					</dd>
				</div>
				<div>
					<dt class="label">Court</dt>
					<dd>{call.booking_id ? 'Booked and paid for' : 'Not booked yet'}</dd>
				</div>
				<div>
					<dt class="label">Filled</dt>
					<dd class="num">{call.filled_players} of {call.needed_players}</dd>
				</div>
			</dl>

			{#if mine && call.responses?.length}
				<h2 class="display display-m">Who has asked</h2>
				<ul class="responses">
					{#each call.responses as response (response.user_id)}
						<li>
							<div class="responder">
								<a href="/players/{response.responder.username}">
									<strong>{response.responder.full_name || response.responder.username}</strong>
								</a>
								<span class="small">@{response.responder.username}</span>
								{#if response.message}<p class="small note">“{response.message}”</p>{/if}
							</div>
							{#if response.accepted}
								<span class="chip">In</span>
							{:else}
								<button
									class="btn btn-secondary"
									disabled={busy || full}
									onclick={() => accept(response.user_id)}
								>
									Give them a place
								</button>
							{/if}
						</li>
					{/each}
				</ul>
			{:else if mine}
				<p class="small quiet">Nobody has asked to join yet.</p>
			{/if}
		</div>

		<aside class="panel card">
			<p class="spots num" class:last={call.spots_remaining === 1}>
				{call.spots_remaining}
			</p>
			<p class="label">{call.spots_remaining === 1 ? 'place left' : 'places left'}</p>

			{#if error}
				<p class="error small" role="alert">{error}</p>
			{/if}

			{#if !session.signedIn}
				<a class="btn btn-primary grow" href="/login">Sign in to join</a>
			{:else if mine}
				<p class="small quiet">This is your game.</p>
				{#if call.status === 'open'}
					<button class="btn btn-secondary grow" disabled={busy} onclick={cancel}>
						Call it off
					</button>
				{/if}
				<button class="btn btn-quiet grow" disabled={busy} onclick={remove}>Delete</button>
			{:else if call.you_responded}
				<p class="small quiet">You've asked to join. The organiser decides who plays.</p>
				<button class="btn btn-secondary grow" disabled={busy} onclick={withdraw}>
					Withdraw
				</button>
			{:else if closed}
				<p class="small quiet">
					{#if over}This game has already kicked off.
					{:else if full}This game is full.
					{:else}This game is no longer taking players.{/if}
				</p>
			{:else}
				<label class="ask" for="ask">
					<span class="label">Say something (optional)</span>
					<textarea
						id="ask"
						bind:value={message}
						maxlength="200"
						rows="3"
						placeholder="I play left back, can be there by 7."
					></textarea>
				</label>
				<button class="btn btn-primary grow" class:loading={busy} disabled={busy} onclick={join}>
					Ask to join
				</button>
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

	.facts {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
		gap: 1rem 1.5rem;
		margin: clamp(1.5rem, 3vw, 2rem) 0;
		padding-top: 1.25rem;
		border-top: 1px solid var(--line);
	}

	dd {
		margin: 0.2rem 0 0;
	}

	h2 {
		margin: 2rem 0 1rem;
	}

	.responses {
		display: grid;
		gap: 0.75rem;
	}

	.responses li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.9rem 1.1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.responder a { color: inherit; text-decoration: none; }
	.responder a:hover strong { color: var(--pine-deep); }

	.responder span {
		display: block;
		color: var(--faint);
	}

	.note {
		margin: 0.35rem 0 0;
		color: var(--muted);
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
		gap: 0.6rem;
		padding: clamp(1.25rem, 3vw, 1.75rem);
	}

	.spots {
		margin: 0;
		font-size: clamp(2.5rem, 6vw, 3.5rem);
		line-height: 1;
		color: var(--pine-deep);
	}

	.spots.last {
		color: var(--brick);
	}

	.ask {
		display: grid;
		gap: 0.35rem;
		margin-top: 0.5rem;
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

	.grow {
		width: 100%;
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
