'use strict';

// Asset classes in a fixed order: each keeps its color slot in every view
const CLASS_ORDER = ['Security', 'ETF', 'Cash'];
const CLASS_LABELS = { Security: 'Aktier/fonde', ETF: "ETF'er", Cash: 'Kontanter' };
const MAX_BARS = 10;
const SELECTED_KEY = 'nordfolio.selectedAccounts';

const state = {
    tab: 'overview',
    portfolio: null,
    transactions: [],
    txRows: [],
    selected: loadSelection(),
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const $ = (id) => document.getElementById(id);

// el builds a DOM node. Children are appended as text, never parsed as HTML,
// so values from the CSV can't inject markup.
function el(tag, props = {}, ...children) {
    const node = document.createElement(tag);
    for (const [key, value] of Object.entries(props)) {
        if (value === undefined || value === null || value === false) continue;
        if (key === 'class') node.className = value;
        else if (key === 'dataset') Object.assign(node.dataset, value);
        else if (key === 'style') Object.assign(node.style, value);
        else if (key in node && typeof value !== 'string') node[key] = value;
        else node.setAttribute(key, value === true ? '' : value);
    }
    for (const child of children.flat()) {
        if (child === undefined || child === null || child === false) continue;
        node.append(child instanceof Node ? child : String(child));
    }
    return node;
}

const numberFormats = {};
function numberFormat(min, max) {
    const key = `${min}-${max}`;
    if (!numberFormats[key]) {
        numberFormats[key] = new Intl.NumberFormat('da-DK', { minimumFractionDigits: min, maximumFractionDigits: max });
    }
    return numberFormats[key];
}

function money(value, currency = 'DKK') {
    return `${numberFormat(2, 2).format(value)} ${currency || 'DKK'}`;
}

// Unit prices keep up to four decimals, e.g. 12,544 EUR
function price(value, currency = 'DKK') {
    return `${numberFormat(2, 4).format(value)} ${currency || 'DKK'}`;
}

function percent(value) {
    return `${numberFormat(1, 1).format(value)} %`;
}

function quantity(value) {
    return numberFormat(0, 4).format(value);
}

function formatDate(iso) {
    if (!iso) return '';
    const date = new Date(`${iso}T00:00:00`);
    return isNaN(date) ? iso : date.toLocaleDateString('da-DK', { day: 'numeric', month: 'short', year: 'numeric' });
}

function accountName(id) {
    const account = state.portfolio?.accounts.find((a) => a.id === id);
    return account?.name || id;
}

async function api(path, options) {
    const res = await fetch(path, options);
    const body = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`);
    return body;
}

function loadSelection() {
    try {
        const saved = JSON.parse(localStorage.getItem(SELECTED_KEY));
        return Array.isArray(saved) ? saved : [];
    } catch {
        return [];
    }
}

function saveSelection() {
    localStorage.setItem(SELECTED_KEY, JSON.stringify(state.selected));
}

function accountsQuery() {
    return state.selected.length ? `?accounts=${encodeURIComponent(state.selected.join(','))}` : '';
}

let statusTimer;
function showStatus(message, type = 'success') {
    const status = $('status');
    status.textContent = message;
    status.className = `alert alert-${type}`;
    clearTimeout(statusTimer);
    if (type === 'success') {
        statusTimer = setTimeout(() => status.classList.add('hidden'), 8000);
    }
}

// ---------------------------------------------------------------------------
// Data loading
// ---------------------------------------------------------------------------

async function refresh() {
    let portfolio = await api(`/api/portfolio${accountsQuery()}`);

    // Drop accounts that no longer exist; an empty selection means all accounts
    const ids = portfolio.accounts.map((a) => a.id);
    let selected = state.selected.filter((id) => ids.includes(id));
    if (selected.length === 0) selected = ids;
    if (selected.join(',') !== state.selected.join(',')) {
        state.selected = selected;
        saveSelection();
        portfolio = await api(`/api/portfolio${accountsQuery()}`);
    }

    state.portfolio = portfolio;
    state.transactions = await api(`/api/transactions${accountsQuery()}`);
    render();
}

function render() {
    const empty = state.portfolio.transaction_count === 0;
    $('upload-panel').classList.toggle('hidden', !empty);
    $('overview').classList.toggle('hidden', empty);
    $('filter-row').classList.toggle('hidden', empty || state.tab === 'assets');
    $('btn-reset').disabled = empty;

    renderAccountToggles();
    renderOverview();
    renderTransactions();
    if (state.tab === 'assets') renderAssetsEditor();
}

// ---------------------------------------------------------------------------
// Tabs and account filter
// ---------------------------------------------------------------------------

const TABS = ['overview', 'assets', 'transactions'];

function switchTab(tab) {
    if (!TABS.includes(tab)) tab = 'overview';
    state.tab = tab;
    history.replaceState(null, '', tab === 'overview' ? location.pathname : `#${tab}`);
    document.querySelectorAll('.tab-btn').forEach((btn) => btn.classList.toggle('active', btn.dataset.tab === tab));
    document.querySelectorAll('.tab-content').forEach((section) => section.classList.toggle('active', section.id === `tab-${tab}`));
    if (state.portfolio) render();
}

function renderAccountToggles() {
    const container = $('account-toggles');
    container.replaceChildren(...state.portfolio.accounts.map((account) => {
        const pressed = state.selected.includes(account.id);
        return el('button', {
            class: 'toggle',
            type: 'button',
            'aria-pressed': String(pressed),
            title: account.id,
            onclick: () => toggleAccount(account.id),
        }, account.name);
    }));

    const asOf = state.portfolio.allocation.as_of;
    $('as-of').textContent = asOf ? `Opgjort pr. ${formatDate(asOf)}` : '';
}

function toggleAccount(id) {
    if (state.selected.includes(id)) {
        if (state.selected.length === 1) {
            showStatus('Mindst én konto skal være valgt.', 'danger');
            return;
        }
        state.selected = state.selected.filter((s) => s !== id);
    } else {
        state.selected = [...state.selected, id];
    }
    saveSelection();
    refresh().catch((err) => showStatus(`Kunne ikke hente data: ${err.message}`, 'danger'));
}

// ---------------------------------------------------------------------------
// Overview
// ---------------------------------------------------------------------------

function renderOverview() {
    const a = state.portfolio.allocation;

    $('kpi-total').textContent = money(a.total_dkk);
    $('kpi-holdings').textContent = `${a.holdings.length} værdipapirer`;
    $('kpi-securities').textContent = money(a.securities_dkk);
    $('kpi-etfs').textContent = money(a.etfs_dkk);
    $('kpi-cash').textContent = money(a.cash_dkk);
    for (const slice of a.by_class) {
        const id = { Security: 'kpi-securities-pct', ETF: 'kpi-etfs-pct', Cash: 'kpi-cash-pct' }[slice.key];
        $(id).textContent = `${percent(slice.weight)} af porteføljen`;
    }

    renderClassBar(a);

    const holdings = a.holdings.map((h) => ({
        label: h.name || h.symbol || h.isin,
        sub: h.symbol && h.symbol !== h.name ? h.symbol : '',
        value: h.book_value_dkk,
        weight: h.weight,
    }));
    if (holdings.length > MAX_BARS) {
        const rest = holdings.splice(MAX_BARS - 1);
        holdings.push({
            label: `Øvrige (${rest.length})`,
            value: rest.reduce((sum, h) => sum + h.value, 0),
            weight: rest.reduce((sum, h) => sum + h.weight, 0),
        });
    }
    renderBars($('holdings-bars'), holdings, 'Ingen værdipapirer i de valgte konti.');

    renderBars($('account-bars'), a.by_account.map((s) => ({
        label: accountName(s.key),
        sub: accountName(s.key) !== s.key ? s.key : '',
        value: s.value_dkk,
        weight: s.weight,
    })));

    renderBars($('currency-bars'), a.by_currency.map((s) => ({
        label: s.key,
        value: s.value_dkk,
        weight: s.weight,
    })));

    renderHoldingsTable(a);
}

// A single stacked bar: part-to-whole for the three asset classes
function renderClassBar(a) {
    const slices = CLASS_ORDER.map((key) => a.by_class.find((s) => s.key === key) || { key, value_dkk: 0, weight: 0 });
    const positive = slices.filter((s) => s.value_dkk > 0);

    $('class-bar').replaceChildren(...positive.map((s) => {
        const tip = `${money(s.value_dkk)} · ${CLASS_LABELS[s.key]} (${percent(s.weight)})`;
        return el('div', {
            class: `stack-segment series-${s.key}`,
            style: { flexGrow: String(s.value_dkk) },
            tabindex: '0',
            'aria-label': tip,
            dataset: { tip },
        }, el('span', { class: 'segment-label' }, percent(s.weight)));
    }));

    // Only keep an inline label when it fits inside its segment
    requestAnimationFrame(() => {
        document.querySelectorAll('#class-bar .stack-segment').forEach((segment) => {
            const label = segment.firstChild;
            label.hidden = false;
            label.hidden = label.scrollWidth + 16 > segment.clientWidth;
        });
    });

    $('class-legend').replaceChildren(...slices.map((s) => el('li', {},
        el('span', { class: `swatch series-${s.key}` }),
        el('span', { class: 'legend-label' }, CLASS_LABELS[s.key]),
        el('span', { class: 'legend-value' }, money(s.value_dkk)),
        el('span', { class: 'legend-pct' }, percent(s.weight)),
    )));
}

// Horizontal bars in a single hue, sorted by value
function renderBars(container, rows, emptyMessage = 'Ingen data.') {
    if (rows.length === 0) {
        container.replaceChildren(el('p', { class: 'no-data-msg' }, emptyMessage));
        return;
    }
    const max = Math.max(...rows.map((r) => r.value), 0);

    container.replaceChildren(...rows.map((row) => {
        const width = max > 0 && row.value > 0 ? (row.value / max) * 100 : 0;
        const tip = `${money(row.value)} · ${row.label} (${percent(row.weight)})`;
        return el('div', { class: 'bar-row', tabindex: '0', 'aria-label': tip, dataset: { tip } },
            el('div', { class: 'bar-label' },
                el('span', { class: 'bar-name' }, row.label),
                row.sub ? el('span', { class: 'bar-sub' }, row.sub) : null,
            ),
            el('div', { class: 'bar-track' }, el('div', { class: 'bar-fill', style: { width: `${width}%` } })),
            el('div', { class: 'bar-value' }, money(row.value)),
            el('div', { class: 'bar-pct' }, percent(row.weight)),
        );
    }));
}

function classBadge(cls) {
    return el('span', { class: `asset-badge badge-${cls}` }, CLASS_LABELS[cls] || cls);
}

function renderHoldingsTable(a) {
    const rows = a.holdings.map((h) => el('tr', {},
        el('td', {},
            el('div', { class: 'cell-strong' }, h.name || h.symbol),
            el('div', { class: 'cell-sub' }, [h.symbol, h.isin].filter(Boolean).join(' · ')),
        ),
        el('td', {}, classBadge(h.class)),
        el('td', {}, h.accounts.map(accountName).join(', ')),
        el('td', { class: 'text-right num' }, quantity(h.quantity)),
        el('td', { class: 'text-right num' }, price(h.avg_cost, h.currency)),
        el('td', { class: 'text-right num' }, money(h.book_value_dkk)),
        el('td', { class: 'text-right num' }, percent(h.weight)),
    ));

    const cash = a.by_class.find((s) => s.key === 'Cash');
    if (cash && cash.value_dkk !== 0) {
        rows.push(el('tr', { class: 'row-cash' },
            el('td', {}, el('div', { class: 'cell-strong' }, 'Kontanter')),
            el('td', {}, classBadge('Cash')),
            el('td', {}, state.selected.map(accountName).join(', ')),
            el('td', {}), el('td', {}),
            el('td', { class: 'text-right num' }, money(cash.value_dkk)),
            el('td', { class: 'text-right num' }, percent(cash.weight)),
        ));
    }

    if (rows.length === 0) {
        rows.push(el('tr', {}, el('td', { colspan: '7', class: 'no-data-msg' }, 'Ingen beholdninger i de valgte konti.')));
    }
    document.querySelector('#holdings-table tbody').replaceChildren(...rows);
}

// ---------------------------------------------------------------------------
// Assets & accounts editor
// ---------------------------------------------------------------------------

function renderAssetsEditor() {
    const { assets, accounts } = state.portfolio;

    const rows = assets.map((asset) => el('tr', { dataset: { isin: asset.isin } },
        el('td', {}, el('code', {}, asset.isin)),
        el('td', {}, el('input', { type: 'text', name: 'symbol', value: asset.symbol, placeholder: 'Ticker', 'aria-label': `Ticker for ${asset.isin}` })),
        el('td', {}, el('input', { type: 'text', name: 'name', value: asset.name, placeholder: asset.symbol, 'aria-label': `Navn for ${asset.isin}` })),
        el('td', {}, el('select', { name: 'class', 'aria-label': `Type for ${asset.isin}` },
            ['Security', 'ETF'].map((cls) => el('option', { value: cls, selected: asset.class === cls }, CLASS_LABELS[cls])),
        )),
    ));
    if (rows.length === 0) {
        rows.push(el('tr', {}, el('td', { colspan: '4', class: 'no-data-msg' }, 'Importer en CSV-fil for at se dine værdipapirer.')));
    }
    document.querySelector('#assets-table tbody').replaceChildren(...rows);

    $('account-names').replaceChildren(...accounts.map((account) => accountField(account.id, account.name)));
}

function accountField(id, name) {
    return el('label', { class: 'field' },
        el('span', {}, `Konto ${id}`),
        el('input', { type: 'text', value: name, placeholder: id, dataset: { account: id } }),
    );
}

async function saveMetadata(event) {
    event.preventDefault();
    const update = { classifications: {}, asset_names: {}, asset_symbols: {}, account_names: {} };

    document.querySelectorAll('#assets-table tbody tr[data-isin]').forEach((row) => {
        const isin = row.dataset.isin;
        update.asset_symbols[isin] = row.querySelector('[name="symbol"]').value;
        update.asset_names[isin] = row.querySelector('[name="name"]').value;
        update.classifications[isin] = row.querySelector('[name="class"]').value;
    });
    document.querySelectorAll('#account-names input').forEach((input) => {
        update.account_names[input.dataset.account] = input.value;
    });

    const status = $('metadata-status');
    status.textContent = 'Gemmer...';
    status.className = 'alert-inline';
    try {
        await api('/api/metadata', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(update) });
        await refresh();
        status.textContent = 'Ændringerne er gemt.';
        status.className = 'alert-inline text-positive';
    } catch (err) {
        status.textContent = `Fejl: ${err.message}`;
        status.className = 'alert-inline text-negative';
    }
}

// ---------------------------------------------------------------------------
// Transactions
// ---------------------------------------------------------------------------

function renderTransactions() {
    const txs = state.transactions;

    const typeFilter = $('tx-type-filter');
    const current = typeFilter.value;
    const types = [...new Set(txs.map((tx) => tx.transaction_type).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'da'));
    typeFilter.replaceChildren(
        el('option', { value: '' }, 'Alle typer'),
        ...types.map((type) => el('option', { value: type, selected: type === current }, type)),
    );

    state.txRows = txs.map((tx) => {
        const priceCurrency = tx.acquisition_currency || tx.result_currency || tx.amount_currency;
        const row = el('tr', {},
            el('td', { class: 'nowrap' }, tx.booking_date),
            el('td', { title: tx.account }, accountName(tx.account)),
            el('td', {}, el('strong', {}, tx.transaction_type)),
            el('td', {}, tx.symbol || '–'),
            el('td', {}, tx.isin ? el('code', {}, tx.isin) : '–'),
            el('td', { class: 'text-right num' }, tx.quantity ? quantity(tx.quantity) : '–'),
            el('td', { class: 'text-right num' }, tx.price ? price(tx.price, priceCurrency) : '–'),
            el('td', { class: `text-right num ${tx.amount > 0 ? 'text-positive' : tx.amount < 0 ? 'text-negative' : ''}` },
                tx.amount ? money(tx.amount, tx.amount_currency) : '–'),
            el('td', { class: 'text-right num' }, tx.balance ? money(tx.balance, tx.amount_currency) : '–'),
            el('td', { class: 'cell-sub' }, tx.transaction_text),
        );
        return { row, type: tx.transaction_type, text: row.textContent.toLowerCase() };
    });

    const rows = state.txRows.map((r) => r.row);
    if (rows.length === 0) {
        rows.push(el('tr', {}, el('td', { colspan: '10', class: 'no-data-msg' }, 'Ingen transaktioner i de valgte konti.')));
    }
    document.querySelector('#transactions-table tbody').replaceChildren(...rows);
    filterTransactions();
}

function filterTransactions() {
    const query = $('tx-search').value.trim().toLowerCase();
    const type = $('tx-type-filter').value;
    let shown = 0;
    for (const r of state.txRows) {
        const visible = (!type || r.type === type) && (!query || r.text.includes(query));
        r.row.hidden = !visible;
        if (visible) shown++;
    }
    const total = state.txRows.length;
    $('tx-count').textContent = shown === total ? `${total} transaktioner` : `Viser ${shown} af ${total} transaktioner`;
}

// ---------------------------------------------------------------------------
// Import and reset
// ---------------------------------------------------------------------------

async function importFile(file) {
    if (!file) return;
    showStatus(`Importerer ${file.name}...`, 'info');

    const form = new FormData();
    form.append('file', file);
    try {
        const result = await api('/api/upload', { method: 'POST', body: form });
        if (result.new_accounts.length > 0) {
            if (state.selected.length > 0) {
                state.selected = [...state.selected, ...result.new_accounts];
                saveSelection();
            }
            await nameNewAccounts(result.new_accounts);
        }
        await refresh();
        showStatus(`${result.added} nye transaktioner importeret. ${result.skipped} var allerede indlæst.`);
    } catch (err) {
        showStatus(`Importen fejlede: ${err.message}`, 'danger');
    } finally {
        $('upload-form').reset();
        $('header-file-input').value = '';
        updateFileLabel();
    }
}

function nameNewAccounts(ids) {
    const dialog = $('accounts-dialog');
    $('new-account-fields').replaceChildren(...ids.map((id) => accountField(id, '')));

    return new Promise((resolve) => {
        dialog.addEventListener('close', async () => {
            if (dialog.returnValue === 'save') {
                const account_names = {};
                dialog.querySelectorAll('input[data-account]').forEach((input) => {
                    account_names[input.dataset.account] = input.value;
                });
                try {
                    await api('/api/metadata', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ account_names }) });
                } catch (err) {
                    showStatus(`Kunne ikke gemme kontonavne: ${err.message}`, 'danger');
                }
            }
            resolve();
        }, { once: true });
        dialog.showModal();
    });
}

async function resetDatabase() {
    if (!confirm('Vil du slette alle importerede transaktioner, navne og klassifikationer? Det kan ikke fortrydes.')) return;
    try {
        await api('/api/reset', { method: 'POST' });
        state.selected = [];
        saveSelection();
        $('status').classList.add('hidden');
        switchTab('overview');
        await refresh();
    } catch (err) {
        showStatus(`Kunne ikke nulstille databasen: ${err.message}`, 'danger');
    }
}

function updateFileLabel() {
    const file = $('upload-file-input').files[0];
    $('file-msg').textContent = file ? file.name : 'Vælg eller træk en CSV-fil hertil';
}

// ---------------------------------------------------------------------------
// Tooltip for chart marks: follows the pointer, or the focused mark
// ---------------------------------------------------------------------------

function setupTooltip() {
    const tooltip = $('tooltip');

    const show = (target, x, y) => {
        tooltip.textContent = target.dataset.tip;
        tooltip.hidden = false;
        const pad = 12;
        const { width, height } = tooltip.getBoundingClientRect();
        const left = Math.min(x + pad, window.innerWidth - width - pad);
        const top = y - height - pad < 0 ? y + pad : y - height - pad;
        tooltip.style.transform = `translate(${Math.max(pad, left)}px, ${top}px)`;
    };
    const hide = () => { tooltip.hidden = true; };

    document.addEventListener('pointermove', (e) => {
        const target = e.target.closest?.('[data-tip]');
        if (target) show(target, e.clientX, e.clientY);
        else hide();
    });
    const showFocused = () => {
        const target = document.activeElement?.closest?.('[data-tip]');
        if (!target) return hide();
        const rect = target.getBoundingClientRect();
        show(target, rect.left + rect.width / 2, rect.top);
    };
    document.addEventListener('focusin', showFocused);
    document.addEventListener('focusout', hide);
    // Focusing a mark can scroll it into view: keep the tooltip on it
    window.addEventListener('scroll', showFocused, { passive: true });
}

// ---------------------------------------------------------------------------
// Startup
// ---------------------------------------------------------------------------

document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('.tab-btn').forEach((btn) => btn.addEventListener('click', () => switchTab(btn.dataset.tab)));

    $('btn-import').addEventListener('click', () => $('header-file-input').click());
    $('header-file-input').addEventListener('change', (e) => importFile(e.target.files[0]));
    $('btn-reset').addEventListener('click', resetDatabase);

    $('upload-file-input').addEventListener('change', updateFileLabel);
    $('upload-form').addEventListener('submit', (e) => {
        e.preventDefault();
        importFile($('upload-file-input').files[0]);
    });
    const dropArea = $('drop-area');
    ['dragenter', 'dragover'].forEach((type) => dropArea.addEventListener(type, () => dropArea.classList.add('dragging')));
    ['dragleave', 'drop'].forEach((type) => dropArea.addEventListener(type, () => dropArea.classList.remove('dragging')));

    $('metadata-form').addEventListener('submit', saveMetadata);
    $('tx-search').addEventListener('input', filterTransactions);
    $('tx-type-filter').addEventListener('change', filterTransactions);

    setupTooltip();
    switchTab(location.hash.slice(1));
    refresh().catch((err) => showStatus(`Kunne ikke hente data: ${err.message}`, 'danger'));
});
