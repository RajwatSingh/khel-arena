<script>
	import { page } from '$app/state';
</script>

<section class="band">
	<div class="shell">
		<div class="card box">
			<p class="label num">Error {page.status}</p>
			<h1 class="display display-l">{page.error?.message ?? 'That page is not here'}</h1>
			<p class="lede">The board is still up. Everything bookable in the valley is one tap away.</p>
			<div class="acts">
				<a class="btn btn-primary" href="/tonight">Find a court</a>
				<a class="btn btn-secondary" href="/">Back to the front</a>
			</div>
		</div>
	</div>
</section>

<style>
	.box {
		padding: clamp(2rem, 6vw, 4rem);
		max-width: 44rem;
	}

	h1 {
		margin-top: 0.7rem;
		max-width: 18ch;
	}

	.lede {
		margin-top: 1rem;
	}

	.acts {
		display: flex;
		flex-wrap: wrap;
		gap: 0.6rem;
		margin-top: 2rem;
	}
</style>
