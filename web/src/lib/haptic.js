const isTouchDevice =
	typeof window !== 'undefined' ? window.matchMedia('(pointer: coarse)').matches : false;

/**
 * Trigger haptic feedback on mobile devices.
 * Uses the Vibration API on Android/modern browsers, and the iOS checkbox
 * "switch" trick on iOS, which has no Vibration API but does give haptic
 * feedback for a native checkbox toggle.
 *
 * @param {number | number[]} pattern - Vibration duration (ms) or pattern.
 * Custom patterns only work on Android devices; iOS uses fixed feedback.
 */
export function haptic(pattern = 50) {
	try {
		if (!isTouchDevice) return;

		if ('vibrate' in navigator) {
			navigator.vibrate(pattern);
			return;
		}

		const label = document.createElement('label');
		label.ariaHidden = 'true';
		label.style.display = 'none';

		const input = document.createElement('input');
		input.type = 'checkbox';
		input.setAttribute('switch', '');
		label.appendChild(input);

		try {
			document.head.appendChild(label);
			label.click();
		} finally {
			document.head.removeChild(label);
		}
	} catch {}
}
