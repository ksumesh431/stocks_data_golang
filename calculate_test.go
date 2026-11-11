package main

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func testDate(value string) time.Time {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return date
}

func dec(value string) decimal.Decimal {
	result, err := decimal.NewFromString(value)
	if err != nil {
		panic(err)
	}
	return result
}

func TestProfitWithReinvestmentAndWithdrawal(t *testing.T) {
	dataset := Dataset{
		Flows: []ExternalFlow{
			{Date: testDate("2025-01-01"), Direction: "Deposit", INR: dec("8600"), USD: dec("100"), Rate: dec("86")},
			{Date: testDate("2025-06-01"), Direction: "Withdrawal", INR: dec("1720"), USD: dec("20"), Rate: dec("86")},
		},
		Trades: []Trade{
			{Date: testDate("2025-02-01"), Symbol: "AAA", Side: "BUY", Quantity: dec("10")},
			{Date: testDate("2025-04-01"), Symbol: "AAA", Side: "SELL", Quantity: dec("5")},
			{Date: testDate("2025-04-01"), Symbol: "BBB", Side: "BUY", Quantity: dec("5")},
		},
	}
	performance := calculatePerformance(dataset, dec("150"), dec("100"), testDate("2026-01-01"))

	if !performance.ProfitUSD.Equal(dec("70")) {
		t.Fatalf("USD profit = %s, want 70", performance.ProfitUSD)
	}
	if !performance.ProfitINR.Equal(dec("8120")) {
		t.Fatalf("INR profit = %s, want 8120", performance.ProfitINR)
	}
	if !performance.NetInvestedINR.Equal(dec("6880")) {
		t.Fatalf("net invested INR = %s, want 6880", performance.NetInvestedINR)
	}
	if !performance.NetInvestedUSD.Equal(dec("80")) {
		t.Fatalf("net invested USD = %s, want 80", performance.NetInvestedUSD)
	}
	if !performance.SimpleRateUSD.Equal(dec("70").Div(dec("80"))) {
		t.Fatalf("cumulative USD return = %s, want 0.875", performance.SimpleRateUSD)
	}
	if performance.INRXIRRErr != "" {
		t.Fatalf("INR XIRR error: %s", performance.INRXIRRErr)
	}
	if performance.USDXIRRErr != "" {
		t.Fatalf("USD XIRR error: %s", performance.USDXIRRErr)
	}
}

func TestCurrencyMovementChangesINRNotUSD(t *testing.T) {
	dataset := Dataset{
		Flows: []ExternalFlow{
			{Date: testDate("2025-01-01"), Direction: "Deposit", INR: dec("5000"), USD: dec("50"), Rate: dec("100")},
		},
	}
	at100 := calculatePerformance(dataset, dec("50"), dec("100"), testDate("2026-01-01"))
	at110 := calculatePerformance(dataset, dec("50"), dec("110"), testDate("2026-01-01"))

	if !at100.ProfitUSD.Equal(at110.ProfitUSD) {
		t.Fatalf("USD profit changed with FX: %s vs %s", at100.ProfitUSD, at110.ProfitUSD)
	}
	if !at100.ProfitINR.IsZero() {
		t.Fatalf("INR profit at constant FX = %s, want 0", at100.ProfitINR)
	}
	if !at110.ProfitINR.Equal(dec("500")) {
		t.Fatalf("INR profit after appreciation = %s, want 500", at110.ProfitINR)
	}
	if !at110.FXImpact.Equal(dec("500")) {
		t.Fatalf("FX impact = %s, want 500", at110.FXImpact)
	}
}

func TestFeesIncludedButTCSNot(t *testing.T) {
	dataset := Dataset{
		Flows: []ExternalFlow{
			{
				Date: testDate("2025-01-01"), Direction: "Deposit",
				INR: dec("10000"), USD: dec("100"), GST: dec("500"),
				BankFee: dec("50"), TCS: dec("100"), Rate: dec("100"),
			},
			{
				Date: testDate("2025-06-01"), Direction: "Withdrawal",
				INR: dec("2000"), USD: dec("20"), GST: dec("100"), BankFee: dec("10"),
			},
		},
	}
	performance := calculatePerformance(dataset, dec("150"), dec("100"), testDate("2026-01-01"))

	if !performance.DepositsINR.Equal(dec("10550")) {
		t.Fatalf("deposits = %s, want 10550", performance.DepositsINR)
	}
	if !performance.WithdrawalsINR.Equal(dec("1890")) {
		t.Fatalf("withdrawals = %s, want 1890", performance.WithdrawalsINR)
	}
	if !performance.ChargesINR.Equal(dec("660")) {
		t.Fatalf("charges = %s, want 660", performance.ChargesINR)
	}
	if !performance.TCSINR.Equal(dec("100")) {
		t.Fatalf("TCS = %s, want 100", performance.TCSINR)
	}
	if !performance.ProfitINR.Equal(dec("6340")) {
		t.Fatalf("profit = %s, want 6340", performance.ProfitINR)
	}
	if !performance.FXImpact.IsZero() {
		t.Fatalf("FX impact = %s, want 0", performance.FXImpact)
	}
}

func TestCalculateYearToDateUSD(t *testing.T) {
	decemberStatement := Statement{Date: testDate("2025-12-31"), LongValue: dec("100")}
	wallet := []WalletEntry{{Date: testDate("2025-12-31"), TradeBalance: decimal.Zero, File: "wallet"}}

	t.Run("times cash flows and excludes prior years", func(t *testing.T) {
		dataset := Dataset{
			Statements: []Statement{decemberStatement},
			Wallet:     wallet,
			Flows: []ExternalFlow{
				{Date: testDate("2025-01-01"), Direction: "Deposit", USD: dec("1000")},
				{Date: testDate("2026-01-06"), Direction: "Deposit", USD: dec("20")},
				{Date: testDate("2026-01-11"), Direction: "Withdrawal", USD: dec("4")},
			},
		}
		got := calculateYearToDateUSD(dataset, dec("125"), testDate("2026-01-11"))
		if got.Err != "" {
			t.Fatal(got.Err)
		}
		if !got.StartDate.Equal(testDate("2025-12-31")) || !got.StartValueUSD.Equal(dec("100")) {
			t.Fatalf("starting point = %s, $%s; want 2025-12-31, $100", got.StartDate, got.StartValueUSD)
		}
		if !got.DepositsUSD.Equal(dec("20")) || !got.WithdrawalsUSD.Equal(dec("4")) {
			t.Fatalf("YTD flows = $%s deposits, $%s withdrawals; want $20, $4", got.DepositsUSD, got.WithdrawalsUSD)
		}
		if !got.ProfitUSD.Equal(dec("9")) {
			t.Fatalf("YTD gain = %s, want 9", got.ProfitUSD)
		}
		wantReturn := dec("9").Div(dec("110"))
		if !got.Return.Equal(wantReturn) {
			t.Fatalf("YTD return = %s, want %s", got.Return, wantReturn)
		}
	})

	tests := []struct {
		name      string
		dataset   Dataset
		errorText string
	}{
		{
			name: "missing December statement",
			dataset: Dataset{
				Statements: []Statement{{Date: testDate("2025-11-30"), LongValue: dec("100")}},
				Wallet:     wallet,
			},
			errorText: "December 2025",
		},
		{
			name:      "missing starting wallet balance",
			dataset:   Dataset{Statements: []Statement{decemberStatement}},
			errorText: "no wallet balance",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := calculateYearToDateUSD(test.dataset, dec("125"), testDate("2026-01-11"))
			if !strings.Contains(got.Err, test.errorText) {
				t.Fatalf("error = %q, want text %q", got.Err, test.errorText)
			}
		})
	}
}

func TestXIRRKnownRate(t *testing.T) {
	rate, err := xirr([]cashFlow{
		{Date: testDate("2025-01-01"), Amount: -100},
		{Date: testDate("2026-01-01"), Amount: 110},
	})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(rate-0.10) > 0.000001 {
		t.Fatalf("XIRR = %f, want 0.10", rate)
	}
}

func TestXIRRRequiresBothSigns(t *testing.T) {
	if _, err := xirr([]cashFlow{
		{Date: testDate("2025-01-01"), Amount: 100},
		{Date: testDate("2026-01-01"), Amount: 110},
	}); err == nil {
		t.Fatal("expected an error for cash flows with one sign")
	}
}

func TestLoss(t *testing.T) {
	dataset := Dataset{
		Flows: []ExternalFlow{
			{Date: testDate("2025-01-01"), Direction: "Deposit", INR: dec("100"), USD: dec("2"), Rate: dec("50")},
		},
	}
	performance := calculatePerformance(dataset, dec("1.5"), dec("50"), testDate("2025-10-01"))
	if !performance.ProfitUSD.Equal(dec("-0.5")) {
		t.Fatalf("USD profit = %s, want -0.5", performance.ProfitUSD)
	}
	if !performance.ProfitINR.Equal(dec("-25")) {
		t.Fatalf("INR profit = %s, want -25", performance.ProfitINR)
	}
}
