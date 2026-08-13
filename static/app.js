// App State
let portfolioData = null;
let transactionList = null;
let currentTab = 'dashboard';
let growthRangeFilter = localStorage.getItem('growthRangeFilter') || 'ytd';
let growthMetric = localStorage.getItem('growthMetric') || 'value';
let availableAccounts = [];
let selectedAccounts = [];

// Chart.js Instances
let growthChartInstance = null;
let allocationChartInstance = null;

// On Page Load
document.addEventListener('DOMContentLoaded', () => {
    loadPortfolioData();
    loadTransactions();
});

// Switch Dashboard Tabs
function switchTab(tabId) {
    currentTab = tabId;
    
    // Update nav buttons
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    const activeBtn = document.querySelector(`.tab-btn[data-tab="${tabId}"]`);
    if (activeBtn) activeBtn.classList.add('active');

    // Update content sections
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    document.getElementById(`tab-${tabId}`).classList.add('active');

    // Trigger re-draws/re-loads if needed
    if (tabId === 'dashboard') {
        renderCharts();
    } else if (tabId === 'assets') {
        renderMetadataEditor();
    } else if (tabId === 'transactions') {
        renderTransactionsTable();
    }
}

// Fetch and load portfolio analysis
async function loadPortfolioData() {
    try {
        const response = await fetch(`/api/portfolio?accounts=${selectedAccounts.join(',')}`);
        if (!response.ok) throw new Error('Failed to load portfolio statistics');
        const data = await response.json();
        portfolioData = data;

        const isFXDisabled = portfolioData.db_summary && portfolioData.db_summary.disable_fx_api;

        // Check if auto-fetch is enabled and we haven't auto-fetched in this session yet
        if (!isFXDisabled && portfolioData.db_summary && portfolioData.db_summary.auto_fetch_rates && !window.autoFetchedThisSession && portfolioData.analysis.dates && portfolioData.analysis.dates.length > 0) {
            window.autoFetchedThisSession = true;
            await fetchAndAutoSaveLiveRates();
        } else if (!isFXDisabled && portfolioData.db_summary && portfolioData.analysis.dates && portfolioData.analysis.dates.length > 0) {
            // Silently check API connectivity in the background to update the status dot
            fetch('/api/live-rates')
                .then(res => updateApiStatusIndicator(res.ok))
                .catch(() => updateApiStatusIndicator(false));
        } else {
            // If FX is disabled or database is empty, update the status dot state cleanly
            updateApiStatusIndicator(false, false);
        }

        // If no transactions yet, show the importer prominently, hide analytics
        if (!portfolioData.analysis.dates || portfolioData.analysis.dates.length === 0) {
            document.getElementById('upload-panel').classList.remove('hidden');
            document.getElementById('analytics-grid').classList.add('hidden');
            updateKPICards(null);
        } else {
            // We have data! Show analytics and stats
            document.getElementById('upload-panel').classList.add('hidden');
            document.getElementById('analytics-grid').classList.remove('hidden');
            
            // Initialize available and selected accounts on first load or change
            if (portfolioData.db && portfolioData.db.transactions) {
                const txs = portfolioData.db.transactions;
                const uniqueAccs = [...new Set(txs.map(tx => tx.account).filter(acc => acc))];
                uniqueAccs.sort();
                
                const sameList = uniqueAccs.length === availableAccounts.length && uniqueAccs.every(v => availableAccounts.includes(v));
                if (!sameList || availableAccounts.length === 0) {
                    availableAccounts = uniqueAccs;
                    selectedAccounts = [...uniqueAccs];
                    renderAccountToggles();
                }
            }

            // Highlight active range filter button on the UI
            document.querySelectorAll('.btn-filter').forEach(btn => btn.classList.remove('active'));
            const activeBtn = document.getElementById(`btn-filter-${growthRangeFilter}`);
            if (activeBtn) activeBtn.classList.add('active');

            // Set metric selector value
            const metricSelect = document.getElementById('growth-metric-select');
            if (metricSelect) {
                metricSelect.value = growthMetric;
            }
            
            updateKPICards(portfolioData.analysis);
            renderHoldingsTable(portfolioData.analysis.assets);
            renderCharts();
        }
    } catch (err) {
        console.error(err);
        showUploadStatus(err.message, 'danger');
    }
}

// Fetch and load transactions ledger
async function loadTransactions() {
    try {
        const response = await fetch('/api/transactions');
        if (!response.ok) throw new Error('Failed to fetch transactions list');
        transactionList = await response.json();
        if (currentTab === 'transactions') {
            renderTransactionsTable();
        }
    } catch (err) {
        console.error(err);
    }
}

// Format numbers as Danish DKK or currency
function formatCurrency(val, currency = 'DKK') {
    const formatter = new Intl.NumberFormat('da-DK', {
        style: 'currency',
        currency: currency,
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
    });
    return formatter.format(val);
}

function formatFloat(val) {
    return new Intl.NumberFormat('da-DK', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 4
    }).format(val);
}

// Update KPI Metrics Cards
function updateKPICards(analysis) {
    if (!analysis) {
        document.querySelectorAll('.kpi-value').forEach(el => el.textContent = '0,00 DKK');
        document.getElementById('kpi-gain-loss').textContent = 'Afkast: 0,00 DKK (0,00%)';
        document.getElementById('kpi-securities-pct').textContent = 'Porteføljevægt: 0,00%';
        document.getElementById('kpi-etfs-pct').textContent = 'Porteføljevægt: 0,00%';
        document.getElementById('kpi-cash-pct').textContent = 'Porteføljevægt: 0,00%';
        return;
    }

    const total = analysis.total_value_dkk;
    const secWeight = total > 0 ? (analysis.total_securities_dkk / total) * 100 : 0;
    const etfWeight = total > 0 ? (analysis.total_etfs_dkk / total) * 100 : 0;
    const cashWeight = total > 0 ? (analysis.total_cash_dkk / total) * 100 : 0;

    // Samlet porteføljeværdi
    document.querySelector('#kpi-total-val .kpi-value').textContent = formatCurrency(total);
    const gainSign = analysis.total_gain_loss_dkk >= 0 ? '+' : '';
    const gainColor = analysis.total_gain_loss_dkk >= 0 ? 'var(--text-positive)' : 'var(--text-negative)';
    const gainLossEl = document.getElementById('kpi-gain-loss');
    if (gainLossEl) {
        gainLossEl.textContent = `Realiseret afkast: ${gainSign}${formatCurrency(analysis.total_gain_loss_dkk)} (${gainSign}${analysis.total_gain_loss_pct.toFixed(2)}%)`;
        gainLossEl.style.color = gainColor;
    }

    // Værdipapirer (Aktier/fonde)
    document.querySelector('#kpi-securities-val .kpi-value').textContent = formatCurrency(analysis.total_securities_dkk);
    document.getElementById('kpi-securities-pct').textContent = `Porteføljevægt: ${secWeight.toFixed(2)}%`;

    // ETF'er
    document.querySelector('#kpi-etfs-val .kpi-value').textContent = formatCurrency(analysis.total_etfs_dkk);
    document.getElementById('kpi-etfs-pct').textContent = `Porteføljevægt: ${etfWeight.toFixed(2)}%`;

    // Kontantbeholdning
    document.querySelector('#kpi-cash-val .kpi-value').textContent = formatCurrency(analysis.total_cash_dkk);
    document.getElementById('kpi-cash-pct').textContent = `Porteføljevægt: ${cashWeight.toFixed(2)}%`;
}

// File drop/upload feedback label
function updateFileLabel() {
    const input = document.getElementById('csv-file-input');
    const label = document.querySelector('.file-msg');
    if (input.files && input.files.length > 0) {
        label.textContent = input.files[0].name;
    } else {
        label.textContent = 'Vælg en CSV-fil...';
    }
}

// Show alert messages on upload/import
function showUploadStatus(msg, type) {
    const el = document.getElementById('upload-status');
    el.className = `alert alert-${type}`;
    el.innerHTML = msg;
    el.classList.remove('hidden');
}

// Handle Form Submission: Upload CSV
async function handleUpload(e) {
    e.preventDefault();
    const input = document.getElementById('csv-file-input');
    if (!input.files || input.files.length === 0) return;

    const file = input.files[0];
    
    // Scan and prompt user for friendly names of any brand-new accounts first
    await checkAndPromptNewAccounts(file);

    const formData = new FormData();
    formData.append('file', file);

    const btn = document.getElementById('upload-btn');
    btn.disabled = true;
    btn.textContent = 'Importerer...';

    showUploadStatus('Analyserer Nordnet CSV og importerer transaktioner. Venligst vent...', 'success');

    try {
        const response = await fetch('/api/upload', {
            method: 'POST',
            body: formData
        });
        const result = await response.json();

        if (!response.ok) {
            throw new Error(result.error || 'Der opstod en serverfejl under importen.');
        }

        const msg = `<strong>Import fuldført!</strong><br/>
                     - Rækker analyseret: ${result.parsed}<br/>
                     - Nye rækker tilføjet: <strong>${result.added}</strong><br/>
                     - Sprunget over (allerede indlæst): ${result.skipped}`;
        showUploadStatus(msg, 'success');
        
        // Reset file input
        input.value = '';
        updateFileLabel();

        // Reload data
        await loadPortfolioData();
        await loadTransactions();
    } catch (err) {
        showUploadStatus(`<strong>Import mislykkedes:</strong><br/>${err.message}`, 'danger');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Indlæs og importer';
    }
}

// Handle single-click file select from header button
async function handleHeaderFileSelect(input) {
    if (!input || !input.files || input.files.length === 0) return;

    const file = input.files[0];
    
    // Scan and prompt user for friendly names of any brand-new accounts first
    await checkAndPromptNewAccounts(file);

    const formData = new FormData();
    formData.append('file', file);

    const statusEl = document.getElementById('header-import-status');
    if (statusEl) {
        statusEl.className = 'alert alert-success';
        statusEl.innerHTML = '<strong>Importerer CSV...</strong> Analyserer og sammensmelter Nordnet transaktioner med det samme. Venligst vent...';
        statusEl.classList.remove('hidden');
    }

    try {
        const response = await fetch('/api/upload', {
            method: 'POST',
            body: formData
        });
        const result = await response.json();

        if (!response.ok) {
            throw new Error(result.error || 'Der opstod en serverfejl under importen.');
        }

        if (statusEl) {
            statusEl.className = 'alert alert-success';
            statusEl.innerHTML = `<strong>Import fuldført!</strong> Analyserede rækker: ${result.parsed} | Nye rækker tilføjet: <strong>${result.added}</strong> | Sprunget over (dubletter): ${result.skipped}.`;
            
            // Auto-hide the success notification after 7 seconds
            setTimeout(() => {
                statusEl.classList.add('hidden');
            }, 7000);
        }

        // Reload dashboard and transaction ledger immediately
        await loadPortfolioData();
        await loadTransactions();
    } catch (err) {
        if (statusEl) {
            statusEl.className = 'alert alert-danger';
            statusEl.innerHTML = `<strong>Import mislykkedes:</strong> ${err.message}`;
        }
    } finally {
        // Reset file input so same file can be imported again if needed
        input.value = '';
    }
}

// Render My Asset Holdings Table
function renderHoldingsTable(assets) {
    const tbody = document.querySelector('#holdings-table tbody');
    tbody.innerHTML = '';

    if (!assets || assets.length === 0) {
        tbody.innerHTML = `<tr><td colspan="6" class="no-data-msg">Ingen aktive værdipapirbeholdninger i din portefølje.</td></tr>`;
        return;
    }

    assets.forEach(asset => {
        const isCash = asset.type === 'Cash';
        const badgeClass = asset.type === 'ETF' ? 'badge-etf' : (isCash ? 'badge-cash' : 'badge-security');
        const dispType = asset.type === 'ETF' ? "ETF" : (isCash ? "Kontanter" : "Værdipapir");

        const row = document.createElement('tr');
        row.innerHTML = `
            <td>
                <strong>${asset.name || asset.symbol || asset.isin}</strong><br/>
                <small class="text-muted" style="font-size: 0.725rem;">${asset.symbol ? asset.symbol + ' • ' : ''}${asset.isin}</small>
            </td>
            <td><span class="asset-badge ${badgeClass}">${dispType}</span></td>
            <td class="text-right">${isCash ? '-' : formatFloat(asset.quantity)}</td>
            <td class="text-right">${isCash ? '-' : formatCurrency(asset.current_price, asset.currency)}</td>
            <td class="text-right">${isCash ? '-' : formatCurrency(asset.book_value_dkk)}</td>
            <td class="text-right"><strong>${asset.percentage.toFixed(2)}%</strong></td>
        `;
        tbody.appendChild(row);
    });
}

// Switch growth range filters (YTD, MTD, Max, 1Y, 6M, 1M)
function filterGrowthRange(range) {
    growthRangeFilter = range;
    localStorage.setItem('growthRangeFilter', range);
    
    document.querySelectorAll('.btn-filter').forEach(btn => btn.classList.remove('active'));
    const activeBtn = Array.from(document.querySelectorAll('.btn-filter')).find(btn => 
        btn.id === `btn-filter-${range}`
    );
    if (activeBtn) activeBtn.classList.add('active');

    renderCharts();
}

// Slice the dataset arrays according to selected growth range filter
function filterChartData(dates, cash, securities, etfs, total, invested, fees, dividends, taxes, simple, twr, drawdown) {
    if (!dates || dates.length === 0) {
        return { dates, cash, securities, etfs, total, invested, fees, dividends, taxes, simple, twr, drawdown };
    }

    if (growthRangeFilter === 'ytd') {
        const lastDateStr = dates[dates.length - 1];
        const lastDate = new Date(lastDateStr);
        const startOfYearStr = `${lastDate.getFullYear()}-01-01`;
        
        const index = dates.findIndex(d => d >= startOfYearStr);
        if (index === -1) {
            return { dates, cash, securities, etfs, total, invested, fees, dividends, taxes, simple, twr, drawdown };
        }

        return {
            dates: dates.slice(index),
            cash: cash.slice(index),
            securities: securities.slice(index),
            etfs: etfs.slice(index),
            total: total.slice(index),
            invested: invested.slice(index),
            fees: fees ? fees.slice(index) : [],
            dividends: dividends ? dividends.slice(index) : [],
            taxes: taxes ? taxes.slice(index) : [],
            simple: simple ? simple.slice(index) : [],
            twr: twr ? twr.slice(index) : [],
            drawdown: drawdown ? drawdown.slice(index) : []
        };
    }

    if (growthRangeFilter === 'mtd') {
        const lastDateStr = dates[dates.length - 1];
        const lastDate = new Date(lastDateStr);
        const month = String(lastDate.getMonth() + 1).padStart(2, '0');
        const startOfMonthStr = `${lastDate.getFullYear()}-${month}-01`;
        
        const index = dates.findIndex(d => d >= startOfMonthStr);
        if (index === -1) {
            return { dates, cash, securities, etfs, total, invested, fees, dividends, taxes, simple, twr, drawdown };
        }

        return {
            dates: dates.slice(index),
            cash: cash.slice(index),
            securities: securities.slice(index),
            etfs: etfs.slice(index),
            total: total.slice(index),
            invested: invested.slice(index),
            fees: fees ? fees.slice(index) : [],
            dividends: dividends ? dividends.slice(index) : [],
            taxes: taxes ? taxes.slice(index) : [],
            simple: simple ? simple.slice(index) : [],
            twr: twr ? twr.slice(index) : [],
            drawdown: drawdown ? drawdown.slice(index) : []
        };
    }

    if (growthRangeFilter === 'all') {
        return { dates, cash, securities, etfs, total, invested, fees, dividends, taxes, simple, twr, drawdown };
    }

    const lastDateStr = dates[dates.length - 1];
    const lastDate = new Date(lastDateStr);
    let filterDate = new Date(lastDate);

    if (growthRangeFilter === '1y') filterDate.setFullYear(lastDate.getFullYear() - 1);
    else if (growthRangeFilter === '6m') filterDate.setMonth(lastDate.getMonth() - 6);
    else if (growthRangeFilter === '1m') filterDate.setMonth(lastDate.getMonth() - 1);

    // Find the index of the first date that is on or after the filterDate
    const index = dates.findIndex(dStr => new Date(dStr) >= filterDate);
    if (index === -1) {
        return { dates, cash, securities, etfs, total, invested, fees, dividends, taxes, simple, twr, drawdown };
    }

    return {
        dates: dates.slice(index),
        cash: cash.slice(index),
        securities: securities.slice(index),
        etfs: etfs.slice(index),
        total: total.slice(index),
        invested: invested.slice(index),
        fees: fees ? fees.slice(index) : [],
        dividends: dividends ? dividends.slice(index) : [],
        taxes: taxes ? taxes.slice(index) : [],
        simple: simple ? simple.slice(index) : [],
        twr: twr ? twr.slice(index) : [],
        drawdown: drawdown ? drawdown.slice(index) : []
    };
}

// Render Charts (Growth and Allocation)
function renderCharts() {
    if (!portfolioData || !portfolioData.analysis || !portfolioData.analysis.dates) return;

    const analysis = portfolioData.analysis;

    // Filter growth datasets based on time range
    const filtered = filterChartData(
        analysis.dates,
        analysis.cash_series,
        analysis.securities_series,
        analysis.etfs_series,
        analysis.total_series,
        analysis.invested_capital_series,
        analysis.fees_series,
        analysis.dividends_series,
        analysis.taxes_series,
        analysis.simple_return_series,
        analysis.twrr_series,
        analysis.drawdown_series
    );

    // Update Period Performance report card next to the chart
    updatePeriodReturnMetrics(filtered);

    // Render both charts after a brief, silent timeout to guarantee that the CSS Grid
    // column tracking calculations are 100% complete and non-zero on the browser.
    setTimeout(() => {
        // 1. GROWTH CHART (LINE OR BAR)
        const ctxGrowth = 'growthChart';
        if (growthChartInstance) {
            growthChartInstance.destroy();
        }

        // Configure datasets, tooltips, types, labels and Y-axis scales depending on active growth metric mode
        let chartType = 'line';
        let chartLabels = filtered.dates;
        let datasets = [];
    let tooltipCallbackLabel = null;
    let yAxisTicksCallback = null;

    if (growthMetric === 'simple') {
        // Calculate simulated MSCI World Benchmark (8% p.a. opportunity cost starting from 0%)
        const benchmarkData = [];
        const annualRate = 0.08;
        const dailyRate = Math.pow(1 + annualRate, 1 / 365) - 1;
        for (let i = 0; i < filtered.dates.length; i++) {
            const val = (Math.pow(1 + dailyRate, i) - 1) * 100;
            benchmarkData.push(val);
        }

        datasets = [
            {
                label: 'Simpelt afkast (%)',
                data: filtered.simple,
                borderColor: '#10b981',
                backgroundColor: 'rgba(16, 185, 129, 0.05)',
                borderWidth: 3,
                fill: true,
                tension: 0.1,
                pointRadius: filtered.dates.length < 50 ? 3 : 0,
                pointHoverRadius: 6
            },
            {
                label: 'MSCI World (Benchmark 8% p.a.)',
                data: benchmarkData,
                borderColor: '#f59e0b',
                borderWidth: 1.5,
                borderDash: [5, 5],
                fill: false,
                tension: 0.1,
                pointRadius: 0,
                pointHoverRadius: 4
            }
        ];
        tooltipCallbackLabel = function(context) {
            return `${context.dataset.label}: ${context.parsed.y.toFixed(2)}%`;
        };
        yAxisTicksCallback = function(value) {
            return `${value.toFixed(1)}%`;
        };
    } else if (growthMetric === 'twr') {
        // Calculate simulated MSCI World Benchmark (8% p.a. opportunity cost starting from 0%)
        const benchmarkData = [];
        const annualRate = 0.08;
        const dailyRate = Math.pow(1 + annualRate, 1 / 365) - 1;
        for (let i = 0; i < filtered.dates.length; i++) {
            const val = (Math.pow(1 + dailyRate, i) - 1) * 100;
            benchmarkData.push(val);
        }

        datasets = [
            {
                label: 'Tidsvægtet afkast (TWR %)',
                data: filtered.twr,
                borderColor: '#38bdf8',
                backgroundColor: 'rgba(56, 189, 248, 0.05)',
                borderWidth: 3,
                fill: true,
                tension: 0.1,
                pointRadius: filtered.dates.length < 50 ? 3 : 0,
                pointHoverRadius: 6
            },
            {
                label: 'MSCI World (Benchmark 8% p.a.)',
                data: benchmarkData,
                borderColor: '#f59e0b',
                borderWidth: 1.5,
                borderDash: [5, 5],
                fill: false,
                tension: 0.1,
                pointRadius: 0,
                pointHoverRadius: 4
            }
        ];
        tooltipCallbackLabel = function(context) {
            return `${context.dataset.label}: ${context.parsed.y.toFixed(2)}%`;
        };
        yAxisTicksCallback = function(value) {
            return `${value.toFixed(1)}%`;
        };
    } else if (growthMetric === 'drawdown') {
        datasets = [
            {
                label: 'Portefølje drawdown (%)',
                data: filtered.drawdown,
                borderColor: '#ef4444',
                backgroundColor: 'rgba(239, 68, 68, 0.08)',
                borderWidth: 2,
                fill: true,
                tension: 0.1,
                pointRadius: 0,
                pointHoverRadius: 4
            }
        ];
        tooltipCallbackLabel = function(context) {
            return `${context.dataset.label}: ${context.parsed.y.toFixed(2)}%`;
        };
        yAxisTicksCallback = function(value) {
            return `${value.toFixed(1)}%`;
        };
    } else if (growthMetric === 'monthly') {
        chartType = 'bar';
        
        // Group daily metrics by Year-Month
        const monthsMap = {};
        for (let i = 0; i < filtered.dates.length; i++) {
            const dStr = filtered.dates[i];
            const yyyymm = dStr.substring(0, 7);
            if (!monthsMap[yyyymm]) {
                monthsMap[yyyymm] = [];
            }
            monthsMap[yyyymm].push({
                index: i,
                total: filtered.total[i],
                invested: filtered.invested[i]
            });
        }

        const monthlyLabels = [];
        const monthlyData = [];

        Object.keys(monthsMap).sort().forEach(yyyymm => {
            const days = monthsMap[yyyymm];
            const firstDay = days[0];
            const lastDay = days[days.length - 1];
            
            const startVal = firstDay.total;
            const endVal = lastDay.total;
            const netInflows = lastDay.invested - firstDay.invested;
            
            const gainLoss = endVal - startVal - netInflows;
            const denominator = startVal + Math.max(0, netInflows);
            let monthlyPct = 0.0;
            if (denominator > 0) {
                monthlyPct = (gainLoss / denominator) * 100.0;
            } else if (startVal > 0) {
                monthlyPct = (gainLoss / startVal) * 100.0;
            }
            
            const monthNames = ["Jan", "Feb", "Mar", "Apr", "Maj", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dec"];
            const mIdx = parseInt(yyyymm.substring(5, 7)) - 1;
            const label = `${monthNames[mIdx]} ${yyyymm.substring(0, 4)}`;
            
            monthlyLabels.push(label);
            monthlyData.push(monthlyPct);
        });

        chartLabels = monthlyLabels;

        // Custom color map (green bars for profitable months, red bars for pullbacks)
        const barColors = monthlyData.map(val => val >= 0 ? 'rgba(16, 185, 129, 0.7)' : 'rgba(239, 68, 68, 0.7)');
        const borderColors = monthlyData.map(val => val >= 0 ? '#10b981' : '#ef4444');

        datasets = [
            {
                label: 'Månedligt afkast (%)',
                data: monthlyData,
                backgroundColor: barColors,
                borderColor: borderColors,
                borderWidth: 1.5,
                borderRadius: 4
            }
        ];
        tooltipCallbackLabel = function(context) {
            return `${context.dataset.label}: ${context.parsed.y.toFixed(2)}%`;
        };
        yAxisTicksCallback = function(value) {
            return `${value.toFixed(1)}%`;
        };
    } else {
        // 'value' (Default portfolio value in DKK)
        datasets = [
            {
                label: 'Samlet værdi',
                data: filtered.total,
                borderColor: '#38bdf8',
                backgroundColor: 'rgba(56, 189, 248, 0.05)',
                borderWidth: 3,
                fill: true,
                tension: 0.1,
                pointRadius: filtered.dates.length < 50 ? 3 : 0,
                pointHoverRadius: 6
            },
            {
                label: 'Investeret kapital',
                data: filtered.invested,
                borderColor: '#64748b',
                borderWidth: 1.5,
                borderDash: [5, 5],
                fill: false,
                tension: 0,
                pointRadius: 0,
                pointHoverRadius: 4
            },
            {
                label: 'Værdipapirer (Aktier/fonde)',
                data: filtered.securities,
                borderColor: '#10b981',
                borderWidth: 1.5,
                fill: false,
                tension: 0.1,
                pointRadius: 0,
                pointHoverRadius: 4
            },
            {
                label: 'ETF\'er',
                data: filtered.etfs,
                borderColor: '#818cf8',
                borderWidth: 1.5,
                fill: false,
                tension: 0.1,
                pointRadius: 0,
                pointHoverRadius: 4
            },
            {
                label: 'Kontanter',
                data: filtered.cash,
                borderColor: '#f59e0b',
                borderWidth: 1.5,
                fill: false,
                tension: 0.1,
                pointRadius: 0,
                pointHoverRadius: 4
            }
        ];
        tooltipCallbackLabel = function(context) {
            return `${context.dataset.label}: ${formatCurrency(context.parsed.y)}`;
        };
        yAxisTicksCallback = function(value) {
            return formatCurrency(value);
        };
    }

    growthChartInstance = new Chart(ctxGrowth, {
        type: chartType,
        data: {
            labels: chartLabels,
            datasets: datasets
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
                mode: 'index',
                intersect: false
            },
            plugins: {
                legend: {
                    position: 'top',
                    labels: { color: '#94a3b8', font: { family: 'inherit', size: 11 } }
                },
                tooltip: {
                    callbacks: {
                        label: tooltipCallbackLabel
                    }
                }
            },
            scales: {
                x: {
                    grid: { color: '#334155', drawOnChartArea: false },
                    ticks: { color: '#94a3b8', font: { family: 'inherit', size: 10 } }
                },
                y: {
                    grid: { color: 'rgba(51, 65, 85, 0.5)' },
                    ticks: { 
                        color: '#94a3b8', 
                        font: { family: 'inherit', size: 10 },
                        callback: yAxisTicksCallback
                    }
                }
            }
        }
    });

    // 2. ALLOCATION CHART (DOUGHNUT)
    const ctxAlloc = 'allocationChart';
    if (allocationChartInstance) {
        allocationChartInstance.destroy();
    }

    const secVal = analysis.total_securities_dkk;
    const etfVal = analysis.total_etfs_dkk;
    const cashVal = analysis.total_cash_dkk;
    const totalVal = secVal + etfVal + cashVal;

    // Update total value in the card's subtitle dynamically
    const subtitle = document.querySelector('.allocation-card .card-subtitle');
    if (subtitle) {
        subtitle.textContent = totalVal > 0 ? `Samlet værdi: ${formatCurrency(totalVal)}` : 'Aktuel markedsværdi fordelt på aktiver';
    }

    // Update custom HTML mini-legend next to the chart inside the top banner card
    const legendContainer = document.getElementById('allocation-mini-legend');
    if (legendContainer) {
        legendContainer.innerHTML = '';
        if (totalVal > 0) {
            const items = [];
            if (secVal > 0) items.push({ label: 'Værdipapirer', val: secVal, color: '#10b981' });
            if (etfVal > 0) items.push({ label: "ETF'er", val: etfVal, color: '#818cf8' });
            if (cashVal !== 0) items.push({ label: 'Kontanter', val: cashVal, color: '#f59e0b' });
            
            items.forEach(item => {
                const pct = (item.val / totalVal) * 100;
                const div = document.createElement('div');
                div.style.display = 'flex';
                div.style.alignItems = 'center';
                div.style.gap = '0.35rem';
                div.innerHTML = `
                    <span style="display: inline-block; width: 8px; height: 8px; border-radius: 50%; background-color: ${item.color};"></span>
                    <span style="color: var(--text-muted); font-size: 0.75rem;">${item.label}: <strong style="color: var(--text-primary);">${pct.toFixed(1)}%</strong></span>
                `;
                legendContainer.appendChild(div);
            });
        } else {
            legendContainer.innerHTML = '<span>Ingen aktive beholdninger</span>';
        }
    }

    const allocLabels = [];
    const allocData = [];
    const allocColors = [];

    if (secVal > 0) {
        allocLabels.push('Værdipapirer');
        allocData.push(secVal);
        allocColors.push('#10b981');
    }
    if (etfVal > 0) {
        allocLabels.push("ETF'er");
        allocData.push(etfVal);
        allocColors.push('#818cf8');
    }
    if (cashVal !== 0) {
        allocLabels.push('Kontanter');
        allocData.push(Math.max(0, cashVal));
        allocColors.push('#f59e0b');
    }

    // Default if empty
    if (allocData.length === 0) {
        allocLabels.push('Ingen aktiver');
        allocData.push(1);
        allocColors.push('#334155');
    }

    allocationChartInstance = new Chart(ctxAlloc, {
        type: 'doughnut',
        data: {
            labels: allocLabels,
            datasets: [{
                data: allocData,
                backgroundColor: allocColors,
                borderColor: '#1e293b',
                borderWidth: 2
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false // Hide default legend since we render a gorgeous custom HTML legend on the left!
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            if (context.label === 'Ingen aktiver') return 'Ingen aktiver';
                            return ` ${context.label}: ${formatCurrency(context.raw)}`;
                        }
                    }
                }
            },
            cutout: '65%'
        }
    });
}, 50);
}

// Render Asset Metadata and Prices Editor Form
function renderMetadataEditor() {
    if (!portfolioData || !portfolioData.db) return;

    const dbSummary = portfolioData.db_summary;
    const db = portfolioData.db;
    const tbody = document.querySelector('#metadata-editor-table tbody');
    tbody.innerHTML = '';

    const isins = Object.keys(db.asset_symbols || {});

    if (isins.length === 0) {
        tbody.innerHTML = `<tr><td colspan="6" class="no-data-msg">No assets registered in the database. Please import transaction CSV first.</td></tr>`;
        return;
    }

    // Sort ISINs alphabetically by symbol for a stable display
    isins.sort((a, b) => (db.asset_symbols[a] || '').localeCompare(db.asset_symbols[b] || ''));

    isins.forEach(isin => {
        const symbol = db.asset_symbols[isin] || '';
        const name = db.asset_names[isin] || symbol || isin;
        const type = db.classifications[isin] || 'Security';
        const currency = db.manual_currencies[isin] || 'DKK';

        // Find average unit cost from processed ledger
        const assetAnalysis = portfolioData.analysis && portfolioData.analysis.assets 
            ? portfolioData.analysis.assets.find(a => a.isin === isin) 
            : null;
        const avgUnitCost = assetAnalysis ? assetAnalysis.current_price : 0;
        const avgUnitCostFormatted = avgUnitCost > 0 ? formatCurrency(avgUnitCost, currency) : '-';

        const row = document.createElement('tr');
        row.innerHTML = `
            <td><code>${isin}</code></td>
            <td>
                <input type="text" name="symbol-${isin}" value="${symbol}" placeholder="Ticker Symbol">
            </td>
            <td>
                <input type="text" name="name-${isin}" value="${name}" placeholder="Friendly Asset Name">
            </td>
            <td>
                <select name="class-${isin}">
                    <option value="Security" ${type === 'Security' ? 'selected' : ''}>Security (Stock / Mutual Fund)</option>
                    <option value="ETF" ${type === 'ETF' ? 'selected' : ''}>ETF (Exchange Traded Fund)</option>
                </select>
            </td>
            <td class="text-right" style="font-weight: 600; color: var(--text-muted); padding-top: 1.35rem;">
                ${avgUnitCostFormatted}
            </td>
            <td>
                <select name="currency-${isin}">
                    <option value="DKK" ${currency === 'DKK' ? 'selected' : ''}>DKK</option>
                    <option value="EUR" ${currency === 'EUR' ? 'selected' : ''}>EUR</option>
                    <option value="USD" ${currency === 'USD' ? 'selected' : ''}>USD</option>
                    <option value="SEK" ${currency === 'SEK' ? 'selected' : ''}>SEK</option>
                    <option value="NOK" ${currency === 'NOK' ? 'selected' : ''}>NOK</option>
                </select>
            </td>
        `;
        tbody.appendChild(row);
    });

    // Sync checkbox and warning states
    const autoFetchChecked = dbSummary.auto_fetch_rates || false;
    const checkbox = document.getElementById('setting-auto-fetch');
    const warning = document.getElementById('warning-auto-fetch');
    const fetchBtn = document.getElementById('btn-fetch-rates');

    if (dbSummary.disable_fx_api) {
        if (checkbox) {
            checkbox.checked = false;
            checkbox.disabled = true;
        }
        if (warning) {
            warning.style.display = 'block';
            warning.innerHTML = '<strong>FX-API deaktiveret:</strong> Hentning af live-valutakurser er deaktiveret på denne server. Indtast og administrer venligst dine valutakurser manuelt.';
            warning.style.backgroundColor = 'rgba(239, 68, 68, 0.08)';
            warning.style.borderColor = 'rgba(239, 68, 68, 0.25)';
            warning.style.color = '#fca5a5';
        }
        if (fetchBtn) {
            fetchBtn.disabled = true;
            fetchBtn.textContent = 'FX-API deaktiveret';
        }
    } else {
        if (checkbox) {
            checkbox.checked = autoFetchChecked;
            checkbox.disabled = false;
        }
        if (warning) {
            warning.style.display = autoFetchChecked ? 'block' : 'none';
            warning.innerHTML = '<strong>Bemærk:</strong> Når automatisk hentning er aktiveret, vil alle manuelle justeringer af disse kurser automatisk blive overskrevet af live-markedskurser fra Frankfurter (ECB), hver gang dashboardet åbnes.';
            warning.style.backgroundColor = '';
            warning.style.borderColor = '';
            warning.style.color = '';
        }
        if (fetchBtn) {
            fetchBtn.disabled = false;
            fetchBtn.textContent = 'Hent live-kurser';
        }
    }

    // Populate exchange rates grid
    const ratesContainer = document.getElementById('rates-grid-container');
    ratesContainer.innerHTML = '';
    
    const rates = dbSummary.exchange_rates || { "DKK": 1, "EUR": 7.46, "USD": 6.90 };
    Object.keys(rates).forEach(currency => {
        const div = document.createElement('div');
        div.className = 'rate-control';
        div.innerHTML = `
            <label for="rate-${currency}">1 ${currency} in DKK</label>
            <input type="number" step="any" name="rate-${currency}" id="rate-${currency}" value="${rates[currency]}" ${currency === 'DKK' ? 'readonly' : ''} oninput="updateRateDiffBadges()">
        `;
        ratesContainer.appendChild(div);
    });

    // Populate diff badges for rates
    updateRateDiffBadges();

    // Populate account friendly names renaming grid
    const renameContainer = document.getElementById('accounts-renaming-container');
    if (renameContainer) {
        renameContainer.innerHTML = '';
        const dbAccNames = db.account_names || {};
        availableAccounts.forEach(acc => {
            const friendlyName = dbAccNames[acc] || acc;
            const div = document.createElement('div');
            div.className = 'rate-control';
            div.innerHTML = `
                <label for="accname-${acc}">Account ID ${acc}</label>
                <input type="text" name="accname-${acc}" id="accname-${acc}" value="${friendlyName}" placeholder="Friendly Label">
            `;
            renameContainer.appendChild(div);
        });
    }
}

// Save Metadata/Prices and Exchange Rates changes
async function handleMetadataSave(e) {
    e.preventDefault();
    if (!portfolioData || !portfolioData.db) return;

    const form = document.getElementById('metadata-form');
    const formData = new FormData(form);

    const classifications = {};
    const asset_names = {};
    const manual_prices = {};
    const manual_currencies = {};
    const exchange_rates = {};

    const isins = Object.keys(portfolioData.db.asset_symbols || {});

    // Parse assets fields
    isins.forEach(isin => {
        const nameVal = form.querySelector(`[name="name-${isin}"]`).value.trim();
        if (nameVal) asset_names[isin] = nameVal;

        const classVal = form.querySelector(`[name="class-${isin}"]`).value;
        classifications[isin] = classVal;

        manual_prices[isin] = portfolioData.db.manual_prices[isin] || 0;

        const currVal = form.querySelector(`[name="currency-${isin}"]`).value;
        manual_currencies[isin] = currVal;
    });

    // Parse currency rates fields
    const currencies = Object.keys(portfolioData.db_summary.exchange_rates || { "DKK": 1, "EUR": 7.46, "USD": 6.90 });
    currencies.forEach(currency => {
        const rateInput = document.getElementById(`rate-${currency}`);
        if (rateInput) {
            const rateVal = parseFloat(rateInput.value);
            exchange_rates[currency] = isNaN(rateVal) ? 1.0 : rateVal;
        }
    });

    // Parse account friendly names
    const account_names = {};
    availableAccounts.forEach(acc => {
        const nameInput = document.getElementById(`accname-${acc}`);
        if (nameInput) {
            const nameVal = nameInput.value.trim();
            account_names[acc] = nameVal ? nameVal : acc;
        }
    });

    const autoFetchCheckbox = document.getElementById('setting-auto-fetch');
    const auto_fetch_rates = autoFetchCheckbox ? autoFetchCheckbox.checked : false;

    const payload = {
        classifications,
        asset_names,
        manual_prices,
        manual_currencies,
        exchange_rates,
        account_names,
        auto_fetch_rates
    };

    const statusEl = document.getElementById('metadata-status');
    statusEl.textContent = 'Gemmer...';
    statusEl.className = 'alert-inline';
    statusEl.classList.remove('hidden');

    try {
        const response = await fetch('/api/metadata', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const result = await response.json();

        if (!response.ok) throw new Error(result.error || 'Kunne ikke opdatere metadata på serveren');

        statusEl.textContent = 'Ændringer gemt med succes!';
        statusEl.className = 'alert-inline text-positive';
        
        // Reload dashboard immediately to reflect changed values
        await loadPortfolioData();
        // Re-render editor inputs to clear diff badges and sync active checkboxes
        renderMetadataEditor();

        setTimeout(() => {
            statusEl.classList.add('hidden');
        }, 3000);
    } catch (err) {
        statusEl.textContent = `Fejl: ${err.message}`;
        statusEl.className = 'alert-inline text-negative';
    }
}

// Render Transactions Ledger log table
function renderTransactionsTable() {
    const tbody = document.querySelector('#transactions-ledger-table tbody');
    tbody.innerHTML = '';

    if (!transactionList || transactionList.length === 0) {
        tbody.innerHTML = `<tr><td colspan="12" class="no-data-msg">Ingen transaktioner indlæst. Gå til Overblik for at importere din CSV.</td></tr>`;
        return;
    }

    transactionList.forEach(tx => {
        const amountClass = tx.amount >= 0 ? 'text-positive' : 'text-negative';
        const amountSign = tx.amount >= 0 ? '+' : '';

        const row = document.createElement('tr');
        row.innerHTML = `
            <td><small><code>${tx.id}</code></small></td>
            <td>${tx.booking_date}</td>
            <td>${tx.trade_date}</td>
            <td><code>${tx.account}</code></td>
            <td><strong>${tx.transaction_type}</strong></td>
            <td><strong>${tx.symbol || '-'}</strong></td>
            <td><code>${tx.isin || '-'}</code></td>
            <td class="text-right">${tx.quantity !== 0 ? formatFloat(tx.quantity) : '-'}</td>
            <td class="text-right">${tx.price !== 0 ? formatCurrency(tx.price, tx.acquisition_currency || tx.amount_currency) : '-'}</td>
            <td class="text-right ${amountClass}">${amountSign}${formatCurrency(tx.amount, tx.amount_currency)}</td>
            <td class="text-right">${tx.balance !== 0 ? formatCurrency(tx.balance, tx.amount_currency) : '-'}</td>
            <td><small>${tx.transaction_text || ''}</small></td>
        `;
        tbody.appendChild(row);
    });
}

// Client-side quick filter / search for transactions table
function filterTransactionsTable() {
    const query = document.getElementById('tx-search-input').value.toLowerCase();
    const rows = document.querySelectorAll('#transactions-ledger-table tbody tr');

    rows.forEach(row => {
        if (row.classList.contains('no-data-msg')) return;
        const text = row.textContent.toLowerCase();
        if (text.includes(query)) {
            row.style.display = '';
        } else {
            row.style.display = 'none';
        }
    });
}

// Confirm and trigger Database Reset
async function confirmResetDB() {
    if (!confirm('ADVARSEL: Er du sikker på, at du vil nulstille databasen? Dette vil permanent slette ALLE importerede transaktioner og aktivindstillinger!')) {
        return;
    }

    try {
        const response = await fetch('/api/reset', { method: 'POST' });
        const result = await response.json();

        if (!response.ok) throw new Error(result.error || 'Kunne ikke nulstille databasen');

        alert('Databasen er blevet tømt med succes!');
        
        // Reload and switch back to dashboard tab
        portfolioData = null;
        transactionList = null;
        switchTab('dashboard');
        await loadPortfolioData();
        await loadTransactions();
    } catch (err) {
        alert(`Fejl under sletning af database: ${err.message}`);
    }
}



// Calculate and update portfolio performance metrics relative to selected time period
function updatePeriodReturnMetrics(filtered) {
    const dkkEl = document.getElementById('period-return-dkk');
    const pctEl = document.getElementById('period-return-pct');
    const startEl = document.getElementById('period-start-val');
    const endEl = document.getElementById('period-end-val');
    const depositsEl = document.getElementById('period-net-deposits');
    const dividendsEl = document.getElementById('period-dividends');
    const taxesEl = document.getElementById('period-taxes');
    const feesEl = document.getElementById('period-fees');
    const startLbl = document.getElementById('period-start-date-lbl');
    const endLbl = document.getElementById('period-end-date-lbl');
    const badge = document.getElementById('period-badge');

    if (!dkkEl || !pctEl || !startEl || !endEl || !depositsEl || !startLbl || !endLbl) return;

    if (!filtered || !filtered.dates || filtered.dates.length === 0) {
        dkkEl.textContent = '0,00 DKK';
        pctEl.textContent = '0,00%';
        return;
    }

    const n = filtered.dates.length;
    const startDate = filtered.dates[0];
    const endDate = filtered.dates[n - 1];

    const startVal = filtered.total[0];
    const endVal = filtered.total[n - 1];

    // Net external deposits during this filtered period is the difference in cumulative invested capital
    const startInvested = filtered.invested[0];
    const endInvested = filtered.invested[n - 1];
    const netDeposits = endInvested - startInvested;

    // Period-specific fees, dividends, and taxes from the chronological series
    const startFees = filtered.fees && filtered.fees.length > 0 ? filtered.fees[0] : 0;
    const endFees = filtered.fees && filtered.fees.length > 0 ? filtered.fees[n - 1] : 0;
    const periodFees = endFees - startFees;

    const startDividends = filtered.dividends && filtered.dividends.length > 0 ? filtered.dividends[0] : 0;
    const endDividends = filtered.dividends && filtered.dividends.length > 0 ? filtered.dividends[n - 1] : 0;
    const periodDividends = endDividends - startDividends;

    const startTaxes = filtered.taxes && filtered.taxes.length > 0 ? filtered.taxes[0] : 0;
    const endTaxes = filtered.taxes && filtered.taxes.length > 0 ? filtered.taxes[n - 1] : 0;
    const periodTaxes = endTaxes - startTaxes;

    // Period Gain/Loss = Ending Value - Starting Value - Net Deposits
    const periodGainLoss = endVal - startVal - netDeposits;

    // Period Gain/Loss % relative to starting capital + deposits
    let periodGainLossPct = 0.0;
    const denominator = startVal + Math.max(0, netDeposits);
    if (denominator > 0) {
        periodGainLossPct = (periodGainLoss / denominator) * 100.0;
    } else if (startVal > 0) {
        periodGainLossPct = (periodGainLoss / startVal) * 100.0;
    }

    // Update large display
    dkkEl.textContent = formatCurrency(periodGainLoss);
    
    const sign = periodGainLoss >= 0 ? '+' : '';
    pctEl.textContent = `${sign}${periodGainLossPct.toFixed(2)}% i perioden`;
    
    if (periodGainLoss >= 0) {
        dkkEl.className = 'period-return-value text-positive';
        pctEl.className = 'period-return-percent text-positive';
    } else {
        dkkEl.className = 'period-return-value text-negative';
        pctEl.className = 'period-return-percent text-negative';
    }

    // Update sub metrics
    startEl.textContent = formatCurrency(startVal);
    endEl.textContent = formatCurrency(endVal);
    depositsEl.textContent = formatCurrency(netDeposits);

    if (dividendsEl) {
        const signDiv = periodDividends > 0 ? '+' : '';
        dividendsEl.textContent = `${signDiv}${formatCurrency(periodDividends)}`;
    }
    if (taxesEl) {
        const signTax = periodTaxes > 0 ? '-' : '';
        taxesEl.textContent = `${signTax}${formatCurrency(periodTaxes)}`;
    }
    if (feesEl) {
        const signFee = periodFees > 0 ? '-' : '';
        feesEl.textContent = `${signFee}${formatCurrency(periodFees)}`;
    }

    startLbl.textContent = `Startværdi (${startDate})`;
    endLbl.textContent = `Slutværdi (${endDate})`;

    if (badge) {
        badge.textContent = growthRangeFilter.toUpperCase();
    }
}

// Render account selection toggle buttons dynamically on the dashboard
function renderAccountToggles() {
    const container = document.getElementById('accounts-toggle-container');
    if (!container) return;

    container.innerHTML = '';

    availableAccounts.forEach(acc => {
        const isSelected = selectedAccounts.includes(acc);
        const btn = document.createElement('button');
        
        btn.className = isSelected ? 'btn btn-primary btn-sm' : 'btn btn-danger btn-sm';
        
        // Override inactive styling for toggles
        if (!isSelected) {
            btn.style.backgroundColor = 'transparent';
            btn.style.color = 'var(--text-muted)';
            btn.style.border = '1px solid var(--border-color)';
        } else {
            btn.style.backgroundColor = 'var(--primary)';
            btn.style.color = '#0f172a';
            btn.style.border = '1px solid var(--primary)';
        }

        btn.style.padding = '0.35rem 0.85rem';
        btn.style.fontSize = '0.8rem';
        btn.style.borderRadius = '0.375rem';
        btn.style.cursor = 'pointer';
        btn.style.fontWeight = '600';
        btn.style.transition = 'var(--transition)';
        
        // Friendly labeling directly from persistent database map
        const friendlyName = (portfolioData.db && portfolioData.db.account_names && portfolioData.db.account_names[acc]) 
            ? portfolioData.db.account_names[acc] 
            : `Depot ${acc}`;

        btn.textContent = friendlyName;
        btn.onclick = () => toggleAccount(acc);
        container.appendChild(btn);
    });
}

// Toggle individual account visibility and trigger dynamic calculations
function toggleAccount(acc) {
    const isSelected = selectedAccounts.includes(acc);

    if (isSelected) {
        if (selectedAccounts.length <= 1) {
            alert('Mindst én konto skal forblive valgt for at kunne visualisere data.');
            return;
        }
        selectedAccounts = selectedAccounts.filter(v => v !== acc);
    } else {
        selectedAccounts.push(acc);
    }

    renderAccountToggles();
    loadPortfolioData();
}

// Scan CSV client-side on upload, and prompt for friendly names of any brand-new accounts
async function checkAndPromptNewAccounts(file) {
    return new Promise((resolve) => {
        const bomReader = new FileReader();
        bomReader.onload = function(e1) {
            const arr = new Uint8Array(e1.target.result);
            let encoding = 'utf-8';
            if (arr.length >= 2) {
                if (arr[0] === 0xFF && arr[1] === 0xFE) {
                    encoding = 'utf-16le';
                } else if (arr[0] === 0xFE && arr[1] === 0xFF) {
                    encoding = 'utf-16be';
                }
            }

            // Read the file as text using the detected encoding
            const textReader = new FileReader();
            textReader.onload = async function(e2) {
                const text = e2.target.result;
                const lines = text.split('\n');
                const fileAccounts = new Set();
                
                // Collect unique account numbers from column index 4 (Depot)
                for (let i = 1; i < lines.length; i++) {
                    if (!lines[i]) continue;
                    const cols = lines[i].split('\t');
                    if (cols.length > 4) {
                        const acc = cols[4].trim();
                        if (acc && acc !== "Depot") {
                            fileAccounts.add(acc);
                        }
                    }
                }

                // Find any accounts in the file that are not yet registered
                const newAccounts = [...fileAccounts].filter(acc => !availableAccounts.includes(acc));
                if (newAccounts.length > 0) {
                    const newNames = {};
                    for (const acc of newAccounts) {
                        // Pre-suggest nice defaults for common test depots
                        const defaultLabel = acc === '12345678' ? 'Aktiesparekonto (ASK)' : (acc === '87654321' ? 'Investeringsdepot' : `Depot ${acc}`);
                        const promptName = prompt(`Fandt et nyt kontodepot i din CSV: "${acc}".\n\nIndtast et beskrivende navn for denne konto:`, defaultLabel);
                        newNames[acc] = (promptName && promptName.trim() !== '') ? promptName.trim() : acc;
                    }

                    // Pre-save these friendly names to the database immediately
                    try {
                        await fetch('/api/metadata', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ account_names: newNames })
                        });
                    } catch (err) {
                        console.error('Failed to pre-save account names:', err);
                    }
                }
                resolve();
            };
            textReader.onerror = () => resolve();
            textReader.readAsText(file, encoding);
        };
        bomReader.onerror = () => resolve();
        bomReader.readAsArrayBuffer(file.slice(0, 4)); // Read first 4 bytes to check BOM
    });
}

// Handle change of the growth representation metric dropdown
function changeGrowthMetric() {
    const select = document.getElementById('growth-metric-select');
    if (!select) return;
    growthMetric = select.value;
    localStorage.setItem('growthMetric', growthMetric);
    renderCharts();
}

// Fetch live currency rates from backend API proxy (Adblocker-proof and same-origin)
async function fetchLiveRates(e) {
    if (e) e.preventDefault();

    const btn = document.getElementById('btn-fetch-rates');
    const statusEl = document.getElementById('metadata-status');
    
    if (btn) {
        btn.disabled = true;
        btn.textContent = 'Henter...';
    }

    try {
        const response = await fetch('/api/live-rates');
        if (!response.ok) throw new Error('Live FX-kurs endpoint returnerede en fejl');

        const data = await response.json();
        if (!data || !data.success || !data.rates) throw new Error('Ugyldig svar-struktur');

        const rates = data.rates;
        let updatedCount = 0;

        // Loop over each input element present in the DOM
        const rateInputs = document.querySelectorAll('[id^="rate-"]');
        rateInputs.forEach(input => {
            const currency = input.id.replace('rate-', '');
            if (currency === 'DKK') return;

            if (rates[currency]) {
                const rateToDKK = rates[currency];
                input.value = rateToDKK.toFixed(4);
                
                // Add a subtle green glow flash as visual feedback
                input.style.transition = 'box-shadow 0.3s ease, border-color 0.3s ease';
                input.style.borderColor = '#10b981';
                input.style.boxShadow = '0 0 8px rgba(16, 185, 129, 0.4)';
                
                setTimeout(() => {
                    input.style.borderColor = '';
                    input.style.boxShadow = '';
                }, 1500);

                updatedCount++;
            }
        });

        // Update the differences badges
        updateRateDiffBadges();
        updateApiStatusIndicator(true);

        if (statusEl) {
            statusEl.textContent = `Hentede succesfuldt ${updatedCount} live-valutakurser! Gennemse og klik på "Gem ændringer".`;
            statusEl.className = 'alert-inline text-positive';
            statusEl.classList.remove('hidden');
            setTimeout(() => {
                statusEl.classList.add('hidden');
            }, 5000);
        }

    } catch (err) {
        console.error('Failed to fetch live FX rates:', err);
        updateApiStatusIndicator(false);
        if (statusEl) {
            statusEl.textContent = 'Kunne ikke hente live-kurser. Opdater venligst manuelt.';
            statusEl.className = 'alert-inline text-negative';
            statusEl.classList.remove('hidden');
            setTimeout(() => {
                statusEl.classList.add('hidden');
            }, 4000);
        }
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.textContent = 'Hent live-kurser';
        }
    }
}

// Automatically fetch and save live currency rates in the background (if setting is enabled on load)
async function fetchAndAutoSaveLiveRates() {
    updateApiStatusIndicator(true, true); // Show "Updating..." state
    try {
        const response = await fetch('/api/live-rates');
        if (!response.ok) throw new Error('API offline');
        const data = await response.json();
        if (!data || !data.success || !data.rates) {
            updateApiStatusIndicator(false, false);
            return;
        }

        const rates = data.rates;
        const cachedRates = portfolioData.db.exchange_rates || {};
        let changed = false;
        const updatedRates = { ...cachedRates };

        Object.keys(cachedRates).forEach(curr => {
            if (curr === 'DKK') return;
            if (rates[curr]) {
                const liveRate = rates[curr];
                // If the difference is more than 0.0001, treat as changed
                if (Math.abs(updatedRates[curr] - liveRate) > 0.0001) {
                    updatedRates[curr] = parseFloat(liveRate.toFixed(4));
                    changed = true;
                }
            }
        });

        if (changed) {
            // Silently save back to backend to keep calculations exact
            await fetch('/api/metadata', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    exchange_rates: updatedRates,
                    auto_fetch_rates: true,
                    classifications: portfolioData.db.classifications || {},
                    asset_names: portfolioData.db.asset_names || {},
                    manual_prices: portfolioData.db.manual_prices || {},
                    manual_currencies: portfolioData.db.manual_currencies || {},
                    account_names: portfolioData.db.account_names || {}
                })
            });

            // Re-fetch portfolio data to reflect live rates instantly
            const responseReload = await fetch(`/api/portfolio?accounts=${selectedAccounts.join(',')}`);
            if (responseReload.ok) {
                portfolioData = await responseReload.json();
                
                // Trigger dynamic re-rendering of all dashboard metrics and charts instantly!
                updateKPICards(portfolioData.analysis);
                renderHoldingsTable(portfolioData.analysis.assets);
                renderCharts();
            }
        }
        updateApiStatusIndicator(true, false); // Set back to "Live"
    } catch (err) {
        console.error('Auto-fetch FX rates failed:', err);
        updateApiStatusIndicator(false, false); // Set to "Error"
    }
}

// Update API connectivity green/red glowing status indicator dot
function updateApiStatusIndicator(isOnline, isLoading) {
    const dot = document.getElementById('api-status-dot');
    const text = document.getElementById('api-status-text');
    
    const isFXDisabled = portfolioData && portfolioData.db_summary && portfolioData.db_summary.disable_fx_api;

    if (dot && text) {
        if (isFXDisabled) {
            dot.style.backgroundColor = '#64748b';
            dot.style.boxShadow = '0 0 8px #64748b';
            dot.title = 'API-forbindelse: Deaktiveret (offline-tilstand)';
            text.textContent = 'API-forbindelse: Deaktiveret (offline-tilstand)';
        } else if (isLoading) {
            dot.style.backgroundColor = '#eab308';
            dot.style.boxShadow = '0 0 8px #eab308';
            dot.title = 'API-forbindelse: Henter seneste valutakurser...';
            text.textContent = 'API-forbindelse: Opdaterer...';
        } else if (isOnline) {
            dot.style.backgroundColor = '#10b981';
            dot.style.boxShadow = '0 0 8px #10b981';
            dot.title = 'API-forbindelse: Online';
            text.textContent = 'API-forbindelse: Online';
        } else {
            dot.style.backgroundColor = '#ef4444';
            dot.style.boxShadow = '0 0 8px #ef4444';
            dot.title = 'API-forbindelse: Offline/fejl';
            text.textContent = 'API-forbindelse: Offline/fejl';
        }
    }

    // Update header-level API status badge
    const hBadge = document.getElementById('header-api-badge');
    const hDot = document.getElementById('header-api-status-dot');
    const hText = document.getElementById('header-api-status-text');
    
    const autoFetchEnabled = !isFXDisabled && portfolioData && portfolioData.db_summary && portfolioData.db_summary.auto_fetch_rates;

    if (hBadge) {
        hBadge.style.display = autoFetchEnabled ? 'flex' : 'none';
    }

    if (hDot && hText && autoFetchEnabled) {
        if (isLoading) {
            hDot.style.backgroundColor = '#eab308';
            hDot.style.boxShadow = '0 0 6px #eab308';
            hText.textContent = 'FX-kurser: Opdaterer';
            hText.style.color = '#fde047';
        } else if (isOnline) {
            hDot.style.backgroundColor = '#10b981';
            hDot.style.boxShadow = '0 0 6px #10b981';
            hText.textContent = 'FX-kurser: Live';
            hText.style.color = 'var(--text-primary)';
        } else {
            hDot.style.backgroundColor = '#ef4444';
            hDot.style.boxShadow = '0 0 6px #ef4444';
            hText.textContent = 'FX-kurser: Fejl';
            hText.style.color = '#f87171';
        }
    }
}

// Calculate and show differences between currently entered rates and the saved/cached database rates
function updateRateDiffBadges() {
    if (!portfolioData || !portfolioData.db) return;
    const savedRates = portfolioData.db.exchange_rates || {};

    Object.keys(savedRates).forEach(curr => {
        if (curr === 'DKK') return;
        const input = document.getElementById(`rate-${curr}`);
        if (!input) return;

        const currentVal = parseFloat(input.value);
        const savedVal = savedRates[curr];
        const badgeId = `rate-diff-${curr}`;
        let badge = document.getElementById(badgeId);

        // If no badge wrapper exists yet, create it dynamically
        if (!badge) {
            badge = document.createElement('span');
            badge.id = badgeId;
            badge.style.fontSize = '0.75rem';
            badge.style.fontWeight = '600';
            badge.style.marginLeft = '0.75rem';
            badge.style.verticalAlign = 'middle';
            input.parentNode.appendChild(badge);
        }

        if (isNaN(currentVal) || currentVal === savedVal) {
            badge.textContent = '';
        } else {
            const diff = currentVal - savedVal;
            const pct = (diff / savedVal) * 100;
            const sign = pct >= 0 ? '+' : '';
            const icon = pct >= 0 ? '▲' : '▼';
            const color = pct >= 0 ? '#10b981' : '#ef4444';
            badge.textContent = `${icon} ${sign}${pct.toFixed(2)}% vs gemt`;
            badge.style.color = color;
        }
    });
}

// Handle checkbox toggle for auto-fetching setting on load
function toggleAutoFetchSetting() {
    const checkbox = document.getElementById('setting-auto-fetch');
    const warning = document.getElementById('warning-auto-fetch');
    if (!checkbox || !warning) return;
    warning.style.display = checkbox.checked ? 'block' : 'none';
}
