<script>
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { session, clock } from '$lib/session.svelte.js';
	import CentreMark from './CentreMark.svelte';
	import SiteSearch from './SiteSearch.svelte';

	const ticking = clock();

	// Primary nav stays at four: the two ways in (a court, a game) and the
	// two registers behind them. Teams, tournaments and the back office are
	// reachable from the pages that need them and from the footer.
	// A venue operator and a player want different bars. The operator's four
	// are their dashboard and back office plus the two public boards they
	// still need to see; the player's are the two ways in and their holds.
	const playerLinks = [
		{ href: '/tonight', label: 'Find a court' },
		{ href: '/games', label: 'Games' },
		{ href: '/arenas', label: 'Arenas' },
		{ href: '/bookings', label: 'My bookings' }
	];

	const ownerLinks = [
		{ href: '/dashboard', label: 'Dashboard' },
		{ href: '/manage', label: 'My venues' },
		{ href: '/arenas', label: 'Arenas' },
		{ href: '/games', label: 'Games' }
	];

	const user = $derived(session.user);
	const isOwner = $derived(user?.account_type === 'arena_owner');
	const links = $derived(isOwner ? ownerLinks : playerLinks);

	// Two initials from the full name, or the first letter of the username
	// when there is no name to work from.
	const initials = $derived.by(() => {
		if (!user) return '';
		const parts = (user.full_name ?? '').trim().split(/\s+/).filter(Boolean);
		if (parts.length >= 2) return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
		if (parts.length === 1) return parts[0][0].toUpperCase();
		return (user.username?.[0] ?? '?').toUpperCase();
	});

	// Kathmandu runs at UTC+05:45 all year.
	const kathmandu = $derived(
		new Date(ticking.now + (5 * 60 + 45) * 60_000).toISOString().slice(11, 16)
	);

	let menuOpen = $state(false); // phone nav sheet
	let profileOpen = $state(false); // avatar panel
	let avatarFailed = $state(false);
	let profileEl = $state();
	let menuEl = $state();

	// Any navigation closes whatever was open.
	$effect(() => {
		page.url.pathname;
		menuOpen = false;
		profileOpen = false;
	});

	// A new user means a new picture to try.
	$effect(() => {
		user?.avatar_url;
		avatarFailed = false;
	});

	function current(href) {
		return page.url.pathname === href || page.url.pathname.startsWith(href + '/');
	}

	function toggleProfile() {
		profileOpen = !profileOpen;
		if (profileOpen) menuOpen = false;
	}

	function toggleMenu() {
		menuOpen = !menuOpen;
		if (menuOpen) profileOpen = false;
	}

	function onWindowClick(e) {
		if (profileOpen && profileEl && !profileEl.contains(e.target)) profileOpen = false;
		if (menuOpen && menuEl && !menuEl.contains(e.target)) menuOpen = false;
	}

	function onKeydown(e) {
		if (e.key === 'Escape' && (profileOpen || menuOpen)) {
			profileOpen = false;
			menuOpen = false;
		}
	}

	// Signing out always ends on the sign-in page. replaceState swaps out the
	// page being left, so Back does not land on it; and every private page
	// checks the session when it loads, so going further back lands on sign
	// in too rather than on anything that was private.
	async function signOut() {
		profileOpen = false;
		try {
			await session.signOut();
		} catch {
			// The local session is cleared whatever the server said; a failed
			// network call must not leave the person on the page they left.
		}
		await goto('/login', { replaceState: true, invalidateAll: true });
	}
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

<header>
	<div class="shell bar">
		<a class="mark" href="/">
			<CentreMark width={24} label="Khel Arena" />
			<span>Khel Arena</span>
		</a>

		<nav aria-label="Primary" class="desktop-nav">
			{#each links as link (link.href)}
				<a href={link.href} class="tab" aria-current={current(link.href) ? 'page' : undefined}>
					{link.label}
				</a>
			{/each}
		</nav>

		<div class="right">
			<SiteSearch />
			<span class="clock num">Kathmandu {kathmandu}</span>

			{#if session.signedIn && user}
				<div class="profile" bind:this={profileEl}>
					<button
						class="avatar-btn"
						aria-label="Account: {user.username}"
						aria-haspopup="true"
						aria-expanded={profileOpen}
						aria-controls="profile-panel"
						onclick={toggleProfile}
					>
						{#if user.avatar_url && !avatarFailed}
							<img src={user.avatar_url} alt="" onerror={() => (avatarFailed = true)} />
						{:else}
							<span aria-hidden="true">{initials}</span>
						{/if}
					</button>

					{#if profileOpen}
						<div class="panel profile-panel" id="profile-panel">
							<div class="who">
								<p class="name">{user.full_name || user.username}</p>
								<p class="handle">@{user.username}</p>
								{#if user.email}<p class="email">{user.email}</p>{/if}
								<span class="role">{isOwner ? 'Venue owner' : 'Player'}</span>
							</div>
							<div class="actions">
								<a href="/players/{user.username}">View profile</a>
								<button class="signout" onclick={signOut}>Sign out</button>
							</div>
						</div>
					{/if}
				</div>
			{:else}
				<a class="btn btn-quiet signin" href="/login">Sign in</a>
				<a class="btn btn-primary get-started" href="/register">Get started</a>
			{/if}

			<div class="menu" bind:this={menuEl}>
				<button
					class="menu-btn"
					aria-label="Menu"
					aria-expanded={menuOpen}
					aria-controls="mobile-nav"
					onclick={toggleMenu}
				>
					<svg width="20" height="20" viewBox="0 0 20 20" aria-hidden="true">
						{#if menuOpen}
							<path d="M5 5l10 10M15 5L5 15" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
						{:else}
							<path d="M3 6h14M3 10h14M3 14h14" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
						{/if}
					</svg>
				</button>

				{#if menuOpen}
					<nav aria-label="Primary" class="panel mobile-nav" id="mobile-nav">
						{#each links as link (link.href)}
							<a href={link.href} aria-current={current(link.href) ? 'page' : undefined}>
								{link.label}
							</a>
						{/each}
						{#if !session.signedIn}
							<a href="/register" class="mobile-cta">Get started</a>
						{/if}
					</nav>
				{/if}
			</div>
		</div>
	</div>
</header>

<style>
	/* A light bar, so the dark forest band each page opens with reads as its
	   own title block instead of merging with the nav into one slab. */
	header {
		position: sticky;
		top: 0;
		z-index: 40;
		background: color-mix(in srgb, var(--surface) 88%, transparent);
		backdrop-filter: blur(12px);
		-webkit-backdrop-filter: blur(12px);
		border-bottom: 1px solid var(--line);
	}

	.bar {
		display: flex;
		align-items: center;
		gap: clamp(0.75rem, 2.5vw, 1.5rem);
		min-height: 64px;
		padding-block: 0.5rem;
	}

	.mark {
		display: inline-flex;
		align-items: center;
		gap: 0.6rem;
		font-family: var(--display);
		font-size: 1.1875rem;
		font-weight: 800;
		letter-spacing: -0.02em;
		white-space: nowrap;
		color: var(--ink);
		flex-shrink: 0;
	}

	.mark :global(svg) {
		color: var(--pine);
		transition: transform 0.5s var(--ease);
	}

	.mark:hover :global(svg) {
		transform: rotate(180deg);
	}

	.desktop-nav {
		display: flex;
		gap: 0.2rem;
		margin-right: auto;
		min-width: 0;
	}

	.tab {
		padding: 0.45rem 0.85rem;
		border-radius: var(--r-pill);
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--muted);
		white-space: nowrap;
		transition:
			background-color var(--dur-hover) var(--ease),
			color var(--dur-hover) var(--ease);
	}

	.tab:hover {
		background: var(--surface-sunk);
		color: var(--ink);
	}

	.tab[aria-current='page'] {
		background: var(--pine-wash);
		color: var(--pine-deep);
	}

	.right {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		flex-shrink: 0;
	}

	.right :global(.btn) {
		white-space: nowrap;
		padding-block: 0.6em;
	}

	.clock {
		font-size: 0.875rem;
		color: var(--muted);
		white-space: nowrap;
	}

	/* ---- avatar + panel ---- */

	.profile,
	.menu {
		position: relative;
	}

	.avatar-btn {
		display: grid;
		place-items: center;
		width: 36px;
		height: 36px;
		padding: 0;
		border: 0;
		border-radius: 50%;
		overflow: hidden;
		background: var(--pine);
		color: #ffffff;
		font-size: 0.8125rem;
		font-weight: 700;
		letter-spacing: 0.02em;
		cursor: pointer;
		box-shadow: 0 0 0 2px var(--surface), 0 0 0 3px var(--line-strong);
		transition: box-shadow var(--dur-hover) var(--ease);
	}

	.avatar-btn:hover,
	.avatar-btn[aria-expanded='true'] {
		box-shadow: 0 0 0 2px var(--surface), 0 0 0 3px var(--pine);
	}

	.avatar-btn img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.panel {
		position: absolute;
		top: calc(100% + 0.6rem);
		right: 0;
		z-index: 50;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		box-shadow: var(--shadow);
	}

	.profile-panel {
		width: min(17rem, calc(100vw - 2rem));
		overflow: hidden;
	}

	.who {
		padding: 1rem 1rem 0.9rem;
		border-bottom: 1px solid var(--line);
	}

	.who p {
		margin: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.name {
		font-weight: 700;
		color: var(--ink);
	}

	.handle,
	.email {
		font-size: 0.875rem;
		color: var(--muted);
	}

	.role {
		display: inline-block;
		margin-top: 0.6rem;
		padding: 0.15rem 0.55rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.75rem;
		font-weight: 600;
	}

	.actions {
		display: flex;
		flex-direction: column;
		padding: 0.35rem;
	}

	.actions a,
	.actions button,
	.mobile-nav a {
		display: block;
		width: 100%;
		padding: 0.6rem 0.75rem;
		border: 0;
		border-radius: var(--r-sm);
		background: none;
		text-align: left;
		font: inherit;
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--ink);
		text-decoration: none;
		cursor: pointer;
	}

	.actions a:hover,
	.actions button:hover,
	.mobile-nav a:hover {
		background: var(--surface-sunk);
	}

	.signout {
		color: var(--brick) !important;
	}

	/* ---- phone menu ---- */

	.menu-btn {
		display: none;
		place-items: center;
		width: 38px;
		height: 38px;
		padding: 0;
		border: 1px solid var(--line);
		border-radius: 50%;
		background: var(--surface);
		color: var(--ink);
		cursor: pointer;
	}

	.mobile-nav {
		width: min(15rem, calc(100vw - 2rem));
		padding: 0.35rem;
	}

	.mobile-nav a[aria-current='page'] {
		background: var(--pine-wash);
		color: var(--pine-deep);
	}

	.mobile-cta {
		margin-top: 0.25rem;
		border-top: 1px solid var(--line) !important;
		color: var(--pine) !important;
	}

	@media (max-width: 1180px) {
		.clock {
			display: none;
		}
	}

	/* Below this the four tabs no longer fit beside the mark, so they move
	   into the menu instead of wrapping onto a second row. */
	@media (max-width: 940px) {
		.desktop-nav {
			display: none;
		}

		.right {
			margin-left: auto;
		}

		.menu-btn {
			display: grid;
		}

		.get-started {
			display: none;
		}
	}
</style>
