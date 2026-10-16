// Runs before first paint so the theme never flashes. A classic script, not a
// module, because modules are deferred. Preferences are per-browser state and
// are deliberately not arguments, so they stay out of the URL. A stored theme
// overrides the app's default; with nothing stored the server's choice stands.
// The same goes for the sidebar, which a person may narrow to its icon rail.
//
// The sidebar is part of each page, so a followed link draws it again at the top. Its place is kept for
// the tab (not for the browser: it is where this visit has got to) and put back as the sidebar is parsed,
// before first paint, so a link far down the list is followed without the list jumping.
(function () {
  var el = document.documentElement;
  el.classList.add('js');
  try {
    var theme = localStorage.getItem('webui.theme');
    if (theme === 'dark' || theme === 'light') el.dataset.theme = theme;
    if (localStorage.getItem('webui.sidebar') === 'collapsed') el.dataset.sidebar = 'collapsed';
  } catch (e) {}
})();

(function () {
  var KEY = 'webui.sidebar.scroll';
  function place() {
    return document.querySelector('.side-scroll');
  }
  // Leaving a page, whichever way, keeps where the sidebar was.
  addEventListener('pagehide', function () {
    var side = place();
    if (!side) return;
    try {
      sessionStorage.setItem(KEY, String(side.scrollTop));
    } catch (e) {}
  });

  var saved = 0;
  try {
    saved = parseInt(sessionStorage.getItem(KEY), 10) || 0;
  } catch (e) {}
  if (!saved) return;

  // The sidebar is not parsed yet. Put the place back each time the parser adds to the document, until the
  // list is complete (what follows it has begun), or the page is: not after, so that a person who scrolls a
  // page that is still arriving is not taken back.
  var watch = new MutationObserver(function () {
    var side = place();
    if (!side) return;
    side.scrollTop = saved;
    if (side.nextElementSibling) watch.disconnect();
  });
  watch.observe(document, { childList: true, subtree: true });
  document.addEventListener('DOMContentLoaded', function () {
    var side = place();
    if (side) side.scrollTop = saved;
    watch.disconnect();
  });
})();

// The content region is drawn again by every link too, so a tab, a sort or a page of a table that is followed
// on a page that has been scrolled would take the reader back to the top of it. Where the content was is kept for
// the tab with the address's path, and put back only when the next page has the same path: going to another page
// starts at its top, as it should. It is put back as the content arrives, and not after the reader has scrolled
// it themselves, so a page that is still being sent does not fight them.
(function () {
  var KEY = 'webui.content.scroll';
  function place() {
    return document.querySelector('main.scroll');
  }
  addEventListener('pagehide', function () {
    var main = place();
    if (!main) return;
    try {
      sessionStorage.setItem(KEY, JSON.stringify({ path: location.pathname, top: main.scrollTop }));
    } catch (e) {}
  });

  var saved = 0;
  try {
    var got = JSON.parse(sessionStorage.getItem(KEY) || 'null');
    if (got && got.path === location.pathname) saved = got.top || 0;
  } catch (e) {}
  if (!saved) return;

  var watch;
  function stop() {
    watch.disconnect();
    ['wheel', 'touchstart', 'keydown', 'mousedown'].forEach(function (name) {
      removeEventListener(name, stop, true);
    });
  }
  function put() {
    var main = place();
    if (!main) return false;
    main.scrollTop = saved;
    // Done once the content is as long as it was: what is not there yet cannot be scrolled to.
    return main.scrollHeight - main.clientHeight >= saved;
  }
  watch = new MutationObserver(function () {
    if (put()) stop();
  });
  watch.observe(document, { childList: true, subtree: true });
  ['wheel', 'touchstart', 'keydown', 'mousedown'].forEach(function (name) {
    addEventListener(name, stop, true);
  });
  document.addEventListener('DOMContentLoaded', function () {
    put();
    stop();
  });
})();
