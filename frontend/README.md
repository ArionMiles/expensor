# Expensor Frontend

React 18 + Vite + TypeScript + Tailwind CSS.

---

## Design Language

### Color System

All colors are defined as HSL CSS variables in `src/index.css`. Never use raw hex or RGB values — always reference semantic variables through Tailwind classes.

```
--background:          222.2 84% 4.9%     Very dark blue-black. Page background.
--foreground:          210 40% 98%        Off-white. Primary text.

--card:                222.2 84% 4.9%     Same as background. Card/panel surfaces.
--card-foreground:     210 40% 98%        Text inside cards.

--primary:             217.2 91.2% 59.8%  Medium blue. CTAs, links, active states, amounts.
--primary-foreground:  222.2 47.4% 11.2%  Dark text on primary backgrounds.

--secondary:           217.2 32.6% 17.5%  Dark blue-gray. Input backgrounds, table headers.
--secondary-foreground:210 40% 98%

--muted:               217.2 32.6% 17.5%  Same as secondary. Subtle fills.
--muted-foreground:    215 20.2% 65.1%    Gray-blue. Secondary text, placeholders, labels.

--accent:              217.2 32.6% 17.5%  Hover state fills.
--accent-foreground:   210 40% 98%

--border:              217.2 32.6% 17.5%  All borders.
--input:               217.2 32.6% 17.5%  Input backgrounds.
--ring:                224.3 76.3% 48%    Focus rings.

--destructive:         0 62.8% 30.6%      Errors, delete actions.
--destructive-foreground: 210 40% 98%

--success:             142.1 70.6% 45.3%  Positive states, connected status, start daemon.
--success-foreground:  144.9 80.4% 10%

--warning:             32.1 94.6% 43.7%   Pending states, in-progress operations.
--warning-foreground:  20.9 91.7% 14.1%

--radius:              0.5rem             All border radii derive from this.
```

### Tailwind Color Classes

Use semantic Tailwind classes — never `text-[var(--something)]`.

| Purpose | Class |
|---|---|
| Page background | `bg-background` |
| Card / panel fill | `bg-card` |
| Input / secondary fill | `bg-secondary` |
| Hover fill | `bg-accent` / `hover:bg-accent` |
| All borders | `border-border` |
| Primary text | `text-foreground` |
| Secondary / muted text | `text-muted-foreground` |
| CTA, links, amounts | `text-primary` / `bg-primary` |
| Text on primary bg | `text-primary-foreground` |
| Errors | `text-destructive` / `border-destructive` |
| Success states | `text-success` / `bg-success` / `border-success` |
| Pending / warning | `text-warning` / `bg-warning` |
| Focus ring | `focus:ring-1 focus:ring-ring` |

### Typography

System font stack — no custom typefaces.

```
font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif
```

- **Base size**: 16px root. `text-sm` (14px) for table content and dense UI areas.
- **Labels / metadata**: `text-xs` (12px)
- **Micro labels**: `text-[10px]` sparingly
- **Weights**: `font-medium` for section labels, `font-semibold` for headings, default (400) for body
- **Monospace** (`font-mono`): amounts, IDs, dates, code references — not for UI labels or nav

### Spacing & Radius

- **Border radius**: All elements use `rounded-sm`, `rounded-md`, or `rounded-lg` — derived from `--radius: 0.5rem`. No sharp corners.
- **Inputs / buttons**: `rounded-md` (`calc(0.5rem - 2px)` ≈ 6px)
- **Cards / panels**: `rounded-lg` (`0.5rem` = 8px)
- **Spacing**: Tailwind scale. Common values: `p-4`, `p-6`, `px-3 py-2`, `gap-2`, `gap-4`.

### Shadows

- Cards and panels: `shadow-sm`
- No heavy shadows or glow effects

### Never Use Native Browser Controls

This project has a custom dark-themed design language. Native browser controls break it and must never be used:

| ❌ Never use | ✅ Use instead |
|---|---|
| `<select>` | `InlineSelect` from `@/components/InlineSelect` |
| `<datalist>` | Custom combobox with styled `<ul>` dropdown (see `SourceCombobox` in `pages/rules/RuleForm.tsx`) |
| `confirm()` / `alert()` / `prompt()` | `ConfirmModal` from `@/components/ConfirmModal` |
| `title="..."` attribute (browser tooltip) | CSS `group-hover:block` div or `position:fixed` with `onMouseEnter` state |

#### Custom combobox pattern (for free-text + suggestions)

```tsx
function MyCombobox({ value, onChange, options }: { value: string; onChange: (v: string) => void; options: string[] }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const filtered = options.filter((o) => o.toLowerCase().includes(value.toLowerCase()))

  return (
    <div ref={ref} className="relative">
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => setOpen(true)}
        className="w-full rounded border border-border bg-input px-2 py-1.5 text-sm"
      />
      {open && filtered.length > 0 && (
        <ul className="absolute left-0 top-full z-50 mt-0.5 w-full overflow-y-auto rounded-md border border-border bg-card shadow-lg">
          {filtered.map((opt) => (
            <li key={opt} onMouseDown={() => { onChange(opt); setOpen(false) }}
              className="cursor-pointer px-3 py-1.5 text-sm text-foreground hover:bg-accent">
              {opt}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
```

#### Dropdown overflow in tables

Dropdowns inside `overflow-x-auto` containers must use `position: fixed` to escape clipping:

```tsx
// Capture position on open:
const rect = containerRef.current?.getBoundingClientRect()
setDropdownPos({ top: rect.bottom + 2, left: rect.left })

// Render with fixed position:
<ul style={{ position: 'fixed', top: dropdownPos.top, left: dropdownPos.left }}
  className="z-50 min-w-[140px] rounded-md border border-border bg-card shadow-lg">
```

See `LabelCombobox.tsx` and `InlineSelect.tsx` for complete implementations.

#### Disabled elements and hover events

Disabled form elements do not fire `mouseenter`/`mouseleave` in browsers. Wrap them in a `<span>` to receive hover events:

```tsx
<span onMouseEnter={showTooltip} onMouseLeave={hideTooltip}>
  <button disabled className="pointer-events-none opacity-30">Action</button>
</span>
```

### Component Patterns

#### Cards / Panels
```tsx
<div className="rounded-lg border border-border bg-card p-4 shadow-sm">
```

#### Primary Button
```tsx
<button className="px-4 py-2 text-sm rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors">
```

#### Ghost / Secondary Button
```tsx
<button className="px-4 py-2 text-sm rounded-md border border-border text-muted-foreground hover:text-foreground hover:bg-accent transition-colors">
```

#### Destructive (hover-reveal) Button
```tsx
<button className="px-3 py-1.5 text-xs rounded-md border border-border text-muted-foreground hover:border-destructive hover:text-destructive transition-colors">
```

#### Text Input
```tsx
<input className="w-full px-3 py-2 text-sm rounded-md bg-secondary border border-border text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring focus:border-primary" />
```

#### Status Badge (inline)
```tsx
// connected
<span className="text-[10px] px-1.5 py-0.5 rounded-sm border border-success/50 text-success bg-success/10">
  ● Connected
</span>

// warning
<span className="text-[10px] px-1.5 py-0.5 rounded-sm border border-warning/50 text-warning bg-warning/10">
  ○ Auth required
</span>

// neutral
<span className="text-[10px] px-1.5 py-0.5 rounded-sm border border-border text-muted-foreground">
  ○ Not configured
</span>
```

#### Table
```tsx
<div className="rounded-lg border border-border overflow-x-auto bg-card shadow-sm">
  <table>
    <thead>
      <tr className="border-b border-border bg-secondary/50">
        <th className="px-3 py-2.5 text-left text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
          Column
        </th>
      </tr>
    </thead>
    <tbody>
      <tr className="border-b border-border hover:bg-accent/50 transition-colors">
        <td className="px-3 py-2.5 text-sm text-foreground">...</td>
      </tr>
    </tbody>
  </table>
</div>
```

#### Loading Skeleton
```tsx
<div className="h-3 w-32 bg-secondary rounded animate-pulse" />
```

### Amounts

Currency amounts are always:
- `font-mono` — tabular numerals in a monospace face
- `text-primary` — primary blue
- `tabular-nums` — prevents layout shift as digits change

```tsx
<span className="font-mono text-primary tabular-nums">
  {formatCurrency(amount, currency)}
</span>
```

### Page Layout

All pages share the same layout structure:

```tsx
<div className="min-h-screen bg-background flex flex-col">
  <header className="border-b border-border px-6 py-3 flex items-center justify-between bg-card">
    {/* Logo + Nav */}
  </header>
  {/* optional: <DaemonStatusBar /> */}
  <main className="flex-1 px-6 py-6 max-w-6xl mx-auto w-full">
    {/* content */}
  </main>
</div>
```

### Navigation

- Logo: `text-sm font-semibold text-primary`
- Active nav link: `text-foreground font-medium`
- Inactive nav link: `text-muted-foreground hover:text-foreground`

---

## Project Structure

```
src/
├── api/
│   ├── client.ts       Axios instance + all API methods
│   ├── queries.ts      TanStack Query hooks (useQuery / useMutation)
│   └── types.ts        TypeScript interfaces for all API responses
├── components/
│   ├── DaemonStatusBar.tsx
│   ├── ErrorBoundary.tsx
│   ├── LabelChip.tsx
│   ├── Pagination.tsx
│   └── StatusBadge.tsx
├── lib/
│   └── utils.ts        cn(), formatCurrency(), formatDate(), getLabelColor(), etc.
├── pages/
│   ├── Dashboard.tsx
│   ├── Transactions.tsx
│   └── setup/
│       ├── Wizard.tsx          Overview + step-wizard shell
│       └── steps/
│           ├── SelectReader.tsx
│           ├── UploadCredentials.tsx
│           ├── OAuthStep.tsx
│           ├── ConfigureStep.tsx
│           └── ReviewAndStart.tsx
├── App.tsx
└── index.css           CSS variables + base styles
```

## Development

```bash
# From repo root
task run:frontend

# Or directly
cd frontend && npm run dev
```

Requires the backend to be running at `localhost:8080`. Use `task dev` to start postgres, backend, and frontend together.
