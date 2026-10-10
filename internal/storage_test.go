package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorageAddTransactionsDedupes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")
	s, err := NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}

	txs := []Transaction{
		{ID: "1", Account: "A", ISIN: "IE1", Symbol: "ETF1"},
		{ID: "2", Account: "A", ISIN: "DK1", Symbol: "STOCK"},
	}
	if n, err := s.AddTransactions(txs); err != nil || n != 2 {
		t.Fatalf("first import: added %d, err %v", n, err)
	}
	if n, err := s.AddTransactions(append(txs, Transaction{ID: "3", Account: "B"})); err != nil || n != 1 {
		t.Fatalf("overlapping import: added %d, err %v", n, err)
	}

	// Reopen from disk
	s, err = NewStorage(path)
	if err != nil {
		t.Fatal(err)
	}
	db := s.GetDB()
	if len(db.Transactions) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(db.Transactions))
	}
	if db.Classifications["IE1"] != AssetClassETF || db.Classifications["DK1"] != AssetClassSecurity {
		t.Errorf("Unexpected classifications: %v", db.Classifications)
	}
	if db.AccountNames["B"] != "B" {
		t.Errorf("New account should default to its ID, got %q", db.AccountNames["B"])
	}
}

func TestStorageSaveMetadata(t *testing.T) {
	s, err := NewStorage(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTransactions([]Transaction{{ID: "1", Account: "A", ISIN: "DK1", Symbol: "X"}}); err != nil {
		t.Fatal(err)
	}

	err = s.SaveMetadata(MetadataUpdate{
		Classifications: map[string]AssetClass{"DK1": AssetClassETF, "DK2": "Bogus"},
		AssetNames:      map[string]string{"DK1": "  Index Fund "},
		AccountNames:    map[string]string{"A": " ", "B": "Aktiesparekonto"},
	})
	if err != nil {
		t.Fatal(err)
	}

	db := s.GetDB()
	if db.Classifications["DK1"] != AssetClassETF {
		t.Errorf("classification not saved")
	}
	if _, ok := db.Classifications["DK2"]; ok {
		t.Errorf("invalid class should be ignored")
	}
	if db.AssetNames["DK1"] != "Index Fund" {
		t.Errorf("name = %q", db.AssetNames["DK1"])
	}
	if db.AccountNames["A"] != "A" || db.AccountNames["B"] != "Aktiesparekonto" {
		t.Errorf("account names = %v", db.AccountNames)
	}
}

func TestStorageLoadsLegacyDatabase(t *testing.T) {
	// Databases from earlier versions contain price and FX settings
	path := filepath.Join(t.TempDir(), "db.json")
	legacy := `{
  "transactions": [{"id": "1", "account": "A", "isin": "DK1", "symbol": "X"}],
  "classifications": {"DK1": "ETF"},
  "asset_names": {"DK1": "Fund"},
  "asset_symbols": {"DK1": "X"},
  "manual_prices": {"DK1": 100},
  "manual_currencies": {"DK1": "DKK"},
  "exchange_rates": {"DKK": 1, "EUR": 7.46},
  "account_names": null,
  "auto_fetch_rates": true
}`
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := NewStorage(path)
	if err != nil {
		t.Fatalf("legacy database failed to load: %v", err)
	}
	db := s.GetDB()
	if len(db.Transactions) != 1 || db.Classifications["DK1"] != AssetClassETF || db.AccountNames == nil {
		t.Errorf("legacy database not loaded correctly: %+v", db)
	}
}

func TestStorageReset(t *testing.T) {
	s, err := NewStorage(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTransactions([]Transaction{{ID: "1", Account: "A"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	if db := s.GetDB(); len(db.Transactions) != 0 || len(db.AccountNames) != 0 {
		t.Errorf("database not empty after reset: %+v", db)
	}
}
