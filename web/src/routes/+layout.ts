// TidyDAV ships as a single-page app embedded in the Go binary: no server-side
// rendering, no prerendering. The Go server serves the static assets and falls
// back to index.html for client-side routes.
export const ssr = false;
export const prerender = false;
export const csr = true;
