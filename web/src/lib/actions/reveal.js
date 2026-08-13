/**
 * Brings a section in the moment it first crosses into view, once, then
 * lets go for good — scrolling back past it never replays or reverses the
 * motion. `.reveal`/`.reveal-in` are plain CSS (blur clears, the element
 * lifts a few pixels, opacity settles); this action's only job is flipping
 * the class at the right moment and never touching it again.
 */
export function reveal(node, { delay = 0 } = {}) {
	if (typeof IntersectionObserver === 'undefined') return {};

	// Disabled, not sped up: reduced motion means the section is just there,
	// never gated behind a scroll intersection in the first place.
	if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return {};

	node.classList.add('reveal');
	node.style.setProperty('--reveal-delay', `${delay}ms`);

	const observer = new IntersectionObserver(
		(entries) => {
			for (const entry of entries) {
				if (!entry.isIntersecting) continue;
				node.classList.add('reveal-in');
				// Once, ever — the whole point of a one-shot reveal.
				observer.unobserve(node);
			}
		},
		{ threshold: 0.15 }
	);

	observer.observe(node);

	return {
		destroy() {
			observer.disconnect();
		}
	};
}
