package main

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type ExternalFlow struct {
	Date      time.Time
	Direction string // "Deposit" or "Withdrawal"
	INR       decimal.Decimal
	USD       decimal.Decimal
	GST       decimal.Decimal
	BankFee   decimal.Decimal
	TCS       decimal.Decimal
	Rate      decimal.Decimal
	Source    string
}

type Trade struct {
	Date      time.Time
	Symbol    string
	Side      string
	Ref       string
	Quantity  decimal.Decimal
	Price     decimal.Decimal
	Amount    decimal.Decimal
	Brokerage decimal.Decimal
	Source    string
}

type WalletEntry struct {
	Date         time.Time
	Type         string
	Description  string
	Movement     string
	Symbol       string
	Amount       decimal.Decimal
	Quantity     decimal.Decimal
	Price        decimal.Decimal
	Commission   decimal.Decimal
	TradeBalance decimal.Decimal
	Source       string
	Order        int
	File         string
}

type Holding struct {
	Symbol   string
	Quantity decimal.Decimal
	AvgPrice decimal.Decimal
	Source   string
}

type HoldingsReport struct {
	Date     time.Time
	Holdings []Holding
	Source   string
}

type StatementHolding struct {
	Symbol     string
	Quantity   decimal.Decimal
	Market     decimal.Decimal
	MarketVal  decimal.Decimal
	CostPrice  decimal.Decimal
	CostBasis  decimal.Decimal
	Unrealized decimal.Decimal
}

type Statement struct {
	Date      time.Time
	Holdings  []StatementHolding
	LongValue decimal.Decimal
	Source    string
}

type Dataset struct {
	Flows      []ExternalFlow
	Trades     []Trade
	Wallet     []WalletEntry
	Holdings   []HoldingsReport
	Statements []Statement
	Warnings   []string
}

func (d *Dataset) warn(format string, args ...any) {
	d.Warnings = append(d.Warnings, fmt.Sprintf(format, args...))
}
