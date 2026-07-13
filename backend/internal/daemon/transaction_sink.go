package daemon

import (
	"context"
	"log/slog"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type transactionSink struct {
	writer        store.TransactionBatchWriter
	tenant        store.Tenant
	logger        *slog.Logger
	batchSize     int
	flushInterval time.Duration
}

func newTransactionSink(writer store.TransactionBatchWriter, cfg store.IngestionConfig, logger *slog.Logger) (*transactionSink, error) {
	if writer == nil {
		return nil, errors.E("daemon.transaction_sink.new", errors.FailedPrecondition, "transaction batch writer is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.BatchSize <= 0 {
		return nil, errors.E("daemon.transaction_sink.new", errors.InvalidInput, "transaction batch size must be positive")
	}
	if cfg.FlushInterval <= 0 {
		return nil, errors.E("daemon.transaction_sink.new", errors.InvalidInput, "transaction flush interval must be positive")
	}
	return &transactionSink{
		writer:        writer,
		tenant:        cfg.Tenant,
		logger:        logger,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
	}, nil
}

func (s *transactionSink) Write(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
	batch := make([]*api.TransactionDetails, 0, s.batchSize)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := s.flushBatch(ctx, &batch, ackChan); err != nil {
				s.logger.Error("failed to flush final batch", "error", err)
			}
			return ctx.Err()
		case txn, ok := <-in:
			if !ok {
				return s.flushBatch(ctx, &batch, ackChan)
			}
			batch = append(batch, txn)
			if len(batch) >= s.batchSize {
				if err := s.flushBatch(ctx, &batch, ackChan); err != nil {
					return err
				}
			}
		case <-ticker.C:
			if err := s.flushBatch(ctx, &batch, ackChan); err != nil {
				return err
			}
		}
	}
}

func (s *transactionSink) flushBatch(ctx context.Context, batch *[]*api.TransactionDetails, ackChan chan<- string) error {
	if len(*batch) == 0 {
		return nil
	}
	if err := s.writer.Write(ctx, store.IngestionBatch{
		Tenant:       s.tenant,
		Transactions: *batch,
	}); err != nil {
		return err
	}
	for _, txn := range *batch {
		if txn.MessageID == "" {
			continue
		}
		select {
		case ackChan <- txn.MessageID:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.logger.Info("wrote transaction batch", "count", len(*batch))
	*batch = (*batch)[:0]
	return nil
}
