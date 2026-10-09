// Progressive enhancement. Every control in the page is a real link or form and
// the app works with this file switched off; the script changes how much of the
// page a followed link replaces, and adds a few conveniences that need script.
//
// The rule that decides what a click does: a link to a DIFFERENT path is a
// navigation and is left alone (the browser handles it, and the CSS view
// transition cross-fades it). A link to the SAME path with different arguments,
// inside a panel, belongs to that panel, so only that panel is replaced.
(function () {
  'use strict';

  var LEAF_HEADER = 'X-Webui-Leaf';
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

  /* -------------------------------- toasts -------------------------------- */

  // Toasts the server rendered (a saved form, a rejected one) dismiss
  // themselves; without script they stay until the next page, which is fine.
  function dismissToasts() {
    $$('.toast').forEach(function (el) {
      setTimeout(function () {
        el.classList.add('out');
        setTimeout(function () {
          el.remove();
        }, 220);
      }, 3400);
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

  // The address is the truth. Going back or forward to a panel state reloads
  // the page, which is correct and needs no history bookkeeping.
  window.addEventListener('popstate', function () {
    location.reload();
  });

  /* ------------------------------ column filters --------------------------- */

  // A filter is a <details>, which opens and submits on its own. Script only
  // keeps one open at a time, closes it on a click elsewhere or on Escape, and
  // applies a filter to its panel alone, like any other link inside it.
  // The settings menu is a <details> too, and shares all of it: one popover open
  // at a time, closed by a click elsewhere or Escape.
  var POPOVERS = 'details.filter, details.menu';
  function closeFilters(except) {
    $$('details.filter[open], details.menu[open]').forEach(function (d) {
      if (d !== except) d.removeAttribute('open');
    });
  }

  document.addEventListener(
    'toggle',
    function (e) {
      if (e.target.matches && e.target.matches(POPOVERS) && e.target.open) {
        closeFilters(e.target);
        var first = $('input:not([type="hidden"])', e.target);
        if (first) first.focus();
      }
    },
    true,
  );

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
  dismissToasts();
})();
