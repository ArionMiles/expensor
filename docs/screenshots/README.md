# Screenshot Fixtures

Use this directory for screenshot assets and reusable anonymized seed data.

Dashboard screenshot:

- Page: Dashboard
- Theme: light
- Sidebar: expanded
- Data: `dashboard-seed.json`
- Output: `dashboard-light.png`

Regenerate:

```bash
task screenshots:readme
```

Keep seed data deterministic so screenshots can be regenerated when the UI changes.
