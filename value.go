package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Quote struct {
	Symbol string
	Price  decimal.Decimal
	AsOf   time.Time
	Source string
}

type HoldingValue struct {
	Symbol      string
	Quantity    decimal.Decimal
	AverageCost decimal.Decimal
	MarketPrice decimal.Decimal
	CostBasis   decimal.Decimal
	MarketValue decimal.Decimal
	Unrealized  decimal.Decimal
}

type Valuation struct {
	AsOf      time.Time
	Holdings  []HoldingValue
	StockUSD  decimal.Decimal
	WalletUSD decimal.Decimal
	TotalUSD  decimal.Decimal
	WalletOn  time.Time
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

func buildValuation(dataset Dataset, report HoldingsReport, quotes map[string]Quote, asOf time.Time) (Valuation, error) {
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	asOf = time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)

	var valuation Valuation
	valuation.AsOf = asOf
	for _, holding := range report.Holdings {
		if !holding.Quantity.IsPositive() {
			continue
		}
		quote, ok := quotes[holding.Symbol]
		if !ok || !quote.Price.IsPositive() {
			return Valuation{}, fmt.Errorf("missing current price for %s", holding.Symbol)
		}
		cost := holding.Quantity.Mul(holding.AvgPrice)
		value := holding.Quantity.Mul(quote.Price)
		valuation.Holdings = append(valuation.Holdings, HoldingValue{
			Symbol:      holding.Symbol,
			Quantity:    holding.Quantity,
			AverageCost: holding.AvgPrice,
			MarketPrice: quote.Price,
			CostBasis:   cost,
			MarketValue: value,
			Unrealized:  value.Sub(cost),
		})
		valuation.StockUSD = valuation.StockUSD.Add(value)
	}
	sort.Slice(valuation.Holdings, func(i, j int) bool {
		return valuation.Holdings[i].Symbol < valuation.Holdings[j].Symbol
	})

	wallet, ok := walletBalanceAt(dataset.Wallet, asOf)
	if !ok {
		return Valuation{}, fmt.Errorf("no wallet balance available on or before %s", asOf.Format("2006-01-02"))
	}
	valuation.WalletUSD = wallet.TradeBalance
	valuation.WalletOn = wallet.Date
	valuation.TotalUSD = valuation.StockUSD.Add(valuation.WalletUSD)
	return valuation, nil
}

func resolveQuotes(symbols []string, manualPath string) (map[string]Quote, []string, error) {
	result := make(map[string]Quote, len(symbols))
	manual := make(map[string]decimal.Decimal)
	if manualPath != "" {
		var err error
		manual, err = readPricesCSV(manualPath)
		if err != nil {
			return result, nil, err
		}
	}

	var warnings []string
	var missing []string
	for _, symbol := range symbols {
		if price, ok := manual[symbol]; ok && price.IsPositive() {
			result[symbol] = Quote{
				Symbol: symbol,
				Price:  price,
				AsOf:   time.Now().UTC(),
				Source: "manual prices file",
			}
			continue
		}
		quote, err := fetchYahooQuote(symbol)
		if err != nil {
			missing = append(missing, symbol)
			warnings = append(warnings, fmt.Sprintf("quote %s: %v", symbol, err))
			continue
		}
		result[symbol] = quote
	}
	if len(missing) > 0 {
		return result, warnings, fmt.Errorf("missing prices for %s", strings.Join(missing, ", "))
	}
	return result, warnings, nil
}

func readPricesCSV(path string) (map[string]decimal.Decimal, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open prices file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read prices header: %w", err)
	}
	symbolColumn, priceColumn := -1, -1
	for index, value := range header {
		switch normalizeHeader(value) {
		case "symbol", "ticker":
			symbolColumn = index
		case "price", "price usd", "usd":
			priceColumn = index
		}
	}
	if symbolColumn < 0 || priceColumn < 0 {
		return nil, fmt.Errorf("prices CSV needs Symbol and Price columns")
	}
	prices := make(map[string]decimal.Decimal)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read prices file: %w", err)
		}
		symbol := strings.ToUpper(strings.TrimSpace(rowValue(row, symbolColumn)))
		price, err := parseDecimal(rowValue(row, priceColumn))
		if err != nil || symbol == "" {
			continue
		}
		prices[symbol] = price
	}
	return prices, nil
}

func fetchYahooQuote(symbol string) (Quote, error) {
	endpoint := "https://query1.finance.yahoo.com/v8/finance/chart/" +
		url.PathEscape(symbol) + "?range=1d&interval=1d"
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return Quote{}, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 stocks-project/1.0")
	response, err := httpClient.Do(request)
	if err != nil {
		return Quote{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("market data returned %s", response.Status)
	}
	var payload yahooChart
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&payload); err != nil {
		return Quote{}, err
	}
	if len(payload.Chart.Result) == 0 {
		return Quote{}, fmt.Errorf("no quote result")
	}
	meta := payload.Chart.Result[0].Meta
	if meta.RegularMarketPrice <= 0 {
		return Quote{}, fmt.Errorf("quote has no regular market price")
	}
	return Quote{
		Symbol: symbol,
		Price:  decimal.NewFromFloat(meta.RegularMarketPrice).Round(6),
		AsOf:   time.Unix(meta.RegularMarketTime, 0).UTC(),
		Source: "Yahoo Finance",
	}, nil
}

func fetchUSDINR() (decimal.Decimal, time.Time, error) {
	quote, err := fetchYahooQuote("USDINR=X")
	if err != nil {
		return decimal.Zero, time.Time{}, err
	}
	return quote.Price, quote.AsOf, nil
}

func fetchUSDINRAt(date time.Time) (decimal.Decimal, error) {
	start := date.AddDate(0, 0, -7).Unix()
	end := date.AddDate(0, 0, 1).Unix()
	endpoint := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/USDINR%%3DX?period1=%d&period2=%d&interval=1d",
		start, end)
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return decimal.Zero, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 stocks-project/1.0")
	response, err := httpClient.Do(request)
	if err != nil {
		return decimal.Zero, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return decimal.Zero, fmt.Errorf("FX data returned %s", response.Status)
	}
	var payload yahooChart
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&payload); err != nil {
		return decimal.Zero, err
	}
	if len(payload.Chart.Result) == 0 || len(payload.Chart.Result[0].Indicators.Quote) == 0 {
		return decimal.Zero, fmt.Errorf("no FX series")
	}
	result := payload.Chart.Result[0]
	closes := result.Indicators.Quote[0].Close
	var best float64
	for index, timestamp := range result.Timestamp {
		if index >= len(closes) || closes[index] == nil {
			continue
		}
		pointTime := time.Unix(timestamp, 0).UTC()
		if pointTime.After(date.Add(24 * time.Hour)) {
			continue
		}
		best = *closes[index]
	}
	if best <= 0 {
		return decimal.Zero, fmt.Errorf("no FX close on or before %s", date.Format("2006-01-02"))
	}
	return decimal.NewFromFloat(best).Round(6), nil
}

type yahooChart struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				RegularMarketTime  int64   `json:"regularMarketTime"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []*float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

func fetchHistoricalUSDINRMap(statements []Statement, warnings *[]string) map[string]decimal.Decimal {
	result := make(map[string]decimal.Decimal)
	for _, statement := range statements {
		key := statement.Date.Format("2006-01")
		if _, exists := result[key]; exists {
			continue
		}
		rate, err := fetchUSDINRAt(statement.Date)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf(
				"historical FX for %s: %v", key, err))
			continue
		}
		result[key] = rate
	}
	return result
}

func parseOptionalDecimal(value string) (decimal.Decimal, bool, error) {
	if strings.TrimSpace(value) == "" {
		return decimal.Zero, false, nil
	}
	parsed, err := parseDecimal(value)
	if err != nil {
		return decimal.Zero, false, err
	}
	return parsed, true, nil
}
