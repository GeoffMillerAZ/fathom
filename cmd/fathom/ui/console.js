// fathom observatory console — the small L2 layer (vision 35 §5: works
// without JavaScript; better with it). Today it only makes verb chips
// copyable: every chip's command is selectable text without it. Classic
// script, no dependencies, no network.
//
// A chip's accessible name is its aria-label ("Copy the command …"), which a
// changed button text never reaches, so the outcome is also announced
// through one polite live region (WCAG 4.1.3 status messages). The region
// exists from page load, before any announcement, so screen readers track it.
(function (w, d) {
  'use strict';
  function announcer() {
    var region = d.querySelector('[data-console-announce]');
    if (region) return region;
    region = d.createElement('p');
    region.setAttribute('data-console-announce', '');
    region.setAttribute('role', 'status');
    region.setAttribute('aria-live', 'polite');
    region.className = 'sr-only';
    d.body.appendChild(region);
    return region;
  }
  function reveal(root) {
    var buttons = root.querySelectorAll('[data-console-copy]');
    if (!buttons.length || !w.navigator || !w.navigator.clipboard) return; // no clipboard: the command stays selectable text
    var region = announcer();
    Array.prototype.forEach.call(buttons, function (btn) {
      btn.hidden = false;
      if (btn.getAttribute('data-console-copy-bound')) return;
      btn.setAttribute('data-console-copy-bound', '1');
      btn.addEventListener('click', function () {
        var text = btn.getAttribute('data-console-copy') || '';
        var label = btn.textContent;
        region.textContent = '';
        w.navigator.clipboard.writeText(text).then(function () {
          btn.textContent = 'Copied';
          region.textContent = 'Copied the command ' + text;
        }, function () {
          btn.textContent = 'Copy failed';
          region.textContent = 'Copy failed — select the command text instead';
        }).then(function () {
          w.setTimeout(function () { btn.textContent = label; }, 1500);
        });
      });
    });
  }
  if (d.readyState === 'loading') d.addEventListener('DOMContentLoaded', function () { reveal(d); });
  else reveal(d);
})(window, document);
