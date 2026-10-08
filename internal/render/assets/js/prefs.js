// Runs before first paint so the theme never flashes. A classic script, not a
// module, because modules are deferred. Preferences are per-browser state and
// are deliberately not arguments, so they stay out of the URL. A stored theme
// overrides the app's default; with nothing stored the server's choice stands.
(function () {
  var el = document.documentElement;
  el.classList.add('js');
  try {
    var theme = localStorage.getItem('webui.theme');
    if (theme === 'dark' || theme === 'light') el.dataset.theme = theme;
  } catch (e) {}
})();
