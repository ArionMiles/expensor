// Package sheets implements a Writer that writes transactions to Google Sheets.
package sheets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/avast/retry-go"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/ArionMiles/expensor/pkg/api"
	"github.com/ArionMiles/expensor/pkg/writer/buffered"
)

// Default configuration values for buffered writes.
const (
	DefaultBatchSize     = 10
	DefaultFlushInterval = 30 * time.Second
)

// Writer writes transactions to a Google Sheet with buffered batching.
type Writer struct {
	client      *sheets.Service
	spreadsheet *sheets.Spreadsheet
	sheetName   string
	logger      *slog.Logger
	buffered    *buffered.Writer
}

// Config holds configuration for the Sheets writer.
type Config struct {
	// SheetTitle is the title for a new spreadsheet (if SheetID is empty).
	SheetTitle string
	// SheetID is the ID of an existing spreadsheet to use.
	SheetID string
	// SheetName is the name of the sheet within the spreadsheet.
	SheetName string
	// BatchSize is the number of transactions to buffer before writing.
	// Defaults to DefaultBatchSize.
	BatchSize int
	// FlushInterval is the interval between automatic flushes.
	// Defaults to DefaultFlushInterval.
	FlushInterval time.Duration
}

// New creates a new Sheets writer.
func New(httpClient *http.Client, cfg Config, logger *slog.Logger) (*Writer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	client, err := sheets.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("creating sheets service: %w", err)
	}

	w := &Writer{
		client:    client,
		sheetName: cfg.SheetName,
		logger:    logger,
	}

	spreadsheet, err := w.initSpreadsheet(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing spreadsheet: %w", err)
	}
	w.spreadsheet = spreadsheet

	// Set defaults for buffered config
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	flushInterval := cfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = DefaultFlushInterval
	}

	// Create buffered writer
	w.buffered = buffered.New(
		w.flushBatch,
		buffered.Config{
			BatchSize:     batchSize,
			FlushInterval: flushInterval,
		},
		logger.With("component", "sheets_buffer"),
	)

	logger.Info("sheets writer initialized",
		"spreadsheet_id", spreadsheet.SpreadsheetId,
		"batch_size", batchSize,
		"flush_interval", flushInterval,
	)

	return w, nil
}

func (w *Writer) initSpreadsheet(ctx context.Context, cfg Config) (*sheets.Spreadsheet, error) {
	// Try to get existing spreadsheet
	if cfg.SheetID != "" {
		spreadsheet, err := w.client.Spreadsheets.Get(cfg.SheetID).Context(ctx).Do()
		if err == nil {
			w.logger.Info("using existing spreadsheet", "title", spreadsheet.Properties.Title, "id", cfg.SheetID)
			return spreadsheet, nil
		}
		w.logger.Warn("failed to get spreadsheet, will create new one", "id", cfg.SheetID, "error", err)
	}

	// Create new spreadsheet
	spreadsheet, err := w.client.Spreadsheets.Create(&sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title: cfg.SheetTitle,
		},
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("creating spreadsheet: %w", err)
	}

	w.logger.Info("created new spreadsheet", "title", cfg.SheetTitle, "id", spreadsheet.SpreadsheetId)

	// Write headers
	if err := w.writeHeaders(ctx, spreadsheet.SpreadsheetId, cfg.SheetName); err != nil {
		return nil, fmt.Errorf("writing headers: %w", err)
	}

	return spreadsheet, nil
}

func (w *Writer) writeHeaders(ctx context.Context, spreadsheetID, sheetName string) error {
	headerRange := fmt.Sprintf("%s!A1:G1", sheetName)
	headerReq := sheets.ValueRange{
		Values: [][]any{
			{"Date/Time", "Timestamp", "Expense", "Amount", "Category", "Needs/Wants/Investments", "Source"},
		},
	}

	_, err := w.client.Spreadsheets.Values.Update(spreadsheetID, headerRange, &headerReq).
		ValueInputOption("RAW").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("updating headers: %w", err)
	}

	w.logger.Info("wrote headers to spreadsheet")
	return nil
}

// Write consumes transactions from the input channel and writes them to Google Sheets.
func (w *Writer) Write(ctx context.Context, in <-chan *api.TransactionDetails) error {
	w.logger.Info("sheets writer started")
	return w.buffered.Write(ctx, in)
}

// flushBatch writes a batch of transactions to Google Sheets in a single API call.
func (w *Writer) flushBatch(transactions []*api.TransactionDetails) error {
	if len(transactions) == 0 {
		return nil
	}

	// Build values array for batch write
	values := make([][]any, 0, len(transactions))
	for _, t := range transactions {
		values = append(values, []any{
			t.Timestamp,
			t.MerchantInfo,
			t.Amount,
			t.Category,
			t.Bucket,
			t.Source,
		})
	}

	writeRange := fmt.Sprintf("%s!A2:F2", w.sheetName)
	writeReq := sheets.ValueRange{
		Values: values,
	}

	// Use context.Background() since we're called from buffered.Writer
	// which handles context cancellation at a higher level
	ctx := context.Background()

	err := retry.Do(
		func() error {
			_, err := w.client.Spreadsheets.Values.Append(w.spreadsheet.SpreadsheetId, writeRange, &writeReq).
				ValueInputOption("USER_ENTERED").
				InsertDataOption("INSERT_ROWS").
				Context(ctx).
				Do()
			return err
		},
		retry.RetryIf(func(err error) bool {
			var apiErr *googleapi.Error
			if errors.As(err, &apiErr) && apiErr.Code == http.StatusTooManyRequests {
				w.logger.Warn("rate limited, will retry", "error", err)
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.Delay(60*time.Second),
		retry.LastErrorOnly(true),
	)
	if err != nil {
		return fmt.Errorf("appending batch to sheet: %w", err)
	}

	w.logger.Info("wrote transaction batch",
		"count", len(transactions),
		"first_merchant", transactions[0].MerchantInfo,
	)

	return nil
}

// SpreadsheetID returns the ID of the spreadsheet being written to.
func (w *Writer) SpreadsheetID() string {
	if w.spreadsheet == nil {
		return ""
	}
	return w.spreadsheet.SpreadsheetId
}

// BufferLen returns the current number of buffered transactions.
func (w *Writer) BufferLen() int {
	if w.buffered == nil {
		return 0
	}
	return w.buffered.BufferLen()
}
