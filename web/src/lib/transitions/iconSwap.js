import { cubicOut } from 'svelte/easing';

/**
 * Swaps one icon for another in the same slot: it grows in from a quarter
 * of its size, clears a blur, and fades up — and, run in reverse, the
 * outgoing icon does the identical move backwards. Same technique as
 * chanhdai.com's icon-swap component (scale 0.25→1, blur 4px→0, opacity
 * 0→1, ~300ms, no bounce), reimplemented here as a plain Svelte transition
 * since nothing else on this site pulls in an animation library.
 */
export function iconSwap(_node, { duration = 300 } = {}) {
	if (
		typeof window !== 'undefined' &&
		window.matchMedia('(prefers-reduced-motion: reduce)').matches
	) {
		return { duration: 0 };
	}

	return {
		duration,
		easing: cubicOut,
		css: (t) => `
			opacity: ${t};
			transform: scale(${0.25 + 0.75 * t});
			filter: blur(${(1 - t) * 4}px);
		`
	};
}
