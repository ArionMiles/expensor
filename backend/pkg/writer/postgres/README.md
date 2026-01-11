# PostgreSQL Writer

The PostgreSQL writer plugin stores expense transactions in a PostgreSQL database with support for multi-currency transactions, labels, and full-text search.

## Features

- **Multi-currency support**: Store amounts in different currencies with exchange rate tracking
- **Automatic migrations**: Database schema is created automatically on first run
- **Batch writes**: Configurable batch size for optimal performance
- **Upsert logic**: Handles duplicate transactions using message_id as unique key
- **Transaction labels**: Many-to-many relationship for flexible categorization
- **Connection pooling**: Efficient connection management with configurable pool size
- **Full-text search**: GIN indexes for fast merchant and description search

## Database Schema

### Transactions Table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| message_id | VARCHAR(255) | Unique email message ID |
| amount | NUMERIC(19,4) | Transaction amount |
| currency | VARCHAR(3) | Currency code (e.g., INR, USD, EUR) |
| original_amount | NUMERIC(19,4) | Original amount if converted |
| original_currency | VARCHAR(3) | Original currency if converted |
| exchange_rate | NUMERIC(10,6) | Exchange rate used for conversion |
| timestamp | TIMESTAMPTZ | Transaction timestamp |
| merchant_info | TEXT | Merchant name/description |
| category | VARCHAR(100) | Transaction category |
| bucket | VARCHAR(50) | Budget bucket (Need/Want/Investment) |
| source | VARCHAR(100) | Transaction source (e.g., Credit Card - ICICI) |
| description | TEXT | User-added description |
| metadata | JSONB | Additional metadata |
| created_at | TIMESTAMPTZ | Record creation time |
| updated_at | TIMESTAMPTZ | Record last update time |

### Transaction Labels Table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| transaction_id | UUID | Foreign key to transactions |
| label | VARCHAR(100) | Label name |
| created_at | TIMESTAMPTZ | Label creation time |

## Configuration

### Environment Variables

```bash
EXPENSOR_WRITER=postgres
POSTGRES_HOST=localhost          # PostgreSQL host
POSTGRES_PORT=5432              # PostgreSQL port (default: 5432)
POSTGRES_DB=expensor            # Database name
POSTGRES_USER=expensor          # Database user
POSTGRES_PASSWORD=secret        # Database password
POSTGRES_SSLMODE=disable        # SSL mode (disable/require/verify-ca/verify-full)
```

### JSON Configuration

```json
{
  "host": "localhost",
  "port": 5432,
  "database": "expensor",
  "user": "expensor",
  "password": "secret",
  "sslmode": "disable",
  "batchSize": 10,
  "flushInterval": 30,
  "maxPoolSize": 10
}
```

Set via environment variable:

```bash
EXPENSOR_WRITER_CONFIG='{
  "host": "postgres",
  "port": 5432,
  "database": "expensor",
  "user": "expensor",
  "password": "expensor_password",
  "sslmode": "disable"
}'
```

## Docker Compose Example

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: expensor
      POSTGRES_USER: expensor
      POSTGRES_PASSWORD: expensor_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U expensor"]
      interval: 10s
      timeout: 5s
      retries: 5

  expensor:
    image: expensor:latest
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      - EXPENSOR_READER=gmail
      - EXPENSOR_WRITER=postgres
      - POSTGRES_HOST=postgres
      - POSTGRES_PORT=5432
      - POSTGRES_DB=expensor
      - POSTGRES_USER=expensor
      - POSTGRES_PASSWORD=expensor_password
      - POSTGRES_SSLMODE=disable
    volumes:
      - expensor_data:/app/data
    ports:
      - "8080:8080"

volumes:
  postgres_data:
  expensor_data:
```

## Performance Tuning

### Batch Size

Controls how many transactions are buffered before writing to the database. Higher values reduce database round-trips but increase memory usage.

- **Small workload** (< 100 txns/day): 5-10
- **Medium workload** (100-1000 txns/day): 10-20
- **Large workload** (> 1000 txns/day): 20-50

### Flush Interval

Time between automatic flushes (in seconds). Ensures transactions are written even if batch size isn't reached.

- **Real-time updates**: 10-15 seconds
- **Normal usage**: 30 seconds (default)
- **Batch processing**: 60+ seconds

### Connection Pool

Maximum number of concurrent database connections.

- **Single user**: 5-10
- **Multiple users**: 10-20
- **High concurrency**: 20-50

## Multi-Currency Examples

### Simple Transaction (INR)

```go
txn := &api.TransactionDetails{
    MessageID:    "msg123",
    Amount:       1234.56,
    Currency:     "INR",
    Timestamp:    "2026-01-11T10:00:00Z",
    MerchantInfo: "SWIGGY",
    Category:     "Food",
    Bucket:       "Wants",
    Source:       "Credit Card - ICICI",
}
```

### Foreign Currency with Conversion

```go
originalAmount := 100.00
originalCurrency := "USD"
exchangeRate := 83.50
convertedAmount := originalAmount * exchangeRate

txn := &api.TransactionDetails{
    MessageID:        "msg456",
    Amount:           convertedAmount, // 8350.00
    Currency:         "INR",
    OriginalAmount:   &originalAmount,
    OriginalCurrency: &originalCurrency,
    ExchangeRate:     &exchangeRate,
    Timestamp:        "2026-01-11T10:00:00Z",
    MerchantInfo:     "Amazon.com",
    Category:         "Shopping",
    Bucket:           "Wants",
    Source:           "Credit Card - ICICI",
}
```

### Transaction with Labels

```go
txn := &api.TransactionDetails{
    MessageID:    "msg789",
    Amount:       500.00,
    Currency:     "INR",
    Timestamp:    "2026-01-11T10:00:00Z",
    MerchantInfo: "Starbucks",
    Category:     "Food",
    Bucket:       "Wants",
    Source:       "Credit Card - ICICI",
    Description:  "Coffee with team",
    Labels:       []string{"work", "coffee", "team-expense"},
}
```

## Indexes

The following indexes are created automatically for optimal query performance:

1. **idx_transactions_timestamp**: B-tree index on timestamp (DESC) for chronological queries
2. **idx_transactions_merchant**: GIN index for full-text search on merchant_info
3. **idx_transactions_description**: GIN index for full-text search on description
4. **idx_transaction_labels_label**: B-tree index on label for filtering by label
5. **idx_transactions_currency**: B-tree index for filtering by currency
6. **idx_transactions_category**: B-tree index for filtering by category
7. **idx_transactions_bucket**: B-tree index for filtering by bucket

## Example Queries

### Find all transactions in USD

```sql
SELECT * FROM transactions WHERE currency = 'USD';
```

### Full-text search on merchant

```sql
SELECT *
FROM transactions
WHERE to_tsvector('english', merchant_info) @@ to_tsquery('english', 'starbucks');
```

### Transactions with specific label

```sql
SELECT t.*
FROM transactions t
JOIN transaction_labels tl ON t.id = tl.transaction_id
WHERE tl.label = 'work';
```

### Multi-currency summary

```sql
SELECT
    currency,
    COUNT(*) as count,
    SUM(amount) as total
FROM transactions
GROUP BY currency
ORDER BY total DESC;
```

## Error Handling

The writer handles the following scenarios:

- **Duplicate transactions**: Upserts based on message_id
- **Connection failures**: Returns error, allows retry
- **Transaction rollback**: All-or-nothing batch writes
- **Invalid timestamps**: Falls back to current time
- **Missing currency**: Defaults to INR

## Migration

The migration SQL is embedded in the binary and runs automatically on first connection. The migration is idempotent and safe to run multiple times.

## Security Considerations

1. **Use strong passwords** for database credentials
2. **Enable SSL/TLS** in production (set `sslmode=require` or higher)
3. **Restrict database user permissions** to only what's needed
4. **Use connection pooling** to prevent connection exhaustion
5. **Validate input data** before writing (handled by writer)
6. **Use parameterized queries** to prevent SQL injection (handled by pgx)

## Troubleshooting

### Connection Issues

```
Error: failed to create http client: pinging database: ...
```

**Solutions:**
- Verify PostgreSQL is running
- Check host/port configuration
- Ensure database exists
- Verify user credentials
- Check firewall rules

### Migration Failures

```
Error: running migrations: executing migration: ...
```

**Solutions:**
- Check database permissions
- Verify PostgreSQL version (16+ recommended)
- Review migration SQL in logs
- Ensure database is empty or compatible

### Performance Issues

- Increase batch size for bulk operations
- Adjust flush interval based on workload
- Scale connection pool size
- Monitor database query performance
- Add custom indexes for specific queries
