# Contributing to Expensor

Thank you for your interest in contributing to Expensor! This guide will help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Issue Guidelines](#issue-guidelines)

## Code of Conduct

This project follows the standard open source code of conduct. Please be respectful and constructive in all interactions.

## Getting Started

### Prerequisites

- Go 1.25.5 or later
- Task 3.x ([installation guide](https://taskfile.dev/installation/))
- Git
- Docker (optional, for building images)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/expensor.git
   cd expensor
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/ArionMiles/expensor.git
   ```

## Development Workflow

### 1. Create a Branch

Always create a new branch for your work:

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test additions/improvements

### 2. Make Your Changes

Write clean, well-documented code following Go best practices:

```go
// Good: Clear function with documentation
// ExtractAmount parses a currency string and returns the amount as float64.
// It handles common formats like "Rs. 1,234.56" and "$1,234.56".
func ExtractAmount(text string) (float64, error) {
    // Implementation
}

// Bad: No documentation, unclear purpose
func extract(s string) float64 {
    // Implementation
}
```

### 3. Format and Lint

Before committing:

```bash
# Format code (imports + gofumpt)
task fmt

# Run linter
task lint
```

### 4. Write Tests

Add tests for new functionality:

```go
func TestExtractAmount(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    float64
        wantErr bool
    }{
        {
            name:    "valid Indian rupees",
            input:   "Rs. 1,234.56",
            want:    1234.56,
            wantErr: false,
        },
        {
            name:    "invalid format",
            input:   "not a number",
            want:    0,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ExtractAmount(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ExtractAmount() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("ExtractAmount() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

Run tests:

```bash
# Run all tests
task test

# Run with coverage
task test:cover
```

### 5. Commit Your Changes

Write clear, descriptive commit messages:

```bash
git add .
git commit -m "feat: add support for HDFC Bank transaction emails

- Add regex pattern for HDFC debit card emails
- Extract amount, date, and merchant name
- Add tests for new pattern

Fixes #123"
```

Commit message format:
```
<type>: <subject>

<body>

<footer>
```

Types:
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation changes
- `style` - Formatting, missing semicolons, etc.
- `refactor` - Code restructuring
- `test` - Adding tests
- `chore` - Maintenance tasks

### 6. Keep Your Branch Updated

Regularly sync with upstream:

```bash
git fetch upstream
git rebase upstream/main
```

### 7. Run CI Checks Locally

Before pushing, run the same checks as CI:

```bash
task ci
```

This runs:
- Production linting configuration
- All unit tests

### 8. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub.

## Coding Standards

### Go Style Guide

Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) and [Effective Go](https://golang.org/doc/effective_go.html).

### Project-Specific Guidelines

#### 1. Error Handling

Always handle errors explicitly:

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to read config: %w", err)
}

// Bad: Ignoring errors
_ = file.Close()

// Good: Defer with error check
defer func() {
    if err := file.Close(); err != nil {
        log.Printf("failed to close file: %v", err)
    }
}()
```

#### 2. Logging

Use structured logging with `log/slog`:

```go
// Good: Structured logging
logger.Info("processing transaction",
    "amount", amount,
    "merchant", merchant,
    "date", date)

// Bad: String concatenation
log.Printf("Processing transaction: %f from %s on %s", amount, merchant, date)
```

#### 3. Context Usage

Always pass context for cancellation:

```go
// Good: Context-aware
func ProcessEmails(ctx context.Context, emails []string) error {
    for _, email := range emails {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Process email
        }
    }
    return nil
}
```

#### 4. Interface Design

Keep interfaces small and focused:

```go
// Good: Small, focused interface
type Reader interface {
    Read(ctx context.Context) (<-chan Transaction, error)
}

// Bad: Large interface with many methods
type ReaderWriterProcessor interface {
    Read() []Transaction
    Write([]Transaction) error
    Process() error
    Validate() error
}
```

#### 5. Package Organization

```
pkg/
├── reader/          # Email reading implementations
│   └── gmail/
├── writer/          # Output writers (sheets, csv, json)
│   ├── sheets/
│   ├── csv/
│   └── json/
├── config/          # Configuration handling
├── logging/         # Logging setup
└── api/             # API clients
```

### Linter Configuration

The project uses golangci-lint with two configurations:

- `.golangci.toml` - Local development (less strict)
- `.golangci-prod.toml` - CI/Production (strict)

Run incrementally for new code:

```bash
task lint:new
```

## Testing

### Test Requirements

- All new code must have tests
- Aim for >80% code coverage
- Use table-driven tests
- Test error cases

### Test Structure

```go
func TestFunctionName(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"

    // Act
    result, err := FunctionName(input)

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

### Running Tests

```bash
# All tests
task test

# With coverage
task test:cover

# Specific package
go test ./pkg/reader/gmail/...

# Verbose
go test -v ./...

# Race detector
go test -race ./...
```

## Pull Request Process

### Before Submitting

1. Run all checks: `task ci`
2. Update documentation if needed
3. Ensure all tests pass
4. Rebase on latest main

### PR Description

Use the provided template and include:

- **Description:** What changes were made and why
- **Type of Change:** Feature, bug fix, etc.
- **Related Issue:** Link to issue(s)
- **Testing:** How you tested the changes
- **Screenshots:** If applicable

### Review Process

1. Automated checks must pass (linting, tests, security)
2. At least one maintainer approval required
3. Address all review comments
4. Keep PR focused and reasonably sized
5. Squash commits if requested

### After Merge

- Your branch will be deleted automatically
- Changes will be included in the next release
- You'll be credited in the release notes

## Issue Guidelines

### Before Creating an Issue

1. Search existing issues to avoid duplicates
2. Try the latest version

### Issue Templates

Use the appropriate template:

- **Bug Report:** For bugs and unexpected behavior
- **Feature Request:** For new features or enhancements
- **Bank Support:** For adding support for new banks

### Bug Report Requirements

- Clear title and description
- Steps to reproduce
- Expected vs actual behavior
- Version information
- Logs (if applicable)

### Feature Request Requirements

- Problem statement
- Proposed solution
- Alternative solutions considered
- Use cases

## Adding Bank Support

To add support for a new bank:

1. Create an issue using the "Bank Support" template
2. Provide a **redacted** sample email
3. Fork and create a branch: `feature/bank-BANKNAME`
4. Add regex patterns to `cmd/expensor/config/rules.json`
5. Add tests with sample data
6. Submit a pull request

Example rule structure:

```json
{
  "name": "Bank Name Debit Card",
  "query": "from:alerts@bank.com subject:transaction",
  "patterns": {
    "amount": "Rs\\. ([0-9,]+\\.[0-9]{2})",
    "merchant": "at (.+?) on",
    "date": "on (\\d{2}/\\d{2}/\\d{4})"
  }
}
```

## Getting Help

- **Questions:** Open a discussion on GitHub
- **Issues:** Use issue templates
- **Security:** Email maintainers directly (don't open public issues)

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (MIT License).

## Recognition

Contributors are recognized in:
- GitHub contributors list
- Release notes

Thank you for contributing to Expensor!
