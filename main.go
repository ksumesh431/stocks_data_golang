package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	reportsDir := flag.String("reports", "./Reports", "directory containing report subdirectories")
	pricesFile := flag.String("prices", "", "optional CSV with Symbol,Price columns")
	usdInrFlag := flag.String("usdinr", "", "optional manual USD/INR rate")
	csvFile := flag.String("csv", "", "optional holdings CSV output path")
	historyCSV := flag.String("history-csv", "", "optional history CSV output path")
	excelFile := flag.String("excel", "portfolio_report.xlsx", "Excel report output path (empty to skip)")
	showHistory := flag.Bool("history", false, "show month-end history from PDF statements")
	flag.Parse()

	files, err := discoverReports(*reportsDir)
	if err != nil {
		return err
	}
	dataset, err := loadDataset(files)
	if err != nil {
		return err
	}
	holdings := latestHoldings(dataset)

	symbols := make([]string, 0, len(holdings.Holdings))
	for _, holding := range holdings.Holdings {
		if holding.Quantity.IsPositive() {
			symbols = append(symbols, holding.Symbol)
		}
	}
	quotes, quoteWarnings, err := resolveQuotes(symbols, *pricesFile)
	if err != nil {
		return err
	}

	manualRate, hasManualRate, err := parseOptionalDecimal(*usdInrFlag)
	if err != nil {
		return fmt.Errorf("invalid -usdinr: %w", err)
	}
	var usdInr decimal.Decimal
	var fxTime time.Time
	if hasManualRate {
		if !manualRate.IsPositive() {
			return fmt.Errorf("-usdinr must be positive")
		}
		usdInr = manualRate
		fxTime = time.Now().UTC()
	} else {
		usdInr, fxTime, err = fetchUSDINR()
		if err != nil {
			return fmt.Errorf("fetch USD/INR (or pass -usdinr): %w", err)
		}
	}

	asOf := time.Now().UTC()
	valuation, err := buildValuation(dataset, holdings, quotes, asOf)
	if err != nil {
		return err
	}
	performance := calculatePerformance(dataset, valuation.TotalUSD, usdInr, asOf)
	performance.YTD = calculateYearToDateUSD(dataset, valuation.TotalUSD, asOf)

	warnings := append([]string(nil), dataset.Warnings...)
	warnings = append(warnings, quoteWarnings...)
	if asOf.Sub(valuation.WalletOn) > 7*24*time.Hour {
		warnings = append(warnings, fmt.Sprintf(
			"wallet balance is dated %s; download a newer running-balance report",
			valuation.WalletOn.Format("2006-01-02")))
	}
	if asOf.Sub(holdings.Date) > 7*24*time.Hour {
		warnings = append(warnings, fmt.Sprintf(
			"holdings snapshot is dated %s; download a newer holdings report",
			holdings.Date.Format("2006-01-02")))
	}
	quoteTime, quoteSources := summarizeQuoteTimes(quotes)
	if !quoteTime.IsZero() && asOf.Sub(quoteTime) > 7*24*time.Hour {
		warnings = append(warnings, fmt.Sprintf(
			"latest stock quote is dated %s; prices may be stale",
			quoteTime.Format("2006-01-02")))
	}
	if !quoteTime.IsZero() {
		fmt.Printf("Prices from %s, latest quote time %s\n",
			strings.Join(quoteSources, ", "), quoteTime.Format(time.RFC3339))
	}
	fmt.Printf("USD/INR %s at %s\n\n", usdInr.StringFixed(4), fxTime.Format(time.RFC3339))
	printSummary(os.Stdout, dataset, valuation, performance, warnings)

	if *csvFile != "" {
		if err := writeHoldingsCSV(*csvFile, valuation); err != nil {
			return fmt.Errorf("write holdings CSV: %w", err)
		}
		fmt.Printf("\nHoldings CSV written to %s\n", *csvFile)
	}

	var historyPoints []HistoryPoint
	excelWarnings := warnings
	if *showHistory || *historyCSV != "" {
		historyWarnings := append([]string(nil), warnings...)
		fxRates := fetchHistoricalUSDINRMap(dataset.Statements, &historyWarnings)
		points := calculateHistory(dataset, fxRates)
		historyPoints = points
		excelWarnings = historyWarnings
		printHistory(os.Stdout, points)
		if len(historyWarnings) > len(warnings) {
			fmt.Fprintln(os.Stdout, "\nHistory warnings")
			for _, warning := range historyWarnings[len(warnings):] {
				fmt.Printf("- %s\n", warning)
			}
		}
		if *historyCSV != "" {
			if err := writeHistoryCSV(*historyCSV, points); err != nil {
				return fmt.Errorf("write history CSV: %w", err)
			}
			fmt.Printf("\nHistory CSV written to %s\n", *historyCSV)
		}
	}

	if *excelFile != "" {
		if historyPoints == nil {
			historyPoints = calculateHistory(dataset, nil)
		}
		info := excelInfo{
			Generated:    time.Now().UTC(),
			QuoteTime:    quoteTime,
			QuoteSources: quoteSources,
			FXTime:       fxTime,
		}
		if err := writeExcel(*excelFile, dataset, valuation, performance, historyPoints, excelWarnings, info); err != nil {
			return err
		}
		fmt.Printf("\nExcel report written to %s\n", *excelFile)
	}
	return nil
}
