# Project-specific instructions

This file is loaded by OpenCode through the `instructions` list in
`.opencode/opencode.jsonc`. It complements `AGENTS.md`.

Use it for durable, specific rules and preferences that come up during
conversations. Keep `AGENTS.md` as the stable project overview; append
accumulated instructions here.

## Instructions

<!-- Add entries below as they are agreed on. Keep each one concise. -->

- 2026-11-11 — Use Go for this project. Before generating Go code, load the
  relevant Go skill(s) for the task: `golang-pro` by default, plus
  `golang-safety`, `golang-concurrency`, `golang-data-structures`,
  `golang-design-patterns`, `golang-performance`, `golang-benchmark`, or
  `golang-troubleshooting` when the work touches that area.
- 2026-11-11 — Keep the code as simple and readable as possible. No unnecessary
  complexity; this is a personal project.
- 2026-11-11 — `Reports/` is refreshed manually from INDmoney. Code must
  discover report files dynamically and handle new, replaced, and multiple
  versions without hardcoded filenames.
- 2026-11-11 — Plan approved. Implement the portfolio-level INR/USD/FX
  return calculator described in `.agents/plan.md`, using the report roles and
  data rules recorded there.
- 2026-11-11 — Initial implementation completed. Go is at
  `/usr/local/go/bin/go`; use that path in tool commands. Prefer
  `-prices` + `-usdinr` for deterministic offline runs.
- 2026-11-11 — Keep the CLI output and also generate an Excel report
  (`portfolio_report.xlsx` by default) with summary, holdings, history, warnings,
  and charts. `-excel ""` skips it. Add `-history` when INR history columns and
  XIRR are wanted in the workbook.
