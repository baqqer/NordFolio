package internal

type AssetClass string

const (
	AssetClassSecurity AssetClass = "Security"
	AssetClassETF      AssetClass = "ETF"
	AssetClassCash     AssetClass = "Cash"
)

type Transaction struct {
	ID                  string  `json:"id"`
	BookingDate         string  `json:"booking_date"` // YYYY-MM-DD
	TradeDate           string  `json:"trade_date"`   // YYYY-MM-DD
	ValueDate           string  `json:"value_date"`   // YYYY-MM-DD
	Account             string  `json:"account"`
	TransactionType     string  `json:"transaction_type"` // e.g., KØBT, SOLGT, INDSÆTTELSE, HÆVNING, INDBATALING
	Symbol              string  `json:"symbol"`
	ISIN                string  `json:"isin"`
	Quantity            float64 `json:"quantity"`
	Price               float64 `json:"price"`
	Interest            float64 `json:"interest"`
	Fees                float64 `json:"fees"`
	FeeCurrency         string  `json:"fee_currency"`
	Amount              float64 `json:"amount"` // Net cash impact in account base currency
	AmountCurrency      string  `json:"amount_currency"`
	AcquisitionValue    float64 `json:"acquisition_value"`
	AcquisitionCurrency string  `json:"acquisition_currency"`
	Result              float64 `json:"result"`
	ResultCurrency      string  `json:"result_currency"`
	TotalQuantity       float64 `json:"total_quantity"`
	Balance             float64 `json:"balance"`
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

type AppDatabase struct {
	Transactions     []Transaction          `json:"transactions"`
	Classifications  map[string]AssetClass  `json:"classifications"`   // ISIN -> "Security" | "ETF"
	AssetNames       map[string]string      `json:"asset_names"`       // ISIN -> Friendly Name
	AssetSymbols     map[string]string      `json:"asset_symbols"`     // ISIN -> Symbol (ticker)
	ManualPrices     map[string]float64     `json:"manual_prices"`     // ISIN -> Price in native currency
	ManualCurrencies map[string]string      `json:"manual_currencies"` // ISIN -> Currency (e.g. DKK, EUR, USD)
	ExchangeRates    map[string]float64     `json:"exchange_rates"`    // Currency -> Rate to DKK (e.g. USD: 6.90, EUR: 7.46)
	AccountNames     map[string]string      `json:"account_names"`     // Account ID -> Friendly Name (Defaults to ID)
	AutoFetchRates   bool                   `json:"auto_fetch_rates"`  // Automatically query and save live FX rates on load
}
