// count.js — the counting tween for appkit/ui/motion. Vanilla, dependency-free,
// no build step, no framework bindings (ADR-0003 / D5). Classic script, one
// global.
//
// A number that changes between two states should travel between them, because
// the movement is what says "this went down" rather than "this is a different
// number". That is the whole feature.
//
// ── PROVENANCE ─────────────────────────────────────────────────────────────
//
// Promoted from geoffmiller-cloud, where the same tween existed byte-identically
// in SpecFinOps.jsx and SpecFinOpsUtil.jsx (whose own comment admitted the
// duplication). What was stripped on the way up: React.useState/useRef/useEffect,
// the [target] dependency array, and the component's own reveal timing. What
// survived is the part that generalises — the ease, the duration, the
// reduced-motion short-circuit, and the cancel. It is L2 under ADR-0017, where
// the bar is one consumer plus a Script()/Handler() pair, because a 23-line
// pure function is additive, namespaced, individually ignorable and cheap to
// delete.
//
//   AppKitCount.to(from, target, onValue[, opts])  → cancel()
//
// onValue is called with an integer on each frame and exactly once with the
// exact target at the end, so the final rendered value is never an easing
// artifact. opts: {duration = 620, round = Math.round}.
//
// ── WHY IT JUMPS RATHER THAN SLOWS ─────────────────────────────────────────
//
// Under prefers-reduced-motion the tween does not run at a gentler speed — it
// does not run at all, and onValue is called once with the target. A slower
// interpolation is still interpolation, and interpolation is the thing that was
// asked to stop. The end state was always legible on its own; that is the
// invariant motion.css exists to hold, and it holds here too.
(function (global) {
  "use strict";

  function reduced() {
    return (
      typeof global.matchMedia === "function" &&
      global.matchMedia("(prefers-reduced-motion: reduce)").matches
    );
  }

  // to tweens from → target, calling onValue each frame. Returns a cancel
  // function that is always safe to call, including after the tween finished.
  function to(from, target, onValue, opts) {
    var o = opts || {};
    var dur = typeof o.duration === "number" && o.duration > 0 ? o.duration : 620;
    var round = typeof o.round === "function" ? o.round : Math.round;
    var noop = function () {};

    if (from === target || reduced() || typeof global.requestAnimationFrame !== "function") {
      onValue(target);
      return noop;
    }

    var t0 = Date.now();
    var id = 0;
    var step = function () {
      var p = Math.min(1, (Date.now() - t0) / dur);
      // Cubic ease-out: fast enough to read as a response, settled enough that
      // the last third is legible rather than a blur.
      var e = 1 - Math.pow(1 - p, 3);
      if (p < 1) {
        onValue(round(from + (target - from) * e));
        id = global.requestAnimationFrame(step);
      } else {
        onValue(target);
        id = 0;
      }
    };
    id = global.requestAnimationFrame(step);

    return function cancel() {
      if (id) {
        global.cancelAnimationFrame(id);
        id = 0;
      }
    };
  }

  global.AppKitCount = { to: to, reduced: reduced };
})(typeof window !== "undefined" ? window : this);
