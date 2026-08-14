<script>
	/**
	 * A line typed into place, letter by letter, cursor blinking only while
	 * it's actually mid-word — starts once on scroll-into-view (mirrors
	 * reveal.js's one-shot contract) rather than replaying on every visit
	 * to the footer.
	 */
	let { text = '', speed = 38, startDelay = 0 } = $props();

	let node = $state(null);
	let shown = $state(0);
	let started = $state(false);
	let done = $state(false);

	function typeIt() {
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
			shown = text.length;
			started = true;
			done = true;
			return;
		}

		started = true;
		let i = 0;
		const timer = setInterval(() => {
			i += 1;
			shown = i;
			if (i >= text.length) {
				clearInterval(timer);
				done = true;
			}
		}, speed);
	}

	function begin() {
		if (startDelay > 0) setTimeout(typeIt, startDelay);
		else typeIt();
	}

	$effect(() => {
		if (!node || typeof IntersectionObserver === 'undefined') return;
		const observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) {
						begin();
						observer.unobserve(node);
					}
				}
			},
			{ threshold: 0.4 }
		);
		observer.observe(node);
		return () => observer.disconnect();
	});
</script>

<span bind:this={node} class="typing" aria-label={text}>
	<span aria-hidden="true">{text.slice(0, shown)}</span><span
		class="caret"
		class:typing-now={started && !done}
		aria-hidden="true"
	></span>
</span>

<style>
	.typing {
		display: inline-block;
	}

	.caret {
		display: inline-block;
		width: 2px;
		height: 0.85em;
		margin-left: 2px;
		background: currentColor;
		vertical-align: -0.1em;
		opacity: 0;
	}

	.caret.typing-now {
		opacity: 1;
		animation: caret-blink 0.85s step-end infinite;
	}

	@keyframes caret-blink {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0;
		}
	}
</style>
