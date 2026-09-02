<script>
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api/index.js';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
	import Field from '$lib/components/Field.svelte';
	import { formatNPR } from '$lib/time.js';
	import { reveal } from '$lib/actions/reveal.js';

	let { data } = $props();

	let form = $state({ name: '', area: '', opens_at: '06:00', closes_at: '22:00' });
	let busy = $state(false);
	let error = $state(null);
	let fieldErrors = $state({});

	async function create(event) {
		event.preventDefault();
		busy = true;
		error = null;
		fieldErrors = {};

		try {
			const arena = await api.createArena({ ...form, amenities: [] });
			goto(`/manage/${arena.id}`);
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
	<title>Manage arenas | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="head forest-band">
	<div class="shell">
		<p class="label fade-up">The back office</p>
		<h1 class="display display-l"><TextAnimate text="Your venues" /></h1>
		<p class="lede fade-up fade-up-1">
			Hours, courts and rates are yours to set. Nothing here is marked up — what you charge is what
			a player pays.
		</p>
		<div class="pitch-mark fade-up fade-up-2"><CentreMark wide /></div>
	</div>
</section>

<section class="body">
	<div class="shell">
		{#if error}
			<p class="error small" role="alert">{error}</p>
		{/if}

		{#if data.arenas.length}
			<ul class="arenas">
				{#each data.arenas as arena, i (arena.id)}
					<li use:reveal={{ delay: Math.min(i, 6) * 60 }}>
						<a class="record" href="/manage/{arena.id}">
							<div class="who">
								<h3 class="display display-m">{arena.name}</h3>
								<p class="small">{arena.area}, {arena.city} · {arena.opens_at}–{arena.closes_at}</p>
							</div>
							<div class="right">
								<span class="num courts">{arena.court_count}</span>
								<span class="label">{arena.court_count === 1 ? 'court' : 'courts'}</span>
								{#if arena.from_price_npr}
									<span class="small from num">from {formatNPR(arena.from_price_npr)}</span>
								{/if}
							</div>
						</a>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="empty prose">
				You don't have a venue listed yet. Register one below — you'll need an arena owner account.
			</p>
		{/if}

		<form class="card" onsubmit={create}>
			<h2 class="display display-m">List a venue</h2>
			<Field name="name" label="Name" bind:value={form.name} error={fieldErrors.name} required
				maxlength="80" placeholder="Dhuku Futsal" />
			<Field name="area" label="Area" bind:value={form.area} error={fieldErrors.area} required
				placeholder="Jhamsikhel" />
			<div class="pair">
				<Field name="opens_at" label="Opens" type="time" bind:value={form.opens_at}
					error={fieldErrors.opens_at} required />
				<Field name="closes_at" label="Closes" type="time" bind:value={form.closes_at}
					error={fieldErrors.closes_at} required />
			</div>
			<button class="btn btn-primary" class:loading={busy} disabled={busy}>Register</button>
			<p class="small quiet">
				The web address is made from the name and can't be changed afterwards — it ends up in every
				link anyone shares.
			</p>
		</form>
	</div>
</section>

<style>
	h1 { margin-top: 0.6rem; }
	.head .lede { margin-top: 1rem; max-width: 54ch; }
	.body { padding-block: clamp(2rem, 4vw, 3rem) var(--band); }

	.arenas { display: grid; margin-bottom: clamp(2rem, 4vw, 3rem); }

	.record {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1.5rem;
		padding: clamp(1rem, 2.5vw, 1.4rem) 0;
		border-top: 1px solid var(--line);
		color: inherit;
		text-decoration: none;
		transition: border-color var(--dur-hover) var(--ease);
	}

	.record:hover, .record:focus-visible { border-top-color: var(--pine); }

	h3 { margin: 0; }
	.record:hover h3 { color: var(--pine-deep); }
	.who .small { margin: 0.2rem 0 0; color: var(--faint); }

	.right {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex: none;
	}

	.courts {
		font-size: clamp(1.6rem, 3vw, 2.1rem);
		line-height: 1;
		color: var(--pine-deep);
	}

	.right .label { margin-top: 0.2rem; color: var(--faint); }
	.from { margin-top: 0.35rem; color: var(--muted); }

	form {
		display: grid;
		gap: 1rem;
		max-width: 34rem;
		padding: clamp(1.5rem, 3vw, 2rem);
	}

	h2 { margin: 0; }

	.pair { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }

	.empty {
		padding: clamp(2rem, 5vw, 3rem);
		margin-bottom: clamp(2rem, 4vw, 3rem);
		border: 1px solid var(--line);
		border-radius: var(--r-lg);
		background: var(--surface);
		text-align: center;
		color: var(--muted);
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
