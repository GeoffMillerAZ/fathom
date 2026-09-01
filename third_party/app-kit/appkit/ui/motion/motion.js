// motion.js — the optional style-flush helper for appkit/ui/motion. Vanilla,
// dependency-free, no build step, no Node (ADR-0002), no framework bindings
// (ADR-0003 / D5). It enhances markup the server already rendered and owns no
// state of its own.
//
// IT IS OPTIONAL. motion.css is the contribution; this file is a convenience.
// A consumer that toggles `ak-state-b` with its own code, with a signal, with a
// checkbox, or server-side on the initial render needs none of this and loses
// nothing. Everything here is three lines you could write yourself — they are
// here because one of them is three lines nobody knows to write.
//
//   AppKitMotion.flush(el)          force a style recalculation, now
//   AppKitMotion.enter(el, cls)     flush, then add cls (default "ak-state-b")
//   AppKitMotion.toggle(el, on, cls)  set the state class; no flush needed
//   AppKitMotion.frame(fn)          run fn after the next real paint
//   AppKitMotion.reduced()          true when the reader asked for less motion
//
// ── WHY flush() EXISTS ─────────────────────────────────────────────────────
//
// A CSS transition fires on a CHANGE between two computed styles. The browser
// computes style once per frame, at the end. So if an element is inserted into
// the DOM and its state class is added in the SAME frame, the browser only ever
// computes one style for it — the final one. There was no previous value to
// interpolate from, so the transition does not run. The element simply appears
// in its end state.
//
// Nothing reports this. No exception, no console warning, no invalid
// declaration, nothing in the computed-styles panel afterwards. The CSS is
// correct, the class is on the element, and the animation silently never
// happened. It is one of the few CSS failures with no artifact to find.
//
// Reading a layout property (getBoundingClientRect) forces the browser to
// compute style and layout right then. That commits the pre-transition value,
// so the class added on the next line is a genuine change and the transition
// runs:
//
//   panel.classList.add("ak-motion");     // both states now in the DOM
//   AppKitMotion.flush(panel);            // ← without this line, nothing tweens
//   panel.classList.add("ak-state-b");
//
// frame() is the alternative for callers who would rather wait than force:
// two nested requestAnimationFrame calls put the class change after a real
// paint of the initial state. One rAF is not reliably enough — the callback can
// still run before the first paint of freshly inserted nodes.
(function (w, d) {
  "use strict";

  var STATE = "ak-state-b";

  // flush forces a synchronous style + layout recalculation of el (default the
  // document element) and returns the width it read. The read is the point;
  // the value is not. Do not "optimise" this line away.
  //
  // getBoundingClientRect() reads back on both HTML and SVG because it is a
  // method of Element. The older idiom read a layout-width *property*, which is
  // defined only on HTMLElement: on an SVGElement (an <svg>, <g>, <rect> …) it
  // is `undefined`, forcing no layout — so flush() and enter() were a silent
  // no-op on exactly the SVG marks appkit/ui/motion animates (see motion.go's
  // example). motion_test.go pins this method choice.
  function flush(el) {
    return (el || d.documentElement).getBoundingClientRect().width;
  }

  function frame(fn) {
    w.requestAnimationFrame(function () {
      w.requestAnimationFrame(fn);
    });
  }

  // enter commits the current style, then switches to state B, so a transition
  // applied in the same frame as mount actually runs. Use it on first paint.
  function enter(el, cls) {
    if (!el) return;
    flush(el);
    el.classList.add(cls || STATE);
  }

  // toggle sets or clears the state class on an element that is already
  // painted. No flush: the previous style has been computed for at least a
  // frame, so the change interpolates on its own.
  function toggle(el, on, cls) {
    if (!el) return;
    el.classList.toggle(cls || STATE, !!on);
  }

  // reduced reports the reader's preference. The CSS already honours it — this
  // is for JS that would otherwise wait on a transition that will not play.
  function reduced() {
    return !!(w.matchMedia && w.matchMedia("(prefers-reduced-motion: reduce)").matches);
  }

  w.AppKitMotion = {
    STATE: STATE,
    flush: flush,
    frame: frame,
    enter: enter,
    toggle: toggle,
    reduced: reduced,
  };
})(window, document);
