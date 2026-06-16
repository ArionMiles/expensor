DELETE FROM rules
WHERE predefined = TRUE
  AND name IN (
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
