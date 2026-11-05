// Builds the Monaco editor assets that webui embeds, from the pinned monaco-editor
// package. Run it with `make monaco`. Its output is committed, because the module is
// built by `go get` and there is no step for this on the other side.
//
// What it makes:
//
//   pkg/monaco/editor/               the editor: one ES module (our glue and Monaco
//                                    together), its lazily loaded chunks (one for each
//                                    language's highlighting), the stylesheet, the icon
//                                    font and the plain editor worker.
//   pkg/monaco/<service>/worker/     the worker of each language service, one package for
//                                    each, so an application that does not import it does
//                                    not carry it.
//   pkg/lang/lang.go                 a constant for each language Monaco highlights,
//   pkg/webui/language_gen.go        and the list of them that the editor checks a Language against.
import * as esbuild from 'esbuild';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const monacoDir = path.join(here, 'node_modules/monaco-editor');
const version = JSON.parse(fs.readFileSync(path.join(monacoDir, 'package.json'), 'utf8')).version;

const coreOut = path.join(root, 'pkg/monaco/editor');
const services = { json: 'json', css: 'css', html: 'html', typescript: 'ts' };

const common = {
  bundle: true,
  minify: true,
  legalComments: 'none',
  logLevel: 'warning',
  loader: { '.ttf': 'file' },
  assetNames: 'assets/[name]-[hash]',
};

fs.rmSync(coreOut, { recursive: true, force: true });
fs.mkdirSync(coreOut, { recursive: true });

// ------------------------------------------------------------------------- core
const core = await esbuild.build({
  ...common,
  entryPoints: { code: path.join(here, 'src/code.js') },
  outdir: coreOut,
  format: 'esm',
  splitting: true,
  chunkNames: 'chunks/[name]-[hash]',
  entryNames: '[name]-[hash]',
  metafile: true,
});

// Each lazily loaded chunk that imports the editor's styles gets its own copy of them,
// and none is ever linked: the page links the one beside the entry.
for (const file of fs.readdirSync(path.join(coreOut, 'chunks'))) {
  if (file.endsWith('.css')) fs.rmSync(path.join(coreOut, 'chunks', file));
}

const worker = async (name, entry, outdir) => {
  const result = await esbuild.build({
    ...common,
    entryPoints: { [name]: entry },
    outdir,
    format: 'iife',
    entryNames: '[name]-[hash]',
    metafile: true,
  });
  return Object.keys(result.metafile.outputs)[0];
};

const editorWorker = await worker('editor.worker', 'monaco-editor/editor/editor.worker.js', path.join(coreOut, 'workers'));

const outputs = Object.keys(core.metafile.outputs).map((o) => path.relative(coreOut, path.resolve(here, o)));
const entry = outputs.find((o) => /^code-[A-Z0-9]+\.js$/.test(o));
const style = outputs.find((o) => /^code-[A-Z0-9]+\.css$/.test(o));
if (!entry || !style) throw new Error(`no entry or stylesheet in ${outputs.join(', ')}`);

fs.writeFileSync(
  path.join(coreOut, 'manifest.json'),
  JSON.stringify({ version, entry, style, editorWorker: path.relative(coreOut, path.resolve(here, editorWorker)) }, null, 2) + '\n',
);
fs.copyFileSync(path.join(monacoDir, 'LICENSE'), path.join(coreOut, 'LICENSE-monaco.txt'));
fs.copyFileSync(path.join(monacoDir, 'ThirdPartyNotices.txt'), path.join(coreOut, 'THIRD-PARTY-NOTICES-monaco.txt'));

// -------------------------------------------------------------- language services
const entries = {
  json: 'monaco-editor/language/json/json.worker.js',
  css: 'monaco-editor/language/css/css.worker.js',
  html: 'monaco-editor/language/html/html.worker.js',
  typescript: 'monaco-editor/language/typescript/ts.worker.js',
};
for (const [pkg, id] of Object.entries(services)) {
  const dir = path.join(root, 'pkg/monaco', pkg, 'worker');
  fs.rmSync(dir, { recursive: true, force: true });
  fs.mkdirSync(dir, { recursive: true });
  await worker(`${id}.worker`, entries[pkg], dir);
}

// --------------------------------------------------------------------- languages
// Monaco highlights what its definitions register, and the json feature registers json.
const definitions = path.join(monacoDir, 'esm/vs/languages/definitions');
const ids = new Set(['plaintext', 'json']);
for (const dir of fs.readdirSync(definitions)) {
  const file = path.join(definitions, dir, 'register.js');
  if (!fs.existsSync(file)) continue;
  for (const m of fs.readFileSync(file, 'utf8').matchAll(/id:\s*"([^"]+)"/g)) {
    if (/^[a-z0-9-]+$/.test(m[1])) ids.add(m[1]); // freemarker also registers dotted variants of itself
  }
}

const names = {
  plaintext: 'PlainText', cpp: 'CPP', csharp: 'CSharp', fsharp: 'FSharp', qsharp: 'QSharp', 'objective-c': 'ObjectiveC',
  sql: 'SQL', json: 'JSON', yaml: 'YAML', html: 'HTML', css: 'CSS', scss: 'SCSS', xml: 'XML', php: 'PHP', hcl: 'HCL', mdx: 'MDX',
  ini: 'INI', csp: 'CSP', ecl: 'ECL', pgsql: 'PostgreSQL', mysql: 'MySQL', msdax: 'MSDAX', st: 'StructuredText', sb: 'SmallBasic',
  pla: 'PLA', vb: 'VisualBasic', wgsl: 'WGSL', azcli: 'AzureCLI', bat: 'Batch', systemverilog: 'SystemVerilog', graphql: 'GraphQL',
  restructuredtext: 'ReStructuredText', powerquery: 'PowerQuery', powershell: 'PowerShell', javascript: 'JavaScript',
  coffeescript: 'CoffeeScript', typescript: 'TypeScript', typespec: 'TypeSpec', protobuf: 'Protobuf', freemarker2: 'Freemarker', postiats: 'ATS', m3: 'Modula3',
  cameligo: 'CameLIGO', pascaligo: 'PascaLIGO', lexon: 'Lexon', redis: 'Redis', redshift: 'Redshift', sophia: 'Sophia',
};
const pascal = (id) => names[id] ?? id.split(/[-_]/).map((p) => p[0].toUpperCase() + p.slice(1)).join('');

const sorted = [...ids].sort((a, b) => pascal(a).localeCompare(pascal(b)));
const constants = sorted.map((id) => `\t${pascal(id)} webui.Language = ${JSON.stringify(id)}`);
fs.mkdirSync(path.join(root, 'pkg/lang'), { recursive: true });
fs.writeFileSync(
  path.join(root, 'pkg/lang/lang.go'),
  `// Code generated by tools/monaco/build.mjs from monaco-editor ${version}; DO NOT EDIT.

// Package lang has a constant for each language the code editor highlights, to give an [webui.Editor] or a [webui.Diff] as its Language: lang.Go, lang.JSON, lang.YAML,
// lang.Markdown and so on. They are every language Monaco ships a highlighter for, which is all of them whether or not a language service is imported, and their
// values are Monaco's own language ids. They are a package of their own so that they are not most of the names in the root package. A Language is a string, so
// a program that wants one of its own may convert one: webui.Language("go"); one that Monaco does not highlight is refused when the app is compiled.
package lang

import "github.com/Olian04/webui/pkg/webui"

const (
${constants.join('\n')}
)
`,
);
fs.writeFileSync(
  path.join(root, 'pkg/webui/language_gen.go'),
  `// Code generated by tools/monaco/build.mjs from monaco-editor ${version}; DO NOT EDIT.

package webui

// languages is every language the editor highlights, by Monaco's own language id, for validation. The constants for them are in pkg/lang.
var languages = map[Language]bool{
${sorted.map((id) => `\t${JSON.stringify(id)}: true,`).join('\n')}
}
`,
);

execFileSync('gofmt', ['-w', path.join(root, 'pkg/webui/language_gen.go'), path.join(root, 'pkg/lang/lang.go')]);
console.log(`monaco ${version}: ${entry}, ${sorted.length} languages`);
