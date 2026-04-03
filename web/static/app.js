(function () {
  'use strict';

  let burnChart = null;
  let tokenChart = null;
  const MAX_CHART_POINTS = 720;
  const MAX_EVENTS = 100;

  let ws = null;
  let reconnectTimer = null;

  function connect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(proto + '//' + location.host + '/ws');

    ws.onopen = function () {
      document.getElementById('disconnected').classList.remove('show');
      if (reconnectTimer) { clearInterval(reconnectTimer); reconnectTimer = null; }
    };

    ws.onclose = function () {
      document.getElementById('disconnected').classList.add('show');
      if (!reconnectTimer) {
        reconnectTimer = setInterval(connect, 3000);
      }
    };

    ws.onmessage = function (e) {
      var msg = JSON.parse(e.data);
      switch (msg.type) {
        case 'state': handleState(msg); break;
        case 'history': handleHistory(msg); break;
        case 'event': handleEvent(msg); break;
      }
    };
  }

  function gradientColor(pct) {
    if (pct >= 90) return '#f38ba8';
    if (pct >= 70) return '#fab387';
    if (pct >= 50) return '#f9e2af';
    return '#a6e3a1';
  }

  function gradientCSS(pct) {
    if (pct >= 70) return 'linear-gradient(90deg, #f9e2af, #fab387, #f38ba8)';
    if (pct >= 50) return 'linear-gradient(90deg, #a6e3a1, #f9e2af, #fab387)';
    return 'linear-gradient(90deg, #a6e3a1, #f9e2af)';
  }

  function handleState(msg) {
    updateUsage(msg.usage);
    updateBurnRate(msg.burn_rate);
    updateModels(msg.models);
    updateSessions(msg.sessions);
    updateFooter();
  }

  function updateUsage(usage) {
    if (!usage.has_data) return;

    var stale = document.getElementById('stale-indicator');
    stale.textContent = usage.is_stale ? '[STALE]' : '';
    stale.style.color = usage.is_stale ? '#fab387' : '';

    var fivePct = usage.five_hour.used_pct;
    var fiveBar = document.getElementById('five-hour-bar');
    fiveBar.style.width = fivePct + '%';
    fiveBar.style.background = gradientCSS(fivePct);
    document.getElementById('five-hour-label').textContent =
      Math.round(fivePct) + '% ' + usage.five_hour.severity;
    document.getElementById('five-hour-resets').textContent =
      'Resets at: ' + new Date(usage.five_hour.resets_at).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});

    var sevenPct = usage.seven_day.used_pct;
    var sevenBar = document.getElementById('seven-day-bar');
    sevenBar.style.width = sevenPct + '%';
    sevenBar.style.background = gradientCSS(sevenPct);
    document.getElementById('seven-day-label').textContent =
      Math.round(sevenPct) + '% ' + usage.seven_day.severity;
    document.getElementById('seven-day-resets').textContent =
      'Resets at: ' + new Date(usage.seven_day.resets_at).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'});
  }

  function updateBurnRate(br) {
    var rateEl = document.getElementById('burn-rate');
    var velEl = document.getElementById('token-velocity');
    var tteBox = document.getElementById('tte-box');
    var tteVal = document.getElementById('tte-value');

    if (!br.has_data) {
      rateEl.textContent = '--';
      velEl.textContent = '--';
      tteVal.textContent = '--';
      return;
    }

    rateEl.textContent = br.pct_per_hour.toFixed(1) + '%';
    rateEl.style.color = gradientColor(br.pct_per_hour * 5);

    var tph = br.tokens_per_hour;
    velEl.textContent = (tph >= 1e6 ? (tph/1e6).toFixed(1)+'M' : tph >= 1e3 ? Math.round(tph/1e3)+'k' : Math.round(tph)) + ' t/h';

    if (br.tte_minutes <= 0) {
      tteVal.textContent = 'Safe';
      tteBox.className = 'tte-box safe';
    } else {
      var h = Math.floor(br.tte_minutes / 60);
      var m = Math.round(br.tte_minutes % 60);
      tteVal.textContent = h > 0 ? 'Limit in ~' + h + 'h ' + m + 'm' : 'Limit in ~' + m + 'm';
      tteBox.className = 'tte-box';
    }
  }

  function updateModels(models) {
    var container = document.getElementById('model-cards');
    if (!models || Object.keys(models).length === 0) {
      container.innerHTML = '';
      return;
    }

    var modelColors = { 'Opus': '#cba6f7', 'Sonnet': '#89b4fa', 'Haiku': '#a6e3a1' };
    function colorFor(name) {
      for (var key in modelColors) {
        if (name.indexOf(key) !== -1) return modelColors[key];
      }
      return '#cdd6f4';
    }

    var html = '';
    for (var name in models) {
      var info = models[name];
      var col = colorFor(name);
      var tokens = info.total_tokens >= 1e6
        ? (info.total_tokens/1e6).toFixed(1)+'M'
        : info.total_tokens >= 1e3
          ? Math.round(info.total_tokens/1e3)+'k'
          : info.total_tokens;
      html += '<div class="model-card">' +
        '<div style="display:flex;align-items:center;gap:6px;margin-bottom:8px;">' +
          '<span class="dot" style="background:' + col + ';"></span>' +
          '<span style="font-size:11px;font-weight:600;">' + name + '</span>' +
        '</div>' +
        '<div class="tokens" style="color:' + col + ';">' + tokens + '</div>' +
        '<div style="color:var(--on-surface-variant);font-size:10px;">tokens (' + info.pct.toFixed(0) + '%)</div>' +
        '<div class="model-bar">' +
          '<div class="model-bar-fill" style="width:' + info.pct + '%;background:' + col + ';"></div>' +
        '</div>' +
      '</div>';
    }
    container.innerHTML = html;
  }

  function updateSessions(sessions) {
    var body = document.getElementById('session-body');
    var totalEl = document.getElementById('session-total');
    var loadEl = document.getElementById('session-load');

    totalEl.textContent = 'Total: ' + String(sessions.length).padStart(2, '0');

    var hasBlocked = sessions.some(function(s) { return s.status === 'blocked'; });
    var hasWaiting = sessions.some(function(s) { return s.status === 'waiting'; });
    if (hasBlocked) {
      loadEl.innerHTML = '<span style="color:var(--error);">Load: Critical</span>';
    } else if (hasWaiting) {
      loadEl.innerHTML = '<span style="color:var(--secondary);">Load: Attention</span>';
    } else {
      loadEl.innerHTML = '<span style="color:var(--green);">Load: Nominal</span>';
    }

    var html = '';
    for (var i = 0; i < sessions.length; i++) {
      var s = sessions[i];
      var modelClass = s.model.indexOf('Opus') !== -1 ? 'opus'
        : s.model.indexOf('Sonnet') !== -1 ? 'sonnet'
        : s.model.indexOf('Haiku') !== -1 ? 'haiku' : '';
      var shortModel = s.model.split(' ')[0] || s.model;
      var statusIcons = {
        working: '<span class="material-symbols-outlined icon-sm">sync</span>',
        waiting: '<span class="material-symbols-outlined icon-sm">hourglass_empty</span>',
        blocked: '<span class="material-symbols-outlined icon-sm" style="font-variation-settings:\'FILL\' 1;">notifications_active</span>',
        idle: '<span class="material-symbols-outlined icon-sm">circle</span>',
        zombie_state: ''
      };
      var icon = statusIcons[s.status] || '';
      var label = s.status.replace('_', ' ').replace(/\b\w/g, function(c) { return c.toUpperCase(); });

      html += '<tr>' +
        '<td><span class="session-id" title="' + s.cwd + '" data-pid="' + s.pid + '">' + s.hex_id + '</span></td>' +
        '<td>' + s.name + '</td>' +
        '<td><span class="model-badge ' + modelClass + '">' + shortModel + '</span></td>' +
        '<td style="color:var(--on-surface-variant);font-family:JetBrains Mono,monospace;">' + s.latency + '</td>' +
        '<td><span class="status-chip ' + s.status + '">' + icon + ' ' + label + '</span></td>' +
      '</tr>';
    }
    body.innerHTML = html;
  }

  function updateFooter() {
    var now = new Date();
    document.getElementById('footer-left').textContent =
      'claude-monitor v1.0.0 // ' + now.toISOString().slice(0,19) + 'Z';
    document.getElementById('footer-right').textContent = 'Connected';
    document.getElementById('header-time').textContent = now.toLocaleTimeString();
    document.getElementById('header-version').textContent = 'v1.0.0';
  }

  function handleHistory(msg) {
    var time = new Date(msg.timestamp);
    var label = time.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'});

    if (burnChart) {
      burnChart.data.labels.push(label);
      burnChart.data.datasets[0].data.push(msg.burn_rate_pct_per_hour);
      if (burnChart.data.labels.length > MAX_CHART_POINTS) {
        burnChart.data.labels.shift();
        burnChart.data.datasets[0].data.shift();
      }
      burnChart.update('none');
    }

    if (tokenChart) {
      tokenChart.data.labels.push(label);
      tokenChart.data.datasets[0].data.push(msg.total_tokens);
      if (tokenChart.data.labels.length > MAX_CHART_POINTS) {
        tokenChart.data.labels.shift();
        tokenChart.data.datasets[0].data.shift();
      }
      tokenChart.update('none');
    }
  }

  function handleEvent(msg) {
    var log = document.getElementById('event-log');
    var time = new Date(msg.timestamp).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'});
    var modelTag = msg.model ? ' <span class="model-ref">(' + msg.model.split(' ')[0] + ')</span>' : '';
    var hexId = '0x' + (msg.pid & 0xFFFF).toString(16).toUpperCase().padStart(4, '0');
    var entry = document.createElement('div');
    entry.className = 'event-entry';
    entry.innerHTML = '<span class="time">' + time + '</span> &nbsp; <span class="session-ref">' + hexId + '</span> ' + msg.session + modelTag + ' &nbsp; ' + msg.detail;
    log.insertBefore(entry, log.firstChild);

    while (log.children.length > MAX_EVENTS) {
      log.removeChild(log.lastChild);
    }
  }

  function initCharts() {
    var chartDefaults = {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      scales: {
        x: {
          display: false,
          ticks: { color: '#45475a', maxTicksLimit: 6 },
          grid: { color: 'rgba(69,71,90,0.2)' }
        },
        y: {
          ticks: { color: '#45475a', font: { size: 9 } },
          grid: { color: 'rgba(69,71,90,0.2)' }
        }
      },
      plugins: { legend: { display: false } }
    };

    burnChart = new Chart(document.getElementById('burn-chart'), {
      type: 'line',
      data: {
        labels: [],
        datasets: [{
          data: [],
          borderColor: '#cba6f7',
          backgroundColor: 'rgba(203,166,247,0.1)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 1.5
        }]
      },
      options: Object.assign({}, chartDefaults, {
        scales: Object.assign({}, chartDefaults.scales, {
          y: Object.assign({}, chartDefaults.scales.y, { beginAtZero: true })
        })
      })
    });

    tokenChart = new Chart(document.getElementById('token-chart'), {
      type: 'line',
      data: {
        labels: [],
        datasets: [{
          data: [],
          borderColor: '#89b4fa',
          backgroundColor: 'rgba(137,180,250,0.1)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 1.5
        }]
      },
      options: chartDefaults
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    initCharts();
    connect();

    // Event delegation for session ID clicks
    document.addEventListener('click', function (e) {
      var el = e.target.closest('.session-id');
      if (!el) return;
      var pid = el.getAttribute('data-pid');
      if (!pid) return;
      fetch('/api/tmux/focus/' + pid, { method: 'POST' })
        .then(function (r) { return r.json(); })
        .then(function (data) {
          if (!data.ok) console.warn('tmux focus failed:', data);
        })
        .catch(function (err) { console.warn('tmux focus error:', err); });
    });
  });
})();
