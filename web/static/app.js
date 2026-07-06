(function () {
  "use strict";

  var rootList = () => document.getElementById("quote-list");

  function selectedIds() {
    var list = rootList();
    if (!list) return [];
    return Array.from(list.querySelectorAll("input.select:checked"))
      .map(function (cb) { return Number(cb.value); });
  }

  function currentCat() { var z = document.getElementById("root-zone"); return z ? (z.dataset.cat || "") : ""; }
  function currentCol() { var z = document.getElementById("collection-zone"); return z ? (z.dataset.cid || "") : ""; }
  function inputValue(name) { var el = document.querySelector('input[name="' + name + '"]'); return el ? el.value : ""; }
  function ctxQuery() { return "?cat=" + currentCat() + "&col=" + currentCol(); }

  // Toggle the bulk/delete/insert affordances based on whether any root quote
  // is checked.
  function refreshBulk() {
    var any = selectedIds().length > 0;
    document.querySelectorAll('[data-action="bulk-delete"], [data-action="new-collection"]')
      .forEach(function (b) { b.disabled = !any; });
    document.querySelectorAll(".insert-gap").forEach(function (g) {
      g.classList.toggle("is-ready", any);
    });
  }

  // Sync the two columns' header rows (.zone__head, .zone__toolbar) to the
  // taller of the pair so the divider lines stay aligned across columns,
  // regardless of content or htmx swaps. min-height (not height) so a row may
  // still grow; the inline value is reset first so the measurement reflects the
  // natural height. Guarded so the empty-collection state (one toolbar) and any
  // transient measurement error can never break the rest of the IIFE.
  function equalizeRow(sel) {
    var els = Array.prototype.slice.call(document.querySelectorAll(sel));
    if (els.length < 2) { els.forEach(function (e) { e.style.minHeight = ""; }); return; }
    try {
      els.forEach(function (e) { e.style.minHeight = ""; });
      var max = Math.max.apply(null, els.map(function (e) { return e.offsetHeight; }));
      els.forEach(function (e) { e.style.minHeight = max + "px"; });
    } catch (err) { /* never break the caller */ }
  }
  function equalizeHeaders() {
    equalizeRow(".zone__head");
    equalizeRow(".zone__toolbar");
  }

  function flash(btn, text) {
    var orig = btn.textContent;
    btn.textContent = text;
    btn.disabled = true;
    setTimeout(function () { btn.textContent = orig; btn.disabled = false; }, 1200);
  }

  // copyFromUrl writes url's text to the clipboard. The write is started
  // synchronously (within the user gesture) by handing ClipboardItem a promise
  // for the data, so transient activation is not lost while the text is fetched.
  function copyFromUrl(url) {
    var data = fetch(url).then(function (r) { return r.text(); })
      .then(function (t) { return new Blob([t], { type: "text/plain" }); });
    if (window.ClipboardItem && navigator.clipboard && navigator.clipboard.write) {
      return navigator.clipboard.write([new ClipboardItem({ "text/plain": data })]);
    }
    return data.then(function (blob) { return blob.text(); })
      .then(function (t) { return navigator.clipboard.writeText(t); });
  }

  // applyFragments parses an HTML response and replaces every element in the
  // current document whose id matches a top-level element in the response. Used
  // for mutation responses that bundle a primary target with out-of-band swaps
  // (e.g. the refreshed zone plus a rail).
  function applyFragments(html) {
    var tpl = document.createElement("template");
    tpl.innerHTML = html.trim();
    Array.from(tpl.content.children).forEach(function (el) {
      if (!el.id) return;
      var existing = document.getElementById(el.id);
      if (existing) existing.replaceWith(el);
    });
    refreshBulk();
    equalizeHeaders();
  }

  async function postForm(url, body) {
    var res = await fetch(url, { method: "POST", body: body });
    if (!res.ok) return "";
    return await res.text();
  }

  document.addEventListener("change", function (e) {
    if (e.target.id === "select-all") {
      var list = rootList();
      if (list) list.querySelectorAll("input.select").forEach(function (cb) { cb.checked = e.target.checked; });
      refreshBulk();
    } else if (e.target.classList.contains("select")) {
      refreshBulk();
    }
  });

  document.addEventListener("click", async function (e) {
    var btn = e.target.closest("[data-action]");
    if (!btn) return;
    var action = btn.dataset.action;

    if (action === "copy") {
      copyFromUrl("/quotes/" + btn.dataset.id + "/copy").then(function () { flash(btn, "Copied"); }).catch(function () {});
    }

    if (action === "copy-all") {
      copyFromUrl(btn.dataset.export || "/export.txt").then(function () { flash(btn, btn.dataset.flash || "Copied all"); }).catch(function () {});
    }

    if (action === "new-collection") {
      var ids = selectedIds();
      if (ids.length === 0) return;
      var body = new URLSearchParams();
      ids.forEach(function (id) { body.append("id", id); });
      var html = await postForm("/collections" + ctxQuery(), body);
      if (html) applyFragments(html);
    }

    if (action === "insert-collection") {
      var cid = currentCol();
      if (!cid) return;
      var ins = selectedIds();
      if (ins.length === 0) return;
      var ibody = new URLSearchParams();
      ins.forEach(function (id) { ibody.append("id", id); });
      ibody.set("pos", btn.dataset.pos);
      var ihtml = await postForm("/collections/" + cid + "/insert" + ctxQuery(), ibody);
      if (ihtml) applyFragments(ihtml);
    }

    if (action === "bulk-delete") {
      var dels = selectedIds();
      if (dels.length === 0) return;
      if (!confirm("Delete " + dels.length + " quote(s)?")) return;
      var dbody = new URLSearchParams();
      dels.forEach(function (id) { dbody.append("id", id); });
      var del = await fetch("/quotes/delete", { method: "POST", body: dbody });
      if (del.ok) location.reload();
    }

    if (action === "merge-duplicates") {
      var rep = btn.dataset.id;
      var n = btn.dataset.count || "";
      if (!confirm("Merge " + n + " duplicate(s) into the shortest passage?")) return;
      var q = "?cat=" + currentCat() + "&col=" + currentCol() +
        "&rq=" + encodeURIComponent(inputValue("rq")) + "&cq=" + encodeURIComponent(inputValue("cq"));
      var res = await fetch("/duplicates/" + rep + "/merge" + q, { method: "POST" });
      var mhtml = res.ok ? await res.text() : "";
      if (mhtml) applyFragments(mhtml);
    }

    if (action === "cancel") {
      var slot = document.getElementById("form-slot");
      if (slot && slot.contains(btn)) { slot.innerHTML = ""; return; }
      location.reload();
    }

    if (action === "cancel-categories") {
      var chipsSlot = document.getElementById("chips-" + btn.dataset.id);
      fetch("/quotes/" + btn.dataset.id + "/categories")
        .then(function (r) { return r.text(); })
        .then(function (html) {
          var t = document.createElement("template");
          t.innerHTML = html.trim();
          var node = t.content.firstElementChild;
          if (chipsSlot && node) chipsSlot.replaceWith(node);
        });
    }

    if (action === "rename-category") { startRename(btn, "category"); }
    if (action === "rename-collection") { startRename(btn, "collection"); }

    if (action === "focus-quote") {
      // Jump to the first (representative) duplicate of its group. The link's
      // href ("/#quote-{id}") is the no-JS fallback; with JS we swap in place.
      e.preventDefault();
      var focusId = btn.dataset.id;
      var reveal = function () { flashQuote(focusId); };
      var zone = document.getElementById("root-zone");
      if (zone && zone.dataset.cat === "0" && document.getElementById("quote-" + focusId)) {
        reveal();
      } else {
        // Ensure Home (all quotes) is loaded, dropping any category filter so the
        // representative is guaranteed to be in the DOM.
        fetch("/pane/root?col=" + currentCol())
          .then(function (r) { return r.ok ? r.text() : ""; })
          .then(function (html) { if (html) { applyFragments(html); reveal(); } });
      }
    }
  });

  // flashQuote scrolls a quote into view and briefly highlights it, used both by
  // the duplicate "focus" action and after a re-sorting edit.
  function flashQuote(id) {
    var el = document.getElementById("quote-" + id);
    if (!el) return;
    el.scrollIntoView({ behavior: "smooth", block: "start" });
    el.classList.add("is-flash");
    setTimeout(function () { el.classList.remove("is-flash"); }, 1600);
  }

  // flashFromList highlights the quote a freshly swapped #quote-list was marked
  // with (set by an edit that may have re-sorted the block).
  function flashFromList() {
    var list = document.getElementById("quote-list");
    if (!list) return;
    var fid = list.dataset.flashId;
    if (fid) {
      list.removeAttribute("data-flash-id");
      flashQuote(fid);
    }
  }

  // Inline rename (category or collection): swap the link for an input, commit
  // on Enter/blur, then refresh the matching rail (and the collection zone if
  // the renamed collection is currently active).
  function startRename(btn, kind) {
    var row = btn.closest(".rail__row");
    if (!row || row.querySelector(".rail__rename")) return;
    var link = row.querySelector(".rail__link");
    if (!link) return;
    var id = btn.dataset.id;
    var name = btn.dataset.name || link.textContent.trim();
    var input = document.createElement("input");
    input.className = "rail__rename";
    input.type = "text";
    input.value = name;
    input.maxLength = 100;
    row.replaceChild(input, link);
    input.focus();
    input.select();
    var settled = false;
    function commit() {
      if (settled) return;
      settled = true;
      var val = input.value.trim();
      if (val && val !== name) {
        var body = new URLSearchParams();
        body.set("name", val);
        var url = (kind === "collection" ? "/collections/" : "/categories/") + id + "/rename" + ctxQuery();
        fetch(url, { method: "POST", body: body })
          .then(function (r) { return r.ok ? r.text() : ""; })
          .then(function (html) {
            if (html) {
              applyFragments(html);
              if (kind === "collection" && String(currentCol()) === String(id)) {
                refreshCollectionPane();
              }
            }
          });
      } else {
        refreshRail(kind);
      }
    }
    input.addEventListener("blur", commit);
    input.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter") { ev.preventDefault(); input.blur(); }
      else if (ev.key === "Escape") { settled = true; refreshRail(kind); }
    });
  }

  function refreshRail(kind) {
    fetch("/rail/" + kind + ctxQuery())
      .then(function (r) { return r.ok ? r.text() : ""; })
      .then(function (html) { if (html) applyFragments(html); });
  }

  function refreshCollectionPane() {
    fetch("/pane/collection" + ctxQuery())
      .then(function (r) { return r.ok ? r.text() : ""; })
      .then(function (html) { if (html) applyFragments(html); });
  }

  // Form submissions that return a rail (and possibly an OOB chip/zone).
  document.addEventListener("submit", async function (e) {
    var form = e.target.closest && e.target.closest("form[data-action]");
    if (!form) return;
    var action = form.dataset.action;

    if (action === "new-category") {
      e.preventDefault();
      var input = form.querySelector('input[name="name"]');
      var name = (input && input.value || "").trim();
      if (!name) return;
      var body = new URLSearchParams();
      body.set("name", name);
      var addBtn = form.querySelector('button[type="submit"]');
      var res = await fetch("/categories" + ctxQuery(), { method: "POST", body: body });
      if (res.ok) { applyFragments(await res.text()); form.reset(); }
      else if (res.status === 409 && addBtn) { flash(addBtn, "Exists"); }
    }

    if (action === "save-categories") {
      e.preventDefault();
      var qid = form.dataset.id;
      var body = new URLSearchParams(new FormData(form));
      var html = await postForm("/quotes/" + qid + "/categories" + ctxQuery(), body);
      if (html) applyFragments(html);
    }

    if (action === "restore") {
      e.preventDefault();
      var fileInput = form.querySelector('input[type="file"]');
      if (!fileInput || !fileInput.files || !fileInput.files[0]) return;
      if (!confirm("Replace the entire corpus with this backup? This cannot be undone.")) return;
      var text = await fileInput.files[0].text();
      var res = await fetch("/restore", { method: "POST", body: text, headers: { "Content-Type": "application/json" } });
      if (res.ok) location.reload();
      else flash(form.querySelector('button[type="submit"]'), "Failed");
    }
  });

  // Clear the create form after a successful add, so several can be added quickly.
  document.body.addEventListener("htmx:afterRequest", function (e) {
    var form = e.detail && e.detail.elt;
    if (form && form.tagName === "FORM" && form.getAttribute("hx-post") === "/quotes" && e.detail.successful) {
      form.reset();
    }
  });

  // After any htmx swap (zone/rail selection), re-evaluate selection state and
  // re-sync header row heights (a swap may replace a whole .zone section).
  document.body.addEventListener("htmx:afterSwap", function () {
    refreshBulk();
    equalizeHeaders();
    flashFromList();
  });

  // --- drag-and-drop reorder within the active collection ---
  var dragId = null;

  document.addEventListener("dragstart", function (e) {
    var art = e.target.closest && e.target.closest(".quote--ro");
    if (!art) return;
    dragId = art.dataset.id;
    art.classList.add("dragging");
    e.dataTransfer.effectAllowed = "move";
  });

  document.addEventListener("dragend", function (e) {
    var art = e.target.closest && e.target.closest(".quote--ro");
    if (art) art.classList.remove("dragging");
    dragId = null;
  });

  document.addEventListener("dragover", function (e) {
    if (!e.target.closest || !e.target.closest("#collection-list .quote--ro")) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  });

  document.addEventListener("drop", async function (e) {
    var target = e.target.closest && e.target.closest("#collection-list .quote--ro");
    var list = document.getElementById("collection-list");
    if (!target || !list || dragId === null) return;
    e.preventDefault();
    var dragged = list.querySelector('.quote--ro[data-id="' + dragId + '"]');
    if (!dragged || dragged === target) return;
    var rect = target.getBoundingClientRect();
    var after = e.clientY - rect.top > rect.height / 2;
    if (after) target.after(dragged); else target.before(dragged);
    var ids = Array.from(list.querySelectorAll(".quote--ro")).map(function (el) { return Number(el.dataset.id); });
    await fetch(list.dataset.reorder, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids: ids }),
    });
  });

  refreshBulk();
  equalizeHeaders();
  window.addEventListener("resize", equalizeHeaders);
  if (document.fonts && document.fonts.ready) document.fonts.ready.then(equalizeHeaders);
})();
