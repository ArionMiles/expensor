# Starlight Documentation Site — Design Spec (Deferred)

**Date:** 2026-04-04
**Status:** DEFERRED — implement after project is ready for broad public sharing

---

## Goal

Host project documentation at a dedicated URL using [Starlight](https://starlight.astro.build/) (Astro-based static site generator). Gives the project a polished, searchable, version-aware documentation site.

## Scope (when the time comes)

- Deployed via GitHub Pages or Cloudflare Pages on push to `main`
- Content: Getting Started, Configuration reference (env vars, rules.json format), API reference, Architecture overview, Changelog
- Versioning: single version (latest) until the API stabilises
- Search: Starlight's built-in Pagefind search

## Non-goals

- Self-hosted docs server
- Per-release versioned docs (defer until API stable)

## Trigger condition

This spec becomes active when:
- All critical feature PRs are merged
- The author is ready to share the project publicly

---

*No implementation plan has been written. Create one when this spec is activated.*
