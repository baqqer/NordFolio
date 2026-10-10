package internal

import "testing"

func TestAllocationAVCO(t *testing.T) {
	db := newDatabase()
	db.Transactions = []Transaction{
		{ID: "1", BookingDate: "2026-01-01", Account: "A", TransactionType: "INDBETALING", Amount: 10000, Balance: 10000},
		{ID: "2", BookingDate: "2026-01-02", Account: "A", TransactionType: "KØBT", ISIN: "DK1", Symbol: "X",
			Quantity: 10, Price: 100, Amount: -1025, AcquisitionValue: 1025, AcquisitionCurrency: "DKK", TotalQuantity: 10, Balance: 8975},
		{ID: "3", BookingDate: "2026-01-03", Account: "A", TransactionType: "KØBT", ISIN: "DK1", Symbol: "X",
			Quantity: 10, Price: 200, Amount: -2025, AcquisitionValue: 2025, AcquisitionCurrency: "DKK", TotalQuantity: 20, Balance: 6950},
		{ID: "4", BookingDate: "2026-01-04", Account: "A", TransactionType: "SOLGT", ISIN: "DK1", Symbol: "X",
			Quantity: 5, Price: 300, Amount: 1475, TotalQuantity: 15, Balance: 8425},
	}

	a := BuildAllocation(db, nil)
	if len(a.Holdings) != 1 {
		t.Fatalf("Expected 1 holding, got %d", len(a.Holdings))
	}
	h := a.Holdings[0]

	// Average cost 3050/20 = 152.50 is kept for the remaining 15 units
	approx(t, "quantity", h.Quantity, 15)
	approx(t, "avg cost", h.AvgCost, 152.50)
	approx(t, "book value", h.BookValueDKK, 15*152.50)
	approx(t, "cash", a.CashDKK, 8425)
	approx(t, "total", a.TotalDKK, 8425+15*152.50)
	if h.Class != AssetClassSecurity || h.Currency != "DKK" {
		t.Errorf("Unclassified holding should default to Security/DKK, got %s/%s", h.Class, h.Currency)
	}
}

func TestAllocationPartialHistory(t *testing.T) {
	// The export starts after 50 units were already held and cash was 500
	db := newDatabase()
	db.Transactions = []Transaction{
		{ID: "1", BookingDate: "2026-03-01", Account: "A", TransactionType: "SOLGT", ISIN: "US1", Symbol: "Y",
			Quantity: 10, Price: 20, ExchangeRate: 7, Amount: 1400, ResultCurrency: "USD", TotalQuantity: 40, Balance: 1900},
	}

	a := BuildAllocation(db, nil)
	if len(a.Holdings) != 1 {
		t.Fatalf("Expected 1 holding, got %d", len(a.Holdings))
	}
	h := a.Holdings[0]
	approx(t, "quantity", h.Quantity, 40)
	approx(t, "avg cost (USD)", h.AvgCost, 20)
	approx(t, "book value", h.BookValueDKK, 40*20*7)
	approx(t, "cash", a.CashDKK, 1900)
	if h.Currency != "USD" {
		t.Errorf("currency = %q, want USD", h.Currency)
	}
}

func TestAllocationPerAccountPositions(t *testing.T) {
	// The same security in two accounts is tracked separately and then summed
	db := newDatabase()
	db.Transactions = []Transaction{
		{ID: "1", BookingDate: "2026-01-01", Account: "A", TransactionType: "KØBT", ISIN: "DK1", Quantity: 10, Price: 100, AcquisitionValue: 1000, AcquisitionCurrency: "DKK", TotalQuantity: 10},
		{ID: "2", BookingDate: "2026-01-02", Account: "B", TransactionType: "KØBT", ISIN: "DK1", Quantity: 10, Price: 200, AcquisitionValue: 2000, AcquisitionCurrency: "DKK", TotalQuantity: 10},
		{ID: "3", BookingDate: "2026-01-03", Account: "B", TransactionType: "SOLGT", ISIN: "DK1", Quantity: 10, Price: 250, TotalQuantity: 0},
	}

	a := BuildAllocation(db, nil)
	if len(a.Holdings) != 1 {
		t.Fatalf("Expected 1 holding, got %d", len(a.Holdings))
	}
	approx(t, "quantity", a.Holdings[0].Quantity, 10)
	approx(t, "book value", a.Holdings[0].BookValueDKK, 1000)
	if got := a.Holdings[0].Accounts; len(got) != 1 || got[0] != "A" {
		t.Errorf("accounts = %v, want [A]", got)
	}

	b := BuildAllocation(db, map[string]bool{"B": true})
	if len(b.Holdings) != 0 {
		t.Errorf("Account B should hold nothing, got %+v", b.Holdings)
	}
}

func TestAllocationEmpty(t *testing.T) {
	a := BuildAllocation(newDatabase(), nil)
	if a.TotalDKK != 0 || len(a.Holdings) != 0 || len(a.ByClass) != 3 {
		t.Errorf("Unexpected empty allocation: %+v", a)
	}
}

func TestSortTransactionsNumericIDs(t *testing.T) {
	txs := []Transaction{
		{ID: "10", BookingDate: "2026-01-01"},
		{ID: "9", BookingDate: "2026-01-01"},
		{ID: "1", BookingDate: "2026-01-02"},
	}
	SortTransactions(txs)
	if txs[0].ID != "9" || txs[1].ID != "10" || txs[2].ID != "1" {
		t.Errorf("Wrong order: %s %s %s", txs[0].ID, txs[1].ID, txs[2].ID)
	}
}
