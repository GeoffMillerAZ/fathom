// chat.js — dependency-free compose comforts + a small streaming API for
// appkit/ui/chat. Loopback-safe (D5): the app owns every endpoint (POST + SSE);
// this only wires the UI. No framework, no build step.
//
// Markup hooks (emitted by the Go render functions):
//   [data-chat-form]                  the composer <form>            (+ optional data-chat-draft-key)
//   [data-chat-input]                 the message <textarea>
//   [data-chat-send] / [data-chat-stop]  submit / stop buttons
//   [data-chat-scroll]                the scroll region (transcript)
//   [data-chat-list]                  the message container inside it
//   [data-chat-empty]                 the empty-state placeholder
//   [data-chat-jump]                  the "jump to latest" pill
//   [data-chat-msg] .chat-msg__body   a message + its copyable body
//   [data-chat-copy]                  a copy button
//
// App-driven streaming API (call from your SSE onmessage):
//   AppKitChat.streaming(true|false)  toggles Stop/Send button state
//   AppKitChat.thinking(true|false)   shows/removes the "typing" bubble
//   AppKitChat.appendDelta(text)      appends streamed text to the live bubble (creating it)
//   AppKitChat.finalize(htmlOrNull)   swaps the live bubble body to final HTML, ends streaming
//   Listen for the 'appkit-chat:stop' event to cancel your stream.
(function () {
  "use strict";

  var MAX_H = 192;
  var THINKING =
    '<div class="chat-msg flex flex-col items-start gap-1" data-chat-thinking>' +
    '<div class="max-w-[42rem] rounded-lg border border-line bg-surface-raised px-4 py-3">' +
    '<span class="chat-dots inline-flex gap-1" aria-label="Assistant is thinking">' +
    '<span class="size-1.5 animate-pulse rounded-full bg-ink-faint"></span>' +
    '<span class="size-1.5 animate-pulse rounded-full bg-ink-faint [animation-delay:150ms]"></span>' +
    '<span class="size-1.5 animate-pulse rounded-full bg-ink-faint [animation-delay:300ms]"></span>' +
    "</span></div></div>";
  var LIVE_SHELL =
    '<div class="chat-msg flex flex-col items-start gap-1" data-chat-live>' +
    '<div class="max-w-[42rem] rounded-lg border border-line bg-surface-raised px-4 py-3">' +
    '<div class="chat-msg__body text-sm text-ink"></div></div></div>';

  function autosize(el) {
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, MAX_H) + "px";
  }
  function atBottom(r) {
    return r.scrollHeight - r.scrollTop - r.clientHeight < 40;
  }
  function toBottom(r) {
    if (r) r.scrollTop = r.scrollHeight;
  }
  function region() {
    return document.querySelector("[data-chat-scroll]");
  }
  // The message container; creates one (clearing the empty state) if absent.
  function list(r) {
    var l = r.querySelector("[data-chat-list]");
    if (l) return l;
    var empty = r.querySelector("[data-chat-empty]");
    if (empty) empty.remove();
    l = document.createElement("div");
    l.className = "flex flex-col gap-5 p-4";
    l.setAttribute("data-chat-list", "");
    var jump = r.querySelector("[data-chat-jump]");
    r.insertBefore(l, jump || null);
    return l;
  }

  // ── delegated input behaviours ──────────────────────────────────────────
  document.addEventListener("input", function (e) {
    var t = e.target;
    if (!t.matches || !t.matches("[data-chat-input]")) return;
    autosize(t);
    var form = t.closest("[data-chat-form]");
    var k = form && form.getAttribute("data-chat-draft-key");
    if (k) try { localStorage.setItem(k, t.value); } catch (_) {}
  });

  document.addEventListener("keydown", function (e) {
    var t = e.target;
    if (!t.matches || !t.matches("[data-chat-input]")) return;
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      var form = t.closest("[data-chat-form]");
      if (form) (form.requestSubmit ? form.requestSubmit() : form.submit());
    }
  });

  document.addEventListener("submit", function (e) {
    var form = e.target;
    if (!form.matches || !form.matches("[data-chat-form]")) return;
    var k = form.getAttribute("data-chat-draft-key");
    if (k) try { localStorage.removeItem(k); } catch (_) {}
  });

  // ── delegated clicks: copy / jump / stop ────────────────────────────────
  document.addEventListener("click", function (e) {
    var t = e.target.closest && e.target.closest("[data-chat-copy],[data-chat-jump],[data-chat-stop]");
    if (!t) return;
    if (t.hasAttribute("data-chat-copy")) {
      var msg = t.closest("[data-chat-msg]");
      var body = msg && msg.querySelector(".chat-msg__body");
      if (body && navigator.clipboard) {
        navigator.clipboard.writeText(body.innerText.trim());
        var o = t.textContent;
        t.textContent = "Copied";
        setTimeout(function () { t.textContent = o; }, 1200);
      }
    } else if (t.hasAttribute("data-chat-jump")) {
      var r = region();
      toBottom(r);
      t.hidden = true;
    } else if (t.hasAttribute("data-chat-stop")) {
      document.dispatchEvent(new CustomEvent("appkit-chat:stop"));
    }
  });

  function wireScroll(r) {
    var jump = r.querySelector("[data-chat-jump]");
    r.addEventListener("scroll", function () {
      if (jump) jump.hidden = atBottom(r);
    });
  }

  function init() {
    document.querySelectorAll("[data-chat-form]").forEach(function (form) {
      var input = form.querySelector("[data-chat-input]");
      if (!input) return;
      var k = form.getAttribute("data-chat-draft-key");
      if (k) try {
        var v = localStorage.getItem(k);
        if (v && !input.value) input.value = v;
      } catch (_) {}
      autosize(input);
    });
    document.querySelectorAll("[data-chat-scroll]").forEach(function (r) {
      toBottom(r);
      wireScroll(r);
    });
  }

  // ── app-driven streaming API ────────────────────────────────────────────
  var AppKitChat = {
    streaming: function (on) {
      document.querySelectorAll("[data-chat-stop]").forEach(function (b) { b.hidden = !on; });
      document.querySelectorAll("[data-chat-send]").forEach(function (b) { b.disabled = !!on; });
    },
    thinking: function (on) {
      var r = region();
      if (!r) return;
      var ex = r.querySelector("[data-chat-thinking]");
      if (on) {
        if (!ex) {
          var d = document.createElement("div");
          d.innerHTML = THINKING;
          list(r).appendChild(d.firstChild);
          toBottom(r);
        }
      } else if (ex) {
        ex.remove();
      }
    },
    appendDelta: function (text) {
      var r = region();
      if (!r) return;
      this.thinking(false);
      var live = r.querySelector("[data-chat-live] .chat-msg__body");
      if (!live) {
        var d = document.createElement("div");
        d.innerHTML = LIVE_SHELL;
        list(r).appendChild(d.firstChild);
        live = r.querySelector("[data-chat-live] .chat-msg__body");
      }
      var stick = atBottom(r);
      live.textContent += text;
      if (stick) toBottom(r);
    },
    finalize: function (htmlOrNull) {
      var r = region();
      if (!r) return;
      var msg = r.querySelector("[data-chat-live]");
      if (msg) {
        if (htmlOrNull != null) {
          var body = msg.querySelector(".chat-msg__body");
          if (body) body.innerHTML = htmlOrNull;
        }
        msg.removeAttribute("data-chat-live");
      }
      this.streaming(false);
      if (atBottom(r)) toBottom(r);
    },
  };
  window.AppKitChat = AppKitChat;

  if (document.readyState !== "loading") init();
  else document.addEventListener("DOMContentLoaded", init);
})();
