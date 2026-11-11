package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var dateLayouts = []string{
	"2006-01-02",
	"2006-01",
	"02 Jan 2006, 03:04 PM",
	"02 Jan 2006, 03:04PM",
	"2 Jan 2006, 03:04 PM",
	"02 January 2006, 03:04 PM",
	"01/02/2006",
	"2 January 2006",
	"02 Jan 2006",
	"2 Jan 2006",
}

func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	for _, layout := range dateLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", value)
}

func parseDecimal(value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ",", "")
	value = strings.ReplaceAll(value, "$", "")
	value = strings.ReplaceAll(value, "₹", "")
	value = strings.ReplaceAll(value, "Â", "")
	value = strings.ReplaceAll(value, "%", "")
	if value == "" || value == "-" || value == "--" {
		return decimal.Zero, nil
	}
	negative := false
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
		negative = true
		value = strings.TrimSuffix(strings.TrimPrefix(value, "("), ")")
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid number %q", value)
	}
	result := decimal.NewFromFloat(number)
	if negative {
		result = result.Neg()
	}
	return result, nil
}

func parseMoney(value string) decimal.Decimal {
	result, err := parseDecimal(value)
	if err != nil {
		return decimal.Zero
	}
	return result
}

func normalizeHeader(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func rowValue(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func findHeader(rows [][]string, required ...string) (int, map[string]int, error) {
	for rowIndex, row := range rows {
		columns := make(map[string]int, len(row))
		for columnIndex, cell := range row {
			name := normalizeHeader(cell)
			if name == "" {
				continue
			}
			columns[name] = columnIndex
			alias := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "($)", ""), " (usd)", ""))
			if alias != "" && alias != name {
				if _, exists := columns[alias]; !exists {
					columns[alias] = columnIndex
				}
			}
		}
		matches := true
		for _, name := range required {
			if _, ok := columns[normalizeHeader(name)]; !ok {
				matches = false
				break
			}
		}
		if matches {
			return rowIndex, columns, nil
		}
	}
	return -1, nil, fmt.Errorf("header not found: %s", strings.Join(required, ", "))
}

func lastDayOfMonth(year int, month time.Month) time.Time {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
}

func monthEnd(value time.Time) time.Time {
	return lastDayOfMonth(value.Year(), value.Month())
}
