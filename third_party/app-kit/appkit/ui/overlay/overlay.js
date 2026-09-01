// appkit/ui/overlay — loopback-safe, dependency-free overlay behaviour for the
// kit's Dialog / Drawer / Toast primitives. Vanilla JS, no build step, no Node
// (ADR-0002), no framework bindings (ADR-0003 / D5). It only enhances markup the
// server already rendered: native <dialog> elements driven by data-* hooks.
//
//   <button data-overlay-open="my-dialog">Open</button>
//   <dialog id="my-dialog" data-overlay-light-dismiss> … <button data-overlay-dismiss>×</button> … </dialog>
//
// Registers window.AppKitOverlay = { open(id), close(id), toast(opts) }.
(function (w, d) {
  "use strict";

  function dialogById(id) {
    var el = d.getElementById(id);
    return el && el.tagName === "DIALOG" ? el : null;
  }
  function open(id) {
    var el = dialogById(id);
    if (el && typeof el.showModal === "function" && !el.open) el.showModal();
  }
  function close(id) {
    var el = dialogById(id);
    if (el && typeof el.close === "function" && el.open) el.close();
  }

  // Delegated triggers: open by id; dismiss the nearest enclosing dialog.
  d.addEventListener("click", function (e) {
    var opener = e.target.closest("[data-overlay-open]");
    if (opener) {
      e.preventDefault();
      open(opener.getAttribute("data-overlay-open"));
      return;
    }
    var dismiss = e.target.closest("[data-overlay-dismiss]");
    if (dismiss) {
      e.preventDefault();
      var dlg = dismiss.closest("dialog");
      if (dlg && typeof dlg.close === "function") dlg.close();
    }
  });

  // Light-dismiss: a click outside the panel of a [data-overlay-light-dismiss]
  // dialog closes it. (Native <dialog> also closes on Escape for free.)
  d.addEventListener("click", function (e) {
    var dlg = e.target;
    if (dlg.tagName !== "DIALOG" || !dlg.open) return;
    if (!dlg.hasAttribute("data-overlay-light-dismiss")) return;
    var panel = dlg.querySelector("[data-overlay-panel]");
    if (!panel) return;
    var r = panel.getBoundingClientRect();
    var inside =
      e.clientX >= r.left && e.clientX <= r.right &&
      e.clientY >= r.top && e.clientY <= r.bottom;
    if (!inside) dlg.close();
  });

  // Toast: append to the shared polite live-region (server-rendered when present,
  // else created on demand). Auto-dismiss after opts.timeout ms (default 4000;
  // 0 = sticky). Classes mirror the server-rendered toast so styling is identical.
  var TOAST_BASE =
    "rounded-md border px-4 py-3 text-sm shadow-lg max-w-sm";
  var TOAST_ROLE = {
    info: "border-info bg-info-soft text-ink",
    success: "border-positive bg-positive-soft text-ink",
    caution: "border-caution bg-caution-soft text-ink",
    danger: "border-danger bg-danger-soft text-ink",
  };
  function region() {
    var el = d.getElementById("ak-toast-region");
    if (!el) {
      el = d.createElement("div");
      el.id = "ak-toast-region";
      el.setAttribute("role", "status");
      el.setAttribute("aria-live", "polite");
      el.className = "fixed bottom-4 right-4 z-50 flex flex-col gap-2";
      d.body.appendChild(el);
    }
    return el;
  }
  function toast(opts) {
    opts = opts || {};
    var role = TOAST_ROLE[opts.role] ? opts.role : "info";
    var t = d.createElement("div");
    t.className = TOAST_BASE + " " + TOAST_ROLE[role];
    t.setAttribute("role", role === "danger" ? "alert" : "status");
    t.textContent = String(opts.message == null ? "" : opts.message);
    region().appendChild(t);
    var ms = typeof opts.timeout === "number" ? opts.timeout : 4000;
    if (ms > 0) w.setTimeout(function () { t.remove(); }, ms);
    return t;
  }

  w.AppKitOverlay = { open: open, close: close, toast: toast };
})(window, document);
