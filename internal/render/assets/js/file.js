// A file chooser, with script: a file can be dropped on it, a picture of an image that is chosen is shown, a file that is too large or
// not one of the kinds the field takes is said to be so before the request is sent, and a file that is chosen and not sent can be
// cleared. Without this script the field is the browser's own file chooser and the form is posted as it is, which is all that it needs.
//
// The input stays the control. A drop puts the file in it, as if it had been chosen, so what is sent is the browser's own multipart
// post, from the input, and not a request that script makes. The server is told nothing of any of this and checks the file by what it
// holds, which script cannot see: what is said here is only what a name, a size and the type that the browser gives can say, in the
// words that the server uses.
//
// What the field takes is in data-file on the input, as JSON: {max, maxMessage, accept: [".png", "image/png", ...], kindMessage, image,
// preview}. image says that a file that is chosen may be shown if it is a picture, and preview is the address on this site of the picture
// of the file that is stored, which is there only for a field that takes nothing but images.
(function () {
  'use strict';

  // A picture is shown only for the images a browser draws and that cannot run, and only up to this size: it is read into the page as a
  // data: address, which the content security policy allows for an image and a blob: address it does not, and a large one is a lot to hold
  // to show a thumbnail. SVG is not among them, since it is a document that can hold script.
  var IMAGE = /^image\/(png|jpeg|gif|webp)$/;
  var PREVIEW_MAX = 4 * 1024 * 1024;

  function config(input) {
    try {
      return JSON.parse(input.getAttribute('data-file'));
    } catch (err) {
      return null;
    }
  }

  // What is said of a file before it is sent, "" for nothing. The size is exact. The kind is only what the name and the type that the
  // browser gives say, and that is a hint: a file whose type the browser does not know is not said to be wrong, since the server would
  // take it, and what is refused here only after both the type and the extension say it is not one of those the field takes.
  function problem(cfg, file) {
    if (cfg.max && file.size > cfg.max) return cfg.maxMessage;
    if (cfg.accept && cfg.accept.length && file.type) {
      var name = file.name.toLowerCase();
      var type = file.type.toLowerCase();
      var ok = cfg.accept.some(function (token) {
        return token.charAt(0) === '.' ? name.slice(-token.length) === token : token === type;
      });
      if (!ok) return cfg.kindMessage || '';
    }
    return '';
  }

  function element(tag, className, text) {
    var el = document.createElement(tag);
    el.className = className;
    if (text) el.textContent = text;
    return el;
  }

  function enhance(input) {
    if (input.hasAttribute('data-file-ready')) return;
    var cfg = config(input);
    if (!cfg) return;
    input.setAttribute('data-file-ready', '');

    // The drop zone is a frame around the input, with what to do beside it and the picture below it. The input is moved into it, and is
    // still a form's control: nothing about it changes but where it is.
    var zone = element('div', 'file-drop');
    var row = element('div', 'file-row');
    var clear = element('button', 'btn btn-ghost btn-sm file-clear', 'Clear');
    clear.type = 'button';
    clear.hidden = true;
    var hint = element('span', 'file-hint', 'or drop a file here');
    var shown = element('div', 'file-preview');
    shown.hidden = true;
    input.parentNode.insertBefore(zone, input);
    row.appendChild(input);
    row.appendChild(clear);
    zone.appendChild(row);
    zone.appendChild(hint);
    zone.appendChild(shown);

    // The picture of the file that is stored, beside its name, when the field takes only images and the app gave an address of this site.
    var current = input.closest('.file') && input.closest('.file').querySelector('.file-current');
    if (current && cfg.preview) {
      var thumb = element('img', 'file-thumb');
      thumb.alt = '';
      thumb.src = cfg.preview;
      thumb.addEventListener('error', function () {
        thumb.remove(); // not there, or not a picture: the name is what there is
      });
      current.insertBefore(thumb, current.firstChild);
    }

    var token = 0; // which file the picture being read is for, so that a slow one is not shown for a file that has been replaced
    function sync() {
      var file = input.files && input.files[0];
      token++;
      shown.hidden = true;
      shown.textContent = '';
      clear.hidden = !file;
      input.classList.remove('invalid');
      if (!file) {
        input.setCustomValidity('');
        return;
      }
      var said = problem(cfg, file);
      input.setCustomValidity(said);
      if (said) {
        input.classList.add('invalid');
        input.reportValidity();
        return;
      }
      if (!cfg.image || !IMAGE.test(file.type) || file.size > PREVIEW_MAX) return;
      var mine = token;
      var reader = new FileReader();
      reader.onload = function () {
        if (mine !== token || typeof reader.result !== 'string') return;
        var img = element('img', '');
        img.alt = 'Preview of ' + file.name;
        img.src = reader.result;
        shown.appendChild(img);
        shown.hidden = false;
      };
      reader.readAsDataURL(file);
    }

    input.addEventListener('change', sync);
    clear.addEventListener('click', function () {
      input.value = '';
      sync();
      input.focus();
    });
    if (input.form) {
      input.form.addEventListener('reset', function () {
        setTimeout(sync); // after the form has emptied the input
      });
    }

    // A drop is a choice. A file is what is being dragged only when the types say so: text dragged from a page is left to the browser.
    function carriesFiles(e) {
      return e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types || [], 'Files') >= 0;
    }
    zone.addEventListener('dragenter', function (e) {
      if (!carriesFiles(e)) return;
      e.preventDefault();
      zone.classList.add('dragging');
    });
    zone.addEventListener('dragover', function (e) {
      if (!carriesFiles(e)) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = 'copy';
      zone.classList.add('dragging');
    });
    zone.addEventListener('dragleave', function (e) {
      if (!zone.contains(e.relatedTarget)) zone.classList.remove('dragging');
    });
    zone.addEventListener('drop', function (e) {
      if (!carriesFiles(e)) return;
      e.preventDefault();
      zone.classList.remove('dragging');
      var files = e.dataTransfer.files;
      if (!files || !files.length) return;
      // One file to a field, whatever was dropped: the first.
      try {
        var one = new DataTransfer();
        one.items.add(files[0]);
        input.files = one.files;
      } catch (err) {
        return; // a browser that cannot be given a file leaves the chooser to be used
      }
      input.dispatchEvent(new Event('change', { bubbles: true })); // as if it had been chosen: the input says the rest
    });

    sync();
  }

  function enhanceAll(root) {
    var list = (root || document).querySelectorAll ? (root || document).querySelectorAll('input[type="file"][data-file]') : [];
    for (var i = 0; i < list.length; i++) enhance(list[i]);
  }

  enhanceAll(document);
  // A panel that is replaced by script brings its fields with it: they are set up when they arrive.
  new MutationObserver(function (changes) {
    changes.forEach(function (c) {
      c.addedNodes.forEach(function (n) {
        if (n.nodeType === 1) {
          if (n.matches && n.matches('input[type="file"][data-file]')) enhance(n);
          else enhanceAll(n);
        }
      });
    });
  }).observe(document.documentElement, { childList: true, subtree: true });
})();
