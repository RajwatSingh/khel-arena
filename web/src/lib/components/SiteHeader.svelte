<script>
	import { page } from '$app/state';
	import { session, clock } from '$lib/session.svelte.js';
	import CentreMark from './CentreMark.svelte';

	const ticking = clock();

	const links = [
		{ href: '/tonight', label: 'Find a court' },
		{ href: '/arenas', label: 'Arenas' },
		{ href: '/bookings', label: 'My bookings' }
	];

	// Kathmandu runs at UTC+05:45 all year.
	const kathmandu = $derived(
		new Date(ticking.now + (5 * 60 + 45) * 60_000).toISOString().slice(11, 16)
	);

	function current(href) {
		return page.url.pathname === href || page.url.pathname.startsWith(href + '/');
	}
</script>

<header>
	<div class="shell bar">
		<a class="mark" href="/">
			<CentreMark width={26} label="Khel Arena" />
			<span>Khel Arena</span>
		</a>

		<nav aria-label="Primary">
			{#each links as link (link.href)}
				<a href={link.href} class="tab" aria-current={current(link.href) ? 'page' : undefined}>
					{link.label}
				</a>
			{/each}
		</nav>

		<div class="right">
			<span class="clock num">Kathmandu {kathmandu}</span>
			{#if session.signedIn}
				<button class="btn btn-quiet" onclick={() => session.signOut()}>Sign out</button>
				<a class="btn btn-primary" href="/bookings">{session.user.username}</a>
			{:else}
				<a class="tab" href="/login">Sign in</a>
				<a class="btn btn-primary" href="/tonight">Book a court</a>
			{/if}
		</div>
	</div>
</header>

<style>
	header {
		position: sticky;
		top: 0;
		z-index: 40;
		background: color-mix(in srgb, var(--field) 88%, transparent);
		backdrop-filter: blur(12px);
		border-bottom: 1px solid var(--line);
	}

	.bar {
		display: flex;
		align-items: center;
		gap: 1.5rem;
		min-height: 76px;
	}

	.mark {
		display: inline-flex;
		align-items: center;
		gap: 0.65rem;
		font-family: var(--display);
		font-size: 1.375rem;
		font-weight: 500;
		letter-spacing: -0.02em;
		white-space: nowrap;
		color: var(--on-field);
	}

	.mark :global(svg) {
		color: var(--accent-on-field);
	}

	nav {
		display: flex;
		gap: 0.35rem;
		margin-right: auto;
	}

	.tab {
		padding: 0.5rem 0.95rem;
		border-radius: var(--r-pill);
		font-size: 0.9375rem;
		font-weight: 500;
		color: var(--on-field-muted);
		transition:
			background-color 0.18s var(--ease),
			color 0.18s var(--ease);
	}

	.tab:hover {
		background: var(--surface);
		color: var(--ink);
	}

	.tab[aria-current='page'] {
		background: var(--surface);
		color: var(--ink);
		box-shadow: var(--shadow-sm);
	}

	.right {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.clock {
		font-size: 0.875rem;
		color: var(--on-field-faint);
		white-space: nowrap;
	}

	@media (max-width: 1040px) {
		.clock {
			display: none;
		}
	}

	@media (max-width: 880px) {
		.bar {
			flex-wrap: wrap;
			gap: 0.6rem 1rem;
			padding-block: 0.9rem;
			min-height: 0;
		}

		nav {
			order: 3;
			width: 100%;
			margin-right: 0;
			overflow-x: auto;
			padding-bottom: 0.15rem;
		}

		.right {
			margin-left: auto;
		}
	}
</style>
