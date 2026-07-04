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

Seeded review stack:

```bash
task screenshots:review
```

This recreates the component backend, applies the deterministic seed data, builds
the frontend, and serves the review app at `http://127.0.0.1:4173`. Sign in with
`john.smith@example.com` and `component admin password`, then review the
Dashboard and Transactions pages before capturing. Stop the preview with
`Ctrl-C`; if the component containers are still running, clean them up with:

```bash
docker compose -f tests/component/docker-compose.yml down -v --remove-orphans
```

Seeded high-resolution dashboard and transactions captures:

```bash
task screenshots:live
```

This uses the same component seed and preview setup as `screenshots:review`,
signs in with the component admin fixture, and writes all four screenshot files.
Dashboard screenshots are captured in all-time mode so the deterministic seed
data does not need timestamp adjustments as calendar months pass.

Keep screenshot data deterministic so screenshots can be regenerated when the UI changes.
