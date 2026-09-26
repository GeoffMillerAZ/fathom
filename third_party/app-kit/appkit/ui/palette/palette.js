// appkit/ui/palette — dependency-free command palette behaviour (L2). Classic
// script, no build step, no framework. It only enhances markup rendered by the Go
// package (palette.RenderTrigger / RenderDialog / RenderResults); without it the
// trigger is an ordinary link to the app's GET search page.
//
//   <a href="/search" data-palette-open="cmd">Search</a>
//   <dialog id="cmd"> … <div data-palette="cmd" data-palette-url="/search" …> … </dialog>
//
// Keyboard contract: the configured shortcut (Mod+K by default; Command on Apple
// platforms, Control elsewhere) opens the palette from anywhere and focuses its
// input. Typing fetches the results fragment after a debounce; a newer query
// aborts the older request and late responses are ignored. ArrowDown/ArrowUp move
// the aria-activedescendant selection (wrapping), Enter follows the selected link,
// Escape closes and focus returns to the invoker. Enter with no selection, or while
// the shown results belong to an older query, submits the native GET form.
//
// Registers window.AppKitPalette = { init(scope), open(id, invoker), close(id) }.
// init(scope) wires every [data-palette] root in scope (the root itself, any
// container, or the document by default), forgets palettes no longer in the
// document and re-upgrades every trigger; call it after inserting, replacing or
// removing palette or trigger markup. Triggers are rendered without a shortcut
// hint or aria-keyshortcuts; only a trigger whose palette is mounted gets them.
(function (w, d) {
  "use strict";
  if (w.AppKitPalette) return;

  var HEADER = "X-AppKit-Palette";
  var MAX_CHARS = 512 * 1024;
  var instances = []; // wired palettes, in wiring order
  var byId = {};

  function apple() {
    var nav = w.navigator || {};
    var platform = (nav.userAgentData && nav.userAgentData.platform) || nav.platform || "";
    return /mac|iphone|ipad|ipod/i.test(platform);
  }

  // parseShortcut turns "mod+shift+k" into modifier flags plus one key.
  function parseShortcut(spec) {
    if (!spec || spec === "none") return null;
    var parts = String(spec).toLowerCase().split("+");
    var s = { key: parts.pop(), ctrl: false, meta: false, alt: false, shift: false, mods: parts };
    for (var i = 0; i < parts.length; i++) {
      var m = parts[i];
      if (m === "mod") {
        if (apple()) s.meta = true; else s.ctrl = true;
      } else if (m === "ctrl" || m === "meta" || m === "alt" || m === "shift") {
        s[m] = true;
      } else {
        return null;
      }
    }
    return s.key ? s : null;
  }

  function keyMatches(e, key) {
    var typed = String(e.key || "").toLowerCase();
    if (typed === key) return true;
    // A Latin letter or digit is the layout's own answer: Dvorak's physical K
    // types "t", so Mod+T is not Mod+K. The physical key stands in only when the
    // key produced something else (a non-Latin letter, an Option-modified symbol
    // or a shifted digit).
    if (/^[a-z0-9]$/.test(typed)) return false;
    var code = e.code || "";
    if (/^[a-z]$/.test(key)) return code === "Key" + key.toUpperCase();
    if (/^[0-9]$/.test(key)) return code === "Digit" + key;
    return key === "/" && code === "Slash";
  }

  function shortcutMatches(e, s) {
    return !!s && !!e.metaKey === s.meta && !!e.ctrlKey === s.ctrl &&
      !!e.altKey === s.alt && !!e.shiftKey === s.shift && keyMatches(e, s.key);
  }

  function editable(el) {
    if (!el) return false;
    if (el.isContentEditable) return true;
    var tag = el.tagName;
    return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
  }

  // hint and ariaKeys localize the server-rendered (non-Apple) shortcut text.
  function hint(s) {
    var key = s.key.toUpperCase();
    if (apple()) {
      var sym = "";
      if (s.ctrl) sym += "⌃";
      if (s.alt) sym += "⌥";
      if (s.shift) sym += "⇧";
      if (s.meta) sym += "⌘";
      return sym + key;
    }
    var out = [];
    if (s.ctrl) out.push("Ctrl");
    if (s.alt) out.push("Alt");
    if (s.shift) out.push("Shift");
    if (s.meta) out.push("Meta");
    out.push(key);
    return out.join("+");
  }
  function ariaKeys(s) {
    var out = [];
    if (s.ctrl) out.push("Control");
    if (s.alt) out.push("Alt");
    if (s.shift) out.push("Shift");
    if (s.meta) out.push("Meta");
    out.push(s.key === "/" ? "Slash" : s.key.toUpperCase());
    return out.join("+");
  }

  // connected is false once swapped-out markup has left the document.
  function connected(p) {
    return p.root.isConnected !== false && p.dialog.isConnected !== false;
  }

  function num(v, fallback) {
    var n = parseInt(v, 10);
    return isFinite(n) && n > 0 ? n : fallback;
  }

  function Palette(root) {
    this.root = root;
    this.dialog = root.closest("dialog");
    this.id = root.getAttribute("data-palette");
    this.input = root.querySelector("[data-palette-input]");
    this.form = root.querySelector("[data-palette-form]");
    this.results = root.querySelector("[data-palette-results]");
    this.status = root.querySelector("[data-palette-status]");
    this.busy = root.querySelector("[data-palette-busy]");
    this.errorTemplate = root.querySelector("template[data-palette-error]");
    this.action = root.getAttribute("data-palette-action") || "";
    this.url = root.getAttribute("data-palette-url") || this.action;
    this.param = root.getAttribute("data-palette-param") || "q";
    this.debounce = num(root.getAttribute("data-palette-debounce"), 150);
    this.timeout = num(root.getAttribute("data-palette-timeout"), 8000);
    this.label = root.getAttribute("data-palette-label") || "Search";
    this.shortcut = parseShortcut(root.getAttribute("data-palette-shortcut"));
    var first = this.results && this.results.firstElementChild;
    this.idle = first ? first.cloneNode(true) : null;
    this.seq = 0;         // identity of the newest request; older responses are ignored
    this.timer = 0;       // pending debounce
    this.inflight = null; // AbortController of the newest request, or true without AbortController
    this.options = [];
    this.active = -1;
    this.shown = "";      // the query the displayed fragment answers
    this.invoker = null;
    this.announceTimer = 0;
  }

  var P = Palette.prototype;

  P.query = function () {
    return String(this.input.value || "").trim();
  };

  P.fresh = function () {
    return !this.timer && !this.inflight && this.shown === this.query();
  };

  P.setBusy = function () {
    if (this.busy) this.busy.hidden = !(this.timer || this.inflight);
    if (this.results) {
      if (this.timer || this.inflight) this.results.setAttribute("aria-busy", "true");
      else this.results.removeAttribute("aria-busy");
    }
  };

  P.cancel = function () {
    if (this.timer) w.clearTimeout(this.timer);
    this.timer = 0;
    this.seq++;
    if (this.inflight && this.inflight.abort) this.inflight.abort();
    this.inflight = null;
    this.setBusy();
  };

  // syncPageLink keeps the footer's search-page link (outside the data-palette
  // body) on the current query, preserving the action's own context parameters.
  P.syncPageLink = function () {
    var link = this.dialog.querySelector("[data-palette-page]");
    if (!link || !this.action) return;
    var target;
    try {
      target = new w.URL(this.action, d.baseURI);
    } catch (err) {
      return;
    }
    var q = this.query();
    if (q) target.searchParams.set(this.param, q);
    link.setAttribute("href", target.origin === w.location.origin ? target.pathname + target.search : this.action);
  };

  P.schedule = function () {
    var self = this;
    this.syncPageLink();
    var q = this.query();
    // Any in-flight answer is for an older query now: abort it before debouncing.
    this.cancel();
    if (q === "") {
      if (this.idle) this.show(this.idle.cloneNode(true), "");
      return;
    }
    if (q === this.shown) return; // the displayed fragment already answers q
    this.timer = w.setTimeout(function () {
      self.timer = 0;
      self.request(q);
    }, this.debounce);
    this.setBusy();
  };

  P.request = function (q) {
    var self = this;
    if (this.inflight && this.inflight.abort) this.inflight.abort(); // cancel the stale request
    var seq = ++this.seq;
    var controller = typeof w.AbortController === "function" ? new w.AbortController() : null;
    this.inflight = controller || true;
    this.setBusy();
    var target = null;
    try {
      target = new w.URL(this.url, d.baseURI);
    } catch (err) {
      target = null;
    }
    if (!target || target.origin !== w.location.origin || typeof w.fetch !== "function") {
      this.fail(q, seq); // cross-origin fragments are refused, never inserted
      return;
    }
    target.searchParams.set(this.param, q);
    var timedOut = w.setTimeout(function () {
      if (controller) controller.abort();
      self.fail(q, seq);
    }, this.timeout);
    var init = {
      method: "GET",
      credentials: "same-origin",
      headers: { Accept: "text/html" },
    };
    init.headers[HEADER] = "1";
    if (controller) init.signal = controller.signal;
    w.fetch(target.href, init).then(function (res) {
      if (!res.ok) throw new Error("status " + res.status);
      return res.text();
    }).then(function (text) {
      w.clearTimeout(timedOut);
      if (seq !== self.seq) return; // superseded or closed
      var fragment = self.parse(text, q);
      if (!fragment) throw new Error("no results fragment");
      self.inflight = null;
      self.show(fragment, q);
    }).catch(function () {
      w.clearTimeout(timedOut);
      self.fail(q, seq);
    });
  };

  // parse accepts only a response that is exactly one palette fragment answering
  // q: whitespace and comments may surround it, nothing else. A full page is a
  // failure even when it renders this palette (and so carries its idle fragment),
  // as is a fragment for another query or an oversized body.
  P.parse = function (text, q) {
    if (typeof text !== "string" || text.length > MAX_CHARS) return null;
    var tpl = d.createElement("template");
    tpl.innerHTML = text;
    var nodes = tpl.content ? tpl.content.childNodes : [];
    var root = null;
    for (var i = 0; i < nodes.length; i++) {
      var n = nodes[i];
      if (n.nodeType === 8) continue; // comment
      if (n.nodeType === 3 && !/\S/.test(n.nodeValue || "")) continue; // whitespace
      if (n.nodeType !== 1 || root) return null;
      root = n;
    }
    if (!root || !root.hasAttribute("data-palette-fragment")) return null;
    return root.getAttribute("data-palette-query") === q ? root : null;
  };

  P.fail = function (q, seq) {
    // Superseded, closed or already answered (a timeout then its abort): ignore.
    if (seq !== this.seq || !this.inflight) return;
    this.inflight = null;
    var node = this.errorTemplate && this.errorTemplate.content
      ? this.errorTemplate.content.querySelector("[data-palette-fragment]")
      : null;
    if (!node) {
      this.setBusy();
      return;
    }
    node = node.cloneNode(true);
    node.setAttribute("data-palette-query", q);
    this.show(node, q);
  };

  P.show = function (fragment, q) {
    var r = this.results;
    while (r.firstChild) r.removeChild(r.firstChild);
    r.appendChild(fragment);
    this.shown = q;
    this.upgrade(fragment);
    this.setBusy();
    this.announce(fragment.getAttribute("data-palette-announce") || "");
  };

  // upgrade gives the server-rendered headings/lists/links combobox semantics.
  P.upgrade = function (fragment) {
    var id = this.id;
    var list = fragment.querySelector("[data-palette-list]");
    var options = [];
    if (list) {
      list.setAttribute("role", "listbox");
      list.id = id + "-listbox";
      list.setAttribute("aria-label", this.label);
      var groups = list.querySelectorAll("[data-palette-group]");
      for (var g = 0; g < groups.length; g++) {
        var group = groups[g];
        group.setAttribute("role", "group");
        var heading = group.querySelector("h2,h3,h4,h5,h6");
        if (heading) {
          heading.id = id + "-group-" + g;
          heading.setAttribute("role", "presentation");
          group.setAttribute("aria-labelledby", heading.id);
        }
        var wrappers = group.querySelectorAll("ul,li");
        for (var k = 0; k < wrappers.length; k++) wrappers[k].setAttribute("role", "none");
      }
      var links = list.querySelectorAll("[data-palette-option]");
      for (var i = 0; i < links.length; i++) {
        var o = links[i];
        o.id = id + "-option-" + i;
        o.setAttribute("role", "option");
        o.setAttribute("aria-selected", "false");
        o.setAttribute("tabindex", "-1");
        options.push(o);
      }
    }
    this.options = options;
    this.active = -1;
    if (options.length) {
      this.input.setAttribute("aria-controls", id + "-listbox");
      this.input.setAttribute("aria-expanded", "true");
      this.select(0, false);
    } else {
      this.input.setAttribute("aria-controls", this.results.id);
      this.input.setAttribute("aria-expanded", "false");
      this.input.removeAttribute("aria-activedescendant");
    }
  };

  P.select = function (index, scroll) {
    if (this.active >= 0 && this.options[this.active]) {
      this.options[this.active].setAttribute("aria-selected", "false");
    }
    this.active = index;
    var o = this.options[index];
    if (!o) {
      this.input.removeAttribute("aria-activedescendant");
      return;
    }
    o.setAttribute("aria-selected", "true");
    this.input.setAttribute("aria-activedescendant", o.id);
    if (scroll && o.scrollIntoView) o.scrollIntoView({ block: "nearest" });
  };

  P.announce = function (text) {
    var s = this.status;
    if (!s) return;
    if (this.announceTimer) w.clearTimeout(this.announceTimer);
    this.announceTimer = 0;
    s.textContent = "";
    if (!text) return;
    // Clear first so an identical message is announced again.
    this.announceTimer = w.setTimeout(function () { s.textContent = text; }, 30);
  };

  P.onKey = function (e) {
    if (e.isComposing || e.keyCode === 229) return;
    var n = this.options.length;
    switch (e.key) {
      case "ArrowDown":
        if (!n) return;
        e.preventDefault();
        this.select(this.active < n - 1 ? this.active + 1 : 0, true);
        return;
      case "ArrowUp":
        if (!n) return;
        e.preventDefault();
        this.select(this.active > 0 ? this.active - 1 : n - 1, true);
        return;
      case "Enter":
        if (e.shiftKey || e.altKey || e.metaKey || e.ctrlKey) return;
        var o = this.options[this.active];
        if (o && this.fresh()) {
          e.preventDefault();
          this.follow(o);
        }
        // Otherwise the native GET form submits to the full search page.
        return;
      case "Escape":
        e.preventDefault();
        this.close();
        return;
    }
  };

  P.follow = function (option) {
    this.close();
    option.click();
  };

  P.open = function (invoker) {
    var dlg = this.dialog;
    // A palette whose markup was swapped out cannot open (showModal would throw).
    if (!dlg || typeof dlg.showModal !== "function" || !connected(this)) return false;
    if (!dlg.open) {
      this.invoker = invoker || d.activeElement;
      dlg.showModal();
      if (this.query() !== this.shown) this.schedule();
    }
    this.input.focus();
    if (this.input.select) this.input.select();
    return true;
  };

  P.close = function () {
    if (this.dialog && this.dialog.open) this.dialog.close();
  };

  P.onClose = function () {
    this.cancel();
    var back = this.invoker;
    this.invoker = null;
    if (back && back !== d.body && back.isConnected !== false && typeof back.focus === "function") {
      back.focus();
    }
  };

  P.onClick = function (e) {
    var t = e.target;
    var dismiss = t && t.closest ? t.closest("[data-overlay-dismiss]") : null;
    if (dismiss && this.dialog.contains(dismiss)) {
      e.preventDefault();
      this.close();
      return;
    }
    if (t === this.dialog) {
      var panel = this.dialog.querySelector("[data-overlay-panel]");
      if (!panel) return;
      var r = panel.getBoundingClientRect();
      if (e.clientX < r.left || e.clientX > r.right || e.clientY < r.top || e.clientY > r.bottom) this.close();
      return;
    }
    var option = t && t.closest ? t.closest("[data-palette-option]") : null;
    if (option && this.results.contains(option) && !e.defaultPrevented &&
        e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey) {
      this.close(); // the link's own navigation continues
    }
  };

  P.onPointer = function (e) {
    var option = e.target && e.target.closest ? e.target.closest("[data-palette-option]") : null;
    if (!option) return;
    var i = this.options.indexOf(option);
    if (i >= 0 && i !== this.active) this.select(i, false);
  };

  P.wire = function () {
    var self = this;
    if (this.dialog && !this.dialog.getAttribute("aria-label") && !this.dialog.getAttribute("aria-labelledby")) {
      this.dialog.setAttribute("aria-label", this.label);
    }
    this.input.addEventListener("input", function () { self.schedule(); });
    this.input.addEventListener("keydown", function (e) { self.onKey(e); });
    this.results.addEventListener("mousemove", function (e) { self.onPointer(e); });
    this.dialog.addEventListener("click", function (e) { self.onClick(e); });
    this.dialog.addEventListener("close", function () { self.onClose(); });
  };

  // prune forgets palettes whose markup has left the document, so a swapped-out
  // instance can neither intercept the shortcut nor answer open(id).
  function prune() {
    var kept = [];
    for (var i = 0; i < instances.length; i++) {
      var p = instances[i];
      if (connected(p)) kept.push(p);
      else if (byId[p.id] === p) delete byId[p.id];
    }
    instances = kept;
  }

  // upgradeTriggers localizes every trigger whose palette is mounted: the popup
  // relationship, aria-keyshortcuts and the visible hint in platform notation.
  // A trigger without a mounted palette (or with the shortcut disabled) promises
  // no shortcut: it is only a link to the search page.
  function upgradeTriggers() {
    var triggers = d.querySelectorAll("[data-palette-open]");
    for (var i = 0; i < triggers.length; i++) {
      var t = triggers[i];
      var p = byId[t.getAttribute("data-palette-open")];
      var h = t.querySelector("[data-palette-hint]");
      if (p) t.setAttribute("aria-haspopup", "dialog");
      else t.removeAttribute("aria-haspopup");
      if (p && p.shortcut) {
        t.setAttribute("aria-keyshortcuts", ariaKeys(p.shortcut));
        if (h) {
          h.textContent = hint(p.shortcut);
          h.hidden = false;
        }
      } else {
        t.removeAttribute("aria-keyshortcuts");
        if (h) h.hidden = true;
      }
    }
  }

  function init(scope) {
    var s = scope || d;
    var roots = [];
    if (s.matches && s.matches("[data-palette]")) roots.push(s);
    var found = s.querySelectorAll ? s.querySelectorAll("[data-palette]") : [];
    for (var j = 0; j < found.length; j++) roots.push(found[j]);
    prune();
    for (var i = 0; i < roots.length; i++) {
      var root = roots[i];
      if (root.__appkitPalette) continue;
      var p = new Palette(root);
      if (!p.dialog || !p.id || !p.input || !p.results || !connected(p)) continue;
      root.__appkitPalette = p;
      p.wire();
      instances.push(p);
      byId[p.id] = p;
    }
    upgradeTriggers();
  }

  d.addEventListener("click", function (e) {
    var t = e.target && e.target.closest ? e.target.closest("[data-palette-open]") : null;
    if (!t || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    var p = byId[t.getAttribute("data-palette-open")];
    if (p && p.open(t)) e.preventDefault(); // otherwise the link reaches the search page
  });

  d.addEventListener("keydown", function (e) {
    if (e.defaultPrevented || e.isComposing) return;
    for (var i = 0; i < instances.length; i++) {
      var p = instances[i];
      var s = p.shortcut;
      if (!s || !connected(p) || !shortcutMatches(e, s)) continue;
      if (!s.ctrl && !s.meta && !s.alt && editable(e.target)) continue;
      if (p.open(d.activeElement)) e.preventDefault();
      return;
    }
  });

  if (d.readyState === "loading") {
    d.addEventListener("DOMContentLoaded", function () { init(d); });
  } else {
    init(d);
  }

  w.AppKitPalette = {
    init: init,
    open: function (id, invoker) {
      var p = byId[id];
      return p ? p.open(invoker) : false;
    },
    close: function (id) {
      var p = byId[id];
      if (p) p.close();
    },
  };
})(window, document);
