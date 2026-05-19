# First-Run Preferences Onboarding Design

## Goal

Fresh Expensor installs must collect base currency, timezone, and time format before the user configures a reader. Existing installs should continue without interruption when those values already exist.

## Current State

The backend seeds `base_currency` as `INR` during app config initialization. Timezone and time format are stored under `app.timezone` and `app.time_format`; when they are missing, the frontend falls back to the browser timezone and `HH:mm` for display. Reader setup lives at `/setup`, and the general settings page already exposes controls for base currency, timezone, and time format.

## Backend Design

Fresh databases should no longer seed `base_currency`. `scan_interval` and `lookback_days` remain seeded because they are operational defaults, not user locale preferences.

Add a setup status API:

- `GET /api/config/setup-status`
- Response:

```json
{
  "required": true,
  "missing": ["base_currency", "timezone", "time_format"]
}
```

`required` is true when any of `base_currency`, `app.timezone`, or `app.time_format` is missing or blank. `missing` uses frontend-friendly field names: `base_currency`, `timezone`, and `time_format`.

Existing databases with non-empty values are treated as complete. Existing databases missing one or more values are guided through onboarding. This is intentionally lenient about backward compatibility while preserving the new first-run behavior.

Existing config endpoints remain responsible for validation:

- `PUT /api/config/base-currency` validates a three-letter ISO-like currency code.
- `PUT /api/config/timezone` validates an IANA timezone.
- `PUT /api/config/time-format` validates the supported display formats.

## Frontend Design

Add a preferences step to `/setup`. When setup status is incomplete, `/setup` shows this step before reader selection. Saving writes the three values, refreshes setup status, and advances to reader selection.

When the user navigates directly to `/setup?step=guide&reader=gmail` or another reader-focused setup state while preferences are incomplete, the wizard still shows the preferences step first. The page should explain that these display preferences are required before connecting a reader.

The preferences step should reuse the existing visual patterns from `GeneralSettings`: custom currency combobox, timezone combobox, and time format dropdown. It must not introduce native `<select>`, `<datalist>`, `alert`, `confirm`, or `prompt`.

The settings page may continue to use browser/display fallbacks while values are unset, but the setup step should make the required action explicit before reader configuration.

## Data Flow

1. `/setup` loads setup status.
2. If setup is required, the wizard renders the preferences step.
3. The user saves base currency, timezone, and time format.
4. The frontend calls the existing config update endpoints.
5. The setup status query is invalidated.
6. Reader selection and existing reader setup steps become available.

## Testing

Backend tests should cover:

- Fresh or blank config reports setup as required with the expected missing fields.
- Config with all three preference values reports setup as complete.
- Fresh app config initialization does not seed `base_currency`.

Frontend tests should cover:

- `/setup` shows the preferences step instead of reader selection when setup status is incomplete.
- Saving preferences advances to reader configuration.
- Reader-focused setup URLs are gated by the preferences step while setup status is incomplete.

## Out of Scope

This change does not block every reader API endpoint at the backend layer. The user-facing app flow is gated first; stricter backend enforcement can be added later if direct API misuse becomes a practical problem.
