# Group B — Transaction Enrichment: Design Spec

**Date:** 2026-04-03  
**Features:** Label taxonomy (#6), Editable category/bucket (#10)  
**Scope:** New config tables + API endpoints + inline editing in Transactions UI

---

## 1. Label Taxonomy (#6)

### Goal
Move labels from an implicit, free-form system (any string can be a label) to an explicit managed taxonomy. Users pick labels from a curated list; new labels are created through a "Create new" flow. Labels can also be applied in bulk to all transactions matching a merchant pattern.

### Data Model

**Migration:** `backend/migrations/002_labels_taxonomy.sql`

```sql
CREATE TABLE IF NOT EXISTS labels (
    name        TEXT PRIMARY KEY,
    color       TEXT NOT NULL DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed defaults
INSERT INTO labels (name, color) VALUES
    ('food',        '#f59e0b'),
    ('transport',   '#3b82f6'),
    ('shopping',    '#8b5cf6'),
    ('utilities',   '#06b6d4'),
    ('healthcare',  '#10b981'),
    ('entertainment','#ec4899'),
    ('travel',      '#f97316'),
    ('recurring',   '#6366f1')
ON CONFLICT (name) DO NOTHING;
```

The existing `transaction_labels.label` column remains a `TEXT` FK-less reference. No FK added — this keeps the existing data intact and avoids cascading complications. Referential integrity enforced at the application layer.

### API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/config/labels` | List all labels (name + color) |
| POST | `/api/config/labels` | Create a new label `{name, color?}` |
| PUT | `/api/config/labels/{name}` | Update color |
| DELETE | `/api/config/labels/{name}` | Delete label (does not remove from transactions) |
| POST | `/api/config/labels/{name}/apply` | Bulk-apply label to all transactions where `merchant_info ILIKE $pattern` |

**`POST /api/config/labels/{name}/apply` body:**
```json
{ "merchant_pattern": "swiggy" }
```
Runs `INSERT INTO transaction_labels SELECT id, $label FROM transactions WHERE merchant_info ILIKE '%' || $pattern || '%' ON CONFLICT DO NOTHING`.

### Store Layer

**File:** `backend/internal/store/store.go`

New methods:
```go
ListLabels(ctx context.Context) ([]Label, error)
CreateLabel(ctx context.Context, name, color string) error
UpdateLabel(ctx context.Context, name, color string) error
DeleteLabel(ctx context.Context, name string) error
ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) // returns rows affected
```

```go
type Label struct {
    Name      string    `json:"name"`
    Color     string    `json:"color"`
    CreatedAt time.Time `json:"created_at"`
}
```

### Frontend

**Transactions table — label addition flow:**
- The existing "+ label" button opens a `<Combobox>` (searchable dropdown) populated from `GET /api/config/labels`.
- At the bottom of the dropdown: "+ Create new label" → inline form for name + color picker (a palette of 8 preset colors).
- Selected label is passed to the existing `POST /api/transactions/{id}/labels`.

**Settings page — Labels tab:**
- Table of all labels with color swatch, name, edit (color only) and delete actions.
- "+ New label" button at top.
- Each row has "Apply to transactions" action: opens a modal with a merchant pattern input, previews match count, confirms bulk apply.

**Files:**
- `frontend/src/components/LabelCombobox.tsx` (new — replaces the current inline label input)
- `frontend/src/pages/settings/LabelsSettings.tsx` (new)
- `frontend/src/api/queries.ts` — add label config query hooks

---

## 2. Category & Bucket Editing (#10)

### Goal
Let users correct or override the category and bucket assigned to a transaction directly from the Transactions table. Both fields are constrained to user-managed lists with seeded defaults.

### Data Model

**Migration:** `backend/migrations/003_categories_buckets.sql`

```sql
CREATE TABLE IF NOT EXISTS categories (
    name        TEXT PRIMARY KEY,
    description TEXT,
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS buckets (
    name        TEXT PRIMARY KEY,
    description TEXT,
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed defaults derived from existing rules.json categories
INSERT INTO categories (name, is_default) VALUES
    ('food & dining',   true),
    ('transport',       true),
    ('shopping',        true),
    ('utilities',       true),
    ('healthcare',      true),
    ('entertainment',   true),
    ('travel',          true),
    ('finance',         true),
    ('uncategorized',   true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO buckets (name, is_default) VALUES
    ('needs',   true),
    ('wants',   true),
    ('savings', true),
    ('income',  true)
ON CONFLICT (name) DO NOTHING;
```

### API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/config/categories` | List all categories |
| POST | `/api/config/categories` | Create category `{name, description?}` |
| DELETE | `/api/config/categories/{name}` | Delete (reject if `is_default=true`) |
| GET | `/api/config/buckets` | List all buckets |
| POST | `/api/config/buckets` | Create bucket `{name, description?}` |
| DELETE | `/api/config/buckets/{name}` | Delete (reject if `is_default=true`) |
| PUT | `/api/transactions/{id}` | Extended to accept `{category?, bucket?, description?}` |

**`PUT /api/transactions/{id}` update:**
The existing endpoint only accepts `description`. Extend body to:
```json
{
  "description": "optional",
  "category": "optional — must exist in categories table",
  "bucket": "optional — must exist in buckets table"
}
```
Backend validates against the config tables before writing. Returns 422 if value not in list.

### Store Layer

**File:** `backend/internal/store/store.go`

New methods:
```go
ListCategories(ctx context.Context) ([]Category, error)
CreateCategory(ctx context.Context, name, description string) error
DeleteCategory(ctx context.Context, name string) error
ListBuckets(ctx context.Context) ([]Bucket, error)
CreateBucket(ctx context.Context, name, description string) error
DeleteBucket(ctx context.Context, name string) error
UpdateTransaction(ctx context.Context, id string, update TransactionUpdate) error
```

```go
type Category struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    IsDefault   bool   `json:"is_default"`
}

type Bucket struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    IsDefault   bool   `json:"is_default"`
}

type TransactionUpdate struct {
    Description *string
    Category    *string
    Bucket      *string
}
```

The existing `UpdateDescription` method is kept for backwards compatibility; `UpdateTransaction` supersedes it for new callers.

### Frontend

**Transactions table — inline editing:**
- Category and Bucket cells render as plain text by default.
- On click/focus: the cell becomes a `<Select>` dropdown populated from the respective config endpoint.
- Selecting a value calls `PUT /api/transactions/{id}` with the new value.
- Escape cancels without saving; selecting the same value is a no-op.
- Uses the same edit-on-click pattern as the existing Description cell.

**Settings page — Categories tab and Buckets tab:**
- Two tabs in Settings (alongside Labels).
- Each shows a table: name, description, default badge, delete button (disabled for defaults).
- "+ New" button at top of each tab.

**Files:**
- `frontend/src/components/InlineSelect.tsx` (new — reusable inline select for category/bucket cells)
- `frontend/src/pages/settings/CategoriesSettings.tsx` (new)
- `frontend/src/pages/settings/BucketsSettings.tsx` (new)
- `frontend/src/api/queries.ts` — add config query hooks for categories and buckets

---

## Error Handling

- Attempting to delete a default category/bucket returns `409 Conflict` with a descriptive error.
- Setting a transaction's category/bucket to a value not in the config table returns `422 Unprocessable Entity`.
- Deleting a label that is currently applied to transactions is allowed (soft delete — transactions retain the label string, it just no longer appears in the managed list). A warning is shown in the UI.

## Migrations Strategy

Migrations run via the existing `RunMigrations` mechanism in `backend/pkg/writer/postgres/postgres.go`. The embedded `*.sql` approach is extended: migrations are numbered (`001_`, `002_`, `003_`) and run idempotently using `IF NOT EXISTS` and `ON CONFLICT DO NOTHING`.
