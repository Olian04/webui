// Runs before first paint so the theme never flashes. Classic script, not a
// module, because modules are deferred. Preferences are per-browser session
// state; they are deliberately NOT arguments, so they stay out of the URL.
(function () {
  var g = function (k, d) { try { return localStorage.getItem('wui.' + k) || d; } catch (e) { return d; } };
  var el = document.documentElement;
  el.dataset.theme = g('theme', 'dark');
  el.dataset.role = g('role', 'editor');
})();
