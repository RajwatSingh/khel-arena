<script module>
	import { api } from '$lib/api/index.js';

	// What search looks through, fetched the first time the box is used and
	// kept for the rest of the visit. There is no search endpoint: the lists
	// are small (every arena, the open games, the areas), so matching
	// happens here, as you type, with no request per keystroke.
	let cache = null;

	function loadIndex() {
		cache ??= Promise.all([
			api.listArenas().catch(() => []),
			api.callFeed({ limit: 50 }).catch(() => []),
			api.listAreas().catch(() => [])
		]).then(([arenas, games, areas]) => ({
			arenas: arenas ?? [],
			games: (games ?? []).filter((g) => g.status === 'open'),
			areas: areas ?? []
		}));
		return cache;
	}

	const norm = (s) =>
		(s ?? '')
			.toString()
			.toLowerCase()
			.normalize('NFD')
			.replace(/[̀-ͯ]/g, '');

	/**
	 * Every word typed has to match one of the fields. A match at the start
	 * of the first field (the name) counts most, then the start of any
	 * word, then anywhere. Zero means no match.
	 */
	function score(query, fields) {
		const words = norm(query).split(/\s+/).filter(Boolean);
		let total = 0;
		for (const w of words) {
			let best = 0;
			fields.forEach((field, i) => {
				const v = norm(field);
				if (!v) return;
				let s = 0;
				if (v.startsWith(w)) s = 3;
				else if (v.split(/[\s·,()/-]+/).some((part) => part.startsWith(w))) s = 2;
				else if (v.includes(w)) s = 1;
				best = Math.max(best, s * (i === 0 ? 1 : 0.6));
			});
			if (!best) return 0;
			total += best;
		}
		return total;
	}

	function top(items, query, fields, limit) {
		return items
			.map((item) => ({ item, s: score(query, fields(item)) }))
			.filter((r) => r.s > 0)
			.sort((a, b) => b.s - a.s)
			.slice(0, limit)
			.map((r) => r.item);
	}

	/** Splits text into plain and matched runs, for highlighting. */
	function highlight(text, query) {
		const words = query
			.trim()
			.split(/\s+/)
			.filter(Boolean)
			.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
		if (!words.length || !text) return [{ t: text ?? '', hit: false }];
		const split = new RegExp(`(${words.join('|')})`, 'gi');
		const whole = new RegExp(`^(${words.join('|')})$`, 'i');
		return text
			.split(split)
			.filter(Boolean)
			.map((t) => ({ t, hit: whole.test(t) }));
	}
</script>

<script>
	import { tick } from 'svelte';
	import { fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { session } from '$lib/session.svelte.js';
	import { formatDate, formatNPR, formatTime } from '$lib/time.js';

	let query = $state('');
	let index = $state(null);
	let focused = $state(false);
	// Phones: the box is hidden behind the magnifier until it's tapped.
	let expanded = $state(false);
	let active = $state(0);
	let rootEl = $state();
	let inputEl = $state();

	const showResults = $derived(focused && query.trim().length > 0);

	// Navigating anywhere clears it.
	$effect(() => {
		page.url.pathname;
		query = '';
		expanded = false;
		focused = false;
	});

	function onFocus() {
		focused = true;
		loadIndex().then((i) => (index = i));
	}

	async function expand() {
		expanded = !expanded;
		if (!expanded) return;
		await tick();
		inputEl?.focus();
	}

	function close() {
		focused = false;
		expanded = false;
		inputEl?.blur();
	}

	const isOwner = $derived(session.user?.account_type === 'arena_owner');

	const pages = $derived([
		{ label: 'Find a court', sub: 'Free hours, by time', href: '/tonight' },
		{ label: 'Arenas', sub: 'Every venue in the valley', href: '/arenas' },
		{ label: 'Games needing players', sub: 'Join a game', href: '/games' },
		{ label: 'Post a game', sub: 'Find players for your booking', href: '/games/new' },
		{ label: 'My bookings', sub: 'Your holds and games', href: '/bookings' },
		...(isOwner
			? [
					{ label: 'Dashboard', sub: 'Your venues at a glance', href: '/dashboard' },
					{ label: 'My venues', sub: 'Courts, prices and payments', href: '/manage' }
				]
			: [])
	]);

	const sections = $derived.by(() => {
		const q = query.trim();
		if (!q) return [];
		const arenas = index?.arenas ?? [];
		const games = index?.games ?? [];
		const areas = index?.areas ?? [];

		const out = [
			{
				title: 'Arenas',
				items: top(arenas, q, (a) => [a.name, a.area, a.city, ...(a.sports ?? [])], 5).map(
					(a) => ({
						kind: 'arena',
						label: a.name,
						sub: [a.area, a.from_price_npr ? `from NPR ${formatNPR(a.from_price_npr)}` : null]
							.filter(Boolean)
							.join(' · '),
						href: `/arenas/${a.slug}`
					})
				)
			},
			{
				title: 'Areas',
				items: top(areas, q, (a) => [a], 3).map((a) => ({
					kind: 'area',
					label: `Courts in ${a}`,
					sub: 'Free hours in this area',
					href: `/tonight?area=${encodeURIComponent(a)}`
				}))
			},
			{
				title: 'Games',
				items: top(games, q, (g) => [g.title, g.arena_name, g.arena_area], 4).map((g) => ({
					kind: 'game',
					label: g.title,
					sub: [
						g.arena_name,
						`${formatDate(g.starts_at)} ${formatTime(g.starts_at)}`,
						`${g.spots_remaining} ${g.spots_remaining === 1 ? 'place' : 'places'} left`
					]
						.filter(Boolean)
						.join(' · '),
					href: `/games/${g.id}`
				}))
			},
			{
				title: 'Pages',
				items: top(pages, q, (p) => [p.label, p.sub], 3).map((p) => ({ kind: 'page', ...p }))
			}
		];

		// Players can only be looked up by exact username, so a query that
		// could be one gets a direct link.
		const handle = q.replace(/^@/, '');
		if (/^[a-z0-9_.-]{2,30}$/i.test(handle)) {
			out.push({
				title: 'Players',
				items: [
					{
						kind: 'player',
						label: `@${handle}`,
						sub: 'Open this player’s profile',
						href: `/players/${encodeURIComponent(handle.toLowerCase())}`
					}
				]
			});
		}
		return out.filter((s) => s.items.length);
	});

	const flat = $derived(sections.flatMap((s) => s.items));

	function go(item) {
		if (!item) return;
		close();
		goto(item.href);
	}

	// Up, down and Enter inside the box, as in any search field.
	function onKeydown(e) {
		if (e.key === 'ArrowDown' && flat.length) {
			e.preventDefault();
			active = (active + 1) % flat.length;
		} else if (e.key === 'ArrowUp' && flat.length) {
			e.preventDefault();
			active = (active - 1 + flat.length) % flat.length;
		} else if (e.key === 'Enter') {
			e.preventDefault();
			go(flat[active]);
		} else if (e.key === 'Escape') {
			close();
		}
	}

	// Clicking anywhere else closes the results.
	function onWindowPointerdown(e) {
		if ((focused || expanded) && rootEl && !rootEl.contains(e.target)) {
			focused = false;
			expanded = false;
		}
	}
</script>

<svelte:window onpointerdown={onWindowPointerdown} />

<div class="search" class:expanded bind:this={rootEl}>
	<button
		class="icon-btn"
		type="button"
		aria-label="Search"
		aria-expanded={expanded}
		onclick={expand}
	>
		<svg viewBox="0 0 20 20" width="17" height="17" aria-hidden="true">
			<circle cx="8.75" cy="8.75" r="5.75" />
			<path d="M13 13l4 4" />
		</svg>
	</button>

	<div class="box">
		<div class="field">
			<svg viewBox="0 0 20 20" width="16" height="16" aria-hidden="true">
				<circle cx="8.75" cy="8.75" r="5.75" />
				<path d="M13 13l4 4" />
			</svg>
			<input
				bind:this={inputEl}
				bind:value={query}
				oninput={() => (active = 0)}
				onfocus={onFocus}
				onkeydown={onKeydown}
				type="search"
				placeholder="Search arenas, areas, games…"
				aria-label="Search"
				autocomplete="off"
				spellcheck="false"
				role="combobox"
				aria-expanded={showResults}
				aria-controls="search-results"
				aria-activedescendant={showResults && flat.length ? `search-opt-${active}` : undefined}
			/>
			{#if query}
				<button
					type="button"
					class="clear"
					aria-label="Clear search"
					onclick={() => {
						query = '';
						inputEl?.focus();
					}}
				>
					<svg viewBox="0 0 20 20" width="14" height="14" aria-hidden="true">
						<path d="M5.5 5.5l9 9M14.5 5.5l-9 9" />
					</svg>
				</button>
			{/if}
		</div>

		{#if showResults}
			<div
				class="results"
				id="search-results"
				role="listbox"
				aria-label="Search results"
				transition:fly={{ y: -6, duration: 160, easing: cubicOut }}
			>
				{#if !index}
					<p class="note">Searching…</p>
				{:else if flat.length === 0}
					<div class="note">
						<p>Nothing matches “{query.trim()}”.</p>
						<p class="dim">Try an arena name, an area like Jhamsikhel, or a game.</p>
					</div>
				{:else}
					{#each sections as section (section.title)}
						<p class="section">{section.title}</p>
						{#each section.items as item (item.href)}
							{@const i = flat.indexOf(item)}
							<a
								id="search-opt-{i}"
								class="option"
								role="option"
								aria-selected={i === active}
								href={item.href}
								onmousemove={() => (active = i)}
								onclick={(e) => {
									e.preventDefault();
									go(item);
								}}
							>
								<span class="icon" aria-hidden="true">
									{#if item.kind === 'arena'}
										<svg viewBox="0 0 20 20"><path d="M10 17.5s5.5-4.9 5.5-9.25a5.5 5.5 0 10-11 0C4.5 12.6 10 17.5 10 17.5z" /><circle cx="10" cy="8.25" r="2" /></svg>
									{:else if item.kind === 'area'}
										<svg viewBox="0 0 20 20"><path d="M3 5.5l4.5-2 5 2 4.5-2v11l-4.5 2-5-2-4.5 2z" /><path d="M7.5 3.5v11M12.5 5.5v11" /></svg>
									{:else if item.kind === 'game'}
										<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="6.5" /><path d="M10 6.5l3 2.2-1.1 3.5H8.1L7 8.7z" /></svg>
									{:else if item.kind === 'player'}
										<svg viewBox="0 0 20 20"><circle cx="10" cy="7" r="3" /><path d="M4 16.5c.8-3 3.2-4.5 6-4.5s5.2 1.5 6 4.5" /></svg>
									{:else}
										<svg viewBox="0 0 20 20"><path d="M4 10h11M11 6l4 4-4 4" /></svg>
									{/if}
								</span>
								<span class="text">
									<span class="opt-label">
										{#each highlight(item.label, query) as part, k (k)}
											{#if part.hit}<mark>{part.t}</mark>{:else}{part.t}{/if}
										{/each}
									</span>
									{#if item.sub}<span class="sub">{item.sub}</span>{/if}
								</span>
							</a>
						{/each}
					{/each}
				{/if}
			</div>
		{/if}
	</div>
</div>

<style>
	.search {
		position: relative;
	}

	svg {
		flex-shrink: 0;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.8;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	/* --- the box --- */

	.icon-btn {
		display: none;
	}

	.field {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		width: 16rem;
		height: 38px;
		padding: 0 0.4rem 0 0.8rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-pill);
		background: #ffffff;
		color: var(--muted);
		transition:
			border-color var(--dur-hover) var(--ease),
			box-shadow var(--dur-hover) var(--ease);
	}

	.field:hover {
		border-color: var(--pine);
	}

	.field:focus-within {
		border-color: var(--pine);
		box-shadow: 0 0 0 3px var(--pine-wash);
	}

	input {
		flex: 1;
		min-width: 0;
		height: 100%;
		border: 0;
		background: none;
		font: inherit;
		font-size: 0.9rem;
		color: var(--ink);
		outline: none;
	}

	input::placeholder {
		color: var(--faint);
	}

	input::-webkit-search-cancel-button {
		display: none;
	}

	.clear {
		display: grid;
		place-items: center;
		width: 26px;
		height: 26px;
		padding: 0;
		border: 0;
		border-radius: 50%;
		background: var(--field);
		color: var(--muted);
		cursor: pointer;
	}

	.clear:hover {
		color: var(--ink);
	}

	/* --- results --- */

	.results {
		position: absolute;
		top: calc(100% + 0.5rem);
		right: 0;
		z-index: 60;
		width: min(26rem, calc(100vw - 2rem));
		max-height: min(28rem, calc(100dvh - 6rem));
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 0.35rem 0.4rem 0.5rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-md);
		background: #ffffff;
		box-shadow:
			0 2px 6px rgba(16, 33, 22, 0.08),
			0 24px 48px -20px rgba(16, 33, 22, 0.4);
	}

	.section {
		margin: 0.6rem 0.6rem 0.25rem;
		font-size: 0.6875rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--faint);
	}

	.option {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		align-items: center;
		gap: 0.7rem;
		padding: 0.5rem 0.6rem;
		border-radius: var(--r-sm);
		color: var(--ink);
		text-decoration: none;
	}

	.option[aria-selected='true'] {
		background: var(--pine-wash);
	}

	.icon {
		display: grid;
		place-items: center;
		width: 32px;
		height: 32px;
		border-radius: 9px;
		background: var(--field);
		color: var(--pine-deep);
	}

	.option[aria-selected='true'] .icon {
		background: #ffffff;
	}

	.icon svg {
		width: 17px;
		height: 17px;
		stroke-width: 1.6;
	}

	.text {
		display: grid;
		min-width: 0;
	}

	.opt-label,
	.sub {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.opt-label {
		font-size: 0.9375rem;
		font-weight: 600;
	}

	.sub {
		font-size: 0.8125rem;
		color: var(--muted);
	}

	mark {
		background: none;
		color: var(--pine-deep);
		text-decoration: underline;
		text-decoration-color: color-mix(in srgb, var(--pine) 50%, transparent);
		text-decoration-thickness: 2px;
		text-underline-offset: 0.2em;
	}

	.note {
		margin: 0;
		padding: 1.25rem 1rem;
		font-size: 0.9rem;
		color: var(--muted);
		text-align: center;
	}

	.note p {
		margin: 0;
	}

	.dim {
		margin-top: 0.3rem !important;
		font-size: 0.8125rem;
		color: var(--faint);
	}

	/* Narrower bars: a magnifier button. Tapping it opens the box as a
	   full-width row under the header, with results below it. */
	@media (max-width: 1240px) {
		.search {
			position: static;
		}

		.icon-btn {
			display: grid;
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

		.expanded .icon-btn {
			border-color: var(--pine);
			background: var(--pine-wash);
		}

		.box {
			display: none;
		}

		.expanded .box {
			display: block;
			position: absolute;
			top: 100%;
			left: 0;
			right: 0;
			padding: 0.6rem var(--gutter) 0.75rem;
			background: var(--surface);
			border-bottom: 1px solid var(--line);
			box-shadow: 0 12px 24px -16px rgba(16, 33, 22, 0.3);
		}

		.field,
		.results {
			max-width: 40rem;
			margin-inline: auto;
		}

		.field {
			width: 100%;
			height: 44px;
		}

		input {
			font-size: 1rem; /* 16px stops iOS zooming in on focus */
		}

		.results {
			position: static;
			width: 100%;
			max-height: calc(100dvh - 10rem);
			margin-top: 0.5rem;
		}
	}
</style>
