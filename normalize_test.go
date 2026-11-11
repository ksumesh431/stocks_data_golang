package main

import (
	"testing"

	"github.com/shopspring/decimal"
)

func walletEntry(file, date, balance string, order int) WalletEntry {
	return WalletEntry{
		File:         file,
		Date:         testDate(date),
		Type:         "BUY",
		TradeBalance: dec(balance),
		Order:        order,
	}
}

func TestWalletBalanceMergesIncompleteExports(t *testing.T) {
	entries := []WalletEntry{
		walletEntry("latest.xlsx", "2026-05-10", "130", 0),
		walletEntry("latest.xlsx", "2026-05-08", "110", 1),
		walletEntry("old.xlsx", "2026-05-08", "110", 0),
		walletEntry("old.xlsx", "2025-05-29", "100", 1),
	}
	entry, ok := walletBalanceAt(entries, testDate("2026-05-08"))
	if !ok {
		t.Fatal("expected a wallet entry")
	}
	if !entry.TradeBalance.Equal(dec("110")) {
		t.Fatalf("balance = %s, want 110", entry.TradeBalance)
	}
	if entry.File != "latest.xlsx" {
		t.Fatalf("file = %s, want latest.xlsx", entry.File)
	}

	entry, ok = walletBalanceAt(entries, testDate("2025-12-31"))
	if !ok || !entry.TradeBalance.Equal(dec("100")) {
		t.Fatalf("older balance = %s, want 100", entry.TradeBalance)
	}
}

func TestDeduplicateFlowsAcrossAnnualReports(t *testing.T) {
	flows := []ExternalFlow{
		{Date: testDate("2026-01-01"), Direction: "Deposit", INR: dec("8603"), USD: dec("100")},
		{Date: testDate("2026-01-01"), Direction: "Deposit", INR: dec("8603"), USD: dec("100")},
		{Date: testDate("2026-02-01"), Direction: "Deposit", INR: dec("4301.5"), USD: dec("50")},
	}
	result := deduplicateFlows(flows)
	if len(result) != 2 {
		t.Fatalf("deduplicated flows = %d, want 2", len(result))
	}
}

func TestTradeDeduplicationPrefersNewestVersion(t *testing.T) {
	trades := []Trade{
		{Date: testDate("2026-01-01"), Ref: "abc", Symbol: "AAA", Side: "BUY", Quantity: dec("1"), Amount: dec("10"), Source: "old"},
		{Date: testDate("2026-01-01"), Ref: "abc", Symbol: "AAA", Side: "BUY", Quantity: dec("1"), Amount: dec("11"), Source: "new"},
	}
	result := deduplicateTrades(trades)
	if len(result) != 1 {
		t.Fatalf("trades = %d, want 1", len(result))
	}
	if !result[0].Amount.Equal(dec("11")) {
		t.Fatalf("amount = %s, want latest report value 11", result[0].Amount)
	}
}

func TestStatementHoldingsFromPlainText(t *testing.T) {
	lines := []string{
		"Holdings", "Symbol", "Description", "Quantity", "Market Price",
		"Market Value", "Cost Price", "Unrealized", "TD Cost Basis",
		"*Cash", "USD", "$ --", "$ --", "-$1,092.99", "$ --", "$ --", "$ --",
		"AMZN", "AMAZON COM INC COM", "0.958556882", "$240.35", "$230.39",
		"$220.20", "$19.31", "$211.08",
		"Income",
	}
	holdings, err := statementHoldings(lines)
	if err != nil {
		t.Fatal(err)
	}
	if len(holdings) != 1 || holdings[0].Symbol != "AMZN" {
		t.Fatalf("holdings = %+v", holdings)
	}
	if !holdings[0].MarketVal.Equal(dec("230.39")) {
		t.Fatalf("market value = %s", holdings[0].MarketVal)
	}
	if !holdings[0].CostBasis.Equal(dec("211.08")) {
		t.Fatalf("cost basis = %s", holdings[0].CostBasis)
	}
}

func TestValuationRequiresPrice(t *testing.T) {
	dataset := Dataset{Wallet: []WalletEntry{walletEntry("wallet.xlsx", "2026-01-02", "10", 0)}}
	report := HoldingsReport{
		Date:     testDate("2026-01-02"),
		Holdings: []Holding{{Symbol: "AAA", Quantity: dec("2"), AvgPrice: dec("10")}},
	}
	if _, err := buildValuation(dataset, report, map[string]Quote{}, testDate("2026-01-02")); err == nil {
		t.Fatal("expected missing price error")
	}
	valuation, err := buildValuation(dataset, report, map[string]Quote{
		"AAA": {Symbol: "AAA", Price: dec("15")},
	}, testDate("2026-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	if !valuation.TotalUSD.Equal(dec("40")) {
		t.Fatalf("total = %s, want 40", valuation.TotalUSD)
	}
}

func TestParseMoneyVariants(t *testing.T) {
	cases := map[string]string{
		"$1,234.50": "1234.5",
		"-$16.26":   "-16.26",
		"$ --":      "0",
		"-":         "0",
	}
	for input, want := range cases {
		if got := parseMoney(input); !got.Equal(dec(want)) {
			t.Errorf("parseMoney(%q) = %s, want %s", input, got, want)
		}
	}
	if !decimal.Zero.IsZero() {
		t.Fatal("decimal zero sanity check failed")
	}
}
