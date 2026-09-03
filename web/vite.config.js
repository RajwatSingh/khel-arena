import { sveltekit } from '@sveltejs/kit/vite';

export default {
	plugins: [sveltekit()],
	server: {
		// The Go service (cmd/api). Two path prefixes belong to it:
		//   /v1     — the JSON API
		//   /media  — uploaded arena photos, served from disk by the API
		//             (MEDIA_URL_PREFIX in .env). Without this second entry
		//             every uploaded image resolves against the dev server
		//             instead and 404s — a photo saved as /media/<hash>.png
		//             only exists behind :8080.
		proxy: {
			'/v1': {
				target: process.env.API_ORIGIN ?? 'http://localhost:8080',
				changeOrigin: true
			},
			'/media': {
				target: process.env.API_ORIGIN ?? 'http://localhost:8080',
				changeOrigin: true
			}
		}
	}
};
