// A file upload panel, with script: the files that are chosen are sent one after another, each as a request of its own with a progress bar,
// and what the server answers to each, which is HTML, is what is shown for it. Without this script the panel is a form with a file chooser
// and a button, and the browser posts one file, which is all that it needs.
//
// The server answers script with HTML and nothing else. A post that carries X-Webui-Leaf (the header that a request for one panel's HTML
// carries) is answered with one list item: the tone and the text of the outcome, and a link when the outcome goes somewhere. It is put in
// the list as it is, and nothing is read from it but a link to follow when every file went well. What is sent is the form: its controls, in
// order, and then the one file, which the server reads while it arrives.
//
// What the form is told is in data-upload, as JSON: {multiple}. With multiple the chooser takes several files, a folder can be chosen with a
// button beside it and a folder can be dropped on the frame; each file of a folder is sent with its folder in its name ("photos/2024/a.jpg"),
// which the server reads as its path. file.js has made the chooser a drop zone by the time that this runs, and is left alone for a panel that
// takes one file.
(function () {
  'use strict';

  var LEAF_HEADER = 'X-Webui-Leaf';

  function element(tag, className, text) {
    var el = document.createElement(tag);
    if (className) el.className = className;
    if (text) el.textContent = text;
    return el;
  }

  function config(form) {
    try {
      return JSON.parse(form.getAttribute('data-upload'));
    } catch (err) {
      return null;
    }
  }

  // A file is sent with the name that says where it was: a folder's files carry the folder, which is what a browser sends for a folder that
  // was chosen in a form, and a file that was chosen alone carries its own name.
  function fromFiles(files) {
    var out = [];
    for (var i = 0; files && i < files.length; i++) {
      out.push({ file: files[i], path: files[i].webkitRelativePath || '' });
    }
    return out;
  }

  // A folder that is dropped is a directory entry, which is read in batches until it is empty, and each file of it is found with its path.
  function walk(entry, prefix, out, done) {
    if (entry.isFile) {
      entry.file(
        function (file) {
          out.push({ file: file, path: prefix + file.name });
          done();
        },
        function () {
          done();
        }
      );
      return;
    }
    if (!entry.isDirectory) {
      done();
      return;
    }
    var reader = entry.createReader();
    var children = [];
    (function more() {
      reader.readEntries(
        function (batch) {
          if (batch.length) {
            for (var i = 0; i < batch.length; i++) children.push(batch[i]);
            more();
            return;
          }
          var left = children.length;
          if (!left) {
            done();
            return;
          }
          children.forEach(function (child) {
            walk(child, prefix + entry.name + '/', out, function () {
              if (--left === 0) done();
            });
          });
        },
        function () {
          done();
        }
      );
    })();
  }

  // The controls of the form, as the browser would send them, without the file: the fields come first, in order, which is what lets the server check
  // them before it reads the file.
  function controls(form, data) {
    for (var i = 0; i < form.elements.length; i++) {
      var el = form.elements[i];
      if (!el.name || el.disabled || el.type === 'file' || el.type === 'submit' || el.type === 'button') continue;
      if ((el.type === 'checkbox' || el.type === 'radio') && !el.checked) continue;
      if (el.tagName === 'SELECT') {
        for (var j = 0; j < el.options.length; j++) {
          if (el.options[j].selected) data.append(el.name, el.options[j].value);
        }
      } else {
        data.append(el.name, el.value);
      }
    }
  }

  function enhance(form) {
    if (form.hasAttribute('data-upload-ready')) return;
    var cfg = config(form);
    var input = form.querySelector('input[type="file"][name="file"]');
    var leafInput = form.querySelector('input[name="_leaf"]');
    if (!cfg || !input || input.disabled || !leafInput) return; // a panel that the Guard disabled has a chooser that is disabled, which there is nothing to add to
    form.setAttribute('data-upload-ready', '');

    var leaf = leafInput.value;
    var panel = form.closest('[data-leaf]') || form;
    var submit = form.querySelector('.panel-foot button[type="submit"], .panel-foot button:not([type])');
    var wasRequired = input.required;
    var zone = form.querySelector('.file-drop');
    var row = input.parentNode;

    var list = element('ul', 'upload-queue');
    list.setAttribute('aria-live', 'polite');
    list.hidden = true;
    (form.querySelector('.panel-body') || form).appendChild(list);

    var cancel = element('button', 'btn btn-ghost btn-sm', 'Cancel');
    cancel.type = 'button';
    cancel.hidden = true;
    if (submit && submit.parentNode) submit.parentNode.insertBefore(cancel, submit);

    var items = []; // what is to be sent: {file, path}
    var running = null; // the request that is in flight
    var stopped = false;
    var clearing = false; // the chooser is being emptied by this script, which is not a choice of files

    // The chooser is satisfied by the files that script holds, which are not always in it: those of a folder, or of a drop.
    function setItems(next) {
      items = next;
      input.required = wasRequired && !items.length;
      var rows = [];
      list.textContent = '';
      items.forEach(function (item) {
        var li = element('li', 'upload-item');
        li.setAttribute('data-state', 'ready');
        li.appendChild(element('span', 'upload-name', item.path || item.file.name));
        rows.push(li);
        list.appendChild(li);
      });
      list.hidden = !items.length || !cfg.multiple;
      list._rows = rows;
    }

    if (cfg.multiple) {
      input.multiple = true;
      var picker = element('input');
      picker.type = 'file';
      picker.hidden = true;
      picker.setAttribute('webkitdirectory', '');
      var folder = element('button', 'btn btn-ghost btn-sm file-folder', 'Choose folder');
      folder.type = 'button';
      folder.addEventListener('click', function () {
        picker.click();
      });
      picker.addEventListener('change', function () {
        input.value = '';
        setItems(fromFiles(picker.files));
      });
      row.appendChild(folder);
      row.appendChild(picker);

      // A drop of several files, or of a folder, is taken here, before the chooser's own handler, which takes one file.
      form.addEventListener(
        'drop',
        function (e) {
          if (!zone || !zone.contains(e.target) || !e.dataTransfer || Array.prototype.indexOf.call(e.dataTransfer.types || [], 'Files') < 0) return;
          e.preventDefault();
          e.stopPropagation();
          zone.classList.remove('dragging');
          var entries = [];
          var plain = [];
          var dropped = e.dataTransfer.items || [];
          for (var i = 0; i < dropped.length; i++) {
            if (dropped[i].kind !== 'file') continue;
            var entry = dropped[i].webkitGetAsEntry ? dropped[i].webkitGetAsEntry() : null;
            if (entry) entries.push(entry);
            else plain.push({ file: dropped[i].getAsFile(), path: '' });
          }
          if (!entries.length && !plain.length) plain = fromFiles(e.dataTransfer.files);
          var found = plain.slice();
          var left = entries.length;
          input.value = '';
          if (!left) {
            setItems(found);
            return;
          }
          entries.forEach(function (entry) {
            walk(entry, '', found, function () {
              if (--left === 0) setItems(found);
            });
          });
        },
        true
      );
    }

    input.addEventListener('change', function () {
      if (!clearing) setItems(fromFiles(input.files));
    });

    function busy(on) {
      if (on) panel.setAttribute('aria-busy', 'true');
      else panel.removeAttribute('aria-busy');
      if (submit) submit.disabled = on;
      input.disabled = on;
      cancel.hidden = !on;
    }

    // The outcome of a file: the server's list item, which is put in the file's row as it is, with its tone.
    function show(li, xhr) {
      var progress = li.querySelector('progress');
      if (progress) progress.remove();
      var result = null;
      if (xhr && xhr.responseText) {
        var doc = new DOMParser().parseFromString(xhr.responseText, 'text/html');
        result = doc.querySelector('li.upload-result');
      }
      if (result) {
        li.setAttribute('data-tone', result.getAttribute('data-tone') || '');
        for (var node = result.firstChild; node; node = node.nextSibling) li.appendChild(document.importNode(node, true));
      } else {
        li.setAttribute('data-tone', 'bad');
        li.appendChild(element('span', 'upload-text', 'The upload could not be completed.'));
      }
      li.setAttribute('data-state', 'done');
    }

    function finish(rows) {
      busy(false);
      var all = rows.length > 0;
      var next = null;
      rows.forEach(function (li) {
        if (li.getAttribute('data-tone') !== 'ok' && li.getAttribute('data-tone') !== 'warn') all = false;
        next = li.querySelector('a.upload-next') || next;
      });
      items = [];
      clearing = true;
      input.value = '';
      input.dispatchEvent(new Event('change', { bubbles: true })); // so that file.js, which shows what is chosen, shows that nothing is
      clearing = false;
      input.required = wasRequired;
      // The panels around this one may show what was just sent: they are asked for again. An outcome that went somewhere is followed, as it would
      // be by a browser that was redirected, when every file went well.
      document.dispatchEvent(new CustomEvent('webui:changed', { detail: { leaf: leaf } }));
      if (all && next) window.location.assign(next.href);
    }

    function send(rows, index) {
      if (stopped || index >= items.length) {
        for (var skipped = index; skipped < rows.length; skipped++) {
          if (rows[skipped].getAttribute('data-state') !== 'done') {
            rows[skipped].setAttribute('data-state', 'done');
            rows[skipped].setAttribute('data-tone', 'warn');
            rows[skipped].appendChild(element('span', 'upload-text', 'Not sent.'));
          }
        }
        finish(rows);
        return;
      }
      var item = items[index];
      var li = rows[index];
      var progress = element('progress');
      progress.max = 1;
      progress.value = 0;
      progress.setAttribute('aria-label', 'Sending ' + (item.path || item.file.name));
      li.appendChild(progress);
      li.setAttribute('data-state', 'sending');

      var data = new FormData();
      controls(form, data);
      data.append('file', item.file, item.path || item.file.name);

      var xhr = new XMLHttpRequest();
      running = xhr;
      xhr.open('POST', form.getAttribute('action') || window.location.href);
      xhr.setRequestHeader(LEAF_HEADER, leaf);
      xhr.upload.onprogress = function (e) {
        if (e.lengthComputable) {
          progress.max = e.total;
          progress.value = e.loaded;
        }
      };
      function done() {
        running = null;
        show(li, xhr);
        send(rows, index + 1);
      }
      xhr.onload = done;
      xhr.onerror = function () {
        xhr = null;
        running = null;
        show(li, null);
        send(rows, index + 1);
      };
      xhr.onabort = function () {
        running = null;
        li.setAttribute('data-state', 'done');
        li.setAttribute('data-tone', 'warn');
        if (progress.parentNode) progress.remove();
        li.appendChild(element('span', 'upload-text', 'Cancelled.'));
        send(rows, index + 1);
      };
      xhr.send(data);
    }

    cancel.addEventListener('click', function () {
      stopped = true;
      if (running) running.abort();
    });

    form.addEventListener('submit', function (e) {
      if (!items.length) setItems(fromFiles(input.files));
      if (!items.length) return; // nothing is chosen, which the browser says for itself
      e.preventDefault();
      if (running) return;
      stopped = false;
      list.hidden = false;
      var rows = list._rows;
      rows.forEach(function (li) {
        li.removeAttribute('data-tone');
        li.setAttribute('data-state', 'ready');
        while (li.childNodes.length > 1) li.removeChild(li.lastChild);
      });
      busy(true);
      send(rows, 0);
    });
  }

  function enhanceAll(root) {
    var scope = root || document;
    var forms = scope.querySelectorAll ? scope.querySelectorAll('form[data-upload]') : [];
    for (var i = 0; i < forms.length; i++) enhance(forms[i]);
  }

  enhanceAll(document);
  // A panel that is replaced by script brings its form with it: it is set up when it arrives.
  new MutationObserver(function (changes) {
    changes.forEach(function (c) {
      c.addedNodes.forEach(function (n) {
        if (n.nodeType !== 1) return;
        if (n.matches && n.matches('form[data-upload]')) enhance(n);
        else enhanceAll(n);
      });
    });
  }).observe(document.documentElement, { childList: true, subtree: true });
})();
