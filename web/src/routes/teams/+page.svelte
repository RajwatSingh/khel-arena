<script>
	import { goto, invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import Field from '$lib/components/Field.svelte';
	import { reveal } from '$lib/actions/reveal.js';

	let { data } = $props();

	let joinCode = $state('');
	let newTeam = $state({ name: '', tag: '' });
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});

	async function act(fn) {
		busy = true;
		error = null;
		fieldErrors = {};
		try {
			return await fn();
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

	async function join(event) {
		event.preventDefault();
		const team = await act(() => api.joinTeam(joinCode));
		if (team) {
			joinCode = '';
			await invalidateAll();
		}
	}

	async function create(event) {
		event.preventDefault();
		const team = await act(() => api.createTeam({ name: newTeam.name, tag: newTeam.tag }));
		if (team) goto(`/teams/${team.id}`);
	}
</script>

<svelte:head>
	<title>My teams | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">The squad book</p>
		<h1 class="display display-l"><TextAnimate text="Your teams" /></h1>
		<p class="lede fade-up fade-up-1">
			A team is a name, a tag and a roster. The captain runs the squad and holds the invite code;
			everyone else can leave whenever they like.
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="body">
	<div class="shell">
		{#if error}
			<p class="error small" role="alert">{error}</p>
		{/if}

		{#if data.teams.length}
			<ul class="teams">
				{#each data.teams as team, i (team.id)}
					<li use:reveal={{ delay: Math.min(i, 6) * 60 }}>
						<a class="record" href="/teams/{team.id}">
							<span class="tag num">{team.tag}</span>
							<span class="who">
								<strong class="display display-m">{team.name}</strong>
								<span class="small">
									{team.member_count}
									{team.member_count === 1 ? 'player' : 'players'}
								</span>
							</span>
						</a>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="empty prose">
				You aren't on a team yet. Start one, or join with a code somebody sent you.
			</p>
		{/if}

		<div class="forms">
			<form class="card" onsubmit={create}>
				<h2 class="display display-m">Start a team</h2>
				<Field
					name="name"
					label="Team name"
					bind:value={newTeam.name}
					error={fieldErrors.name}
					required
					maxlength="40"
					placeholder="Yeti FC"
				/>
				<Field
					name="tag"
					label="Tag"
					bind:value={newTeam.tag}
					error={fieldErrors.tag}
					hint="Two to five letters or numbers, in capitals."
					required
					maxlength="5"
					placeholder="YETI"
				/>
				<button class="btn btn-primary" class:loading={busy} disabled={busy}>Create</button>
			</form>

			<form class="card" onsubmit={join}>
				<h2 class="display display-m">Join with a code</h2>
				<Field
					name="code"
					label="Invite code"
					bind:value={joinCode}
					error={fieldErrors.code}
					hint="Eight characters. Case and spacing don't matter."
					required
					placeholder="ABCD1234"
				/>
				<button class="btn btn-secondary" class:loading={busy} disabled={busy}>Join</button>
			</form>
		</div>
	</div>
</section>

<style>
	h1 {
		margin-top: 0.6rem;
	}

	.head .lede {
		margin-top: 1rem;
		max-width: 54ch;
	}

	.body {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	.teams {
		display: grid;
		margin-bottom: clamp(2rem, 4vw, 3rem);
	}

	.record {
		display: flex;
		align-items: center;
		gap: 1.25rem;
		padding: clamp(1rem, 2.5vw, 1.4rem) 0;
		border-top: 1px solid var(--line);
		color: inherit;
		text-decoration: none;
		transition: border-color var(--dur-hover) var(--ease);
	}

	.record:hover,
	.record:focus-visible {
		border-top-color: var(--pine);
	}

	/* The tag is how a squad is known on a team sheet, so it reads as a
	   badge rather than as text on the row. */
	.tag {
		display: grid;
		place-items: center;
		flex: none;
		width: 3.25rem;
		height: 3.25rem;
		border-radius: var(--r-md);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.95rem;
		letter-spacing: 0.04em;
	}

	.who {
		display: grid;
		gap: 0.15rem;
	}

	.who .small {
		color: var(--faint);
	}

	.forms {
		display: grid;
		gap: 1.5rem;
	}

	@media (min-width: 48rem) {
		.forms {
			grid-template-columns: 1fr 1fr;
		}
	}

	form {
		display: grid;
		gap: 1rem;
		align-content: start;
		padding: clamp(1.5rem, 3vw, 2rem);
	}

	h2 {
		margin: 0;
	}

	.empty {
		padding: clamp(2rem, 5vw, 3rem);
		margin-bottom: clamp(2rem, 4vw, 3rem);
		border: 1px solid var(--line);
		border-radius: var(--r-lg);
		background: var(--surface);
		text-align: center;
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
