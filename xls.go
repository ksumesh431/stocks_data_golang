package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/extrame/xls"
)

func importOrders(path string) ([]Trade, error) {
	book, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	rows := book.ReadAllCells(100000)
	headerRow, columns, err := findHeader(rows, "Stock Symbol", "Transaction Type", "Quantity", "Price ($)")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var trades []Trade
	for rowIndex := headerRow + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		symbol := strings.ToUpper(rowValue(row, columns["stock symbol"]))
		side := strings.ToUpper(rowValue(row, columns["transaction type"]))
		if symbol == "" || (side != "BUY" && side != "SELL") {
			continue
		}
		dateValue := rowValue(row, columns["order execution time"])
		if dateValue == "" {
			if placedColumn, ok := columns["order placed time"]; ok {
				dateValue = rowValue(row, placedColumn)
			}
		}
		date, err := parseDate(dateValue)
		if err != nil {
			return nil, fmt.Errorf("%s:%d date: %w", path, rowIndex+1, err)
		}
		quantity, err := parseDecimal(rowValue(row, columns["quantity"]))
		if err != nil {
			return nil, fmt.Errorf("%s:%d quantity: %w", path, rowIndex+1, err)
		}
		price, err := parseDecimal(rowValue(row, columns["price ($)"]))
		if err != nil {
			return nil, fmt.Errorf("%s:%d price: %w", path, rowIndex+1, err)
		}
		amount, err := parseDecimal(rowValue(row, columns["order amount ($)"]))
		if err != nil {
			return nil, fmt.Errorf("%s:%d amount: %w", path, rowIndex+1, err)
		}
		brokerage, _ := parseDecimal(rowValue(row, columns["brokerage ($)"]))
		ref := rowValue(row, columns["broker reference id"])

		trades = append(trades, Trade{
			Date:      date,
			Symbol:    symbol,
			Side:      side,
			Ref:       ref,
			Quantity:  quantity.Abs(),
			Price:     price.Abs(),
			Amount:    amount.Abs(),
			Brokerage: brokerage.Abs(),
			Source:    fmt.Sprintf("%s:%d", filepath.Base(path), rowIndex+1),
		})
	}
	return trades, nil
}

func importHoldings(path string) (HoldingsReport, error) {
	book, err := xls.Open(path, "utf-8")
	if err != nil {
		return HoldingsReport{}, fmt.Errorf("open %s: %w", path, err)
	}
	rows := book.ReadAllCells(100000)

	var snapshotDate = monthEndFromFilename(path)
	for _, row := range rows {
		for index, cell := range row {
			if strings.EqualFold(strings.TrimSpace(cell), "Holdings as on") && index+1 < len(row) {
				if date, err := parseDate(row[index+1]); err == nil {
					snapshotDate = date
				}
			}
		}
	}
	if snapshotDate.IsZero() {
		return HoldingsReport{}, fmt.Errorf("%s: holdings date not found", path)
	}

	headerRow, columns, err := findHeader(rows, "Stock Symbol", "Quantity", "Avg. Price ($)")
	if err != nil {
		return HoldingsReport{}, fmt.Errorf("%s: %w", path, err)
	}
	var holdings []Holding
	for rowIndex := headerRow + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		symbol := strings.ToUpper(rowValue(row, columns["stock symbol"]))
		if symbol == "" || strings.HasPrefix(symbol, "*") {
			continue
		}
		quantity, err := parseDecimal(rowValue(row, columns["quantity"]))
		if err != nil {
			return HoldingsReport{}, fmt.Errorf("%s:%d quantity: %w", path, rowIndex+1, err)
		}
		average, err := parseDecimal(rowValue(row, columns["avg. price ($)"]))
		if err != nil {
			return HoldingsReport{}, fmt.Errorf("%s:%d average price: %w", path, rowIndex+1, err)
		}
		holdings = append(holdings, Holding{
			Symbol:   symbol,
			Quantity: quantity,
			AvgPrice: average,
			Source:   fmt.Sprintf("%s:%d", filepath.Base(path), rowIndex+1),
		})
	}
	return HoldingsReport{
		Date:     snapshotDate,
		Holdings: holdings,
		Source:   filepath.Base(path),
	}, nil
}

func monthEndFromFilename(path string) (result time.Time) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	parts := strings.Split(name, "-")
	for index := 0; index+2 < len(parts); index++ {
		if len(parts[index]) != 4 || len(parts[index+1]) != 2 || len(parts[index+2]) != 2 {
			continue
		}
		date, err := parseDate(strings.Join(parts[index:index+3], "-"))
		if err == nil {
			result = date
		}
	}
	return result
}
