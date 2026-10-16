// sc-graph frontend: cytoscape rendering, node click-through routing, and
// the append-only job log poller. Everything else on the pages is
// server-rendered HTML plus htmx attributes; this file only adds what the
// server cannot render.
(function () {
  'use strict';

  // Fixed palette per node data.type (model.NodeType values).
  var TYPE_COLORS = {
    collection: '#8e8ef2',
    division: '#6ea8fe',
    text: '#4ec9b0',
    segment: '#c586c0',
    edition: '#b8b8b8',
    source: '#b8b8b8',
    headword: '#d7ba7d',
    dpd_root: '#f2a8a8',
    dpd_form: '#9e9e9e'
  };

  var TERMINAL_STATES = { done: true, error: true, canceled: true };

  function colorFor(type) {
    return TYPE_COLORS[type] || '#8a8a8a';
  }

  // Segment node keys look like "pli:<muid>:<segkey>"; text node keys are
  // the bare muid.
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
        // Labels carry display decorations ("lemma (count)" in root
        // families); the id is the citation form.
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
          'color': '#e6e6e6',
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
          'line-color': '#8a8a8a',
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
    // The concentric layout calls levelWidth as a function; a bare number
    // crashes the layout ("levelWidth is not a function").
    return { name: 'concentric', padding: 20, levelWidth: function () { return 2; }, animate: false };
  }

  // renderGraph mounts a cytoscape instance on container from a
  // {elements, meta} payload and wires node click-through.
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

  // The /graph page: dataset picker drives a fetch of the JSON graph API.
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

  // Mini graphs (word page neighborhood) render from data-graph-url.
  function initMiniGraphs() {
    var canvases = document.querySelectorAll('.graph-canvas[data-graph-url]');
    Array.prototype.forEach.call(canvases, function (el) {
      if (el.id === 'graph-canvas') return;
      fetchJSON(el.getAttribute('data-graph-url'))
        .then(function (payload) { renderGraph(el, payload, el.getAttribute('data-layout')); })
        .catch(function () { /* leave the canvas empty */ });
    });
  }

  // The job status partial polls itself via htmx while queued or running;
  // this loop keeps the log growing with after=<seq> and stops once the
  // status element reports a terminal state. The tick is guarded by an
  // in-flight flag so a slow response cannot double-append when the next
  // interval fires before it resolves.
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
        .catch(function () { busy = false; /* retried on the next tick */ });
    }, 2000);
  }

  function boot() {
    initGraphPage();
    initMiniGraphs();
    initJobLog();
  }

  // A swapped-in pipeline panel brings a fresh log container; bind it.
  document.body.addEventListener('htmx:afterSwap', function () {
    initJobLog();
  });

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
