// Package editorctx reads the context of an editor: the files its language service reads
// besides the text, which an application hands over as one folder with a directory for each
// language it gives context to.
//
//	typescript/tsconfig.json   compiler options, for the TypeScript editors
//	typescript/*.d.ts          declarations, at any depth
//	javascript/...             the same, for the JavaScript editors
//	json/schema.json           the schema of a JSON editor's text
//	json/*.json                other files, for the schema to refer to
//
// Reading is done once, when the application is compiled, so that a mistake in the folder is a
// compile error and not a service that quietly knows nothing.
package editorctx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"
)

// Problem is something wrong with a context folder, and how to put it right.
type Problem struct{ Detail, Fix string }

// File is one file of a context.
type File struct {
	// Path is where the language service sees it, inside the language's own directory:
	// "/env.d.ts", or "/node_modules/@types/foo/index.d.ts" for a library.
	Path string

	// Name is what it is served as, the hash of its contents and its extension, so a browser
	// can keep it for good and two files with the same text are one.
	Name string

	Text []byte
}

// Context is what one editor's language is given.
type Context struct {
	// Dir is the directory the files were found in, which is also the language's name.
	Dir string

	// ID is a hash of every file and option, so editors given the same folder share a context.
	ID string

	Files []File

	// Compiler is the compiler options, as the editor's script gives them to TypeScript: set
	// for a TypeScript or JavaScript editor whose directory has a tsconfig.json.
	Compiler map[string]any
}

// dirs are the directories a context may have, and the editor languages each is for.
var dirs = []string{"typescript", "javascript", "json"}

// Dir is the directory of a language, by Monaco's name for it, or "" for a language that has
// no context.
func Dir(language string) string {
	if slices.Contains(dirs, language) {
		return language
	}
	return ""
}

// Service is the language service that reads the context: TypeScript's, which takes
// JavaScript as well, or JSON's.
func (c *Context) Service() string {
	if c.Dir == "json" {
		return "json"
	}
	return "ts"
}

// Read reads what the folder holds for a language, which must be one that Dir knows. It
// returns the problems with the folder, and no Context when there are any.
func Read(fsys fs.FS, language string) (*Context, []Problem) {
	dir := Dir(language)
	var problems []Problem
	bad := func(detail, fix string) { problems = append(problems, Problem{Detail: detail, Fix: fix}) }

	top, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, []Problem{{Detail: "Context cannot be read: " + err.Error(), Fix: "Give it a folder, such as an embed.FS made with //go:embed."}}
	}
	found := false
	for _, e := range top {
		switch {
		case e.Name() == dir && e.IsDir():
			found = true
		case slices.Contains(dirs, e.Name()) && e.IsDir():
		default:
			bad(fmt.Sprintf("Context has %q, which is not the directory of a language", e.Name()),
				fmt.Sprintf("Put the files of a language in a directory named for it, one of %s; use fs.Sub to hand over a folder that holds them.", strings.Join(dirs, ", ")))
		}
	}
	if !found {
		bad(fmt.Sprintf("Context has no %q directory, which is the one for this editor's language", dir),
			fmt.Sprintf("Add %s/ to the folder, or remove Context from the editor.", dir))
	}
	if len(problems) > 0 {
		return nil, problems
	}

	ctx := &Context{Dir: dir}
	walkErr := fs.WalkDir(fsys, dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(name, dir+"/")
		text, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		switch {
		case dir != "json" && rel == "tsconfig.json":
			opts, ps := readTSConfig(text)
			for _, p := range ps {
				bad(dir+"/tsconfig.json: "+p.Detail, p.Fix)
			}
			ctx.Compiler = opts
		case dir != "json" && strings.HasSuffix(rel, ".d.ts"):
			ctx.Files = append(ctx.Files, newFile("/"+rel, ".d.ts", text))
		case dir == "json" && strings.HasSuffix(rel, ".json"):
			var v any
			if err := decodeJSON(text, &v); err != nil {
				bad(fmt.Sprintf("%s/%s is not valid JSON: %v", dir, rel, err), "Fix the file; a schema must be JSON.")
				break
			}
			ctx.Files = append(ctx.Files, newFile("/"+rel, ".json", text))
		case dir == "json":
			bad(fmt.Sprintf("%s/%s is not a .json file", dir, rel), "Only schemas, as .json files, belong in json/.")
		default:
			bad(fmt.Sprintf("%s/%s is neither a tsconfig.json at the top nor a .d.ts file", dir, rel),
				"Only declaration files (*.d.ts) and a top-level tsconfig.json belong here; the editor reads nothing else.")
		}
		return nil
	})
	if walkErr != nil {
		bad(fmt.Sprintf("Context cannot be read: %v", walkErr), "Check the folder.")
	}
	if dir == "json" && !slices.ContainsFunc(ctx.Files, func(f File) bool { return f.Path == "/schema.json" }) && len(problems) == 0 {
		bad("json/ has no schema.json", "Name the schema of the editor's text schema.json; other files are for it to refer to.")
	}
	if len(problems) > 0 {
		return nil, problems
	}
	sort.Slice(ctx.Files, func(i, j int) bool { return ctx.Files[i].Path < ctx.Files[j].Path })
	ctx.ID = id(ctx)
	return ctx, nil
}

func newFile(p, ext string, text []byte) File {
	sum := sha256.Sum256(text)
	return File{Path: path.Clean(p), Name: hex.EncodeToString(sum[:8]) + ext, Text: text}
}

// id hashes what a context holds, so it changes when any file or option does.
func id(c *Context) string {
	var b strings.Builder
	b.WriteString(c.Dir + "\n")
	for _, f := range c.Files {
		b.WriteString(f.Path + " " + f.Name + "\n")
	}
	_, _ = fmt.Fprint(&b, c.Compiler) // a map prints in the order of its keys
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}
