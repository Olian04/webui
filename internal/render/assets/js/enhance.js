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
    syncToolbar(href);

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

  // The toolbar carries the rest of the address in its controls: hidden inputs
  // so setting a filter keeps the sort, and links that clear one filter. A panel
  // that moved changed that rest, so they are rebuilt from the new address. The
  // library's view state is the parameters containing a dot; offsets are never
  // carried, because a changed filter returns every table to its first page.
  function syncToolbar(href) {
    var url = new URL(href, location.href);
    var carried = [];
    url.searchParams.forEach(function (value, name) {
      carried.push([name, value]);
    });
    function without(skip) {
      var q = new URLSearchParams();
      carried.forEach(function (kv) {
        if (kv[0] !== skip && !/\.offset$/.test(kv[0])) q.append(kv[0], kv[1]);
      });
      return q;
    }
    $$('.toolbar .var').forEach(function (pill) {
      var form = $('.var-form', pill);
      var control = form && $('input:not([type="hidden"]), select', form);
      if (!control) return;
      $$('input[type="hidden"]', form).forEach(function (h) {
        h.remove();
      });
      without(control.name).forEach(function (value, name) {
        var h = document.createElement('input');
        h.type = 'hidden';
        h.name = name;
        h.value = value;
        form.insertBefore(h, form.firstChild);
      });
      var clear = $('.var-x', pill);
      if (clear) {
        var rest = without(control.name).toString();
        clear.setAttribute('href', url.pathname + (rest ? '?' + rest : ''));
      }
    });
    var all = $$('.toolbar .tb-right a').find(function (a) {
      return a.textContent.trim() === 'Clear all';
    });
    if (all) {
      var keep = new URLSearchParams();
      carried.forEach(function (kv) {
        if (kv[0].indexOf('.') !== -1 && !/\.offset$/.test(kv[0])) keep.append(kv[0], kv[1]);
      });
      all.setAttribute('href', url.pathname + (keep.toString() ? '?' + keep : ''));
    }
  }

  document.addEventListener('click', function (e) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;

    var kebab = e.target.closest('[data-act="refresh-leaf"]');
    if (kebab) {
      e.preventDefault();
      refreshLeaf(kebab.getAttribute('data-leaf'), location.pathname + location.search, false);
      return;
    }

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

  document.addEventListener('change', function (e) {
    var t = e.target;

    if (t.matches('[data-pickall]')) {
      var form = t.closest('form');
      $$('input[name="_sel"]', form).forEach(function (b) {
        b.checked = t.checked;
      });
    }
    if (t.matches('[data-pickall], input[name="_sel"]')) syncSelection(t.closest('form'));

    // A toolbar control that cannot be typed into applies as soon as it changes.
    if (t.matches('[data-autosubmit]') && t.form) t.form.requestSubmit();

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

  // The list is the compiled app's own navigation, read from the sidebar, so
  // there is no index to fall out of date.
  function pages() {
    return $$('.side-scroll a.nav-item[href]').map(function (a) {
      return { label: a.textContent.trim(), href: a.getAttribute('href') };
    });
  }

  function renderPalette(box, query) {
    var q = query.trim().toLowerCase();
    var hits = pages().filter(function (p) {
      return p.label.toLowerCase().indexOf(q) !== -1;
    });
    box.textContent = '';
    var label = document.createElement('div');
    label.className = 'pal-label';
    label.textContent = 'Pages';
    box.appendChild(label);
    if (!hits.length) {
      var empty = document.createElement('div');
      empty.className = 'pal-empty';
      empty.textContent = 'Nothing matches.';
      box.appendChild(empty);
    }
    hits.forEach(function (p) {
      var a = document.createElement('a');
      a.className = 'pal-item';
      a.href = p.href;
      a.textContent = p.label;
      box.appendChild(a);
    });
    box.classList.add('open');
  }

  document.addEventListener('input', function (e) {
    if (!e.target.matches('[data-palette-input]')) return;
    var box = $('[data-palette]');
    if (box) renderPalette(box, e.target.value);
  });

  document.addEventListener('keydown', function (e) {
    var input = $('[data-palette-input]');
    var box = $('[data-palette]');
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k' && input) {
      e.preventDefault();
      input.focus();
    }
    if (e.key === 'Escape' && box) box.classList.remove('open');
    if (e.key === 'Enter' && input && e.target === input && box) {
      var first = $('.pal-item', box);
      if (first) location.href = first.getAttribute('href');
    }
  });

  document.addEventListener('click', function (e) {
    var box = $('[data-palette]');
    if (box && !e.target.closest('.search') && !e.target.closest('[data-palette]')) box.classList.remove('open');
  });

  /* --------------------------------- start -------------------------------- */

  var theme = $('[data-pref="theme"]');
  if (theme) {
    // Reflect what is on screen: an explicit choice, else the system's.
    var explicit = document.documentElement.getAttribute('data-theme');
    theme.checked = explicit ? explicit === 'light' : window.matchMedia('(prefers-color-scheme: light)').matches;
  }
  syncSelection(document);
  dismissToasts();
})();
