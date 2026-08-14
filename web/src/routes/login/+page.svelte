<script>
	import { goto } from '$app/navigation';
	import { ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import Field from '$lib/components/Field.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';

	let email = $state('');
	let password = $state('');
	let busy = $state(false);
	let error = $state(null);

	async function submit(event) {
		event.preventDefault();
		busy = true;
		error = null;
		try {
			await session.signIn({ email, password });
			goto('/bookings');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Something went wrong on our side.';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Sign in | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="auth forest-band">
	<div class="shell split">
		<div class="say">
			<h1 class="display display-l"><TextAnimate text="Sign in" /></h1>
			<div class="pitch-mark fade-up fade-up-1">
				<CentreMark wide />
			</div>
			<p class="lede fade-up fade-up-2">
				An hour held without a name is just an hour. Sign in and the court comes off the board in
				yours.
			</p>
			<p class="demo small fade-up fade-up-3">
				Nothing is wired to a server yet. Try <strong>rajwat@khelarena.np</strong> with
				<strong>kathmandu2026</strong>.
			</p>
		</div>

		<form class="card fade-up fade-up-1" onsubmit={submit} novalidate>
			<Field
				name="email"
				label="Email"
				type="email"
				bind:value={email}
				autocomplete="email"
				placeholder="you@example.com"
				required
			/>
			<Field
				name="password"
				label="Password"
				type="password"
				bind:value={password}
				autocomplete="current-password"
				required
			/>

			{#if error}
				<p class="error" role="alert">{error}</p>
			{/if}

			<button class="btn btn-primary" class:loading={busy} type="submit" disabled={busy}>
				{busy ? 'Checking…' : 'Sign in'}
			</button>

			<p class="alt small">No account yet? <a class="link" href="/register">Create one</a>.</p>
		</form>
	</div>
</section>

<style>
	.split {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 24rem);
		gap: clamp(2rem, 6vw, 5rem);
		align-items: start;
	}

	.say .lede {
		margin-top: 1.1rem;
	}

	.demo {
		margin-top: 1.75rem;
		padding: 0.9rem 1.1rem;
		border-radius: var(--r-sm);
		background: var(--sand);
		color: #7a3b22;
		max-width: 44ch;
	}

	.demo strong {
		color: inherit;
		font-weight: 600;
	}

	form {
		display: grid;
		gap: 1.35rem;
		padding: clamp(1.5rem, 3vw, 2rem);
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
