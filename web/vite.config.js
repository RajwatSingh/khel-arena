import { sveltekit } from '@sveltejs/kit/vite';

export default {
	plugins: [sveltekit()],
	server: {
		// The Go service (cmd/api, see apiPlan.md) when it exists. Until then the
		// mock in src/lib/api answers instead and nothing reaches this proxy.
		proxy: {
			'/v1': {
				target: process.env.API_ORIGIN ?? 'http://localhost:8080',
				changeOrigin: true
			}
		}
	}
};
