package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Storage is a concurrency-safe JSON file database.
type Storage struct {
	filePath string
	mu       sync.RWMutex
	db       Database
}

func NewStorage(filePath string) (*Storage, error) {
	s := &Storage{filePath: filePath, db: newDatabase()}

	data, err := os.ReadFile(filePath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return s, s.save()
	case err != nil:
		return nil, err
	}

	if err := json.Unmarshal(data, &s.db); err != nil {
		return nil, err
	}
	s.db.ensureMaps()
	return s, nil
}

// save writes the database atomically so a crash never leaves a half-written file.
func (s *Storage) save() error {
	data, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.filePath), ".db-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.filePath)
}

// GetDB returns a copy of the database.
func (s *Storage) GetDB() Database {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.clone()
}

// MetadataUpdate holds user edits; only keys present are changed.
type MetadataUpdate struct {
	Classifications map[string]AssetClass `json:"classifications"`
	AssetNames      map[string]string     `json:"asset_names"`
	AssetSymbols    map[string]string     `json:"asset_symbols"`
	AccountNames    map[string]string     `json:"account_names"`
}

func (s *Storage) SaveMetadata(u MetadataUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range u.Classifications {
		if v == AssetClassSecurity || v == AssetClassETF {
			s.db.Classifications[k] = v
		}
	}
	for k, v := range u.AssetNames {
		s.db.AssetNames[k] = strings.TrimSpace(v)
	}
	for k, v := range u.AssetSymbols {
		s.db.AssetSymbols[k] = strings.TrimSpace(v)
	}
	for k, v := range u.AccountNames {
		if v = strings.TrimSpace(v); v == "" {
			v = k
		}
		s.db.AccountNames[k] = v
	}

	return s.save()
}

// AddTransactions appends transactions whose ID is not already stored and
// registers any new securities and accounts. It returns the number added.
func (s *Storage) AddTransactions(txs []Transaction) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := make(map[string]bool, len(s.db.Transactions))
	for _, tx := range s.db.Transactions {
		existing[tx.ID] = true
	}

	added := 0
	for _, tx := range txs {
		if existing[tx.ID] {
			continue
		}
		existing[tx.ID] = true
		s.db.Transactions = append(s.db.Transactions, tx)
		added++

		if tx.ISIN != "" {
			if _, ok := s.db.AssetSymbols[tx.ISIN]; !ok {
				s.db.AssetSymbols[tx.ISIN] = tx.Symbol
			}
			if _, ok := s.db.AssetNames[tx.ISIN]; !ok {
				s.db.AssetNames[tx.ISIN] = tx.Symbol
			}
			if _, ok := s.db.Classifications[tx.ISIN]; !ok {
				s.db.Classifications[tx.ISIN] = GuessAssetClass(tx.ISIN, tx.Symbol)
			}
		}
		if tx.Account != "" {
			if _, ok := s.db.AccountNames[tx.Account]; !ok {
				s.db.AccountNames[tx.Account] = tx.Account
			}
		}
	}

	if added == 0 {
		return 0, nil
	}
	return added, s.save()
}

// Reset deletes all transactions and settings.
func (s *Storage) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = newDatabase()
	return s.save()
}

// GuessAssetClass separates ETFs from other securities with a heuristic.
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
