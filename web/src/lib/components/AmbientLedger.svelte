<script>
	/**
	 * The signature: the site's own hold/release mechanism, run at whisper
	 * scale behind everything. Purely decorative — CSS does the animating,
	 * this only pauses it when the tab is backgrounded so an unwatched loop
	 * never spends CPU.
	 */
	let paused = $state(false);

	$effect(() => {
		function onVisibility() {
			paused = document.hidden;
		}
		document.addEventListener('visibilitychange', onVisibility);
		return () => document.removeEventListener('visibilitychange', onVisibility);
	});
</script>

<div class="ambient-ledger" class:paused aria-hidden="true">
	<span></span>
	<span></span>
	<span></span>
</div>
