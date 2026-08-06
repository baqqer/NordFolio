package internal

import (
	"os"
	"testing"
)

func TestIntegrationActualCSV(t *testing.T) {
	// Let's use our ParseNordnetCSV with bytes.NewReader
	importFile, err := os.Open("transactions-and-notes-export.csv")
	if err != nil {
		t.Fatalf("Failed to open transactions-and-notes-export.csv: %v", err)
	}
	defer importFile.Close()

	parsedTxs, err := ParseNordnetCSV(importFile)
	if err != nil {
		t.Fatalf("Failed to parse actual CSV: %v", err)
	}

	if len(parsedTxs) == 0 {
		t.Fatalf("No transactions parsed from actual CSV")
	}

	t.Logf("Parsed %d transactions successfully from actual Nordnet CSV", len(parsedTxs))

	// 3. Setup database
	db := AppDatabase{
		Transactions: parsedTxs,
		Classifications: map[string]AssetClass{
			"IE000ANONYM0": AssetClassETF,      // FOO
			"DK000ANONYM0": AssetClassSecurity, // BAR
			"US000ANONYM0": AssetClassSecurity, // BAZ
		},
		AssetSymbols: map[string]string{
			"IE000ANONYM0": "FOO",
			"DK000ANONYM0": "BAR",
			"US000ANONYM0": "BAZ",
		},
		ManualPrices: map[string]float64{
			"IE000ANONYM0": 12.66,
			"DK000ANONYM0": 165.8,
			"US000ANONYM0": 0.3,
		},
		ManualCurrencies: map[string]string{
			"IE000ANONYM0": "EUR",
			"DK000ANONYM0": "DKK",
			"US000ANONYM0": "USD",
		},
		ExchangeRates: map[string]float64{
			"DKK": 1.0,
			"EUR": 7.4931,
			"USD": 6.5367,
		},
	}

	// 4. Run Analysis
	analysis, err := AnalyzePortfolio(db, nil)
	if err != nil {
		t.Fatalf("Failed to analyze actual CSV: %v", err)
	}

	t.Logf("=== Portfolio Valuation Results ===")
	t.Logf("Total Value: %.2f DKK", analysis.TotalValueDKK)
	t.Logf("Total Cash: %.2f DKK", analysis.TotalCashDKK)
	t.Logf("Total Securities: %.2f DKK", analysis.TotalSecuritiesDKK)
	t.Logf("Total ETFs: %.2f DKK", analysis.TotalETFsDKK)
	t.Logf("Total Book Value: %.2f DKK", analysis.TotalBookValueDKK)
	t.Logf("Total Gain/Loss: %.2f DKK (%.2f%%)", analysis.TotalGainLossDKK, analysis.TotalGainLossPct)

	// 5. Verification checks
	// Let's assert that we have the exact cash balance that matches the newest transaction's balance!
	// Newest cash balance of 12345678 is 6030.25 (after transaction 2634966455)
	// Newest cash balance of 87654321 is 0.98 (after transaction 2634966281)
	// Total cash must be 6030.25 + 0.98 = 6031.23 DKK!
	expectedCash := 6031.23
	if analysis.TotalCashDKK != expectedCash {
		t.Errorf("Cash balances do not align. Expected %.2f DKK, got %.2f DKK", expectedCash, analysis.TotalCashDKK)
	} else {
		t.Logf("SUCCESS: Cash balances match exactly at %.2f DKK", expectedCash)
	}

	// Verify FOO holdings are 128 shares!
	var webnQty float64
	var majdcsQty float64
	var mvinQty float64

	for _, asset := range analysis.Assets {
		if asset.Symbol == "FOO" {
			webnQty = asset.Quantity
		} else if asset.Symbol == "BAR" {
			majdcsQty = asset.Quantity
		} else if asset.Symbol == "BAZ" {
			mvinQty = asset.Quantity
		}
	}

	if webnQty != 128 {
		t.Errorf("Expected FOO quantity 128, got %f", webnQty)
	} else {
		t.Logf("SUCCESS: FOO holding quantity is exactly 128 shares")
	}

	if majdcsQty != 36 {
		t.Errorf("Expected BAR quantity 36, got %f", majdcsQty)
	} else {
		t.Logf("SUCCESS: BAR holding quantity is exactly 36 shares")
	}

	if mvinQty != 0 {
		t.Errorf("Expected BAZ quantity 0 (fully sold), got %f", mvinQty)
	} else {
		t.Logf("SUCCESS: BAZ holding quantity is exactly 0 shares (successfully sold)")
	}

	// Verify that starting Invested Capital has been initialized to the pre-existing balance of 6158.04 + 1.96 = 6160.00 DKK!
	if len(analysis.InvestedCapitalSeries) > 0 {
		startInvested := analysis.InvestedCapitalSeries[0]
		expectedStartInvested := 6160.00
		// Account for float precision tolerance of 0.05 DKK
		if startInvested < expectedStartInvested-0.05 || startInvested > expectedStartInvested+0.05 {
			t.Errorf("Starting Invested Capital did not initialize correctly. Expected close to %.2f, got %.2f", expectedStartInvested, startInvested)
		} else {
			t.Logf("SUCCESS: Starting Invested Capital initialized correctly at %.2f DKK", startInvested)
		}
	}
}
