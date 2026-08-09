<script>
	import { goto } from '$app/navigation';
	import { ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import Field from '$lib/components/Field.svelte';

	let form = $state({
		full_name: '',
		username: '',
		email: '',
		password: '',
		skill: 'casual',
		position: ''
	});
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});

	const skills = [
		['casual', 'Casual'],
		['intermediate', 'Intermediate'],
		['competitive', 'Competitive'],
		['semi_pro', 'Semi-pro']
	];

	// Futsal position names, as they are used on the pitch.
	const positions = ['Goleiro', 'Fixo', 'Ala', 'Pivo', 'Universal'];

	async function submit(event) {
		event.preventDefault();
		busy = true;
		error = null;
		fieldErrors = {};
		try {
			await session.signUp({ ...form, position: form.position || null });
			goto('/tonight');
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
</script>

<svelte:head>
	<title>Create an account | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="auth">
	<div class="shell split">
		<div class="say">
			<h1 class="display display-l">Make a player card</h1>
			<p class="lede">
				Name, email, password. The rest is how other players find you when someone is a man short —
				leave it blank and fill it in later.
			</p>
		</div>

		<form class="card" onsubmit={submit} novalidate>
			<Field
				name="full_name"
				label="Full name"
				bind:value={form.full_name}
				error={fieldErrors.full_name}
				autocomplete="name"
			/>
			<Field
				name="username"
				label="Username"
				bind:value={form.username}
				error={fieldErrors.username}
				hint="How you show up on the find-a-player board."
				autocomplete="username"
			/>
			<Field
				name="email"
				label="Email"
				type="email"
				bind:value={form.email}
				error={fieldErrors.email}
				autocomplete="email"
			/>
			<Field
				name="password"
				label="Password"
				type="password"
				bind:value={form.password}
				error={fieldErrors.password}
				hint="At least 10 characters."
				autocomplete="new-password"
			/>

			<div class="field">
				<span class="field-label" id="skill-label">Level</span>
				<div class="chips" role="radiogroup" aria-labelledby="skill-label">
					{#each skills as [value, label] (value)}
						<button
							type="button"
							class="chip"
							class:on={form.skill === value}
							role="radio"
							aria-checked={form.skill === value}
							onclick={() => (form.skill = value)}
						>
							{label}
						</button>
					{/each}
				</div>
			</div>

			<div class="field">
				<label class="field-label" for="position">Position</label>
				<select id="position" bind:value={form.position}>
					<option value="">Not sure yet</option>
					{#each positions as position (position)}
						<option value={position}>{position}</option>
					{/each}
				</select>
			</div>

			{#if error}
				<p class="error" role="alert">{error}</p>
			{/if}

			<button class="btn btn-primary" type="submit" disabled={busy}>
				{busy ? 'Creating…' : 'Create account'}
			</button>

			<p class="alt small">Already have one? <a class="link" href="/login">Sign in</a>.</p>
		</form>
	</div>
</section>

<style>
	.auth {
		padding-block: clamp(3rem, 6vw, 5.5rem);
	}

	.split {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 26rem);
		gap: clamp(2rem, 6vw, 5rem);
		align-items: start;
	}

	.say .lede {
		margin-top: 1.1rem;
	}

	form {
		display: grid;
		gap: 1.35rem;
		padding: clamp(1.5rem, 3vw, 2rem);
		box-shadow: var(--shadow);
	}

	.field {
		display: grid;
		gap: 0.55rem;
	}

	.field-label {
		font-size: 0.9375rem;
		font-weight: 600;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}

	.chip {
		padding: 0.45rem 0.9rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-pill);
		background: var(--surface);
		font-size: 0.9375rem;
		font-weight: 500;
		color: var(--muted);
		cursor: pointer;
		transition:
			border-color 0.18s var(--ease),
			background-color 0.18s var(--ease),
			color 0.18s var(--ease);
	}

	.chip:hover:not(.on) {
		color: var(--ink);
	}

	.chip.on {
		background: var(--pine);
		border-color: var(--pine);
		color: #ffffff;
	}

	select {
		width: 100%;
		padding: 0.75rem 0.95rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-sm);
		background: var(--surface);
		font-size: 1rem;
		cursor: pointer;
	}

	select:focus {
		outline: none;
		border-color: var(--pine);
		box-shadow: 0 0 0 3px var(--pine-wash);
	}

	.error {
		padding: 0.8rem 1rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
		font-size: 0.9375rem;
	}

	.alt {
		color: var(--muted);
	}

	@media (max-width: 820px) {
		.split {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
