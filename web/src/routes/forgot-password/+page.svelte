<script>
	import { dev } from '$app/environment';
	import { api, ApiError } from '$lib/api/index.js';
	import Field from '$lib/components/Field.svelte';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';

	let email = $state('');
	let busy = $state(false);
	let error = $state(null);
	let sent = $state(false);

	async function submit(event) {
		event.preventDefault();
		error = null;
		if (!email.trim()) {
			error = 'Enter the email you signed up with.';
			return;
		}
		busy = true;
		try {
			await api.forgotPassword(email.trim());
			// The API answers the same whether or not the address has an
			// account, so this page says the same thing either way too.
			sent = true;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Something went wrong on our side.';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Forgot password | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="auth forest-band">
	<div class="shell split">
		<div class="say">
			<h1 class="display display-l"><TextAnimate text="Forgot password" /></h1>
			<div class="pitch-mark fade-up fade-up-1">
				<CentreMark wide />
			</div>
			<p class="lede fade-up fade-up-2">
				Tell us the email on your account and we'll send a link to choose a new password.
			</p>
			{#if dev}
				<p class="demo small fade-up fade-up-3">
					Local dev: no email is sent. The reset link is printed in the API server's log.
				</p>
			{/if}
		</div>

		{#if sent}
			<div class="card done fade-up fade-up-1" role="status">
				<h2 class="title">Check your inbox</h2>
				<p>
					If an account exists for <strong>{email.trim()}</strong>, a reset link is on its way. It
					works once and expires in an hour.
				</p>
				<p class="small muted">
					Nothing arrived? Check spam, or
					<button class="link-btn" type="button" onclick={() => (sent = false)}>try again</button>.
				</p>
				<a class="btn btn-primary" href="/login">Back to sign in</a>
			</div>
		{:else}
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

				{#if error}
					<p class="error" role="alert">{error}</p>
				{/if}

				<button class="btn btn-primary" class:loading={busy} type="submit" disabled={busy}>
					{busy ? 'Sending…' : 'Send reset link'}
				</button>

				<p class="alt small">Remembered it? <a class="link" href="/login">Sign in</a>.</p>
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

	.demo {
		margin-top: 1.75rem;
		padding: 0.9rem 1.1rem;
		border-radius: var(--r-sm);
		background: var(--sand);
		color: #7a3b22;
		max-width: 44ch;
	}

	form,
	.done {
		display: grid;
		gap: 1.35rem;
		padding: clamp(1.5rem, 3vw, 2rem);
	}

	.title {
		font-size: 1.25rem;
		font-weight: 600;
	}

	.done strong {
		font-weight: 600;
		overflow-wrap: anywhere;
	}

	.muted,
	.alt {
		color: var(--muted);
	}

	.link-btn {
		padding: 0;
		border: 0;
		background: none;
		color: inherit;
		font: inherit;
		text-decoration: underline;
		cursor: pointer;
	}

	.error {
		padding: 0.8rem 1rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
		font-size: 0.9375rem;
	}

	@media (max-width: 820px) {
		.split {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
