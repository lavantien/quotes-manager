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
    var target = document.getElementById("collection-target");
    if (target) target.disabled = !any;
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

  // swapSidebar replaces the sidebar with fresh HTML fetched from /sidebar, so
  // category changes anywhere refresh both sections (and the counts).
  function swapSidebar(html) {
    var aside = document.querySelector(".sidebar");
    if (aside) aside.outerHTML = html;
  }
  function refreshSidebar() {
    fetch("/sidebar")
      .then(function (r) { return r.ok ? r.text() : ""; })
      .then(function (html) { if (html) swapSidebar(html); });
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
      var target = document.getElementById("collection-target");
      var val = target ? target.value : "new";
      var body = new URLSearchParams();
      cids.forEach(function (id) { body.append("id", id); });
      var url = val === "new" ? "/collections" : "/collections/" + val + "/items";
      var res = await fetch(url, { method: "POST", body: body });
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

    if (action === "cancel-categories") {
      var chipsSlot = document.getElementById("chips-" + btn.dataset.id);
      fetch("/quotes/" + btn.dataset.id + "/categories")
        .then(function (r) { return r.text(); })
        .then(function (html) { if (chipsSlot) chipsSlot.outerHTML = html; });
    }

    if (action === "rename-category") {
      startRename(btn);
    }
  });

  // Inline category rename: swap the link for an input, commit on Enter/blur.
  function startRename(btn) {
    var cat = btn.closest(".sidebar__cat");
    if (!cat || cat.querySelector(".sidebar__rename")) return;
    var link = cat.querySelector(".sidebar__link");
    if (!link) return;
    var id = btn.dataset.id;
    var name = btn.dataset.name || link.textContent.trim();
    var input = document.createElement("input");
    input.className = "sidebar__rename";
    input.type = "text";
    input.value = name;
    input.maxLength = 100;
    cat.replaceChild(input, link);
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
        fetch("/categories/" + id + "/rename", { method: "POST", body: body })
          .then(function (r) { if (r.ok) r.text().then(swapSidebar); else refreshSidebar(); });
      } else {
        refreshSidebar();
      }
    }
    input.addEventListener("blur", commit);
    input.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter") { ev.preventDefault(); input.blur(); }
      else if (ev.key === "Escape") { settled = true; refreshSidebar(); }
    });
  }

  // Category mutations that return a fresh sidebar fragment.
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
      var res = await fetch("/categories", { method: "POST", body: body });
      if (res.ok) {
        swapSidebar(await res.text());
      } else if (addBtn) {
        flash(addBtn, "Exists");
      }
    }

    if (action === "save-categories") {
      e.preventDefault();
      var qid = form.dataset.id;
      var saveBody = new URLSearchParams(new FormData(form));
      var saveRes = await fetch("/quotes/" + qid + "/categories", { method: "POST", body: saveBody });
      if (saveRes.ok) {
        var html = await saveRes.text();
        var chips = document.getElementById("chips-" + qid);
        if (chips) chips.outerHTML = html;
        refreshSidebar();
      }
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
    if (!target || !ql || dragId === null || !ql.dataset.reorder) return;
    e.preventDefault();
    var dragged = ql.querySelector('.quote[data-id="' + dragId + '"]');
    if (!dragged || dragged === target) return;
    var rect = target.getBoundingClientRect();
    var after = e.clientY - rect.top > rect.height / 2;
    if (after) target.after(dragged); else target.before(dragged);
    var ids = Array.from(ql.querySelectorAll(".quote")).map(function (el) { return Number(el.dataset.id); });
    await fetch(ql.dataset.reorder, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids: ids }),
    });
  });
})();
