package internal

import (
	"math"
	"sort"
	"strings"
	"time"
)

type AssetDetail struct {
	ISIN         string     `json:"isin"`
	Symbol       string     `json:"symbol"`
	Name         string     `json:"name"`
	Type         AssetClass `json:"type"` // Security, ETF, Cash
	Quantity     float64    `json:"quantity"`
	LastTxPrice  float64    `json:"last_tx_price"`
	CurrentPrice float64    `json:"current_price"`
	Currency     string     `json:"currency"`
	ValueDKK     float64    `json:"value_dkk"`
	BookValueDKK float64    `json:"book_value_dkk"`
	GainLossDKK  float64    `json:"gain_loss_dkk"`
	GainLossPct  float64    `json:"gain_loss_pct"`
	Percentage   float64    `json:"percentage"`
}

type PortfolioAnalysis struct {
	TotalValueDKK      float64       `json:"total_value_dkk"`
	TotalSecuritiesDKK float64       `json:"total_securities_dkk"`
	TotalETFsDKK       float64       `json:"total_etfs_dkk"`
	TotalCashDKK       float64       `json:"total_cash_dkk"`
	TotalBookValueDKK  float64       `json:"total_book_value_dkk"`
	TotalGainLossDKK   float64       `json:"total_gain_loss_dkk"`
	TotalGainLossPct   float64       `json:"total_gain_loss_pct"`
	Assets             []AssetDetail `json:"assets"`

	Dates                 []string  `json:"dates"`
	CashSeries            []float64 `json:"cash_series"`
	SecuritiesSeries      []float64 `json:"securities_series"`
	ETFsSeries            []float64 `json:"etfs_series"`
	TotalSeries           []float64 `json:"total_series"`
	InvestedCapitalSeries []float64 `json:"invested_capital_series"`
	FeesSeries            []float64 `json:"fees_series"`
	DividendsSeries       []float64 `json:"dividends_series"`
	TaxesSeries           []float64 `json:"taxes_series"`
	SimpleReturnSeries    []float64 `json:"simple_return_series"`
	TWRRSeries            []float64 `json:"twrr_series"`
	DrawdownSeries        []float64 `json:"drawdown_series"`
}

// IsInternalTransfer determines if a transaction is an internal transfer between visible accounts
func IsInternalTransfer(tx Transaction, visibleAccounts map[string]bool) bool {
	t := strings.ToLower(tx.TransactionText)
	isTransfer := strings.Contains(t, "internal to") || strings.Contains(t, "internal from") || strings.Contains(t, "intern overførsel") ||
		(tx.TransactionType == "INDSÆTTELSE" && strings.Contains(t, "from")) ||
		(tx.TransactionType == "HÆVNING" && strings.Contains(t, "to"))

	if !isTransfer {
		return false
	}

	// Check if the transfer text explicitly mentions an account ID in our database
	hasMentionedAccount := false
	for acc := range visibleAccounts {
		if strings.Contains(t, acc) {
			hasMentionedAccount = true
			if acc != tx.Account {
				return true // Found the visible counterparty account!
			}
		}
	}

	// If the transfer text doesn't mention any explicit visible account ID,
	// we treat it as internal by default if there are multiple visible accounts.
	if !hasMentionedAccount {
		return len(visibleAccounts) > 1
	}

	return false
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

// IsExternalCashFlow determines if a transaction is an external deposit or withdrawal
func IsExternalCashFlow(tx Transaction, visibleAccounts map[string]bool) bool {
	if IsInternalTransfer(tx, visibleAccounts) {
		return false
	}
	tt := strings.ToUpper(tx.TransactionType)
	if tt == "INDBETALING" {
		return true
	}
	if tt == "UDBETALING" {
		return true
	}
	// If it's a deposit (INDSÆTTELSE) or withdrawal (HÆVNING) and not internal, it's external cash flow
	if tt == "INDSÆTTELSE" || tt == "HÆVNING" {
		return true
	}
	return false
}

func AnalyzePortfolio(db AppDatabase, visibleAccounts map[string]bool) (PortfolioAnalysis, error) {
	txs := make([]Transaction, len(db.Transactions))
	copy(txs, db.Transactions)

	// Sort transactions chronologically
	sort.Slice(txs, func(i, j int) bool {
		if txs[i].BookingDate != txs[j].BookingDate {
			return txs[i].BookingDate < txs[j].BookingDate
		}
		return txs[i].ID < txs[j].ID
	})

	if len(txs) == 0 {
		return PortfolioAnalysis{
			Assets:                make([]AssetDetail, 0),
			Dates:                 make([]string, 0),
			CashSeries:            make([]float64, 0),
			SecuritiesSeries:      make([]float64, 0),
			ETFsSeries:            make([]float64, 0),
			TotalSeries:           make([]float64, 0),
			InvestedCapitalSeries: make([]float64, 0),
			FeesSeries:            make([]float64, 0),
			DividendsSeries:       make([]float64, 0),
			TaxesSeries:           make([]float64, 0),
			SimpleReturnSeries:    make([]float64, 0),
			TWRRSeries:            make([]float64, 0),
			DrawdownSeries:        make([]float64, 0),
		}, nil
	}

	// If no visible accounts specified, default to including all unique accounts found in the database
	if len(visibleAccounts) == 0 {
		visibleAccounts = make(map[string]bool)
		for _, tx := range txs {
			if tx.Account != "" {
				visibleAccounts[tx.Account] = true
			}
		}
	}

	// Filter transactions to only contain rows belonging to visible accounts
	filteredTxs := make([]Transaction, 0)
	for _, tx := range txs {
		if visibleAccounts[tx.Account] {
			filteredTxs = append(filteredTxs, tx)
		}
	}
	txs = filteredTxs

	if len(txs) == 0 {
		return PortfolioAnalysis{
			Assets:                make([]AssetDetail, 0),
			Dates:                 make([]string, 0),
			CashSeries:            make([]float64, 0),
			SecuritiesSeries:      make([]float64, 0),
			ETFsSeries:            make([]float64, 0),
			TotalSeries:           make([]float64, 0),
			InvestedCapitalSeries: make([]float64, 0),
			FeesSeries:            make([]float64, 0),
			DividendsSeries:       make([]float64, 0),
			TaxesSeries:           make([]float64, 0),
			SimpleReturnSeries:    make([]float64, 0),
			TWRRSeries:            make([]float64, 0),
			DrawdownSeries:        make([]float64, 0),
		}, nil
	}

	// 1. Calculate pre-existing (starting) balances before the first transaction of each account / security.
	// This is critical for partial exports so starting cash and holdings are accounted for in Invested Capital.
	preExistingCash := 0.0
	preExistingHoldings := make(map[string]float64)
	preExistingPrices := make(map[string]float64)
	preExistingRates := make(map[string]float64)

	seenAccounts := make(map[string]bool)
	seenISINs := make(map[string]bool)

	for _, tx := range txs {
		// Cash starting balance
		if !seenAccounts[tx.Account] {
			seenAccounts[tx.Account] = true
			preExistingCash += tx.Balance - tx.Amount
		}

		// Security starting holdings
		if tx.ISIN != "" && !seenISINs[tx.ISIN] {
			seenISINs[tx.ISIN] = true
			qty := tx.Quantity
			tt := strings.ToUpper(tx.TransactionType)
			preQty := 0.0

			if IsStockInflow(tt) {
				preQty = tx.TotalQuantity - qty
			} else if IsStockOutflow(tt) {
				preQty = tx.TotalQuantity + qty
			}

			if preQty > 0 {
				preExistingHoldings[tx.ISIN] = preQty
				preExistingPrices[tx.ISIN] = tx.Price
				rate := tx.ExchangeRate
				if rate == 0 {
					rate = 1.0
				}
				preExistingRates[tx.ISIN] = rate
			}
		}
	}

	// 2. Initialize tracking states with the pre-existing starting balances
	depotBalances := make(map[string]float64)
	seenAccsInit := make(map[string]bool)
	for _, tx := range txs {
		if !seenAccsInit[tx.Account] {
			seenAccsInit[tx.Account] = true
			depotBalances[tx.Account] = tx.Balance - tx.Amount
		}
	}

	securityHoldings := make(map[string]float64)
	for isin, qty := range preExistingHoldings {
		securityHoldings[isin] = qty
	}

	lastPrices := make(map[string]float64)
	for isin, price := range preExistingPrices {
		lastPrices[isin] = price
	}

	lastRates := make(map[string]float64)
	for isin, rate := range preExistingRates {
		lastRates[isin] = rate
	}

	bookValues := make(map[string]float64)
	for isin, qty := range preExistingHoldings {
		price := preExistingPrices[isin]
		rate := preExistingRates[isin]
		bookValues[isin] = qty * price * rate
	}

	// Initial Invested Capital is the sum of starting cash + starting security values
	investedCapital := preExistingCash
	for isin, qty := range preExistingHoldings {
		price := preExistingPrices[isin]
		rate := preExistingRates[isin]
		investedCapital += qty * price * rate
	}

	// Group transactions by date
	txGroups := make(map[string][]Transaction)
	var uniqueDates []string
	for _, tx := range txs {
		date := tx.BookingDate
		if _, ok := txGroups[date]; !ok {
			uniqueDates = append(uniqueDates, date)
		}
		txGroups[date] = append(txGroups[date], tx)
	}
	sort.Strings(uniqueDates)

	// Timeline boundaries (strictly capped at newest transaction date)
	startDateStr := uniqueDates[0]
	endDateStr := uniqueDates[len(uniqueDates)-1]

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return PortfolioAnalysis{}, err
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return PortfolioAnalysis{}, err
	}

	var dates []string
	var cashSeries []float64
	var securitiesSeries []float64
	var etfsSeries []float64
	var totalSeries []float64
	var investedCapitalSeries []float64
	var feesSeries []float64
	var dividendsSeries []float64
	var taxesSeries []float64
	var simpleReturnSeries []float64
	var twrrSeries []float64
	var drawdownSeries []float64

	cumFees := 0.0
	cumDividends := 0.0
	cumTaxes := 0.0

	// Chronological TWRR state initialization
	cumTWRR := 1.0

	// Calculate exact starting total portfolio value
	startTotalVal := preExistingCash
	for isin, qty := range preExistingHoldings {
		price := preExistingPrices[isin]
		rate := preExistingRates[isin]
		startTotalVal += qty * price * rate
	}
	prevTotal := startTotalVal
	cumMaxTotal := startTotalVal

	// Daily loop to reconstruct continuous historical portfolio series
	currDate := startDate
	for !currDate.After(endDate) {
		dateStr := currDate.Format("2006-01-02")
		dates = append(dates, dateStr)

		// Apply transactions of this date, if any
		if dayTxs, exists := txGroups[dateStr]; exists {
			for _, tx := range dayTxs {
				// 1. Update depot cash balance
				if tx.Balance != 0 {
					depotBalances[tx.Account] = tx.Balance
				} else {
					depotBalances[tx.Account] += tx.Amount
				}

				// 2. Update cumulative external deposits/withdrawals
				if IsExternalCashFlow(tx, visibleAccounts) {
					investedCapital += tx.Amount
				}

				// Accumulate cumulative costs and dividend income chronologically
				txType := strings.ToUpper(tx.TransactionType)
				
				// 1) Accumulate transaction-level fees / brokerage
				cumFees += tx.Fees
				if txType == "GEBYR" || txType == "KURTAGE" {
					cumFees += math.Abs(tx.Amount)
				}

				// 2) Accumulate dividend cash income
				if strings.Contains(txType, "UDBYTTE") || strings.Contains(txType, "DIVIDEND") {
					cumDividends += tx.Amount
				}

				// 3) Accumulate withholding/asset taxes
				if strings.Contains(txType, "SKAT") || strings.Contains(txType, "TAX") {
					cumTaxes += math.Abs(tx.Amount)
				}

				// 3. Update security quantity, cost basis, and historical prices/exchange rates
				if tx.ISIN != "" {
					qty := tx.Quantity
					tt := strings.ToUpper(tx.TransactionType)

					if tx.ExchangeRate > 0 {
						lastRates[tx.ISIN] = tx.ExchangeRate
					} else if lastRates[tx.ISIN] == 0 {
						lastRates[tx.ISIN] = 1.0
					}

					if qty > 0 {
						currQty := securityHoldings[tx.ISIN]
						currBook := bookValues[tx.ISIN]

						if IsStockInflow(tt) {
							// Inflow
							securityHoldings[tx.ISIN] = currQty + qty
							
							cost := tx.AcquisitionValue
							rate := tx.ExchangeRate
							if rate == 0 {
								rate = 1.0
							}
							
							if cost == 0 {
								cost = qty * tx.Price * rate
							} else {
								// Convert AcquisitionValue in foreign currency to base currency (DKK) if needed
								if tx.AcquisitionCurrency != "DKK" && tx.AcquisitionCurrency != "" {
									cost = cost * rate
								}
							}
							
							bookValues[tx.ISIN] = currBook + cost
							if tx.Price > 0 {
								lastPrices[tx.ISIN] = tx.Price
							}
						} else if IsStockOutflow(tt) {
							// Outflow
							newQty := currQty - qty
							if newQty < 0.00001 {
								newQty = 0
								bookValues[tx.ISIN] = 0
							} else {
								// Average cost basis reduction (AVCO)
								avgCost := currBook / currQty
								bookValues[tx.ISIN] = currBook - (qty * avgCost)
							}
							securityHoldings[tx.ISIN] = newQty
							if tx.Price > 0 {
								lastPrices[tx.ISIN] = tx.Price
							}
						}
					}
				}
			}
		}

		// Calculate total portfolio value for this timeline day
		dayCash := 0.0
		for _, bal := range depotBalances {
			dayCash += bal
		}

		daySecurities := 0.0
		dayETFs := 0.0

		for isin, qty := range securityHoldings {
			if qty > 0 {
				bookValue := bookValues[isin]
				class := db.Classifications[isin]
				if class == AssetClassETF {
					dayETFs += bookValue
				} else {
					daySecurities += bookValue
				}
			}
		}

		dayTotal := dayCash + daySecurities + dayETFs

		// 1) Calculate Simple Return % over time
		simpleReturn := 0.0
		if investedCapital > 0 {
			simpleReturn = (dayTotal - investedCapital) / investedCapital * 100.0
		}

		// 2) Calculate TWRR Daily Sub-Period Return
		dayExternalFlow := 0.0
		if dayTxs, exists := txGroups[dateStr]; exists {
			for _, tx := range dayTxs {
				if IsExternalCashFlow(tx, visibleAccounts) {
					dayExternalFlow += tx.Amount
				}
			}
		}

		rD := 0.0
		if prevTotal > 0 {
			denominator := prevTotal + math.Max(0, dayExternalFlow)
			if denominator > 0 {
				rD = (dayTotal - dayExternalFlow - prevTotal) / denominator
			}
		}
		cumTWRR = cumTWRR * (1.0 + rD)
		twrrPct := (cumTWRR - 1.0) * 100.0

		// 3) Calculate Drawdown % over time
		if dayTotal > cumMaxTotal {
			cumMaxTotal = dayTotal
		}
		drawdown := 0.0
		if cumMaxTotal > 0 {
			drawdown = (dayTotal - cumMaxTotal) / cumMaxTotal * 100.0
		}

		cashSeries = append(cashSeries, dayCash)
		securitiesSeries = append(securitiesSeries, daySecurities)
		etfsSeries = append(etfsSeries, dayETFs)
		totalSeries = append(totalSeries, dayTotal)
		investedCapitalSeries = append(investedCapitalSeries, investedCapital)
		feesSeries = append(feesSeries, cumFees)
		dividendsSeries = append(dividendsSeries, cumDividends)
		taxesSeries = append(taxesSeries, cumTaxes)
		simpleReturnSeries = append(simpleReturnSeries, simpleReturn)
		twrrSeries = append(twrrSeries, twrrPct)
		drawdownSeries = append(drawdownSeries, drawdown)

		prevTotal = dayTotal

		currDate = currDate.AddDate(0, 0, 1)
	}

	// ----------------------------------------------------
	// Calculate final current state using latest manual/overridden prices
	// ----------------------------------------------------
	var assetDetails []AssetDetail
	totalCashDKK := 0.0
	for _, bal := range depotBalances {
		totalCashDKK += bal
	}

	totalSecuritiesDKK := 0.0
	totalETFsDKK := 0.0
	totalBookValueDKK := 0.0

	for isin, qty := range securityHoldings {
		if qty > 0 {
			sym := db.AssetSymbols[isin]
			name := db.AssetNames[isin]
			if name == "" {
				name = sym
			}
			class := db.Classifications[isin]
			if class == "" {
				class = AssetClassSecurity
			}

			currency := db.ManualCurrencies[isin]
			if currency == "" {
				currency = "DKK"
			}

			rate := db.ExchangeRates[currency]
			if rate == 0 {
				rate = lastRates[isin] // fallback to last known transaction rate
			}
			if rate == 0 {
				rate = 1.0
			}

			bookValue := bookValues[isin]
			valueDKK := bookValue

			// Average cost price per share in native currency
			price := 0.0
			if qty > 0 {
				price = (bookValue / qty) / rate
			}

			gainLoss := 0.0
			gainLossPct := 0.0

			if class == AssetClassETF {
				totalETFsDKK += valueDKK
			} else {
				totalSecuritiesDKK += valueDKK
			}
			totalBookValueDKK += bookValue

			assetDetails = append(assetDetails, AssetDetail{
				ISIN:         isin,
				Symbol:       sym,
				Name:         name,
				Type:         class,
				Quantity:     qty,
				LastTxPrice:  lastPrices[isin],
				CurrentPrice: price,
				Currency:     currency,
				ValueDKK:     valueDKK,
				BookValueDKK: bookValue,
				GainLossDKK:  gainLoss,
				GainLossPct:  gainLossPct,
			})
		}
	}

	totalValueDKK := totalCashDKK + totalSecuritiesDKK + totalETFsDKK
	totalGainLossDKK := totalValueDKK - investedCapital
	totalGainLossPct := 0.0
	if investedCapital > 0 {
		totalGainLossPct = (totalGainLossDKK / investedCapital) * 100.0
	}

	// Calculate percentages and sort assets by value
	for i := range assetDetails {
		if totalValueDKK > 0 {
			assetDetails[i].Percentage = (assetDetails[i].ValueDKK / totalValueDKK) * 100.0
		}
	}

	sort.Slice(assetDetails, func(i, j int) bool {
		return assetDetails[i].ValueDKK > assetDetails[j].ValueDKK
	})

	// Add Cash pseudo-asset to details
	if totalCashDKK != 0 {
		cashPct := 0.0
		if totalValueDKK > 0 {
			cashPct = (totalCashDKK / totalValueDKK) * 100.0
		}
		assetDetails = append(assetDetails, AssetDetail{
			ISIN:         "-",
			Symbol:       "CASH",
			Name:         "Cash Holdings",
			Type:         AssetClassCash,
			Quantity:     1,
			LastTxPrice:  totalCashDKK,
			CurrentPrice: totalCashDKK,
			Currency:     "DKK",
			ValueDKK:     totalCashDKK,
			BookValueDKK: totalCashDKK,
			GainLossDKK:  0,
			GainLossPct:  0,
			Percentage:   cashPct,
		})
	}

	sort.Slice(assetDetails, func(i, j int) bool {
		return assetDetails[i].ValueDKK > assetDetails[j].ValueDKK
	})

	return PortfolioAnalysis{
		TotalValueDKK:         totalValueDKK,
		TotalSecuritiesDKK:    totalSecuritiesDKK,
		TotalETFsDKK:          totalETFsDKK,
		TotalCashDKK:          totalCashDKK,
		TotalBookValueDKK:     totalBookValueDKK,
		TotalGainLossDKK:      totalGainLossDKK,
		TotalGainLossPct:      totalGainLossPct,
		Assets:                assetDetails,
		Dates:                 dates,
		CashSeries:            cashSeries,
		SecuritiesSeries:      securitiesSeries,
		ETFsSeries:            etfsSeries,
		TotalSeries:           totalSeries,
		InvestedCapitalSeries: investedCapitalSeries,
		FeesSeries:            feesSeries,
		DividendsSeries:       dividendsSeries,
		TaxesSeries:           taxesSeries,
		SimpleReturnSeries:    simpleReturnSeries,
		TWRRSeries:            twrrSeries,
		DrawdownSeries:        drawdownSeries,
	}, nil
}
