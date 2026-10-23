// The glue between webui's server-rendered editors and diffs and Monaco. It is built
// with Monaco into one bundle (see ../build.mjs), so there is one copy of Monaco on the
// page, and it is loaded only by pages that have an editor or a diff.
//
// Everything it does is an upgrade. The server renders a textarea (or a pre, for a
// viewer, or a unified diff) that works without script; this replaces it with Monaco and
// keeps the textarea in step, so the form posts as it always did.
import * as monaco from 'monaco-editor/editor/editor.main.js';

const configElement = document.getElementById('webui-code');
const config = configElement ? JSON.parse(configElement.textContent || '{}') : {};
const workers = config.workers || {};

// A language service is a worker the page was handed only when an editable editor of
// that language is on it, and the application imported its package. Which service a
// language belongs to is Monaco's.
const SERVICE = {
  json: 'json',
  css: 'css',
  scss: 'css',
  less: 'css',
  html: 'html',
  handlebars: 'html',
  razor: 'html',
  typescript: 'ts',
  javascript: 'ts',
};

self.MonacoEnvironment = {
  getWorker(_id, label) {
    const url = workers[SERVICE[label]] || workers.editor;
    return new Worker(url, { name: label });
  },
};

// Without a service there is only highlighting. Monaco registers the features of all
// four languages on load, and each would ask for its worker, so they are switched off
// here unless the page was given the worker.
const off = (keys, keep = {}) => Object.fromEntries(keys.map((k) => [k, !!keep[k]]));
const CSS_KEYS = ['completionItems', 'hovers', 'documentSymbols', 'definitions', 'references', 'documentHighlights', 'rename', 'colors', 'foldingRanges', 'diagnostics', 'selectionRanges', 'documentFormattingEdits', 'documentRangeFormattingEdits'];
const HTML_KEYS = ['completionItems', 'hovers', 'documentSymbols', 'links', 'documentHighlights', 'rename', 'colors', 'foldingRanges', 'selectionRanges', 'diagnostics', 'documentFormattingEdits', 'documentRangeFormattingEdits'];
const JSON_KEYS = ['documentFormattingEdits', 'documentRangeFormattingEdits', 'completionItems', 'hovers', 'documentSymbols', 'tokens', 'colors', 'foldingRanges', 'diagnostics', 'selectionRanges'];
const TS_KEYS = ['completionItems', 'hovers', 'documentSymbols', 'definitions', 'references', 'documentHighlights', 'rename', 'diagnostics', 'documentRangeFormattingEdits', 'signatureHelp', 'onTypeFormattingEdits', 'codeActions', 'inlayHints'];

if (!workers.json) {
  monaco.json.jsonDefaults.setModeConfiguration(off(JSON_KEYS, { tokens: true }));
  monaco.json.jsonDefaults.setDiagnosticsOptions({ validate: false });
}
if (!workers.css) {
  for (const d of ['cssDefaults', 'scssDefaults', 'lessDefaults']) {
    monaco.css[d].setModeConfiguration(off(CSS_KEYS));
    monaco.css[d].setOptions({ validate: false });
  }
}
if (!workers.html) {
  for (const d of ['htmlDefaults', 'handlebarDefaults', 'razorDefaults']) {
    monaco.html[d].setModeConfiguration(off(HTML_KEYS));
  }
}
if (!workers.ts) {
  for (const d of ['typescriptDefaults', 'javascriptDefaults']) {
    monaco.typescript[d].setModeConfiguration(off(TS_KEYS));
    monaco.typescript[d].setDiagnosticsOptions({ noSemanticValidation: true, noSyntaxValidation: true, noSuggestionDiagnostics: true });
  }
}

// ----------------------------------------------------------------------- theme

// Monaco wants colours as hex, and the application's are CSS values that may be
// computed (color-mix), so each is resolved through the browser and a canvas.
const canvas = document.createElement('canvas').getContext('2d', { willReadFrequently: true });
function colour(variable, fallback) {
  const probe = document.createElement('span');
  probe.style.color = `var(${variable})`;
  document.body.appendChild(probe);
  const resolved = getComputedStyle(probe).color || fallback;
  probe.remove();
  canvas.fillStyle = '#000';
  canvas.fillStyle = resolved;
  const value = canvas.fillStyle;
  if (value.startsWith('#')) return value;
  const m = /rgba?\(([^)]+)\)/.exec(value);
  if (!m) return fallback;
  const [r, g, b, a = 1] = m[1].split(',').map((n) => parseFloat(n));
  const hex = (n) => Math.round(n).toString(16).padStart(2, '0');
  return `#${hex(r)}${hex(g)}${hex(b)}${a < 1 ? hex(a * 255) : ''}`;
}

function isLight() {
  const chosen = document.documentElement.getAttribute('data-theme');
  return chosen ? chosen === 'light' : matchMedia('(prefers-color-scheme: light)').matches;
}

// A reader who asked the system for its own colours, or for more contrast, is given Monaco's high contrast themes
// as the base, which draw the editor's borders and its selection and its find matches with outlines and not by
// colour alone. The application's colours are laid over them as over the others, and in forced colours they resolve
// to the system's.
const highContrast = matchMedia('(forced-colors: active), (prefers-contrast: more)');
function themeBase() {
  if (highContrast.matches) return isLight() ? 'hc-light' : 'hc-black';
  return isLight() ? 'vs' : 'vs-dark';
}

function defineThemes() {
  const colors = {
    'editor.background': colour('--panel', '#181b1f'),
    'editor.foreground': colour('--text', '#ccccdc'),
    'editorLineNumber.foreground': colour('--text-3', '#6e6e7e'),
    'editorLineNumber.activeForeground': colour('--text-2', '#9090a0'),
    'editor.lineHighlightBackground': colour('--hover', '#22252b'),
    'editor.selectionBackground': colour('--ring', '#3d71d955'),
    'editorCursor.foreground': colour('--blue-text', '#6e9fff'),
    'editorWidget.background': colour('--raise', '#22252b'),
    'editorWidget.border': colour('--border-2', '#3a3d44'),
    'editorSuggestWidget.background': colour('--raise', '#22252b'),
    'editorSuggestWidget.border': colour('--border-2', '#3a3d44'),
    'editorHoverWidget.background': colour('--raise', '#22252b'),
    'editorHoverWidget.border': colour('--border-2', '#3a3d44'),
    'scrollbarSlider.background': colour('--border-2', '#3a3d4488'),
    'editorGutter.background': colour('--panel', '#181b1f'),
    'diffEditor.insertedTextBackground': '#2ea04333',
    'diffEditor.removedTextBackground': '#f8514933',
  };
  monaco.editor.defineTheme('webui', { base: themeBase(), inherit: true, rules: [], colors });
}

function applyTheme() {
  defineThemes();
  monaco.editor.setTheme('webui');
}
new MutationObserver(applyTheme).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
matchMedia('(prefers-color-scheme: light)').addEventListener('change', applyTheme);
highContrast.addEventListener('change', applyTheme);


// --------------------------------------------------------------------- context

// What a language service is given besides the text: declaration files and compiler
// options for TypeScript, schemas for JSON. The files are addresses the page was handed
// only because the visitor may edit the editor, each carrying the key that unlocks it, and
// the browser keeps them. Whatever fails to load leaves the editor as it would be without:
// the form still works.
const context = config.context || {};

async function fetchFiles(files) {
  return Promise.all(
    files.map(async (f) => {
      const response = await fetch(f.url);
      if (!response.ok) throw new Error(`${f.path}: ${response.status}`);
      return { path: f.path, text: await response.text() };
    }),
  );
}

// A page's TypeScript service is one program, so its files and options are the page's, set
// once for each language however many editors there are.
const programs = {};
function program(language) {
  const libs = (context.libs || {})[language];
  const options = (context.compiler || {})[language];
  if (!workers.ts || (!libs && !options)) return;
  programs[language] ||= (async () => {
    const defaults = monaco.typescript[language === 'javascript' ? 'javascriptDefaults' : 'typescriptDefaults'];
    // moduleResolution is Node's unless the options say otherwise, which is what finds a
    // library at node_modules/@types/foo.
    defaults.setCompilerOptions({ ...defaults.getCompilerOptions(), moduleResolution: 2, ...options });
    try {
      for (const f of await fetchFiles(libs || [])) defaults.addExtraLib(f.text, `file://${f.path}`);
    } catch (err) {
      console.warn('webui: the editor context did not load:', err);
    }
  })();
}

// A JSON editor's schema is registered for that editor's own text, and let go of with it.
// Each editor has its own copy, so that two with one schema are matched by their own models.
let schemaCount = 0;
function schemas(id, model) {
  const files = (context.schemas || {})[id];
  if (!workers.json || !files) return () => {};
  const base = `file:///schemas/${++schemaCount}`;
  let registered = [];
  let gone = false;
  fetchFiles(files)
    .then((loaded) => {
      if (gone) return;
      registered = loaded.map((f) => ({
        uri: base + f.path,
        schema: JSON.parse(f.text),
        ...(f.path === '/schema.json' ? { fileMatch: [model.uri.toString()] } : {}),
      }));
      const current = monaco.json.jsonDefaults.diagnosticsOptions;
      monaco.json.jsonDefaults.setDiagnosticsOptions({
        ...current,
        validate: true,
        // The application declared the schema as what the text must be, so a value that
        // does not fit is an error and not Monaco's default of a warning.
        schemaValidation: 'error',
        enableSchemaRequest: false,
        schemas: [...(current.schemas || []), ...registered],
      });
    })
    .catch((err) => console.warn('webui: the editor context did not load:', err));
  return () => {
    gone = true;
    if (!registered.length) return;
    const current = monaco.json.jsonDefaults.diagnosticsOptions;
    monaco.json.jsonDefaults.setDiagnosticsOptions({
      ...current,
      schemas: (current.schemas || []).filter((s) => !registered.includes(s)),
    });
  };
}

// An editor's text is a model of its own with an address, which is what lets a schema be
// matched to it and keeps two editors' texts apart as files of one program.
const EXTENSION = { typescript: 'ts', javascript: 'js', json: 'json' };
let editorCount = 0;

// ---------------------------------------------------------------------- mounting

const OPTIONS = {
  automaticLayout: true,
  minimap: { enabled: false },
  scrollBeyondLastLine: false,
  fontSize: 13,
  lineHeight: 20,
  renderLineHighlight: 'line',
  overviewRulerBorder: false,
  padding: { top: 8, bottom: 8 },
  tabSize: 2,
  // An editor that has nothing to scroll leaves the wheel to the page, instead of holding it.
  scrollbar: { alwaysConsumeMouseWheel: false },
};

const mounted = new Map(); // element → what lets its editor go

function host(el) {
  const box = document.createElement('div');
  box.className = 'code-host';
  el.appendChild(box);
  el.classList.add('mounted');
  return box;
}

// What script says is said in the page's status region, which is there on every page (see enhance.js).
function say(text) {
  const region = document.querySelector('[data-announce]');
  if (!region) return;
  region.textContent = '';
  setTimeout(() => {
    region.textContent = text;
  }, 60);
}

// An editor holds the Tab key, since it is a character in the text, and a keyboard has no other way out of it, so
// the editor says how to leave (the page, beside the editor) and does it: Escape lets the next Tab move on, which
// is what Monaco's own Ctrl+M does for every editor on the page and does not say. The editor holds Tab again at
// the next key that is not a Tab, and when it loses the focus. Escape is left to what has a use for it first: a
// list of suggestions, the find box, a snippet, a selection.
const ESCAPE_LEAVES = [
  'editorTextFocus',
  '!suggestWidgetVisible',
  '!findWidgetVisible',
  '!parameterHintsVisible',
  '!renameInputVisible',
  '!editorHoverVisible',
  '!inSnippetMode',
  '!editorHasSelection',
  '!editorHasMultipleSelections',
  '!inlineSuggestionVisible',
].join(' && ');

function leavable(editor) {
  const hold = () => editor.updateOptions({ tabFocusMode: false });
  editor.addCommand(
    monaco.KeyCode.Escape,
    () => {
      editor.updateOptions({ tabFocusMode: true });
      say('Tab now moves focus out of the editor');
    },
    ESCAPE_LEAVES,
  );
  editor.onKeyDown((e) => {
    if (!editor.getOption(monaco.editor.EditorOption.tabFocusMode)) return;
    const k = monaco.KeyCode;
    if ([k.Tab, k.Escape, k.Shift, k.Ctrl, k.Alt, k.Meta].includes(e.keyCode)) return;
    hold();
  });
  editor.onDidBlurEditorText(hold);
}

// The control that a reader's keyboard is in is not the textarea that the server drew, which is hidden, but the
// one Monaco makes (or the element that takes its place where the browser has an EditContext), so what the server
// said about the field is said about that one: that it is required, that it was refused, and where the words are.
function describeInput(editor, textarea, hint) {
  const input = editor.getDomNode()?.querySelector('textarea.inputarea, .native-edit-context');
  if (!input) return () => {};
  const sync = () => {
    const described = [textarea.getAttribute('aria-describedby'), hint.id].filter(Boolean).join(' ');
    input.setAttribute('aria-describedby', described);
    if (textarea.required || textarea.hasAttribute('data-required')) input.setAttribute('aria-required', 'true');
    if (textarea.getAttribute('aria-invalid') === 'true') input.setAttribute('aria-invalid', 'true');
    else input.removeAttribute('aria-invalid');
  };
  sync();
  // Monaco writes some of these again when it takes the focus.
  editor.onDidFocusEditorText(sync);
  return sync;
}

// A required editor that is empty says so before the form leaves, as a required select does (see enhance.js): the
// textarea that would have said so is hidden and cannot be pointed at, and the server asks as well.
function required(editor, textarea, form, sync) {
  if (!textarea.required) return;
  textarea.required = false;
  textarea.setAttribute('data-required', '');
  const body = textarea.closest('.code-body');
  const id = `${textarea.id}-error`;
  const clear = () => {
    const made = document.getElementById(id);
    if (made && made.hasAttribute('data-made')) made.remove();
    textarea.removeAttribute('aria-invalid');
    textarea.setAttribute('aria-describedby', (textarea.getAttribute('aria-describedby') || '').split(' ').filter((x) => x && x !== id).join(' '));
    if (!textarea.getAttribute('aria-describedby')) textarea.removeAttribute('aria-describedby');
    sync();
  };
  form.addEventListener('submit', (e) => {
    if (editor.getValue() !== '') return;
    e.preventDefault();
    let err = document.getElementById(id);
    if (!err) {
      err = document.createElement('div');
      err.className = 'err code-error';
      err.id = id;
      err.setAttribute('role', 'alert');
      err.setAttribute('data-made', '');
      err.textContent = textarea.getAttribute('data-msg-value-missing') || 'This field is required.';
      body.appendChild(err);
    }
    textarea.setAttribute('aria-invalid', 'true');
    const described = (textarea.getAttribute('aria-describedby') || '').split(' ').filter(Boolean);
    if (!described.includes(id)) described.push(id);
    textarea.setAttribute('aria-describedby', described.join(' '));
    sync();
    editor.focus();
  });
  editor.onDidChangeModelContent(() => {
    if (document.getElementById(id)?.hasAttribute('data-made')) clear();
  });
}

// Ctrl+S or Cmd+S is the form's button pressed, so that what the button does is done: a button that asks first
// asks, and one that is not offered (a Guard that disabled it) is not pressed. The form is not sent from here.
function save(form) {
  const buttons = [...form.querySelectorAll('button')].filter((b) => !b.closest('dialog') && !b.disabled);
  const button = buttons.find((b) => b.hasAttribute('data-confirm-open')) || buttons.find((b) => b.type === 'submit');
  if (button) button.click();
}

function mountEditor(el) {
  const language = el.dataset.language || 'plaintext';
  const textarea = el.querySelector('textarea');
  const pre = el.querySelector('pre');
  const readOnly = !textarea;
  const text = textarea ? textarea.value : pre ? pre.textContent : '';
  const extension = EXTENSION[language];
  const model = extension
    ? monaco.editor.createModel(text, language, monaco.Uri.parse(`file:///editor-${++editorCount}.${extension}`))
    : null;
  const editor = monaco.editor.create(host(el), {
    ...OPTIONS,
    ...(model ? { model } : { value: text, language }),
    readOnly,
    domReadOnly: readOnly,
    ariaLabel: el.dataset.label || 'Editor',
  });
  let release = () => {};
  if (model && el.dataset.context) {
    if (language === 'json') release = schemas(el.dataset.context, model);
    else program(language);
  }
  let hint = null;
  if (textarea) {
    // The textarea is what the form posts, so it is kept as the editor has it.
    editor.onDidChangeModelContent(() => {
      textarea.value = editor.getValue();
    });
    hint = document.createElement('p');
    hint.className = 'code-hint';
    hint.id = `${textarea.id}-hint`;
    hint.textContent = 'Tab types a tab. To leave the editor, press Esc and then Tab.';
    el.after(hint);
    leavable(editor);
    const sync = describeInput(editor, textarea, hint);
    const form = textarea.closest('form');
    if (form) {
      required(editor, textarea, form, sync);
      editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => save(form));
    }
  }
  return () => {
    release();
    if (hint) hint.remove();
    editor.dispose();
    if (model) model.dispose();
  };
}

// A diff is side by side from this width of its panel, and one column, the lines of each side interleaved, below it.
// The panel says which on its body (data-diff), because the header above it names the two sides, and in one column
// they are not sides any more.
const SIDE_BY_SIDE_FROM = 900;

function mountDiff(el) {
  const language = el.dataset.language || 'plaintext';
  const side = (name) => {
    const node = el.querySelector(`pre[data-side="${name}"]`);
    return node ? node.textContent : '';
  };
  const editor = monaco.editor.createDiffEditor(host(el), {
    ...OPTIONS,
    readOnly: true,
    originalEditable: false,
    renderSideBySide: el.clientWidth >= SIDE_BY_SIDE_FROM,
    // Monaco would also fall back to one column on its own when it thinks it is short of room, with the
    // header still naming two sides: the mode is ours, and follows the width of the panel as it changes.
    useInlineViewWhenSpaceIsLimited: false,
    ariaLabel: el.dataset.label || 'Changes',
  });
  const body = el.closest('.code-body');
  let sideBySide;
  const follow = () => {
    const wide = el.clientWidth >= SIDE_BY_SIDE_FROM;
    if (wide === sideBySide) return;
    sideBySide = wide;
    editor.updateOptions({ renderSideBySide: wide });
    if (body) body.dataset.diff = wide ? 'split' : 'inline';
  };
  follow();
  const watch = new ResizeObserver(follow);
  watch.observe(el);
  const original = monaco.editor.createModel(side('original'), language);
  const modified = monaco.editor.createModel(side('modified'), language);
  editor.setModel({ original, modified });
  return () => {
    watch.disconnect();
    if (body) delete body.dataset.diff;
    editor.dispose();
    original.dispose();
    modified.dispose();
  };
}

// A fence names its language as people write it, "js" or "yml", and Monaco knows it by
// its id, which is not always that: it is found by id, by alias and by file extension.
const languages = new Map();
function languageOf(name) {
  if (!languages.size) {
    for (const l of monaco.languages.getLanguages()) {
      for (const key of [l.id, ...(l.aliases || []), ...(l.extensions || []).map((e) => e.replace(/^\./, ''))]) {
        if (!languages.has(key.toLowerCase())) languages.set(key.toLowerCase(), l.id);
      }
    }
  }
  return languages.get(name.toLowerCase());
}

// The code blocks of markdown are plain text until the highlighters are here, and stay
// that for a language Monaco does not know.
const blocks = new Map(); // code element → its text and language

// Colouring a text asks for a language's tokenizer, but a language with a service, JSON for
// one, sets its own up when a model is made in it, so one is made, once, and let go.
const asked = new Set();
function ask(id) {
  if (asked.has(id)) return;
  asked.add(id);
  monaco.editor.createModel('', id).dispose();
}

function paint(el) {
  const { text, id } = blocks.get(el);
  ask(id);
  monaco.editor
    .colorize(text, id, { tabSize: 2 })
    .then((html) => {
      if (el.isConnected && el.innerHTML !== html) el.innerHTML = html;
    })
    .catch(() => {
      // It stays as it was.
    });
}

// A language whose tokenizer is registered after it was asked for, as JSON's is, would
// be left uncoloured, so each block is painted again a little later. A block that came
// out the same is left alone.
const REPAINT = [300, 1200, 3000];

function colorize(root = document) {
  for (const el of blocks.keys()) if (!el.isConnected) blocks.delete(el);
  root.querySelectorAll('.md pre > code[class*="language-"]').forEach((el) => {
    if (blocks.has(el) || el.dataset.colorized) return;
    el.dataset.colorized = '1';
    const id = languageOf((/language-(\S+)/.exec(el.className) || [])[1] || '');
    if (!id || id === 'plaintext') return;
    blocks.set(el, { text: el.textContent.replace(/\n$/, ''), id });
    paint(el);
    for (const ms of REPAINT) setTimeout(() => blocks.has(el) && paint(el), ms);
  });
}

function scan(root = document) {
  colorize(root);
  root.querySelectorAll('[data-code-editor], [data-code-diff]').forEach((el) => {
    if (mounted.has(el)) return;
    mounted.set(el, el.hasAttribute('data-code-diff') ? mountDiff(el) : mountEditor(el));
  });
}

// A panel that is replaced takes its editor with it, which is let go of when it leaves,
// and one that arrives, from a refresh of a panel, is mounted.
new MutationObserver((changes) => {
  // What an editor does to its own markup is not a change of the page.
  if (changes.every((c) => c.target.closest && c.target.closest('.code-host'))) return;
  for (const [el, dispose] of mounted) {
    if (!el.isConnected) {
      dispose();
      mounted.delete(el);
    }
  }
  scan();
}).observe(document.body, { childList: true, subtree: true });

applyTheme();
scan();
