<script>
	/**
	 * Enter or replace one venue's eSewa or Khalti key.
	 *
	 * Used twice: on the venue's page, to connect a provider for the first
	 * time, and on the owner's Payment keys page, behind "Change key". The key
	 * is write-only on the server, so the field always starts empty and is
	 * cleared again after a save.
	 */
	import { untrack } from 'svelte';
	import { invalidateAll } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import Field from './Field.svelte';

	let {
		arenaId,
		provider,
		label,
		account = null,
		submitLabel = 'Activate',
		onsaved = () => {},
		oncancel = null,
		onremoved = null
	} = $props();

	// Seeded once from what is saved now; a reload mid-edit must not wipe it.
	let form = $state(
		untrack(() => ({
			secret_key: '',
			merchant_code: account?.merchant_code ?? '',
			live: account?.live ?? false,
			enabled: account?.enabled ?? true
		}))
	);
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});

	async function run(fn) {
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

	async function save(event) {
		event.preventDefault();
		const ok = await run(() =>
			api.setArenaPaymentAccount(arenaId, provider, {
				secret_key: form.secret_key,
				merchant_code: form.merchant_code,
				live: form.live,
				enabled: form.enabled
			})
		);
		form.secret_key = '';
		if (ok) onsaved(provider);
	}

	async function remove() {
		const ok = await run(() => api.removeArenaPaymentAccount(arenaId, provider));
		if (ok && onremoved) onremoved(provider);
	}
</script>

<form class="key-form" onsubmit={save}>
	{#if error}
		<p class="error small" role="alert">{error}</p>
	{/if}

	<Field
		name="{arenaId}-{provider}-secret"
		label={account ? `New ${label} secret key` : `${label} secret key`}
		type="password"
		bind:value={form.secret_key}
		error={fieldErrors.secret_key}
		hint={`From your ${label} merchant dashboard.`}
		autocomplete="off"
		required
	/>
	{#if provider === 'esewa'}
		<Field
			name="{arenaId}-esewa-merchant"
			label="Merchant (product) code"
			bind:value={form.merchant_code}
			error={fieldErrors.merchant_code}
			hint="“EPAYTEST” on the sandbox; your real code in production."
		/>
	{/if}

	<div class="toggles">
		<label class="toggle">
			<input type="checkbox" bind:checked={form.live} />
			Live account (unchecked talks to the {label} sandbox)
		</label>
		<label class="toggle">
			<input type="checkbox" bind:checked={form.enabled} />
			Offer this to players
		</label>
	</div>

	<div class="acts">
		<button class="btn btn-primary" class:loading={busy} disabled={busy}>{submitLabel}</button>
		{#if oncancel}
			<button type="button" class="btn btn-secondary" disabled={busy} onclick={oncancel}>
				Cancel
			</button>
		{/if}
		{#if account && onremoved}
			<button type="button" class="btn btn-quiet remove" disabled={busy} onclick={remove}>
				Remove key
			</button>
		{/if}
	</div>
</form>

<style>
	.key-form {
		display: grid;
		gap: 1rem;
	}

	.error {
		margin: 0;
		padding: 0.6rem 0.75rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}

	.toggles {
		display: grid;
		gap: 0.5rem;
	}

	.toggle {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		font-size: 0.9rem;
		color: var(--muted);
		cursor: pointer;
	}

	.toggle input {
		margin-top: 0.15rem;
		flex: none;
	}

	.acts {
		display: flex;
		flex-wrap: wrap;
		gap: 0.6rem;
	}

	.remove {
		margin-left: auto;
		color: var(--brick);
	}
</style>
