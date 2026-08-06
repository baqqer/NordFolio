package internal

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Server struct {
	storage      *Storage
	disableFXAPI bool
}

func NewServer(storage *Storage, disableFXAPI bool) *Server {
	return &Server{storage: storage, disableFXAPI: disableFXAPI}
}

// GetPortfolio handles GET /api/portfolio
func (s *Server) GetPortfolio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	accountsParam := r.URL.Query().Get("accounts")
	visibleAccounts := make(map[string]bool)
	if accountsParam != "" {
		for _, acc := range strings.Split(accountsParam, ",") {
			acc = strings.TrimSpace(acc)
			if acc != "" {
				visibleAccounts[acc] = true
			}
		}
	}

	db := s.storage.GetDB()
	analysis, err := AnalyzePortfolio(db, visibleAccounts)
	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "Failed to analyze portfolio: "+err.Error())
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"analysis":   analysis,
		"db_summary": s.getDbSummary(db),
		"db":         db,
	})
}

// UploadCSV handles POST /api/upload
func (s *Server) UploadCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10MB max
	if err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Missing file: "+err.Error())
		return
	}
	defer file.Close()

	txs, err := ParseNordnetCSV(file)
	if err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Failed to parse CSV file: "+err.Error())
		return
	}

	if len(txs) == 0 {
		s.respondWithError(w, http.StatusBadRequest, "No valid transaction rows found in the CSV")
		return
	}

	added, err := s.storage.AddTransactions(txs)
	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "Failed to save transactions: "+err.Error())
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "CSV imported successfully",
		"parsed":  len(txs),
		"added":   added,
		"skipped": len(txs) - added,
	})
}

type MetadataUpdateRequest struct {
	Classifications  map[string]AssetClass `json:"classifications"`
	AssetNames       map[string]string     `json:"asset_names"`
	ManualPrices     map[string]float64    `json:"manual_prices"`
	ManualCurrencies map[string]string     `json:"manual_currencies"`
	ExchangeRates    map[string]float64    `json:"exchange_rates"`
	AccountNames     map[string]string     `json:"account_names"`
	AutoFetchRates   bool                  `json:"auto_fetch_rates"`
}

// SaveMetadata handles POST /api/metadata
func (s *Server) SaveMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MetadataUpdateRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	err = s.storage.SaveAssetMetadata(
		req.Classifications,
		req.AssetNames,
		req.ManualPrices,
		req.ManualCurrencies,
		req.ExchangeRates,
		req.AccountNames,
		req.AutoFetchRates,
	)
	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "Failed to save metadata: "+err.Error())
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Metadata updated successfully",
	})
}

// GetTransactions handles GET /api/transactions
func (s *Server) GetTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	db := s.storage.GetDB()
	txs := db.Transactions

	// Sort newest first
	sort.Slice(txs, func(i, j int) bool {
		if txs[i].BookingDate != txs[j].BookingDate {
			return txs[i].BookingDate > txs[j].BookingDate
		}
		return txs[i].ID > txs[j].ID
	})

	s.respondWithJSON(w, http.StatusOK, txs)
}

// ResetDatabase handles POST /api/reset
func (s *Server) ResetDatabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Just overwrite file with default empty db
	s.storage.mu.Lock()
	s.storage.db = AppDatabase{
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
	}
	err := s.storage.save()
	s.storage.mu.Unlock()

	if err != nil {
		s.respondWithError(w, http.StatusInternalServerError, "Failed to reset database: "+err.Error())
		return
	}

	s.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Database reset successfully",
	})
}

func (s *Server) respondWithError(w http.ResponseWriter, status int, message string) {
	s.respondWithJSON(w, status, map[string]string{"error": message})
}

func (s *Server) respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Internal Server Error"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(response)
}

type DbSummary struct {
	TransactionCount int                `json:"transaction_count"`
	AssetCount       int                `json:"asset_count"`
	ExchangeRates    map[string]float64 `json:"exchange_rates"`
	AutoFetchRates   bool               `json:"auto_fetch_rates"`
	DisableFXAPI     bool               `json:"disable_fx_api"`
}

func (s *Server) getDbSummary(db AppDatabase) DbSummary {
	return DbSummary{
		TransactionCount: len(db.Transactions),
		AssetCount:       len(db.AssetSymbols),
		ExchangeRates:    db.ExchangeRates,
		AutoFetchRates:   db.AutoFetchRates,
		DisableFXAPI:     s.disableFXAPI,
	}
}

type LiveRatesResponse struct {
	Success bool               `json:"success"`
	Rates   map[string]float64 `json:"rates"`
}

// GetLiveRates handles GET /api/live-rates
func (s *Server) GetLiveRates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.disableFXAPI {
		s.respondWithError(w, http.StatusForbidden, "Live FX rates fetching is disabled on this server")
		return
	}

	rates, err := fetchLiveFXRates()
	if err != nil {
		s.respondWithError(w, http.StatusBadGateway, "Failed to fetch live rates: "+err.Error())
		return
	}

	s.respondWithJSON(w, http.StatusOK, LiveRatesResponse{
		Success: true,
		Rates:   rates,
	})
}

func fetchLiveFXRates() (map[string]float64, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Try Frankfurter API first with correct base=DKK parameter
	resp, err := client.Get("https://api.frankfurter.app/latest?base=DKK")
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var result struct {
			Rates map[string]float64 `json:"rates"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			converted := make(map[string]float64)
			for curr, rate := range result.Rates {
				if rate > 0 {
					converted[curr] = 1.0 / rate
				}
			}
			converted["DKK"] = 1.0
			return converted, nil
		}
	}

	// Fallback to keyless open ExchangeRate-API (highly reliable backup)
	resp, err = client.Get("https://open.er-api.com/v6/latest/DKK")
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var result struct {
			Rates map[string]float64 `json:"rates"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			converted := make(map[string]float64)
			for curr, rate := range result.Rates {
				if rate > 0 {
					converted[curr] = 1.0 / rate
				}
			}
			converted["DKK"] = 1.0
			return converted, nil
		}
	}

	return nil, errors.New("both primary and fallback FX APIs are unreachable")
}
