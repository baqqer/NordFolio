package internal

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

const maxUploadBytes = 20 << 20

type Server struct {
	storage *Storage
}

func NewServer(storage *Storage) *Server {
	return &Server{storage: storage}
}

// Register adds the API routes to mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/portfolio", s.GetPortfolio)
	mux.HandleFunc("GET /api/transactions", s.GetTransactions)
	mux.HandleFunc("POST /api/upload", s.UploadCSV)
	mux.HandleFunc("POST /api/metadata", s.SaveMetadata)
	mux.HandleFunc("POST /api/reset", s.ResetDatabase)
}

type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Asset struct {
	ISIN   string     `json:"isin"`
	Symbol string     `json:"symbol"`
	Name   string     `json:"name"`
	Class  AssetClass `json:"class"`
}

type PortfolioResponse struct {
	Allocation       Allocation `json:"allocation"`
	Accounts         []Account  `json:"accounts"`
	Assets           []Asset    `json:"assets"`
	TransactionCount int        `json:"transaction_count"`
}

// GetPortfolio handles GET /api/portfolio?accounts=a,b
func (s *Server) GetPortfolio(w http.ResponseWriter, r *http.Request) {
	db := s.storage.GetDB()

	assets := make([]Asset, 0, len(db.AssetSymbols))
	for isin, symbol := range db.AssetSymbols {
		assets = append(assets, Asset{ISIN: isin, Symbol: symbol, Name: db.AssetNames[isin], Class: db.Classifications[isin]})
	}
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].Symbol != assets[j].Symbol {
			return assets[i].Symbol < assets[j].Symbol
		}
		return assets[i].ISIN < assets[j].ISIN
	})

	respondWithJSON(w, http.StatusOK, PortfolioResponse{
		Allocation:       BuildAllocation(db, parseAccounts(r)),
		Accounts:         listAccounts(db),
		Assets:           assets,
		TransactionCount: len(db.Transactions),
	})
}

// GetTransactions handles GET /api/transactions?accounts=a,b (newest first)
func (s *Server) GetTransactions(w http.ResponseWriter, r *http.Request) {
	accounts := parseAccounts(r)
	db := s.storage.GetDB()

	txs := make([]Transaction, 0, len(db.Transactions))
	for _, tx := range db.Transactions {
		if len(accounts) == 0 || accounts[tx.Account] {
			txs = append(txs, tx)
		}
	}
	SortTransactions(txs)
	for i, j := 0, len(txs)-1; i < j; i, j = i+1, j-1 {
		txs[i], txs[j] = txs[j], txs[i]
	}

	respondWithJSON(w, http.StatusOK, txs)
}

// UploadCSV handles POST /api/upload (multipart field "file")
func (s *Server) UploadCSV(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	file, _, err := r.FormFile("file")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Kunne ikke læse filen: "+err.Error())
		return
	}
	defer file.Close()

	txs, err := ParseNordnetCSV(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Kunne ikke læse CSV-filen: "+err.Error())
		return
	}
	if len(txs) == 0 {
		respondWithError(w, http.StatusBadRequest, "Filen indeholder ingen gyldige transaktioner")
		return
	}

	known := s.storage.GetDB().AccountNames
	added, err := s.storage.AddTransactions(txs)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Kunne ikke gemme transaktionerne: "+err.Error())
		return
	}

	newAccounts := []string{}
	seen := map[string]bool{}
	for _, tx := range txs {
		if _, ok := known[tx.Account]; !ok && tx.Account != "" && !seen[tx.Account] {
			seen[tx.Account] = true
			newAccounts = append(newAccounts, tx.Account)
		}
	}
	sort.Strings(newAccounts)

	respondWithJSON(w, http.StatusOK, map[string]any{
		"parsed":       len(txs),
		"added":        added,
		"skipped":      len(txs) - added,
		"new_accounts": newAccounts,
	})
}

// SaveMetadata handles POST /api/metadata
func (s *Server) SaveMetadata(w http.ResponseWriter, r *http.Request) {
	var u MetadataUpdate
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		respondWithError(w, http.StatusBadRequest, "Ugyldig JSON: "+err.Error())
		return
	}
	if err := s.storage.SaveMetadata(u); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Kunne ikke gemme ændringerne: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ResetDatabase handles POST /api/reset
func (s *Server) ResetDatabase(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.Reset(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Kunne ikke nulstille databasen: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func parseAccounts(r *http.Request) map[string]bool {
	accounts := map[string]bool{}
	for _, acc := range strings.Split(r.URL.Query().Get("accounts"), ",") {
		if acc = strings.TrimSpace(acc); acc != "" {
			accounts[acc] = true
		}
	}
	return accounts
}

func listAccounts(db Database) []Account {
	ids := map[string]bool{}
	for id := range db.AccountNames {
		ids[id] = true
	}
	for _, tx := range db.Transactions {
		if tx.Account != "" {
			ids[tx.Account] = true
		}
	}

	accounts := make([]Account, 0, len(ids))
	for id := range ids {
		name := db.AccountNames[id]
		if name == "" {
			name = id
		}
		accounts = append(accounts, Account{ID: id, Name: name})
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	return accounts
}

func respondWithError(w http.ResponseWriter, status int, message string) {
	respondWithJSON(w, status, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, status int, payload any) {
	response, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"error":"Internal Server Error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(response)
}
