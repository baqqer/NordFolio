package internal

import (
	"testing"
)

func TestIsInternalTransfer(t *testing.T) {
	visibleAccs := map[string]bool{
		"87654321": true,
		"12345678": true,
	}

	tests := []struct {
		tx       Transaction
		expected bool
	}{
		{
			Transaction{TransactionType: "INDSÆTTELSE", TransactionText: "Internal from 87654321"},
			true,
		},
		{
			Transaction{TransactionType: "HÆVNING", TransactionText: "Internal to 12345678"},
			true,
		},
		{
			Transaction{TransactionType: "INDBETALING", TransactionText: "FRA LØNKONTO"},
			false,
		},
		{
			Transaction{TransactionType: "INDSÆTTELSE", TransactionText: "Intern overførsel fra depot"},
			true,
		},
	}

	for _, test := range tests {
		result := IsInternalTransfer(test.tx, visibleAccs)
		if result != test.expected {
			t.Errorf("IsInternalTransfer(type=%q, text=%q) = %t; expected %t",
				test.tx.TransactionType, test.tx.TransactionText, result, test.expected)
		}
	}
}

func TestIsExternalCashFlow(t *testing.T) {
	visibleAccs := map[string]bool{
		"87654321": true,
		"12345678": true,
	}

	tests := []struct {
		tx       Transaction
		expected bool
	}{
		{
			Transaction{TransactionType: "INDBETALING", TransactionText: "FRA LØNKONTO"},
			true,
		},
		{
			Transaction{TransactionType: "INDSÆTTELSE", TransactionText: "Internal from 87654321"},
			false, // Internal
		},
		{
			Transaction{TransactionType: "KØBT", TransactionText: ""},
			false, // Buying security is not external cash flow (it's investing existing cash)
		},
		{
			Transaction{TransactionType: "INDSÆTTELSE", TransactionText: "FRA INVESTOR"},
			true, // External deposit
		},
	}

	for _, test := range tests {
		result := IsExternalCashFlow(test.tx, visibleAccs)
		if result != test.expected {
			t.Errorf("IsExternalCashFlow(type=%q, text=%q) = %t; expected %t",
				test.tx.TransactionType, test.tx.TransactionText, result, test.expected)
		}
	}
}

func TestAnalyzePortfolio(t *testing.T) {
	db := AppDatabase{
		Transactions: []Transaction{
			{
				ID:              "1",
				BookingDate:     "2026-01-01",
				Account:         "Acc1",
				TransactionType: "INDBETALING",
				Amount:          10000,
				AmountCurrency:  "DKK",
				Balance:         10000,
			},
			{
				ID:               "2",
				BookingDate:      "2026-01-02",
				Account:          "Acc1",
				TransactionType:  "KØBT",
				Symbol:           "TESTSEC",
				ISIN:             "DK1234567890",
				Quantity:         10,
				Price:            100,
				Amount:           -1025, // 1000 cost + 25 fees
				AmountCurrency:   "DKK",
				AcquisitionValue: 1025,
				Balance:          8975,
			},
		},
		Classifications: map[string]AssetClass{
			"DK1234567890": AssetClassSecurity,
		},
		AssetSymbols: map[string]string{
			"DK1234567890": "TESTSEC",
		},
		AssetNames: map[string]string{
			"DK1234567890": "Test Security",
		},
		ManualPrices: map[string]float64{
			"DK1234567890": 110, // Price went up from 100 to 110
		},
		ManualCurrencies: map[string]string{
			"DK1234567890": "DKK",
		},
		ExchangeRates: map[string]float64{
			"DKK": 1.0,
		},
	}

	analysis, err := AnalyzePortfolio(db, nil)
	if err != nil {
		t.Fatalf("Failed to analyze portfolio: %v", err)
	}

	// Verify current totals (Strictly Realized Return Model: securities held at cost basis)
	// Cash = 8975 DKK
	// Security book value = 1025 DKK (since 25 DKK acquisition fee is capitalized into cost basis)
	// Total portfolio value = 10000 DKK
	if analysis.TotalCashDKK != 8975 {
		t.Errorf("Expected TotalCashDKK 8975, got %f", analysis.TotalCashDKK)
	}
	if analysis.TotalSecuritiesDKK != 1025 {
		t.Errorf("Expected TotalSecuritiesDKK 1025, got %f", analysis.TotalSecuritiesDKK)
	}
	if analysis.TotalValueDKK != 10000 {
		t.Errorf("Expected TotalValueDKK 10000, got %f", analysis.TotalValueDKK)
	}

	// Invested capital is 10000 (from INDBETALING)
	// Realized Gain/Loss = 10000 - 10000 = 0 DKK (the fee is capitalized, no sales have occurred yet)
	// Realized Gain/Loss % = 0%
	if analysis.TotalGainLossDKK != 0 {
		t.Errorf("Expected TotalGainLossDKK 0, got %f", analysis.TotalGainLossDKK)
	}
	if analysis.TotalGainLossPct != 0 {
		t.Errorf("Expected TotalGainLossPct 0, got %f", analysis.TotalGainLossPct)
	}

	// Verify timeline series
	// 2026-01-01 should have cash = 10000, securities = 0, total = 10000, invested = 10000
	// 2026-01-02 should have cash = 8975, securities = 1025, total = 10000, invested = 10000
	if len(analysis.Dates) < 2 {
		t.Fatalf("Expected at least 2 date data points, got %d", len(analysis.Dates))
	}

	// First day:
	if analysis.Dates[0] != "2026-01-01" {
		t.Errorf("Expected first date 2026-01-01, got %s", analysis.Dates[0])
	}
	if analysis.CashSeries[0] != 10000 {
		t.Errorf("Expected day 1 cash 10000, got %f", analysis.CashSeries[0])
	}
	if analysis.TotalSeries[0] != 10000 {
		t.Errorf("Expected day 1 total 10000, got %f", analysis.TotalSeries[0])
	}

	// Second day:
	if analysis.Dates[1] != "2026-01-02" {
		t.Errorf("Expected second date 2026-01-02, got %s", analysis.Dates[1])
	}
	if analysis.CashSeries[1] != 8975 {
		t.Errorf("Expected day 2 cash 8975, got %f", analysis.CashSeries[1])
	}
	if analysis.SecuritiesSeries[1] != 1025 {
		t.Errorf("Expected day 2 securities 1025, got %f", analysis.SecuritiesSeries[1])
	}
	if analysis.TotalSeries[1] != 10000 {
		t.Errorf("Expected day 2 total 10000, got %f", analysis.TotalSeries[1])
	}
}
