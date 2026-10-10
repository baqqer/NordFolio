package internal

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	s, err := NewStorage(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewServer(s).Register(mux)
	return mux
}

func upload(t *testing.T, h http.Handler, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", filepath.Base(path))
	fw.Write(data)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: status %d: %s", rec.Code, rec.Body)
	}
	var res map[string]any
	json.Unmarshal(rec.Body.Bytes(), &res)
	return res
}

func TestAPIUploadAndPortfolio(t *testing.T) {
	h := newTestServer(t)

	res := upload(t, h, "transactions-and-notes-export.csv")
	if res["added"].(float64) != 15 || len(res["new_accounts"].([]any)) != 2 {
		t.Errorf("first upload: %v", res)
	}
	res = upload(t, h, "transactions-and-notes-export.csv")
	if res["added"].(float64) != 0 || res["skipped"].(float64) != 15 || len(res["new_accounts"].([]any)) != 0 {
		t.Errorf("repeated upload: %v", res)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio?accounts=12345678", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("portfolio: status %d", rec.Code)
	}
	var p PortfolioResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Accounts) != 2 || len(p.Assets) != 3 || p.TransactionCount != 15 {
		t.Errorf("unexpected portfolio response: %+v", p)
	}
	approx(t, "filtered cash", p.Allocation.CashDKK, 6030.25)

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/transactions?accounts=87654321", nil))
	var txs []Transaction
	json.Unmarshal(rec.Body.Bytes(), &txs)
	if len(txs) != 8 || txs[0].BookingDate != "2026-08-04" {
		t.Errorf("transactions: got %d, first %+v", len(txs), txs[0])
	}
}

func TestAPIRejectsWrongMethod(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/reset", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/reset: status %d, want 405", rec.Code)
	}
}
