const API_BASE = '';
let validationChart = null;
let fraudChart = null;

function api(endpoint, method) {
  if (method === 'GET') return `${API_BASE}/api/index?ep=${endpoint}`;
  return `${API_BASE}/api/index?ep=${endpoint}`;
}

document.addEventListener('DOMContentLoaded', () => {
  checkHealth();
  loadFeatures();
  setupUpload();
});

async function checkHealth() {
  try {
    const res = await fetch(api('health'));
    if (res.ok) {
      const data = await res.json();
      document.getElementById('navStatus').innerHTML = `<span class="status-dot online"></span><span>Connected (${data.platform})</span>`;
      log('Health check: ' + data.status, 'success');
      return;
    }
  } catch (e) {}
  try {
    const res = await fetch(api('index'));
    if (res.ok) {
      document.getElementById('navStatus').innerHTML = `<span class="status-dot online"></span><span>Connected (Vercel)</span>`;
      log('API online', 'success');
    }
  } catch (e) {
    document.getElementById('navStatus').innerHTML = `<span class="status-dot offline"></span><span>Disconnected</span>`;
    log('Health check failed', 'error');
  }
}

async function loadFeatures() {
  try {
    let res = await fetch(api('version'));
    if (!res.ok) res = await fetch(api('index'));
    const data = await res.json();
    if (data.features) {
      const container = document.getElementById('featureTags');
      data.features.forEach(f => {
        const tag = document.createElement('span');
        tag.className = 'feature-tag';
        tag.textContent = f;
        container.appendChild(tag);
      });
    }
  } catch (e) {}
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

function handleFile(file) {
  if (!file) return;
  log('Reading file: ' + file.name, 'info');
  const reader = new FileReader();
  reader.onload = e => {
    const text = e.target.result;
    if (file.name.endsWith('.json')) {
      document.getElementById('jsonInput').value = text;
      log('JSON loaded: ' + text.length + ' bytes', 'success');
    } else if (file.name.endsWith('.csv')) {
      const json = csvToJson(text);
      document.getElementById('jsonInput').value = JSON.stringify(json, null, 2);
      log('CSV converted: ' + json.cdrs.length + ' records', 'success');
    }
  };
  reader.readAsText(file);
}

function csvToJson(csv) {
  const lines = csv.trim().split('\n');
  if (lines.length < 2) return { cdrs: [] };
  const headers = lines[0].split(',').map(h => h.trim().toLowerCase());
  const cdrs = [];
  for (let i = 1; i < lines.length; i++) {
    const values = lines[i].split(',');
    const obj = {};
    headers.forEach((h, idx) => { let v = values[idx] ? values[idx].trim() : ''; if (!isNaN(v) && v !== '') v = Number(v); obj[h] = v; });
    cdrs.push(obj);
  }
  return { cdrs };
}

function getInputData() {
  const raw = document.getElementById('jsonInput').value.trim();
  if (!raw) { log('No input data', 'error'); return null; }
  try {
    const data = JSON.parse(raw);
    if (!data.cdrs || !Array.isArray(data.cdrs)) { log('Expected {"cdrs": [...]}', 'error'); return null; }
    return data;
  } catch (e) { log('JSON parse error: ' + e.message, 'error'); return null; }
}

async function postApi(endpoint, data) {
  const res = await fetch(api(endpoint), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
  return res.json();
}

async function processScrub() {
  const data = getInputData();
  if (!data) return;
  log(`Scrubbing ${data.cdrs.length} records...`, 'info');
  try {
    const result = await postApi('scrub', data);
    log(`Scrub complete: ${result.processed} processed`, 'success');
    displayScrubResults(result);
  } catch (e) { log('Scrub error: ' + e.message, 'error'); }
}

async function processProfile() {
  const data = getInputData();
  if (!data) return;
  log(`Profiling ${data.cdrs.length} records...`, 'info');
  try {
    const result = await postApi('profile', data);
    log(`Profile: quality score ${result.quality_score}%`, 'success');
    displayProfileResults(result);
  } catch (e) { log('Profile error: ' + e.message, 'error'); }
}

async function processValidate() {
  const data = getInputData();
  if (!data) return;
  log(`Validating ${data.cdrs.length} records...`, 'info');
  try {
    const result = await postApi('validate', data);
    log(`Validation: ${result.valid}/${result.total} passed (${result.pass_rate.toFixed(1)}%)`, 'success');
    displayValidateResults(result);
  } catch (e) { log('Validate error: ' + e.message, 'error'); }
}

function loadDemo() {
  const demo = {cdrs: [
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
  log('Demo loaded: 10 CDR records', 'success');
}

function displayScrubResults(result) {
  document.getElementById('statsSection').style.display = 'block';
  let fraudCount = 0;
  (result.results || []).forEach(r => { if (r.fraud_score > 0.3) fraudCount++; });
  document.getElementById('statTotal').textContent = result.processed;
  document.getElementById('statValid').textContent = result.stats ? result.stats.valid_records : 0;
  document.getElementById('statInvalid').textContent = result.stats ? result.stats.scrubbed_records : 0;
  document.getElementById('statFraud').textContent = fraudCount;
  renderValidationChart(result.stats ? result.stats.valid_records : 0, result.stats ? result.stats.scrubbed_records : 0);
  renderFraudChart(result.results || []);
  if (result.results) {
    const tbody = document.getElementById('resultsBody');
    tbody.innerHTML = '';
    result.results.forEach(r => {
      const fc = r.fraud_score > 0.6 ? 'low' : r.fraud_score > 0.3 ? 'medium' : 'high';
      const vc = r.validation_score > 0.7 ? 'high' : r.validation_score > 0.4 ? 'medium' : 'low';
      tbody.innerHTML += `<tr><td>${r.unique_id}</td><td><span class="status-badge ${r.is_valid ? 'valid' : 'invalid'}">${r.is_valid ? 'VALID' : 'INVALID'}</span></td><td>${(r.fraud_score*100).toFixed(0)}%<span class="score-bar"><span class="score-fill ${fc}" style="width:${r.fraud_score*100}%"></span></span></td><td>${(r.validation_score*100).toFixed(0)}%<span class="score-bar"><span class="score-fill ${vc}" style="width:${r.validation_score*100}%"></span></span></td><td>${r.carrier_name||'-'}</td><td>${r.country||'-'}</td><td>${r.normalized_phone||'-'}</td><td>${r.scrub_reason||'-'}</td></tr>`;
    });
    document.getElementById('resultsSection').style.display = 'block';
  }
}

function displayProfileResults(result) {
  document.getElementById('statsSection').style.display = 'block';
  document.getElementById('statTotal').textContent = result.total_records;
  document.getElementById('statValid').textContent = (result.quality_score||0).toFixed(0)+'%';
  document.getElementById('statInvalid').textContent = (result.issues||[]).length;
  document.getElementById('statFraud').textContent = '-';
  const g = document.querySelector('.gauge-value');
  g.textContent = (result.quality_score||0).toFixed(0)+'%';
  g.style.color = result.quality_score > 80 ? 'var(--success)' : result.quality_score > 50 ? 'var(--warning)' : 'var(--danger)';
  const il = document.getElementById('issuesList');
  il.innerHTML = '';
  (result.issues||[]).forEach(i => { il.innerHTML += `<div class="issue-item"><span class="issue-severity ${(i.severity||'low').toLowerCase()}"></span><span>${i.message||i.rule||'Issue'}</span></div>`; });
  if (result.field_completeness) renderCompletenessChart(result.field_completeness);
}

function displayValidateResults(result) {
  document.getElementById('statsSection').style.display = 'block';
  document.getElementById('statTotal').textContent = result.total;
  document.getElementById('statValid').textContent = result.valid;
  document.getElementById('statInvalid').textContent = result.invalid;
  document.getElementById('statFraud').textContent = result.avg_score ? (result.avg_score*100).toFixed(0)+'%' : '-';
  renderValidationChart(result.valid, result.invalid);
  if (result.results) {
    const tbody = document.getElementById('resultsBody');
    tbody.innerHTML = '';
    result.results.forEach(r => {
      const vc = r.score > 0.7 ? 'high' : r.score > 0.4 ? 'medium' : 'low';
      tbody.innerHTML += `<tr><td>${r.unique_id}</td><td><span class="status-badge ${r.is_valid?'valid':'invalid'}">${r.is_valid?'VALID':'INVALID'}</span></td><td>-</td><td>${(r.score*100).toFixed(0)}%<span class="score-bar"><span class="score-fill ${vc}" style="width:${r.score*100}%"></span></span></td><td>-</td><td>-</td><td>-</td><td>${r.errors} errors, ${r.warnings} warnings</td></tr>`;
    });
    document.getElementById('resultsSection').style.display = 'block';
  }
}

function renderValidationChart(valid, invalid) {
  const ctx = document.getElementById('validationChart').getContext('2d');
  if (validationChart) validationChart.destroy();
  validationChart = new Chart(ctx, { type: 'doughnut', data: { labels: ['Valid','Invalid'], datasets: [{ data: [valid,invalid], backgroundColor: ['#10b981','#ef4444'], borderWidth: 0 }] }, options: { responsive: true, plugins: { legend: { labels: { color: '#94a3b8' } } } } });
}

function renderFraudChart(results) {
  const b = [0,0,0,0,0];
  results.forEach(r => { const s = r.fraud_score; s<=0.1?b[0]++:s<=0.3?b[1]++:s<=0.5?b[2]++:s<=0.7?b[3]++:b[4]++; });
  const ctx = document.getElementById('fraudChart').getContext('2d');
  if (fraudChart) fraudChart.destroy();
  fraudChart = new Chart(ctx, { type: 'bar', data: { labels: ['None','Low','Medium','High','Critical'], datasets: [{ data: b, backgroundColor: ['#10b981','#06b6d4','#f59e0b','#f97316','#ef4444'], borderWidth: 0, borderRadius: 4 }] }, options: { responsive: true, plugins: { legend: { display: false } }, scales: { x: { ticks: { color: '#94a3b8' }, grid: { color: '#1a2236' } }, y: { ticks: { color: '#94a3b8' }, grid: { color: '#1a2236' }, beginAtZero: true } } } });
}

function renderCompletenessChart(c) {
  const ctx = document.getElementById('validationChart').getContext('2d');
  if (validationChart) validationChart.destroy();
  validationChart = new Chart(ctx, { type: 'bar', data: { labels: Object.keys(c), datasets: [{ data: Object.values(c), backgroundColor: Object.values(c).map(v => v>90?'#10b981':v>70?'#f59e0b':'#ef4444'), borderWidth: 0, borderRadius: 4 }] }, options: { indexAxis: 'y', responsive: true, plugins: { legend: { display: false } }, scales: { x: { max: 100, ticks: { color: '#94a3b8' }, grid: { color: '#1a2236' } }, y: { ticks: { color: '#94a3b8' }, grid: { display: false } } } } });
}

function log(msg, type = 'info') {
  const el = document.getElementById('logOutput');
  el.innerHTML += `<div class="log-entry ${type}">[${new Date().toLocaleTimeString()}] ${msg}</div>`;
  el.scrollTop = el.scrollHeight;
}
