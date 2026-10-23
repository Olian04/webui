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
  // A panel that was fetched again says how it came to be in elements that follow it, each saying which element of the bar it replaces
  // (data-replaces, a selector): they are put in their places, and the bar says the latest.
  function noteResponse(fragment, took) {
    fragment.querySelectorAll('[data-oob] > [data-replaces]').forEach(function (el) {
      var old = $(el.getAttribute('data-replaces'));
      el.removeAttribute('data-replaces');
      if (old) old.replaceWith(el);
    });
    if ($('[data-built]')) checkFresh('panel');
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

  // While one panel reloads it says so with aria-busy, which the stylesheet draws as its rows pulsing and out of reach, and nothing else moves: the rows are
  // the ones that were there, and a request that fails leaves them as they were. This says "this panel is reloading"; a whole-page fade would say "the page moved".

  // The same URL, asking for one panel. Returns the panel element, or throws
  // when the answer is anything but that panel: the caller falls back to a
  // plain navigation, which is what the link was all along.
  async function fetchLeaf(id, href, signal, marker) {
    marker = marker || 'data-leaf'; // a panel, or the set of tabs, which is named by its own attribute
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
    if (!next || next.getAttribute(marker) !== id) throw new Error('not that panel');
    noteResponse(tpl.content, performance.now() - started);
    return next;
  }

  async function refreshLeaf(id, href, push) {
    var el = $(leafSelector(id));
    if (!el) return false;

    var previous = inflight.get(id);
    if (previous) previous.abort();
    var ctl = new AbortController();
    inflight.set(id, ctl);

    el.setAttribute('aria-busy', 'true');
    status('GET', href, 'panel ' + id);
    try {
      var next = await fetchLeaf(id, href, ctl.signal);
      el.replaceWith(next);
      if (push) moveTo(href);
      settle(href, next);
    } catch (err) {
      if (err && err.name === 'AbortError') return true; // a newer request took over
      location.href = href;
    } finally {
      if (inflight.get(id) === ctl) inflight.delete(id);
    }
    return true;
  }

  /* ------------------------ a panel that is not final ---------------------- */

  // A panel whose Load said that it is not final (a job that it shows is still going) has data-refresh-after, the seconds to wait, and says so in its head.
  // It is asked for again after that, the same GET with the header that asks for one panel, and replaced by the answer, which says so again or does not: the
  // server ends the loop by not saying it, and script has no end of its own. A poll is not a navigation: the address does not move, nothing is pushed, and
  // the panels around it have nothing new to say. It waits while the tab is hidden, and while focus is in the panel, which a reader or someone typing would
  // lose; and a request that fails ends it, with the panel as it was, which still says to reload.
  var armed = new WeakSet();

  function armRefresh(scope) {
    $$('[data-leaf][data-refresh-after]', scope).concat(scope.matches && scope.matches('[data-leaf][data-refresh-after]') ? [scope] : []).forEach(function (el) {
      if (armed.has(el)) return;
      armed.add(el);
      var secs = parseInt(el.getAttribute('data-refresh-after'), 10);
      if (secs > 0) setTimeout(function () { pollLeaf(el, secs); }, secs * 1000);
    });
  }

  async function pollLeaf(el, secs) {
    if (!el.isConnected) return; // replaced since: the one that replaced it has its own say
    var id = el.getAttribute('data-leaf');
    if (document.hidden) {
      document.addEventListener('visibilitychange', function again() {
        if (document.hidden) return;
        document.removeEventListener('visibilitychange', again);
        pollLeaf(el, secs);
      });
      return;
    }
    if (el.contains(document.activeElement) || inflight.has(id)) {
      setTimeout(function () { pollLeaf(el, secs); }, secs * 1000);
      return;
    }
    var ctl = new AbortController();
    inflight.set(id, ctl);
    try {
      var next = await fetchLeaf(id, location.pathname + location.search, ctl.signal);
      if (el.isConnected) el.replaceWith(next);
    } catch (err) {
      // The panel stays as it was, and says to reload.
    } finally {
      if (inflight.get(id) === ctl) inflight.delete(id);
    }
  }

  armRefresh(document);
  new MutationObserver(function (changes) {
    changes.forEach(function (c) {
      c.addedNodes.forEach(function (n) {
        if (n.nodeType === 1) armRefresh(n);
      });
    });
  }).observe(document.body, { childList: true, subtree: true });

  // After one panel, or a strip of tabs, moved to a new address, the others hold addresses of the old one: the sort and pager links of the panels around it, the
  // links of the other strips of tabs, and the action of a form, which is where it posts to and so where it comes back to. This script does not know how an address is
  // made, so it does not put the new one in: it asks the server for those panels at the new address, which writes them. A panel that is read, and has links that replace
  // it, is replaced; a form keeps what the visitor typed in it and takes the action of the server's copy; the strip of another set of tabs is replaced and its panel is not.
  // `moved` is the element that was just fetched, which is as new as the address.
  function syncPanels(href, moved) {
    var apart = function (el) {
      return !moved || (el !== moved && !moved.contains(el) && !el.contains(moved));
    };
    $$('[data-leaf]')
      .filter(apart)
      .forEach(function (el) {
        var id = el.getAttribute('data-leaf');
        var replaces = $('a[data-in-place]', el);
        var forms = $$('form[method="post"]', el);
        if (!replaces && !forms.length) return;
        fetchLeaf(id, href)
          .then(function (next) {
            if (replaces) {
              var current = $(leafSelector(id));
              if (current) current.replaceWith(next);
              return;
            }
            var fresh = $$('form[method="post"]', next);
            if (fresh.length !== forms.length) return;
            forms.forEach(function (f, i) {
              f.setAttribute('action', fresh[i].getAttribute('action') || '');
            });
          })
          .catch(function () {});
      });
    $$('[data-tabs]')
      .filter(function (set) {
        return !moved || (set !== moved && !moved.contains(set)); // the strip of a set that holds the moved panel is still stale
      })
      .forEach(function (set) {
        var id = set.getAttribute('data-tabs');
        fetchLeaf(id, href, undefined, 'data-tabs')
          .then(function (next) {
            var strip = $('.tabs', set);
            var fresh = $('.tabs', next);
            if (strip && fresh) strip.replaceWith(fresh);
          })
          .catch(function () {});
      });
  }

  // The same address, and the Refresh button leads there too.
  function settle(href, moved) {
    status('GET', href);
    var refresh = $('[data-refresh]');
    if (refresh) refresh.setAttribute('href', href);
    syncPanels(href, moved);
  }

  // Something that script did on the server, a file that was sent, may have changed what the other panels show: they are asked for again, from the
  // server, as they are by a link that replaces one. A panel that is a form is left alone, since it holds what the visitor has typed; a table is not,
  // though it has a form for its filters, which are in the address and so are asked for again with it.
  document.addEventListener('webui:changed', function (e) {
    var from = e.detail && e.detail.leaf;
    $$('[data-leaf]').forEach(function (leaf) {
      var id = leaf.getAttribute('data-leaf');
      if (id === from || ($('form', leaf) && !$('table', leaf))) return;
      fetchLeaf(id, location.pathname + location.search)
        .then(function (next) {
          var current = $(leafSelector(id));
          if (current) current.replaceWith(next);
        })
        .catch(function () {});
    });
  });

  document.addEventListener('click', function (e) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;

    // A link replaces its panel in place when the server says so (data-in-place); any other is a navigation, which is what a link is. The address must still be this page's,
    // since the panel is asked for at the same path, and anything else is left to the browser.
    var a = e.target.closest('a[href][data-in-place]');
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
  // field, through the Constraint Validation API. The server writes what to say for each way a value can fail, as an
  // attribute for each, data-msg- and the name of the ValidityState property that reports it in kebab case
  // (data-msg-value-missing); when a control reports itself invalid, the first of them that is true goes to
  // setCustomValidity, and it is taken back when the value changes, since a custom message keeps a control invalid until it
  // is. Without script the browser says its own words, and the pattern's message, which is the control's title.
  // A pattern attribute is one expression, and a field that has more than one condition on its text (the text rules, which
  // are patterns to the browser, and the pattern) has them all in it, so the browser only says that the value does not
  // match. data-part-N has each condition as an expression, and data-part-N-msg the words for it, in the order the server
  // tests them, and the first that the value does not match is the one said. An expression that this browser cannot read
  // leaves the words for the pattern as a whole.
  function failedPart(el, value) {
    for (var i = 0; el.hasAttribute('data-part-' + i); i++) {
      var expr = el.getAttribute('data-part-' + i);
      var re = null;
      // The v flag is what the HTML specification compiles a pattern with, and u what a browser without it does.
      try {
        re = new RegExp('^(?:' + expr + ')$', 'v');
      } catch (err) {
        try {
          re = new RegExp('^(?:' + expr + ')$', 'u');
        } catch (err2) {
          return '';
        }
      }
      if (!re.test(value)) return el.getAttribute('data-part-' + i + '-msg') || '';
    }
    return '';
  }

  // The server's order: missing, then unreadable, then the length, the pattern, and the bounds.
  var VALIDITY_ORDER = ['valueMissing', 'typeMismatch', 'badInput', 'stepMismatch', 'tooShort', 'tooLong', 'patternMismatch', 'rangeUnderflow', 'rangeOverflow'];

  function messageOf(el, reason) {
    return el.getAttribute('data-msg-' + reason.replace(/[A-Z]/g, function (c) { return '-' + c.toLowerCase(); }));
  }

  function hasMessages(el) {
    return !!el.attributes && Array.prototype.some.call(el.attributes, function (a) { return a.name.indexOf('data-msg-') === 0; });
  }

  document.addEventListener(
    'invalid',
    function (e) {
      var el = e.target;
      if (!hasMessages(el)) return;
      for (var i = 0; i < VALIDITY_ORDER.length; i++) {
        var reason = VALIDITY_ORDER[i];
        var message = el.validity[reason] && messageOf(el, reason);
        if (message) {
          el.setCustomValidity((reason === 'patternMismatch' && failedPart(el, el.value)) || message);
          return;
        }
      }
    },
    true,
  );

  function takeBackValidity(e) {
    var el = e.target;
    if (hasMessages(el) && el.validity && el.validity.customError) el.setCustomValidity('');
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
      noteResponse(tpl.content, performance.now() - started);
      set.replaceWith(next);
      showActiveTab(next, scrolled);
      moveTo(href);
      settle(href, next);
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
  // the page, which is correct and needs no history bookkeeping. A change of the fragment alone (the skip link, a link to
  // a heading) is not another state of the page, and fires the same event, so it is left to the browser.
  var shownAt = location.pathname + location.search;
  function moveTo(href) {
    history.pushState(null, '', href);
    shownAt = href;
  }
  window.addEventListener('popstate', function () {
    if (location.pathname + location.search !== shownAt) location.reload();
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

  /* ------------------------------ description tips ------------------------------ */

  // The popover of a description (an information icon) opens beside its icon, to the right and below, which is out of view for an icon near
  // the right edge or the bottom. When it opens, script measures it and moves it back in: left until its right edge is inside, or above the icon
  // when there is no room below. "Inside" is the window, less the status bar, and the visible part of every box that scrolls or clips and
  // holds it, a scrollbar not being part of it: what is outside one of them is cut off, however much room the window has. Without script it
  // opens where the stylesheet puts it.
  var TIP_MARGIN = 8;
  var STATUS_BAR = 26; // fixed along the bottom of the window

  function clipBounds(el) {
    var b = { left: 0, top: 0, right: window.innerWidth, bottom: window.innerHeight - STATUS_BAR };
    for (var n = el.parentElement; n && n !== document.documentElement; n = n.parentElement) {
      var style = getComputedStyle(n);
      if (style.overflowX === 'visible' && style.overflowY === 'visible') continue;
      var r = n.getBoundingClientRect();
      b.left = Math.max(b.left, r.left + n.clientLeft);
      b.top = Math.max(b.top, r.top + n.clientTop);
      b.right = Math.min(b.right, r.left + n.clientLeft + n.clientWidth);
      b.bottom = Math.min(b.bottom, r.top + n.clientTop + n.clientHeight);
    }
    return b;
  }

  function placeTip(tip) {
    var body = $('.tip-body', tip);
    if (!body) return;
    body.style.left = body.style.top = body.style.bottom = '';
    var r = body.getBoundingClientRect();
    if (!r.width) return; // not shown
    var inside = clipBounds(tip);
    var shift = 0;
    if (r.right > inside.right - TIP_MARGIN) shift = inside.right - TIP_MARGIN - r.right;
    if (r.left + shift < inside.left + TIP_MARGIN) shift = inside.left + TIP_MARGIN - r.left;
    if (shift) body.style.left = body.offsetLeft + shift + 'px';
    if (r.bottom > inside.bottom - TIP_MARGIN && tip.getBoundingClientRect().top - r.height - 6 > inside.top + TIP_MARGIN) {
      body.style.top = 'auto';
      body.style.bottom = 'calc(100% + 6px)';
    }
  }

  // A reason that a gated control gives (the tooltip of a control that is not allowed) is centred over it, which is out of view for one at either
  // edge: it is anchored to the side that is near the edge instead. It is above the control, which is out of view for one at the top of
  // what holds it (the first row of a table, whose header is above it, or a toolbar): it is below it instead, when there is no room above.
  // How tall it is, it being drawn by the stylesheet, is estimated from its text.
  function placeGate(gate) {
    var inside = clipBounds(gate);
    var at = gate.getBoundingClientRect();
    var half = Math.min(140, (inside.right - inside.left) / 2 - TIP_MARGIN);
    var middle = at.left + at.width / 2;
    if (middle - half < inside.left + TIP_MARGIN) gate.setAttribute('data-edge', 'left');
    else if (middle + half > inside.right - TIP_MARGIN) gate.setAttribute('data-edge', 'right');
    else gate.removeAttribute('data-edge');

    var table = gate.closest('table');
    var top = table && table.tHead ? Math.max(inside.top, table.tHead.getBoundingClientRect().bottom) : inside.top; // a header stays over what scrolls under it
    var lines = Math.ceil(((gate.getAttribute('data-guard') || '').length * 6.5) / 262);
    var height = 12 + 16 * Math.max(1, lines);
    if (at.top - 6 - height < top + TIP_MARGIN && at.bottom + 6 + height < inside.bottom - TIP_MARGIN) gate.setAttribute('data-below', '');
    else gate.removeAttribute('data-below');
  }

  function onTipOpen(e) {
    var el = e.target.closest && e.target.closest('.tip, .gate');
    if (!el) return;
    if (el.classList.contains('gate')) placeGate(el);
    else requestAnimationFrame(function () { placeTip(el); }); // after the style that shows it has been applied
  }
  document.addEventListener('mouseover', onTipOpen);
  document.addEventListener('focusin', onTipOpen);

  /* ------------------------------- confirm -------------------------------- */

  // A button that asks before it acts opens a <dialog> with an invoker command, which a current browser does without script.
  // For one that does not know the commands, a click does the same, and a click on the backdrop closes the dialog, as the
  // attribute closedby="any" says.
  if (!('commandForElement' in HTMLButtonElement.prototype)) {
    document.addEventListener('click', function (e) {
      var btn = e.target.closest && e.target.closest('button[command][commandfor]');
      if (btn && !btn.disabled) {
        var dialog = document.getElementById(btn.getAttribute('commandfor'));
        if (dialog && dialog.tagName === 'DIALOG') {
          if (btn.getAttribute('command') === 'show-modal' && !dialog.open) dialog.showModal();
          else if (btn.getAttribute('command') === 'close') dialog.close();
        }
        return;
      }
      if (e.target.tagName === 'DIALOG' && (e.target.classList.contains('confirm') || e.target.classList.contains('preview-dialog'))) e.target.close();
    });
  }

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
      $$('button[name="_act"], button[data-confirm-open]', bar).forEach(function (b) {
        if (b.closest('.gate')) return; // a button that its Guard disabled is not made enabled by a selection
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

  // The sidebar's edge is a handle that script adds, to drag or to move with the arrow keys: a window splitter. Its width is a browser
  // preference, like the collapse, kept in localStorage and put back by prefs.js before first paint. A window too small for the width
  // it was given holds it to what fits, without storing the smaller one.
  var SIDE_DEFAULT = 240;
  var SIDE_MIN = 180;
  function sideMax() {
    return Math.max(SIDE_MIN, Math.min(480, window.innerWidth / 2));
  }
  function enhanceSidebarWidth() {
    var side = $('.sidebar');
    if (!side) return;
    var handle = document.createElement('div');
    handle.className = 'side-resize';
    handle.setAttribute('role', 'separator');
    handle.setAttribute('aria-orientation', 'vertical');
    handle.setAttribute('aria-label', 'Resize sidebar');
    handle.tabIndex = 0;
    if (!side.id) side.id = 'sidebar';
    handle.setAttribute('aria-controls', side.id);
    side.appendChild(handle);

    var root = document.documentElement;
    var want = parseInt(root.style.getPropertyValue('--sidebar-w'), 10) || 0; // what was chosen, which a small window may not fit; 0 for the default
    function clamp(px) {
      return Math.round(Math.max(SIDE_MIN, Math.min(sideMax(), px)));
    }
    function show() {
      if (want) root.style.setProperty('--sidebar-w', clamp(want) + 'px');
      else root.style.removeProperty('--sidebar-w');
      handle.setAttribute('aria-valuemin', String(SIDE_MIN));
      handle.setAttribute('aria-valuemax', String(Math.round(sideMax())));
      handle.setAttribute('aria-valuenow', String(Math.round(side.getBoundingClientRect().width)));
    }
    function choose(px) {
      want = clamp(px);
      show();
      try {
        localStorage.setItem('webui.sidebar.width', String(want));
      } catch (err) {}
    }
    function reset() {
      want = 0;
      show();
      try {
        localStorage.removeItem('webui.sidebar.width');
      } catch (err) {}
    }

    // A drag ends however the pointer is let go of, and the cursor that the whole page takes while it lasts must not outlive it: the button
    // can be released outside the window, over a menu, or after the window has lost focus, and then the handle gets no pointerup.
    var from = null;
    function drop() {
      from = null;
      root.classList.remove('sidebar-dragging');
    }
    handle.addEventListener('pointerdown', function (e) {
      if (e.button !== 0) return;
      drop();
      from = { x: e.clientX, w: side.getBoundingClientRect().width };
      handle.setPointerCapture(e.pointerId);
      root.classList.add('sidebar-dragging');
      e.preventDefault();
    });
    handle.addEventListener('pointermove', function (e) {
      if (!from) return;
      if (e.buttons === 0) drop(); // let go somewhere that was not told
      else choose(from.w + e.clientX - from.x);
    });
    ['pointerup', 'pointercancel', 'lostpointercapture', 'contextmenu'].forEach(function (name) {
      handle.addEventListener(name, drop);
    });
    window.addEventListener('blur', drop);
    window.addEventListener('pointerup', drop, true);
    handle.addEventListener('dblclick', reset);
    handle.addEventListener('keydown', function (e) {
      var now = side.getBoundingClientRect().width;
      var step = e.shiftKey ? 64 : 16;
      switch (e.key) {
        case 'ArrowLeft':
          choose(now - step);
          break;
        case 'ArrowRight':
          choose(now + step);
          break;
        case 'Home':
          choose(SIDE_MIN);
          break;
        case 'End':
          choose(sideMax());
          break;
        case 'Enter':
          reset();
          break;
        default:
          return;
      }
      e.preventDefault();
    });
    window.addEventListener('resize', function () {
      if (want) show();
    });
    show();
  }

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

  // A slider is a number input with a range beside it, which only moves the number: the number is what is submitted. The
  // range shows the number while it moves, with the decimals of its step, which is how the server shows a value that was
  // loaded; and the number moves the range when it is typed in.
  document.addEventListener('input', function (e) {
    var box = e.target.closest && e.target.closest('[data-range]');
    if (!box) return;
    var range = box.querySelector('input[type="range"]');
    var number = box.querySelector('input[type="number"]');
    if (!range || !number) return;
    if (e.target === range) {
      var dot = range.step.indexOf('.');
      var value = parseFloat(range.value);
      number.value = isNaN(value) ? range.value : value.toFixed(dot < 0 ? 0 : range.step.length - dot - 1);
      number.setCustomValidity(''); // what it said of the value before it moved is not about this one
    } else if (e.target === number && number.value !== '' && !number.validity.badInput) {
      range.value = number.value;
    }
  });

  /* -------------------------------- palette ------------------------------- */

  // The page search. Its pages are the compiled app's own navigation, read from
  // the sidebar, so there is no index to fall out of date; pages that offer
  // Search add their own results, fetched as the user types. It is a combobox:
  // arrow keys move through the results, Enter follows one, Escape closes.
  var palette = { items: [], active: -1, moved: false, html: null, query: '', timer: null, ctl: null, state: 'idle' };

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

  // The results are HTML that the server made, of the pages that offer Search, in the order it ranked them: put in the box all at once, so
  // that nothing moves under the pointer as results come in one by one. The box has no results of its own: the search finds things, and does
  // not list the sidebar again. While a question is out the answer to the last one stays, dimmed and out of reach, so the box does not empty
  // and fill at every key.
  function drawPalette() {
    var box = paletteBox();
    var input = paletteInput();
    if (!box || !input) return;

    var previous = palette.items[palette.active];
    box.classList.toggle('pending', palette.state === 'loading');
    if (palette.html !== null) {
      box.innerHTML = palette.html;
    } else {
      // The words for a question that is out are the page's own, in a template that the server wrote.
      var pending = $('template[data-palette-pending]');
      box.innerHTML = pending ? pending.innerHTML : '';
    }
    palette.items = [];
    $$('a.pal-item', box).forEach(function (a, n) {
      a.id = 'palette-item-' + n;
      palette.items.push({ el: a, href: a.getAttribute('href') });
    });

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

  // Ask the pages that offer Search, after a pause in typing, for their results as HTML. A newer question replaces an older one, and a failure is
  // no results.
  function askRemote(query) {
    var input = paletteInput();
    var url = input && input.getAttribute('data-search-url');
    clearTimeout(palette.timer);
    if (palette.ctl) palette.ctl.abort();
    if (!query.trim()) {
      palette.html = null;
      palette.state = 'idle';
      return;
    }
    palette.state = 'loading';
    if (!url) return;
    palette.timer = setTimeout(async function () {
      var ctl = new AbortController();
      palette.ctl = ctl;
      try {
        var res = await fetch(url + '?q=' + encodeURIComponent(query), {
          headers: { [LEAF_HEADER]: 'search', Accept: 'text/html' },
          credentials: 'same-origin',
          signal: ctl.signal,
        });
        // The box's words are the server's, whatever the status: its list, or its words for a search that could not be made.
        var html = await res.text();
        if (query !== palette.query) return;
        palette.html = html;
        palette.state = 'done';
        drawPalette();
      } catch (err) {
        if (err.name === 'AbortError') return; // a newer search replaced it
        // Nothing came back at all, so there are no words of the server's to put in the box: say that, and not that nothing matches.
        var down = $('template[data-palette-down]');
        palette.html = down ? down.innerHTML : '';
        palette.state = 'done';
        drawPalette();
      }
    }, 150);
  }

  function openPalette() {
    var input = paletteInput();
    if (!input) return;
    palette.query = input.value;
    palette.moved = false;
    if (!palette.query.trim()) {
      askRemote(''); // a question that was waiting is not answered
      closePalette(); // nothing typed, nothing to find: the search does not list pages
      return;
    }
    askRemote(palette.query);
    drawPalette();
  }

  // The box is a combobox over a listbox only when script fills the listbox: without script it is a text input in a form with a button, and says
  // nothing of a popup that cannot open.
  (function () {
    var input = paletteInput();
    var box = paletteBox();
    if (!input || !box) return;
    input.setAttribute('role', 'combobox');
    input.setAttribute('aria-expanded', 'false');
    input.setAttribute('aria-controls', box.id);
    input.setAttribute('aria-autocomplete', 'list');
  })();

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
        // While a question is out the results on screen are the last one's: the form is submitted instead, to the page of results.
        if (open && palette.active >= 0 && palette.state !== 'loading') {
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

  // A toolbar has its left controls at its left edge and its right ones at its right when there is room for all of them in a
  // row, and when there is not it has them flow from the left in rows, as one group: the gap between the two groups is
  // gone. Whether there is room is the width of the controls as they are, whatever the rows they are in, so it does not
  // change with the layout that it chooses.
  function fitToolbar(tb) {
    var form = tb.querySelector(':scope > form') || tb; // the buttons' form, when there is one
    if (!tb.querySelector('.toolbar-gap')) return;
    var gap = parseFloat(getComputedStyle(tb).columnGap) || 0;
    var need = 0;
    var n = 0;
    for (var i = 0; i < form.children.length; i++) {
      var el = form.children[i];
      if (el.classList.contains('toolbar-gap') || el.type === 'hidden') continue;
      need += el.offsetWidth;
      n++;
    }
    tb.classList.toggle('wrapped', need + gap * n > tb.clientWidth);
  }
  function enhanceToolbars(root) {
    $$('[data-toolbar]:not([data-fitted])', root).forEach(function (tb) {
      tb.setAttribute('data-fitted', '');
      fitToolbar(tb);
      if (window.ResizeObserver) new ResizeObserver(function () { fitToolbar(tb); }).observe(tb);
    });
  }

  document.addEventListener('click', function (e) {
    if (!e.target.closest('.msel')) closeSelects(null);
  });

  // A panel that is fetched again brings its selects as plain ones.
  new MutationObserver(function () {
    enhanceSelects(document);
    enhanceToolbars(document);
  }).observe(document.body, { childList: true, subtree: true });
  enhanceToolbars(document);

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
  enhanceSidebarWidth();
  syncSelection(document);
  enhanceSelects(document);
  endArrival();
  $$('[data-tabs]').forEach(function (set) {
    showActiveTab(set);
  });
  startStatus();
  dismissToasts();
})();
