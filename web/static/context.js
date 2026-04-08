(function () {
  'use strict';

  var MAX_TIMELINE_POINTS = 200;
  var timelineChart = null;
  var ws = null;
  var reconnectTimer = null;

  var CAT_COLORS = {
    system:   '#f38ba8',
    tools:    '#fab387',
    history:  '#89b4fa',
    results:  '#a6e3a1',
    thinking: '#f9e2af'
  };

  var CAT_LABELS = {
    system:   'System',
    tools:    'Tools',
    history:  'History',
    results:  'Results',
    thinking: 'Thinking'
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
      if (msg.type === 'context') {
        handleContext(msg);
      }
    };
  }

  function handleContext(msg) {
    updateCompositionBar(msg.categories);
    updateStats(msg);
    updateTimeline(msg);
    updateFooter();
  }

  function updateCompositionBar(categories) {
    var bar = document.getElementById('ctx-bar');
    var legend = document.getElementById('ctx-legend');
    var html = '';
    var legendHtml = '';
    var cats = ['system', 'tools', 'history', 'results', 'thinking'];

    for (var i = 0; i < cats.length; i++) {
      var key = cats[i];
      var cat = categories[key];
      if (!cat || cat.pct === 0) continue;
      html += '<div class="ctx-bar-segment" style="width:' + cat.pct +
        '%;background:' + CAT_COLORS[key] + ';" title="' +
        CAT_LABELS[key] + ' ' + cat.pct.toFixed(1) + '%"></div>';
    }
    bar.innerHTML = html;

    for (var j = 0; j < cats.length; j++) {
      var k = cats[j];
      var c = categories[k];
      var tokens = c ? formatTokens(c.tokens) : '0';
      var pct = c ? c.pct.toFixed(1) : '0.0';
      legendHtml += '<span class="ctx-legend-item">' +
        '<span class="ctx-legend-dot" style="background:' + CAT_COLORS[k] + ';"></span>' +
        CAT_LABELS[k] + ' <span style="color:var(--outline);">' + pct + '% (' + tokens + ')</span></span>';
    }
    legend.innerHTML = legendHtml;
  }

  function updateStats(msg) {
    // Context fill
    var fillEl = document.getElementById('ctx-fill');
    var fillDetail = document.getElementById('ctx-fill-detail');
    fillEl.textContent = msg.used_pct.toFixed(1) + '%';
    fillEl.style.color = msg.used_pct >= 80 ? '#f38ba8' : msg.used_pct >= 50 ? '#fab387' : '#a6e3a1';
    fillDetail.textContent = formatTokens(msg.total_input_tokens) + ' / ' + formatTokens(msg.context_window_size);

    // Turn
    document.getElementById('ctx-turn').textContent = msg.turn_number;
    var avg = msg.turn_number > 0 ? Math.round(msg.total_input_tokens / msg.turn_number) : 0;
    document.getElementById('ctx-turn-detail').textContent = '~' + formatTokens(avg) + '/turn avg';

    // Biggest category
    var biggest = '';
    var biggestPct = 0;
    var biggestTokens = 0;
    var cats = ['system', 'tools', 'history', 'results', 'thinking'];
    for (var i = 0; i < cats.length; i++) {
      var cat = msg.categories[cats[i]];
      if (cat && cat.pct > biggestPct) {
        biggest = cats[i];
        biggestPct = cat.pct;
        biggestTokens = cat.tokens;
      }
    }
    var bigEl = document.getElementById('ctx-biggest');
    bigEl.textContent = CAT_LABELS[biggest] || '--';
    bigEl.style.color = CAT_COLORS[biggest] || 'var(--on-surface)';
    document.getElementById('ctx-biggest-detail').textContent =
      biggestPct.toFixed(1) + '% — ' + formatTokens(biggestTokens);

    // Proxy status
    document.getElementById('ctx-proxy-status').textContent = 'Active';
    document.getElementById('ctx-proxy-status').style.color = '#a6e3a1';
    document.getElementById('ctx-proxy-detail').textContent = msg.model || '';
  }

  function updateTimeline(msg) {
    if (!timelineChart) return;
    var label = 'T' + msg.turn_number;
    var cats = ['system', 'tools', 'history', 'results', 'thinking'];

    timelineChart.data.labels.push(label);
    for (var i = 0; i < cats.length; i++) {
      var cat = msg.categories[cats[i]];
      timelineChart.data.datasets[i].data.push(cat ? cat.tokens : 0);
    }

    if (timelineChart.data.labels.length > MAX_TIMELINE_POINTS) {
      timelineChart.data.labels.shift();
      for (var j = 0; j < timelineChart.data.datasets.length; j++) {
        timelineChart.data.datasets[j].data.shift();
      }
    }
    timelineChart.update('none');
  }

  function updateFooter() {
    var now = new Date();
    document.getElementById('footer-left').textContent =
      'claude-monitor // ' + now.toISOString().slice(0, 19) + 'Z';
    document.getElementById('footer-right').textContent = 'Connected';
    document.getElementById('header-time').textContent = now.toLocaleTimeString();
    document.getElementById('header-version').textContent = 'v1.0.0';
  }

  function formatTokens(n) {
    if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
    if (n >= 1e3) return Math.round(n / 1e3) + 'k';
    return String(n);
  }

  function initChart() {
    var cats = ['system', 'tools', 'history', 'results', 'thinking'];
    var datasets = [];

    for (var i = 0; i < cats.length; i++) {
      datasets.push({
        label: CAT_LABELS[cats[i]],
        data: [],
        backgroundColor: CAT_COLORS[cats[i]] + '99',
        borderColor: CAT_COLORS[cats[i]],
        borderWidth: 1,
        fill: true,
        pointRadius: 0,
        tension: 0.3
      });
    }

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
