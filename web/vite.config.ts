import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    // During `npm run dev`, proxy backend routes to the Go API on :8080 so the
    // SvelteKit dev server gives hot reload for the frontend only.
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/ics': 'http://localhost:8080',
      '/health': 'http://localhost:8080'
    }
  },
  test: {
    include: ['src/**/*.{test,spec}.{js,ts}'],
    environment: 'node'
  }
});
