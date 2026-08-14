<script>
	import { page } from '$app/state';
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
</script>

<section class="forest-band">
	<div class="shell box">
		<p class="label num fade-up">Error {page.status}</p>
		<h1 class="display display-l">
			<TextAnimate text={page.error?.message ?? 'That page is not here'} />
		</h1>
		<div class="pitch-mark fade-up fade-up-1">
			<CentreMark wide />
		</div>
		<p class="lede fade-up fade-up-2">
			The board is still up. Everything bookable in the valley is one tap away.
		</p>
		<div class="acts fade-up fade-up-2">
			<a class="btn btn-primary" href="/tonight">Find a court</a>
			<a class="btn btn-secondary" href="/">Back to the front</a>
		</div>
	</div>
</section>

<style>
	.box {
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
