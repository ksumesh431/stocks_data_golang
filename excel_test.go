package main

import (
	"archive/zip"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

func TestWriteExcel(t *testing.T) {
	valuation := Valuation{
		AsOf: testDate("2026-11-11"),
		Holdings: []HoldingValue{
			{
				Symbol: "AAA", Quantity: dec("2"), AverageCost: dec("10"),
				MarketPrice: dec("15"), CostBasis: dec("20"), MarketValue: dec("30"),
				Unrealized: dec("10"),
			},
			{
				Symbol: "BBB", Quantity: dec("1"), AverageCost: dec("20"),
				MarketPrice: dec("25"), CostBasis: dec("20"), MarketValue: dec("25"),
				Unrealized: dec("5"),
			},
		},
		StockUSD:  dec("55"),
		WalletUSD: dec("10"),
		TotalUSD:  dec("65"),
		WalletOn:  testDate("2026-11-10"),
	}
	performance := Performance{
		AsOf:           testDate("2026-11-11"),
		DepositsINR:    dec("5000"),
		WithdrawalsINR: dec("500"),
		ChargesINR:     dec("125"),
		TCSINR:         dec("50"),
		NetInvestedINR: dec("4500"),
		DepositsUSD:    dec("60"),
		WithdrawalsUSD: dec("5"),
		NetInvestedUSD: dec("55"),
		ValueINR:       dec("5460"),
		USDINR:         dec("84"),
		ValueUSD:       dec("65"),
		ProfitINR:      dec("960"),
		ProfitUSD:      dec("10"),
		SimpleRate:     dec("0.2133"),
		SimpleRateUSD:  dec("0.1818"),
		YTD: PeriodPerformance{
			StartDate:      testDate("2025-12-31"),
			StartValueUSD:  dec("50"),
			DepositsUSD:    dec("5"),
			WithdrawalsUSD: dec("2"),
			ProfitUSD:      dec("12"),
			Return:         dec("0.23"),
		},
		INRXIRR:          dec("0.15"),
		USDXIRR:          dec("0.12"),
		ReferenceRate:    dec("83.3333"),
		ConstantFXProfit: dec("708.33"),
		FXImpact:         dec("251.67"),
	}
	points := []HistoryPoint{
		{
			Date:     testDate("2026-10-31"),
			ValueUSD: dec("60"),
			ValueINR: dec("5040"),
			Performance: Performance{
				ProfitUSD: dec("5"),
				ProfitINR: dec("420"),
			},
		},
	}
	warnings := []string{"synthetic warning"}
	info := excelInfo{
		Generated:    time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
		QuoteTime:    time.Date(2026, 11, 11, 11, 0, 0, 0, time.UTC),
		QuoteSources: []string{"manual prices file"},
		FXTime:       time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC),
	}
	path := filepath.Join(t.TempDir(), "report.xlsx")
	if err := writeExcel(path, Dataset{Flows: make([]ExternalFlow, 17)}, valuation, performance, points, warnings, info); err != nil {
		t.Fatal(err)
	}

	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()

	for _, sheet := range []string{summarySheet, holdingsSheet, historySheet, warningsSheet} {
		found := false
		for _, name := range book.GetSheetList() {
			if name == sheet {
				found = true
			}
		}
		if !found {
			t.Fatalf("sheet %s missing from %v", sheet, book.GetSheetList())
		}
	}

	if got := cellNumber(t, book, summarySheet, "B8"); math.Abs(got-65) > 0.001 {
		t.Fatalf("summary total USD = %v, want 65", got)
	}
	if got := cellNumber(t, book, summarySheet, "B9"); math.Abs(got-5460) > 0.001 {
		t.Fatalf("summary total INR = %v, want 5460", got)
	}
	if got := cellNumber(t, book, summarySheet, "B21"); math.Abs(got-0.15) > 0.0001 {
		t.Fatalf("summary INR XIRR = %v, want 0.15", got)
	}
	if got := cellNumber(t, book, summarySheet, "B26"); math.Abs(got-55) > 0.001 {
		t.Fatalf("summary net invested USD = %v, want 55", got)
	}
	if got := cellNumber(t, book, summarySheet, "B29"); math.Abs(got-0.1818) > 0.0001 {
		t.Fatalf("summary cumulative USD return = %v, want 0.1818", got)
	}
	if got := cellNumber(t, book, summarySheet, "B30"); math.Abs(got-0.12) > 0.0001 {
		t.Fatalf("summary USD XIRR = %v, want 0.12", got)
	}
	if got := cellNumber(t, book, summarySheet, "B37"); math.Abs(got-12) > 0.001 {
		t.Fatalf("summary YTD gain USD = %v, want 12", got)
	}
	if got := cellNumber(t, book, summarySheet, "B38"); math.Abs(got-0.23) > 0.0001 {
		t.Fatalf("summary YTD USD return = %v, want 0.23", got)
	}
	if got, err := book.GetCellValue(summarySheet, "A32"); err != nil || got != "2026 year to date (USD)" {
		t.Fatalf("YTD section = %q, %v", got, err)
	}
	if got, err := book.GetCellValue(holdingsSheet, "A5"); err != nil || got != "AAA" {
		t.Fatalf("first holding = %q, %v", got, err)
	}
	if got := cellNumber(t, book, holdingsSheet, "F5"); math.Abs(got-30) > 0.001 {
		t.Fatalf("first holding market value = %v, want 30", got)
	}
	if got := cellNumber(t, book, historySheet, "B4"); math.Abs(got-60) > 0.001 {
		t.Fatalf("history value = %v, want 60", got)
	}
	if got, err := book.GetCellValue(warningsSheet, "A3"); err != nil || got != "synthetic warning" {
		t.Fatalf("warning cell = %q, %v", got, err)
	}

	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	charts := 0
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "xl/charts/chart") {
			charts++
		}
	}
	if charts != 6 {
		t.Fatalf("chart parts = %d, want 6", charts)
	}
}

func TestWriteExcelWithoutHistoryOrWarnings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.xlsx")
	valuation := Valuation{
		AsOf:     testDate("2026-11-11"),
		StockUSD: decimal.Zero,
		TotalUSD: decimal.Zero,
		WalletOn: testDate("2026-11-10"),
	}
	performance := Performance{USDINR: dec("84")}
	if err := writeExcel(path, Dataset{}, valuation, performance, nil, nil, excelInfo{
		Generated: time.Now(),
		FXTime:    time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got, err := book.GetCellValue(warningsSheet, "A3"); err != nil || !strings.Contains(got, "No warnings") {
		t.Fatalf("empty warning sheet = %q, %v", got, err)
	}
	if got, err := book.GetCellValue(historySheet, "A2"); err != nil || !strings.Contains(got, "No monthly statement") {
		t.Fatalf("empty history sheet = %q, %v", got, err)
	}
}

func cellNumber(t *testing.T, book *excelize.File, sheet, cell string) float64 {
	t.Helper()
	value, err := book.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		t.Fatalf("%s!%s = %q: %v", sheet, cell, value, err)
	}
	return number
}
