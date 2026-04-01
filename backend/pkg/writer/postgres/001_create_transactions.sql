-- Create transactions table with multi-currency support
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id VARCHAR(255) UNIQUE NOT NULL,
    amount NUMERIC(19,4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'INR',
    original_amount NUMERIC(19,4),
    original_currency VARCHAR(3),
    exchange_rate NUMERIC(10,6),
    timestamp TIMESTAMPTZ NOT NULL,
    merchant_info TEXT NOT NULL,
    category VARCHAR(100),
    bucket VARCHAR(50),
    source VARCHAR(100) NOT NULL,
    description TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create transaction_labels table for many-to-many relationship
CREATE TABLE IF NOT EXISTS transaction_labels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID REFERENCES transactions(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(transaction_id, label)
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_merchant ON transactions USING gin(to_tsvector('english', merchant_info));
CREATE INDEX IF NOT EXISTS idx_transactions_description ON transactions USING gin(to_tsvector('english', description));
CREATE INDEX IF NOT EXISTS idx_transaction_labels_label ON transaction_labels(label);
CREATE INDEX IF NOT EXISTS idx_transactions_currency ON transactions(currency);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category);
CREATE INDEX IF NOT EXISTS idx_transactions_bucket ON transactions(bucket);
CREATE INDEX IF NOT EXISTS idx_transaction_labels_transaction_id ON transaction_labels(transaction_id);

-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for updated_at
DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;
CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
