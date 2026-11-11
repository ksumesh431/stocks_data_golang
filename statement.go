package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/shopspring/decimal"
)

var (
	periodPattern = regexp.MustCompile(`(?i)\b(january|february|march|april|may|june|july|august|september|october|november|december)\b[^0-9]{0,10}(20[0-9]{2})`)
	symbolPattern = regexp.MustCompile(`^[A-Z][A-Z0-9.-]{0,9}$`)
)

var statementMonths = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June,
	"july": time.July, "august": time.August, "september": time.September,
	"october": time.October, "november": time.November, "december": time.December,
}

func importStatement(path string) (Statement, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return Statement{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	var lines []string
	for pageNumber := 1; pageNumber <= reader.NumPage(); pageNumber++ {
		pageText, err := reader.Page(pageNumber).GetPlainText(nil)
		if err != nil {
			return Statement{}, fmt.Errorf("%s page %d: %w", path, pageNumber, err)
		}
		for _, line := range strings.Split(pageText, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
	}

	date := statementPeriod(lines)
	if date.IsZero() {
		date = statementDateFromFilename(path)
	}
	if date.IsZero() {
		return Statement{}, fmt.Errorf("%s: statement period not found", path)
	}
	holdings, err := statementHoldings(lines)
	if err != nil {
		return Statement{}, fmt.Errorf("%s: %w", path, err)
	}
	longValue := decimal.Zero
	for _, holding := range holdings {
		longValue = longValue.Add(holding.MarketVal)
	}
	return Statement{
		Date:      monthEnd(date),
		Holdings:  holdings,
		LongValue: longValue,
		Source:    path,
	}, nil
}

func statementPeriod(lines []string) time.Time {
	limit := len(lines)
	if limit > 60 {
		limit = 60
	}
	for _, line := range lines[:limit] {
		match := periodPattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		month := statementMonths[strings.ToLower(match[1])]
		var year int
		if _, err := fmt.Sscanf(match[2], "%d", &year); err != nil {
			continue
		}
		return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Time{}
}

func statementDateFromFilename(path string) time.Time {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if len(name) >= 7 {
		date, err := parseDate(name[len(name)-7:])
		if err == nil {
			return date
		}
	}
	return time.Time{}
}

func statementHoldings(lines []string) ([]StatementHolding, error) {
	headerIndex := -1
	for index, line := range lines {
		if line == "Symbol" && index+7 < len(lines) &&
			lines[index+1] == "Description" &&
			lines[index+2] == "Quantity" &&
			lines[index+3] == "Market Price" &&
			lines[index+4] == "Market Value" &&
			lines[index+5] == "Cost Price" &&
			lines[index+6] == "Unrealized" &&
			lines[index+7] == "TD Cost Basis" {
			headerIndex = index
			break
		}
	}
	if headerIndex < 0 {
		return nil, fmt.Errorf("holdings table not found")
	}

	var holdings []StatementHolding
	for index := headerIndex + 8; index < len(lines); index++ {
		symbol := strings.ToUpper(lines[index])
		if symbol == "Income" || symbol == "Transaction" ||
			strings.HasPrefix(symbol, "DEPOSIT") || symbol == "Fees" {
			break
		}
		if !symbolPattern.MatchString(symbol) || strings.HasPrefix(symbol, "*") {
			continue
		}
		if index+7 >= len(lines) {
			break
		}
		quantity, err := parseDecimal(lines[index+2])
		if err != nil {
			continue
		}
		market, err := parseDecimal(lines[index+3])
		if err != nil {
			continue
		}
		marketValue, err := parseDecimal(lines[index+4])
		if err != nil {
			continue
		}
		costPrice, err := parseDecimal(lines[index+5])
		if err != nil {
			continue
		}
		unrealized, err := parseDecimal(lines[index+6])
		if err != nil {
			continue
		}
		costBasis, err := parseDecimal(lines[index+7])
		if err != nil {
			continue
		}
		holdings = append(holdings, StatementHolding{
			Symbol:     symbol,
			Quantity:   quantity,
			Market:     market,
			MarketVal:  marketValue,
			CostPrice:  costPrice,
			CostBasis:  costBasis,
			Unrealized: unrealized,
		})
		index += 7
	}
	if len(holdings) == 0 {
		return nil, nil
	}
	return holdings, nil
}
