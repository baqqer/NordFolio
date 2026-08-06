package internal

import (
	"bytes"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// Index mappings for Nordnet CSV export
const (
	colID                 = 0
	colBookingDate        = 1
	colTradeDate          = 2
	colValueDate          = 3
	colAccount            = 4
	colTransactionType    = 5
	colSymbol             = 6
	colISIN               = 7
	colQuantity           = 8
	colPrice              = 9
	colInterest           = 10
	colFees               = 11
	colFeeCurrency        = 12
	colAmount             = 13
	colAmountCurrency     = 14
	colAcquisitionValue   = 15
	colAcquisitionCurrency = 16
	colResult             = 17
	colResultCurrency     = 18
	colTotalQuantity      = 19
	colBalance            = 20
	colExchangeRate       = 21
	colTransactionText    = 22
	colCancellationDate   = 23
	colNoteNumber         = 24
	colVerificationNumber  = 25
	colBrokerage          = 26
	colBrokerageCurrency  = 27
	colAvgPrice           = 28
	colOriginalInterest   = 29
)

// ParseNordnetFloat converts a string formatted in the Danish/European numeric style (comma as decimal separator) to a float64
func ParseNordnetFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Nordnet exports use comma (,) as the decimal separator.
	// Any dots (.) are thousand separators.
	// Replace all dots with nothing, then replace commas with dots.
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

// DecodeToUTF8 converts bytes of unknown encoding (UTF-16LE, UTF-16BE, UTF-8, WINDOWS-1252) into valid UTF-8
func DecodeToUTF8(data []byte) []byte {
	if len(data) < 2 {
		return data
	}

	// 1. Check for UTF-16 Byte Order Marks (BOM)
	if data[0] == 0xFF && data[1] == 0xFE {
		// UTF-16LE
		dec := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder()
		if res, err := dec.Bytes(data); err == nil {
			return res
		}
	}
	if data[0] == 0xFE && data[1] == 0xFF {
		// UTF-16BE
		dec := unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder()
		if res, err := dec.Bytes(data); err == nil {
			return res
		}
	}

	// 2. Heuristic check for UTF-16 without BOM (common in localized system exports)
	// Look at the density of null bytes (0x00) in the first 100 bytes
	limit := len(data)
	if limit > 100 {
		limit = 100
	}
	nullCount := 0
	evenNulls := 0
	oddNulls := 0
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			nullCount++
			if i%2 == 0 {
				evenNulls++
			} else {
				oddNulls++
			}
		}
	}

	// In ASCII text encoded in UTF-16, roughly 50% of the bytes are null bytes
	if nullCount > 0 && float64(nullCount)/float64(limit) > 0.3 {
		if evenNulls > oddNulls {
			// UTF-16BE (nulls are at even indices 0, 2, 4...)
			dec := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()
			if res, err := dec.Bytes(data); err == nil {
				return res
			}
		} else {
			// UTF-16LE (nulls are at odd indices 1, 3, 5...)
			dec := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
			if res, err := dec.Bytes(data); err == nil {
				return res
			}
		}
	}

	// 3. Fallback to UTF-8 vs WINDOWS-1252
	// Remove standard UTF-8 BOM if present
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	// If it is valid UTF-8 and contains no stray null bytes (which should be rare in plain text CSV)
	if utf8.Valid(data) && !bytes.Contains(data, []byte{0}) {
		return data
	}

	// Decode from WINDOWS-1252 (Standard single-byte West European encoding)
	decoder := charmap.Windows1252.NewDecoder()
	if decoded, err := decoder.Bytes(data); err == nil {
		return decoded
	}

	return data
}

// ParseNordnetCSV parses Nordnet's tab-delimited CSV, auto-detecting encoding
func ParseNordnetCSV(r io.Reader) ([]Transaction, error) {
	// Read everything into a buffer to decode
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	utf8Data := DecodeToUTF8(data)

	csvReader := csv.NewReader(bytes.NewReader(utf8Data))
	csvReader.Comma = '\t'
	csvReader.FieldsPerRecord = -1 // Flexible column counts
	csvReader.LazyQuotes = true

	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	var transactions []Transaction

	for idx, row := range rows {
		// Skip header row (identifiable by "Id" or "Bogføringsdag" in first columns)
		if idx == 0 {
			continue
		}
		if len(row) < 6 {
			// Row is too short to be a valid transaction row
			continue
		}

		// Ensure the row has enough columns
		id := strings.TrimSpace(row[colID])
		if id == "" {
			continue
		}

		tx := Transaction{
			ID:              id,
			BookingDate:     strings.TrimSpace(row[colBookingDate]),
			TradeDate:       strings.TrimSpace(row[colTradeDate]),
			ValueDate:       strings.TrimSpace(row[colValueDate]),
			Account:         strings.TrimSpace(row[colAccount]),
			TransactionType: strings.TrimSpace(row[colTransactionType]),
		}

		// Parse remaining optional/index-safe columns
		if len(row) > colSymbol {
			tx.Symbol = strings.TrimSpace(row[colSymbol])
		}
		if len(row) > colISIN {
			tx.ISIN = strings.TrimSpace(row[colISIN])
		}
		if len(row) > colQuantity {
			tx.Quantity = ParseNordnetFloat(row[colQuantity])
		}
		if len(row) > colPrice {
			tx.Price = ParseNordnetFloat(row[colPrice])
		}
		if len(row) > colInterest {
			tx.Interest = ParseNordnetFloat(row[colInterest])
		}
		if len(row) > colFees {
			tx.Fees = ParseNordnetFloat(row[colFees])
		}
		if len(row) > colFeeCurrency {
			tx.FeeCurrency = strings.TrimSpace(row[colFeeCurrency])
		}
		if len(row) > colAmount {
			tx.Amount = ParseNordnetFloat(row[colAmount])
		}
		if len(row) > colAmountCurrency {
			tx.AmountCurrency = strings.TrimSpace(row[colAmountCurrency])
		}
		if len(row) > colAcquisitionValue {
			tx.AcquisitionValue = ParseNordnetFloat(row[colAcquisitionValue])
		}
		if len(row) > colAcquisitionCurrency {
			tx.AcquisitionCurrency = strings.TrimSpace(row[colAcquisitionCurrency])
		}
		if len(row) > colResult {
			tx.Result = ParseNordnetFloat(row[colResult])
		}
		if len(row) > colResultCurrency {
			tx.ResultCurrency = strings.TrimSpace(row[colResultCurrency])
		}
		if len(row) > colTotalQuantity {
			tx.TotalQuantity = ParseNordnetFloat(row[colTotalQuantity])
		}
		if len(row) > colBalance {
			tx.Balance = ParseNordnetFloat(row[colBalance])
		}
		if len(row) > colExchangeRate {
			tx.ExchangeRate = ParseNordnetFloat(row[colExchangeRate])
		}
		if len(row) > colTransactionText {
			tx.TransactionText = strings.TrimSpace(row[colTransactionText])
		}
		if len(row) > colCancellationDate {
			tx.CancellationDate = strings.TrimSpace(row[colCancellationDate])
		}
		if len(row) > colNoteNumber {
			tx.NoteNumber = strings.TrimSpace(row[colNoteNumber])
		}
		if len(row) > colVerificationNumber {
			tx.VerificationNumber = strings.TrimSpace(row[colVerificationNumber])
		}
		if len(row) > colBrokerage {
			tx.Brokerage = ParseNordnetFloat(row[colBrokerage])
		}
		if len(row) > colBrokerageCurrency {
			tx.BrokerageCurrency = strings.TrimSpace(row[colBrokerageCurrency])
		}
		if len(row) > colAvgPrice {
			tx.AvgPrice = ParseNordnetFloat(row[colAvgPrice])
		}
		if len(row) > colOriginalInterest {
			tx.OriginalInterest = ParseNordnetFloat(row[colOriginalInterest])
		}

		transactions = append(transactions, tx)
	}

	return transactions, nil
}
