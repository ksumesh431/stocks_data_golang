# Stocks Project — Agent Guide

## Purpose

This workspace calculates overall INR and USD performance for a single
INDmoney/Alpaca US-stock portfolio. Source reports are downloaded manually and
live under `Reports/`. The approved design is in `.agents/plan.md`.

The calculation treats bank deposits/withdrawals as external cash flows and all
trades, dividends, fees, and reinvestment as internal movements. Current
valuation uses stock market value plus the INDmoney USD trade balance, with
current USD prices and USD/INR rate.

## Tech stack

- Language: Go. Before writing or generating Go code, load the Go skills that
  fit the task. Use `golang-pro` (`skills-golang-pro`) by default, plus focused
  skills when relevant: `golang-safety`, `golang-concurrency`,
  `golang-data-structures`, `golang-design-patterns`, `golang-performance`,
  `golang-benchmark`, `golang-troubleshooting`.
- Prefer the standard library. Add a dependency only when it clearly makes the
  code simpler.
- This is a personal project: prioritize simple, readable code over
  abstraction, extensibility, or cleverness.

## Code style

- Write the simplest implementation that solves the problem.
- Keep functions and packages small and obvious. Avoid indirection,
  interfaces, and generics unless they remove real duplication.
- Prefer clear names and short comments over documentation-heavy designs.

## Layout

- `Reports/brokerage_reports/` — brokerage and charges reports (`.xls`)
- `Reports/trade_reports/` — order and trade reports (`.xls`)
- `Reports/holding_reports/` — holdings snapshots (`.xls`)
- `Reports/running_balance_reports/` — running balance exports (`.xlsx`)
- `Reports/deposit_and_withdrawl_reports/` — deposit and withdrawal exports (`.xlsx`)
- `Reports/monthly_account_statements/` — monthly account statements (`.pdf`)
- `.opencode/opencode.jsonc` — project OpenCode configuration
- `.agents/instructions.md` — accumulated project-specific instructions

## Reports workflow

Reports are downloaded manually from INDmoney and placed into the matching
subdirectory. They are refreshed over time, so:

- Never hardcode report filenames or a fixed snapshot date.
- Discover report files at runtime from the relevant subdirectory.
- Filenames carry report dates and version suffixes (for example,
  `...-2026-11-11-V04.xls`). Parse them instead of assuming a single file.
- Prefer the newest report by report date/version when duplicates exist, and
  make reruns safe when files are replaced.
- Tolerate extra, older, or newly added files. Skip unknown files instead of
  failing the whole run.
- Treat all report files as read-only inputs. Do not edit, rename, or delete
  them unless explicitly asked.

## Data handling

- `*:Zone.Identifier` files are Windows download metadata; ignore them.
- Do not commit real account numbers, balances, or other personal financial
  data to public repositories or paste it into external services.

## Working conventions

- Ask before adding dependencies, changing directory structure, or deleting
  files. Approved parsing dependencies: `extrame/xls`, `excelize`,
  `ledongthuc/pdf`, and `shopspring/decimal`.
- Inspect the source report format before writing parsers or analysis code.
- Use actual transaction dates from report contents, not financial-year
  filenames. Merge overlapping report coverage instead of assuming the newest
  file is a superset.
- The monthly statement broker ledger cash is not the INDmoney wallet balance.
  Do not add both. Reconcile the economic total explicitly.

## Commands

Go is installed at `/usr/local/go/bin/go` in this WSL environment and may not be
on the tool shell's `PATH`.

```sh
/usr/local/go/bin/go run . -reports ./Reports
/usr/local/go/bin/go run . -reports ./Reports -history
/usr/local/go/bin/go test ./...
/usr/local/go/bin/go vet ./...
```

Every run also writes `portfolio_report.xlsx` with summary, holdings, history,
and warnings sheets plus charts. Use `-excel <path>` to change the path or
`-excel ""` to skip it. Use `-prices prices.csv -usdinr <rate>` for an offline
run. The race detector is unavailable until a C compiler is installed because
CGO is required.

Update this section when the CLI flags or test workflow change.

## Maintaining instructions

- Put stable, repository-wide guidance in this file.
- Append new, specific instructions from conversations to
  `.agents/instructions.md`.
- Keep both files concise and current rather than duplicating content.
