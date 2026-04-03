# Group C — Sidebar Navigation: Design Spec

**Date:** 2026-04-03  
**Features:** Collapsible sidebar replacing top-bar navigation (#12)  
**Scope:** Frontend layout overhaul; new Settings route; all existing pages updated

---

## Goal

Replace the current top-bar navigation with a collapsible left sidebar. The result should feel like a polished SaaS app — consistent chrome, clear hierarchy, room to grow. The sidebar is the single navigation surface for the entire app.

---

## Layout Architecture

### `AppLayout` component

**File:** `frontend/src/components/AppLayout.tsx` (new)

All authenticated routes are wrapped by `AppLayout`:

```
┌─────────────────────────────────┐
│ Sidebar (240px / 56px collapsed)│  Main content area (flex-1)
│                                 │  ─────────────────────────
│  ● Expensor logo / wordmark     │  <Outlet /> or {children}
│                                 │
│  Navigation links:              │
│    Dashboard                    │
│    Transactions                 │
│    ─────────────── (divider)    │
│    Onboarding                   │
│    Settings                     │
│                                 │
│  ─────────────── (future slot)  │
│  Reports  (greyed, "Soon")      │
│  Exports  (greyed, "Soon")      │
│                                 │
│  ─────────────── (bottom)       │
│  Theme toggle (dark/light/sys)  │
│  Collapse toggle [«]            │
└─────────────────────────────────┘
```

**Collapsed state (56px wide):**
- Only icons visible; no labels.
- Logo collapses to the app icon monogram.
- Tooltips appear on hover (native `title` attribute or a lightweight tooltip).
- Theme toggle and collapse toggle remain at the bottom.

**State persistence:**
- `localStorage.getItem('sidebar_collapsed')` → `'true'` | `'false'`.
- Default: expanded.

### `App.tsx` update

Remove current `<Link>` top-bar. Wrap all routes inside `AppLayout`:

```tsx
<Route element={<AppLayout />}>
  <Route path="/" element={<Dashboard />} />
  <Route path="/transactions" element={<Transactions />} />
  <Route path="/setup" element={<Wizard />} />
  <Route path="/settings" element={<Settings />} />
</Route>
```

The `DaemonStatusBar` currently rendered at the top of each page moves into `AppLayout`'s main content header area (a slim bar at the top of the content pane, not inside the sidebar).

---

## Sidebar Component

**File:** `frontend/src/components/Sidebar.tsx` (new)

### Nav item shape

```ts
interface NavItem {
  label: string
  icon: React.ComponentType<{ className?: string }>
  href: string
  soon?: boolean
}
```

### Nav items

```ts
const NAV_ITEMS: NavItem[] = [
  { label: 'Dashboard',     icon: LayoutDashboard, href: '/' },
  { label: 'Transactions',  icon: ArrowLeftRight,  href: '/transactions' },
]

const SECONDARY_NAV: NavItem[] = [
  { label: 'Onboarding',    icon: Plug,            href: '/setup' },
  { label: 'Settings',      icon: Settings2,       href: '/settings' },
]

const FUTURE_NAV: NavItem[] = [
  { label: 'Reports',       icon: FileBarChart,    href: '/reports',  soon: true },
  { label: 'Exports',       icon: Download,        href: '/exports',  soon: true },
]
```

Icons from `lucide-react` (already a common Tailwind ecosystem dependency; add if not present).

### Active state

`useLocation()` from `react-router-dom` — active link gets `bg-accent text-accent-foreground` styling.

### Collapsed behaviour

When `collapsed = true`:
- Container `width` transitions from `240px` to `56px` via CSS `transition: width 200ms ease`.
- Labels have `opacity: 0` and `pointer-events: none` when collapsed, `opacity: 1` when expanded — also transitioned.
- Logo area shows `EX` monogram when collapsed; full "Expensor" wordmark when expanded.

---

## Settings Page

**File:** `frontend/src/pages/Settings.tsx` (new)  
**Route:** `/settings`

Settings is a tabbed page. Tabs:

| Tab | Contents | Spec |
|-----|----------|------|
| Categories | Manage category list | Group B spec |
| Buckets | Manage bucket list | Group B spec |
| Labels | Manage label taxonomy | Group B spec |
| Webhooks | Register / delete webhook URLs | Group F spec |
| Appearance | Base currency selection | see below |

### Appearance tab

A simple form:
- **Base currency** — text input (ISO 4217 code, e.g. `INR`, `USD`). Calls a new `PUT /api/config/base-currency` endpoint which updates `EXPENSOR_BASE_CURRENCY` in the running process (or persists to a `config` table — see below).

**Persisting base currency:** Rather than requiring a server restart to change `EXPENSOR_BASE_CURRENCY`, store it in a `app_config` table:

```sql
CREATE TABLE IF NOT EXISTS app_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO app_config (key, value) VALUES ('base_currency', 'INR')
ON CONFLICT (key) DO NOTHING;
```

`GetStats` reads from this table at query time (falling back to `cfg.BaseCurrency` if the row doesn't exist). This avoids restart-to-reconfigure.

---

## Removed / Migrated Elements

| Current location | New location |
|-----------------|--------------|
| `<Link to="/">Dashboard</Link>` in each page header | Sidebar nav item |
| `<Link to="/transactions">` | Sidebar nav item |
| `<DaemonStatusBar>` in each page | AppLayout content header |
| Top-bar in `Dashboard.tsx` and `Transactions.tsx` | Removed; sidebar provides navigation |

---

## Responsive Behaviour

This app is not primarily a mobile app (it's a personal finance dashboard), so:
- **≥ 768px:** sidebar renders normally (expanded or collapsed per preference).
- **< 768px:** sidebar hidden by default; a hamburger button in the content header toggles a drawer overlay.
- Drawer overlay implementation is deferred — at launch, the app is desktop-first.

---

## Files Created / Modified

| File | Change |
|------|--------|
| `frontend/src/components/AppLayout.tsx` | New |
| `frontend/src/components/Sidebar.tsx` | New |
| `frontend/src/components/ThemeToggle.tsx` | New (see Group A spec) |
| `frontend/src/pages/Settings.tsx` | New |
| `frontend/src/pages/settings/CategoriesSettings.tsx` | New (Group B) |
| `frontend/src/pages/settings/BucketsSettings.tsx` | New (Group B) |
| `frontend/src/pages/settings/LabelsSettings.tsx` | New (Group B) |
| `frontend/src/pages/settings/WebhooksSettings.tsx` | New (Group F) |
| `frontend/src/pages/settings/AppearanceSettings.tsx` | New |
| `frontend/src/App.tsx` | Replace top-bar Links with `AppLayout` wrapper |
| `frontend/src/pages/Dashboard.tsx` | Remove page-level nav links |
| `frontend/src/pages/Transactions.tsx` | Remove page-level nav links |
| `frontend/src/pages/setup/Wizard.tsx` | Remove page-level nav links |
| `backend/internal/api/handlers.go` | Add `GET/PUT /api/config/base-currency` |
| `backend/internal/store/store.go` | Add `GetAppConfig`, `SetAppConfig` |
| `backend/migrations/004_app_config.sql` | New migration for `app_config` table |
