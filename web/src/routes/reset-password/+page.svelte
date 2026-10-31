<script>
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import Field from '$lib/components/Field.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';

	// Read once: the token is removed from the address bar below.
	const token = page.url.searchParams.get('token') ?? '';

	let password = $state('');
	let confirm = $state('');
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});
	let done = $state(false);
	let linkDead = $state(false);

	onMount(() => {
		// The token is a credential for the account. Taking it out of the URL
		// keeps it out of browser history and anything the address is shared to.
		if (token) replaceState('/reset-password', {});
	});

	async function submit(event) {
		event.preventDefault();
		error = null;
		linkDead = false;
		fieldErrors = {};

		if (password !== confirm) {
			fieldErrors = { confirm: "Those passwords don't match." };
			return;
		}

		busy = true;
		try {
			await api.resetPassword({ token, new_password: password });
			done = true;
		} catch (err) {
			if (err instanceof ApiError) {
				const byField = Object.fromEntries(err.fields.map((f) => [f.field, f.message]));
				// The service reports password rules against "password".
				const pw = byField.password ?? byField.new_password;
				if (pw) fieldErrors = { password: pw };
				// A bad or used token is not something the form can fix.
				linkDead = Boolean(byField.token);
				error = byField.token ?? (pw ? null : err.message);
			} else {
				error = 'Something went wrong on our side.';
			}
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Reset password | Khel Arena</title>
	<meta name="robots" content="noindex" />
	<!-- Keep the token out of the Referer header on any outbound link. -->
	<meta name="referrer" content="no-referrer" />
</svelte:head>

<section class="auth forest-band">
	<div class="shell split">
		<div class="say">
			<h1 class="display display-l"><TextAnimate text="New password" /></h1>
			<div class="pitch-mark fade-up fade-up-1">
				<CentreMark wide />
			</div>
			<p class="lede fade-up fade-up-2">
				Choose a new password. Every device signed in to your account will be signed out.
			</p>
		</div>

		{#if done}
			<div class="card panel fade-up fade-up-1" role="status">
				<h2 class="title">Password changed</h2>
				<p>You can sign in with your new password now.</p>
				<a class="btn btn-primary" href="/login">Sign in</a>
			</div>
		{:else if !token}
			<div class="card panel fade-up fade-up-1">
				<h2 class="title">This link is incomplete</h2>
				<p>Open the link from your email again, or ask for a new one.</p>
				<a class="btn btn-primary" href="/forgot-password">Get a new link</a>
			</div>
		{:else}
			<form class="card panel fade-up fade-up-1" onsubmit={submit} novalidate>
				<Field
					name="password"
					label="New password"
					type="password"
					bind:value={password}
					autocomplete="new-password"
					hint="At least 10 characters."
					error={fieldErrors.password}
					required
				/>
				<Field
					name="confirm"
					label="Confirm new password"
					type="password"
					bind:value={confirm}
					autocomplete="new-password"
					error={fieldErrors.confirm}
					required
				/>

				{#if error}
					<p class="error" role="alert">
						{error}
						{#if linkDead}<a class="link" href="/forgot-password">Get a new link</a>.{/if}
					</p>
				{/if}

				<button class="btn btn-primary" class:loading={busy} type="submit" disabled={busy}>
					{busy ? 'Saving…' : 'Set new password'}
				</button>
			</form>
		{/if}
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

	.panel {
		display: grid;
		gap: 1.35rem;
		padding: clamp(1.5rem, 3vw, 2rem);
	}

	.title {
		font-size: 1.25rem;
		font-weight: 600;
	}

	.error {
		padding: 0.8rem 1rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
		font-size: 0.9375rem;
	}

	.error .link {
		color: inherit;
	}

	@media (max-width: 820px) {
		.split {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
