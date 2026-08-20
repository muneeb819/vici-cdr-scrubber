const API = '';
let mainChart = null, fraudChartInst = null;
let lastResults = null, lastMeta = null;
let currentPage = 1;
const PER_PAGE = 15;

document.addEventListener('DOMContentLoaded', () => {
  checkHealth();
  loadFeatures();
  setupUpload();
  setupTextareaWatcher();
  document.addEventListener('click', e => {
    const menu = document.getElementById('exportMenu');
    if (menu && menu.classList.contains('open') && !e.target.closest('.export-dropdown')) menu.classList.remove('open');
  });
});

function api(ep) { return `${API}/api/index?ep=${ep}`; }

function toast(msg, type = 'info') {
  const c = document.getElementById('toast-container');
  const t = document.createElement('div');
  t.className = `toast ${type}`;
  const icons = { success: '&#10003;', error: '&#10007;', info: '&#9432;', warning: '&#9888;' };
  t.innerHTML = `<span>${icons[type] || ''}</span><span>${msg}</span>`;
  c.appendChild(t);
  setTimeout(() => { t.classList.add('out'); setTimeout(() => t.remove(), 300); }, 3500);
}

function log(msg, type = 'info') {
  const el = document.getElementById('logOutput');
  el.innerHTML += `<div class="log-entry ${type}">[${new Date().toLocaleTimeString()}] ${msg}</div>`;
  el.scrollTop = el.scrollHeight;
}

function setStep(n) {
  document.querySelectorAll('.step').forEach(s => {
    const sn = parseInt(s.dataset.step);
    s.classList.remove('active', 'done');
    if (sn < n) s.classList.add('done');
    if (sn === n) s.classList.add('active');
  });
}

function setLoading(btn, on) {
  if (!btn) return;
  if (on) { btn.classList.add('loading'); btn.disabled = true; }
  else { btn.classList.remove('loading'); btn.disabled = false; }
}

function setupUpload() {
  const zone = document.getElementById('uploadZone');
  const input = document.getElementById('fileInput');
  zone.addEventListener('click', () => input.click());
  zone.addEventListener('dragover', e => { e.preventDefault(); zone.classList.add('dragover'); });
  zone.addEventListener('dragleave', () => zone.classList.remove('dragover'));
  zone.addEventListener('drop', e => { e.preventDefault(); zone.classList.remove('dragover'); handleFile(e.dataTransfer.files[0]); });
  input.addEventListener('change', e => handleFile(e.target.files[0]));
}

function setupTextareaWatcher() {
  const ta = document.getElementById('jsonInput');
  const cc = document.getElementById('charCount');
  ta.addEventListener('input', () => {
    cc.textContent = ta.value.length.toLocaleString() + ' chars';
  });
}

function handleFile(file) {
  if (!file) return;
  log('Reading: ' + file.name, 'info');
  const reader = new FileReader();
  reader.onload = e => {
    const text = e.target.result;
    if (file.name.endsWith('.json')) {
      try {
        const parsed = JSON.parse(text);
        if (parsed.cdrs) {
          document.getElementById('jsonInput').value = JSON.stringify(parsed, null, 2);
        } else if (Array.isArray(parsed)) {
          document.getElementById('jsonInput').value = JSON.stringify({ cdrs: parsed }, null, 2);
        } else {
          document.getElementById('jsonInput').value = JSON.stringify({ cdrs: [parsed] }, null, 2);
        }
        toast(`Loaded ${file.name} (${formatNum(countCdrs())} records)`, 'success');
        log('JSON loaded: ' + text.length + ' bytes', 'success');
        setStep(2);
      } catch (err) {
        toast('Invalid JSON file', 'error');
        log('JSON parse error: ' + err.message, 'error');
      }
    } else if (file.name.endsWith('.csv') || file.name.endsWith('.txt')) {
      const json = csvToJson(text);
      document.getElementById('jsonInput').value = JSON.stringify(json, null, 2);
      toast(`Loaded ${file.name} (${formatNum(json.cdrs.length)} records)`, 'success');
      log('CSV converted: ' + json.cdrs.length + ' records', 'success');
      setStep(2);
    } else {
      toast('Unsupported file type', 'error');
    }
    document.getElementById('charCount').textContent = document.getElementById('jsonInput').value.length.toLocaleString() + ' chars';
  };
  reader.readAsText(file);
}

function csvToJson(csv) {
  const lines = csv.trim().split(/\r?\n/);
  if (lines.length < 2) return { cdrs: [] };
  const headers = lines[0].split(',').map(h => h.trim().toLowerCase().replace(/['"]/g, ''));
  const cdrs = [];
  for (let i = 1; i < lines.length; i++) {
    const vals = lines[i].split(',');
    if (vals.length < 2) continue;
    const obj = {};
    headers.forEach((h, idx) => {
      let v = (vals[idx] || '').trim().replace(/^["']|["']$/g, '');
      if (v !== '' && !isNaN(v)) v = Number(v);
      obj[h] = v;
    });
    if (Object.keys(obj).length > 0) cdrs.push(obj);
  }
  return { cdrs };
}

function clearInput() {
  document.getElementById('jsonInput').value = '';
  document.getElementById('charCount').textContent = '0 chars';
  setStep(1);
}

function countCdrs() {
  try {
    const d = JSON.parse(document.getElementById('jsonInput').value);
    return d.cdrs ? d.cdrs.length : 0;
  } catch { return 0; }
}

function getInputData() {
  const raw = document.getElementById('jsonInput').value.trim();
  if (!raw) { toast('Please upload or paste CDR data first', 'warning'); return null; }
  try {
    let data = JSON.parse(raw);
    if (Array.isArray(data)) data = { cdrs: data };
    if (!data.cdrs || !Array.isArray(data.cdrs) || data.cdrs.length === 0) {
      toast('Expected { "cdrs": [...] } format', 'error');
      return null;
    }
    setStep(2);
    return data;
  } catch (e) {
    toast('JSON parse error: ' + e.message, 'error');
    return null;
  }
}

async function postApi(ep, data) {
  const res = await fetch(api(ep), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
  if (!res.ok) throw new Error(`Server error (${res.status})`);
  return res.json();
}

function setButtonsDisabled(disabled) {
  ['btnScrub', 'btnProfile', 'btnValidate'].forEach(id => {
    const btn = document.getElementById(id);
    if (btn) btn.disabled = disabled;
  });
}

// ── Scrub ──
async function processScrub() {
  const data = getInputData();
  if (!data) return;
  const btn = document.getElementById('btnScrub');
  setLoading(btn, true); setButtonsDisabled(true);
  log(`Scrubbing ${data.cdrs.length} records...`, 'info');
  try {
    const result = await postApi('scrub', data);
    lastResults = result.results || [];
    lastMeta = { type: 'scrub', processed: result.processed, stats: result.stats };
    toast(`Scrubbed ${result.processed} records successfully`, 'success');
    log(`Scrub complete: ${result.processed} processed`, 'success');
    setStep(3);
    renderScrubResults(result);
  } catch (e) { toast('Scrub failed: ' + e.message, 'error'); log('Scrub error: ' + e.message, 'error'); }
  setLoading(btn, false); setButtonsDisabled(false);
}

function renderScrubResults(r) {
  showStatsCard();
  let fraudCount = 0;
  (r.results || []).forEach(x => { if (x.fraud_score > 0.3) fraudCount++; });
  setStatValues(r.processed, r.stats?.valid_records || 0, r.stats?.scrubbed_records || 0, fraudCount);
  renderChart('doughnut', ['Valid', 'Scrubbed'], [r.stats?.valid_records || 0, r.stats?.scrubbed_records || 0], ['#10b981', '#ef4444']);
  renderFraudChart(r.results || []);

  const cols = ['unique_id', 'is_valid', 'fraud_score', 'validation_score', 'carrier_name', 'country', 'normalized_phone', 'scrub_reason'];
  const headers = ['ID', 'Status', 'Fraud', 'Validation', 'Carrier', 'Country', 'Phone', 'Reason'];
  const rows = (r.results || []).map(x => [
    x.unique_id,
    x.is_valid ? 'VALID' : 'INVALID',
    (x.fraud_score * 100).toFixed(0) + '%|' + x.fraud_score,
    (x.validation_score * 100).toFixed(0) + '%|' + x.validation_score,
    x.carrier_name || '-',
    x.country || '-',
    x.normalized_phone || '-',
    x.scrub_reason || '-'
  ]);
  buildTable(headers, rows, cols);
  document.getElementById('resultsCard').style.display = 'block';
  document.getElementById('resultsTitle').textContent = 'Scrub Results';
}

// ── Profile ──
async function processProfile() {
  const data = getInputData();
  if (!data) return;
  const btn = document.getElementById('btnProfile');
  setLoading(btn, true); setButtonsDisabled(true);
  log(`Profiling ${data.cdrs.length} records...`, 'info');
  try {
    const result = await postApi('profile', data);
    lastResults = result; lastMeta = { type: 'profile' };
    toast(`Profile complete: quality ${(result.quality_score || 0).toFixed(0)}%`, 'success');
    log(`Profile: quality score ${result.quality_score}%`, 'success');
    setStep(3);
    renderProfileResults(result);
  } catch (e) { toast('Profile failed: ' + e.message, 'error'); log('Profile error: ' + e.message, 'error'); }
  setLoading(btn, false); setButtonsDisabled(false);
}

function renderProfileResults(r) {
  showStatsCard();
  setStatValues(r.total_records || 0, (r.quality_score || 0).toFixed(0) + '%', (r.issues || []).length, '-');
  if (r.field_completeness) {
    renderChart('bar', Object.keys(r.field_completeness), Object.values(r.field_completeness),
      Object.values(r.field_completeness).map(v => v > 90 ? '#10b981' : v > 70 ? '#f59e0b' : '#ef4444'));
  }
  // Quality ring
  document.getElementById('qualityCard').style.display = 'block';
  const q = r.quality_score || 0;
  const arc = document.getElementById('qualityArc');
  arc.style.stroke = q > 80 ? '#10b981' : q > 50 ? '#f59e0b' : '#ef4444';
  setTimeout(() => { arc.style.strokeDashoffset = 314 - (314 * q / 100); }, 100);
  document.getElementById('qualityVal').textContent = q.toFixed(0) + '%';
  // Issues
  const il = document.getElementById('issuesList');
  il.innerHTML = '';
  (r.issues || []).forEach(i => {
    il.innerHTML += `<div class="issue-item"><span class="issue-dot ${(i.severity || 'low').toLowerCase()}"></span><span>${i.message || i.rule || 'Issue'}</span></div>`;
  });
  if ((r.issues || []).length > 0) document.getElementById('issuesCard').style.display = 'block';
  document.getElementById('resultsCard').style.display = 'none';
}

// ── Validate ──
async function processValidate() {
  const data = getInputData();
  if (!data) return;
  const btn = document.getElementById('btnValidate');
  setLoading(btn, true); setButtonsDisabled(true);
  log(`Validating ${data.cdrs.length} records...`, 'info');
  try {
    const result = await postApi('validate', data);
    lastResults = result.results || []; lastMeta = { type: 'validate', total: result.total, valid: result.valid, invalid: result.invalid, pass_rate: result.pass_rate, avg_score: result.avg_score };
    toast(`Validation: ${result.valid}/${result.total} passed`, 'success');
    log(`Validation: ${result.valid}/${result.total} passed (${result.pass_rate.toFixed(1)}%)`, 'success');
    setStep(3);
    renderValidateResults(result);
  } catch (e) { toast('Validation failed: ' + e.message, 'error'); log('Validate error: ' + e.message, 'error'); }
  setLoading(btn, false); setButtonsDisabled(false);
}

function renderValidateResults(r) {
  showStatsCard();
  setStatValues(r.total, r.valid, r.invalid, r.avg_score ? (r.avg_score * 100).toFixed(0) + '%' : '-');
  renderChart('doughnut', ['Valid', 'Invalid'], [r.valid, r.invalid], ['#10b981', '#ef4444']);

  const cols = ['unique_id', 'is_valid', 'score', 'errors', 'warnings'];
  const headers = ['ID', 'Status', 'Score', 'Errors', 'Warnings'];
  const rows = (r.results || []).map(x => [
    x.unique_id,
    x.is_valid ? 'VALID' : 'INVALID',
    (x.score * 100).toFixed(0) + '%|' + x.score,
    String(x.errors),
    String(x.warnings)
  ]);
  buildTable(headers, rows, cols);
  document.getElementById('resultsCard').style.display = 'block';
  document.getElementById('resultsTitle').textContent = 'Validation Results';
}

// ── Stats ──
function showStatsCard() {
  document.getElementById('statsCard').style.display = 'block';
  document.getElementById('chartCard').style.display = 'block';
  document.getElementById('fraudCard').style.display = 'block';
}
function setStatValues(t, v, i, f) {
  document.getElementById('statTotal').textContent = t;
  document.getElementById('statValid').textContent = v;
  document.getElementById('statInvalid').textContent = i;
  document.getElementById('statFraud').textContent = f;
}

// ── Charts ──
function renderChart(type, labels, data, colors) {
  const ctx = document.getElementById('mainChart').getContext('2d');
  if (mainChart) mainChart.destroy();
  mainChart = new Chart(ctx, {
    type, data: { labels, datasets: [{ data, backgroundColor: colors, borderWidth: 0, borderRadius: type === 'bar' ? 4 : 0 }] },
    options: {
      responsive: true, maintainAspectRatio: true,
      plugins: { legend: { display: type === 'doughnut', labels: { color: '#8b99b5', font: { size: 11 } } } },
      scales: type === 'bar' ? { x: { ticks: { color: '#8b99b5', font: { size: 10 } }, grid: { color: '#1e2a42' } }, y: { ticks: { color: '#8b99b5', font: { size: 10 } }, grid: { color: '#1e2a42' }, beginAtZero: true } } : undefined
    }
  });
}

function renderFraudChart(results) {
  const b = [0, 0, 0, 0, 0];
  results.forEach(r => { const s = r.fraud_score; s <= 0.1 ? b[0]++ : s <= 0.3 ? b[1]++ : s <= 0.5 ? b[2]++ : s <= 0.7 ? b[3]++ : b[4]++; });
  const ctx = document.getElementById('fraudChart').getContext('2d');
  if (fraudChartInst) fraudChartInst.destroy();
  fraudChartInst = new Chart(ctx, {
    type: 'bar', data: { labels: ['None', 'Low', 'Med', 'High', 'Critical'], datasets: [{ data: b, backgroundColor: ['#10b981', '#06b6d4', '#f59e0b', '#f97316', '#ef4444'], borderWidth: 0, borderRadius: 4 }] },
    options: {
      responsive: true, plugins: { legend: { display: false } },
      scales: { x: { ticks: { color: '#8b99b5', font: { size: 10 } }, grid: { color: '#1e2a42' } }, y: { ticks: { color: '#8b99b5', font: { size: 10 } }, grid: { color: '#1e2a42' }, beginAtZero: true } }
    }
  });
}

// ── Table ──
let tableData = [], tableHeaders = [], tableCols = [];
function buildTable(headers, rows, cols) {
  tableHeaders = headers; tableData = rows; tableCols = cols; currentPage = 1;
  renderTablePage();
}

function renderTablePage() {
  const start = (currentPage - 1) * PER_PAGE;
  const page = tableData.slice(start, start + PER_PAGE);
  const head = document.getElementById('resultsHead');
  const body = document.getElementById('resultsBody');
  head.innerHTML = '<tr>' + tableHeaders.map(h => `<th>${h}</th>`).join('') + '</tr>';
  body.innerHTML = '';
  page.forEach(row => {
    const tr = document.createElement('tr');
    row.forEach((cell, i) => {
      const td = document.createElement('td');
      if (typeof cell === 'string' && cell.includes('|') && (tableCols[i] === 'is_valid' || tableCols[i].includes('score'))) {
        const parts = cell.split('|');
        const pct = parseFloat(parts[0]);
        const val = parseFloat(parts[1]);
        const cls = val > 0.7 ? 'high' : val > 0.4 ? 'medium' : 'low';
        if (tableCols[i] === 'is_valid') {
          td.innerHTML = `<span class="status-pill ${parts[0] === 'VALID' ? 'valid' : 'invalid'}">${parts[0]}</span>`;
        } else {
          td.innerHTML = `${parts[0]}<span class="score-bar"><span class="score-fill ${cls}" style="width:${pct}%"></span></span>`;
        }
      } else if (cell === 'VALID' || cell === 'INVALID') {
        td.innerHTML = `<span class="status-pill ${cell.toLowerCase()}">${cell}</span>`;
      } else {
        td.textContent = cell;
      }
      tr.appendChild(td);
    });
    body.appendChild(tr);
  });
  document.getElementById('rowCount').textContent = `${formatNum(tableData.length)} records`;
  renderPagination();
}

function renderPagination() {
  const pages = Math.ceil(tableData.length / PER_PAGE);
  const pg = document.getElementById('pagination');
  pg.innerHTML = '';
  if (pages <= 1) return;
  for (let i = 1; i <= pages; i++) {
    const btn = document.createElement('button');
    btn.className = 'page-btn' + (i === currentPage ? ' active' : '');
    btn.textContent = i;
    btn.onclick = () => { currentPage = i; renderTablePage(); };
    pg.appendChild(btn);
  }
}

function filterTable() {
  const q = document.getElementById('searchInput').value.toLowerCase();
  if (!lastResults) return;
  if (lastMeta?.type === 'scrub') {
    const filtered = lastResults.filter(r => Object.values(r).some(v => String(v).toLowerCase().includes(q)));
    const rows = filtered.map(x => [x.unique_id, x.is_valid ? 'VALID' : 'INVALID', (x.fraud_score*100).toFixed(0)+'%|'+x.fraud_score, (x.validation_score*100).toFixed(0)+'%|'+x.validation_score, x.carrier_name||'-', x.country||'-', x.normalized_phone||'-', x.scrub_reason||'-']);
    tableData = rows;
  } else if (lastMeta?.type === 'validate') {
    const filtered = lastResults.filter(r => Object.values(r).some(v => String(v).toLowerCase().includes(q)));
    const rows = filtered.map(x => [x.unique_id, x.is_valid ? 'VALID' : 'INVALID', (x.score*100).toFixed(0)+'%|'+x.score, String(x.errors), String(x.warnings)]);
    tableData = rows;
  }
  currentPage = 1;
  renderTablePage();
}

// ── Export ──
function toggleExportMenu() {
  document.getElementById('exportMenu').classList.toggle('open');
}

function getExportFilename(ext) {
  const ts = new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-');
  return `cdr-results-${ts}.${ext}`;
}

function exportAs(fmt) {
  document.getElementById('exportMenu').classList.remove('open');
  if (!lastResults && !lastMeta) { toast('No results to export', 'warning'); return; }

  if (lastMeta?.type === 'profile') {
    exportProfile(fmt);
    return;
  }

  const headers = getExportHeaders();
  const rows = getExportRows();

  switch (fmt) {
    case 'csv': exportCSV(headers, rows); break;
    case 'json': exportJSON(); break;
    case 'xlsx': exportXLSX(headers, rows); break;
    case 'pdf': exportPDF(headers, rows); break;
    case 'txt': exportTXT(headers, rows); break;
  }
}

function getExportHeaders() {
  if (lastMeta?.type === 'scrub') return ['Unique ID', 'Valid', 'Scrub Reason', 'Fraud Score', 'Validation Score', 'Carrier', 'Country', 'Normalized Phone'];
  if (lastMeta?.type === 'validate') return ['Unique ID', 'Valid', 'Score', 'Errors', 'Warnings'];
  return [];
}

function getExportRows() {
  if (lastMeta?.type === 'scrub') return (lastResults || []).map(r => [r.unique_id, r.is_valid, r.scrub_reason || '', r.fraud_score, r.validation_score, r.carrier_name || '', r.country || '', r.normalized_phone || '']);
  if (lastMeta?.type === 'validate') return (lastResults || []).map(r => [r.unique_id, r.is_valid, r.score, r.errors, r.warnings]);
  return [];
}

function exportCSV(headers, rows) {
  const csv = [headers.join(','), ...rows.map(r => r.map(v => `"${String(v).replace(/"/g, '""')}"`).join(','))].join('\n');
  download(csv, getExportFilename('csv'), 'text/csv');
  toast('Exported as CSV', 'success');
}

function exportJSON() {
  let data;
  if (lastMeta?.type === 'scrub') data = { export_time: new Date().toISOString(), meta: lastMeta, results: lastResults };
  else if (lastMeta?.type === 'validate') data = { export_time: new Date().toISOString(), meta: lastMeta, results: lastResults };
  else data = lastResults;
  download(JSON.stringify(data, null, 2), getExportFilename('json'), 'application/json');
  toast('Exported as JSON', 'success');
}

function exportXLSX(headers, rows) {
  const ws = XLSX.utils.aoa_to_sheet([headers, ...rows]);
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, 'Results');
  if (lastMeta) {
    const metaWs = XLSX.utils.aoa_to_sheet([['Type', lastMeta.type], ['Processed', lastMeta.processed || lastMeta.total || ''], ['Time', new Date().toISOString()]]);
    XLSX.utils.book_append_sheet(wb, metaWs, 'Summary');
  }
  XLSX.writeFile(wb, getExportFilename('xlsx'));
  toast('Exported as Excel', 'success');
}

function exportPDF(headers, rows) {
  const { jsPDF } = window.jspdf;
  const doc = new jsPDF({ orientation: rows[0] && rows[0].length > 5 ? 'landscape' : 'portrait' });
  doc.setFontSize(16);
  doc.text('CDR Scrub Results', 14, 20);
  doc.setFontSize(10);
  doc.setTextColor(100);
  doc.text(`Generated: ${new Date().toLocaleString()} | Records: ${rows.length}`, 14, 28);
  doc.autoTable({ head: [headers], body: rows, startY: 34, styles: { fontSize: 8 }, headStyles: { fillColor: [99, 102, 241] }, alternateRowStyles: { fillColor: [245, 247, 250] } });
  doc.save(getExportFilename('pdf'));
  toast('Exported as PDF', 'success');
}

function exportTXT(headers, rows) {
  const sep = '\t';
  const txt = [headers.join(sep), ...rows.map(r => r.map(v => String(v)).join(sep))].join('\n');
  download(txt, getExportFilename('txt'), 'text/plain');
  toast('Exported as Text', 'success');
}

function exportProfile(fmt) {
  const r = lastResults;
  if (fmt === 'json') {
    download(JSON.stringify(r, null, 2), getExportFilename('json'), 'application/json');
  } else if (fmt === 'csv') {
    const rows = [['Total Records', r.total_records], ['Quality Score', r.quality_score], ['Issues Found', (r.issues || []).length]];
    if (r.field_completeness) {
      Object.entries(r.field_completeness).forEach(([k, v]) => rows.push([k, v.toFixed(1) + '%']));
    }
    const csv = [['Metric', 'Value'], ...rows.map(r => r.join(','))].join('\n');
    download(csv, getExportFilename('csv'), 'text/csv');
  } else if (fmt === 'xlsx') {
    const rows = [['Total Records', r.total_records], ['Quality Score', r.quality_score]];
    if (r.field_completeness) Object.entries(r.field_completeness).forEach(([k, v]) => rows.push([k, v.toFixed(1) + '%']));
    const ws = XLSX.utils.aoa_to_sheet([['Metric', 'Value'], ...rows]);
    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, 'Profile');
    XLSX.writeFile(wb, getExportFilename('xlsx'));
  } else if (fmt === 'pdf') {
    const { jsPDF } = window.jspdf;
    const doc = new jsPDF();
    doc.setFontSize(16); doc.text('CDR Profile Report', 14, 20);
    doc.setFontSize(10); doc.text(`Quality Score: ${r.quality_score}%  |  Total Records: ${r.total_records}`, 14, 28);
    if (r.field_completeness) {
      doc.autoTable({ head: [['Field', 'Completeness']], body: Object.entries(r.field_completeness).map(([k, v]) => [k, v.toFixed(1) + '%']), startY: 36, headStyles: { fillColor: [99, 102, 241] } });
    }
    doc.save(getExportFilename('pdf'));
  } else {
    const lines = [`CDR Profile Report`, `Total Records: ${r.total_records}`, `Quality Score: ${r.quality_score}%`, ``];
    if (r.field_completeness) Object.entries(r.field_completeness).forEach(([k, v]) => lines.push(`${k}: ${v.toFixed(1)}%`));
    download(lines.join('\n'), getExportFilename('txt'), 'text/plain');
  }
  toast(`Exported as ${fmt.toUpperCase()}`, 'success');
}

function download(content, filename, mime) {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url; a.download = filename; a.click();
  URL.revokeObjectURL(url);
}

function formatNum(n) { return Number(n).toLocaleString(); }

// ── Demo ──
function loadDemo() {
  const demo = { cdrs: [
    { unique_id: "CDR001", caller_code: "1001", customer_phone_number: "+12025551234", phone_code: "1", campaign_id: "CAMP01", status: "ANSWERED", call_duration: 120 },
    { unique_id: "CDR002", caller_code: "1002", customer_phone_number: "+44207946001", phone_code: "44", campaign_id: "CAMP01", status: "ANSWERED", call_duration: 45 },
    { unique_id: "CDR003", caller_code: "1001", customer_phone_number: "+12025555678", phone_code: "1", campaign_id: "CAMP01", status: "NO ANSWER", call_duration: 0 },
    { unique_id: "CDR004", caller_code: "1003", customer_phone_number: "+912212345678", phone_code: "91", campaign_id: "CAMP02", status: "ANSWERED", call_duration: 300 },
    { unique_id: "CDR005", caller_code: "1001", customer_phone_number: "+12025559999", phone_code: "1", campaign_id: "CAMP01", status: "BUSY", call_duration: 0 },
    { unique_id: "CDR006", caller_code: "1004", customer_phone_number: "+61298765432", phone_code: "61", campaign_id: "CAMP02", status: "ANSWERED", call_duration: 180 },
    { unique_id: "CDR007", caller_code: "1001", customer_phone_number: "+12025551111", phone_code: "1", campaign_id: "CAMP01", status: "ANSWERED", call_duration: 2 },
    { unique_id: "CDR008", caller_code: "1005", customer_phone_number: "+81312345678", phone_code: "81", campaign_id: "CAMP03", status: "ANSWERED", call_duration: 90 },
    { unique_id: "CDR009", caller_code: "1001", customer_phone_number: "+12025552222", phone_code: "1", campaign_id: "CAMP01", status: "ANSWERED", call_duration: 1 },
    { unique_id: "CDR010", caller_code: "1006", customer_phone_number: "+5511987654321", phone_code: "55", campaign_id: "CAMP03", status: "FAILED", call_duration: 0 }
  ]};
  document.getElementById('jsonInput').value = JSON.stringify(demo, null, 2);
  document.getElementById('charCount').textContent = document.getElementById('jsonInput').value.length.toLocaleString() + ' chars';
  toast('Demo loaded: 10 CDR records', 'success');
  log('Demo loaded: 10 CDR records', 'success');
  setStep(2);
}

// ── Health / Features ──
async function checkHealth() {
  try {
    const res = await fetch(api('health'));
    if (res.ok) {
      const d = await res.json();
      document.getElementById('navStatus').innerHTML = `<span class="status-pulse online"></span><span>Connected (${d.platform})</span>`;
      log('Health: ' + d.status, 'success');
      return;
    }
  } catch {}
  try {
    const res = await fetch(api('index'));
    if (res.ok) {
      document.getElementById('navStatus').innerHTML = `<span class="status-pulse online"></span><span>Connected</span>`;
      log('API online', 'success');
    }
  } catch {
    document.getElementById('navStatus').innerHTML = `<span class="status-pulse offline"></span><span>Disconnected</span>`;
    log('Health check failed', 'error');
  }
}

async function loadFeatures() {
  try {
    let res = await fetch(api('version'));
    if (!res.ok) res = await fetch(api('index'));
    const d = await res.json();
    if (d.features) {
      const c = document.getElementById('featureTags');
      d.features.forEach(f => {
        const t = document.createElement('span');
        t.className = 'feature-tag';
        t.textContent = f;
        c.appendChild(t);
      });
    }
  } catch {}
}

function toggleTheme() {
  toast('Dark mode is the only mode', 'info');
}
