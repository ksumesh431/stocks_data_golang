package main

import (
	"fmt"
	"math"
	"time"

	"github.com/shopspring/decimal"
)

type Performance struct {
	AsOf time.Time

	DepositsINR    decimal.Decimal
	WithdrawalsINR decimal.Decimal
	ChargesINR     decimal.Decimal
	TCSINR         decimal.Decimal
	NetInvestedINR decimal.Decimal

	DepositsUSD    decimal.Decimal
	WithdrawalsUSD decimal.Decimal
	NetInvestedUSD decimal.Decimal

	ValueINR decimal.Decimal
	USDINR   decimal.Decimal
	ValueUSD decimal.Decimal

	ProfitINR     decimal.Decimal
	ProfitUSD     decimal.Decimal
	SimpleRate    decimal.Decimal
	SimpleRateUSD decimal.Decimal

	YTD PeriodPerformance

	INRXIRR    decimal.Decimal
	INRXIRRErr string
	USDXIRR    decimal.Decimal
	USDXIRRErr string

	ReferenceRate    decimal.Decimal
	ConstantFXProfit decimal.Decimal
	FXImpact         decimal.Decimal
}

type PeriodPerformance struct {
	StartDate      time.Time
	StartValueUSD  decimal.Decimal
	DepositsUSD    decimal.Decimal
	WithdrawalsUSD decimal.Decimal
	ProfitUSD      decimal.Decimal
	Return         decimal.Decimal
	Err            string
	ReturnErr      string
}

type cashFlow struct {
	Date   time.Time
	Amount float64
}

func calculatePerformance(dataset Dataset, endValueUSD, usdInr decimal.Decimal, asOf time.Time) Performance {
	performance := Performance{
		AsOf:     asOf,
		ValueUSD: endValueUSD,
		USDINR:   usdInr,
	}
	performance.ValueINR = endValueUSD.Mul(usdInr)

	var inrFlows []cashFlow
	var usdFlows []cashFlow
	for _, flow := range dataset.Flows {
		if flow.Date.After(asOf) {
			continue
		}
		charges := flow.GST.Add(flow.BankFee)
		performance.ChargesINR = performance.ChargesINR.Add(charges)
		performance.TCSINR = performance.TCSINR.Add(flow.TCS)

		switch flow.Direction {
		case "Deposit":
			contribution := flow.INR.Add(charges)
			performance.DepositsINR = performance.DepositsINR.Add(contribution)
			performance.DepositsUSD = performance.DepositsUSD.Add(flow.USD)
			inrFlows = append(inrFlows, cashFlow{Date: flow.Date, Amount: -contribution.InexactFloat64()})
			usdFlows = append(usdFlows, cashFlow{Date: flow.Date, Amount: -flow.USD.InexactFloat64()})
			if performance.ReferenceRate.IsZero() {
				performance.ReferenceRate = effectiveRate(flow)
			}
		case "Withdrawal":
			received := flow.INR.Sub(charges)
			if received.IsNegative() {
				received = decimal.Zero
			}
			performance.WithdrawalsINR = performance.WithdrawalsINR.Add(received)
			performance.WithdrawalsUSD = performance.WithdrawalsUSD.Add(flow.USD)
			inrFlows = append(inrFlows, cashFlow{Date: flow.Date, Amount: received.InexactFloat64()})
			usdFlows = append(usdFlows, cashFlow{Date: flow.Date, Amount: flow.USD.InexactFloat64()})
		}
	}
	performance.NetInvestedINR = performance.DepositsINR.Sub(performance.WithdrawalsINR)
	performance.NetInvestedUSD = performance.DepositsUSD.Sub(performance.WithdrawalsUSD)
	performance.ProfitINR = performance.ValueINR.Add(performance.WithdrawalsINR).Sub(performance.DepositsINR)
	performance.ProfitUSD = performance.ValueUSD.Add(performance.WithdrawalsUSD).Sub(performance.DepositsUSD)

	if performance.NetInvestedINR.IsPositive() {
		performance.SimpleRate = performance.ProfitINR.Div(performance.NetInvestedINR)
	}
	if performance.NetInvestedUSD.IsPositive() {
		performance.SimpleRateUSD = performance.ProfitUSD.Div(performance.NetInvestedUSD)
	}

	if performance.ValueINR.IsPositive() {
		inrFlows = append(inrFlows, cashFlow{Date: asOf, Amount: performance.ValueINR.InexactFloat64()})
	}
	if performance.ValueUSD.IsPositive() {
		usdFlows = append(usdFlows, cashFlow{Date: asOf, Amount: performance.ValueUSD.InexactFloat64()})
	}
	inrRate, err := xirr(inrFlows)
	if err != nil {
		performance.INRXIRRErr = err.Error()
	} else {
		performance.INRXIRR = decimal.NewFromFloat(inrRate)
	}
	usdRate, err := xirr(usdFlows)
	if err != nil {
		performance.USDXIRRErr = err.Error()
	} else {
		performance.USDXIRR = decimal.NewFromFloat(usdRate)
	}

	if performance.ReferenceRate.IsPositive() {
		performance.ConstantFXProfit = performance.ReferenceRate.Mul(performance.ProfitUSD).
			Sub(performance.ChargesINR)
		performance.FXImpact = performance.ProfitINR.Sub(performance.ConstantFXProfit)
	}
	return performance
}

func calculateYearToDateUSD(dataset Dataset, endValueUSD decimal.Decimal, asOf time.Time) PeriodPerformance {
	result := PeriodPerformance{}
	yearStart := time.Date(asOf.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)

	var startStatement Statement
	foundStatement := false
	for _, statement := range dataset.Statements {
		if statement.Date.Year() != asOf.Year()-1 || statement.Date.Month() != time.December {
			continue
		}
		if !foundStatement || statement.Date.After(startStatement.Date) {
			startStatement = statement
			foundStatement = true
		}
	}
	if !foundStatement {
		result.Err = fmt.Sprintf("no December %d statement is available for the starting value", asOf.Year()-1)
		return result
	}
	wallet, ok := walletBalanceAt(dataset.Wallet, startStatement.Date)
	if !ok {
		result.Err = fmt.Sprintf("no wallet balance is available on or before %s", startStatement.Date.Format("2006-01-02"))
		return result
	}
	result.StartDate = startStatement.Date
	result.StartValueUSD = startStatement.LongValue.Add(wallet.TradeBalance)

	weightedFlows := decimal.Zero
	totalTime := asOf.Sub(yearStart)
	for _, flow := range dataset.Flows {
		if flow.Date.Before(yearStart) || flow.Date.After(asOf) {
			continue
		}
		signedAmount := decimal.Zero
		switch flow.Direction {
		case "Deposit":
			result.DepositsUSD = result.DepositsUSD.Add(flow.USD)
			signedAmount = flow.USD
		case "Withdrawal":
			result.WithdrawalsUSD = result.WithdrawalsUSD.Add(flow.USD)
			signedAmount = flow.USD.Neg()
		default:
			continue
		}
		if totalTime > 0 {
			weight := decimal.NewFromInt(asOf.Sub(flow.Date).Nanoseconds()).
				Div(decimal.NewFromInt(totalTime.Nanoseconds()))
			weightedFlows = weightedFlows.Add(signedAmount.Mul(weight))
		}
	}
	result.ProfitUSD = endValueUSD.Sub(result.StartValueUSD).
		Sub(result.DepositsUSD).Add(result.WithdrawalsUSD)

	if totalTime <= 0 {
		result.ReturnErr = "YTD return needs at least one day"
		return result
	}
	denominator := result.StartValueUSD.Add(weightedFlows)
	if !denominator.IsPositive() {
		result.ReturnErr = "starting value and timed cash flows must total more than zero"
		return result
	}
	result.Return = result.ProfitUSD.Div(denominator)
	return result
}

func xirr(flows []cashFlow) (float64, error) {
	if len(flows) < 2 {
		return 0, fmt.Errorf("need at least two cash flows")
	}
	var filtered []cashFlow
	var first time.Time
	positive, negative := false, false
	for _, flow := range flows {
		if flow.Amount == 0 {
			continue
		}
		if first.IsZero() || flow.Date.Before(first) {
			first = flow.Date
		}
		if flow.Amount > 0 {
			positive = true
		} else {
			negative = true
		}
		filtered = append(filtered, flow)
	}
	if !positive || !negative {
		return 0, fmt.Errorf("cash flows need both signs")
	}
	if first.IsZero() {
		return 0, fmt.Errorf("no cash flow dates")
	}

	npv := func(rate float64) float64 {
		total := 0.0
		for _, flow := range filtered {
			years := flow.Date.Sub(first).Hours() / 24 / 365
			total += flow.Amount / math.Pow(1+rate, years)
		}
		return total
	}

	const steps = 2000
	lowLog := math.Log(1e-9)
	highLog := math.Log(1000)
	step := (highLog - lowLog) / steps
	previousLog := lowLog
	previousRate := math.Exp(previousLog) - 1
	previousValue := npv(previousRate)
	var brackets [][2]float64
	for index := 1; index <= steps; index++ {
		currentLog := lowLog + step*float64(index)
		currentRate := math.Exp(currentLog) - 1
		currentValue := npv(currentRate)
		if math.Abs(currentValue) < 1e-7 {
			return currentRate, nil
		}
		if previousValue*currentValue < 0 {
			brackets = append(brackets, [2]float64{previousRate, currentRate})
		}
		previousLog = currentLog
		previousRate = currentRate
		previousValue = currentValue
	}
	if len(brackets) == 0 {
		return 0, fmt.Errorf("no real XIRR solution")
	}
	if len(brackets) > 1 {
		return 0, fmt.Errorf("multiple XIRR solutions")
	}
	low, high := brackets[0][0], brackets[0][1]
	for iteration := 0; iteration < 200; iteration++ {
		middle := (low + high) / 2
		value := npv(middle)
		if math.Abs(value) < 1e-9 {
			return middle, nil
		}
		if npv(low)*value <= 0 {
			high = middle
		} else {
			low = middle
		}
	}
	return (low + high) / 2, nil
}

type HistoryPoint struct {
	Date        time.Time
	ValueUSD    decimal.Decimal
	ValueINR    decimal.Decimal
	Performance Performance
	FXMissing   bool
}

func calculateHistory(dataset Dataset, fxRates map[string]decimal.Decimal) []HistoryPoint {
	if len(dataset.Statements) == 0 || len(dataset.Flows) == 0 {
		return nil
	}
	firstFlow := dataset.Flows[0].Date
	var points []HistoryPoint
	for _, statement := range dataset.Statements {
		if statement.Date.Before(firstFlow) {
			continue
		}
		wallet, ok := walletBalanceAt(dataset.Wallet, statement.Date)
		if !ok {
			continue
		}
		valueUSD := statement.LongValue.Add(wallet.TradeBalance)
		rate, hasRate := fxRates[statement.Date.Format("2006-01")]
		point := HistoryPoint{
			Date:      statement.Date,
			ValueUSD:  valueUSD,
			FXMissing: !hasRate,
		}
		if hasRate {
			point.ValueINR = valueUSD.Mul(rate)
			point.Performance = calculatePerformance(dataset, valueUSD, rate, statement.Date)
		} else {
			point.Performance = calculatePerformance(dataset, valueUSD, decimal.Zero, statement.Date)
			point.Performance.ValueINR = decimal.Zero
			point.Performance.ProfitINR = decimal.Zero
		}
		points = append(points, point)
	}
	return points
}
