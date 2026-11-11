package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/shopspring/decimal"
)

func printSummary(writer io.Writer, dataset Dataset, valuation Valuation, performance Performance, warnings []string) {
	fmt.Fprintf(writer, "Portfolio performance as of %s\n", valuation.AsOf.Format("2006-01-02"))
	fmt.Fprintf(writer, "Stock value:  $%s\n", money(valuation.StockUSD))
	fmt.Fprintf(writer, "Wallet cash:  $%s (balance date %s)\n", money(valuation.WalletUSD), valuation.WalletOn.Format("2006-01-02"))
	fmt.Fprintf(writer, "Total value:  $%s  |  INR %s at USD/INR %s\n\n",
		money(valuation.TotalUSD), money(performance.ValueINR), performance.USDINR.StringFixed(4))

	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "INR return\tAmount")
	fmt.Fprintf(table, "Contributions (including fees)\t%s\n", money(performance.DepositsINR))
	fmt.Fprintf(table, "Withdrawals (net of fees)\t%s\n", money(performance.WithdrawalsINR))
	fmt.Fprintf(table, "Charges included\t%s\n", money(performance.ChargesINR))
	if performance.TCSINR.IsPositive() {
		fmt.Fprintf(table, "TCS (tracked, not deducted)\t%s\n", money(performance.TCSINR))
	}
	fmt.Fprintf(table, "Net invested\t%s\n", money(performance.NetInvestedINR))
	fmt.Fprintf(table, "Current value\t%s\n", money(performance.ValueINR))
	fmt.Fprintf(table, "Absolute profit\t%s\n", money(performance.ProfitINR))
	if performance.NetInvestedINR.IsPositive() {
		fmt.Fprintf(table, "Simple return\t%s\n", percent(performance.SimpleRate))
	}
	if performance.INRXIRRErr == "" {
		fmt.Fprintf(table, "INR XIRR\t%s\n", percent(performance.INRXIRR))
	} else {
		fmt.Fprintf(table, "INR XIRR\tunavailable: %s\n", performance.INRXIRRErr)
	}
	fmt.Fprintln(table)

	fmt.Fprintln(table, "USD return\tAmount")
	fmt.Fprintf(table, "Deposits\t$%s\n", money(performance.DepositsUSD))
	fmt.Fprintf(table, "Withdrawals\t$%s\n", money(performance.WithdrawalsUSD))
	fmt.Fprintf(table, "Current value\t$%s\n", money(performance.ValueUSD))
	fmt.Fprintf(table, "Absolute profit\t$%s\n", money(performance.ProfitUSD))
	if performance.USDXIRRErr == "" {
		fmt.Fprintf(table, "USD XIRR\t%s\n", percent(performance.USDXIRR))
	} else {
		fmt.Fprintf(table, "USD XIRR\tunavailable: %s\n", performance.USDXIRRErr)
	}
	fmt.Fprintln(table)

	if performance.ReferenceRate.IsPositive() {
		fmt.Fprintln(table, "FX effect\tAmount")
		fmt.Fprintf(table, "Reference USD/INR\t%s\n", performance.ReferenceRate.StringFixed(4))
		fmt.Fprintf(table, "Profit at constant FX\t%s\n", money(performance.ConstantFXProfit))
		fmt.Fprintf(table, "FX and conversion impact\t%s\n", money(performance.FXImpact))
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(writer, "output error: %v\n", err)
	}

	if len(valuation.Holdings) > 0 {
		fmt.Fprintln(writer, "\nHoldings")
		holdingTable := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
		fmt.Fprintln(holdingTable, "Symbol\tQuantity\tAvg cost\tMarket price\tCost basis\tMarket value\tUnrealized")
		for _, holding := range valuation.Holdings {
			fmt.Fprintf(holdingTable, "%s\t%s\t$%s\t$%s\t$%s\t$%s\t$%s\n",
				holding.Symbol,
				quantity(holding.Quantity),
				money(holding.AverageCost),
				money(holding.MarketPrice),
				money(holding.CostBasis),
				money(holding.MarketValue),
				money(holding.Unrealized),
			)
		}
		if err := holdingTable.Flush(); err != nil {
			fmt.Fprintf(writer, "output error: %v\n", err)
		}
	}

	if len(warnings) > 0 {
		fmt.Fprintln(writer, "\nWarnings")
		for _, warning := range warnings {
			fmt.Fprintf(writer, "- %s\n", warning)
		}
	}

	fmt.Fprintf(writer, "\nReports: %d external flows, %d unique trades, %d wallet rows, %d statement(s)\n",
		len(dataset.Flows), len(dataset.Trades), len(dataset.Wallet), len(dataset.Statements))
}

func printHistory(writer io.Writer, points []HistoryPoint) {
	if len(points) == 0 {
		fmt.Fprintln(writer, "\nNo historical statement points available.")
		return
	}
	fmt.Fprintln(writer, "\nHistory")
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "Month end\tValue USD\tProfit USD\tValue INR\tProfit INR\tINR XIRR")
	for _, point := range points {
		valueINR := "-"
		profitINR := "-"
		xirrValue := "-"
		if !point.FXMissing {
			valueINR = money(point.ValueINR)
			profitINR = money(point.Performance.ProfitINR)
			if point.Performance.INRXIRRErr == "" {
				xirrValue = percent(point.Performance.INRXIRR)
			}
		}
		fmt.Fprintf(table, "%s\t$%s\t$%s\t%s\t%s\t%s\n",
			point.Date.Format("2006-01-02"),
			money(point.ValueUSD),
			money(point.Performance.ProfitUSD),
			valueINR,
			profitINR,
			xirrValue,
		)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(writer, "output error: %v\n", err)
	}
}

func writeHoldingsCSV(path string, valuation Valuation) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{
		"symbol", "quantity", "average_cost_usd", "market_price_usd",
		"cost_basis_usd", "market_value_usd", "unrealized_usd",
	}); err != nil {
		return err
	}
	for _, holding := range valuation.Holdings {
		if err := writer.Write([]string{
			holding.Symbol,
			holding.Quantity.String(),
			money(holding.AverageCost),
			money(holding.MarketPrice),
			money(holding.CostBasis),
			money(holding.MarketValue),
			money(holding.Unrealized),
		}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writeHistoryCSV(path string, points []HistoryPoint) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{
		"month_end", "value_usd", "profit_usd", "value_inr", "profit_inr", "inr_xirr",
	}); err != nil {
		return err
	}
	for _, point := range points {
		valueINR, profitINR, inrXIRR := "", "", ""
		if !point.FXMissing {
			valueINR = money(point.ValueINR)
			profitINR = money(point.Performance.ProfitINR)
			if point.Performance.INRXIRRErr == "" {
				inrXIRR = percent(point.Performance.INRXIRR)
			}
		}
		if err := writer.Write([]string{
			point.Date.Format("2006-01-02"),
			money(point.ValueUSD),
			money(point.Performance.ProfitUSD),
			valueINR,
			profitINR,
			inrXIRR,
		}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func money(value decimal.Decimal) string {
	return value.Round(2).StringFixed(2)
}

func quantity(value decimal.Decimal) string {
	result := value.Round(9).String()
	result = strings.TrimRight(result, "0")
	result = strings.TrimRight(result, ".")
	return result
}

func percent(value decimal.Decimal) string {
	return value.Mul(decimal.NewFromInt(100)).Round(2).StringFixed(2) + "%"
}

func summarizeQuoteTimes(quotes map[string]Quote) (time.Time, []string) {
	var latest time.Time
	sources := make(map[string]bool)
	for _, quote := range quotes {
		if quote.AsOf.After(latest) {
			latest = quote.AsOf
		}
		sources[quote.Source] = true
	}
	var names []string
	for source := range sources {
		names = append(names, source)
	}
	sort.Strings(names)
	return latest, names
}
