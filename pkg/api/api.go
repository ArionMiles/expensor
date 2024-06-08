package api

import (
	"context"
	"regexp"
)

type ExpenseReader interface {
	Read(ctx context.Context) error
	Write(ctx context.Context) error
}

// TransactionDetails struct to store transaction details
type TransactionDetails struct {
	Amount       float64
	Timestamp    string
	MerchantInfo string
}

type Rule struct {
	Query        string
	Amount       *regexp.Regexp
	MerchantInfo *regexp.Regexp
}
