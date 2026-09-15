# WhatsApp Analytics Implementation Plan

**Goal:** Deliver the approved Vue 3 JavaScript / Go Gin / PostgreSQL redirect and advertising analytics application.
**Architecture:** The Go redirect endpoint persists events before 302; authenticated APIs serve the independent Vue admin. Daily unique counts never sum into period uniques.
**Constraints:** No GitHub commits. No fake analytics. WhatsApp destinations only. Default cookies disabled until operator selects an appropriate collection policy. Raw IP is not persisted. No claim of confirmed human or WhatsApp conversion.

- [x] Backend tests: target validation, cookie reuse, bot/HEAD/prefetch exclusion, cross-day UV, auth/CSRF, attribution conflict, spend validation, database fail-open.
- [x] Backend service: migrations, secure sessions, redirect collection, regional database integration, daily retention, analytics, CSV and audit.
- [x] Frontend: login, live overview, links editor, event table, advertising costs and settings. Independent delegated task.
- [x] Deployment: Compose, HTTPS ingress example, environment template, backup and restore instructions.
- [x] Verify: Go tests with real isolated PostgreSQL, race tests, frontend production build, browser smoke and bounded load test. Document limitations and setup.
