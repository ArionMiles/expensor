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
	Category     string
	// Need/Want/Investments
	Bucket string
	Source string
}

type Rule struct {
	Name         string
	Query        string
	Amount       *regexp.Regexp
	MerchantInfo *regexp.Regexp
	Enabled      bool
	Source       string
}

type Labels map[string]struct {
	Category string `json:"category"`
	Bucket   string `json:"bucket"`
}
