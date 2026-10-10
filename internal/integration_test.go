package internal

import (
	"math"
	"os"
	"testing"
)

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Errorf("%s = %.4f, want %.4f", name, got, want)
	}
}

func loadSampleDB(t *testing.T) Database {
	t.Helper()
	f, err := os.Open("transactions-and-notes-export.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	txs, err := ParseNordnetCSV(f)
	if err != nil {
		t.Fatalf("Failed to parse sample CSV: %v", err)
	}
	if len(txs) != 15 {
		t.Fatalf("Expected 15 transactions, got %d", len(txs))
	}

	db := newDatabase()
	db.Transactions = txs
	db.Classifications = map[string]AssetClass{
		"IE000ANONYM0": AssetClassETF,
		"DK000ANONYM0": AssetClassSecurity,
		"US000ANONYM0": AssetClassSecurity,
	}
	db.AssetSymbols = map[string]string{
		"IE000ANONYM0": "FOO",
		"DK000ANONYM0": "BAR",
		"US000ANONYM0": "BAZ",
	}
	return db
}

func TestIntegrationSampleCSV(t *testing.T) {
	a := BuildAllocation(loadSampleDB(t), nil)

	// Latest "Saldo" per account: 6030.25 + 0.98
	approx(t, "cash", a.CashDKK, 6031.23)

	// BAR: 36 shares bought for 5993.80 DKK including brokerage
	approx(t, "securities", a.SecuritiesDKK, 5993.80)

	// FOO: 818.70 EUR @ 7.4907 + 800.92 EUR @ 7.4931
	fooDKK := 818.70*7.4907 + 800.92*7.4931
	approx(t, "ETFs", a.ETFsDKK, fooDKK)
	approx(t, "total", a.TotalDKK, 6031.23+5993.80+fooDKK)

	if a.AsOf != "2026-08-04" {
		t.Errorf("as_of = %q, want 2026-08-04", a.AsOf)
	}

	// BAZ was fully sold and must not appear
	if len(a.Holdings) != 2 {
		t.Fatalf("Expected 2 holdings, got %+v", a.Holdings)
	}
	foo, bar := a.Holdings[0], a.Holdings[1]
	if foo.Symbol != "FOO" || bar.Symbol != "BAR" {
		t.Fatalf("Holdings not sorted by book value: %s, %s", foo.Symbol, bar.Symbol)
	}
	approx(t, "FOO quantity", foo.Quantity, 128)
	approx(t, "FOO avg cost (EUR)", foo.AvgCost, (818.70+800.92)/128)
	if foo.Currency != "EUR" || foo.Class != AssetClassETF {
		t.Errorf("FOO currency/class = %s/%s", foo.Currency, foo.Class)
	}
	approx(t, "BAR quantity", bar.Quantity, 36)
	approx(t, "BAR avg cost (DKK)", bar.AvgCost, 5993.80/36)
	if bar.Name != "BAR" {
		t.Errorf("Name should fall back to symbol, got %q", bar.Name)
	}

	sum := 0.0
	for _, s := range a.ByClass {
		sum += s.Weight
	}
	approx(t, "class weights", sum, 100)

	byAccount := map[string]float64{}
	for _, s := range a.ByAccount {
		byAccount[s.Key] = s.ValueDKK
	}
	approx(t, "account 12345678", byAccount["12345678"], 6030.25+5993.80+fooDKK)
	approx(t, "account 87654321", byAccount["87654321"], 0.98)

	byCurrency := map[string]float64{}
	for _, s := range a.ByCurrency {
		byCurrency[s.Key] = s.ValueDKK
	}
	approx(t, "EUR", byCurrency["EUR"], fooDKK)
	approx(t, "DKK", byCurrency["DKK"], 5993.80+6031.23)
}

func TestIntegrationAccountFilter(t *testing.T) {
	a := BuildAllocation(loadSampleDB(t), map[string]bool{"87654321": true})

	approx(t, "cash", a.CashDKK, 0.98)
	approx(t, "total", a.TotalDKK, 0.98)
	if len(a.Holdings) != 0 {
		t.Errorf("Expected no holdings, got %+v", a.Holdings)
	}
}
