# i18n String Rules

For adding a new locale or translating existing catalog entries, see [Adding Language Support](adding-translations.md).

- Shared navigation, command, settings tab, and repeated control labels use `MessageKey`.
- Page-specific one-off copy may remain inline until the page is touched for feature work.
- Dates, times, currency, and counts use helpers from `frontend/src/i18n/format.ts`.
- Do not concatenate translated fragments around dynamic values. Add a complete message helper instead.

Review command:

```bash
rg -n "toLocaleString\\('en|toLocaleDateString\\('en|Intl\\." frontend/src
```
