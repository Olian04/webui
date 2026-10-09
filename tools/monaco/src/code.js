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
  monaco.editor.defineTheme('webui', { base: isLight() ? 'vs' : 'vs-dark', inherit: true, rules: [], colors });
}

function applyTheme() {
  defineThemes();
  monaco.editor.setTheme('webui');
}
new MutationObserver(applyTheme).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
matchMedia('(prefers-color-scheme: light)').addEventListener('change', applyTheme);

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

function mountEditor(el) {
  const language = el.dataset.language || 'plaintext';
  const textarea = el.querySelector('textarea');
  const pre = el.querySelector('pre');
  const readOnly = !textarea;
  const text = textarea ? textarea.value : pre ? pre.textContent : '';
  const editor = monaco.editor.create(host(el), {
    ...OPTIONS,
    value: text,
    language,
    readOnly,
    domReadOnly: readOnly,
    ariaLabel: el.dataset.label || 'Editor',
  });
  if (textarea) {
    // The textarea is what the form posts, so it is kept as the editor has it.
    editor.onDidChangeModelContent(() => {
      textarea.value = editor.getValue();
    });
    const form = textarea.closest('form');
    if (form) {
      editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => form.requestSubmit());
    }
  }
  return () => editor.dispose();
}

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
    renderSideBySide: el.clientWidth >= 900,
    ariaLabel: el.dataset.label || 'Changes',
  });
  const original = monaco.editor.createModel(side('original'), language);
  const modified = monaco.editor.createModel(side('modified'), language);
  editor.setModel({ original, modified });
  return () => {
    editor.dispose();
    original.dispose();
    modified.dispose();
  };
}

function scan(root = document) {
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
