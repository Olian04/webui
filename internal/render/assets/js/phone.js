// A phone number, edited with script: a selector of the country calling code in front of the national number, which is spaced for
// reading as it is typed. What is sent is E.164, "+46701234567", in a hidden input that has the name of the field. Without this
// script the field is the one input it is, holding the number as it is typed, and the server reads that the same way.
//
// The countries are the server's, set as self.webuiPhoneCountries by the first line of this file: [{i: "SE", n: "Sweden", c: "46",
// g: [2, 3, 2, 2], t: 1, m: 1}], where g is how many digits are in each group of the national number, t says the national format
// has a leading 0 that the international one drops, and m is the country that a code that several share is taken to be.
(function () {
  'use strict';

  var countries = self.webuiPhoneCountries || [];
  var byISO = {};
  var byCode = {};
  countries.forEach(function (c) {
    byISO[c.i] = c;
    if (!byCode[c.c] || c.m) byCode[c.c] = c;
  });

  var MAX_DIGITS = 15; // E.164

  function digitsOf(s) {
    return s.replace(/\D/g, '');
  }

  // The country a number is of, by its code: the longest one that its digits start with, which is what makes "+358" Finland
  // and not "+3" and a code that is not one. A code that several countries share is the one that is the main of them.
  function countryOf(digits) {
    for (var n = 3; n >= 1; n--) {
      var c = byCode[digits.slice(0, n)];
      if (c) return c;
    }
    return null;
  }

  // The digits of a national number in groups, spaced: as the country says, with the rest in one, or in threes for a
  // country that does not say. A group of one is joined to the one before it, so that nothing ends in a lone digit.
  function spaced(country, digits) {
    var sizes = (country && country.g) || null;
    var out = [];
    var at = 0;
    if (sizes) {
      for (var i = 0; i < sizes.length && at < digits.length; i++) {
        out.push(digits.slice(at, at + sizes[i]));
        at += sizes[i];
      }
      if (at < digits.length) out.push(digits.slice(at));
    } else {
      while (at < digits.length) {
        out.push(digits.slice(at, at + 3));
        at += 3;
      }
      if (out.length > 1 && out[out.length - 1].length === 1) {
        out[out.length - 2] += out.pop();
      }
    }
    return out.join(' ');
  }

  // The country of the browser's language, when the list has it: "sv-SE" is Sweden.
  function browserCountry() {
    var langs = navigator.languages && navigator.languages.length ? navigator.languages : [navigator.language || ''];
    for (var i = 0; i < langs.length; i++) {
      var m = /[-_]([A-Za-z]{2})\b/.exec(langs[i] || '');
      if (m && byISO[m[1].toUpperCase()]) return byISO[m[1].toUpperCase()];
    }
    return null;
  }

  function enhance(input) {
    if (input.hasAttribute('data-phone-done') || !countries.length) return;
    input.setAttribute('data-phone-done', '');
    var held = input.value.trim();
    // A number that is there and is not international, or has an extension, is not one that can be split without changing
    // it, and is left as the input it is.
    if (held !== '' && !/^\+[\d\s().\-\/]*$/.test(held)) return;

    var country = null;
    var national = '';
    if (held !== '') {
      var all = digitsOf(held);
      country = countryOf(all);
      national = country ? all.slice(country.c.length) : all;
    } else {
      country = byISO[input.getAttribute('data-country') || ''] || browserCountry();
    }

    var name = input.getAttribute('name');
    var wrap = document.createElement('div');
    wrap.className = 'phone';
    var select = document.createElement('select');
    select.className = 'input phone-cc';
    select.setAttribute('aria-label', 'Country calling code');
    var none = document.createElement('option');
    none.value = '';
    none.textContent = '—';
    select.appendChild(none);
    countries
      .slice()
      .sort(function (a, b) {
        return a.n.localeCompare(b.n);
      })
      .forEach(function (c) {
        var o = document.createElement('option');
        o.value = c.i;
        o.textContent = '+' + c.c + ' ' + c.n;
        select.appendChild(o);
      });
    var hidden = document.createElement('input');
    hidden.type = 'hidden';
    if (name) hidden.name = name;
    if (input.form !== null && input.getAttribute('form')) hidden.setAttribute('form', input.getAttribute('form'));

    // What the browser would check of the number as it is typed is about the national part now, and is the server's of the
    // number that is sent.
    ['name', 'pattern', 'minlength', 'maxlength', 'title', 'form'].forEach(function (a) {
      input.removeAttribute(a);
    });
    input.setAttribute('inputmode', 'tel');
    input.setAttribute('autocomplete', 'tel-national');
    input.parentNode.insertBefore(wrap, input);
    wrap.appendChild(select);
    wrap.appendChild(input);
    wrap.appendChild(hidden);

    function limit() {
      return MAX_DIGITS - (country ? country.c.length : 0);
    }

    function commit(notify) {
      var digits = digitsOf(input.value);
      hidden.value = digits === '' ? '' : country ? '+' + country.c + digits : input.value.trim();
      if (notify) hidden.dispatchEvent(new Event('change', { bubbles: true }));
    }

    function show(digits, caretDigits) {
      if (country && country.t && digits.charAt(0) === '0') {
        digits = digits.slice(1); // the trunk 0 is only in the national format
        if (caretDigits > 0) caretDigits--;
      }
      digits = digits.slice(0, limit());
      var text = country ? spaced(country, digits) : digits;
      input.value = text;
      if (caretDigits !== null && document.activeElement === input) {
        var seen = 0;
        var at = 0;
        while (at < text.length && seen < caretDigits) {
          if (/\d/.test(text.charAt(at))) seen++;
          at++;
        }
        input.setSelectionRange(at, at);
      }
      input.setCustomValidity('');
      commit(false);
    }

    function setCountry(c) {
      country = c;
      select.value = c ? c.i : '';
      input.placeholder = c ? placeholder(c) : '';
    }

    // An example of how a number of the country is spaced, for the empty input to show, unless the field has a placeholder.
    var own = input.getAttribute('placeholder');
    function placeholder(c) {
      return own || spaced(c, '1234567890123'.slice(0, c.g ? c.g.reduce(function (a, b) { return a + b; }, 0) : 9));
    }

    setCountry(country);
    show(national, null);

    input.addEventListener('input', function () {
      var before = input.value.slice(0, input.selectionStart || 0);
      // A whole number pasted with its code, "+46 70 123 45 67", is read as one: the code picks the country.
      if (/^\s*\+/.test(input.value)) {
        var all = digitsOf(input.value);
        var found = countryOf(all);
        if (found) {
          setCountry(found);
          show(all.slice(found.c.length), null);
          commit(true);
          return;
        }
      }
      show(digitsOf(input.value), digitsOf(before).length);
      commit(true);
    });
    input.addEventListener('change', function () {
      commit(true);
    });
    select.addEventListener('change', function () {
      setCountry(byISO[select.value] || null);
      show(digitsOf(input.value), null);
      commit(true);
      input.focus();
    });
  }

  function enhanceAll(root) {
    var list = (root || document).querySelectorAll ? (root || document).querySelectorAll('input[data-phone]') : [];
    for (var i = 0; i < list.length; i++) enhance(list[i]);
  }

  enhanceAll(document);
  // A panel that is replaced by script brings its fields with it: they are split when they arrive.
  new MutationObserver(function (changes) {
    changes.forEach(function (c) {
      c.addedNodes.forEach(function (n) {
        if (n.nodeType === 1) {
          if (n.matches && n.matches('input[data-phone]')) enhance(n);
          else enhanceAll(n);
        }
      });
    });
  }).observe(document.documentElement, { childList: true, subtree: true });
})();
