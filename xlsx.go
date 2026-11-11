package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

func importLRS(path string) ([]ExternalFlow, error) {
	book, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer book.Close()

	var flows []ExternalFlow
	for _, sheet := range book.GetSheetList() {
		rows, err := book.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("read %s sheet %s: %w", path, sheet, err)
		}
		headerRow, columns, err := findHeader(rows, "Transaction Date", "Money Movement")
		if err != nil {
			continue
		}
		for rowIndex := headerRow + 1; rowIndex < len(rows); rowIndex++ {
			row := rows[rowIndex]
			dateValue := rowValue(row, columns["transaction date"])
			if dateValue == "" {
				continue
			}
			date, err := parseDate(dateValue)
			if err != nil {
				continue
			}
			movement := rowValue(row, columns["money movement"])
			direction, ok := parseDirection(movement)
			if !ok {
				continue
			}
			inr, err := parseDecimal(rowValue(row, columns["inr amount"]))
			if err != nil {
				return nil, fmt.Errorf("%s:%d INR amount: %w", path, rowIndex+1, err)
			}
			usd, err := parseDecimal(rowValue(row, columns["usd amount"]))
			if err != nil {
				return nil, fmt.Errorf("%s:%d USD amount: %w", path, rowIndex+1, err)
			}
			gst, _ := parseDecimal(rowValue(row, columns["gst charged"]))
			bankFee, _ := parseDecimal(rowValue(row, columns["bank fixed fee"]))
			tcs, _ := parseDecimal(rowValue(row, columns["tcs charged(indicative)"]))
			rate, _ := parseDecimal(rowValue(row, columns["exchange rate"]))

			flows = append(flows, ExternalFlow{
				Date:      date,
				Direction: direction,
				INR:       inr.Abs(),
				USD:       usd.Abs(),
				GST:       gst.Abs(),
				BankFee:   bankFee.Abs(),
				TCS:       tcs.Abs(),
				Rate:      rate.Abs(),
				Source:    fmt.Sprintf("%s:%d", filepath.Base(path), rowIndex+1),
			})
		}
	}
	return flows, nil
}

func importWallet(path string) ([]WalletEntry, error) {
	book, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer book.Close()

	var entries []WalletEntry
	for _, sheet := range book.GetSheetList() {
		rows, err := book.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("read %s sheet %s: %w", path, sheet, err)
		}
		headerRow, columns, err := findHeader(rows, "Date", "Type", "Updated Trade Balance")
		if err != nil {
			continue
		}
		order := 0
		for rowIndex := headerRow + 1; rowIndex < len(rows); rowIndex++ {
			row := rows[rowIndex]
			dateValue := rowValue(row, columns["date"])
			if dateValue == "" {
				continue
			}
			date, err := parseDate(dateValue)
			if err != nil {
				continue
			}
			entryType := strings.ToUpper(rowValue(row, columns["type"]))
			if entryType == "" {
				continue
			}
			amount, _ := parseDecimal(rowValue(row, columns["amount"]))
			quantity, _ := parseDecimal(rowValue(row, columns["quantity"]))
			price, _ := parseDecimal(rowValue(row, columns["price"]))
			commission, _ := parseDecimal(rowValue(row, columns["commission"]))
			tradeBalance, err := parseDecimal(rowValue(row, columns["updated trade balance"]))
			if err != nil {
				return nil, fmt.Errorf("%s:%d trade balance: %w", path, rowIndex+1, err)
			}
			entries = append(entries, WalletEntry{
				Date:         date,
				Type:         entryType,
				Description:  rowValue(row, columns["description"]),
				Movement:     rowValue(row, columns["money movement"]),
				Symbol:       strings.ToUpper(rowValue(row, columns["ticker"])),
				Amount:       amount.Abs(),
				Quantity:     quantity,
				Price:        price,
				Commission:   commission.Abs(),
				TradeBalance: tradeBalance,
				Source:       fmt.Sprintf("%s:%d", filepath.Base(path), rowIndex+1),
				Order:        order,
				File:         path,
			})
			order++
		}
	}
	return entries, nil
}

func parseDirection(value string) (string, bool) {
	switch {
	case strings.Contains(strings.ToLower(value), "withdraw"):
		return "Withdrawal", true
	case strings.Contains(strings.ToLower(value), "deposit"):
		return "Deposit", true
	default:
		return "", false
	}
}

func effectiveRate(flow ExternalFlow) decimal.Decimal {
	if flow.Rate.IsPositive() {
		return flow.Rate
	}
	if flow.USD.IsPositive() {
		return flow.INR.Div(flow.USD)
	}
	return decimal.Zero
}
