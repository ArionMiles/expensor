# Thunderbird Reader

The Thunderbird reader plugin extracts expense transactions from local Thunderbird mailbox files (MBOX format).

## Features

- Reads transactions from local Thunderbird MBOX mailboxes
- No OAuth required (reads local files)
- Supports multiple mailboxes (Inbox, Archive, custom folders)
- State tracking to avoid reprocessing emails
- Periodic scanning with configurable interval
- Rule-based transaction extraction with regex patterns
- Multi-platform support (macOS, Linux, Windows)
- Handles various MIME encodings (multipart, base64, quoted-printable)
- Character set decoding (UTF-8, ISO-8859-1, Windows-1252, etc.)

## Configuration

### Example Configuration

```json
{
  "reader": "thunderbird",
  "readerConfig": {
    "profilePath": "/Users/username/Library/Thunderbird/Profiles/abc123.default",
    "mailboxes": ["Inbox", "Archive"],
    "stateFile": "/var/lib/expensor/thunderbird-state.json",
    "interval": 60,
    "rules": [
      {
        "name": "HDFC Bank",
        "senderEmail": "alerts@hdfcbank.net",
        "subjectContains": "debited",
        "amountRegex": "Rs\\.\\s*([\\d,]+\\.?\\d*)",
        "merchantInfoRegex": "at\\s+([A-Z0-9\\s]+)",
        "enabled": true,
        "source": "HDFC Credit Card"
      },
      {
        "name": "ICICI Bank",
        "senderEmail": "credit_cards@icicibank.com",
        "subjectContains": "transaction alert",
        "amountRegex": "INR\\s*([\\d,]+\\.?\\d*)",
        "merchantInfoRegex": "for\\s+([A-Z\\s]+)",
        "enabled": true,
        "source": "ICICI Credit Card"
      }
    ],
    "labels": {
      "AMAZON": {
        "category": "Shopping",
        "bucket": "Want"
      },
      "SWIGGY": {
        "category": "Food Delivery",
        "bucket": "Want"
      },
      "ZEPTO": {
        "category": "Groceries",
        "bucket": "Need"
      }
    }
  }
}
```

### Configuration Fields

#### profilePath (required)
Path to your Thunderbird profile directory.

**Platform-specific defaults:**
- macOS: `~/Library/Thunderbird/Profiles/<profile-name>`
- Linux: `~/.thunderbird/<profile-name>`
- Windows: `%APPDATA%/Thunderbird/Profiles/<profile-name>`

To find your profile path:
1. Open Thunderbird
2. Go to Help > More Troubleshooting Information
3. Find "Profile Folder" and click "Open Folder"
4. Copy the path

#### mailboxes (required)
Array of mailbox names to scan. Common mailbox names:
- `Inbox`
- `Sent`
- `Archive`
- `Trash`
- Custom folder names

#### stateFile (required)
Path to the JSON file that stores processed message IDs. This prevents reprocessing the same emails.

#### interval (optional)
Interval in seconds between mailbox scans. Default: 60 seconds.

#### rules (required)
Array of transaction extraction rules.

**Rule fields:**
- `name`: Human-readable rule identifier
- `senderEmail`: Match emails from this sender (case-insensitive substring match)
- `subjectContains`: Match emails with this text in subject (case-insensitive substring match)
- `amountRegex`: Regex pattern to extract transaction amount (must have one capture group)
- `merchantInfoRegex`: Regex pattern to extract merchant name (must have one capture group)
- `enabled`: Boolean to enable/disable the rule
- `source`: Transaction source identifier (e.g., bank name)

**Both `senderEmail` and `subjectContains` can be specified** - the rule will match only if both conditions are met.

#### labels (required)
Merchant-to-category mappings. Keys are merchant names (case-sensitive), values have:
- `category`: Expense category (e.g., "Shopping", "Food", "Transport")
- `bucket`: Budget bucket classification (e.g., "Need", "Want", "Investment")

## How It Works

1. **Profile Discovery**: The reader locates your Thunderbird profile and finds the specified mailboxes
2. **MBOX Reading**: Parses MBOX files using the standard MBOX format
3. **Rule Matching**: For each unprocessed message, checks sender and subject against enabled rules
4. **Body Extraction**: Extracts email body, handling multipart MIME, various encodings, and character sets
5. **Transaction Extraction**: Uses regex patterns to extract amount and merchant from email body
6. **Label Lookup**: Maps merchant names to categories and buckets
7. **State Tracking**: Records processed message IDs in the state file
8. **Periodic Scanning**: Repeats the process at the configured interval

## Finding Mailbox Locations

Thunderbird stores mailboxes in these locations within the profile directory:

```
<profile>/Mail/Local Folders/
  ├── Inbox
  ├── Sent
  ├── Trash
  └── ...

<profile>/Mail/<account-name>/
  ├── INBOX
  ├── Sent
  └── ...

<profile>/ImapMail/<server>/
  ├── INBOX
  ├── Archive
  └── ...
```

The reader automatically searches these locations for the specified mailbox names.

## State Management

The state file stores a JSON map of processed message IDs:

```json
{
  "processed_messages": {
    "abc123...": "2024-01-13T10:30:00Z",
    "def456...": "2024-01-13T11:45:00Z"
  }
}
```

Each message key is a SHA256 hash of:
- Mailbox file path
- Message-ID header
- Date header

This ensures messages are uniquely identified even across different mailboxes.

## Regex Pattern Examples

### Amount Patterns

```regex
Rs\.\s*([\d,]+\.?\d*)           # Matches: Rs. 1,234.56
INR\s*([\d,]+\.?\d*)            # Matches: INR 500.00
₹\s*([\d,]+\.?\d*)              # Matches: ₹ 2,500
\$\s*([\d,]+\.?\d*)             # Matches: $ 99.99
```

### Merchant Patterns

```regex
at\s+([A-Z0-9\s]+)              # Matches: at AMAZON INDIA
on\s+([A-Z\s]+)                 # Matches: on SWIGGY
for\s+([A-Z\s]+)                # Matches: for UBER TRIP
to\s+([A-Z\s]+)                 # Matches: to NETFLIX COM
```

## Performance Considerations

- **Large Mailboxes**: For mailboxes with thousands of messages, the initial scan may take a few minutes. Subsequent scans are fast since processed messages are skipped.
- **State File Size**: The state file grows with each processed message. Consider periodic pruning of old entries.
- **Scan Interval**: Set an appropriate interval based on email frequency. For transaction alerts, 60-300 seconds is reasonable.

## Limitations

- Read-only: Unlike the Gmail reader, Thunderbird reader cannot mark emails as read
- Local only: Requires local access to Thunderbird profile directory
- MBOX format only: Does not support Maildir format
- No real-time updates: Relies on periodic scanning

## Troubleshooting

### Mailbox Not Found

If you see "mailbox not found" errors:
1. Verify the profile path is correct
2. Check that the mailbox name matches exactly (case-sensitive)
3. Look in `<profile>/Mail/` subdirectories for your mailbox files
4. Use `ls -la <profile>/Mail/Local\ Folders/` to see available mailboxes

### No Transactions Extracted

If transactions aren't being extracted:
1. Check that rule `senderEmail` and `subjectContains` match your emails
2. Test regex patterns using a regex tester (e.g., regex101.com)
3. Verify the state file isn't marking messages as already processed
4. Enable debug logging to see which messages are being processed

### State File Issues

If messages are being reprocessed:
1. Check write permissions for the state file
2. Verify the state file isn't being deleted between runs
3. Look for errors in the logs about state file saving

## Example Integration

```go
package main

import (
    "context"
    "log/slog"

    "github.com/ArionMiles/expensor/backend/pkg/api"
    "github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
)

func main() {
    cfg := thunderbird.Config{
        ProfilePath: "/path/to/profile",
        Mailboxes:   []string{"Inbox"},
        StateFile:   "/tmp/state.json",
        Interval:    60 * time.Second,
        Rules:       []thunderbird.Rule{ /* ... */ },
        Labels:      make(api.Labels),
    }

    reader, err := thunderbird.New(cfg, slog.Default())
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    out := make(chan *api.TransactionDetails, 100)
    ackChan := make(chan string, 100)

    go reader.Read(ctx, out, ackChan)

    for txn := range out {
        // Process transaction
        ackChan <- txn.MessageID
    }
}
```

## Testing

Run tests with:

```bash
cd backend
go test -v ./pkg/reader/thunderbird/...
go test -v ./pkg/plugins/readers/thunderbird/...
```

Check coverage:

```bash
go test -cover ./pkg/reader/thunderbird/...
```
