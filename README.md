# Stocks portfolio performance

A small Go CLI that calculates overall USD and INR performance for an INDmoney
US-stock portfolio from manually downloaded reports.

It treats bank deposits and withdrawals as external cash flows. Buying, selling,
dividends, fees, and reinvesting sale proceeds are internal movements and do not
reset the return calculation.

## Reports

Place the latest reports in the existing subdirectories under `Reports/`:

- `deposit_and_withdrawl_reports/` — LRS/deposit-withdrawal XLSX
- `running_balance_reports/` — USD wallet XLSX
- `trade_reports/` — order/trade XLS
- `holding_reports/` — holdings XLS
- `monthly_account_statements/` — monthly statement PDF
- `brokerage_reports/` — optional validation XLS

## Report locations

| Report | Where to download it | Destination folder | Usage |
|---|---|---|---|
| Holdings report | [INDmoney US taxation documents](https://www.indmoney.com/widget/page?page=usDocDownloadPage&documentCode=HOLDINGS_REPORT&category=REPORTS) | `reports_directory/holding_reports/` | Required; provides current quantities and broker cost basis |
| Trade / order report | [INDmoney order report download](https://www.indmoney.com/widget/page?page=usDocDownloadPage&documentCode=ORDER_REPORT&category=REPORTS), or the INDmoney mobile app | `reports_directory/trade_reports/` | Required; provides buys, sells, quantities, prices, and brokerage |
| Monthly account statements | [INDmoney US account statements](https://www.indmoney.com/investments/us-stocks/related-documents/account-statements) | `reports_directory/monthly_account_statements/` | Optional validation history; download the available PDF statements |
| Deposit and withdrawal report | INDmoney app → Buying Power → question mark (top left) → US Stock Reports -> running balance report | `reports_directory/deposit_and_withdrawl_reports/` | Required for historical USD/INR funding rates and XIRR cash flows |
| Running balance / wallet statement | INDmoney app → Buying Power → question mark (top left) → US Stock Reports | `reports_directory/running_balance_reports/` | Required for dividends, withholding tax, and current cash balance |


The tool discovers files at runtime and ignores `:Zone.Identifier` files.
Reports may overlap or be replaced. External flows and trades are deduplicated,
and wallet history is merged across incomplete exports.

## Run

Go is available at `/usr/local/go/bin/go` in this WSL environment.

```sh
go run . -reports ./Reports
```

The default run fetches current stock prices and USD/INR from Yahoo Finance. It
sends ticker symbols only—not report contents, account numbers, or balances.

Useful flags:

```text
-reports string    report root directory (default "./Reports")
-prices string     manual CSV with Symbol,Price columns
-usdinr string     manual USD/INR rate
-excel string      Excel report path (default "portfolio_report.xlsx", empty to skip)
-history           show month-end history from statement PDFs
-csv string        write holdings to a CSV file
-history-csv path  write month-end history to a CSV file
```

Every run writes `portfolio_report.xlsx` next to the working directory unless
`-excel ""` is passed. The workbook has:

- `Summary` — portfolio value, cumulative and year-to-date USD returns, INR/USD profit and XIRR, FX impact, report counts
- `Holdings` — per-symbol quantities, costs, market values, unrealized gain/loss
- `History` — month-end values and profit with a trend chart
- `Warnings` — validation warnings, or a clean-reconciliation note

Charts cover the INR cash-flow breakdown, stock/cash allocation, market value by
holding, holding allocation, portfolio value over time, and monthly profit. The
`History` sheet has USD values by default; pass `-history` to add INR values and
INR XIRR, which also fetches historical USD/INR rates.

Offline example:

```sh
go run . -reports ./Reports -usdinr 93.68 -prices prices.csv
```

A prices file has this shape:

```csv
symbol,price
AAPL,230.10
MSFT,500.50
```

## What is calculated

```text
Portfolio USD = stock market value + INDmoney USD trade balance
Portfolio INR = Portfolio USD * current USD/INR

INR profit = Portfolio INR + INR withdrawals - INR contributions
USD profit = Portfolio USD + USD withdrawals - USD deposits
```

INR contributions include GST and known bank fees. TCS is reported separately.
INR and USD XIRR use only external bank flows plus the final portfolio value.
XIRR is annualized. Cumulative return and year-to-date (YTD) return are not
annualized.

The cumulative USD return is USD profit divided by net USD invested. YTD starts
from the previous December statement's stock value plus its INDmoney wallet
balance. YTD gain removes deposits and adds back withdrawals during the year;
the YTD percentage uses Modified Dietz to account for the timing of those cash
flows. A previous December statement and starting wallet balance are required.

The FX section shows the result at a constant reference rate (the first
deposit's effective rate) and the difference from actual INR profit. This is
labelled as FX and conversion impact because it includes remittance spreads as
well as exchange-rate movement.

The monthly statement's broker ledger cash is intentionally not added to the
INDmoney wallet balance. Those are different ledgers, and the tool reconciles
the economic portfolio total instead.

## Verify

```sh
go test ./...
go vet ./...
```

`go test -race ./...` requires a C compiler (CGO) in this environment.

## Layout

```text
main.go        flags and top-level flow
discover.go    report discovery
xlsx.go        LRS and wallet importers
xls.go         orders and holdings importers
statement.go   monthly statement PDF importer
normalize.go   deduplication and reconciliation
calculate.go   profit, XIRR, FX calculations
value.go       valuation, prices, USD/INR
output.go      console and CSV output
```
