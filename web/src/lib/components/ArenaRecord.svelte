<script>
	import { SPORT_LABELS } from '$lib/api/index.js';
	import { formatNPR } from '$lib/time.js';

	let { arena } = $props();

	const sports = $derived([...new Set(arena.courts.map((c) => SPORT_LABELS[c.sport] ?? c.sport))]);
	const shown = $derived(arena.amenities.slice(0, 3));
	const rest = $derived(arena.amenities.length - shown.length);
</script>

<a class="card record" href="/arenas/{arena.slug}">
	<div class="head">
		<div>
			<h3 class="display display-m">{arena.name}</h3>
			<p class="where small">{arena.area}, {arena.city}</p>
		</div>
		<span class="rating num" aria-label="Rated {arena.rating} out of 5 from {arena.review_count} reviews">
			<svg viewBox="0 0 12 12" aria-hidden="true"
				><path
					d="M6 .8l1.6 3.3 3.6.5-2.6 2.5.6 3.6L6 9l-3.2 1.7.6-3.6L.8 4.6l3.6-.5z"
				/></svg
			>
			{arena.rating}
			<span class="reviews">({arena.review_count})</span>
		</span>
	</div>

	<p class="blurb">{arena.description}</p>

	<ul class="amenities small">
		{#each shown as amenity (amenity)}
			<li>{amenity}</li>
		{/each}
		{#if rest > 0}
			<li class="more">+{rest} more</li>
		{/if}
	</ul>

	<div class="foot">
		<dl>
			<div>
				<dt class="label">Courts</dt>
				<dd>{arena.court_count} · {sports.join(', ')}</dd>
			</div>
			<div>
				<dt class="label">Open</dt>
				<dd class="num">{arena.opens_at}–{arena.closes_at}</dd>
			</div>
		</dl>
		<p class="price">
			<span class="from">from</span>
			<strong class="num">NPR {formatNPR(arena.from_price_npr)}</strong>
			<span class="per">/ hour</span>
		</p>
	</div>
</a>

<style>
	.record {
		display: flex;
		flex-direction: column;
		gap: 1.15rem;
		height: 100%;
		padding: clamp(1.4rem, 2.5vw, 1.85rem);
		transition:
			border-color 0.2s var(--ease),
			box-shadow 0.2s var(--ease),
			transform 0.2s var(--ease);
	}

	.record:hover,
	.record:focus-visible {
		border-color: var(--line-strong);
		box-shadow: var(--shadow);
		transform: translateY(-2px);
	}

	.head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
	}

	h3 {
		transition: color 0.2s var(--ease);
	}

	.record:hover h3 {
		color: var(--pine);
	}

	.where {
		margin-top: 0.15rem;
		color: var(--muted);
	}

	.rating {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		flex-shrink: 0;
		padding: 0.3rem 0.65rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine);
		font-size: 0.875rem;
		font-weight: 600;
	}

	.rating svg {
		width: 12px;
		height: 12px;
		fill: currentColor;
	}

	.reviews {
		font-weight: 500;
		opacity: 0.7;
	}

	.blurb {
		color: var(--muted);
		font-size: 1rem;
		line-height: 1.6;
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.amenities {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}

	.amenities li {
		padding: 0.25rem 0.7rem;
		border: 1px solid var(--line);
		border-radius: var(--r-pill);
		color: var(--muted);
		font-size: 0.8125rem;
	}

	.amenities .more {
		border-style: dashed;
		color: var(--faint);
	}

	.foot {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem;
		margin-top: auto;
		padding-top: 1.15rem;
		border-top: 1px solid var(--line);
	}

	dl {
		display: flex;
		gap: 1.75rem;
	}

	dd {
		margin: 0;
		margin-top: 0.2rem;
		font-size: 0.9375rem;
		color: var(--ink);
	}

	.price {
		text-align: right;
		white-space: nowrap;
	}

	.from,
	.per {
		font-size: 0.8125rem;
		color: var(--faint);
	}

	.price strong {
		font-family: var(--display);
		font-size: 1.35rem;
		font-weight: 500;
		letter-spacing: -0.02em;
	}

	@media (max-width: 520px) {
		.foot {
			flex-direction: column;
			align-items: flex-start;
		}

		.price {
			text-align: left;
		}
	}
</style>
