package internal

import (
	"bytes"
	"testing"

	"golang.org/x/text/encoding/unicode"
)

func TestParseNordnetFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"", 0},
		{"0", 0},
		{"12", 12},
		{"0,3", 0.3},
		{"-5993,8", -5993.8},
		{"6.536,7", 6536.7}, // thousands dot, comma decimal
		{"12,544", 12.544},
		{"7,4907", 7.4907},
	}

	for _, test := range tests {
		result := ParseNordnetFloat(test.input)
		if result != test.expected {
			t.Errorf("ParseNordnetFloat(%q) = %f; expected %f", test.input, result, test.expected)
		}
	}
}

func TestParseNordnetCSV(t *testing.T) {
	mockCSV := `Id	Bogføringsdag	Handelsdag	Valørdag	Depot	Transaktionstype	Værdipapirer	ISIN	Antal	Kurs	Rente	Samlede afgifter	Valuta	Beløb	Valuta	Indkøbsværdi	Valuta	Resultat	Valuta	Totalt antal	Saldo	Vekslingskurs	Transaktionstekst	Makuleringsdato	Notanummer	Verifikationsnummer	Kurtage	Valuta	Middelkurs	Oprindelig rente
2621339587	2026-07-22	2026-07-22	2026-07-23	37185618	SOLGT	MVIN	US5949603048	1	0,3	0	0,98	DKK	0,98	DKK			-29,47	USD	0	0,98	6,5367			2172694187	2172694187	0,98	DKK		
2599877585	2026-07-03	2026-07-03	2026-07-07	70530241	KØBT	MAJDCS	DK0062615746	36	165,8	0	25	DKK	-5993,8	DKK	5993,8	DKK	0	DKK	36	30,25				2166722686	2166722686	25	DKK		
`

	reader := bytes.NewReader([]byte(mockCSV))
	txs, err := ParseNordnetCSV(reader)
	if err != nil {
		t.Fatalf("Failed to parse CSV: %v", err)
	}

	if len(txs) != 2 {
		t.Fatalf("Expected 2 transactions, got %d", len(txs))
	}

	// Verify first transaction (SOLGT)
	t1 := txs[0]
	if t1.ID != "2621339587" {
		t.Errorf("Expected ID 2621339587, got %q", t1.ID)
	}
	if t1.TransactionType != "SOLGT" {
		t.Errorf("Expected SOLGT, got %q", t1.TransactionType)
	}
	if t1.Symbol != "MVIN" {
		t.Errorf("Expected MVIN, got %q", t1.Symbol)
	}
	if t1.ISIN != "US5949603048" {
		t.Errorf("Expected US5949603048, got %q", t1.ISIN)
	}
	if t1.Quantity != 1 {
		t.Errorf("Expected quantity 1, got %f", t1.Quantity)
	}
	if t1.Price != 0.3 {
		t.Errorf("Expected price 0.3, got %f", t1.Price)
	}
	if t1.Amount != 0.98 {
		t.Errorf("Expected amount 0.98, got %f", t1.Amount)
	}
	if t1.Result != -29.47 {
		t.Errorf("Expected result -29.47, got %f", t1.Result)
	}
	if t1.ExchangeRate != 6.5367 {
		t.Errorf("Expected ExchangeRate 6.5367, got %f", t1.ExchangeRate)
	}

	// Verify second transaction (KØBT)
	t2 := txs[1]
	if t2.ID != "2599877585" {
		t.Errorf("Expected ID 2599877585, got %q", t2.ID)
	}
	if t2.TransactionType != "KØBT" {
		t.Errorf("Expected KØBT, got %q", t2.TransactionType)
	}
	if t2.Quantity != 36 {
		t.Errorf("Expected quantity 36, got %f", t2.Quantity)
	}
	if t2.Price != 165.8 {
		t.Errorf("Expected price 165.8, got %f", t2.Price)
	}
	if t2.Amount != -5993.8 {
		t.Errorf("Expected amount -5993.8, got %f", t2.Amount)
	}
	if t2.AcquisitionValue != 5993.8 {
		t.Errorf("Expected AcquisitionValue 5993.8, got %f", t2.AcquisitionValue)
	}
	if t2.Balance != 30.25 {
		t.Errorf("Expected Balance 30.25, got %f", t2.Balance)
	}
}

func TestParseUTF16CSV(t *testing.T) {
	text := "Id	Bogføringsdag	Handelsdag	Valørdag	Depot	Transaktionstype\n123	2026-08-04	2026-08-04	2026-08-04	70530241	INDSÆTTELSE\n"
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder()
	utf16Bytes, err := encoder.Bytes([]byte(text))
	if err != nil {
		t.Fatalf("Failed to encode to UTF-16LE: %v", err)
	}

	txs, err := ParseNordnetCSV(bytes.NewReader(utf16Bytes))
	if err != nil {
		t.Fatalf("Failed to parse UTF-16LE CSV: %v", err)
	}

	if len(txs) != 1 {
		t.Fatalf("Expected 1 transaction, got %d", len(txs))
	}

	if txs[0].ID != "123" {
		t.Errorf("Expected ID '123', got %q", txs[0].ID)
	}
	if txs[0].TransactionType != "INDSÆTTELSE" {
		t.Errorf("Expected 'INDSÆTTELSE', got %q", txs[0].TransactionType)
	}
}
