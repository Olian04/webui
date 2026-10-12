package editorctx

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Olian04/webui/test/util/assert"
)

func files(m map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, text := range m {
		out[name] = &fstest.MapFile{Data: []byte(text)}
	}
	return out
}

func details(ps []Problem) string {
	var parts []string
	for _, p := range ps {
		parts = append(parts, p.Detail)
	}
	return strings.Join(parts, "\n")
}

func TestStripJSONCReadsWhatTscReads(t *testing.T) {
	t.Parallel()

	src := `{
  // a line comment
  "compilerOptions": { /* a block
     comment */
    "target": "es2022", // trailing
    "lib": ["es2022", "dom",],
    "paths": { "@/*": ["src//*"], },
  },
}`
	var v struct {
		CompilerOptions struct {
			Target string
			Lib    []string
			Paths  map[string][]string
		}
	}
	assert.NoError(t, json.Unmarshal(stripJSONC([]byte(src)), &v))
	assert.Equal(t, v.CompilerOptions.Target, "es2022")
	assert.Equal(t, len(v.CompilerOptions.Lib), 2)
	assert.Equal(t, v.CompilerOptions.Paths["@/*"][0], "src//*") // a comment marker inside a string stays
}

func FuzzStripJSONCNeverGrowsAndKeepsPlainJSON(f *testing.F) {
	f.Add(`{"a": [1, 2], "b": "x//y"}`)
	f.Add(`{"a": 1, /* c */ }`)
	f.Add(`"unterminated`)
	f.Fuzz(func(t *testing.T, s string) {
		out := stripJSONC([]byte(s))
		if len(out) > len(s) {
			t.Fatalf("grew: %q -> %q", s, out)
		}
		// Plain JSON, which has no comment and no trailing comma, is not changed.
		if json.Valid([]byte(s)) && !strings.Contains(s, "//") && !strings.Contains(s, "/*") && string(out) != s {
			t.Fatalf("changed valid JSON: %q -> %q", s, out)
		}
	})
}

func TestATSConfigBecomesTheObjectTheServiceIsGiven(t *testing.T) {
	t.Parallel()

	ctx, ps := Read(files(map[string]string{
		"typescript/tsconfig.json":                      `{"compilerOptions": {"target": "ES2022", "module": "esnext", "moduleResolution": "bundler", "lib": ["es2022", "dom"], "strict": true, "noUnusedLocals": false, "paths": {"@/*": ["./x/*"]}}}`,
		"typescript/env.d.ts":                           "declare const host: string;",
		"typescript/node_modules/@types/foo/index.d.ts": "export const foo: number;",
		"json/schema.json":                              `{}`,
	}), "typescript")
	assert.Equal(t, details(ps), "")
	assert.Equal(t, ctx.Compiler["target"], 9)
	assert.Equal(t, ctx.Compiler["module"], 99)
	assert.Equal(t, ctx.Compiler["moduleResolution"], 100)
	assert.Equal(t, ctx.Compiler["strict"], true)
	assert.Equal(t, ctx.Compiler["noUnusedLocals"], false)
	assert.Equal(t, ctx.Compiler["lib"].([]string)[1], "lib.dom.d.ts")
	assert.Equal(t, len(ctx.Files), 2)
	assert.Equal(t, ctx.Files[0].Path, "/env.d.ts")
	assert.Equal(t, ctx.Files[1].Path, "/node_modules/@types/foo/index.d.ts")
	assert.True(t, strings.HasSuffix(ctx.Files[0].Name, ".d.ts"))
}

func TestAContextWithTheSameFilesIsTheSameContext(t *testing.T) {
	t.Parallel()

	a, _ := Read(files(map[string]string{"typescript/a.d.ts": "declare const a: 1;"}), "typescript")
	b, _ := Read(files(map[string]string{"typescript/a.d.ts": "declare const a: 1;"}), "typescript")
	c, _ := Read(files(map[string]string{"typescript/a.d.ts": "declare const a: 2;"}), "typescript")
	assert.Equal(t, a.ID, b.ID)
	assert.True(t, a.ID != c.ID)
	assert.True(t, a.Files[0].Name != c.Files[0].Name)
}

func TestASchemaFolderNeedsAValidSchemaJSON(t *testing.T) {
	t.Parallel()

	ctx, ps := Read(files(map[string]string{
		"json/schema.json":      `{"$ref": "./defs/common.json"}`,
		"json/defs/common.json": `{"type": "string"}`,
	}), "json")
	assert.Equal(t, details(ps), "")
	assert.Equal(t, len(ctx.Files), 2)

	_, ps = Read(files(map[string]string{"json/other.json": `{}`}), "json")
	assert.Contains(t, details(ps), "has no schema.json")
	_, ps = Read(files(map[string]string{"json/schema.json": `{`}), "json")
	assert.Contains(t, details(ps), "json/schema.json is not valid JSON")
	_, ps = Read(files(map[string]string{"json/schema.json": `{}`, "json/notes.txt": "x"}), "json")
	assert.Contains(t, details(ps), "json/notes.txt is not a .json file")
}

func TestAFolderThatDoesNotFitTheEditorIsAnErrorThatNamesWhy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		language string
		fs       map[string]string
		want     string
	}{
		{"no directory for the language", "typescript", map[string]string{"json/schema.json": `{}`}, `no "typescript" directory`},
		{"a stray top-level file", "typescript", map[string]string{"typescript/a.d.ts": "", "env.d.ts": ""}, `"env.d.ts", which is not the directory of a language`},
		{"a misspelt directory", "typescript", map[string]string{"typescript/a.d.ts": "", "typscript/b.d.ts": ""}, `"typscript", which is not the directory`},
		{"a source file", "typescript", map[string]string{"typescript/a.ts": ""}, "typescript/a.ts is neither"},
		{"a nested tsconfig", "typescript", map[string]string{"typescript/sub/tsconfig.json": "{}"}, "typescript/sub/tsconfig.json is neither"},
		{"broken tsconfig", "typescript", map[string]string{"typescript/tsconfig.json": "{"}, "tsconfig.json is not valid JSON"},
		{"project key", "typescript", map[string]string{"typescript/tsconfig.json": `{"extends": "./base.json"}`}, `"extends", which describes a project`},
		{"unknown key", "typescript", map[string]string{"typescript/tsconfig.json": `{"scripts": {}}`}, `"scripts", which the editor does not read`},
		{"unknown option", "typescript", map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"outDir": "x"}}`}, "compilerOptions.outDir is not an option the editor reads"},
		{"bad enum value", "typescript", map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"target": "es1999"}}`}, `compilerOptions.target is "es1999", which is not one of`},
		{"bad flag", "javascript", map[string]string{"javascript/tsconfig.json": `{"compilerOptions": {"strict": "yes"}}`}, "compilerOptions.strict must be true or false"},
		{"bad lib", "typescript", map[string]string{"typescript/tsconfig.json": `{"compilerOptions": {"lib": ["es2099"]}}`}, `names "es2099"`},
	}
	for _, tc := range cases {
		ctx, ps := Read(files(tc.fs), tc.language)
		if ctx != nil {
			t.Errorf("%s: a context was made from a bad folder", tc.name)
		}
		assert.Contains(t, details(ps), tc.want)
		for _, p := range ps {
			assert.True(t, p.Fix != "")
		}
	}
}

func TestOnlyTheLanguagesDirectoryIsReadAndTheOthersAreLeftAlone(t *testing.T) {
	t.Parallel()

	fsys := files(map[string]string{"typescript/a.d.ts": "", "json/schema.json": `{}`, "javascript/b.d.ts": ""})
	ts, ps := Read(fsys, "typescript")
	assert.Equal(t, details(ps), "")
	assert.Equal(t, len(ts.Files), 1)
	js, _ := Read(fsys, "javascript")
	assert.Equal(t, js.Files[0].Path, "/b.d.ts")
}

func TestDirKnowsTheLanguagesThatHaveContext(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Dir("typescript"), "typescript")
	assert.Equal(t, Dir("javascript"), "javascript")
	assert.Equal(t, Dir("json"), "json")
	assert.Equal(t, Dir("css"), "")
	assert.Equal(t, Dir(""), "")
}
