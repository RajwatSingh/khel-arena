<script>
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import PaymentKeyForm from '$lib/components/PaymentKeyForm.svelte';
	import PaymentKeyStatus from '$lib/components/PaymentKeyStatus.svelte';

	let { data } = $props();

	const PROVIDERS = [
		['esewa', 'eSewa'],
		['khalti', 'Khalti']
	];

	// Which card has its form open, as "arenaId:provider". One at a time, so
	// the page never shows two half-typed keys.
	let editing = $state(null);
	// The card whose key was just saved, so its status can say so.
	let justSaved = $state(null);

	const keyOf = (arenaId, provider) => `${arenaId}:${provider}`;
	const byProvider = (accounts) => Object.fromEntries((accounts ?? []).map((a) => [a.provider, a]));

	function open(arenaId, provider) {
		editing = keyOf(arenaId, provider);
		justSaved = null;
	}

	function saved(arenaId, provider) {
		editing = null;
		justSaved = keyOf(arenaId, provider);
	}
</script>

<svelte:head>
	<title>Payment keys | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">Your account</p>
		<h1 class="display display-l"><TextAnimate text="Payment keys" /></h1>
		<p class="lede fade-up fade-up-1">
			The eSewa and Khalti merchant keys behind each of your venues. Players' payments land in the
			account a key belongs to. Keys are stored encrypted and never shown again; you see only the
			last four characters.
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="body">
	<div class="shell">
		{#if data.venues.length}
			{#each data.venues as { arena, accounts } (arena.id)}
				{@const savedKeys = byProvider(accounts)}
				<section class="venue" id="venue-{arena.id}">
					<h2 class="display display-m">
						<a class="venue-link" href="/manage/{arena.id}">{arena.name}</a>
					</h2>
					<ul class="keys">
						{#each PROVIDERS as [provider, label] (provider)}
							{@const account = savedKeys[provider]}
							{@const key = keyOf(arena.id, provider)}
							<li class="key card">
								<div class="key-head">
									<h3 class="display display-m">{label}</h3>
									{#if editing !== key}
										<button class="btn btn-secondary" onclick={() => open(arena.id, provider)}>
											{account ? 'Change key' : 'Add key'}
										</button>
									{/if}
								</div>

								{#if account}
									<PaymentKeyStatus {label} {account} fresh={justSaved === key} />
								{:else}
									<p class="small quiet none">Not set up. Players pay this venue in cash.</p>
								{/if}

								{#if editing === key}
									<PaymentKeyForm
										arenaId={arena.id}
										{provider}
										{label}
										{account}
										submitLabel={account ? 'Save new key' : `Activate ${label}`}
										onsaved={() => saved(arena.id, provider)}
										oncancel={() => (editing = null)}
										onremoved={() => (editing = null)}
									/>
								{/if}
							</li>
						{/each}
					</ul>
				</section>
			{/each}
		{:else}
			<div class="empty">
				<p class="prose">No venue listed yet. Once you register one, its payment keys show here.</p>
				<a class="btn btn-primary" href="/manage">List a venue</a>
			</div>
		{/if}
	</div>
</section>

<style>
	h1 {
		margin-top: 0.6rem;
	}
	.head .lede {
		margin-top: 1rem;
		max-width: 58ch;
	}
	.body {
		padding-block: clamp(2rem, 4vw, 3rem) var(--band);
	}

	.venue {
		margin-bottom: clamp(2rem, 4vw, 3rem);
		scroll-margin-top: 6rem;
	}

	.venue h2 {
		margin: 0 0 1rem;
	}

	.venue-link {
		color: inherit;
		text-decoration: none;
	}
	.venue-link:hover,
	.venue-link:focus-visible {
		color: var(--pine-deep);
	}

	.keys {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(100%, 22rem), 1fr));
		gap: 1rem;
	}

	.key {
		display: grid;
		align-content: start;
		gap: 1rem;
		padding: clamp(1.25rem, 3vw, 1.6rem);
	}

	.key-head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem 0.75rem;
	}

	.key-head h3 {
		margin: 0;
	}

	.none {
		margin: 0;
	}
	.quiet {
		color: var(--muted);
	}

	.empty {
		display: grid;
		justify-items: start;
		gap: 1rem;
	}
</style>
