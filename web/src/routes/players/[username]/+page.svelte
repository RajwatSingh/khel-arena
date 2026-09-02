<script>
	import { invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import Field from '$lib/components/Field.svelte';
	import { formatDate } from '$lib/time.js';
	import { SKILL_LABELS } from '$lib/skills.js';

	let { data } = $props();

	const player = $derived(data.player);
	const isMe = $derived(session.user?.id === player.id);
	const winRate = $derived(
		player.matches_played ? Math.round((player.matches_won / player.matches_played) * 100) : null
	);

	let clip = $state({ title: '', url: '' });
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});

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

	async function addClip(event) {
		event.preventDefault();
		const ok = await act(() => api.addHighlight({ title: clip.title, url: clip.url }));
		if (ok) clip = { title: '', url: '' };
	}

	const dropClip = (id) => act(() => api.deleteHighlight(id));
</script>

<svelte:head>
	<title>{player.full_name || player.username} | Khel Arena</title>
	<meta
		name="description"
		content="{player.full_name || player.username} — {SKILL_LABELS[player.skill] ?? player.skill} futsal in {player.city || 'Kathmandu'}."
	/>
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">Player card</p>
		<h1 class="display display-l">
			<TextAnimate text={player.full_name || player.username} />
		</h1>
		<p class="lede fade-up fade-up-1">
			@{player.username}
			{#if player.position}· {player.position}{/if}
			{#if player.jersey_number}· #{player.jersey_number}{/if}
			· {SKILL_LABELS[player.skill] ?? player.skill}
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

			{#if player.bio}
				<p class="prose bio">{player.bio}</p>
			{/if}

			<h2 class="display display-m">Highlights</h2>
			{#if player.highlights.length}
				<ul class="clips">
					{#each player.highlights as highlight (highlight.id)}
						<li>
							<a class="clip" href={highlight.url} target="_blank" rel="noopener noreferrer">
								{highlight.title}
							</a>
							{#if isMe}
								<button class="btn btn-quiet" disabled={busy} onclick={() => dropClip(highlight.id)}>
									Remove
								</button>
							{/if}
						</li>
					{/each}
				</ul>
			{:else}
				<p class="small quiet">
					{isMe ? "You haven't added any clips yet." : 'No clips yet.'}
				</p>
			{/if}

			{#if isMe}
				<form class="card" onsubmit={addClip}>
					<h3 class="label">Add a clip</h3>
					<Field
						name="clip-title"
						label="What is it?"
						bind:value={clip.title}
						error={fieldErrors.title}
						required
						maxlength="80"
						placeholder="Hat-trick v Dhuku"
					/>
					<Field
						name="clip-url"
						label="Link"
						type="url"
						bind:value={clip.url}
						error={fieldErrors.url}
						required
						placeholder="https://youtube.com/watch?v=…"
					/>
					<button class="btn btn-secondary" class:loading={busy} disabled={busy}>Add</button>
				</form>
			{/if}
		</div>

		<aside class="panel card">
			<dl class="stats">
				<div>
					<dt class="label">Played</dt>
					<dd class="num big">{player.matches_played}</dd>
				</div>
				<div>
					<dt class="label">Won</dt>
					<dd class="num big">{player.matches_won}</dd>
				</div>
				{#if winRate !== null}
					<div>
						<dt class="label">Win rate</dt>
						<dd class="num">{winRate}%</dd>
					</div>
				{/if}
				<div>
					<dt class="label">Community</dt>
					<dd class="num">{player.community_score}</dd>
				</div>
			</dl>

			<hr />

			<dl class="facts">
				{#if player.city}
					<div><dt class="label">City</dt><dd>{player.city}</dd></div>
				{/if}
				{#if player.preferred_foot}
					<div><dt class="label">Foot</dt><dd>{player.preferred_foot}</dd></div>
				{/if}
				<div><dt class="label">Joined</dt><dd>{formatDate(player.joined_at)}</dd></div>
			</dl>
		</aside>
	</div>
</section>

<style>
	h1 { margin-top: 0.6rem; }
	.head .lede { margin-top: 1rem; }
	.body { padding-block: clamp(2rem, 4vw, 3rem) var(--band); }

	.split {
		display: grid;
		gap: clamp(1.5rem, 4vw, 3rem);
		align-items: start;
	}

	@media (min-width: 60rem) {
		.split { grid-template-columns: minmax(0, 1fr) 18rem; }
	}

	.bio { margin-bottom: 2rem; }
	h2 { margin: 0 0 1rem; }
	h3 { margin: 0; }

	.clips { display: grid; gap: 0.5rem; margin-bottom: 1.5rem; }

	.clips li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.8rem 1.1rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.clip { color: inherit; }
	.clip:hover { color: var(--pine-deep); }

	form {
		display: grid;
		gap: 0.85rem;
		max-width: 26rem;
		padding: clamp(1.25rem, 3vw, 1.75rem);
	}

	.panel { display: grid; gap: 1rem; padding: clamp(1.25rem, 3vw, 1.75rem); }

	.stats {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
	}

	.facts { display: grid; gap: 0.75rem; }

	dd { margin: 0.15rem 0 0; }
	.big { font-size: 1.8rem; line-height: 1; color: var(--pine-deep); }

	hr {
		height: 1px;
		border: 0;
		background: var(--line);
	}

	.quiet { color: var(--muted); }

	.error {
		padding: 0.6rem 0.75rem;
		margin-bottom: 1.5rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}
</style>
