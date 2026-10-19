// Regenerates portable.json: what a browser's regular expression engine says about each pattern in the corpus below, for
// each input, so that the Go tests can hold the library's own answers against it without a browser.
//
//	node internal/rules/testdata/gen.mjs
//
// An HTML pattern attribute is compiled as new RegExp("^(?:" + pattern + ")$", "v") (the "u" flag before browsers had "v"),
// and that is what is run here, with both flags. The engine is the one in node, V8; Firefox and Safari have their own, and
// the corpus holds only what the specifications settle.
import { writeFileSync } from 'node:fs';

// What the patterns are tried on: the edges of the things that differ between engines.
export const inputs = [
  '', 'a', 'A', 'ab', 'abc', 'ABC', 'a b', ' ', '  ', '\t', 'é', 'É', 'ß', 'İ', 'ı', '\u212a', '\u017f', '😀', 'a😀', '٣',
  '1', '12', '123', '1.5', 'a-b', 'a_b', 'a@b.c', 'a\nb', 'a\rb', 'a\u2028b', 'a\u2029b', 'a\u00a0b', 'a\u000bb',
  'a\ufeffb', 'a\u2003b', 'a\u3000b', '-', '[', ']', '{', '}', '(', ')', '/', '\\', '^', '$', 'x$', '$x', '日本語', 'Ωmega',
  'ǅ', 'a\u0301', 'a.b', 'aaaa', 'ab ab', '192.168.1.10', '555-1234', 'ab|cd', 'a&b', 'a!b', 'a,b', 'a;b', 'a:b', 'a~b',
  'a\'b', 'a"b', 'a`b', 'a=b', 'a<b', 'a>b', 'a#b', 'a%b', 'a+b', 'a*b', 'a?b', 'a@@b', '\u0378', 'x', 'xy', 'Ab', 'aB',
  '٣٣', 'ａ', 'a1', '\n', 'a\n', '\r', '\u2028', '\u2029', '\u000b', '\f', '\u00a0', '\u1680', '\u180e', '\u2000', '\u200a',
  '\u200b', '\u202f', '\u205f', '\u2003', '\u3000', '\ufeff', '\u0085', '\u001c', '\u001f', ' a', 'a ', ' a ', 'a\tb',
];

// What is meant to work in both engines, and what is meant not to. The list is what the library's check is held to.
export const patterns = [
  // literals and the syntax characters, escaped
  'abc', 'a|b', 'a.c', 'a.*', '.*', '.+', '.', '..', 'é', '日本', '😀', '[😀]', ' ', 'a b', '[ ]', '[a ]', 'x\\$', '\\$x', '\\^', '\\.',
  '\\\\', '\\/', '\\(a\\)', '\\[a\\]', '\\{a\\}', 'a\\|b', '\\+', '\\*', '\\?', '/', 'a/b', '-', 'a-b', '@', 'a@b', ',', ';', ':', '!',
  '#', '%', '&', '~', '=', '<', '>', "'", '"', '`', 'a&&b', 'a@@b', 'a!!b', 'a--b', ']', '}', 'a}', '{', 'a{', 'a{,3}', 'a{x}', 'a{1',
  // classes
  '\\d+', '\\D+', '\\w+', '\\W+', '\\s+', '\\S+', '\\b', '\\Bb', '\\bab\\b', 'a\\b', '\\bé', 'é\\b', '[a-z]+', '[A-Za-z0-9_]+', '[^a-z]+',
  '[abc]', '[a-c]', '[a-]', '[-a]', '[a\\-z]', '[\\-a]', '[a-z-]', '[\\w.\\-]+', '[\\w.\\-]+@[\\w.\\-]+', '[\\d]', '[\\D]', '[\\w]', '[\\W]',
  '[\\s]', '[^\\d]', '[\\d\\w]', '[\\d-z]', '[a-\\d]', '[]a]', '[^]a]', '[\\]a]', '[^\\]a]', '[\\^a]', '[a^]', '[^^]', '[\\\\]', '[\\[]', '[[]',
  '[a|b]', '[\\|]', '[(]', '[)]', '[\\(\\)]', '[{]', '[}]', '[\\{\\}]', '[/]', '[\\/]', '[.]', '[\\.]', '[+]', '[*]', '[?]', '[$]', '[\\$]', '[@]',
  '[,]', '[:]', '[;]', '[!]', '[#]', '[%]', '[&]', '[~]', '[=]', '[<]', '[>]', "['\"]", '[`]', '[_]', '[@@]', '[a&&b]', '[!!]', '[##]', '[%%]',
  '[a--b]', '[a-é]', '[é]', '[ -~]', '[\\x20-\\x7e]', '[a-z][0-9]', '[^\\n]', '[^a]', '[\\t]', '[\\n]', '[[:alpha:]]', '[[:^alpha:]]', '[[:digit:]]',
  // groups
  '(ab)+', '(?:ab)+', '(a|b)*c', '()', '(?:)', 'a||b', '(|a)', '(a|)', '(a)(b)', '((a))', '(?:(?:a))', '(?P<x>a)', '(?<x>a)', '(?i)a', '(?s).',
  '(?m)^a', '(?U)a+', '(?-i)a', '(?i:a)', '(?s:.)',
  // repetition
  'a{2}', 'a{2,}', 'a{2,3}', 'a{0}', 'a{0,0}', 'a{3,2}', 'a+?', 'a*?', 'a??', 'a{2}?', 'a{2,}?', 'a{2,3}?', 'a++', 'a*+', 'a?+', 'a{2}+', 'a**', 'a*{2}',
  'a{2}{3}', 'a{1001}', 'a{1000}', '(?:a{1000}){1000}', '^*', '$+', '\\b+', '\\B*', '^?', '(?:^)*', '(?:\\b)+', '*a', '+a', '?a', '{2}', '|*',
  // anchors
  '^a', 'a$', '^abc$', '^', '$', '^$', 'a^b', 'a$b', '(^a)', '(a$)', '^a|b$', '\\Aa', 'a\\z', '\\Aa\\z', 'a\\Z', '\\G', '\\Qa.b\\E', '\\Q', '\\E',
  // escapes
  '\\_', '\\-', '\\"', '\\ ', '\\\'', '\\@', '\\!', '\\#', '\\%', '\\&', '\\,', '\\:', '\;', '\\<', '\\>', '\\=', '\\~', '\\`', '\\x41', '\\x{41}',
  '\\u0041', '\\u{41}', '\\t', '\\n', '\\r', '\\f', '\\v', '\\a', '\\0', '\\012', '\\cA', '\\e', '\\C', '\\a', '\\y', '\\é', '\\😀', '\\1', '\\k<x>',
  // unicode properties
  '\\pL', '\\pN', '\\PL', '\\p{L}+', '\\P{L}+', '\\p{Lu}', '\\p{Ll}', '\\p{Lt}', '\\p{Lm}', '\\p{Lo}', '\\p{N}+', '\\p{Nd}', '\\p{Nl}', '\\p{No}',
  '\\p{P}', '\\p{Pc}', '\\p{Pd}', '\\p{Ps}', '\\p{Pe}', '\\p{Pi}', '\\p{Pf}', '\\p{Po}', '\\p{S}', '\\p{Sm}', '\\p{Sc}', '\\p{Sk}', '\\p{So}',
  '\\p{Z}', '\\p{Zs}', '\\p{Zl}', '\\p{Zp}', '\\p{M}', '\\p{Mn}', '\\p{Mc}', '\\p{Me}', '\\p{C}', '\\p{Cc}', '\\p{Cf}', '\\p{Co}', '\\p{Cs}',
  '\\p{Cn}', '\\p{Greek}', '\\p{Latin}', '\\p{Han}', '\\p{Letter}', '\\p{Lowercase_Letter}', '\\p{Script=Greek}', '\\p{sc=Greek}',
  '\\p{Alphabetic}', '\\p{Any}', '\\p{ASCII}', '\\p{^L}', '\\P{^L}', '\\p', '\\p{', '\\p{}', '\\p{L', '\\p{l}', '\\p{LU}', '\\pl',
  '[\\p{L}\\d]+', '[\\p{L} ]+', '[\\p{L}-]+', '[\\p{L}\\-]+', '[^\\p{L}]', '[\\P{L}]', '[\\p{Lu}\\p{Ll}]', '[\\pL]', '[\\p{^L}]',
  // white space and the dot, which a browser reads as more characters than Go does
  '\\s', '\\S', '[\\S]', '[^\\S]', '[\\S ]', '[\\s\\d]', '[a\\s]', '[^\\s\\d]', '\\s*\\S+\\s*', '[\\S\\s]', '[\\W\\S]', '^\\S+@\\S+$', '.?',
  '.{2}', '.*?', '(.)(.)', '[.]*', '(?:.|\\n)', ' *', '\\p{L}+(?: \\p{L}+)*', '[\\p{L}\\p{N}_]+', '[^.]', '[^\\n\\r]', '\\x0b', '\\xa0', '[\\xa0]', '[\\x00-\\x1f]',
  // real patterns
  '\\d{3}-\\d{4}', '[0-9]{1,3}(\\.[0-9]{1,3}){3}', '\\d{1,3}(\\.\\d{1,3}){3}', '[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]{2,}', '[a-z][a-z0-9-]*',
  '[a-z][a-z0-9\\-]*', '[A-Z]{2}\\d{4}', '(\\+\\d{1,3})?\\d{6,14}', '[\\p{L}\\p{M}\\- ]+', '\\p{Lu}\\p{Ll}+', '[^\\s]+', '[^ \\t]+',
  'https?://[^ ]+', 'https?:\\/\\/[^ ]+', '(foo|bar)-\\d+', '#[0-9a-f]{6}', '\\#[0-9a-f]{6}', '[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}',
];

function run(src, flags) {
  let re;
  try {
    re = new RegExp('^(?:' + src + ')$', flags);
  } catch (e) {
    return { ok: false };
  }
  return { ok: true, matches: inputs.map((s) => (re.test(s) ? '1' : '0')).join('') }; // a digit for each input: matched or not
}

// The code points, as ranges, that a regular expression has one of its own meaning for: \s, and what a dot does not match.
function ranges(re) {
  const out = [];
  for (let cp = 0; cp <= 0x10ffff; cp++) {
    if (cp >= 0xd800 && cp <= 0xdfff) continue;
    if (!re.test(String.fromCodePoint(cp))) continue;
    const last = out[out.length - 1];
    if (last && last[1] === cp - 1) last[1] = cp;
    else out.push([cp, cp]);
  }
  return out;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const out = { engine: 'V8 ' + process.versions.v8, white: ranges(/^\s$/u), notDot: ranges({ test: (c) => !/^.$/u.test(c) }), inputs, patterns: patterns.map((p) => ({ pattern: p, v: run(p, 'v'), u: run(p, 'u') })) };
  writeFileSync(new URL('./portable.json', import.meta.url), '{\n "engine": ' + JSON.stringify(out.engine) + ',\n "white": ' + JSON.stringify(out.white) + ',\n "notDot": ' + JSON.stringify(out.notDot) + ',\n "inputs": ' + JSON.stringify(inputs) + ',\n "patterns": [\n' + out.patterns.map((p) => '  ' + JSON.stringify(p)).join(',\n') + '\n ]\n}' + '\n');
  console.log(out.patterns.length + ' patterns, ' + inputs.length + ' inputs, ' + out.engine);
}
