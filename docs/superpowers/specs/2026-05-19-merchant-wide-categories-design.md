# Merchant-Wide Category/Bucket Application — Design Spec

**Date:** 2026-05-19
**Status:** Complete
**Scope:** Auto-prompt to apply category/bucket changes across all transactions from the same merchant, with future-scan persistence

---

## Problem

When a user corrects the category or bucket on a transaction, the change applies only to that single row. All other transactions from the same merchant retain the old (wrong) category. The user must edit each transaction individually. There is also no way to set a persistent category rule for a merchant so that future scans pick it up automatically.

---

## Solution Overview

After a successful inline category or bucket edit on a transaction, a `SlideNotification` appears offering to apply that category/bucket to all transactions from the same merchant. On confirmation, a single backend endpoint atomically:

1. Bulk-updates all existing transactions with matching `merchant_info`
2. Upserts a `user_locked = true` entry into `merchant_categories` so future scans categorize new transactions from that merchant automatically

---

## Backend

### New `Storer` method

```go
CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (rowsUpdated int, err error)
```

Runs two statements inside a single DB transaction:

```sql
-- 1. Update existing transactions
UPDATE transactions
SET category = $2, bucket = $3
WHERE merchant_info = $1;

-- 2. Persist for future scans (user_locked prevents community sync from overwriting)
INSERT INTO merchant_categories (fragment, category, bucket, user_locked)
VALUES ($1, $2, $3, true)
ON CONFLICT (fragment) DO UPDATE
SET category    = EXCLUDED.category,
    bucket      = EXCLUDED.bucket,
    user_locked = true;
```

Returns the number of transaction rows updated. If either statement fails, the transaction rolls back.

### New handler: `POST /api/merchants/categorize`

**Request body:**
```json
{
  "merchant":  "Netflix",
  "category":  "Entertainment",
  "bucket":    "Wants"
}
```

**Validation:** `merchant` must be non-empty. `category` and `bucket` may be empty strings (allows bulk-clearing).

**Response:** `{"updated": N}` where N is the count of transaction rows updated.

**Registration:** alongside existing muted-merchants routes in `registerRoutes`.

**Compile-time check:** add `CategorizeMerchant` to the `Storer` interface in `internal/api/store.go`. The existing `var _ Storer = (*store.Store)(nil)` assertion catches mismatches. Add the corresponding no-op method to `mockStore` in `handlers_test.go`.

---

## Frontend

### Trigger

The inline category and bucket editors in `Transactions.tsx` already call `updateFields({ id, patch })` on commit. After a successful mutation, check if the transaction has a non-empty `merchant_info`. If so, set a pending prompt state:

```tsx
const [pendingMerchantCat, setPendingMerchantCat] = useState<{
  merchant: string
  category: string
  bucket: string
} | null>(null)
```

Category edit:
```tsx
onCommit={(category) =>
  updateFields({ id: tx.id, patch: { category } }, {
    onSuccess: () => {
      if (tx.merchant_info) {
        setPendingMerchantCat({ merchant: tx.merchant_info, category, bucket: tx.bucket })
      }
    },
  })
}
```

Bucket edit follows the same pattern, using the current `tx.category` value.

If both category and bucket are changed in quick succession, the second change overwrites `pendingMerchantCat` — the prompt always reflects the latest committed values.

### `SlideNotification` content

```
Apply "[category]" to all [merchant] transactions?    [Apply]  [Dismiss]
```

If category is empty (clearing): `Apply "" to all [merchant] transactions?` — still valid.

**On Apply:**
1. Call `POST /api/merchants/categorize` with `{ merchant, category, bucket }`
2. On success: `queryClient.invalidateQueries(['transactions'])` to refresh the list
3. Clear `pendingMerchantCat`

**On Dismiss or auto-timeout:** clear `pendingMerchantCat`, no further action.

### Edge cases

| Scenario | Behavior |
|----------|----------|
| `tx.merchant_info` is empty | Skip prompt — no merchant to apply to |
| User dismisses prompt | Change applied to single transaction only; `merchant_categories` not updated |
| Second edit while prompt is visible | `pendingMerchantCat` is replaced; prompt updates to reflect new values |
| `POST /api/merchants/categorize` fails | Show inline error in notification; transaction list not invalidated |
| Merchant has only one transaction | Prompt still fires; applying is a no-op for existing rows but sets the `merchant_categories` rule for future scans |

---

## What This Does Not Cover

- Applying labels merchant-wide (labels are many-to-many; a separate design would be needed)
- Explicit "categorize all from merchant" menu item (can be added later as a complement)
- Undo after applying merchant-wide (use the individual inline editor to revert)
- Partial merchant matching (the update uses exact `merchant_info` equality, not substring — consistent with how mute-by-merchant works)
