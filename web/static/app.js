(function () {
  "use strict";

  var list = () => document.getElementById("quote-list");

  function selectedIds() {
    return Array.from(document.querySelectorAll(".quote__check input.select:checked"))
      .map(function (cb) { return Number(cb.value); });
  }

  function refreshBulk() {
    var any = selectedIds().length > 0;
    document.querySelectorAll('[data-action="bulk-delete"], [data-action="add-collection"]')
      .forEach(function (b) { b.disabled = !any; });
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
  // Falls back to fetch-then-writeText where ClipboardItem is unavailable.
  function copyFromUrl(url) {
    var data = fetch(url).then(function (r) { return r.text(); })
      .then(function (t) { return new Blob([t], { type: "text/plain" }); });
    if (window.ClipboardItem && navigator.clipboard && navigator.clipboard.write) {
      return navigator.clipboard.write([new ClipboardItem({ "text/plain": data })]);
    }
    return data.then(function (blob) { return blob.text(); })
      .then(function (t) { return navigator.clipboard.writeText(t); });
  }

  document.addEventListener("change", function (e) {
    if (e.target.id === "select-all") {
      document.querySelectorAll(".quote__check input.select").forEach(function (cb) { cb.checked = e.target.checked; });
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
      copyFromUrl(btn.dataset.export || "/export.txt").then(function () { flash(btn, "Copied all"); }).catch(function () {});
    }

    if (action === "add-collection") {
      var cids = selectedIds();
      if (cids.length === 0) return;
      var body = new URLSearchParams();
      cids.forEach(function (id) { body.append("id", id); });
      var res = await fetch("/collections", { method: "POST", body: body });
      var loc = res.headers.get("HX-Redirect");
      if (res.ok && loc) location.href = loc;
    }

    if (action === "bulk-delete") {
      var ids = selectedIds();
      if (ids.length === 0) return;
      if (!confirm("Delete " + ids.length + " quote(s)?")) return;
      var body = new URLSearchParams();
      ids.forEach(function (id) { body.append("id", id); });
      var del = await fetch("/quotes/delete", { method: "POST", body: body });
      if (del.ok) location.reload();
    }

    if (action === "cancel") {
      var slot = document.getElementById("form-slot");
      if (slot && slot.contains(btn)) { slot.innerHTML = ""; return; }
      // Inline edit: the block was replaced by the form; reload to restore it.
      location.reload();
    }
  });

  // Clear the create form after a successful add, so several can be added quickly.
  document.body.addEventListener("htmx:afterRequest", function (e) {
    var form = e.detail && e.detail.elt;
    if (form && form.tagName === "FORM" && form.getAttribute("hx-post") === "/quotes" && e.detail.successful) {
      form.reset();
    }
  });

  // --- drag and drop reorder ---
  var dragId = null;

  document.addEventListener("dragstart", function (e) {
    var art = e.target.closest && e.target.closest(".quote");
    if (!art) return;
    dragId = art.dataset.id;
    art.classList.add("dragging");
    e.dataTransfer.effectAllowed = "move";
  });

  document.addEventListener("dragend", function (e) {
    var art = e.target.closest && e.target.closest(".quote");
    if (art) art.classList.remove("dragging");
    dragId = null;
  });

  document.addEventListener("dragover", function (e) {
    if (!e.target.closest || !e.target.closest(".quote")) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  });

  document.addEventListener("drop", async function (e) {
    var target = e.target.closest && e.target.closest(".quote");
    var ql = list();
    if (!target || !ql || dragId === null) return;
    e.preventDefault();
    var dragged = ql.querySelector('.quote[data-id="' + dragId + '"]');
    if (!dragged || dragged === target) return;
    var rect = target.getBoundingClientRect();
    var after = e.clientY - rect.top > rect.height / 2;
    if (after) target.after(dragged); else target.before(dragged);
    var ids = Array.from(ql.querySelectorAll(".quote")).map(function (el) { return Number(el.dataset.id); });
    await fetch("/quotes/reorder", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids: ids }),
    });
  });
})();
