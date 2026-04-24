(function () {
  'use strict';

  var MAX_TIMELINE_POINTS = 200;
  var timelineChart = null;
  var ws = null;
  var reconnectTimer = null;
  var hasProxyData = false;

  // Cache-split palette.
  var CAT_COLORS = {
    cache_read:     '#a6e3a1',
    cache_creation: '#f9e2af',
    input:          '#f38ba8'
  };
  var CAT_LABELS = {
    cache_read:     'Cache Read',
    cache_creation: 'Cache Creation',
    input:          'Fresh Input'
  };

  function connect() {
    var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
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
      if (msg.type === 'jsonl_context') {
        handleJSONL(msg);
      } else if (msg.type === 'context') {
        // Proxy data — only honor it if the user actually routed through it.
        hasProxyData = true;
        handleProxyContext(msg);
      }
    };
  }

  function formatTokens(n) {
    if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
    if (n >= 1e3) return Math.round(n / 1e3) + 'k';
    return String(n);
  }

  function handleJSONL(msg) {
    document.getElementById('ctx-empty').style.display = 'none';

    updateFocus(msg);
    updateCompositionBar(msg);
    updateStats(msg);
    updateTimeline(msg);
    updateFooter();
  }

  function updateFocus(msg) {
    var title = document.getElementById('ctx-focus-title');
    title.textContent = msg.project_slug || msg.session_id || '--';
    var model = (msg.model || '').replace(/^claude-/, '');
    var meta = [];
    if (model) meta.push(model);
    if (msg.active_sessions > 1) meta.push(msg.active_sessions + ' active sessions');
    if (msg.last_turn_at) meta.push('turn at ' + new Date(msg.last_turn_at).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit',second:'2-digit'}));
    document.getElementById('ctx-focus-meta').textContent = meta.join('  //  ');

    var src = document.getElementById('ctx-focus-source');
    src.textContent = hasProxyData ? 'proxy + jsonl' : 'jsonl';
  }

  function updateCompositionBar(msg) {
    var total = msg.total_input_tokens || 0;
    var bar = document.getElementById('ctx-bar');
    var legend = document.getElementById('ctx-legend');

    if (total === 0) {
      bar.innerHTML = '';
      legend.innerHTML = '';
      return;
    }

    var cats = [
      { key: 'cache_read',     value: msg.cache_read_tokens || 0 },
      { key: 'cache_creation', value: msg.cache_creation_tokens || 0 },
      { key: 'input',          value: msg.input_tokens || 0 }
    ];

    var barHtml = '';
    var legendHtml = '';
    for (var i = 0; i < cats.length; i++) {
      var c = cats[i];
      var pct = total > 0 ? (c.value / total * 100) : 0;
      if (pct > 0) {
        barHtml += '<div class="ctx-bar-segment" style="width:' + pct.toFixed(2) +
          '%;background:' + CAT_COLORS[c.key] + ';" title="' +
          CAT_LABELS[c.key] + ' ' + pct.toFixed(1) + '%"></div>';
      }
      legendHtml += '<span class="ctx-legend-item">' +
        '<span class="ctx-legend-dot" style="background:' + CAT_COLORS[c.key] + ';"></span>' +
        CAT_LABELS[c.key] + ' <span style="color:var(--outline);">' +
        pct.toFixed(1) + '% (' + formatTokens(c.value) + ')</span></span>';
    }
    bar.innerHTML = barHtml;
    legend.innerHTML = legendHtml;
  }

  function updateStats(msg) {
    var fillEl = document.getElementById('ctx-fill');
    var fillDetail = document.getElementById('ctx-fill-detail');
    var pct = msg.used_pct || 0;
    fillEl.textContent = pct.toFixed(1) + '%';
    fillEl.style.color = pct >= 80 ? '#f38ba8' : pct >= 50 ? '#fab387' : '#a6e3a1';
    fillDetail.textContent = formatTokens(msg.total_input_tokens) + ' / ' + formatTokens(msg.context_window_size);

    document.getElementById('ctx-turn').textContent = msg.turn_number;
    var avg = msg.turn_number > 0 ? Math.round((msg.total_input_tokens || 0) / msg.turn_number) : 0;
    document.getElementById('ctx-turn-detail').textContent = avg > 0 ? '~' + formatTokens(avg) + '/turn avg' : '';

    var cacheEl = document.getElementById('ctx-cache');
    var cacheDetail = document.getElementById('ctx-cache-detail');
    var hit = msg.cache_hit_ratio || 0;
    cacheEl.textContent = hit.toFixed(0) + '%';
    cacheEl.style.color = hit >= 80 ? '#a6e3a1' : hit >= 50 ? '#f9e2af' : '#f38ba8';
    cacheDetail.textContent = formatTokens(msg.cache_read_tokens) + ' read / ' +
      formatTokens(msg.cache_creation_tokens) + ' new';

    var deltaEl = document.getElementById('ctx-delta');
    var deltaDetail = document.getElementById('ctx-delta-detail');
    var delta = msg.input_delta || 0;
    deltaEl.textContent = (delta >= 0 ? '+' : '') + formatTokens(Math.abs(delta));
    deltaEl.style.color = delta < 0 ? '#a6e3a1' : delta > 5000 ? '#fab387' : '#cdd6f4';
    deltaDetail.textContent = delta === 0
      ? 'stable'
      : delta > 0 ? 'context grew' : 'context shrank';
  }

  function updateTimeline(msg) {
    if (!timelineChart) return;
    var turns = msg.turns || [];

    var labels = turns.map(function (t) { return 'T' + t.turn; });
    var cacheRead = turns.map(function (t) { return t.cache_read; });
    var cacheCreation = turns.map(function (t) { return t.cache_creation; });
    var input = turns.map(function (t) { return t.input; });

    if (labels.length > MAX_TIMELINE_POINTS) {
      var trim = labels.length - MAX_TIMELINE_POINTS;
      labels = labels.slice(trim);
      cacheRead = cacheRead.slice(trim);
      cacheCreation = cacheCreation.slice(trim);
      input = input.slice(trim);
    }

    timelineChart.data.labels = labels;
    timelineChart.data.datasets[0].data = cacheRead;
    timelineChart.data.datasets[1].data = cacheCreation;
    timelineChart.data.datasets[2].data = input;
    timelineChart.update('none');
  }

  function handleProxyContext(msg) {
    // When proxy data IS flowing, prefer it for composition/timeline because
    // it has the true 5-category split. For now we just note its presence —
    // full proxy rendering can be re-added later if needed.
    var src = document.getElementById('ctx-focus-source');
    if (src) src.textContent = 'proxy + jsonl';
  }

  function updateFooter() {
    var now = new Date();
    document.getElementById('footer-left').textContent =
      'claude-monitor // ' + now.toISOString().slice(0, 19) + 'Z';
    document.getElementById('footer-right').textContent = 'Connected';
    document.getElementById('header-time').textContent = now.toLocaleTimeString();
    document.getElementById('header-version').textContent = 'v1.1.0';
  }

  function initChart() {
    var keys = ['cache_read', 'cache_creation', 'input'];
    var datasets = keys.map(function (k) {
      return {
        label: CAT_LABELS[k],
        data: [],
        backgroundColor: CAT_COLORS[k] + '99',
        borderColor: CAT_COLORS[k],
        borderWidth: 1,
        fill: true,
        pointRadius: 0,
        tension: 0.25
      };
    });

    timelineChart = new Chart(document.getElementById('ctx-timeline-chart'), {
      type: 'line',
      data: { labels: [], datasets: datasets },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        scales: {
          x: {
            display: true,
            ticks: { color: '#45475a', maxTicksLimit: 10, font: { size: 9 } },
            grid: { color: 'rgba(69,71,90,0.2)' }
          },
          y: {
            stacked: true,
            ticks: {
              color: '#45475a',
              font: { size: 9 },
              callback: function (v) { return v >= 1e3 ? Math.round(v / 1e3) + 'k' : v; }
            },
            grid: { color: 'rgba(69,71,90,0.2)' }
          }
        },
        plugins: {
          legend: { display: false },
          tooltip: {
            mode: 'index',
            intersect: false,
            callbacks: {
              label: function (ctx) {
                return ctx.dataset.label + ': ' + formatTokens(ctx.raw);
              }
            }
          }
        }
      }
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    initChart();
    connect();
  });
})();
