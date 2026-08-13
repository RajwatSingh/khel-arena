<script module>
	// One off-screen canvas, reused for every measurement — creating a new
	// one per truncation call would be wasteful for something that can run
	// on every resize.
	let cachedCtx = null;

	function getCtx() {
		if (!cachedCtx) {
			cachedCtx = document.createElement('canvas').getContext('2d');
		}
		return cachedCtx;
	}

	function measure(text, font) {
		const ctx = getCtx();
		ctx.font = font;
		return ctx.measureText(text).width;
	}

	function computeTruncated(text, width, font, ellipsis) {
		const fullWidth = measure(text, font);
		if (fullWidth <= width) return text;

		const ellipsisWidth = measure(ellipsis, font);
		const available = width - ellipsisWidth;
		if (available <= 0) return ellipsis;

		// Binary search the longest (start + end) pair that still fits,
		// splitting evenly between the two halves.
		let lo = 0;
		let hi = text.length;
		while (lo < hi) {
			const mid = Math.ceil((lo + hi) / 2);
			const startLen = Math.floor(mid / 2);
			const endLen = Math.ceil(mid / 2);
			const combined = text.slice(0, startLen) + text.slice(text.length - endLen);
			if (measure(combined, font) <= available) lo = mid;
			else hi = mid - 1;
		}

		const startLen = Math.floor(lo / 2);
		const endLen = Math.ceil(lo / 2);
		return text.slice(0, startLen) + ellipsis + text.slice(text.length - endLen);
	}
</script>

<script>
	import { untrack } from 'svelte';

	/**
	 * Truncates in the middle instead of at the end, so a long name still
	 * shows where it starts and where it ends — "Chandragiri Sports…b" reads
	 * as noise, "Chandra…Hub" still names the place. Measures real glyph
	 * widths against the element's own computed font on a cached off-screen
	 * canvas, since CSS text-overflow can only ever cut from one side.
	 */
	let { text, ellipsis = '…', class: className = '' } = $props();

	let el = $state(null);
	// Just the initial snapshot before measurement runs — the effect below
	// keeps it in sync with `text` from then on, untrack() says as much.
	let displayed = $state(untrack(() => text));

	$effect(() => {
		if (!el) return;

		function recalc(width) {
			const cs = getComputedStyle(el);
			const font = `${cs.fontStyle} ${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`;
			displayed = computeTruncated(text, width, font, ellipsis);
		}

		let timeoutId;
		let rafId;
		const ro = new ResizeObserver(([entry]) => {
			clearTimeout(timeoutId);
			timeoutId = setTimeout(() => {
				rafId = requestAnimationFrame(() => recalc(entry.contentRect.width));
			}, 120);
		});

		recalc(el.offsetWidth);
		ro.observe(el);

		return () => {
			ro.disconnect();
			clearTimeout(timeoutId);
			if (rafId) cancelAnimationFrame(rafId);
		};
	});
</script>

<span bind:this={el} class="mid-trunc {className}" title={text}>{displayed}</span>

<style>
	.mid-trunc {
		display: block;
		width: 100%;
		overflow: hidden;
		white-space: nowrap;
		/* A plain end-ellipsis safety net for the moment before layout and
		   JS measurement have run — the middle-elided version replaces it
		   as soon as the effect fires. */
		text-overflow: ellipsis;
	}
</style>
