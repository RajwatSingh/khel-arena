<script>
	/**
	 * A venue's gallery.
	 *
	 * A horizontal strip rather than a grid: an arena has a handful of photos,
	 * and a strip that scrolls says "there are more" without reserving a screen
	 * of empty rows for venues that only uploaded two.
	 */
	let { photos } = $props();

	// A photo row whose file has gone missing (or a seed row pointing at a
	// host that never resolves) should drop out of the strip rather than
	// leave a broken-image box behind.
	let broken = $state(new Set());
	const shown = $derived(photos.filter((p) => !broken.has(p.id)));
</script>

{#if shown.length}
	<section class="gallery">
		<h2 class="display display-m">The place</h2>
		<ul>
			{#each shown as photo (photo.id)}
				<li>
					<figure>
						<img
							src={photo.url}
							alt={photo.caption || 'The arena'}
							loading="lazy"
							onerror={() => (broken = new Set(broken).add(photo.id))}
						/>
						{#if photo.caption}<figcaption class="small">{photo.caption}</figcaption>{/if}
					</figure>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<style>
	.gallery { margin-top: clamp(2.5rem, 5vw, 4rem); }
	h2 { margin: 0 0 1rem; }

	ul {
		display: flex;
		gap: 0.75rem;
		overflow-x: auto;
		/* The strip scrolls inside itself; the page never does. */
		padding-bottom: 0.5rem;
		scroll-snap-type: x mandatory;
	}

	li {
		flex: none;
		width: clamp(14rem, 40vw, 22rem);
		scroll-snap-align: start;
	}

	img {
		display: block;
		width: 100%;
		aspect-ratio: 4 / 3;
		object-fit: cover;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface-sunk);
	}

	figure { margin: 0; }
	figcaption { margin-top: 0.4rem; color: var(--faint); }
</style>
