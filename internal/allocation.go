package internal

import (
	"sort"
	"strings"
)

// Holding is one security aggregated across the selected accounts, valued at
// its AVCO cost basis (book value). Market prices are deliberately not used.
type Holding struct {
	ISIN         string     `json:"isin"`
	Symbol       string     `json:"symbol"`
	Name         string     `json:"name"`
	Class        AssetClass `json:"class"`
	Currency     string     `json:"currency"` // Trading currency of the security
	Quantity     float64    `json:"quantity"`
	AvgCost      float64    `json:"avg_cost"` // Average cost per unit in the trading currency
	BookValueDKK float64    `json:"book_value_dkk"`
	Weight       float64    `json:"weight"` // Percentage of the total book value
	Accounts     []string   `json:"accounts"`
}

// Slice is one part of a part-to-whole breakdown.
type Slice struct {
	Key      string  `json:"key"`
	ValueDKK float64 `json:"value_dkk"`
	Weight   float64 `json:"weight"`
}

type Allocation struct {
	AsOf          string    `json:"as_of"` // Latest booking date included
	TotalDKK      float64   `json:"total_dkk"`
	SecuritiesDKK float64   `json:"securities_dkk"`
	ETFsDKK       float64   `json:"etfs_dkk"`
	CashDKK       float64   `json:"cash_dkk"`
	Holdings      []Holding `json:"holdings"`
	ByClass       []Slice   `json:"by_class"`
	ByAccount     []Slice   `json:"by_account"`
	ByCurrency    []Slice   `json:"by_currency"`
}

func IsStockInflow(txType string) bool {
	tt := strings.ToUpper(strings.TrimSpace(txType))
	return tt == "KØBT" ||
		tt == "INDSÆTTELSE" ||
		strings.HasPrefix(tt, "BYTTE INDLÆG") ||
		strings.HasPrefix(tt, "INDLÆG") ||
		strings.HasPrefix(tt, "FUSION INDLÆG") ||
		strings.HasPrefix(tt, "SPLIT INDLÆG")
}

func IsStockOutflow(txType string) bool {
	tt := strings.ToUpper(strings.TrimSpace(txType))
	return tt == "SOLGT" ||
		tt == "HÆVNING" ||
		strings.HasPrefix(tt, "BYTTE OVERF") ||
		strings.HasPrefix(tt, "INDLØSNING OVERF") ||
		strings.HasPrefix(tt, "FUSION OVERF") ||
		strings.HasPrefix(tt, "SPLIT OVERF")
}

// position is a security holding within a single account.
type position struct {
	account    string
	isin       string
	quantity   float64
	bookDKK    float64 // AVCO cost basis in DKK
	bookNative float64 // AVCO cost basis in the trading currency
}

const dustQuantity = 0.00001

// SortTransactions orders transactions chronologically. Nordnet IDs increase
// over time, so they break ties within a booking date.
func SortTransactions(txs []Transaction) {
	sort.SliceStable(txs, func(i, j int) bool {
		a, b := txs[i], txs[j]
		if a.BookingDate != b.BookingDate {
			return a.BookingDate < b.BookingDate
		}
		if len(a.ID) != len(b.ID) {
			return len(a.ID) < len(b.ID)
		}
		return a.ID < b.ID
	})
}

// BuildAllocation replays the transactions of the selected accounts (all
// accounts if none are selected) and returns the current holdings and cash.
//
// Exports that don't start at the first trade are handled by reconstructing
// opening balances from the first row of each account and security: cash from
// "Saldo" minus "Beløb", and quantity from "Totalt antal" before the row. The
// cost basis of such opening holdings is estimated at that row's price.
func BuildAllocation(db Database, accounts map[string]bool) Allocation {
	txs := make([]Transaction, 0, len(db.Transactions))
	for _, tx := range db.Transactions {
		if len(accounts) == 0 || accounts[tx.Account] {
			txs = append(txs, tx)
		}
	}
	SortTransactions(txs)

	cash := map[string]float64{}
	positions := map[string]*position{}
	currencies := map[string]string{}
	var accountOrder []string
	var positionOrder []string
	asOf := ""

	for _, tx := range txs {
		asOf = tx.BookingDate

		// Cash: "Saldo" is the account balance after the row. Rows without a
		// balance fall back to applying the amount.
		if _, seen := cash[tx.Account]; !seen {
			accountOrder = append(accountOrder, tx.Account)
			cash[tx.Account] = tx.Balance - tx.Amount
		}
		if tx.Balance != 0 {
			cash[tx.Account] = tx.Balance
		} else {
			cash[tx.Account] += tx.Amount
		}

		if tx.ISIN == "" {
			continue
		}

		if c := securityCurrency(tx); c != "" {
			currencies[tx.ISIN] = c
		}

		rate := tx.ExchangeRate
		if rate == 0 {
			rate = 1
		}
		inflow := IsStockInflow(tx.TransactionType)
		outflow := !inflow && IsStockOutflow(tx.TransactionType)

		key := tx.Account + "|" + tx.ISIN
		pos, seen := positions[key]
		if !seen {
			pos = &position{account: tx.Account, isin: tx.ISIN}
			positions[key] = pos
			positionOrder = append(positionOrder, key)

			// Reconstruct a holding that predates the export
			opening := 0.0
			if inflow {
				opening = tx.TotalQuantity - tx.Quantity
			} else if outflow {
				opening = tx.TotalQuantity + tx.Quantity
			}
			if opening > dustQuantity {
				pos.quantity = opening
				pos.bookNative = opening * tx.Price
				pos.bookDKK = pos.bookNative * rate
			}
		}

		if tx.Quantity <= 0 {
			continue
		}

		switch {
		case inflow:
			costNative, costDKK := tx.Quantity*tx.Price, tx.Quantity*tx.Price*rate
			if tx.AcquisitionValue != 0 {
				costNative = tx.AcquisitionValue
				costDKK = tx.AcquisitionValue
				if tx.AcquisitionCurrency != "" && tx.AcquisitionCurrency != "DKK" {
					costDKK *= rate
				}
			}
			pos.quantity += tx.Quantity
			pos.bookNative += costNative
			pos.bookDKK += costDKK

		case outflow:
			remaining := pos.quantity - tx.Quantity
			if remaining < dustQuantity {
				*pos = position{account: pos.account, isin: pos.isin}
				continue
			}
			// AVCO: the remaining units keep their average cost
			share := remaining / pos.quantity
			pos.quantity = remaining
			pos.bookNative *= share
			pos.bookDKK *= share
		}
	}

	a := Allocation{AsOf: asOf, Holdings: []Holding{}}

	byAccount := map[string]float64{}
	byCurrency := map[string]float64{}
	holdings := map[string]*Holding{}
	var holdingOrder []string

	for _, key := range positionOrder {
		pos := positions[key]
		if pos.quantity <= dustQuantity {
			continue
		}
		h, ok := holdings[pos.isin]
		if !ok {
			h = &Holding{
				ISIN:     pos.isin,
				Symbol:   db.AssetSymbols[pos.isin],
				Name:     db.AssetNames[pos.isin],
				Class:    db.Classifications[pos.isin],
				Currency: currencies[pos.isin],
				Accounts: []string{},
			}
			if h.Name == "" {
				h.Name = h.Symbol
			}
			if h.Class == "" {
				h.Class = AssetClassSecurity
			}
			if h.Currency == "" {
				h.Currency = "DKK"
			}
			holdings[pos.isin] = h
			holdingOrder = append(holdingOrder, pos.isin)
		}
		h.Quantity += pos.quantity
		h.BookValueDKK += pos.bookDKK
		h.AvgCost += pos.bookNative // Divided by quantity below
		h.Accounts = append(h.Accounts, pos.account)

		byAccount[pos.account] += pos.bookDKK
		byCurrency[h.Currency] += pos.bookDKK
		if h.Class == AssetClassETF {
			a.ETFsDKK += pos.bookDKK
		} else {
			a.SecuritiesDKK += pos.bookDKK
		}
	}

	for _, acc := range accountOrder {
		a.CashDKK += cash[acc]
		byAccount[acc] += cash[acc]
	}
	if a.CashDKK != 0 {
		byCurrency["DKK"] += a.CashDKK
	}
	a.TotalDKK = a.SecuritiesDKK + a.ETFsDKK + a.CashDKK

	for _, isin := range holdingOrder {
		h := holdings[isin]
		h.AvgCost /= h.Quantity
		h.Weight = weight(h.BookValueDKK, a.TotalDKK)
		a.Holdings = append(a.Holdings, *h)
	}
	sort.SliceStable(a.Holdings, func(i, j int) bool {
		return a.Holdings[i].BookValueDKK > a.Holdings[j].BookValueDKK
	})

	// Classes keep a fixed order so their colors never move between views
	a.ByClass = []Slice{
		{Key: string(AssetClassSecurity), ValueDKK: a.SecuritiesDKK, Weight: weight(a.SecuritiesDKK, a.TotalDKK)},
		{Key: string(AssetClassETF), ValueDKK: a.ETFsDKK, Weight: weight(a.ETFsDKK, a.TotalDKK)},
		{Key: string(AssetClassCash), ValueDKK: a.CashDKK, Weight: weight(a.CashDKK, a.TotalDKK)},
	}
	a.ByAccount = toSlices(byAccount, a.TotalDKK)
	a.ByCurrency = toSlices(byCurrency, a.TotalDKK)

	return a
}

// securityCurrency returns the trading currency a row reveals, if any.
func securityCurrency(tx Transaction) string {
	if tx.AcquisitionCurrency != "" {
		return tx.AcquisitionCurrency
	}
	return tx.ResultCurrency
}

func weight(value, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return value / total * 100
}

func toSlices(values map[string]float64, total float64) []Slice {
	slices := make([]Slice, 0, len(values))
	for key, value := range values {
		slices = append(slices, Slice{Key: key, ValueDKK: value, Weight: weight(value, total)})
	}
	sort.Slice(slices, func(i, j int) bool {
		if slices[i].ValueDKK != slices[j].ValueDKK {
			return slices[i].ValueDKK > slices[j].ValueDKK
		}
		return slices[i].Key < slices[j].Key
	})
	return slices
}
