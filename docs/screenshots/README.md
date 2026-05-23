# Screenshots

Use this directory for screenshot assets and reusable anonymized seed data.

Current screenshots:

- `transactions-light.png` - Transactions page, light theme.
- `transactions-dark.png` - Transactions page, dark theme.
- `dashboard-light.png` - Dashboard, light theme.
- `dashboard-dark.png` - Dashboard, dark theme.

The README hero uses `transactions-light.png`.

Legacy mocked dashboard fixture:

```bash
task screenshots:readme
```

Live high-resolution dashboard and transactions captures:

```bash
task screenshots:live
```

Keep screenshot data deterministic so screenshots can be regenerated when the UI changes.
