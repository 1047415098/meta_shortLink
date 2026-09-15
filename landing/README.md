# Public landing application

Independent Vue 3 / Vue Router application. Page files use `template`, `script setup`, then `style scoped`. The public application has no admin UI dependencies and no page-local component directory.

- `src/views/LandingView.vue`: existing research catalog and consultation form.
- `src/views/UnavailableView.vue`: unavailable or missing bootstrap data.
- `src/bootstrap.js`: reads the Go-generated `#landing-data` once per document.
- `src/lib/contact.js`: countdown lifecycle and native POST classification.

Run `npm ci`, `npm test`, then `npm run build`. The build emits `dist/index.html`, JS/CSS with gzip and Brotli variants under `dist/landing-assets/`, and catalog images under `dist/landing-assets/images/`.

Set the Go server's `LANDING_DIR` to this `dist` directory. It replaces `<!--LANDING_BOOTSTRAP-->` with a safe JSON script containing `{link, ticket, cookie_enabled, error?}`. Link fields use the existing API's snake_case names. Public HTML must come from Go so the original request is recorded exactly once; the Vue app does not fetch the short link again. A standalone Vite page without bootstrap shows the unavailable view.

Consultation remains a native `POST /:code/contact` with the signed ticket and a `manual` or `auto` trigger. Zero delay disables automatic submission. Timers start on mount and stop on submit, page hide, or unmount. Back-cache restoration and history reload suppress further automatic submission while keeping manual consultation available.
