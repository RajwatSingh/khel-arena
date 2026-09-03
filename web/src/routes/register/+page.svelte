<script>
	import CentreMark from '$lib/components/CentreMark.svelte';
	import TextAnimate from '$lib/components/TextAnimate.svelte';
</script>

<svelte:head>
	<title>Create an account | Khel Arena</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<section class="auth forest-band">
	<div class="shell">
		<div class="say">
			<h1 class="display display-l"><TextAnimate text="Which way in?" /></h1>
			<div class="pitch-mark fade-up fade-up-1">
				<CentreMark wide />
			</div>
			<p class="lede fade-up fade-up-2">
				Two kinds of account. Pick the one that fits — you can't be both on the same login.
			</p>
		</div>

		<div class="choices">
			<a class="choice card fade-up fade-up-1" href="/register/player">
				<span class="tag">Play</span>
				<h2 class="display display-m">I'm here to book and play</h2>
				<p>
					Hold a court by the hour, find a game when you're a man short, track your record and
					reviews.
				</p>
				<span class="go">Create a player card →</span>
			</a>

			<a class="choice card fade-up fade-up-2" href="/register/owner">
				<span class="tag">Run a venue</span>
				<h2 class="display display-m">I run a futsal</h2>
				<p>
					List your courts, set your own hours and rates, and see every booking, the money it made
					and the cash still owed on one dashboard.
				</p>
				<span class="go">Open a venue account →</span>
			</a>
		</div>

		<p class="alt small">Already have an account? <a class="link" href="/login">Sign in</a>.</p>
	</div>
</section>

<style>
	.say .lede {
		margin-top: 1.1rem;
		max-width: 52ch;
	}

	.choices {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr));
		gap: clamp(1rem, 3vw, 1.75rem);
		margin-top: clamp(1.75rem, 4vw, 2.75rem);
	}

	.choice {
		display: grid;
		gap: 0.75rem;
		align-content: start;
		padding: clamp(1.5rem, 3vw, 2rem);
		color: inherit;
		text-decoration: none;
		transition:
			border-color var(--dur-hover) var(--ease),
			transform var(--dur-hover) var(--ease);
	}

	.choice:hover,
	.choice:focus-visible {
		border-color: var(--pine);
		transform: translateY(-2px);
	}

	.choice h2 {
		margin: 0;
	}

	.choice p {
		margin: 0;
		color: var(--muted);
		font-size: 0.9375rem;
	}

	.tag {
		justify-self: start;
		padding: 0.2rem 0.6rem;
		border-radius: var(--r-pill);
		background: var(--pine-wash);
		color: var(--pine-deep);
		font-size: 0.8125rem;
		font-weight: 600;
	}

	.go {
		margin-top: 0.35rem;
		font-weight: 600;
		color: var(--pine-deep);
	}

	.alt {
		margin-top: clamp(1.5rem, 4vw, 2.5rem);
		color: var(--muted);
	}
</style>
