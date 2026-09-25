// fathom observatory console — the small L2 layer (vision 35 §5: works
// without JavaScript; better with it). Today it only makes verb chips
// copyable: every chip's command is selectable text without it. Classic
// script, no dependencies, no network.
(function (w, d) {
  'use strict';
  function reveal(root) {
    Array.prototype.forEach.call(root.querySelectorAll('[data-console-copy]'), function (btn) {
      if (!w.navigator || !w.navigator.clipboard) return; // no clipboard: the command stays selectable text
      btn.hidden = false;
      if (btn.getAttribute('data-console-copy-bound')) return;
      btn.setAttribute('data-console-copy-bound', '1');
      btn.addEventListener('click', function () {
        var text = btn.getAttribute('data-console-copy') || '';
        var label = btn.textContent;
        w.navigator.clipboard.writeText(text).then(function () {
          btn.textContent = 'Copied';
        }, function () {
          btn.textContent = 'Copy failed';
        }).then(function () {
          w.setTimeout(function () { btn.textContent = label; }, 1500);
        });
      });
    });
  }
  if (d.readyState === 'loading') d.addEventListener('DOMContentLoaded', function () { reveal(d); });
  else reveal(d);
})(window, document);
