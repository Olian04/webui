// Progressive enhancement. Every control in the page is a real link or form and
// the app works with this file switched off; the script changes how much of the
// page a followed link replaces, and adds a few conveniences that need script.
//
// The rule that decides what a click does: a link to a DIFFERENT path is a
// navigation and is left alone (the browser handles it). A link to the SAME path with different arguments,
// inside a panel, belongs to that panel, so only that panel is replaced.
(function () {
  'use strict';

  var LEAF_HEADER = 'X-Webui-Leaf';
  var BUILT_HEADER = 'X-Webui-Built';
  var inflight = new Map();

  function $(selector, root) {
    return (root || document).querySelector(selector);
  }
  function $$(selector, root) {
    return Array.prototype.slice.call((root || document).querySelectorAll(selector));
  }
  function leafSelector(id) {
    return '[data-leaf="' + CSS.escape(id) + '"]';
  }

  /* ------------------------------ status bar ------------------------------ */

  // The honesty bar: say what is in flight, and what the screen now shows.
  function status(method, url, note) {
    var m = $('[data-method]');
    var u = $('[data-url]');
    var n = $('[data-netmsg]');
    var log = $('[data-netlog]');
    if (m) m.textContent = method;
    if (u) u.textContent = url;
    if (n) n.textContent = note || '';
    if (log) log.classList.toggle('on', !!note);
  }

  // How the page came to be. The server says when it built the HTML and how long it took; the
  // browser says how long it took to get and show it, and says "stale page" when what it was given is old.
  // When the HTML was built is not shown with script: it is what the badge is worked out from, and a reader
  // has nothing to do with it unless it is wrong, which the badge says. Without script it is shown, since
  // nothing else can say it.
  // A page is built for each request and sent with no-store, so one that arrives long after it
  // was built was not built for this request: a cache or a proxy between here and the server
  // answered, or this computer's clock is not the server's. The clock is why this only says so.
  var STALE_MS = 15000;

  function ago(ms) {
    var s = Math.round(ms / 1000);
    if (s < 2) return 'just now';
    if (s < 60) return s + ' s ago';
    var m = Math.floor(s / 60);
    if (m < 60) return m + ' min ago';
    return Math.floor(m / 60) + ' h ago';
  }
  function builtAt() {
    var el = $('[data-built]');
    var at = el ? new Date(el.getAttribute('datetime')) : null;
    return at && !isNaN(at.getTime()) ? at : null;
  }
  function stale(reason) {
    var badge = $('[data-stale]');
    if (!badge) return;
    badge.textContent = reason ? 'stale page' : '';
    badge.title = reason || '';
    badge.hidden = !reason;
  }
  // Whether what is on screen was built for the request that brought it.
  function checkFresh(source) {
    var at = builtAt();
    if (!at) return;
    var age = Date.now() - at.getTime();
    if (age > STALE_MS) {
      stale(
        'This ' + source + ' was built ' + ago(age) + ', before it was asked for. A cache or a proxy between the server and here may have answered, or this computer\'s clock differs from the server\'s.',
      );
    } else {
      stale('');
    }
  }
  function millis(ms) {
    return (ms < 10 ? ms.toFixed(1) : Math.round(ms)) + ' ms';
  }
  // The page took ms from the request to being loaded, which the server's render time is part of: the delivery is the
  // rest (the network, the assets and the browser), so the two add up.
  function loaded(ms) {
    var el = $('[data-loaded]');
    var build = $('[data-build]');
    if (!el) return;
    var render = build ? parseFloat(build.textContent) : 0;
    if (isNaN(render)) render = 0;
    var delivery = Math.max(ms - render, 0);
    el.textContent = millis(delivery);
    el.hidden = false;
    var group = $('[data-timing]');
    if (group) {
      group.title =
        'The server took ' + millis(render) + ' to build this page. Delivery took ' + millis(delivery) +
        ': the network, the assets and the browser, until the page was loaded. ' + millis(render + delivery) + ' in all.';
    }
  }
  // A panel that was fetched again has its own build time, and the status bar says the latest.
  function noteResponse(res, took) {
    var built = res.headers.get(BUILT_HEADER);
    var el = $('[data-built]');
    if (built && el) {
      el.setAttribute('datetime', built);
      checkFresh('panel');
    }
    var dur = /dur=([\d.]+)/.exec(res.headers.get('Server-Timing') || '');
    var build = $('[data-build]');
    if (dur && build) build.textContent = millis(parseFloat(dur[1]));
    loaded(took);
  }
  // A page that is sent as it is built ends with a mark, which hides the spinner and the "loading" in the bar
  // (see .arriving). A page that ends without it was cut short: the connection broke, or the server stopped. The
  // browser says it is done, so what the page says must not still be waiting.
  function endArrival() {
    var spinner = $('.arriving');
    if (!spinner || $('.arrived')) return;
    spinner.remove();
    var loading = $('.status-loading');
    if (!loading) return;
    var badge = document.createElement('span');
    badge.className = 'sb-badge sb-warn';
    badge.title = 'The page ended before all of it arrived. Refresh to try again.';
    badge.textContent = 'incomplete';
    loading.className = 'status-meta';
    loading.textContent = '';
    loading.appendChild(badge);
  }
  function startStatus() {
    var nav = performance.getEntriesByType ? performance.getEntriesByType('navigation')[0] : null;
    if (nav && nav.transferSize === 0 && nav.decodedBodySize > 0 && nav.type !== 'reload') {
      stale('The browser showed this page from its own cache, so it may not be what the server would send now.');
    } else {
      checkFresh('page');
    }
    function done() {
      var n = performance.getEntriesByType ? performance.getEntriesByType('navigation')[0] : null;
      if (n && n.loadEventEnd > 0) loaded(n.loadEventEnd - n.startTime);
    }
    if (document.readyState === 'complete') setTimeout(done, 0);
    else addEventListener('load', function () { setTimeout(done, 0); });
    // A page restored from the back-forward cache is the one that was left, not a new request.
    addEventListener('pageshow', function (e) {
      if (!e.persisted) return;
      stale('This page was restored as it was left, so it may not be what the server would send now.');
    });
  }

  /* -------------------------------- toasts -------------------------------- */

  // Toasts the server rendered (a saved form, a rejected one) dismiss
  // themselves; without script they stay until the next page, which is fine.
  // The time left is kept while one is hovered or has focus, so a message being
  // read, or about to be dismissed with the button, does not vanish under the
  // pointer. The button works without script (see .toast-close).
  var TOAST_MS = 3400;

  function dismissToasts() {
    $$('.toast').forEach(function (el) {
      var left = TOAST_MS;
      var since = 0;
      var timer = 0;
      function run() {
        since = Date.now();
        timer = setTimeout(leave, left);
      }
      function hold() {
        if (!timer) return;
        clearTimeout(timer);
        timer = 0;
        left -= Date.now() - since;
      }
      function leave() {
        el.classList.add('out');
        setTimeout(function () {
          el.remove();
        }, 220);
      }
      el.addEventListener('mouseenter', hold);
      el.addEventListener('focusin', hold);
      el.addEventListener('mouseleave', function () {
        if (!timer && !el.matches(':focus-within')) run();
      });
      el.addEventListener('focusout', function () {
        if (!timer && !el.matches(':hover')) run();
      });
      run();
    });
  }

  /* ------------------------------ panel refresh --------------------------- */

  // While one panel reloads, its rows pulse and nothing else moves. This says
  // "this panel is reloading"; a whole-page fade would say "the page moved".
  function skeleton(el) {
    var body = $('tbody', el);
    if (!body) return;
    var columns = $$('thead th', el).length || 3;
    var rows = Math.min(Math.max($$('tbody tr', el).length, 3), 10);
    var widths = [70, 55, 40, 35, 50, 60];
    body.textContent = '';
    for (var r = 0; r < rows; r++) {
      var tr = document.createElement('tr');
      tr.className = 'skel-row';
      tr.setAttribute('aria-hidden', 'true');
      for (var c = 0; c < columns; c++) {
        var td = document.createElement('td');
        var bar = document.createElement('span');
        bar.className = 'skel';
        bar.style.width = widths[c % widths.length] + '%';
        td.appendChild(bar);
        tr.appendChild(td);
      }
      body.appendChild(tr);
    }
  }

  // The same URL, asking for one panel. Returns the panel element, or throws
  // when the answer is anything but that panel: the caller falls back to a
  // plain navigation, which is what the link was all along.
  async function fetchLeaf(id, href, signal) {
    var started = performance.now();
    var res = await fetch(href, {
      headers: { [LEAF_HEADER]: id, Accept: 'text/html' },
      credentials: 'same-origin',
      signal: signal,
    });
    var type = res.headers.get('Content-Type') || '';
    if (!res.ok || res.redirected || type.indexOf('text/html') !== 0) throw new Error('unexpected response');
    var tpl = document.createElement('template');
    tpl.innerHTML = await res.text(); // our own markup, from our own origin
    var next = tpl.content.firstElementChild;
    if (!next || next.getAttribute('data-leaf') !== id) throw new Error('not that panel');
    noteResponse(res, performance.now() - started);
    return next;
  }

  async function refreshLeaf(id, href, push) {
    var el = $(leafSelector(id));
    if (!el) return false;

    var previous = inflight.get(id);
    if (previous) previous.abort();
    var ctl = new AbortController();
    inflight.set(id, ctl);

    skeleton(el);
    status('GET', href, 'panel ' + id);
    try {
      var next = await fetchLeaf(id, href, ctl.signal);
      el.replaceWith(next);
      if (push) history.pushState(null, '', href);
      settle(href, id);
    } catch (err) {
      if (err && err.name === 'AbortError') return true; // a newer request took over
      location.href = href;
    } finally {
      if (inflight.get(id) === ctl) inflight.delete(id);
    }
    return true;
  }

  // After one panel moved to a new address, everything that embeds the old one
  // is stale: the Refresh button, a form's action, and the sort and pager links
  // of the panels around it. Forms keep what the user typed, so their action is
  // patched in place; panels with links are fetched again.
  function settle(href, movedId) {
    status('GET', href);
    var refresh = $('[data-refresh]');
    if (refresh) refresh.setAttribute('href', href);

    $$('[data-leaf]').forEach(function (leaf) {
      var id = leaf.getAttribute('data-leaf');
      if (id === movedId) return;
      $$('form', leaf).forEach(function (f) {
        if (f.method === 'post') f.setAttribute('action', href);
      });
      var links = $$('a[href]', leaf).some(function (a) {
        return new URL(a.href, location.href).pathname === location.pathname;
      });
      if (links) {
        fetchLeaf(id, href)
          .then(function (next) {
            var current = $(leafSelector(id));
            if (current) current.replaceWith(next);
          })
          .catch(function () {});
      }
    });
  }

  document.addEventListener('click', function (e) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;

    var a = e.target.closest('a[href]');
    if (!a || a.target || a.hasAttribute('download')) return;
    var leaf = a.closest('[data-leaf]');
    if (!leaf) return;
    var dest = new URL(a.href, location.href);
    if (dest.origin !== location.origin || dest.pathname !== location.pathname) return;

    e.preventDefault();
    refreshLeaf(leaf.getAttribute('data-leaf'), dest.pathname + dest.search, true);
  });

  /* ------------------------------- validation ------------------------------ */

  // The browser checks a form's constraints before it sends it, and says what it found in words of its own: "Please
  // match the format requested." The page knows what it means, and says it in the words the server uses beneath the
  // field, through the Constraint Validation API. The server writes what to say for each way a value can fail, by the
  // name of the ValidityState property that reports it, in data-validity; when a control reports itself invalid, the
  // first of them that is true goes to setCustomValidity, and it is taken back when the value changes, since a
  // custom message keeps a control invalid until it is. Without script the browser says its own words, and the
  // pattern's message, which is the control's title.
  // The server's order: missing, then unreadable, then the length, the pattern, and the bounds.
  var VALIDITY_ORDER = ['valueMissing', 'typeMismatch', 'badInput', 'stepMismatch', 'tooShort', 'tooLong', 'patternMismatch', 'rangeUnderflow', 'rangeOverflow'];

  document.addEventListener(
    'invalid',
    function (e) {
      var el = e.target;
      var raw = el.getAttribute && el.getAttribute('data-validity');
      if (!raw) return;
      var messages;
      try {
        messages = JSON.parse(raw);
      } catch (err) {
        return;
      }
      for (var i = 0; i < VALIDITY_ORDER.length; i++) {
        var reason = VALIDITY_ORDER[i];
        if (el.validity[reason] && messages[reason]) {
          el.setCustomValidity(messages[reason]);
          return;
        }
      }
    },
    true,
  );

  function takeBackValidity(e) {
    var el = e.target;
    if (el.hasAttribute && el.hasAttribute('data-validity') && el.validity && el.validity.customError) el.setCustomValidity('');
  }
  document.addEventListener('input', takeBackValidity, true);
  document.addEventListener('change', takeBackValidity, true);

  /* ---------------------------------- tabs --------------------------------- */

  // A tab is a link to the same page with another tab selected, and followed with script it asks for the Tabs alone:
  // its strip and the tab that is selected, so only what the new tab shows is read, and the rest of the page, which
  // has nothing new to say, is left as it is. The Tabs is replaced, the address moves, and the links of the page that
  // carry the selection (a sort, a pager, the other strips) are put right in place, where a panel that moved would have
  // had the panels around it asked for again. Anything that is not an answer with the Tabs in it, or that needs what the
  // page was not sent with, is a plain navigation, which is what the link was all along: a tab with an editor or a diff,
  // whose language service and files are told to the page as it is made, and a text with code in it, when the page has
  // no editor to colour it with.
  var EDITORS = '[data-code-editor], [data-code-diff]';
  var CODE_BLOCKS = '.md pre > code[class*="language-"]';

  // Puts the selection right in the address of every link and form of the page that is not in `except`.
  function retarget(except, key, value) {
    $$('a[href]').forEach(function (a) {
      if (except.contains(a)) return;
      var url = new URL(a.href, location.href);
      if (url.origin !== location.origin || url.pathname !== location.pathname) return;
      if (value === null) url.searchParams.delete(key);
      else url.searchParams.set(key, value);
      a.setAttribute('href', url.pathname + url.search);
    });
    $$('form').forEach(function (f) {
      if (except.contains(f)) return;
      var action = new URL(f.getAttribute('action') || location.href, location.href);
      if (action.pathname !== location.pathname) return;
      if (f.method === 'post') {
        // Where a form posts, and so where it comes back to, has the selection in it.
        if (value === null) action.searchParams.delete(key);
        else action.searchParams.set(key, value);
        f.setAttribute('action', action.pathname + action.search);
        return;
      }
      // A filter carries the rest of the address in fields.
      var field = f.querySelector('input[type="hidden"][name="' + CSS.escape(key) + '"]');
      if (value === null) {
        if (field) field.remove();
      } else if (field) {
        field.value = value;
      } else {
        field = document.createElement('input');
        field.type = 'hidden';
        field.name = key;
        field.value = value;
        f.appendChild(field);
      }
    });
  }

  // Brings the active tab of a strip that scrolls into view: in the middle of it when the strip is new, and, when it
  // replaces one that was scrolled, where that was, moved only as far as the active tab needs.
  function showActiveTab(set, scrolled) {
    var strip = $('.tabs', set);
    var active = strip && $('.tab.active', strip);
    if (!active || strip.scrollWidth <= strip.clientWidth) return;
    var left = active.offsetLeft;
    var right = left + active.offsetWidth;
    if (scrolled === undefined) {
      strip.scrollLeft = left - (strip.clientWidth - active.offsetWidth) / 2;
      return;
    }
    strip.scrollLeft = scrolled;
    if (left < strip.scrollLeft) strip.scrollLeft = left - 8;
    else if (right > strip.scrollLeft + strip.clientWidth) strip.scrollLeft = right - strip.clientWidth + 8;
  }

  async function selectTab(set, link) {
    var id = set.getAttribute('data-tabs');
    var key = set.getAttribute('data-tabkey');
    var dest = new URL(link.href, location.href);
    var href = dest.pathname + dest.search;

    var previous = inflight.get(id);
    if (previous) previous.abort();
    var ctl = new AbortController();
    inflight.set(id, ctl);

    set.setAttribute('aria-busy', 'true');
    status('GET', href, 'tab ' + id);
    var focused = document.activeElement === link;
    var scrolled = $('.tabs', set) ? $('.tabs', set).scrollLeft : 0;
    try {
      var started = performance.now();
      var res = await fetch(href, {
        headers: { [LEAF_HEADER]: id, Accept: 'text/html' },
        credentials: 'same-origin',
        signal: ctl.signal,
      });
      var type = res.headers.get('Content-Type') || '';
      if (!res.ok || res.redirected || type.indexOf('text/html') !== 0) throw new Error('unexpected response');
      var tpl = document.createElement('template');
      tpl.innerHTML = await res.text(); // our own markup, from our own origin
      var next = tpl.content.firstElementChild;
      if (!next || next.getAttribute('data-tabs') !== id) throw new Error('not that tabs');
      if (next.querySelector(EDITORS)) throw new Error('needs the editor the page was made with');
      if (next.querySelector(CODE_BLOCKS) && !document.getElementById('webui-code')) throw new Error('needs the editor to colour code');
      noteResponse(res, performance.now() - started);
      set.replaceWith(next);
      showActiveTab(next, scrolled);
      history.pushState(null, '', href);
      var refresh = $('[data-refresh]');
      if (refresh) refresh.setAttribute('href', href);
      retarget(next, key, dest.searchParams.get(key));
      status('GET', href);
      var active = $('.tab.active', next);
      if (focused && active) active.focus();
    } catch (err) {
      if (err && err.name === 'AbortError') return; // a newer request took over
      location.href = href;
    } finally {
      if (inflight.get(id) === ctl) inflight.delete(id);
      set.removeAttribute('aria-busy');
    }
  }

  document.addEventListener('click', function (e) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    var a = e.target.closest('.tabs a[href]');
    var set = a && a.closest('[data-tabs]');
    if (!set || a.target || a.hasAttribute('download')) return;
    var dest = new URL(a.href, location.href);
    if (dest.origin !== location.origin || dest.pathname !== location.pathname) return;
    e.preventDefault();
    selectTab(set, a);
  });

  // The address is the truth. Going back or forward to a panel state reloads
  // the page, which is correct and needs no history bookkeeping.
  window.addEventListener('popstate', function () {
    location.reload();
  });

  /* ------------------------------ column filters --------------------------- */

  // A filter is a <details>, which opens and submits on its own. Script only
  // keeps one open at a time, closes it on a click elsewhere or on Escape, and
  // applies a filter to its panel alone, like any other link inside it.
  // The app's menu is a <details> too, and shares all of it: one popover open
  // at a time, closed by a click elsewhere or Escape.
  var POPOVERS = 'details.filter, details.menu';
  function closeFilters(except) {
    $$('details.filter[open], details.menu[open]').forEach(function (d) {
      if (d !== except) d.removeAttribute('open');
    });
  }

  // A filter's form opens beneath its header, inside a table body that scrolls, which
  // would clip it. While it is open, script pins it to the window at that spot, and
  // lets it go when the body scrolls. Without script it stays where it is.
  function pin(d) {
    var pop = $('.filter-pop', d);
    var btn = $('summary', d);
    if (!pop || !btn) return;
    var at = btn.getBoundingClientRect();
    pop.classList.add('pinned');
    pop.style.top = at.bottom + 6 + 'px';
    var width = pop.offsetWidth;
    var left = d.closest('th.num') ? at.right - width : at.left;
    pop.style.left = Math.max(8, Math.min(left, window.innerWidth - width - 8)) + 'px';
  }

  function unpin(d) {
    var pop = $('.filter-pop', d);
    if (!pop) return;
    pop.classList.remove('pinned');
    pop.style.top = pop.style.left = '';
  }

  document.addEventListener(
    'toggle',
    function (e) {
      if (!e.target.matches || !e.target.matches(POPOVERS)) return;
      if (e.target.open) {
        closeFilters(e.target);
        if (e.target.matches('details.filter')) pin(e.target);
        var first = $('input:not([type="hidden"])', e.target);
        if (first) first.focus();
      } else if (e.target.matches('details.filter')) {
        unpin(e.target);
      }
    },
    true,
  );

  // The pinned form follows its header, which moves when a pane that holds it scrolls
  // or the window is resized. It stays open: a scroll somewhere else, such as the
  // sidebar, does not touch it, and a resize may be the keyboard of a phone opening.
  function repin() {
    $$('details.filter[open]').forEach(pin);
  }

  document.addEventListener(
    'scroll',
    function (e) {
      var open = $('details.filter[open]');
      if (open && e.target.contains && e.target.contains(open)) repin();
    },
    true,
  );

  window.addEventListener('resize', repin);

  document.addEventListener('click', function (e) {
    if (!e.target.closest(POPOVERS)) closeFilters(null);
  });

  document.addEventListener('keydown', function (e) {
    if (e.key !== 'Escape') return;
    var open = $('details.filter[open], details.menu[open]');
    if (!open) return;
    closeFilters(null);
    var btn = $('summary', open);
    if (btn) btn.focus();
  });

  document.addEventListener('submit', function (e) {
    var form = e.target;
    if (!form.matches || !form.matches('[data-filter-form]')) return;
    var leaf = form.closest('[data-leaf]');
    if (!leaf) return;
    var params = new URLSearchParams();
    new FormData(form).forEach(function (value, name) {
      if (typeof value === 'string' && value.trim() !== '') params.append(name, value);
    });
    var dest = new URL(form.getAttribute('action'), location.href);
    var href = dest.pathname + (params.toString() ? '?' + params : '');
    e.preventDefault();
    refreshLeaf(leaf.getAttribute('data-leaf'), href, true);
  });

  /* ------------------------------- selection ------------------------------ */

  function syncSelection(scope) {
    $$('.actionbar', scope).forEach(function (bar) {
      var form = bar.closest('form');
      var boxes = $$('input[name="_sel"]', form);
      var n = boxes.filter(function (b) {
        return b.checked;
      }).length;
      var count = $('[data-selcount]', bar);
      if (count) count.textContent = n + ' selected';
      $$('button[name="_act"]', bar).forEach(function (b) {
        b.disabled = n === 0; // a bulk action over nothing is not an action
      });
    });
  }

  // The sidebar's collapse button narrows it to the icon rail and back. The
  // choice is a browser preference, like the theme, so it stays out of the URL.
  document.addEventListener('click', function (e) {
    var btn = e.target.closest('[data-sidebar-toggle]');
    if (!btn) return;
    var root = document.documentElement;
    var collapsed = root.getAttribute('data-sidebar') !== 'collapsed';
    if (collapsed) root.setAttribute('data-sidebar', 'collapsed');
    else root.removeAttribute('data-sidebar');
    syncSidebarToggle();
    try {
      if (collapsed) localStorage.setItem('webui.sidebar', 'collapsed');
      else localStorage.removeItem('webui.sidebar');
    } catch (err) {}
  });

  function syncSidebarToggle() {
    var btn = $('[data-sidebar-toggle]');
    if (!btn) return;
    var collapsed = document.documentElement.getAttribute('data-sidebar') === 'collapsed';
    btn.setAttribute('aria-expanded', collapsed ? 'false' : 'true');
    btn.title = collapsed ? 'Expand sidebar' : 'Collapse sidebar';
  }

  document.addEventListener('change', function (e) {
    var t = e.target;

    if (t.matches('[data-pickall]')) {
      var form = t.closest('form');
      $$('input[name="_sel"]', form).forEach(function (b) {
        b.checked = t.checked;
      });
    }
    if (t.matches('[data-pickall], input[name="_sel"]')) syncSelection(t.closest('form'));

    if (t.matches('[data-pref="theme"]')) {
      var theme = t.checked ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', theme);
      try {
        localStorage.setItem('webui.theme', theme);
      } catch (err) {}
    }
  });

  // A range shows its value while it moves, not only when it is let go.
  document.addEventListener('input', function (e) {
    if (!e.target.matches('input[type="range"]')) return;
    var out = e.target.parentNode.querySelector('[data-range-value]');
    if (out) out.textContent = e.target.value;
  });

  /* -------------------------------- palette ------------------------------- */

  // The page search. Its pages are the compiled app's own navigation, read from
  // the sidebar, so there is no index to fall out of date; pages that offer
  // Search add their own results, fetched as the user types. It is a combobox:
  // arrow keys move through the results, Enter follows one, Escape closes.
  var palette = { items: [], active: -1, moved: false, remote: [], query: '', timer: null, ctl: null };

  function paletteBox() {
    return $('[data-palette]');
  }
  function paletteInput() {
    return $('[data-palette-input]');
  }

  // Only a path on this site is ever put in an href.
  function safeHref(href) {
    return typeof href === 'string' && href.charAt(0) === '/' && href.charAt(1) !== '/' && href.charAt(1) !== '\\';
  }

  function pages() {
    return $$('.side-scroll a.nav-item[href]').map(function (a) {
      // The label, not the whole link: an entry with no icon holds its initial in
      // a span of its own, which is for the collapsed sidebar and not a name.
      var label = a.querySelector('span:not(.ni-letter)');
      return { group: 'Pages', title: (label || a).textContent.trim(), desc: '', href: a.getAttribute('href') };
    });
  }

  function groupsFor(query) {
    var q = query.trim().toLowerCase();
    var local = pages().filter(function (p) {
      return p.title.toLowerCase().indexOf(q) !== -1;
    });
    var groups = [];
    function add(hit) {
      var g = groups.find(function (x) {
        return x.label === hit.group;
      });
      if (!g) groups.push((g = { label: hit.group, hits: [] }));
      g.hits.push(hit);
    }
    local.forEach(add);
    palette.remote.forEach(function (hit) {
      if (safeHref(hit.href)) add(hit);
    });
    return groups;
  }

  function drawPalette() {
    var box = paletteBox();
    var input = paletteInput();
    if (!box || !input) return;

    var previous = palette.items[palette.active];
    box.textContent = '';
    palette.items = [];

    var groups = groupsFor(palette.query);
    groups.forEach(function (g) {
      var label = document.createElement('div');
      label.className = 'pal-label';
      label.textContent = g.label;
      box.appendChild(label);
      g.hits.forEach(function (hit) {
        var a = document.createElement('a');
        a.className = 'pal-item';
        a.id = 'palette-item-' + palette.items.length;
        a.setAttribute('role', 'option');
        a.href = hit.href;
        var title = document.createElement('span');
        title.textContent = hit.title;
        a.appendChild(title);
        if (hit.desc) {
          var desc = document.createElement('span');
          desc.className = 'mono';
          desc.textContent = hit.desc;
          a.appendChild(desc);
        }
        box.appendChild(a);
        palette.items.push({ el: a, href: hit.href });
      });
    });
    if (!palette.items.length) {
      var empty = document.createElement('div');
      empty.className = 'pal-empty';
      empty.textContent = 'Nothing matches.';
      box.appendChild(empty);
    }

    // Results arriving while the user is arrowing through them must not move the
    // highlight out from under them.
    var keep = palette.moved && previous ? palette.items.findIndex(function (i) {
      return i.href === previous.href;
    }) : -1;
    setActive(keep >= 0 ? keep : palette.items.length ? 0 : -1);

    box.classList.add('open');
    input.setAttribute('aria-expanded', 'true');
  }

  function setActive(i) {
    var input = paletteInput();
    palette.active = i;
    palette.items.forEach(function (item, n) {
      item.el.classList.toggle('active', n === i);
      item.el.setAttribute('aria-selected', n === i ? 'true' : 'false');
    });
    if (i >= 0) {
      var el = palette.items[i].el;
      input.setAttribute('aria-activedescendant', el.id);
      el.scrollIntoView({ block: 'nearest' });
    } else {
      input.removeAttribute('aria-activedescendant');
    }
  }

  function closePalette() {
    var box = paletteBox();
    var input = paletteInput();
    if (box) box.classList.remove('open');
    if (input) {
      input.setAttribute('aria-expanded', 'false');
      input.removeAttribute('aria-activedescendant');
    }
    palette.moved = false;
  }

  // Ask the pages that offer Search, after a pause in typing. A newer question
  // replaces an older one, and a failure leaves just the navigation.
  function askRemote(query) {
    var input = paletteInput();
    var url = input && input.getAttribute('data-search-url');
    clearTimeout(palette.timer);
    if (palette.ctl) palette.ctl.abort();
    palette.remote = [];
    if (!url || !query.trim()) return;
    palette.timer = setTimeout(async function () {
      var ctl = new AbortController();
      palette.ctl = ctl;
      try {
        var res = await fetch(url + '?q=' + encodeURIComponent(query), {
          headers: { Accept: 'application/json' },
          credentials: 'same-origin',
          signal: ctl.signal,
        });
        if (!res.ok) return;
        var body = await res.json();
        if (query !== palette.query) return;
        palette.remote = Array.isArray(body.results) ? body.results : [];
        drawPalette();
      } catch (err) {
        /* the navigation results stand */
      }
    }, 150);
  }

  function openPalette() {
    var input = paletteInput();
    if (!input) return;
    palette.query = input.value;
    palette.moved = false;
    drawPalette();
    askRemote(palette.query);
  }

  document.addEventListener('focusin', function (e) {
    if (e.target.matches('[data-palette-input]')) openPalette();
  });

  document.addEventListener('input', function (e) {
    if (e.target.matches('[data-palette-input]')) openPalette();
  });

  document.addEventListener('mouseover', function (e) {
    var item = e.target.closest('.pal-item');
    if (!item) return;
    var i = palette.items.findIndex(function (x) {
      return x.el === item;
    });
    if (i >= 0 && i !== palette.active) {
      palette.moved = true;
      setActive(i);
    }
  });

  // Cmd+K or Ctrl+K, whichever the platform has; the hint shows the usual one.
  // It is read in the capture phase so nothing in the page can swallow it, and by
  // physical key as well as by character, so it works on layouts where K is not
  // "k".
  var isMac = /Mac|iPhone|iPad/.test(navigator.platform || '');

  function isShortcut(e) {
    return (e.metaKey || e.ctrlKey) && ((e.key || '').toLowerCase() === 'k' || e.code === 'KeyK');
  }

  document.addEventListener(
    'keydown',
    function (e) {
      var input = paletteInput();
      if (isShortcut(e) && input) {
        e.preventDefault();
        input.focus();
        input.select();
        return;
      }
      handlePaletteKey(e, input);
    },
    true,
  );

  function handlePaletteKey(e, input) {
    if (!input || e.target !== input) return;

    var open = paletteBox().classList.contains('open');
    var n = palette.items.length;
    switch (e.key) {
      case 'ArrowDown':
      case 'ArrowUp':
        e.preventDefault();
        if (!open) {
          openPalette();
          return;
        }
        if (!n) return;
        palette.moved = true;
        setActive(e.key === 'ArrowDown' ? (palette.active + 1) % n : (palette.active - 1 + n) % n);
        break;
      case 'Enter':
        if (open && palette.active >= 0) {
          e.preventDefault();
          location.href = palette.items[palette.active].href;
        }
        break;
      case 'Escape':
        closePalette();
        break;
      case 'Tab':
        closePalette();
        break;
    }
  }

  document.addEventListener('click', function (e) {
    if (!e.target.closest('.search') && !e.target.closest('[data-palette]')) closePalette();
  });

  /* ------------------------------ choice lists ----------------------------- */

  // A select is a real <select> in the markup, and that is what is submitted. With script it is
  // hidden, and a field takes its place that shows what is chosen as the badges it is when read,
  // over a list of the options in which a click ticks one. The field is a combobox: Enter, Space and
  // the arrow keys open it and move through the list, Enter or Space ticks, Backspace unticks the
  // last of several, Escape closes. It changes the <select>, and tells it so, and nothing else.

  var selectCount = 0;

  function make(tag, cls, text) {
    var node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text != null) node.textContent = text;
    return node;
  }

  function enhanceSelect(sel) {
    var multi = sel.multiple;
    var required = sel.required;
    var id = sel.id || 'select-' + ++selectCount;
    var options = Array.prototype.filter.call(sel.options, function (o) {
      return o.value !== '';
    });
    var emptyText = required ? 'Choose…' : 'None';

    var wrap = make('div', 'msel' + (multi ? ' msel-multi' : ''));
    var field = make('div', 'input msel-field' + (sel.classList.contains('invalid') ? ' invalid' : ''));
    field.id = id + '-field';
    field.tabIndex = 0;
    field.setAttribute('role', 'combobox');
    field.setAttribute('aria-haspopup', 'listbox');
    field.setAttribute('aria-expanded', 'false');
    field.setAttribute('aria-controls', id + '-list');
    var labelled = document.getElementById(id + '-label');
    if (labelled) field.setAttribute('aria-labelledby', id + '-label');
    ['aria-invalid', 'aria-describedby'].forEach(function (name) {
      if (sel.hasAttribute(name)) field.setAttribute(name, sel.getAttribute(name));
    });
    var chips = make('span', 'badges');
    var hint = make('span', 'msel-hint', emptyText);
    var chevron = make('span', 'msel-chevron');
    chevron.setAttribute('aria-hidden', 'true');
    field.appendChild(chips);
    field.appendChild(hint);
    field.appendChild(chevron);

    var list = make('div', 'msel-pop');
    list.id = id + '-list';
    list.setAttribute('role', 'listbox');
    list.hidden = true;
    if (multi) list.setAttribute('aria-multiselectable', 'true');
    if (labelled) list.setAttribute('aria-labelledby', id + '-label');

    // One row for each option, and for a select of one that may be left empty, a row for none.
    var rows = [];
    function row(option, none) {
      var r = make('div', 'msel-opt' + (none ? ' msel-none' : ''));
      r.id = id + '-opt-' + rows.length;
      r.setAttribute('role', 'option');
      r.appendChild(make('span', 'msel-check'));
      if (none) {
        r.appendChild(make('span', 'msel-none-text', 'None'));
      } else {
        r.appendChild(badge(option));
      }
      r.option = option;
      rows.push(r);
      list.appendChild(r);
    }
    if (!multi && !required) row(null, true);
    options.forEach(function (o) {
      row(o, false);
    });

    function badge(option) {
      var b = make('span', 'badge ' + (option.getAttribute('data-badge') || 'badge-mute'));
      var dot = make('span', 'dot');
      dot.setAttribute('aria-hidden', 'true');
      b.appendChild(dot);
      b.appendChild(document.createTextNode(option.textContent));
      return b;
    }

    var active = -1;
    function chosen() {
      return options.filter(function (o) {
        return o.selected;
      });
    }
    function draw() {
      var picked = chosen();
      chips.textContent = '';
      picked.forEach(function (o) {
        chips.appendChild(badge(o));
      });
      hint.hidden = picked.length > 0;
      rows.forEach(function (r, i) {
        var on = r.option ? r.option.selected : picked.length === 0;
        r.setAttribute('aria-selected', on ? 'true' : 'false');
        r.classList.toggle('on', on);
        r.classList.toggle('active', i === active);
      });
      if (active >= 0) {
        field.setAttribute('aria-activedescendant', rows[active].id);
      } else {
        field.removeAttribute('aria-activedescendant');
      }
    }
    function changed() {
      field.classList.remove('invalid');
      field.removeAttribute('aria-invalid');
      sel.dispatchEvent(new Event('change', { bubbles: true }));
      draw();
    }
    function toggle(r) {
      if (!r.option) {
        options.forEach(function (o) {
          o.selected = false;
        });
        sel.value = '';
      } else if (multi) {
        r.option.selected = !r.option.selected;
      } else {
        options.forEach(function (o) {
          o.selected = o === r.option;
        });
      }
      changed();
      if (!multi) close(true);
    }

    function isOpen() {
      return !list.hidden;
    }
    function open() {
      if (isOpen()) return;
      closeSelects(wrap);
      list.hidden = false;
      wrap.classList.add('open');
      field.setAttribute('aria-expanded', 'true');
      // Above the field when there is not room beneath it and there is above.
      var box = field.getBoundingClientRect();
      var room = window.innerHeight - box.bottom;
      list.classList.toggle('up', room < Math.min(list.scrollHeight, 260) + 12 && box.top > room);
      var first = rows.findIndex(function (r) {
        return r.classList.contains('on') && r.option;
      });
      move(first >= 0 ? first : 0);
    }
    function close(refocus) {
      if (!isOpen()) return;
      list.hidden = true;
      wrap.classList.remove('open');
      field.setAttribute('aria-expanded', 'false');
      active = -1;
      draw();
      if (refocus) field.focus();
    }
    function move(i) {
      active = Math.max(0, Math.min(rows.length - 1, i));
      draw();
      rows[active].scrollIntoView({ block: 'nearest' });
    }
    wrap.close = close;

    field.addEventListener('click', function () {
      if (isOpen()) close(false);
      else open();
    });
    // The list takes the click, and the focus stays on the field.
    list.addEventListener('mousedown', function (e) {
      e.preventDefault();
    });
    list.addEventListener('click', function (e) {
      var r = e.target.closest('.msel-opt');
      if (r) toggle(r);
    });
    list.addEventListener('mousemove', function (e) {
      var r = e.target.closest('.msel-opt');
      if (r && rows.indexOf(r) !== active) {
        active = rows.indexOf(r);
        draw();
      }
    });
    field.addEventListener('keydown', function (e) {
      if (e.altKey || e.ctrlKey || e.metaKey) return;
      var key = e.key;
      if (!isOpen()) {
        if (key === 'ArrowDown' || key === 'ArrowUp' || key === 'Enter' || key === ' ') {
          e.preventDefault();
          open();
        } else if (multi && (key === 'Backspace' || key === 'Delete')) {
          var picked = chosen();
          if (picked.length) {
            picked[picked.length - 1].selected = false;
            changed();
          }
        }
        return;
      }
      switch (key) {
        case 'ArrowDown':
          e.preventDefault();
          move(active + 1);
          break;
        case 'ArrowUp':
          e.preventDefault();
          move(active - 1);
          break;
        case 'Home':
          e.preventDefault();
          move(0);
          break;
        case 'End':
          e.preventDefault();
          move(rows.length - 1);
          break;
        case 'Enter':
        case ' ':
          e.preventDefault();
          if (active >= 0) toggle(rows[active]);
          break;
        case 'Escape':
          e.preventDefault();
          e.stopPropagation();
          close(true);
          break;
        case 'Tab':
          close(false);
          break;
        default:
          // Typing the start of an option moves to the next one that begins so.
          if (key.length === 1) {
            var typed = key.toLowerCase();
            for (var n = 1; n <= rows.length; n++) {
              var at = (active + n) % rows.length;
              if (rows[at].textContent.trim().toLowerCase().indexOf(typed) === 0) {
                move(at);
                break;
              }
            }
          }
      }
    });
    field.addEventListener('focusout', function (e) {
      if (!wrap.contains(e.relatedTarget)) close(false);
    });

    // A select that must have a choice says so before the form leaves, since the <select> that
    // would have said so is hidden and cannot be pointed at. The server asks as well.
    if (required) {
      sel.required = false;
      var form = sel.form;
      if (form) {
        form.addEventListener('submit', function (e) {
          if (chosen().length === 0) {
            e.preventDefault();
            field.classList.add('invalid');
            field.setAttribute('aria-invalid', 'true');
            field.focus();
          }
        });
      }
    }
    if (labelled) {
      labelled.addEventListener('click', function (e) {
        e.preventDefault();
        field.focus();
      });
    }

    sel.hidden = true;
    sel.setAttribute('data-enhanced', '');
    wrap.appendChild(field);
    wrap.appendChild(list);
    sel.parentNode.insertBefore(wrap, sel);
    draw();
  }

  function closeSelects(except) {
    $$('.msel.open').forEach(function (w) {
      if (w !== except && w.close) w.close(false);
    });
  }

  function enhanceSelects(root) {
    $$('select[data-select]:not([data-enhanced])', root).forEach(enhanceSelect);
  }

  document.addEventListener('click', function (e) {
    if (!e.target.closest('.msel')) closeSelects(null);
  });

  // A panel that is fetched again brings its selects as plain ones.
  new MutationObserver(function () {
    enhanceSelects(document);
  }).observe(document.body, { childList: true, subtree: true });

  /* --------------------------------- start -------------------------------- */

  // The hint beside the search box says the key this platform has.
  var hint = $('.search .kbd');
  if (hint && !isMac) hint.textContent = 'Ctrl K';

  var theme = $('[data-pref="theme"]');
  if (theme) {
    // Reflect what is on screen: an explicit choice, else the system's.
    var explicit = document.documentElement.getAttribute('data-theme');
    theme.checked = explicit ? explicit === 'light' : window.matchMedia('(prefers-color-scheme: light)').matches;
  }
  syncSidebarToggle();
  syncSelection(document);
  enhanceSelects(document);
  endArrival();
  $$('[data-tabs]').forEach(function (set) {
    showActiveTab(set);
  });
  startStatus();
  dismissToasts();
})();
