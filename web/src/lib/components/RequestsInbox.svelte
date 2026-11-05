<script>
	import { untrack } from 'svelte';
	import { invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import { formatDateLong, formatTime } from '$lib/time.js';

	// Requests waiting on the signed-in player, grouped by their game. Fetched
	// here rather than in the page's load, which runs signed out: like the game
	// page, this waits until the session is known.
	let games = $state([]);
	let busy = $state(null);
	let error = $state(null);

	const count = $derived(games.reduce((n, g) => n + g.responses.length, 0));

	async function refresh() {
		games = await api.callRequests();
	}

	$effect(() => {
		if (!session.signedIn) {
			games = [];
			return;
		}
		untrack(() => refresh().catch(() => {}));
	});

	// Re-read after accepting rather than patching by hand: the server owns
	// the place count, and a game that just filled drops out of this list.
	async function accept(game, response) {
		busy = `${game.id}:${response.user_id}`;
		error = null;
		try {
			await api.acceptResponder(game.id, response.user_id);
			await Promise.all([refresh(), invalidateAll()]);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'That did not go through.';
		} finally {
			busy = null;
		}
	}

	const nameOf = (u) => u.full_name || u.username;
</script>

{#if games.length}
	<section class="inbox card" aria-labelledby="inbox-title">
		<header>
			<p class="label">New requests</p>
			<h2 id="inbox-title" class="display display-m">
				<span class="num">{count}</span>
				{count === 1 ? 'player wants' : 'players want'} in
			</h2>
		</header>

		{#if error}
			<p class="error small" role="alert">{error}</p>
		{/if}

		<ul class="games">
			{#each games as game (game.id)}
				<li>
					<div class="game">
						<a class="title" href="/games/{game.id}">{game.title}</a>
						<span class="small meta">
							{formatDateLong(game.starts_at)} at {formatTime(game.starts_at)} ·
							<span class="num">{game.spots_remaining}</span>
							{game.spots_remaining === 1 ? 'place' : 'places'} left
						</span>
					</div>

					<ul class="requests">
						{#each game.responses as response (response.user_id)}
							<li>
								<div class="responder">
									<a href="/players/{response.responder.username}">
										<strong>{nameOf(response.responder)}</strong>
									</a>
									<span class="small">@{response.responder.username}</span>
									{#if response.message}<p class="small note">“{response.message}”</p>{/if}
								</div>
								<button
									class="btn btn-secondary"
									class:loading={busy === `${game.id}:${response.user_id}`}
									disabled={busy !== null || game.spots_remaining === 0}
									onclick={() => accept(game, response)}
								>
									Give them a place
								</button>
							</li>
						{/each}
					</ul>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<style>
	.inbox {
		display: grid;
		gap: 1rem;
		margin-bottom: clamp(1.5rem, 3vw, 2.25rem);
		padding: clamp(1.25rem, 3vw, 1.75rem);
		border-left: 4px solid var(--pine-deep);
	}

	h2 {
		margin-top: 0.35rem;
	}

	.games {
		display: grid;
		gap: 1.25rem;
	}

	.game {
		display: grid;
		gap: 0.2rem;
		margin-bottom: 0.6rem;
	}

	.title {
		color: inherit;
		font-weight: 600;
		text-decoration: none;
	}

	.title:hover {
		color: var(--pine-deep);
	}

	.meta {
		color: var(--muted);
	}

	.requests {
		display: grid;
		gap: 0.6rem;
	}

	.requests li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem 1rem;
		padding: 0.9rem 1.1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.responder {
		min-width: 0;
	}

	.responder a {
		color: inherit;
		text-decoration: none;
	}

	.responder a:hover strong {
		color: var(--pine-deep);
	}

	.responder span {
		display: block;
		color: var(--faint);
	}

	.note {
		margin: 0.35rem 0 0;
		color: var(--muted);
		overflow-wrap: anywhere;
	}

	.error {
		padding: 0.6rem 0.75rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}
</style>
