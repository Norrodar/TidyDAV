import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    // Build a single-page app and emit it where the Go binary embeds it
    // (internal/ui/dist via //go:embed all:dist).
    adapter: adapter({
      pages: '../internal/ui/dist',
      assets: '../internal/ui/dist',
      fallback: 'index.html',
      precompress: false,
      strict: true
    }),
    // Relative asset paths so the embedded app works when served at the root.
    paths: { relative: true }
  }
};

export default config;
