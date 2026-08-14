<script>
	/**
	 * A headline reveal, word by word: the same blur-and-rise the site
	 * already uses for scroll-triggered sections (.reveal in app.css),
	 * just staggered per word instead of applied to a whole block. Drop
	 * it inside an existing heading — it renders inline spans only, so
	 * the heading keeps its own margin/max-width/balance rules. Re-runs
	 * whenever `text` changes — a scoreboard flipping to a new line reads
	 * as a feature here (the /tonight date, an arena name), not a glitch.
	 */
	let { text = '', stagger = 40, delay = 0 } = $props();

	const words = $derived(text.split(' ').filter(Boolean));
</script>

{#each words as word, i (i + '-' + word)}<span
		class="ta-word"
		style="--i: {i}; --delay: {delay}ms; --stagger: {stagger}ms">{word}</span
	>{i < words.length - 1 ? ' ' : ''}{/each}

<style>
	.ta-word {
		display: inline-block;
		opacity: 0;
		filter: blur(9px);
		transform: translateY(0.3em);
		animation: ta-word-in 640ms var(--ease-reveal) forwards;
		animation-delay: calc(var(--delay) + var(--i) * var(--stagger));
		will-change: opacity, filter, transform;
	}

	@keyframes ta-word-in {
		to {
			opacity: 1;
			filter: blur(0);
			transform: none;
		}
	}
</style>
