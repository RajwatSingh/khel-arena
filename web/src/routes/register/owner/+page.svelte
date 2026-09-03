<script>
	import { goto } from '$app/navigation';
	import { ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import Field from '$lib/components/Field.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';

	let form = $state({
		full_name: '',
		username: '',
		email: '',
		password: '',
		account_type: 'arena_owner'
	});
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});

	async function submit(event) {
		event.preventDefault();
		busy = true;
		error = null;
		fieldErrors = {};
		try {
			await session.signUp({ ...form });
			goto('/dashboard');
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
	<title>Open a venue account | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="auth forest-band">
	<div class="shell split">
		<div class="say">
			<h1 class="display display-l"><TextAnimate text="Open a venue account" /></h1>
			<div class="pitch-mark fade-up fade-up-1">
				<CentreMark wide />
			</div>
			<p class="lede fade-up fade-up-2">
				This is the operator's login. Once you're in, list your futsal, add its courts and rates,
				and watch the bookings, earnings and unpaid cash on your dashboard.
			</p>
		</div>

		<form class="card fade-up fade-up-1" onsubmit={submit} novalidate>
			<Field
				name="full_name"
				label="Your name"
				bind:value={form.full_name}
				error={fieldErrors.full_name}
				autocomplete="name"
			/>
			<Field
				name="username"
				label="Username"
				bind:value={form.username}
				error={fieldErrors.username}
				hint="Used on your public venue pages."
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

			{#if error}
				<p class="error" role="alert">{error}</p>
			{/if}

			<button class="btn btn-primary" class:loading={busy} type="submit" disabled={busy}>
				{busy ? 'Creating…' : 'Create venue account'}
			</button>

			<p class="alt small">Already have one? <a class="link" href="/login">Sign in</a>.</p>
			<p class="alt small">Just here to play? <a class="link" href="/register/player">Make a player card</a>.</p>
		</form>
	</div>
</section>

<style>
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
