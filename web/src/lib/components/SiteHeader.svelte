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
				<a class="btn btn-primary username" href="/bookings">{session.user.username}</a>
			{:else}
				<a class="btn btn-quiet" href="/login">Sign in</a>
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
		overflow: hidden;
		/* The one dark band moves to the top of every page: same forest
		   ground as .forest-band, just compressed into a bar instead of a
		   hero. The two floodlights live on ::before/::after below so each
		   can breathe on its own clock, the way two pitch lights never
		   dim and brighten in step. */
		background: var(--forest);
		backdrop-filter: blur(12px);
		border-bottom: 1px solid color-mix(in srgb, var(--on-field) 14%, transparent);
	}

	header::before,
	header::after {
		content: '';
		position: absolute;
		top: 0;
		bottom: 0;
		z-index: 0;
		width: 58%;
		pointer-events: none;
		animation: floodlight-breathe var(--amb-a) ease-in-out infinite;
	}

	header::before {
		left: 0;
		background: radial-gradient(65% 260% at 0% 0%, color-mix(in srgb, var(--accent-on-field) 38%, transparent), transparent 75%);
	}

	header::after {
		right: 0;
		background: radial-gradient(60% 260% at 100% 100%, color-mix(in srgb, var(--pine) 85%, transparent), transparent 70%);
		animation-duration: var(--amb-b);
		animation-delay: calc(var(--amb-a) * -0.5);
	}

	@keyframes floodlight-breathe {
		0%,
		100% {
			opacity: 0.7;
		}
		50% {
			opacity: 1;
		}
	}

	.bar {
		position: relative;
		z-index: 1;
		display: flex;
		align-items: center;
		gap: clamp(0.75rem, 2.5vw, 1.5rem);
		min-height: 76px;
		padding-block: 0.75rem;
	}

	.mark {
		display: inline-flex;
		align-items: center;
		gap: 0.65rem;
		font-family: var(--display);
		font-size: 1.3125rem;
		font-weight: 800;
		letter-spacing: -0.02em;
		white-space: nowrap;
		color: var(--on-field);
		flex-shrink: 0;
	}

	.mark :global(svg) {
		color: var(--accent-on-field);
		transition: transform 0.5s var(--ease);
	}

	.mark:hover :global(svg) {
		transform: rotate(180deg);
	}

	nav {
		display: flex;
		gap: 0.2rem;
		margin-right: auto;
		min-width: 0;
	}

	.tab {
		padding: 0.5rem 0.85rem;
		border-radius: var(--r-pill);
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--on-field-muted);
		white-space: nowrap;
		flex-shrink: 0;
		transition:
			background-color var(--dur-hover) var(--ease),
			color var(--dur-hover) var(--ease);
	}

	.tab:hover {
		background: color-mix(in srgb, var(--on-field) 10%, transparent);
		color: var(--on-field);
	}

	.tab[aria-current='page'] {
		background: color-mix(in srgb, var(--on-field) 15%, transparent);
		color: var(--on-field);
		box-shadow: none;
	}

	.right {
		display: flex;
		align-items: center;
		gap: clamp(0.5rem, 1.5vw, 0.75rem);
		flex-shrink: 0;
	}

	.right :global(.btn) {
		flex-shrink: 0;
		white-space: nowrap;
	}

	/* .btn-quiet / .btn-primary are tuned for the light --field ground
	   everywhere else on the site; here, on --forest, they need the
	   inverse treatment — a plain on-field link and a bright contrast
	   pill, the same high-contrast pairing the reference bar uses, just
	   in Khel Arena's own greens instead of black-on-white. */
	.right :global(.btn-quiet) {
		border-color: color-mix(in srgb, var(--on-field) 26%, transparent);
		background: transparent;
		color: var(--on-field-muted);
	}

	.right :global(.btn-quiet:hover:not(:disabled)) {
		border-color: color-mix(in srgb, var(--on-field) 50%, transparent);
		color: var(--on-field);
	}

	.right :global(.btn-primary) {
		background: var(--on-field);
		color: var(--forest);
	}

	.right :global(.btn-primary:hover:not(:disabled)) {
		background: var(--accent-on-field);
	}

	.username {
		max-width: 12rem;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.clock {
		font-size: 0.875rem;
		color: var(--on-field-faint);
		white-space: nowrap;
		flex-shrink: 0;
	}

	@media (max-width: 1180px) {
		.clock {
			display: none;
		}
	}

	@media (max-width: 940px) {
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
			scrollbar-width: thin;
			padding-bottom: 0.15rem;
			-webkit-mask-image: linear-gradient(
				to right,
				transparent,
				#000 1.25rem,
				#000 calc(100% - 1.25rem),
				transparent
			);
			mask-image: linear-gradient(
				to right,
				transparent,
				#000 1.25rem,
				#000 calc(100% - 1.25rem),
				transparent
			);
		}

		.right {
			margin-left: auto;
		}
	}
</style>
