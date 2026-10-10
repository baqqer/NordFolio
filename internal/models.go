package internal

type AssetClass string

const (
	AssetClassSecurity AssetClass = "Security"
	AssetClassETF      AssetClass = "ETF"
	AssetClassCash     AssetClass = "Cash"
)

// Transaction is one row of a Nordnet "transactions-and-notes-export.csv" file.
type Transaction struct {
	ID                  string  `json:"id"`
	BookingDate         string  `json:"booking_date"` // YYYY-MM-DD
	TradeDate           string  `json:"trade_date"`   // YYYY-MM-DD
	ValueDate           string  `json:"value_date"`   // YYYY-MM-DD
	Account             string  `json:"account"`
	TransactionType     string  `json:"transaction_type"` // e.g. KØBT, SOLGT, INDBETALING, UDBYTTE
	Symbol              string  `json:"symbol"`
	ISIN                string  `json:"isin"`
	Quantity            float64 `json:"quantity"`
	Price               float64 `json:"price"`
	Interest            float64 `json:"interest"`
	Fees                float64 `json:"fees"`
	FeeCurrency         string  `json:"fee_currency"`
	Amount              float64 `json:"amount"` // Net cash impact in the account currency
	AmountCurrency      string  `json:"amount_currency"`
	AcquisitionValue    float64 `json:"acquisition_value"`
	AcquisitionCurrency string  `json:"acquisition_currency"`
	Result              float64 `json:"result"`
	ResultCurrency      string  `json:"result_currency"`
	TotalQuantity       float64 `json:"total_quantity"` // Holding in the account after this row
	Balance             float64 `json:"balance"`        // Cash balance in the account after this row
	ExchangeRate        float64 `json:"exchange_rate"`
	TransactionText     string  `json:"transaction_text"`
	CancellationDate    string  `json:"cancellation_date"`
	NoteNumber          string  `json:"note_number"`
	VerificationNumber  string  `json:"verification_number"`
	Brokerage           float64 `json:"brokerage"`
	BrokerageCurrency   string  `json:"brokerage_currency"`
	AvgPrice            float64 `json:"avg_price"`
	OriginalInterest    float64 `json:"original_interest"`
}

// Database is the content of db.json. Field names are kept stable so existing
// database files keep loading; keys from older versions are ignored.
type Database struct {
	Transactions    []Transaction         `json:"transactions"`
	Classifications map[string]AssetClass `json:"classifications"` // ISIN -> Security | ETF
	AssetNames      map[string]string     `json:"asset_names"`     // ISIN -> friendly name
	AssetSymbols    map[string]string     `json:"asset_symbols"`   // ISIN -> ticker
	AccountNames    map[string]string     `json:"account_names"`   // account ID -> friendly name
}

func newDatabase() Database {
	db := Database{Transactions: []Transaction{}}
	db.ensureMaps()
	return db
}

func (db *Database) ensureMaps() {
	if db.Transactions == nil {
		db.Transactions = []Transaction{}
	}
	if db.Classifications == nil {
		db.Classifications = map[string]AssetClass{}
	}
	if db.AssetNames == nil {
		db.AssetNames = map[string]string{}
	}
	if db.AssetSymbols == nil {
		db.AssetSymbols = map[string]string{}
	}
	if db.AccountNames == nil {
		db.AccountNames = map[string]string{}
	}
}

// clone returns a deep copy so callers can read it without holding the storage lock.
func (db Database) clone() Database {
	c := Database{
		Transactions:    append([]Transaction(nil), db.Transactions...),
		Classifications: make(map[string]AssetClass, len(db.Classifications)),
		AssetNames:      make(map[string]string, len(db.AssetNames)),
		AssetSymbols:    make(map[string]string, len(db.AssetSymbols)),
		AccountNames:    make(map[string]string, len(db.AccountNames)),
	}
	for k, v := range db.Classifications {
		c.Classifications[k] = v
	}
	for k, v := range db.AssetNames {
		c.AssetNames[k] = v
	}
	for k, v := range db.AssetSymbols {
		c.AssetSymbols[k] = v
	}
	for k, v := range db.AccountNames {
		c.AccountNames[k] = v
	}
	c.ensureMaps()
	return c
}
