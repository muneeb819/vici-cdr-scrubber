const API_BASE = '';
let validationChart = null;
let fraudChart = null;

document.addEventListener('DOMContentLoaded', () => {
  checkHealth();
  loadFeatures();
  setupUpload();
});

async function checkHealth() {
  try {
    const res = await fetch(`${API_BASE}/api/health`);
    const data = await res.json();
    const el = document.getElementById('navStatus');
    el.innerHTML = `<span class="status-dot online"></span><span>Connected (${data.platform})</span>`;
    log('Health check: ' + data.status, 'success');
  } catch (e) {
    const el = document.getElementById('navStatus');
    el.innerHTML = `<span class="status-dot offline"></span><span>Disconnected</span>`;
    log('Health check failed: ' + e.message, 'error');
  }
}

async function loadFeatures() {
  try {
    const res = await fetch(`${API_BASE}/api/version`);
    const data = await res.json();
    const container = document.getElementById('featureTags');
    data.features.forEach(f => {
      const tag = document.createElement('span');
      tag.className = 'feature-tag';
      tag.textContent = f;
      container.appendChild(tag);
    });
  } catch (e) {}
}

function setupUpload() {
  const zone = document.getElementById('uploadZone');
  const input = document.getElementById('fileInput');

  zone.addEventListener('click', () => input.click());
  zone.addEventListener('dragover', e => { e.preventDefault(); zone.classList.add('dragover'); });
  zone.addEventListener('dragleave', () => zone.classList.remove('dragover'));
  zone.addEventListener('drop', e => {
    e.preventDefault();
    zone.classList.remove('dragover');
    handleFile(e.dataTransfer.files[0]);
  });
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
      log('CSV converted to JSON: ' + json.cdrs.length + ' records', 'success');
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
    headers.forEach((h, idx) => {
      let val = values[idx] ? values[idx].trim() : '';
      if (!isNaN(val) && val !== '') val = Number(val);
      obj[h] = val;
    });
    cdrs.push(obj);
  }
  return { cdrs };
}

function getInputData() {
  const raw = document.getElementById('jsonInput').value.trim();
  if (!raw) { log('No input data provided', 'error'); return null; }
  try {
    const data = JSON.parse(raw);
    if (!data.cdrs || !Array.isArray(data.cdrs)) {
      log('Invalid format: expected {"cdrs": [...]}', 'error');
      return null;
    }
    return data;
  } catch (e) {
    log('JSON parse error: ' + e.message, 'error');
    return null;
  }
}

async function processScrub() {
  const data = getInputData();
  if (!data) return;
  log(`Scrubbing ${data.cdrs.length} records...`, 'info');

  try {
    const res = await fetch(`${API_BASE}/api/scrub`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    const result = await res.json();
    log(`Scrub complete: ${result.processed} processed`, 'success');
    displayScrubResults(result);
  } catch (e) {
    log('Scrub error: ' + e.message, 'error');
  }
}

async function processProfile() {
  const data = getInputData();
  if (!data) return;
  log(`Profiling ${data.cdrs.length} records...`, 'info');

  try {
    const res = await fetch(`${API_BASE}/api/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    const result = await res.json();
    log(`Profile complete: quality score ${result.quality_score}%`, 'success');
    displayProfileResults(result);
  } catch (e) {
    log('Profile error: ' + e.message, 'error');
  }
}

async function processValidate() {
  const data = getInputData();
  if (!data) return;
  log(`Validating ${data.cdrs.length} records...`, 'info');

  try {
    const res = await fetch(`${API_BASE}/api/validate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    const result = await res.json();
    log(`Validation: ${result.valid}/${result.total} passed (${result.pass_rate.toFixed(1)}%)`, 'success');
    displayValidateResults(result);
  } catch (e) {
    log('Validate error: ' + e.message, 'error');
  }
}

function loadDemo() {
  const demo = {
    cdrs: [
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
    ]
  };
  document.getElementById('jsonInput').value = JSON.stringify(demo, null, 2);
  log('Demo data loaded: 10 CDR records', 'success');
}

function displayScrubResults(result) {
  const section = document.getElementById('statsSection');
  section.style.display = 'block';

  let fraudCount = 0;
  if (result.results) {
    result.results.forEach(r => { if (r.fraud_score > 0.3) fraudCount++; });
  }

  const valid = result.stats ? result.stats.valid_records : 0;
  const invalid = result.stats ? result.stats.scrubbed_records : 0;

  document.getElementById('statTotal').textContent = result.processed;
  document.getElementById('statValid').textContent = valid;
  document.getElementById('statInvalid').textContent = invalid;
  document.getElementById('statFraud').textContent = fraudCount;

  renderValidationChart(valid, invalid);
  renderFraudChart(result.results || []);

  if (result.results) {
    const tbody = document.getElementById('resultsBody');
    tbody.innerHTML = '';
    result.results.forEach(r => {
      const fraudClass = r.fraud_score > 0.6 ? 'low' : r.fraud_score > 0.3 ? 'medium' : 'high';
      const valClass = r.validation_score > 0.7 ? 'high' : r.validation_score > 0.4 ? 'medium' : 'low';
      tbody.innerHTML += `
        <tr>
          <td>${r.unique_id}</td>
          <td><span class="status-badge ${r.is_valid ? 'valid' : 'invalid'}">${r.is_valid ? 'VALID' : 'INVALID'}</span></td>
          <td>${(r.fraud_score * 100).toFixed(0)}%<span class="score-bar"><span class="score-fill ${fraudClass}" style="width:${r.fraud_score * 100}%"></span></span></td>
          <td>${(r.validation_score * 100).toFixed(0)}%<span class="score-bar"><span class="score-fill ${valClass}" style="width:${r.validation_score * 100}%"></span></span></td>
          <td>${r.carrier_name || '-'}</td>
          <td>${r.country || '-'}</td>
          <td>${r.normalized_phone || '-'}</td>
          <td>${r.scrub_reason || '-'}</td>
        </tr>`;
    });
    document.getElementById('resultsSection').style.display = 'block';
  }
}

function displayProfileResults(result) {
  const section = document.getElementById('statsSection');
  section.style.display = 'block';

  document.getElementById('statTotal').textContent = result.total_records;
  document.getElementById('statValid').textContent = (result.quality_score || 0).toFixed(0) + '%';
  document.getElementById('statInvalid').textContent = (result.issues || []).length;
  document.getElementById('statFraud').textContent = '-';

  const gauge = document.querySelector('.gauge-value');
  gauge.textContent = (result.quality_score || 0).toFixed(0) + '%';
  gauge.style.color = result.quality_score > 80 ? 'var(--success)' : result.quality_score > 50 ? 'var(--warning)' : 'var(--danger)';

  const issuesList = document.getElementById('issuesList');
  issuesList.innerHTML = '';
  (result.issues || []).forEach(issue => {
    const sev = (issue.severity || 'low').toLowerCase();
    issuesList.innerHTML += `
      <div class="issue-item">
        <span class="issue-severity ${sev}"></span>
        <span>${issue.message || issue.rule || 'Issue found'}</span>
      </div>`;
  });

  if (result.field_completeness) {
    renderCompletenessChart(result.field_completeness);
  }
}

function displayValidateResults(result) {
  const section = document.getElementById('statsSection');
  section.style.display = 'block';

  document.getElementById('statTotal').textContent = result.total;
  document.getElementById('statValid').textContent = result.valid;
  document.getElementById('statInvalid').textContent = result.invalid;
  document.getElementById('statFraud').textContent = result.avg_score ? (result.avg_score * 100).toFixed(0) + '%' : '-';

  renderValidationChart(result.valid, result.invalid);

  if (result.results) {
    const tbody = document.getElementById('resultsBody');
    tbody.innerHTML = '';
    result.results.forEach(r => {
      const valClass = r.score > 0.7 ? 'high' : r.score > 0.4 ? 'medium' : 'low';
      tbody.innerHTML += `
        <tr>
          <td>${r.unique_id}</td>
          <td><span class="status-badge ${r.is_valid ? 'valid' : 'invalid'}">${r.is_valid ? 'VALID' : 'INVALID'}</span></td>
          <td>-</td>
          <td>${(r.score * 100).toFixed(0)}%<span class="score-bar"><span class="score-fill ${valClass}" style="width:${r.score * 100}%"></span></span></td>
          <td>-</td>
          <td>-</td>
          <td>-</td>
          <td>${r.errors} errors, ${r.warnings} warnings</td>
        </tr>`;
    });
    document.getElementById('resultsSection').style.display = 'block';
  }
}

function renderValidationChart(valid, invalid) {
  const ctx = document.getElementById('validationChart').getContext('2d');
  if (validationChart) validationChart.destroy();
  validationChart = new Chart(ctx, {
    type: 'doughnut',
    data: {
      labels: ['Valid', 'Invalid'],
      datasets: [{
        data: [valid, invalid],
        backgroundColor: ['#10b981', '#ef4444'],
        borderWidth: 0
      }]
    },
    options: {
      responsive: true,
      plugins: {
        legend: { labels: { color: '#94a3b8' } }
      }
    }
  });
}

function renderFraudChart(results) {
  const buckets = [0, 0, 0, 0, 0];
  const labels = ['None', 'Low', 'Medium', 'High', 'Critical'];
  results.forEach(r => {
    const s = r.fraud_score;
    if (s <= 0.1) buckets[0]++;
    else if (s <= 0.3) buckets[1]++;
    else if (s <= 0.5) buckets[2]++;
    else if (s <= 0.7) buckets[3]++;
    else buckets[4]++;
  });
  const ctx = document.getElementById('fraudChart').getContext('2d');
  if (fraudChart) fraudChart.destroy();
  fraudChart = new Chart(ctx, {
    type: 'bar',
    data: {
      labels,
      datasets: [{
        data: buckets,
        backgroundColor: ['#10b981', '#06b6d4', '#f59e0b', '#f97316', '#ef4444'],
        borderWidth: 0,
        borderRadius: 4
      }]
    },
    options: {
      responsive: true,
      plugins: { legend: { display: false } },
      scales: {
        x: { ticks: { color: '#94a3b8' }, grid: { color: '#1a2236' } },
        y: { ticks: { color: '#94a3b8' }, grid: { color: '#1a2236' }, beginAtZero: true }
      }
    }
  });
}

function renderCompletenessChart(completeness) {
  const ctx = document.getElementById('validationChart').getContext('2d');
  if (validationChart) validationChart.destroy();
  const labels = Object.keys(completeness);
  const values = Object.values(completeness);
  validationChart = new Chart(ctx, {
    type: 'bar',
    data: {
      labels,
      datasets: [{
        label: 'Completeness %',
        data: values,
        backgroundColor: values.map(v => v > 90 ? '#10b981' : v > 70 ? '#f59e0b' : '#ef4444'),
        borderWidth: 0,
        borderRadius: 4
      }]
    },
    options: {
      indexAxis: 'y',
      responsive: true,
      plugins: { legend: { display: false } },
      scales: {
        x: { max: 100, ticks: { color: '#94a3b8' }, grid: { color: '#1a2236' } },
        y: { ticks: { color: '#94a3b8' }, grid: { display: false } }
      }
    }
  });
}

function log(msg, type = 'info') {
  const el = document.getElementById('logOutput');
  const time = new Date().toLocaleTimeString();
  el.innerHTML += `<div class="log-entry ${type}">[${time}] ${msg}</div>`;
  el.scrollTop = el.scrollHeight;
}
