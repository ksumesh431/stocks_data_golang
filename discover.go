package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ReportFiles struct {
	LRS        []string
	Wallet     []string
	Orders     []string
	Holdings   []string
	Brokerage  []string
	Statements []string
}

func discoverReports(root string) (ReportFiles, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ReportFiles{}, fmt.Errorf("read reports directory: %w", err)
	}

	var files ReportFiles
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		paths, err := reportFiles(filepath.Join(root, dir))
		if err != nil {
			return ReportFiles{}, err
		}
		switch dir {
		case "deposit_and_withdrawl_reports":
			files.LRS = append(files.LRS, paths...)
		case "running_balance_reports":
			files.Wallet = append(files.Wallet, paths...)
		case "trade_reports":
			files.Orders = append(files.Orders, paths...)
		case "holding_reports":
			files.Holdings = append(files.Holdings, paths...)
		case "brokerage_reports":
			files.Brokerage = append(files.Brokerage, paths...)
		case "monthly_account_statements":
			files.Statements = append(files.Statements, paths...)
		default:
			continue
		}
	}
	sort.Strings(files.LRS)
	sort.Strings(files.Wallet)
	sort.Strings(files.Orders)
	sort.Strings(files.Holdings)
	sort.Strings(files.Brokerage)
	sort.Strings(files.Statements)
	return files, nil
}

func reportFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || strings.Contains(entry.Name(), ":Zone.Identifier") {
			continue
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension == ".xls" || extension == ".xlsx" || extension == ".pdf" {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	return paths, nil
}
