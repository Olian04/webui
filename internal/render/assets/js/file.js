// A file chooser, with script: a file can be dropped on it, a picture of an image that is chosen is shown, a file that is too large or
// not one of the kinds the field takes is said to be so before the request is sent, and a file that is chosen and not sent can be
// cleared. Without this script the field is the browser's own file chooser and the form is posted as it is, which is all that it needs.
//
// The input stays the control (a file upload panel is the exception, whose post upload.js makes with its own request, one file at a time). A drop puts the file in it, as if it had been chosen, so what is sent is the browser's own multipart
// post, from the input, and not a request that script makes. The server is told nothing of any of this and checks the file by what it
// holds, which script cannot see: what is said here is only what a name, a size and the type that the browser gives can say, in the
// words that the server uses.
//
// What the field takes is in data-file on the input, as JSON: {max, maxMessage, accept: [".png", "image/png", ...], kindMessage, image}.
// image says that a file that is chosen may be shown if it is a picture.
//
// A picture is small, and clicking it opens the file at its full size in a dialog. The picture of the stored file is the server's, with
// its own dialog, which a button opens with an invoker command and so needs none of this. A picture of a file that is chosen is made here,
// scaled down, with a dialog of the same look that is made when it is first wanted, and the file itself is read only when the dialog is
// opened, so that a large image is not held in the page to show a few dozen pixels of it.
(function () {
  'use strict';

  // A picture is shown only for the images a browser draws and that cannot run, and only up to this size: it is read into the page as a
  // data: address, which the content security policy allows for an image and a blob: address it does not, and a large one is a lot to hold
  // to show a thumbnail. SVG is not among them, since it is a document that can hold script.
  var IMAGE = /^image\/(png|jpeg|gif|webp)$/;
  var PREVIEW_MAX = 4 * 1024 * 1024;
  var THUMB = 192; // the longest side of the picture of a chosen file, twice what it is shown at, for a screen of a high density

  // The dialog that shows a file that is chosen at its full size: one for the page, made when it is first wanted. The file is read when
  // the dialog is opened and let go of when it is closed, so nothing is held for a picture that nobody opened. It is a native dialog,
  // which has the focus, Escape, the backdrop and the top layer; a click on the backdrop closes it where closedby is not known.
  var dialog = null;
  function openPreview(src, name) {
    if (!dialog) {
      dialog = element('dialog', 'preview-dialog');
      dialog.setAttribute('closedby', 'any');
      dialog.addEventListener('click', function (e) {
        if (e.target === dialog) dialog.close();
      });
      dialog.addEventListener('close', function () {
        var img = dialog.querySelector('img');
        if (img) img.removeAttribute('src');
      });
      var close = element('button', 'btn btn-ghost btn-sm preview-close', 'Close');
      close.type = 'button';
      close.addEventListener('click', function () {
        dialog.close();
      });
      dialog.appendChild(element('img', 'preview-full'));
      dialog.appendChild(element('div', 'preview-name'));
      dialog.appendChild(close);
      document.body.appendChild(dialog);
    }
    var full = dialog.querySelector('img');
    full.alt = name;
    full.src = src;
    dialog.setAttribute('aria-label', 'Preview of ' + name);
    dialog.querySelector('.preview-name').textContent = name;
    dialog.showModal();
  }

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

  // The file as a data: address, which the policy lets an image be and a blob: address it does not.
  function readAsData(file, done) {
    var reader = new FileReader();
    reader.onload = function () {
      if (typeof reader.result === 'string') done(reader.result);
    };
    reader.readAsDataURL(file);
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
      readAsData(file, function (data) {
        if (mine !== token) return;
        // Scaled down, in a canvas, to a picture of a few kilobytes: the page holds that, and not the image.
        var full = new Image();
        full.onload = function () {
          if (mine !== token) return;
          var ratio = Math.min(1, THUMB / Math.max(full.naturalWidth, full.naturalHeight));
          var canvas = document.createElement('canvas');
          canvas.width = Math.max(1, Math.round(full.naturalWidth * ratio));
          canvas.height = Math.max(1, Math.round(full.naturalHeight * ratio));
          canvas.getContext('2d').drawImage(full, 0, 0, canvas.width, canvas.height);
          var link = element('a', 'file-thumb-chosen');
          link.href = '#';
          link.title = 'Preview ' + file.name;
          var img = element('img', '');
          img.alt = '';
          img.src = canvas.toDataURL('image/png');
          link.appendChild(img);
          link.addEventListener('click', function (e) {
            e.preventDefault();
            e.stopPropagation();
            readAsData(file, function (again) {
              openPreview(again, file.name); // the file as it was chosen, read now
            });
          });
          shown.appendChild(link);
          shown.hidden = false;
        };
        full.src = data;
      });
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
