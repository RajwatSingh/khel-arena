<script>
	/**
	 * An arena's reviews, and the caller's own.
	 *
	 * A review has to be earned by having played at the venue, so this asks
	 * the server whether it may offer the form at all rather than showing one
	 * that would be refused. Three states: write, edit, or explain.
	 */
	import { api, ApiError } from '$lib/api/index.js';
	import { session } from '$lib/session.svelte.js';
	import { formatDate } from '$lib/time.js';

	let { arenaId } = $props();

	let reviews = $state([]);
	let mine = $state(null);
	let canReview = $state(false);
	let form = $state({ rating: 5, comment: '' });
	let busy = $state(false);
	let error = $state(null);
	let version = $state(0);

	$effect(() => {
		void version;
		const id = arenaId;
		let cancelled = false;

		api
			.arenaReviews(id)
			.then((next) => {
				if (!cancelled) reviews = next;
			})
			.catch(() => {
				if (!cancelled) reviews = [];
			});

		if (!session.signedIn) {
			mine = null;
			canReview = false;
			return;
		}

		api
			.myReview(id)
			.then((next) => {
				if (cancelled) return;
				mine = next.review;
				canReview = next.can_review;
				if (next.review) {
					form = { rating: next.review.rating, comment: next.review.comment };
				}
			})
			.catch(() => {
				if (!cancelled) {
					mine = null;
					canReview = false;
				}
			});

		return () => {
			cancelled = true;
		};
	});

	async function act(fn) {
		busy = true;
		error = null;
		try {
			await fn();
			version++;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'That did not go through.';
		} finally {
			busy = false;
		}
	}

	async function save(event) {
		event.preventDefault();
		await act(() =>
			api.reviewArena(arenaId, { rating: Number(form.rating), comment: form.comment })
		);
	}

	const remove = () => act(() => api.deleteReview(arenaId));

	const stars = (n) => '★'.repeat(n) + '☆'.repeat(5 - n);
</script>

<section class="reviews">
	<h2 class="display display-m">What players say</h2>

	{#if error}
		<p class="error small" role="alert">{error}</p>
	{/if}

	{#if session.signedIn}
		{#if canReview}
			<form class="card mine" onsubmit={save}>
				<h3 class="label">{mine ? 'Your review' : 'Leave a review'}</h3>

				<fieldset class="rating">
					<legend class="sr-only">Rating</legend>
					{#each [1, 2, 3, 4, 5] as n (n)}
						<button
							type="button"
							class="star"
							class:on={n <= form.rating}
							aria-label="{n} out of 5"
							aria-pressed={n === form.rating}
							onclick={() => (form.rating = n)}
						>
							★
						</button>
					{/each}
				</fieldset>

				<label class="say" for="review-comment">
					<span class="sr-only">Comment</span>
					<textarea
						id="review-comment"
						bind:value={form.comment}
						maxlength="500"
						rows="3"
						placeholder="Flat turf, lights work, parking is tight after seven."
					></textarea>
				</label>

				<div class="actions">
					<button class="btn btn-primary" class:loading={busy} disabled={busy}>
						{mine ? 'Update' : 'Post'}
					</button>
					{#if mine}
						<button type="button" class="btn btn-quiet" disabled={busy} onclick={remove}>
							Delete
						</button>
					{/if}
				</div>
			</form>
		{:else}
			<p class="small quiet gate">
				You can review this arena once you've played here. That's what keeps the ratings worth
				reading.
			</p>
		{/if}
	{/if}

	{#if reviews.length}
		<ul>
			{#each reviews as review (review.id)}
				<li>
					<div class="head">
						<strong>{review.author?.full_name || review.author?.username || 'A player'}</strong>
						<span class="score" aria-label="{review.rating} out of 5">{stars(review.rating)}</span>
					</div>
					{#if review.comment}<p class="comment">{review.comment}</p>{/if}
					<p class="when small">{formatDate(review.created_at)}</p>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="small quiet">No reviews yet.</p>
	{/if}
</section>

<style>
	.reviews { margin-top: clamp(2.5rem, 5vw, 4rem); }
	h2 { margin: 0 0 1rem; }

	.mine {
		display: grid;
		gap: 0.85rem;
		max-width: 34rem;
		margin-bottom: 1.5rem;
		padding: clamp(1.25rem, 3vw, 1.75rem);
	}

	h3 { margin: 0; }

	.rating { display: flex; gap: 0.15rem; padding: 0; border: 0; margin: 0; }

	/* Stars rather than a number: a rating is a feeling, and five buttons say
	   that better than a select does. */
	.star {
		padding: 0;
		border: 0;
		background: none;
		font-size: 1.6rem;
		line-height: 1;
		color: var(--line-strong);
		cursor: pointer;
		transition: color var(--dur-hover) var(--ease);
	}

	.star.on { color: var(--pine); }
	.star:hover { color: var(--pine-deep); }

	textarea {
		width: 100%;
		padding: 0.6rem 0.75rem;
		border: 1px solid var(--line-strong);
		border-radius: var(--r-sm);
		background: var(--field);
		font: inherit;
		font-size: 0.92rem;
		resize: vertical;
	}

	textarea:focus-visible { outline: 2px solid var(--pine); outline-offset: 1px; }

	.actions { display: flex; gap: 0.5rem; }

	.gate {
		max-width: 44ch;
		margin-bottom: 1.5rem;
		padding: 0.85rem 1rem;
		border: 1px dashed var(--line-strong);
		border-radius: var(--r-md);
		color: var(--muted);
	}

	ul { display: grid; gap: 0.75rem; }

	li {
		padding: 1rem 1.15rem;
		border: 1px solid var(--line);
		border-radius: var(--r-md);
		background: var(--surface);
	}

	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
	}

	.score { color: var(--pine); letter-spacing: 0.1em; }
	.comment { margin: 0.5rem 0 0; }
	.when { margin: 0.4rem 0 0; color: var(--faint); }
	.quiet { color: var(--muted); }

	.error {
		padding: 0.6rem 0.75rem;
		margin-bottom: 1rem;
		border-radius: var(--r-sm);
		background: var(--brick-wash);
		color: var(--brick);
	}
</style>
