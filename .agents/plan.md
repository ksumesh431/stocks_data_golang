# Approved implementation plan

Status: implemented 2026-11-11.

## Goal

Calculate overall performance of a single INDmoney/Alpaca US-stock portfolio from
reports that are downloaded manually and refreshed over time.

The core question is portfolio-level, not per-deposit tracing:

- External INR deposits and bank withdrawals cross the portfolio boundary.
- Buys, sells, dividends, fees, and reinvestment are internal movements.
- Report data is combined with current stock prices and the current USD/INR rate.

## Required outputs

- INR contributions, withdrawals, net invested amount, current value, absolute
  profit, simple return, and INR XIRR.
- USD equivalent absolute profit and USD XIRR.
- FX/conversion impact versus a constant reference exchange rate.
- Holdings table with quantity, cost, market value, and unrealized gain/loss.
- Validation warnings for unmatched, missing, stale, or conflicting data.

INR returns include GST and known bank fees. TCS is shown separately and is not
silently treated as an investment expense.

## Correct financial model

```text
Portfolio USD = stock market value + INDmoney USD trade balance
Portfolio INR = Portfolio USD * current USD/INR rate

INR profit = Portfolio INR + INR withdrawals - INR contributions
USD profit = Portfolio USD + USD withdrawals - USD deposits
```

XIRR cash flows:

- Deposit: negative flow on the bank transaction date.
- Withdrawal: positive flow on the bank transaction date.
- Current portfolio value: positive terminal flow on the valuation date.
- Internal trades and reinvestment: no external cash flow.

FX impact is reported against a constant reference rate (default: first
deposit's effective rate):

```text
Constant-FX profit = reference rate * USD profit - applicable INR charges
FX impact = actual INR profit - constant-FX profit
```

This is labelled as FX and conversion impact, not pure market FX.

## Report roles

- LRS/deposit-withdrawal XLSX: canonical external INR/USD flows and conversion
  rates.
- Order/trade XLS: canonical trades, quantities, prices, brokerage, broker IDs.
- Holdings XLS: latest quantities and average USD cost; its total value is a
  cost basis, not current market value.
- Running-balance XLSX: USD wallet balance, dividends, withholding, commissions,
  settlement, and migration information. Merge overlapping exports.
- Monthly statement PDF: month-end market prices/values and independent
  reconciliation.
- Brokerage XLS: validation only.

## Data rules

- Never hardcode filenames or snapshot dates.
- Discover files at runtime; ignore `:Zone.Identifier`.
- Prefer internal dates over filenames.
- Use version only as a tiebreaker after checking report coverage.
- Merge coverage when the newest wallet export is not a superset.
- Do not treat `DEPOSIT SETTLEMENT` as another deposit.
- Do not treat `WALLET ADJUSTMENT` as a contribution.
- Normalize wallet sell quantities using the transaction type.
- Use broker reference IDs to deduplicate trades.
- Use the order report as canonical for trades and LRS as canonical for external
  flows to avoid double counting.
- The monthly statement's broker ledger cash can be negative and differs from
  the INDmoney wallet. Do not add it to the economic portfolio total; use stock
  market value plus the latest INDmoney trade balance and reconcile explicitly.
- Missing current prices must produce a warning or error, never a fake
  cost-based valuation.

## Architecture

Use a flat, single-package Go CLI. No database, web UI, or layered architecture.

```text
main.go              flags and top-level flow
discover.go          file discovery and coverage metadata
xlsx.go              LRS and wallet importers
xls.go               orders, holdings, brokerage importers
statement.go         monthly PDF importer
model.go             shared data structures
normalize.go         normalization and deduplication
calculate.go         profit, XIRR, FX calculations
value.go             valuation, quotes, wallet balance
output.go            console output
*_test.go            synthetic table-driven tests
```

Use decimal arithmetic for money/quantities. Convert to float only for the XIRR
root calculation. Keep every imported record traceable to its source file/row.

## Implementation order

1. Persist instructions; initialize module and report importers.
2. Normalize/deduplicate and reconcile overlapping reports.
3. Value holdings and calculate INR/USD profit, XIRR, and FX impact.
4. Parse month-end statements and add historical mode.
5. Add automatic quotes/FX with manual overrides, tests, and documentation.

## Approved decisions

- Simple CLI with console/CSV output; no web UI initially.
- Net INR returns include GST and known fees; TCS tracked separately.
- Automatic current quotes with manual offline overrides.
- Portfolio-level INR/USD/FX performance first; no per-stock tax-lot accounting.

## Verification

Run `gofmt`, `go vet ./...`, `go test ./...`, and `go test -race ./...`.
Synthetic tests must cover reinvestment, external withdrawals, currency moves,
version overlap, duplicate reports, missing prices, and XIRR edge cases.
