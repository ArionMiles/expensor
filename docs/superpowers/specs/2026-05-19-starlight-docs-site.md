# Starlight Documentation Site — Design Spec (Deferred)

**Date:** 2026-05-19
**Status:** DEFERRED — implement after project is ready for broad public sharing

---

## Goal

Host project documentation at a dedicated URL using [Starlight](https://starlight.astro.build/) (Astro-based static site generator). Gives the project a polished, searchable, version-aware documentation site.

## Scope (when the time comes)

- Deployed via GitHub Pages or Cloudflare Pages on push to `main`
- Content: Getting Started, Configuration reference (env vars, rules.json format), API reference, Architecture overview, Changelog
- Versioning: single version (latest) until the API stabilises
- Search: Starlight's built-in Pagefind search

## API reference

By the time this spec is activated, `openapi.yaml` will already exist in the repo root (added as part of the Webhooks feature). The Starlight site should import it using the [Scalar Astro integration](https://github.com/scalar/scalar/tree/main/packages/astro-api-reference) to render an interactive API reference page. The embedded `/docs` route in the binary can be removed at that point, or kept as a convenience for self-hosters who don't visit the docs site.

## Non-goals

- Self-hosted docs server
- Per-release versioned docs (defer until API stable)

## Trigger condition

This spec becomes active when:
- All critical feature PRs are merged
- The author is ready to share the project publicly

---

*No implementation plan has been written. Create one when this spec is activated.*
