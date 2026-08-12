<script>
	/**
	 * One labelled input, with the field error the API returns for it.
	 *
	 * `Registration.Validate()` reports every problem at once so a signup form
	 * can render them all (apiPlan.md §4). Errors here are keyed by the same
	 * snake_case field names the wire uses.
	 */
	let {
		name,
		label,
		type = 'text',
		value = $bindable(''),
		hint = '',
		error = '',
		...rest
	} = $props();

	const id = $derived(`f-${name}`);
</script>

<div class="field">
	<label for={id}>{label}</label>
	<input
		{id}
		{name}
		{type}
		bind:value
		aria-invalid={error ? 'true' : undefined}
		aria-describedby={error ? `${id}-err` : hint ? `${id}-hint` : undefined}
		{...rest}
	/>
	{#if error}
		<p class="err" id="{id}-err">{error}</p>
	{:else if hint}
		<p class="hint" id="{id}-hint">{hint}</p>
	{/if}
</div>

<style>
	.field {
		display: grid;
		gap: 0.4rem;
	}

	label {
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--ink);
	}

	input {
		width: 100%;
		padding: 0.75rem 0.95rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-sm);
		background: var(--surface);
		font-size: 1rem;
		transition:
			border-color var(--dur-hover) var(--ease),
			box-shadow var(--dur-hover) var(--ease);
	}

	input::placeholder {
		color: var(--faint);
	}

	input:focus {
		outline: none;
		border-color: var(--pine);
		box-shadow: 0 0 0 3px var(--pine-wash);
	}

	input[aria-invalid='true'] {
		border-color: var(--brick);
	}

	input[aria-invalid='true']:focus {
		box-shadow: 0 0 0 3px var(--brick-wash);
	}

	.hint,
	.err {
		font-size: 0.875rem;
		line-height: 1.5;
	}

	.hint {
		color: var(--faint);
	}

	.err {
		color: var(--brick);
	}
</style>
