package internal

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
)

type Storage struct {
	filePath string
	mu       sync.RWMutex
	db       AppDatabase
}

func NewStorage(filePath string) (*Storage, error) {
	s := &Storage{
		filePath: filePath,
	}
	err := s.load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Initialize with defaults
			s.db = AppDatabase{
				Transactions:     make([]Transaction, 0),
				Classifications:  make(map[string]AssetClass),
				AssetNames:       make(map[string]string),
				AssetSymbols:     make(map[string]string),
				ManualPrices:     make(map[string]float64),
				ManualCurrencies: make(map[string]string),
				ExchangeRates: map[string]float64{
					"DKK": 1.0,
					"EUR": 7.46,
					"USD": 6.90,
				},
				AccountNames:     make(map[string]string),
				AutoFetchRates:   true,
			}
			err = s.save()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	// In case loaded DB has nil maps (due to older schema or manual JSON edit)
	s.ensureMaps()
	return s, nil
}

func (s *Storage) ensureMaps() {
	if s.db.Classifications == nil {
		s.db.Classifications = make(map[string]AssetClass)
	}
	if s.db.AssetNames == nil {
		s.db.AssetNames = make(map[string]string)
	}
	if s.db.AssetSymbols == nil {
		s.db.AssetSymbols = make(map[string]string)
	}
	if s.db.ManualPrices == nil {
		s.db.ManualPrices = make(map[string]float64)
	}
	if s.db.ManualCurrencies == nil {
		s.db.ManualCurrencies = make(map[string]string)
	}
	if s.db.ExchangeRates == nil {
		s.db.ExchangeRates = map[string]float64{
			"DKK": 1.0,
			"EUR": 7.46,
			"USD": 6.90,
		}
	}
	if s.db.AccountNames == nil {
		s.db.AccountNames = make(map[string]string)
	}
}

func (s *Storage) load() error {
	f, err := os.Open(s.filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.db)
}

func (s *Storage) save() error {
	data, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// GetDB returns a copy of the database
func (s *Storage) GetDB() AppDatabase {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Perform a deep copy of transactions
	txs := make([]Transaction, len(s.db.Transactions))
	copy(txs, s.db.Transactions)

	// Copy maps
	classifications := make(map[string]AssetClass)
	for k, v := range s.db.Classifications {
		classifications[k] = v
	}

	names := make(map[string]string)
	for k, v := range s.db.AssetNames {
		names[k] = v
	}

	symbols := make(map[string]string)
	for k, v := range s.db.AssetSymbols {
		symbols[k] = v
	}

	prices := make(map[string]float64)
	for k, v := range s.db.ManualPrices {
		prices[k] = v
	}

	currencies := make(map[string]string)
	for k, v := range s.db.ManualCurrencies {
		currencies[k] = v
	}

	rates := make(map[string]float64)
	for k, v := range s.db.ExchangeRates {
		rates[k] = v
	}

	accNames := make(map[string]string)
	for k, v := range s.db.AccountNames {
		accNames[k] = v
	}

	return AppDatabase{
		Transactions:     txs,
		Classifications:  classifications,
		AssetNames:       names,
		AssetSymbols:     symbols,
		ManualPrices:     prices,
		ManualCurrencies: currencies,
		ExchangeRates:    rates,
		AccountNames:     accNames,
		AutoFetchRates:   s.db.AutoFetchRates,
	}
}

// SaveClassifications updates the asset classifications, friendly names, symbols, manual prices/currencies
func (s *Storage) SaveAssetMetadata(
	classifications map[string]AssetClass,
	names map[string]string,
	prices map[string]float64,
	currencies map[string]string,
	rates map[string]float64,
	accountNames map[string]string,
	autoFetchRates bool,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureMaps()

	s.db.AutoFetchRates = autoFetchRates

	for k, v := range classifications {
		s.db.Classifications[k] = v
	}
	for k, v := range names {
		s.db.AssetNames[k] = v
	}
	for k, v := range prices {
		s.db.ManualPrices[k] = v
	}
	for k, v := range currencies {
		s.db.ManualCurrencies[k] = v
	}
	for k, v := range rates {
		s.db.ExchangeRates[k] = v
	}
	for k, v := range accountNames {
		s.db.AccountNames[k] = v
	}

	return s.save()
}

// AddTransactions appends only non-duplicate transactions (by checking ID) and returns number of added transactions
func (s *Storage) AddTransactions(txs []Transaction) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureMaps()

	existingIDs := make(map[string]bool)
	for _, tx := range s.db.Transactions {
		existingIDs[tx.ID] = true
	}

	addedCount := 0
	for _, tx := range txs {
		if !existingIDs[tx.ID] {
			s.db.Transactions = append(s.db.Transactions, tx)
			existingIDs[tx.ID] = true
			addedCount++

			// Auto-register any new assets in asset symbols and classifications (defaulting to Security/Stock)
			if tx.ISIN != "" {
				if _, ok := s.db.AssetSymbols[tx.ISIN]; !ok {
					s.db.AssetSymbols[tx.ISIN] = tx.Symbol
				}
				if _, ok := s.db.Classifications[tx.ISIN]; !ok {
					s.db.Classifications[tx.ISIN] = GuessAssetClass(tx.ISIN, tx.Symbol)
				}
				if _, ok := s.db.AssetNames[tx.ISIN]; !ok {
					s.db.AssetNames[tx.ISIN] = tx.Symbol // default name to symbol
				}
				// Default manual price if not present
				if _, ok := s.db.ManualPrices[tx.ISIN]; !ok {
					if tx.Price > 0 {
						s.db.ManualPrices[tx.ISIN] = tx.Price
						s.db.ManualCurrencies[tx.ISIN] = tx.AcquisitionCurrency
						if s.db.ManualCurrencies[tx.ISIN] == "" {
							s.db.ManualCurrencies[tx.ISIN] = tx.AmountCurrency
						}
						if s.db.ManualCurrencies[tx.ISIN] == "" {
							s.db.ManualCurrencies[tx.ISIN] = "DKK"
						}
					}
				}
			}

			// Auto-register new accounts with default names (the account ID itself)
			if tx.Account != "" {
				if _, ok := s.db.AccountNames[tx.Account]; !ok {
					s.db.AccountNames[tx.Account] = tx.Account
				}
			}

			// Auto-register any new currencies discovered in transactions
			if tx.AmountCurrency != "" {
				if _, ok := s.db.ExchangeRates[tx.AmountCurrency]; !ok {
					s.db.ExchangeRates[tx.AmountCurrency] = 1.0
				}
			}
			if tx.AcquisitionCurrency != "" {
				if _, ok := s.db.ExchangeRates[tx.AcquisitionCurrency]; !ok {
					s.db.ExchangeRates[tx.AcquisitionCurrency] = 1.0
				}
			}
		}
	}

	if addedCount > 0 {
		err := s.save()
		if err != nil {
			return 0, err
		}
	}

	return addedCount, nil
}

// GuessAssetClass heuristics to automatically separate Stocks/Securities from ETFs
func GuessAssetClass(isin, symbol string) AssetClass {
	isin = strings.ToUpper(isin)
	symbol = strings.ToUpper(symbol)

	// Many ETFs are Irish (IE) or Luxembourgish (LU) domiciled,
	// and often contain "ETF" or "UCITS" in their symbol or name.
	if strings.Contains(symbol, "ETF") || strings.Contains(symbol, "UCITS") || strings.Contains(symbol, "ACC") || strings.Contains(symbol, "DIST") {
		return AssetClassETF
	}
	if strings.HasPrefix(isin, "IE") || strings.HasPrefix(isin, "LU") {
		return AssetClassETF
	}
	return AssetClassSecurity
}
