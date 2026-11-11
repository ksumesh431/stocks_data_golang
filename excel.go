package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

const (
	summarySheet  = "Summary"
	holdingsSheet = "Holdings"
	historySheet  = "History"
	warningsSheet = "Warnings"
)

type excelInfo struct {
	Generated    time.Time
	QuoteTime    time.Time
	QuoteSources []string
	FXTime       time.Time
}

type excelStyles struct {
	title    int
	subtitle int
	section  int
	money    int
	percent  int
	rate     int
	count    int
	note     int
	wrap     int
}

func writeExcel(path string, dataset Dataset, valuation Valuation, performance Performance, points []HistoryPoint, warnings []string, info excelInfo) error {
	book := excelize.NewFile()
	defer book.Close()

	if err := book.SetSheetName(book.GetSheetName(0), summarySheet); err != nil {
		return fmt.Errorf("rename summary sheet: %w", err)
	}
	for _, sheet := range []string{holdingsSheet, historySheet, warningsSheet} {
		if _, err := book.NewSheet(sheet); err != nil {
			return fmt.Errorf("create sheet %s: %w", sheet, err)
		}
	}
	book.SetActiveSheet(0)

	styles, err := newExcelStyles(book)
	if err != nil {
		return err
	}
	if err := writeSummarySheet(book, dataset, valuation, performance, info, styles); err != nil {
		return err
	}
	if err := writeHoldingsSheet(book, valuation, styles); err != nil {
		return err
	}
	if err := writeHistorySheet(book, points, styles); err != nil {
		return err
	}
	if err := writeWarningsSheet(book, warnings, styles); err != nil {
		return err
	}
	if err := book.SaveAs(path); err != nil {
		return fmt.Errorf("save Excel report: %w", err)
	}
	return nil
}

func newExcelStyles(book *excelize.File) (excelStyles, error) {
	var styles excelStyles
	var styleErr error
	newStyle := func(definition *excelize.Style) int {
		if styleErr != nil {
			return 0
		}
		style, err := book.NewStyle(definition)
		if err != nil {
			styleErr = err
		}
		return style
	}

	styles.title = newStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 16, Color: "1F3864"},
	})
	styles.subtitle = newStyle(&excelize.Style{
		Font: &excelize.Font{Size: 10, Color: "595959"},
	})
	styles.section = newStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11, Color: "1F3864"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"DDEBF7"}},
	})
	styles.money = newStyle(&excelize.Style{NumFmt: 4})
	styles.percent = newStyle(&excelize.Style{NumFmt: 10})
	styles.rate = newStyle(&excelize.Style{NumFmt: 4, CustomNumFmt: stringPointer("0.0000")})
	styles.count = newStyle(&excelize.Style{NumFmt: 1})
	styles.note = newStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Color: "7F7F7F"},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	styles.wrap = newStyle(&excelize.Style{
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	if styleErr != nil {
		return styles, fmt.Errorf("create Excel styles: %w", styleErr)
	}
	return styles, nil
}

func stringPointer(value string) *string {
	return &value
}

func writeSummarySheet(book *excelize.File, dataset Dataset, valuation Valuation, performance Performance, info excelInfo, styles excelStyles) error {
	writer := newSheetWriter(book, summarySheet)
	writer.value("A1", "US Stock Portfolio Performance")
	writer.style("A1", styles.title)
	writer.value("A2", fmt.Sprintf("Valuation date %s   |   USD/INR %s (as of %s)   |   Generated %s",
		valuation.AsOf.Format("2006-01-02"), performance.USDINR.StringFixed(4),
		info.FXTime.Format("2006-01-02 15:04"), info.Generated.Format("2006-01-02 15:04")))
	writer.style("A2", styles.subtitle)
	priceNote := "Prices: manual prices file"
	if len(info.QuoteSources) > 0 {
		priceNote = fmt.Sprintf("Prices: %s, latest quote %s", strings.Join(info.QuoteSources, ", "), info.QuoteTime.Format("2006-01-02 15:04 UTC"))
	}
	writer.value("A3", priceNote)
	writer.style("A3", styles.subtitle)

	section := func(row int, title string) {
		writer.value(fmt.Sprintf("A%d", row), title)
		writer.styleRange(fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), styles.section)
	}
	line := func(row int, label string, value any, style int) {
		writer.value(fmt.Sprintf("A%d", row), label)
		writer.value(fmt.Sprintf("B%d", row), value)
		writer.style(fmt.Sprintf("B%d", row), style)
	}

	section(5, "Portfolio")
	line(6, "Stock value (USD)", valuation.StockUSD.InexactFloat64(), styles.money)
	line(7, "Wallet cash (USD)", valuation.WalletUSD.InexactFloat64(), styles.money)
	line(8, "Total value (USD)", valuation.TotalUSD.InexactFloat64(), styles.money)
	line(9, "Total value (INR)", performance.ValueINR.InexactFloat64(), styles.money)
	line(10, "Wallet balance date", valuation.WalletOn.Format("2006-01-02"), 0)

	section(12, "INR return")
	line(13, "Contributions (including fees)", performance.DepositsINR.InexactFloat64(), styles.money)
	line(14, "Withdrawals (net of fees)", performance.WithdrawalsINR.InexactFloat64(), styles.money)
	line(15, "Charges included", performance.ChargesINR.InexactFloat64(), styles.money)
	line(16, "TCS (tracked, not deducted)", performance.TCSINR.InexactFloat64(), styles.money)
	line(17, "Net invested", performance.NetInvestedINR.InexactFloat64(), styles.money)
	line(18, "Current value", performance.ValueINR.InexactFloat64(), styles.money)
	line(19, "Absolute profit", performance.ProfitINR.InexactFloat64(), styles.money)
	line(20, "Cumulative return (not annualized)", performance.SimpleRate.InexactFloat64(), styles.percent)
	writeXIRRLine(line, 21, "INR XIRR (annualized)", performance.INRXIRR, performance.INRXIRRErr, styles)

	section(23, "USD return (since inception)")
	line(24, "Deposits", performance.DepositsUSD.InexactFloat64(), styles.money)
	line(25, "Withdrawals", performance.WithdrawalsUSD.InexactFloat64(), styles.money)
	line(26, "Net invested", performance.NetInvestedUSD.InexactFloat64(), styles.money)
	line(27, "Current value", performance.ValueUSD.InexactFloat64(), styles.money)
	line(28, "Absolute profit", performance.ProfitUSD.InexactFloat64(), styles.money)
	line(29, "Cumulative return (not annualized)", performance.SimpleRateUSD.InexactFloat64(), styles.percent)
	writeXIRRLine(line, 30, "USD XIRR (annualized)", performance.USDXIRR, performance.USDXIRRErr, styles)

	section(32, fmt.Sprintf("%d year to date (USD)", valuation.AsOf.Year()))
	if performance.YTD.StartDate.IsZero() {
		message := performance.YTD.Err
		if message == "" {
			message = "not calculated"
		}
		line(33, "Status", "unavailable: "+message, 0)
	} else {
		line(33, "Starting value ("+performance.YTD.StartDate.Format("2006-01-02")+")", performance.YTD.StartValueUSD.InexactFloat64(), styles.money)
		line(34, "Deposits this year", performance.YTD.DepositsUSD.InexactFloat64(), styles.money)
		line(35, "Withdrawals this year", performance.YTD.WithdrawalsUSD.InexactFloat64(), styles.money)
		line(36, "Current value", performance.ValueUSD.InexactFloat64(), styles.money)
		line(37, "Investment gain this year", performance.YTD.ProfitUSD.InexactFloat64(), styles.money)
		if performance.YTD.ReturnErr == "" {
			line(38, "YTD return (not annualized)", performance.YTD.Return.InexactFloat64(), styles.percent)
		} else {
			line(38, "YTD return (not annualized)", "unavailable: "+performance.YTD.ReturnErr, 0)
		}
	}

	section(40, "FX and conversion impact")
	line(41, "Reference USD/INR", performance.ReferenceRate.InexactFloat64(), styles.rate)
	line(42, "Profit at constant FX (INR)", performance.ConstantFXProfit.InexactFloat64(), styles.money)
	line(43, "FX and conversion impact (INR)", performance.FXImpact.InexactFloat64(), styles.money)

	section(45, "Reports")
	line(46, "External flows", len(dataset.Flows), styles.count)
	line(47, "Unique trades", len(dataset.Trades), styles.count)
	line(48, "Wallet rows", len(dataset.Wallet), styles.count)
	line(49, "Monthly statements", len(dataset.Statements), styles.count)

	writer.columnWidth("A", "A", 36)
	writer.columnWidth("B", "B", 18)
	writer.columnWidth("M", "M", 22)
	writer.columnWidth("N", "N", 16)

	// Chart source data lives in M:N so the charts do not cover it.
	writer.value("M4", "INR cash flow")
	writer.value("N4", "Amount")
	writer.styleRange("M4", "N4", styles.section)
	chartRows := []struct {
		label string
		value float64
	}{
		{"Contributions", performance.DepositsINR.InexactFloat64()},
		{"Withdrawals", performance.WithdrawalsINR.InexactFloat64()},
		{"Net invested", performance.NetInvestedINR.InexactFloat64()},
		{"Current value", performance.ValueINR.InexactFloat64()},
		{"Profit", performance.ProfitINR.InexactFloat64()},
	}
	for index, item := range chartRows {
		row := 5 + index
		writer.value(fmt.Sprintf("M%d", row), item.label)
		writer.value(fmt.Sprintf("N%d", row), item.value)
		writer.style(fmt.Sprintf("N%d", row), styles.money)
	}

	writer.value("M12", "Allocation (USD)")
	writer.value("N12", "Value")
	writer.styleRange("M12", "N12", styles.section)
	writer.value("M13", "Stocks")
	writer.value("N13", valuation.StockUSD.InexactFloat64())
	writer.style("N13", styles.money)
	writer.value("M14", "Cash")
	writer.value("N14", valuation.WalletUSD.InexactFloat64())
	writer.style("N14", styles.money)

	writer.value("A51", "Cumulative and YTD returns are not annualized; XIRR is annualized. YTD uses Modified Dietz to account for deposit/withdrawal timing.")
	writer.style("A51", styles.note)
	writer.value("A52", "Charts show INR cash flows and the USD stock/cash split. See Holdings and History for detail.")
	writer.style("A52", styles.note)

	if err := writer.err(); err != nil {
		return fmt.Errorf("write summary sheet: %w", err)
	}

	if err := book.AddChart(summarySheet, "D5", &excelize.Chart{
		Type: excelize.Col,
		Series: []excelize.ChartSeries{{
			Name:       fmt.Sprintf("%s!$N$4", summarySheet),
			Categories: fmt.Sprintf("%s!$M$5:$M$9", summarySheet),
			Values:     fmt.Sprintf("%s!$N$5:$N$9", summarySheet),
		}},
		Title:     chartTitle("INR return breakdown"),
		Legend:    excelize.ChartLegend{Position: "none"},
		Dimension: excelize.ChartDimension{Width: 520, Height: 300},
	}); err != nil {
		return fmt.Errorf("add summary cash flow chart: %w", err)
	}

	if err := book.AddChart(summarySheet, "D27", &excelize.Chart{
		Type: excelize.Pie,
		Series: []excelize.ChartSeries{{
			Name:       fmt.Sprintf("%s!$N$12", summarySheet),
			Categories: fmt.Sprintf("%s!$M$13:$M$14", summarySheet),
			Values:     fmt.Sprintf("%s!$N$13:$N$14", summarySheet),
		}},
		Title:     chartTitle("Portfolio allocation (USD)"),
		Legend:    excelize.ChartLegend{Position: "bottom"},
		PlotArea:  excelize.ChartPlotArea{ShowPercent: true, ShowLeaderLines: true},
		Dimension: excelize.ChartDimension{Width: 420, Height: 280},
	}); err != nil {
		return fmt.Errorf("add summary allocation chart: %w", err)
	}
	return nil
}

func writeXIRRLine(line func(int, string, any, int), row int, label string, value decimal.Decimal, xirrErr string, styles excelStyles) {
	if xirrErr == "" {
		line(row, label, value.InexactFloat64(), styles.percent)
		return
	}
	line(row, label, "unavailable: "+xirrErr, 0)
}

func writeHoldingsSheet(book *excelize.File, valuation Valuation, styles excelStyles) error {
	writer := newSheetWriter(book, holdingsSheet)
	writer.value("A1", "Holdings as of "+valuation.AsOf.Format("2006-01-02"))
	writer.style("A1", styles.title)
	writer.value("A2", fmt.Sprintf("Stock market value $%s   |   Wallet cash $%s   |   Total $%s",
		money(valuation.StockUSD), money(valuation.WalletUSD), money(valuation.TotalUSD)))
	writer.style("A2", styles.subtitle)

	if len(valuation.Holdings) == 0 {
		writer.value("A4", "No current holdings.")
		writer.style("A4", styles.note)
		return writer.err()
	}

	headers := []string{
		"Symbol", "Quantity", "Average cost (USD)", "Market price (USD)",
		"Cost basis (USD)", "Market value (USD)", "Unrealized (USD)",
	}
	writer.textRow(4, headers...)
	firstRow := 5
	for index, holding := range valuation.Holdings {
		row := firstRow + index
		writer.row(row,
			holding.Symbol,
			holding.Quantity.InexactFloat64(),
			holding.AverageCost.InexactFloat64(),
			holding.MarketPrice.InexactFloat64(),
			holding.CostBasis.InexactFloat64(),
			holding.MarketValue.InexactFloat64(),
			holding.Unrealized.InexactFloat64(),
		)
		writer.styleRange(fmt.Sprintf("C%d", row), fmt.Sprintf("G%d", row), styles.money)
	}
	lastRow := firstRow + len(valuation.Holdings) - 1

	totalRow := lastRow + 2
	writer.value(fmt.Sprintf("A%d", totalRow), "Total")
	writer.style(fmt.Sprintf("A%d", totalRow), styles.section)
	writer.value(fmt.Sprintf("E%d", totalRow), valuation.StockUSD.InexactFloat64())
	writer.value(fmt.Sprintf("F%d", totalRow), valuation.StockUSD.InexactFloat64())
	writer.value(fmt.Sprintf("G%d", totalRow), valuation.HoldingsUnrealized().InexactFloat64())
	writer.styleRange(fmt.Sprintf("E%d", totalRow), fmt.Sprintf("G%d", totalRow), styles.money)

	writer.columnWidth("A", "A", 10)
	writer.columnWidth("B", "G", 18)
	writer.columnWidth("I", "T", 12)
	if err := writer.err(); err != nil {
		return fmt.Errorf("write holdings sheet: %w", err)
	}

	if err := book.AddTable(holdingsSheet, &excelize.Table{
		Range:     fmt.Sprintf("A4:G%d", lastRow),
		Name:      "HoldingsTable",
		StyleName: "TableStyleMedium2",
	}); err != nil {
		return fmt.Errorf("add holdings table: %w", err)
	}

	if err := book.AddChart(holdingsSheet, "I4", &excelize.Chart{
		Type: excelize.Bar,
		Series: []excelize.ChartSeries{{
			Name:       fmt.Sprintf("%s!$F$4", holdingsSheet),
			Categories: fmt.Sprintf("%s!$A$%d:$A$%d", holdingsSheet, firstRow, lastRow),
			Values:     fmt.Sprintf("%s!$F$%d:$F$%d", holdingsSheet, firstRow, lastRow),
		}},
		Title:     chartTitle("Market value by holding (USD)"),
		Legend:    excelize.ChartLegend{Position: "none"},
		Dimension: excelize.ChartDimension{Width: 640, Height: 380},
	}); err != nil {
		return fmt.Errorf("add holdings value chart: %w", err)
	}

	if err := book.AddChart(holdingsSheet, "I29", &excelize.Chart{
		Type: excelize.Pie,
		Series: []excelize.ChartSeries{{
			Name:       fmt.Sprintf("%s!$F$4", holdingsSheet),
			Categories: fmt.Sprintf("%s!$A$%d:$A$%d", holdingsSheet, firstRow, lastRow),
			Values:     fmt.Sprintf("%s!$F$%d:$F$%d", holdingsSheet, firstRow, lastRow),
		}},
		Title:     chartTitle("Allocation by holding"),
		Legend:    excelize.ChartLegend{Position: "right"},
		PlotArea:  excelize.ChartPlotArea{ShowPercent: true, ShowLeaderLines: true},
		Dimension: excelize.ChartDimension{Width: 560, Height: 340},
	}); err != nil {
		return fmt.Errorf("add holdings allocation chart: %w", err)
	}
	return nil
}

func writeHistorySheet(book *excelize.File, points []HistoryPoint, styles excelStyles) error {
	writer := newSheetWriter(book, historySheet)
	writer.value("A1", "Month-end history")
	writer.style("A1", styles.title)
	if len(points) == 0 {
		writer.value("A2", "No monthly statement data available.")
		writer.style("A2", styles.note)
		return writer.err()
	}

	hasINR := false
	for _, point := range points {
		if !point.FXMissing {
			hasINR = true
			break
		}
	}
	headers := []string{"Month end", "Value (USD)", "Profit (USD)"}
	if hasINR {
		headers = append(headers, "Value (INR)", "Profit (INR)", "INR XIRR")
	}
	writer.textRow(3, headers...)
	firstRow := 4
	for index, point := range points {
		row := firstRow + index
		writer.value(fmt.Sprintf("A%d", row), point.Date.Format("2006-01-02"))
		writer.value(fmt.Sprintf("B%d", row), point.ValueUSD.InexactFloat64())
		writer.value(fmt.Sprintf("C%d", row), point.Performance.ProfitUSD.InexactFloat64())
		writer.styleRange(fmt.Sprintf("B%d", row), fmt.Sprintf("C%d", row), styles.money)
		if !hasINR {
			continue
		}
		if point.FXMissing {
			continue
		}
		writer.value(fmt.Sprintf("D%d", row), point.ValueINR.InexactFloat64())
		writer.value(fmt.Sprintf("E%d", row), point.Performance.ProfitINR.InexactFloat64())
		writer.styleRange(fmt.Sprintf("D%d", row), fmt.Sprintf("E%d", row), styles.money)
		if point.Performance.INRXIRRErr == "" {
			writer.value(fmt.Sprintf("F%d", row), point.Performance.INRXIRR.InexactFloat64())
			writer.style(fmt.Sprintf("F%d", row), styles.percent)
		}
	}
	lastRow := firstRow + len(points) - 1
	lastColumn := "C"
	if hasINR {
		lastColumn = "F"
	}
	writer.columnWidth("A", "A", 14)
	writer.columnWidth("B", "F", 16)
	writer.columnWidth("H", "T", 12)
	if err := writer.err(); err != nil {
		return fmt.Errorf("write history sheet: %w", err)
	}

	if err := book.AddTable(historySheet, &excelize.Table{
		Range:     fmt.Sprintf("A3:%s%d", lastColumn, lastRow),
		Name:      "HistoryTable",
		StyleName: "TableStyleMedium2",
	}); err != nil {
		return fmt.Errorf("add history table: %w", err)
	}

	series := []excelize.ChartSeries{{
		Name:       fmt.Sprintf("%s!$B$3", historySheet),
		Categories: fmt.Sprintf("%s!$A$%d:$A$%d", historySheet, firstRow, lastRow),
		Values:     fmt.Sprintf("%s!$B$%d:$B$%d", historySheet, firstRow, lastRow),
		Line:       excelize.LineOptions{Width: 2.25},
	}}
	if hasINR {
		series = append(series, excelize.ChartSeries{
			Name:       fmt.Sprintf("%s!$D$3", historySheet),
			Categories: fmt.Sprintf("%s!$A$%d:$A$%d", historySheet, firstRow, lastRow),
			Values:     fmt.Sprintf("%s!$D$%d:$D$%d", historySheet, firstRow, lastRow),
			Line:       excelize.LineOptions{Width: 2.25},
		})
	}
	if err := book.AddChart(historySheet, "H3", &excelize.Chart{
		Type:      excelize.Line,
		Series:    series,
		Title:     chartTitle("Portfolio value at month end"),
		Legend:    excelize.ChartLegend{Position: "bottom"},
		Dimension: excelize.ChartDimension{Width: 720, Height: 340},
	}); err != nil {
		return fmt.Errorf("add history value chart: %w", err)
	}

	if err := book.AddChart(historySheet, "H22", &excelize.Chart{
		Type: excelize.Col,
		Series: []excelize.ChartSeries{{
			Name:       fmt.Sprintf("%s!$C$3", historySheet),
			Categories: fmt.Sprintf("%s!$A$%d:$A$%d", historySheet, firstRow, lastRow),
			Values:     fmt.Sprintf("%s!$C$%d:$C$%d", historySheet, firstRow, lastRow),
		}},
		Title:     chartTitle("Profit at month end (USD)"),
		Legend:    excelize.ChartLegend{Position: "none"},
		Dimension: excelize.ChartDimension{Width: 720, Height: 300},
	}); err != nil {
		return fmt.Errorf("add history profit chart: %w", err)
	}
	return nil
}

func writeWarningsSheet(book *excelize.File, warnings []string, styles excelStyles) error {
	writer := newSheetWriter(book, warningsSheet)
	writer.value("A1", "Validation warnings")
	writer.style("A1", styles.title)
	if len(warnings) == 0 {
		writer.value("A3", "No warnings. Reports reconciled.")
		writer.style("A3", styles.note)
		return writer.err()
	}
	for index, warning := range warnings {
		writer.value(fmt.Sprintf("A%d", index+3), warning)
	}
	writer.styleRange("A3", fmt.Sprintf("A%d", len(warnings)+2), styles.wrap)
	writer.columnWidth("A", "A", 110)
	if err := writer.err(); err != nil {
		return fmt.Errorf("write warnings sheet: %w", err)
	}
	return nil
}

func chartTitle(text string) excelize.ChartTitle {
	return excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: text}}}
}

func (valuation Valuation) HoldingsUnrealized() decimal.Decimal {
	total := decimal.Zero
	for _, holding := range valuation.Holdings {
		total = total.Add(holding.Unrealized)
	}
	return total
}

// sheetWriter collects the first workbook error so sheet code stays readable.
type sheetWriter struct {
	book     *excelize.File
	sheet    string
	firstErr error
}

func newSheetWriter(book *excelize.File, sheet string) *sheetWriter {
	return &sheetWriter{book: book, sheet: sheet}
}

func (w *sheetWriter) value(cell string, value any) {
	if w.firstErr != nil {
		return
	}
	w.firstErr = w.book.SetCellValue(w.sheet, cell, value)
}

func (w *sheetWriter) style(cell string, style int) {
	if w.firstErr != nil || style == 0 {
		return
	}
	w.firstErr = w.book.SetCellStyle(w.sheet, cell, cell, style)
}

func (w *sheetWriter) styleRange(first, last string, style int) {
	if w.firstErr != nil || style == 0 {
		return
	}
	w.firstErr = w.book.SetCellStyle(w.sheet, first, last, style)
}

func (w *sheetWriter) row(row int, values ...any) {
	if w.firstErr != nil {
		return
	}
	for index, value := range values {
		cell, err := excelize.CoordinatesToCellName(index+1, row)
		if err != nil {
			w.firstErr = err
			return
		}
		if err := w.book.SetCellValue(w.sheet, cell, value); err != nil {
			w.firstErr = err
			return
		}
	}
}

func (w *sheetWriter) textRow(row int, values ...string) {
	if w.firstErr != nil {
		return
	}
	for index, value := range values {
		cell, err := excelize.CoordinatesToCellName(index+1, row)
		if err != nil {
			w.firstErr = err
			return
		}
		if err := w.book.SetCellValue(w.sheet, cell, value); err != nil {
			w.firstErr = err
			return
		}
	}
}

func (w *sheetWriter) columnWidth(first, last string, width float64) {
	if w.firstErr != nil {
		return
	}
	w.firstErr = w.book.SetColWidth(w.sheet, first, last, width)
}

func (w *sheetWriter) err() error {
	return w.firstErr
}
