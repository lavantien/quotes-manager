(function () {
  "use strict";

  var list = () => document.getElementById("quote-list");

  function selectedIds() {
    return Array.from(document.querySelectorAll(".quote__check input.select:checked"))
      .map(function (cb) { return Number(cb.value); });
  }

  function refreshBulk() {
    var btn = document.querySelector('[data-action="bulk-delete"]');
    if (btn) btn.disabled = selectedIds().length === 0;
  }

  function flash(btn, text) {
    var orig = btn.textContent;
    btn.textContent = text;
    btn.disabled = true;
    setTimeout(function () { btn.textContent = orig; btn.disabled = false; }, 1200);
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
      var res = await fetch("/quotes/" + btn.dataset.id + "/copy");
      if (res.ok) { await navigator.clipboard.writeText(await res.text()); flash(btn, "Copied"); }
    }

    if (action === "copy-all") {
      var r = await fetch("/export.txt");
      if (r.ok) { await navigator.clipboard.writeText(await r.text()); flash(btn, "Copied all"); }
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
