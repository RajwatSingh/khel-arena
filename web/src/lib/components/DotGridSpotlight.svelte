<script>
	/**
	 * A second, quieter layer behind everything: the same idea as the
	 * ambient ledger's breathing grid, but a plain dot lattice that answers
	 * back to the cursor — dots near the pointer swell a touch and warm
	 * barely off the page tone, the rest stay a hair's-width above
	 * invisible. Modelled on chanhdai.com's dot-grid-spotlight component,
	 * re-themed to this site's own tokens instead of its default
	 * white-on-dark palette, and kept off the hit-testing tree entirely
	 * (pointer-events: none) so it never stands between a click and the
	 * thing being clicked.
	 *
	 * It still switches itself off — not just fades, off — over anything
	 * a person can click or read as a control: rows and cards here sit
	 * flush against the page (no card boxes left, by design), so without
	 * this the glow would light up directly behind the text someone is
	 * trying to read.
	 */
	const spacing = 34;
	const baseRadius = 1;
	const activeRadius = 2.4;
	const interactionRadius = 150;
	const baseAlpha = 0.16;
	const activeMinAlpha = 0.3;
	const activeMaxAlpha = 0.65;

	const interactiveSelector =
		'a, button, select, input, textarea, label, [role="button"], [tabindex]';

	function mixHex(hexA, hexB, t) {
		const a = toRgb(hexA);
		const b = toRgb(hexB);
		const r = Math.round(a[0] + (b[0] - a[0]) * t);
		const g = Math.round(a[1] + (b[1] - a[1]) * t);
		const bl = Math.round(a[2] + (b[2] - a[2]) * t);
		return `rgb(${r}, ${g}, ${bl})`;
	}

	function toRgb(hex) {
		const h = hex.replace('#', '');
		const full = h.length === 3
			? h.split('').map((c) => c + c).join('')
			: h;
		const n = parseInt(full, 16);
		return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
	}

	let canvas;
	let paused = $state(false);

	$effect(() => {
		if (typeof window === 'undefined' || !canvas) return;
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

		const ctx = canvas.getContext('2d');
		if (!ctx) return;

		const styles = getComputedStyle(document.documentElement);
		const dotColor = styles.getPropertyValue('--line-strong').trim() || '#cabb9e';
		// A pine tint mostly cut with the page's own ground colour — close
		// enough to the field that it reads as "this patch woke up," not as
		// a distinct green shape sitting on the page. Mixed by hand rather
		// than CSS color-mix(): canvas fillStyle can't resolve var(), and
		// color-mix() support in canvas specifically still isn't universal.
		const pine = styles.getPropertyValue('--pine').trim() || '#5c7a3e';
		const field = styles.getPropertyValue('--field').trim() || '#f6f3ea';
		const activeDotColor = mixHex(pine, field, 0.7);

		let width = 0;
		let height = 0;
		let frame = null;
		const mouse = { x: -1000, y: -1000, active: false };

		function draw() {
			ctx.clearRect(0, 0, width, height);
			const offsetX = (width % spacing) / 2;
			const offsetY = (height % spacing) / 2;

			for (let x = offsetX; x <= width; x += spacing) {
				for (let y = offsetY; y <= height; y += spacing) {
					const dx = x - mouse.x;
					const dy = y - mouse.y;
					const distance = Math.sqrt(dx * dx + dy * dy);

					let radius = baseRadius;
					let color = dotColor;
					let alpha = baseAlpha;

					if (mouse.active && distance < interactionRadius) {
						const factor = 1 - distance / interactionRadius;
						radius = baseRadius + (activeRadius - baseRadius) * factor;
						color = activeDotColor;
						alpha = activeMinAlpha + (activeMaxAlpha - activeMinAlpha) * factor;
					}

					ctx.globalAlpha = alpha;
					ctx.beginPath();
					ctx.arc(x, y, radius, 0, Math.PI * 2);
					ctx.fillStyle = color;
					ctx.fill();
				}
			}
			ctx.globalAlpha = 1;
		}

		function queueDraw() {
			if (frame !== null) return;
			frame = requestAnimationFrame(() => {
				draw();
				frame = null;
			});
		}

		function resize() {
			const dpr = window.devicePixelRatio || 1;
			width = window.innerWidth;
			height = window.innerHeight;
			canvas.width = width * dpr;
			canvas.height = height * dpr;
			canvas.style.width = `${width}px`;
			canvas.style.height = `${height}px`;
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			draw();
		}

		function onMouseMove(e) {
			mouse.x = e.clientX;
			mouse.y = e.clientY;
			// The canvas sits out of the hit-testing tree, so e.target is
			// already whatever real element is under the pointer — if that's
			// a link, a button, a row, anything clickable or readable-as-a-
			// control, the glow has no business lighting up behind it.
			mouse.active = !e.target.closest?.(interactiveSelector);
			queueDraw();
		}

		// Window "mouseout" with no relatedTarget is the standard tell for
		// the pointer having actually left the viewport, not just moved
		// between two elements inside it.
		function onMouseOut(e) {
			if (e.relatedTarget) return;
			mouse.active = false;
			queueDraw();
		}

		function onVisibility() {
			paused = document.hidden;
		}

		resize();
		window.addEventListener('resize', resize);
		window.addEventListener('mousemove', onMouseMove);
		window.addEventListener('mouseout', onMouseOut);
		document.addEventListener('visibilitychange', onVisibility);

		return () => {
			window.removeEventListener('resize', resize);
			window.removeEventListener('mousemove', onMouseMove);
			window.removeEventListener('mouseout', onMouseOut);
			document.removeEventListener('visibilitychange', onVisibility);
			if (frame !== null) cancelAnimationFrame(frame);
		};
	});
</script>

<canvas class="dot-grid" class:paused bind:this={canvas} aria-hidden="true"></canvas>

<style>
	.dot-grid {
		position: fixed;
		inset: 0;
		z-index: -1;
		display: block;
		pointer-events: none;
	}

	.dot-grid.paused {
		visibility: hidden;
	}

	@media (prefers-reduced-motion: reduce) {
		.dot-grid {
			display: none;
		}
	}
</style>
