package main

import (
	"os"
	"testing"
)

func TestCurrentReportsLoad(t *testing.T) {
	if _, err := os.Stat("Reports"); err != nil {
		t.Skip("Reports directory is not present")
	}
	files, err := discoverReports("Reports")
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := loadDataset(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(dataset.Flows) == 0 {
		t.Fatal("no external flows parsed")
	}
	if len(dataset.Trades) == 0 {
		t.Fatal("no trades parsed")
	}
	if len(dataset.Holdings) == 0 {
		t.Fatal("no holdings parsed")
	}
	if len(dataset.Statements) == 0 {
		t.Fatal("no monthly statements parsed")
	}
	latest := latestHoldings(dataset)
	if latest.Date.IsZero() {
		t.Fatal("latest holdings has no date")
	}
	for _, holding := range latest.Holdings {
		if holding.Symbol == "" {
			t.Fatal("holding without a symbol")
		}
		if holding.Quantity.IsNegative() {
			t.Fatalf("negative holding quantity for %s", holding.Symbol)
		}
	}
}
