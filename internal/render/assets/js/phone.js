// A phone number's national part, spaced as it is typed. The selector of the country calling code and the input are the server's, in the
// page: a country that is chosen says how its numbers are grouped, as data-groups="2,3,2,2" (how many digits are in each group, the rest
// being one), data-code, its calling code, and data-trunk, that the national format has a leading 0 that the international one drops. What is sent is what the
// input holds and the country that is chosen: the server makes the one number of them. Without this script the number is typed as it is
// and read the same way.
(function () {
  'use strict';

  var MAX_DIGITS = 15; // E.164, the country's code included

  function digitsOf(s) {
    return s.replace(/\D/g, '');
  }

  // The digits in groups, spaced: as the country says, with the rest in one, or in threes for a country that does not say. A group of
  // one is joined to the one before it, so that nothing ends in a lone digit.
  function spaced(groups, digits) {
    var out = [];
    var at = 0;
    if (groups.length) {
      for (var i = 0; i < groups.length && at < digits.length; i++) {
        out.push(digits.slice(at, at + groups[i]));
        at += groups[i];
      }
      if (at < digits.length) out.push(digits.slice(at));
    } else {
      while (at < digits.length) {
        out.push(digits.slice(at, at + 3));
        at += 3;
      }
      if (out.length > 1 && out[out.length - 1].length === 1) out[out.length - 2] += out.pop();
    }
    return out.join(' ');
  }

  function enhance(wrap) {
    if (wrap.hasAttribute('data-phone-done')) return;
    wrap.setAttribute('data-phone-done', '');
    var select = wrap.querySelector('select');
    var input = wrap.querySelector('input');
    if (!select || !input) return;

    function country() {
      var o = select.options[select.selectedIndex];
      if (!o || !o.value) return null;
      var g = o.getAttribute('data-groups');
      return {
        groups: g ? g.split(',').map(Number) : [],
        trunk: o.hasAttribute('data-trunk'),
        code: (o.getAttribute('data-code') || '').length,
      };
    }

    // The input is left as it is typed when a whole number is typed in it ("+46 …": the server reads the code) or no country is chosen.
    function show() {
      var c = country();
      if (!c || /^\s*\+/.test(input.value)) return;
      var before = digitsOf(input.value.slice(0, input.selectionStart || 0)).length;
      var digits = digitsOf(input.value);
      if (c.trunk && digits.charAt(0) === '0') {
        digits = digits.slice(1); // the trunk 0 is only in the national format
        if (before > 0) before--;
      }
      digits = digits.slice(0, MAX_DIGITS - c.code);
      var text = spaced(c.groups, digits);
      input.value = text;
      if (document.activeElement === input) {
        var seen = 0;
        var at = 0;
        while (at < text.length && seen < before) {
          if (/\d/.test(text.charAt(at))) seen++;
          at++;
        }
        input.setSelectionRange(at, at);
      }
      input.setCustomValidity('');
    }

    input.addEventListener('input', show);
    // The focus stays on the selector: a change fires for each arrow key on a closed one, and moving the focus then leaves a keyboard
    // user one country to choose.
    select.addEventListener('change', show);
  }

  function enhanceAll(root) {
    var list = root.querySelectorAll ? root.querySelectorAll('[data-phone]') : [];
    for (var i = 0; i < list.length; i++) enhance(list[i]);
  }

  enhanceAll(document);
  // A panel that is replaced by script brings its fields with it: they are spaced when they arrive.
  new MutationObserver(function (changes) {
    changes.forEach(function (c) {
      c.addedNodes.forEach(function (n) {
        if (n.nodeType !== 1) return;
        if (n.matches && n.matches('[data-phone]')) enhance(n);
        else enhanceAll(n);
      });
    });
  }).observe(document.documentElement, { childList: true, subtree: true });
})();
