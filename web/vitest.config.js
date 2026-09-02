import { defineConfig } from 'vitest/config';

// Deliberately not the config in vite.config.js: these tests cover plain
// JavaScript modules, so loading the SvelteKit plugin would pull in its whole
// route-and-manifest pipeline to run assertions that never touch a component.
//
// When component tests arrive they need that plugin plus a DOM environment, at
// which point this grows a second `projects` entry rather than switching over.
export default defineConfig({
	test: {
		include: ['src/**/*.test.js'],
		environment: 'node'
	}
});
