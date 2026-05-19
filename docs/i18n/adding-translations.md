# Adding Language Support

Expensor has a lightweight i18n foundation. English is the source locale, and translated strings are stored in the typed catalog at `frontend/src/i18n/messages.ts`.

Only strings that have been moved into the catalog can be translated today. Page-specific copy may still be inline; move it into the catalog when touching that page. See [String Extraction Rules](string-extraction.md) for extraction boundaries.

## Add A Locale

1. Pick a BCP 47 locale code such as `hi-IN`, `fr-FR`, or `de-DE`.
2. Open `frontend/src/i18n/messages.ts`.
3. Copy the full `en` catalog into a new top-level locale entry.
4. Translate values only. Do not rename message keys.
5. Run `task lint:fe` to let TypeScript catch missing or extra keys.
6. Run `task test:fe` to verify the frontend still renders.

Example:

```ts
export const messages = {
  en: {
    'nav.dashboard': 'Dashboard',
    'command.search': 'Search destinations',
  },
  'hi-IN': {
    'nav.dashboard': 'Dashboard',
    'command.search': 'Search destinations',
  },
} as const
```

Keep untranslated values in English until a better translation is available. Partial locale objects are not allowed because missing keys would surface as broken UI.

## Add A Translatable String

1. Add a semantic key to `messages.en`, for example `transactions.search.placeholder`.
2. Add the same key to every other locale.
3. Use `const { t } = useI18n()` in the component.
4. Render `t('transactions.search.placeholder')` instead of inline text.
5. Add or update a focused test if the string is part of behavior, accessibility labels, navigation, or routing.

Use complete messages for dynamic text. Do not concatenate translated fragments around variables. Prefer a small helper or a complete keyed message for each sentence shape.

## Formatting

Use helpers from `frontend/src/i18n/format.ts` for user-facing currency, dates, month labels, and counts:

```ts
formatCurrencyForLocale(amount, currency, locale)
formatDateForLocale(date, locale)
formatNumberForLocale(count, locale)
formatMonthForLocale(date, locale)
```

Inside React components, read the active locale from `useI18n()`:

```tsx
const { locale, t } = useI18n()
```

The default app locale is currently `en`. Adding a new locale prepares the catalog; a separate UI/config change is needed to let users choose it at runtime.

## Review Checklist

- `frontend/src/i18n/messages.ts` has the same keys for every locale.
- New shared UI strings use `MessageKey` and `t(...)`.
- User-facing dates, currency, month labels, and counts use i18n format helpers.
- `task lint:fe` passes.
- `task test:fe` passes.
