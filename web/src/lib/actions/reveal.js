/**
 * Fades a section in the moment it crosses into view, once, then lets go.
 * Purely presentational — `.reveal`/`.reveal-in` are plain CSS, so the
 * global prefers-reduced-motion rule in app.css already neutralises this.
 */
export function reveal(node, { delay = 0 } = {}) {
	if (typeof IntersectionObserver === 'undefined') return {};

	// Disabled, not sped up: reduced motion means the section is just there,
	// never gated behind a scroll intersection in the first place.
	if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return {};

	node.classList.add('reveal');
	node.style.transitionDelay = `${delay}ms`;

	const observer = new IntersectionObserver(
		(entries) => {
			for (const entry of entries) {
				if (!entry.isIntersecting) continue;
				node.classList.add('reveal-in');
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
