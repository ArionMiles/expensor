# Store Query Inventory 2026-04-28

Inventory source:

```bash
rg -n "func \\(s \\*Store\\)|Query\\(|QueryRow\\(|Exec\\(" backend/internal/store/store.go
```

| Area | Methods | Query style | Risk | Suggested repository |
|---|---|---|---|---|
| Transactions list/search | `ListTransactions`, `SearchTransactions`, `GetTransaction`, `queryTransactionTotals`, `loadLabels` | Dynamic filters and joins, trusted dynamic order clauses, pagination, label joins | High | `TransactionRepository` |
| Transaction edits | `UpdateDescription`, `UpdateTransaction`, `AddLabel`, `AddLabels`, `RemoveLabel`, `MuteTransaction`, `UpdateMuteReason`, `CategorizeMerchant` | Point updates plus short transactions for label/category side effects | Medium | `TransactionRepository` |
| Dashboard stats and charts | `GetStats`, `GetChartData`, `getChartDataAt`, `GetDashboardData`, `getStatsBetween`, `getChartDataBetween`, `queryCategoryMonthly`, `queryCategoryMonthlyAt`, `queryCategoryMonthlyBetween`, `GetSpendingHeatmap`, `GetAnnualSpend`, `GetMonthlyBreakdownSpend`, `queryTimeBuckets`, `queryStringFloat` | Fixed aggregation SQL with time-window and dimension parameters; some concurrent query fan-out | Medium | `ReportingRepository` |
| Facets | `GetFacets` | Fixed repeated distinct-value queries | Low | `TransactionRepository` or `ReportingRepository` |
| App config | `initAppConfig`, `GetAppConfig`, `SetAppConfig`, `SetActiveReader`, `GetActiveReader`, `GetSyncStatus`, `SetSyncStatus`, `GetCommunityURL`, `SetCommunityURL` | Key/value reads and upserts | Low | `RuntimeRepository` |
| Reader runtime state | `SetReaderSecret`, `GetReaderSecret`, `SetReaderToken`, `GetReaderToken`, `DeleteReaderToken`, `SetReaderConfig`, `GetReaderConfig`, `DeleteReaderRuntime`, `setReaderJSON`, `getReaderJSON` | Reader-scoped row updates with trusted column allowlist helpers | Medium | `RuntimeRepository` |
| Processed messages | `IsMessageProcessed`, `MarkMessageProcessed` | Fixed existence check and upsert | Low | `RuntimeRepository` |
| Labels | `initLabels`, `ListLabels`, `CreateLabel`, `UpdateLabel`, `DeleteLabel`, `ApplyLabelByMerchant`, `RemoveLabelByMerchant`, `GetLabelMappings` | CRUD plus transaction-backed propagation/removal queries | Low for CRUD, medium for propagation | `TaxonomyRepository` or `LabelRepository` during migration |
| Categories and buckets | `initCategoriesBuckets`, `ListCategories`, `CreateCategory`, `DeleteCategory`, `ListBuckets`, `CreateBucket`, `DeleteBucket` | CRUD with default-row delete guards | Low | `TaxonomyRepository` |
| Rules | `initRules`, `ListRules`, `GetRule`, `CreateRule`, `UpdateRule`, `DeleteRule`, `SeedPredefinedRules`, `ImportUserRules` | CRUD, seed upserts, import transaction | Medium | `RuleRepository` |
| Extraction diagnostics | `RecordExtractionDiagnostic`, `ListExtractionDiagnostics`, `GetExtractionDiagnostic`, `UpdateExtractionDiagnosticStatus` | Insert/list/detail/status updates with dynamic list filters | Medium | `DiagnosticRepository` |
| Muted merchants | `MuteByMerchant`, `ListMutedMerchants`, `GetMutedMerchantsWithCount`, `DeleteMutedMerchant`, `UnmuteByPattern`, `DeleteMutedMerchantAndUnmute`, `GetMutedMerchantPatterns` | CRUD plus transaction-backed bulk mute/unmute side effects | Medium | `TransactionRepository` or `MuteRepository` |
| Community content | `SeedMCCCodes`, `SeedMerchantCategories`, `LoadCategorySnapshot`, `SeedMCCCategories` | Bulk seed/upsert and snapshot load | Medium | `ContentRepository` |
| Store helpers | `Close`, `nowTime`, `appTimezone`, `dashboardBaseCurrency`, `dashboardMonthBounds` | No direct SQL or config lookup wrappers | Low | Keep on `Store` or move beside caller repository |

## Observations

- Transaction list/search is the highest-risk migration area because it composes dynamic `WHERE`, `JOIN`, ordering, pagination, totals, and label loading.
- Label/category/bucket CRUD is the safest first extraction target because the SQL is fixed and handlers can keep using `Store` forwarding methods.
- Runtime state is a good second target after labels because DB runtime state is already isolated behind key/value and reader-scoped methods.
- Community sync and reporting should be migrated after instrumentation rules are settled because they contain multi-query workflows where operation naming matters.
