package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func loadDataset(files ReportFiles) (Dataset, error) {
	var dataset Dataset

	for _, path := range files.LRS {
		flows, err := importLRS(path)
		if err != nil {
			dataset.warn("skipping LRS report %s: %v", path, err)
			continue
		}
		dataset.Flows = append(dataset.Flows, flows...)
	}
	dataset.Flows = deduplicateFlows(dataset.Flows)
	if len(dataset.Flows) == 0 {
		return dataset, fmt.Errorf("no external INR/USD flows found")
	}

	for _, path := range files.Orders {
		trades, err := importOrders(path)
		if err != nil {
			dataset.warn("skipping order report %s: %v", path, err)
			continue
		}
		dataset.Trades = append(dataset.Trades, trades...)
	}
	dataset.Trades = deduplicateTrades(dataset.Trades)
	if len(dataset.Trades) == 0 {
		dataset.warn("no trade reports were readable")
	}

	for _, path := range files.Wallet {
		entries, err := importWallet(path)
		if err != nil {
			dataset.warn("skipping wallet report %s: %v", path, err)
			continue
		}
		dataset.Wallet = append(dataset.Wallet, entries...)
	}
	if len(dataset.Wallet) == 0 {
		return dataset, fmt.Errorf("no wallet balance data found")
	}

	for _, path := range files.Holdings {
		report, err := importHoldings(path)
		if err != nil {
			dataset.warn("skipping holdings report %s: %v", path, err)
			continue
		}
		dataset.Holdings = append(dataset.Holdings, report)
	}
	if len(dataset.Holdings) == 0 {
		return dataset, fmt.Errorf("no holdings report found")
	}
	sort.Slice(dataset.Holdings, func(i, j int) bool {
		if dataset.Holdings[i].Date.Equal(dataset.Holdings[j].Date) {
			return dataset.Holdings[i].Source < dataset.Holdings[j].Source
		}
		return dataset.Holdings[i].Date.Before(dataset.Holdings[j].Date)
	})

	for _, path := range files.Statements {
		statement, err := importStatement(path)
		if err != nil {
			dataset.warn("skipping monthly statement %s: %v", path, err)
			continue
		}
		dataset.Statements = append(dataset.Statements, statement)
	}
	dataset.Statements = deduplicateStatements(dataset.Statements)

	dataset.Flows = sortedFlows(dataset.Flows)
	dataset.Trades = sortedTrades(dataset.Trades)
	dataset.Warnings = append(dataset.Warnings, validateDataset(&dataset)...)
	return dataset, nil
}

func deduplicateFlows(flows []ExternalFlow) []ExternalFlow {
	seen := make(map[string]bool, len(flows))
	result := make([]ExternalFlow, 0, len(flows))
	for _, flow := range flows {
		key := strings.Join([]string{
			flow.Date.Format("2006-01-02"),
			flow.Direction,
			flow.INR.Round(4).String(),
			flow.USD.Round(4).String(),
		}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, flow)
	}
	return result
}

func deduplicateTrades(trades []Trade) []Trade {
	byKey := make(map[string]Trade, len(trades))
	for _, trade := range trades {
		key := trade.Ref
		if key == "" {
			key = strings.Join([]string{
				trade.Date.Format("2006-01-02 15:04"),
				trade.Symbol,
				trade.Side,
				trade.Quantity.String(),
				trade.Amount.Round(4).String(),
			}, "|")
		}
		previous, exists := byKey[key]
		if !exists || !trade.Date.Before(previous.Date) {
			byKey[key] = trade
		}
	}
	result := make([]Trade, 0, len(byKey))
	for _, trade := range byKey {
		result = append(result, trade)
	}
	return result
}

func deduplicateStatements(statements []Statement) []Statement {
	byDate := make(map[string]Statement, len(statements))
	for _, statement := range statements {
		key := statement.Date.Format("2006-01")
		previous, exists := byDate[key]
		if !exists || len(statement.Holdings) > len(previous.Holdings) ||
			(len(statement.Holdings) == len(previous.Holdings) && statement.Source > previous.Source) {
			byDate[key] = statement
		}
	}
	result := make([]Statement, 0, len(byDate))
	for _, statement := range byDate {
		result = append(result, statement)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date.Before(result[j].Date) })
	return result
}

func sortedFlows(flows []ExternalFlow) []ExternalFlow {
	sort.Slice(flows, func(i, j int) bool {
		if flows[i].Date.Equal(flows[j].Date) {
			return flows[i].Source < flows[j].Source
		}
		return flows[i].Date.Before(flows[j].Date)
	})
	return flows
}

func sortedTrades(trades []Trade) []Trade {
	sort.Slice(trades, func(i, j int) bool {
		if trades[i].Date.Equal(trades[j].Date) {
			return trades[i].Ref < trades[j].Ref
		}
		return trades[i].Date.Before(trades[j].Date)
	})
	return trades
}

func latestHoldings(dataset Dataset) HoldingsReport {
	return dataset.Holdings[len(dataset.Holdings)-1]
}

func validateDataset(dataset *Dataset) []string {
	var warnings []string
	holdings := latestHoldings(*dataset)
	if holdings.Date.IsZero() {
		return append(warnings, "latest holdings snapshot has no date")
	}

	positions := make(map[string]decimal.Decimal)
	for _, trade := range dataset.Trades {
		if trade.Date.After(holdings.Date.AddDate(0, 0, 1)) {
			continue
		}
		switch trade.Side {
		case "BUY":
			positions[trade.Symbol] = positions[trade.Symbol].Add(trade.Quantity)
		case "SELL":
			positions[trade.Symbol] = positions[trade.Symbol].Sub(trade.Quantity)
		}
	}
	for _, holding := range holdings.Holdings {
		reconstructed := positions[holding.Symbol]
		difference := reconstructed.Sub(holding.Quantity).Abs()
		if difference.GreaterThan(decimal.NewFromFloat(0.000001)) {
			warnings = append(warnings, fmt.Sprintf(
				"position mismatch for %s: order reports %s, holdings report %s",
				holding.Symbol, reconstructed.StringFixed(9), holding.Quantity.StringFixed(9)))
		}
		delete(positions, holding.Symbol)
	}
	for symbol, quantity := range positions {
		if quantity.Abs().GreaterThan(decimal.NewFromFloat(0.000001)) {
			warnings = append(warnings, fmt.Sprintf(
				"order reports show %s shares of %s not present in latest holdings",
				quantity.StringFixed(9), symbol))
		}
	}

	var unmatched []string
	for _, flow := range dataset.Flows {
		if !hasMatchingWalletFlow(dataset.Wallet, flow) {
			unmatched = append(unmatched, flow.Source)
		}
	}
	if len(unmatched) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"unmatched external flows in wallet reports: %s", strings.Join(unmatched, ", ")))
	}
	return warnings
}

func hasMatchingWalletFlow(entries []WalletEntry, flow ExternalFlow) bool {
	wantedType := "DEPOSIT"
	if flow.Direction == "Withdrawal" {
		wantedType = "WITHDRAWAL"
	}
	for _, entry := range entries {
		if entry.Type != wantedType {
			continue
		}
		if entry.Date.Before(flow.Date.AddDate(0, 0, -7)) || entry.Date.After(flow.Date.AddDate(0, 0, 7)) {
			continue
		}
		if entry.Amount.Sub(flow.USD).Abs().LessThanOrEqual(decimal.NewFromFloat(0.01)) {
			return true
		}
	}
	return false
}

func walletBalanceAt(entries []WalletEntry, target time.Time) (WalletEntry, bool) {
	fileLatest := make(map[string]time.Time)
	for _, entry := range entries {
		if latest, ok := fileLatest[entry.File]; !ok || entry.Date.After(latest) {
			fileLatest[entry.File] = entry.Date
		}
	}
	var best WalletEntry
	found := false
	for _, entry := range entries {
		if entry.Date.After(target) {
			continue
		}
		if !found {
			best, found = entry, true
			continue
		}
		switch {
		case entry.Date.After(best.Date):
			best = entry
		case entry.Date.Equal(best.Date) && fileLatest[entry.File].After(fileLatest[best.File]):
			best = entry
		case entry.Date.Equal(best.Date) && fileLatest[entry.File].Equal(fileLatest[best.File]) && entry.Order < best.Order:
			best = entry
		}
	}
	return best, found
}
