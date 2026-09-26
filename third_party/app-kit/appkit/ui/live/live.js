// appkit/ui/live — dependency-free live-freshness subscriber (L2).
//
// It reads an app-owned server-sent event stream and marks server-rendered
// regions as having newer data. It never fetches, replaces or moves region
// content: a matched region only swaps its reserved status line from "As of …"
// to "As of … · Updated — refresh this panel", and a polite live region says so.
//
//   <div data-live-page data-live-url="/events" data-live-events="change">…</div>
//   <section id="plans" data-live-source="plans:fathom" data-live-label="Plans">
//     … <span data-live-asof>…</span><span data-live-updated>… <a data-live-refresh href="#plans">…</a></span>
//   </section>
//
// Registers window.AppKitLive = {version, start(options), stop(), state(), mark(source)}.
// data-live-last-event-id (or start({lastEventId})) is the render-time cursor; an
// explicit start without lastEventId adopts it from the page status naming its URL.
// Events: window "appkit:live-status" (detail: state) and, on a region,
// "appkit:live-updated" (bubbles; detail: {source, id, eventId}).
(function (w, d) {
  "use strict";
  if (w.AppKitLive) return; // loading twice is harmless

  var LIMITS = {
    maxAttempts: [1, 50, 8],
    baseDelay: [100, 60000, 1000],
    maxDelay: [100, 300000, 30000],
    idleTimeout: [0, 600000, 60000],
    hiddenGrace: [0, 600000, 5000],
    announceDelay: [0, 10000, 1000],
    maxEventBytes: [256, 1048576, 65536],
  };
  var MAX_SOURCES_PER_EVENT = 64;
  var MAX_TRACKED = 256;
  var MAX_ID_BYTES = 256;
  // A connection proves itself by staying open this long (or by an event that
  // advances the cursor) before it earns a fresh retry budget.
  var STABLE_AFTER = 10000;
  var FOCUS_KEY = "appkit-live-focus";
  var ACTIVE = { connecting: 1, live: 1, reconnecting: 1, paused: 1 };
  // Every connection-line message. RenderPageStatus stacks each one invisibly
  // in the line's grid cell (live.go connectionMessages, with {attempt} and {max}
  // at their widest), so the line keeps its tallest message's height and no state
  // change moves content. Keep the two lists identical.
  var MESSAGES = {
    connecting: "Connecting to live updates…",
    live: "Live updates on.",
    reconnecting: "Reconnecting (attempt {attempt} of {max})…",
    paused: "Paused while this page is hidden.",
    stopped: "Live updates stopped. Reload for newer data.",
    unavailable: "Live updates unavailable. Reload to retry.",
    unusable: "Live updates unavailable. Reload for newer data.",
  };

  var config = null;
  var status = "idle";
  var error = null;
  var attempt = 0;
  var generation = 0;
  var controller = null;
  var lastEventId = "";
  var connectedFrom = ""; // the cursor this connection resumed from
  var serverRetry = 0;
  var parsed = d.readyState !== "loading"; // regions and page status are in the DOM
  var deferred = false; // start() ran before the document was parsed
  var adoptCursor = false; // take the page status cursor once it is parsed
  var updated = [];
  var counts = { ignored: 0, unmatched: 0, dropped: 0 };
  var timers = {};
  var pendingLabels = [];

  // ── configuration ─────────────────────────────────────────────────────────
  function bounded(name, value) {
    var l = LIMITS[name];
    if (value === undefined || value === null) return l[2];
    if (typeof value !== "number" || !isFinite(value) || Math.floor(value) !== value || value < l[0] || value > l[1]) {
      throw new TypeError("AppKitLive: " + name + " must be an integer from " + l[0] + " to " + l[1]);
    }
    return value;
  }
  function eventNames(list) {
    if (list === undefined || list === null || list.length === 0) return ["message"];
    if (!Array.isArray(list) || list.length > 16) throw new TypeError("AppKitLive: events must be an array of at most 16 names");
    for (var i = 0; i < list.length; i++) {
      if (typeof list[i] !== "string" || !/^[A-Za-z][A-Za-z0-9._:-]{0,63}$/.test(list[i])) {
        throw new TypeError("AppKitLive: invalid event name");
      }
    }
    return list.slice();
  }
  // Header values are byte strings: fetch refuses a character above U+00FF and
  // sends U+0080–U+00FF as single Latin-1 bytes. Send the id's UTF-8 bytes, one
  // per character, as the browser's native SSE client does. Throws URIError for
  // a lone surrogate, which has no UTF-8 form.
  function utf8Bytes(id) {
    return encodeURIComponent(id).replace(/%([0-9A-F]{2})/g, function (_, hex) {
      return String.fromCharCode(parseInt(hex, 16));
    });
  }
  // One line of at most MAX_ID_BYTES UTF-8 bytes, unpadded (fetch strips a
  // leading or trailing space or tab from header values).
  function validCursor(id) {
    if (typeof id !== "string" || /[\u0000\r\n]|^[ \t]|[ \t]$/.test(id)) return false;
    try {
      return utf8Bytes(id).length <= MAX_ID_BYTES;
    } catch (e) {
      return false;
    }
  }
  function configure(o) {
    if (typeof o.url !== "string" || o.url === "" || o.url.indexOf("#") !== -1) {
      throw new TypeError("AppKitLive: url must be a nonempty stream URL without a fragment");
    }
    if (o.sources !== undefined && typeof o.sources !== "function") {
      throw new TypeError("AppKitLive: sources must be a function");
    }
    if (o.lastEventId !== undefined && !validCursor(o.lastEventId)) {
      throw new TypeError("AppKitLive: lastEventId must be one unpadded line of at most " + MAX_ID_BYTES + " UTF-8 bytes");
    }
    var c = {
      url: new w.URL(o.url, w.location.href).href,
      events: eventNames(o.events),
      sources: o.sources || null,
    };
    for (var name in LIMITS) c[name] = bounded(name, o[name]);
    if (c.maxDelay < c.baseDelay) throw new TypeError("AppKitLive: maxDelay must not be below baseDelay");
    return c;
  }

  // ── timers and state ──────────────────────────────────────────────────────
  function later(name, ms, fn) {
    clear(name);
    timers[name] = w.setTimeout(function () {
      delete timers[name];
      fn();
    }, ms);
  }
  function clear(name) {
    if (timers[name] !== undefined) {
      w.clearTimeout(timers[name]);
      delete timers[name];
    }
  }
  function snapshot() {
    return {
      status: status,
      error: error,
      attempt: attempt,
      maxAttempts: config ? config.maxAttempts : LIMITS.maxAttempts[2],
      url: config ? config.url : "",
      lastEventId: lastEventId,
      updated: updated.slice(),
      ignored: counts.ignored,
      unmatched: counts.unmatched,
      dropped: counts.dropped,
    };
  }
  function emit(target, type, detail) {
    try {
      target.dispatchEvent(new w.CustomEvent(type, { bubbles: true, detail: detail }));
    } catch (e) {
      /* an app listener error must not stop the subscriber */
    }
  }
  var emittedAttempt = 0;
  function setStatus(next, code) {
    var changed = next !== status || (code || null) !== error || attempt !== emittedAttempt;
    status = next;
    error = code || null;
    emittedAttempt = attempt;
    render();
    if (changed) emit(w, "appkit:live-status", snapshot());
  }
  // Resuming cannot help without a configuration or a capable browser.
  function resumable() {
    return !!config && error !== "unsupported";
  }
  function connectionText() {
    switch (status) {
      case "reconnecting":
        return MESSAGES.reconnecting.replace("{attempt}", attempt).replace("{max}", config.maxAttempts);
      case "unavailable":
        return resumable() ? MESSAGES.unavailable : MESSAGES.unusable;
      default:
        return MESSAGES[status] || null;
    }
  }
  function render() {
    var text = connectionText();
    var i, nodes;
    if (text !== null) {
      nodes = d.querySelectorAll("[data-live-connection]");
      for (i = 0; i < nodes.length; i++) nodes[i].textContent = text;
    }
    nodes = d.querySelectorAll("[data-live-page]");
    for (i = 0; i < nodes.length; i++) nodes[i].setAttribute("data-live-status", status);
    var running = !!ACTIVE[status];
    // The control keeps its box while unusable (visibility, not display), so
    // showing it moves nothing; a control that cannot act is never offered.
    var usable = status !== "idle" && (running || resumable());
    nodes = d.querySelectorAll("[data-live-toggle]");
    for (i = 0; i < nodes.length; i++) {
      if (usable) nodes[i].hidden = false;
      nodes[i].style.visibility = usable ? "visible" : "hidden";
      var label = nodes[i].querySelector("[data-live-toggle-label]") || nodes[i];
      label.textContent = running ? "Stop live updates" : "Resume live updates";
    }
  }
  function announce(message) {
    var el = d.querySelector("[data-live-announcer]");
    if (!el) {
      el = d.createElement("div");
      el.setAttribute("data-live-announcer", "");
      el.setAttribute("role", "status");
      el.setAttribute("aria-live", "polite");
      el.setAttribute("aria-atomic", "true");
      // Inline visually-hidden styles: no dependency on a consumer CSS build.
      el.style.cssText = "position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0";
      d.body.appendChild(el);
    }
    el.textContent = "";
    later("announceWrite", 50, function () {
      el.textContent = message;
    });
  }
  function flushAnnouncement() {
    var labels = pendingLabels;
    pendingLabels = [];
    if (labels.length === 0) return;
    if (labels.length === 1) {
      announce(labels[0] + " has newer data. Use its refresh link to see it.");
      return;
    }
    var shown = labels.slice(0, 3).join(", ");
    var more = labels.length > 3 ? " and " + (labels.length - 3) + " more" : "";
    announce(labels.length + " panels have newer data: " + shown + more + ".");
  }

  // ── matching ──────────────────────────────────────────────────────────────
  function ownedBy(node, region) {
    return node.closest("[data-live-source]") === region;
  }
  function markRegion(region, source, eventId) {
    if (region.getAttribute("data-live-state") === "updated") return false;
    region.setAttribute("data-live-state", "updated");
    var i, nodes = region.querySelectorAll("[data-live-asof]");
    for (i = 0; i < nodes.length; i++) if (ownedBy(nodes[i], region)) nodes[i].style.visibility = "hidden";
    nodes = region.querySelectorAll("[data-live-updated]");
    for (i = 0; i < nodes.length; i++) if (ownedBy(nodes[i], region)) nodes[i].style.visibility = "visible";
    if (updated.length < MAX_TRACKED) updated.push(region.id || "");
    var label = region.getAttribute("data-live-label") || region.id || "A panel";
    if (pendingLabels.length < MAX_TRACKED) pendingLabels.push(label);
    later("announce", config ? config.announceDelay : LIMITS.announceDelay[2], flushAnnouncement);
    emit(region, "appkit:live-updated", { source: source, id: region.id || "", eventId: eventId });
    return true;
  }
  // Returns how many regions follow source; already-updated regions still count.
  function markSource(source, eventId) {
    if (typeof source !== "string" || source === "") return 0;
    var regions = d.querySelectorAll("[data-live-source]");
    var matched = 0;
    for (var i = 0; i < regions.length; i++) {
      var tokens = (regions[i].getAttribute("data-live-source") || "").split(/\s+/);
      if (source === "*" || tokens.indexOf(source) !== -1) {
        matched++;
        markRegion(regions[i], source, eventId);
      }
    }
    return matched;
  }
  function defaultSources(message) {
    var value = JSON.parse(message.data);
    if (value && typeof value.source === "string") return [value.source];
    if (value && Array.isArray(value.sources)) return value.sources;
    throw new Error("no sources");
  }
  function handle(message) {
    var list;
    try {
      list = config.sources ? config.sources(message) : defaultSources(message);
    } catch (e) {
      counts.ignored++;
      return;
    }
    if (!Array.isArray(list)) {
      counts.ignored++;
      return;
    }
    var matched = 0;
    for (var i = 0; i < list.length && i < MAX_SOURCES_PER_EVENT; i++) matched += markSource(list[i], message.lastEventId);
    if (matched === 0) counts.unmatched++;
    render();
  }

  // ── server-sent event parsing (WHATWG field rules) ────────────────────────
  var parser;
  function resetParser() {
    parser = { pending: "", skipLF: false, bom: false, data: "", size: 0, type: "", id: lastEventId, oversize: false };
  }
  function dispatch() {
    lastEventId = parser.id;
    var data = parser.data, type = parser.type || "message", oversize = parser.oversize;
    // An event that moves the cursor proves the stream works: fresh retry budget.
    if ((data !== "" || oversize) && lastEventId !== connectedFrom) attempt = 0;
    parser.data = "";
    parser.size = 0;
    parser.type = "";
    parser.oversize = false;
    if (oversize) {
      counts.dropped++;
      return;
    }
    if (data === "") return;
    if (config.events.indexOf(type) === -1) return;
    handle({ type: type, data: data.slice(0, -1), lastEventId: lastEventId });
  }
  // Lines alone (a preamble, a comment, a retry field) earn no fresh retry
  // budget, so a server that accepts and then closes still exhausts it.
  function field(line) {
    if (line === "") return dispatch();
    if (line.charCodeAt(0) === 58) return; // ":" comment, e.g. a keepalive
    var colon = line.indexOf(":");
    var name = colon === -1 ? line : line.slice(0, colon);
    var value = colon === -1 ? "" : line.slice(colon + 1);
    if (value.charCodeAt(0) === 32) value = value.slice(1);
    if (name === "event") parser.type = value;
    else if (name === "data") {
      if (parser.size + value.length + 1 > config.maxEventBytes) parser.oversize = true;
      else {
        parser.data += value + "\n";
        parser.size += value.length + 1;
      }
    } else if (name === "id") {
      if (value.indexOf("\u0000") === -1) parser.id = value;
    } else if (name === "retry") {
      if (/^[0-9]+$/.test(value)) serverRetry = parseInt(value, 10);
    }
  }
  // Returns false when the stream violates the line bound.
  function feed(text) {
    if (!parser.bom) {
      parser.bom = true;
      if (text.charCodeAt(0) === 0xfeff) text = text.slice(1);
    }
    var start = 0;
    for (var i = 0; i < text.length; i++) {
      var c = text.charCodeAt(i);
      if (c === 10 && parser.skipLF) {
        parser.skipLF = false;
        start = i + 1;
        continue;
      }
      parser.skipLF = false;
      if (c === 13 || c === 10) {
        var line = parser.pending + text.slice(start, i);
        parser.pending = "";
        field(line);
        if (c === 13) parser.skipLF = true;
        start = i + 1;
      }
    }
    parser.pending += text.slice(start);
    return parser.pending.length <= config.maxEventBytes;
  }

  // ── connection lifecycle ──────────────────────────────────────────────────
  function abort() {
    generation++;
    clear("idle");
    clear("retry");
    clear("stable");
    if (controller) {
      try {
        controller.abort();
      } catch (e) {
        /* already settled */
      }
      controller = null;
    }
  }
  function armIdle(gen) {
    if (!config.idleTimeout) return;
    later("idle", config.idleTimeout, function () {
      if (gen === generation) fail("idle", false);
    });
  }
  // Equal jitter: attempt n waits a random time in [full/2, full], where full
  // doubles from the floor (baseDelay, or a larger server retry) up to maxDelay,
  // and never less than the floor. The first attempt's full delay is the floor
  // itself, so it spreads upward over [floor, 1.5 × floor] instead: clients that
  // lost the same server do not reconnect at the same instant, and none sooner
  // than the floor.
  function delay() {
    var floor = Math.max(config.baseDelay, Math.min(serverRetry, config.maxDelay));
    var full = Math.min(config.maxDelay, floor * Math.pow(2, attempt - 1));
    var low = Math.max(floor, full / 2);
    var high = Math.min(config.maxDelay, Math.max(full, floor * 1.5));
    return Math.round(low + Math.random() * (high - low));
  }
  function fail(code, permanent) {
    abort();
    if (permanent) {
      setStatus("unavailable", code);
      announce("Live updates unavailable. Reload to retry.");
      return;
    }
    attempt++;
    if (attempt > config.maxAttempts) {
      attempt = config.maxAttempts;
      setStatus("unavailable", code);
      announce("Live updates unavailable. Reload to retry.");
      return;
    }
    setStatus("reconnecting", code);
    later("retry", delay(), function () {
      if (d.visibilityState === "hidden") pause();
      else connect();
    });
  }
  function read(reader, gen) {
    var decoder = new w.TextDecoder("utf-8");
    function pump() {
      return reader.read().then(
        function (chunk) {
          if (gen !== generation) return;
          if (chunk.done) {
            feed(decoder.decode());
            if (gen === generation) fail("closed", false);
            return;
          }
          armIdle(gen);
          var within = feed(decoder.decode(chunk.value, { stream: true }));
          if (gen !== generation) return;
          if (!within) return fail("protocol", false);
          return pump();
        },
        function () {
          if (gen === generation) fail("network", false);
        }
      );
    }
    return pump();
  }
  function connect() {
    abort();
    if (typeof w.fetch !== "function" || typeof w.TextDecoder !== "function") {
      setStatus("unavailable", "unsupported");
      return;
    }
    var gen = generation;
    controller = typeof w.AbortController === "function" ? new w.AbortController() : null;
    resetParser();
    connectedFrom = lastEventId;
    setStatus(attempt > 0 ? "reconnecting" : "connecting", error);
    var headers = { Accept: "text/event-stream" };
    if (lastEventId !== "") {
      try {
        headers["Last-Event-ID"] = utf8Bytes(lastEventId);
      } catch (e) {
        /* unreachable for decoded or validated ids; connect without a cursor */
      }
    }
    var init = { method: "GET", headers: headers, cache: "no-store", credentials: "same-origin" };
    if (controller) init.signal = controller.signal;
    armIdle(gen);
    w.fetch(config.url, init).then(
      function (res) {
        if (gen !== generation) return;
        if (res.status === 204) {
          abort();
          setStatus("stopped", "server-closed");
          return;
        }
        var type = String(res.headers.get("Content-Type") || "").split(";")[0].trim().toLowerCase();
        if (res.ok && type === "text/event-stream" && (!res.body || typeof res.body.getReader !== "function")) {
          abort();
          setStatus("unavailable", "unsupported");
          return;
        }
        if (!res.ok || type !== "text/event-stream") {
          var s = res.status;
          var permanent = s >= 400 && s < 500 && s !== 408 && s !== 429;
          return fail((permanent ? "refused-" : "http-") + s, permanent);
        }
        setStatus("live", null);
        armIdle(gen);
        later("stable", STABLE_AFTER, function () {
          if (gen === generation) attempt = 0;
        });
        return read(res.body.getReader(), gen);
      },
      function () {
        if (gen === generation) fail("network", false);
      }
    );
  }
  function pause() {
    if (!ACTIVE[status]) return;
    abort();
    setStatus("paused", null);
  }
  function resume() {
    if (status !== "paused") return;
    attempt = 0;
    connect();
  }

  // ── public API ────────────────────────────────────────────────────────────
  // The render-time cursor of the page status whose data-live-url is this
  // stream, or "" when none names it (or its cursor is invalid).
  function pageCursor(url) {
    var pages = d.querySelectorAll("[data-live-page][data-live-url]");
    for (var i = 0; i < pages.length; i++) {
      var href;
      try {
        href = new w.URL(pages[i].getAttribute("data-live-url"), w.location.href).href;
      } catch (e) {
        continue;
      }
      if (href !== url) continue;
      var id = pages[i].getAttribute("data-live-last-event-id");
      return id !== null && validCursor(id) ? id : "";
    }
    return "";
  }
  function start(options) {
    if (options) {
      var next = configure(options);
      if (config && ACTIVE[status] && next.url === config.url) {
        config = next;
        return snapshot();
      }
      // A cursor belongs to its stream; another URL starts from its own.
      if (config && next.url !== config.url) lastEventId = "";
      config = next;
      // The server-rendered cursor lets the first connection replay anything
      // published between rendering and subscribing. Without an explicit one,
      // adopt the cursor of the page status naming this URL (once it is parsed).
      adoptCursor = false;
      if (lastEventId === "") {
        if (options.lastEventId !== undefined) lastEventId = options.lastEventId;
        else if (!parsed) adoptCursor = true;
        else lastEventId = pageCursor(config.url);
      }
    } else if (!config) {
      throw new TypeError("AppKitLive: start needs {url} the first time");
    } else if (ACTIVE[status]) {
      return snapshot();
    }
    // Regions and the page cursor may not be parsed yet: an event matched now
    // would find no region. Connect once the document is ready.
    if (!parsed) {
      deferred = true;
      return snapshot();
    }
    return begin();
  }
  function begin() {
    var resumed = status === "stopped" || status === "unavailable";
    attempt = 0;
    if (d.visibilityState === "hidden") {
      abort();
      setStatus("paused", null);
    } else connect();
    // Only a resume that is actually running is announced.
    if (resumed && ACTIVE[status]) announce("Live updates resumed.");
    return snapshot();
  }
  function stop() {
    deferred = false;
    abort();
    clear("hidden");
    if (config && status !== "idle") {
      var announceStop = status !== "stopped";
      setStatus("stopped", null);
      if (announceStop) announce("Live updates stopped.");
    }
    return snapshot();
  }
  function mark(source) {
    var matched = markSource(source, lastEventId);
    render();
    return matched;
  }

  // ── document integration ──────────────────────────────────────────────────
  d.addEventListener("click", function (e) {
    var target = e.target && e.target.closest ? e.target : null;
    if (!target) return;
    var toggle = target.closest("[data-live-toggle]");
    if (toggle) {
      e.preventDefault();
      if (ACTIVE[status]) stop();
      else if (resumable()) start();
      return;
    }
    var link = target.closest("a[data-live-refresh]");
    if (!link || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    var url;
    try {
      url = new w.URL(link.getAttribute("href"), w.location.href);
    } catch (err) {
      return;
    }
    try {
      w.sessionStorage.setItem(FOCUS_KEY, url.hash.slice(1));
    } catch (err) {
      /* storage can be unavailable; focus return is a convenience */
    }
    var here = w.location;
    // A fragment-only navigation would scroll without re-rendering. Reload the
    // same document instead; other destinations use ordinary navigation.
    if (url.origin === here.origin && url.pathname === here.pathname && url.search === here.search) {
      e.preventDefault();
      if (url.hash !== here.hash) w.history.replaceState(w.history.state, "", url.href);
      here.reload();
    }
  });
  d.addEventListener("visibilitychange", function () {
    if (!config || !ACTIVE[status]) return;
    if (d.visibilityState === "hidden") {
      if (config.hiddenGrace === 0) pause();
      else later("hidden", config.hiddenGrace, pause);
    } else {
      clear("hidden");
      resume();
    }
  });
  w.addEventListener("online", function () {
    if (status === "reconnecting") {
      clear("retry");
      connect();
    }
  });
  w.addEventListener("pagehide", function () {
    pause();
  });
  w.addEventListener("pageshow", function (e) {
    if (e.persisted && d.visibilityState !== "hidden") resume();
  });

  function restoreFocus() {
    var id = null;
    try {
      id = w.sessionStorage.getItem(FOCUS_KEY);
      w.sessionStorage.removeItem(FOCUS_KEY);
    } catch (e) {
      return;
    }
    if (!id || w.location.hash !== "#" + id) return;
    var el = d.getElementById(id);
    if (el && el.hasAttribute("data-live-source") && typeof el.focus === "function") el.focus({ preventScroll: true });
  }
  function ready() {
    parsed = true;
    restoreFocus();
    if (config) {
      if (adoptCursor && lastEventId === "") lastEventId = pageCursor(config.url);
      adoptCursor = false;
      if (deferred) {
        deferred = false;
        begin();
      }
      return;
    }
    var page = d.querySelector("[data-live-page][data-live-url]");
    if (!page) return;
    var events = (page.getAttribute("data-live-events") || "").split(/\s+/).filter(Boolean);
    var options = { url: page.getAttribute("data-live-url"), events: events };
    if (page.hasAttribute("data-live-last-event-id")) options.lastEventId = page.getAttribute("data-live-last-event-id");
    try {
      start(options);
    } catch (e) {
      setStatus("unavailable", "configuration");
    }
  }
  w.AppKitLive = { version: 1, start: start, stop: stop, state: snapshot, mark: mark };
  if (d.readyState === "loading") d.addEventListener("DOMContentLoaded", ready);
  else ready();
})(window, document);
