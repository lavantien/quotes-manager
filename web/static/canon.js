(function () {
  'use strict';

  var TYPE_COLORS = {
    collection: '#9a6b3f',
    division: '#b25a2a',
    text: '#4a7c59',
    segment: '#8a5a7f',
    edition: '#9c9384',
    source: '#9c9384',
    headword: '#a8842c',
    dpd_root: '#9a3b2f',
    dpd_form: '#7a7268'
  };

  var TERMINAL_STATES = { done: true, error: true, canceled: true };

  function colorFor(type) {
    return TYPE_COLORS[type] || '#a89f8d';
  }

  function muidFromSegmentKey(key) {
    if (key.indexOf('pli:') !== 0) return null;
    var rest = key.slice(4);
    var i = rest.indexOf(':');
    return i < 0 ? null : rest.slice(0, i);
  }

  function hrefForNode(data) {
    switch (data.type) {
      case 'segment':
        var muid = muidFromSegmentKey(data.id);
        return muid ? '/texts/' + encodeURIComponent(muid) : null;
      case 'text':
        return '/texts/' + encodeURIComponent(data.id);
      case 'headword':
        return '/words/' + encodeURIComponent(data.id.replace(/^dpd:/, ''));
      case 'dpd_root':
        return '/graph?dataset=root&param=' + encodeURIComponent(data.label || data.id);
      case 'division':
      case 'collection':
        return '/graph?dataset=tree&param=' + encodeURIComponent(data.id);
      default:
        return null;
    }
  }

  function fetchJSON(url) {
    return fetch(url, { headers: { Accept: 'application/json' } }).then(function (r) {
      if (!r.ok) throw new Error('http ' + r.status);
      return r.json();
    });
  }

  function graphStyle() {
    return [
      {
        selector: 'node',
        style: {
          'background-color': function (ele) { return colorFor(ele.data('type')); },
          'label': 'data(label)',
          'color': '#1c1a17',
          'font-size': 9,
          'text-valign': 'bottom',
          'text-wrap': 'ellipsis',
          'max-width': 120,
          width: 14,
          height: 14
        }
      },
      {
        selector: 'edge',
        style: {
          'width': 2,
          'opacity': 0.55,
          'line-color': '#a89f8d',
          'target-arrow-shape': 'triangle',
          'arrow-scale': 0.6,
          'curve-style': 'bezier'
        }
      }
    ];
  }

  function layoutFor(name) {
    if (name === 'breadthfirst') {
      return { name: 'breadthfirst', directed: true, padding: 20, spacingFactor: 1.3, animate: false };
    }
    return { name: 'concentric', padding: 20, levelWidth: function () { return 2; }, animate: false };
  }

  function renderGraph(container, payload, layoutName) {
    if (typeof cytoscape === 'undefined') return null;
    var elements = (payload && payload.elements) || {};
    var cy = cytoscape({
      container: container,
      elements: { nodes: elements.nodes || [], edges: elements.edges || [] },
      style: graphStyle(),
      layout: layoutFor(layoutName)
    });
    cy.on('tap', 'node', function (ev) {
      var href = hrefForNode(ev.target.data());
      if (href) window.location.assign(href);
    });
    return cy;
  }

  function showBanner(message) {
    var banner = document.getElementById('graph-banner');
    if (!banner) return;
    if (message) {
      banner.textContent = message;
      banner.classList.remove('hidden');
    } else {
      banner.classList.add('hidden');
    }
  }

  function initGraphPage() {
    var form = document.getElementById('graph-form');
    var canvas = document.getElementById('graph-canvas');
    if (!form || !canvas) return;
    var select = document.getElementById('graph-dataset');
    var input = document.getElementById('graph-param');
    var cy = null;

    function activeOption() {
      return select.options[select.selectedIndex];
    }

    function syncInput() {
      var opt = activeOption();
      if (!opt) return;
      input.setAttribute('placeholder', opt.getAttribute('data-placeholder') || '');
      var list = opt.getAttribute('data-list');
      if (list) input.setAttribute('list', list);
    }

    function draw(url, layout) {
      fetchJSON(url)
        .then(function (payload) {
          showBanner(payload.meta && payload.meta.truncated
            ? 'result truncated to caps: ' + (payload.meta.nodes || 0) + ' nodes shown'
            : '');
          if (cy) {
            cy.destroy();
            cy = null;
          }
          cy = renderGraph(canvas, payload, layout);
        })
        .catch(function (err) { showBanner('graph load failed: ' + err.message); });
    }

    function buildURL() {
      var opt = activeOption();
      var value = (input.value || '').trim();
      if (!opt || !value) return null;
      return opt.getAttribute('data-api') + '?' + opt.getAttribute('data-param') +
        '=' + encodeURIComponent(value);
    }

    form.addEventListener('submit', function (ev) {
      ev.preventDefault();
      var url = buildURL();
      if (!url) return;
      draw(url, activeOption().getAttribute('data-layout'));
      history.replaceState(null, '',
        '/graph?dataset=' + encodeURIComponent(select.value) +
        '&param=' + encodeURIComponent(input.value.trim()));
    });

    select.addEventListener('change', syncInput);
    syncInput();
    var initial = buildURL();
    if (initial) draw(initial, activeOption().getAttribute('data-layout'));
  }

  function initMiniGraphs() {
    var canvases = document.querySelectorAll('.graph-canvas[data-graph-url]');
    Array.prototype.forEach.call(canvases, function (el) {
      if (el.id === 'graph-canvas') return;
      fetchJSON(el.getAttribute('data-graph-url'))
        .then(function (payload) { renderGraph(el, payload, el.getAttribute('data-layout')); })
        .catch(function () {});
    });
  }

  function initJobLog() {
    var log = document.getElementById('job-log');
    if (!log || log.getAttribute('data-polling') !== 'true') return;
    if (log.getAttribute('data-bound') === 'true') return;
    log.setAttribute('data-bound', 'true');
    var baseURL = log.getAttribute('data-log-url') || '';
    var next = parseInt(log.getAttribute('data-next-seq') || '0', 10);
    var tbody = log.querySelector('tbody');
    if (!tbody || !baseURL) return;

    function jobTerminal() {
      var status = document.getElementById('job-status');
      if (!status) return true;
      return Object.prototype.hasOwnProperty.call(TERMINAL_STATES, status.getAttribute('data-state'));
    }

    var busy = false;
    window.setInterval(function () {
      if (jobTerminal() || !document.body.contains(log)) {
        return;
      }
      if (busy) return;
      busy = true;
      var joiner = baseURL.indexOf('?') < 0 ? '?' : '&';
      fetchJSON(baseURL + joiner + 'after=' + next)
        .then(function (lines) {
          busy = false;
          if (!lines || !lines.length) return;
          lines.forEach(function (line) {
            var seq = line.seq != null ? line.seq : line.Seq;
            var text = line.text != null ? line.text : line.Text;
            var tr = document.createElement('tr');
            tr.className = 'log-line';
            tr.setAttribute('data-seq', String(seq));
            var seqCell = document.createElement('td');
            seqCell.className = 'seq';
            seqCell.textContent = String(seq);
            var textCell = document.createElement('td');
            textCell.className = 'text';
            textCell.textContent = text == null ? '' : String(text);
            tr.appendChild(seqCell);
            tr.appendChild(textCell);
            tbody.appendChild(tr);
            if (seq >= next) next = seq + 1;
          });
          log.setAttribute('data-next-seq', String(next));
        })
        .catch(function () { busy = false; });
    }, 2000);
  }

  function boot() {
    initGraphPage();
    initMiniGraphs();
    initJobLog();
  }

  document.body.addEventListener('htmx:afterSwap', function () {
    initJobLog();
  });

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
