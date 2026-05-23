-- Remove stale predefined rules that were present in the v1 bundled rule set
-- but are no longer part of the v2 bundled rules.
--
-- This exists separately from 003 because early builds may have already
-- recorded the first 003 migration before stale-rule cleanup was added there.

DELETE FROM rules
WHERE predefined = TRUE
  AND name NOT IN (
      'ICICI Credit Card',
      'HDFC Credit Card',
      'HDFC Credit Card Debit Alert',
      'HDFC Credit Card Payment Made',
      'HDFC UPI',
      'Axis Bank Credit Card',
      'ICICI iMobile Fund Transfer',
      'ICICI NEFT iMobile',
      'ICICI Debit Card'
  );
