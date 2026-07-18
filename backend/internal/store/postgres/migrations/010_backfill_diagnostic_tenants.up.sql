-- Recover diagnostics written through the legacy NULL-tenant path before scans became tenant-aware.
-- Only custom rules have a tenant owner, so built-in and otherwise ambiguous diagnostics remain unassigned.
UPDATE extraction_diagnostics AS diagnostic
SET tenant_id = rule.tenant_id
FROM rules AS rule
WHERE diagnostic.tenant_id IS NULL
  AND diagnostic.rule_id IS NOT NULL
  AND rule.tenant_id IS NOT NULL
  AND rule.predefined = false
  AND diagnostic.rule_id::text = rule.id::text;
