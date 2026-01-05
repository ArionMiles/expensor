# expensor

Expensor is a utility to find expense related emails from my bank and add those expenses to a Google Sheet for expense tracking. The Google Sheet then feeds a Grafana dashboard. Expensor relies on config based rules which can work for almost everyone.

I've documented below why exactly expensor works for me. If you find these reasons just, you might find it useful too. The rules are dead simple regex extractions which are fast (as far as reading emails go) and can be updated/modified pretty easily.

## Design Decisions

It's well known that the transactions recorded in a Bank/CC statement aren't precise enough. It only contains the merchant information which makes it harder to keep track of things like what exact items you purchased, the food you ordered, or path of your cab trips. Getting visibility into these aspects requires fetching them from the source, i.e relying on APIs (if these platforms have public APIs) or scraping them via various different means.

Despite this limitation of granularity, I favor my current approach and implementation for the following reasons.

### Extensibility
I can quickly start recording transactions if I start using a new Bank, or a new Credit Card by simply writing a new rule. Of course, this assumes the new bank/CC would be sending emails as well, but it's more probable that they would send emails than have a programmable API or a better means of exporting & automating my expense tracking workflow.

### Maintainablity
Relying on source of transactions also means a lot more work, since I'd need to build support for them & maintaining them. If for any reason these APIs stop working, or if there's a new source and I do not have sufficient time to add support, I lose visibility into my expenses.

Comparatively, Google's GMail API which I currently use & support remain stable and don't change often if they ever do.

### Minimal intervention
Expensor periodically checks the inbox for new transactions. I then conduct a review of everything it discovers and add/update details as necessary. This works well because this is how I've been tracking my expenses for the past 3 years and my only pain point is in recording these transactions manually into a single pane (a spreadsheet). This takes a huge chunk of work off my hands and enables me to review the details with ease.

If I relied on the main source of transactions, chances are that I'd have to end up either manually collecting data from them and feeding them into expensor or even if I managed to automate them, I have the burden of keeping them up to date, which is neither pleasant nor rewarding.

### I don't need fine granularity of transactions
In the past 3 years since I've been manually tracking my expenses, I've never really had the need to gain insights into what kind of food I'm ordering often, or what routes I'm taking when using Uber/etc. I'm only interested in facts such as the total amount of money I've spent ordering online, or using cabs in a month. The aggregate amounts I'm spending on needs/wants/investments is what I'm actually interested in monitoring.

During manual review of these expenses, if I ever come across a spend I've no clue about, I can dig deeper by going to the merchant's platform against whom the transaction was made, and add comments to the expense report. I do keep granular track of anything I buy for my hobby items, or a big purchase, but those are few and far between.

## How does it work?
Expensor is designed to 
1. Periodically check my inbox
2. Run the queries defined in my [rules](cmd/expensor/config/rules.json) to find emails of interest
3. Extract transaction details like the amount, merchant name, date of transaction.
4. Write them to a Google Sheet.
5. Repeat

## Future
There's a few more items I need to work on to make it more polished and robust, a few of them are mentioned in [#1](https://github.com/ArionMiles/expensor/issues/2). 

Might get around to finishing it off one of these days, haha!
  
## Installation

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/ArionMiles/expensor/releases).

### Docker

Pull the latest Docker image from GitHub Container Registry:

```bash
docker pull ghcr.io/arionmiles/expensor:latest
```

### From Source

Requires Go 1.25.5 or later:

```bash
go install github.com/ArionMiles/expensor/cmd/expensor@latest
```

## Setup

1. **Configure Google Cloud Project**
   - Follow the [Gmail API Quickstart Guide](https://developers.google.com/gmail/api/quickstart/go)
   - Download the OAuth credentials JSON file
   - Enable both Gmail API and Google Sheets API for your project

2. **Run Setup**
   ```bash
   expensor setup --secrets credentials.json
   ```
   This will guide you through OAuth authentication and save the token.

3. **Configure Rules**
   - Copy `cmd/expensor/config/rules.json` to your working directory
   - Customize the rules to match your bank's email format
   - See the rules section below for details

4. **Run Expensor**
   ```bash
   expensor run --config config.json
   ```

## Running with Docker

Create a `docker-compose.yml` file:

```yaml
version: '3.8'
services:
  expensor:
    image: ghcr.io/arionmiles/expensor:latest
    volumes:
      - ./config.json:/app/config.json:ro
      - ./credentials.json:/app/credentials.json:ro
      - ./token.json:/app/token.json
    command: ["run", "--config", "/app/config.json"]
    restart: unless-stopped
```

Run with:

```bash
docker-compose up -d
```

## Configuration

The `config.json` file contains:

- Gmail query patterns to find transaction emails
- Regular expressions to extract transaction details
- Google Sheets configuration
- Polling interval and other settings

Example:

```json
{
  "gmail": {
    "query": "from:alerts@bank.com subject:transaction"
  },
  "sheets": {
    "spreadsheet_id": "your-sheet-id-here"
  },
  "rules": "rules.json"
}
```

## Development

This project uses [Task](https://taskfile.dev) for automation.

### Setup Development Environment

```bash
# Install Task (if not already installed)
brew install go-task/tap/go-task  # macOS
# or
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin

# Install development tools
task install-tools
```

### Common Tasks

```bash
# Format code
task fmt

# Run linter (local config)
task lint

# Run tests
task test

# Run tests with coverage
task test:cover

# Build binary
task build:binary

# Build Docker image
task build:docker

# Run all CI checks
task ci
```

## Expensor doesn't support/recognize transactions from my bank

Open an issue with the email body content and I will take a look, and possibly add the rules into expensor so those are supported.
