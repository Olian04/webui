package webui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The library's words and the internals' are kept apart (docs/terminology.md): what a person who uses the library reads says nothing of how it is built. This is the part of
// that rule that a machine can hold. It reads what a user reads, which is the documentation of every exported name, the package documentation, the texts of the compile errors,
// the README and the skill, and fails on a word that only the insides have, or a word that the glossary retired.
//
// A word is added to the list when it is retired or kept to the insides, and the glossary says so first.

// internalWords are the words of the insides (the intermediate representation, the passes over it, the in-process "may open" handler, the cache of a Guard's answer, the whole
// of the sidebar and the bars) and the words that the glossary retired from the library's own (leaf, the navbar, the chrome).
var internalWords = regexp.MustCompile(
	`\b(leaf|IR|lowers|lowering|lowered|nodes?|gates?|verdicts?|chrome|navbar|shell)\b` +
		`|\b(the|its|own|all|these|those|other|two|three|sibling|Markdown) leaves\b`)

func TestThePublicDocsSayNoInternalWord(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "language_gen.go" { // the names of the languages, and shell is one
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = f
	}

	check := func(where, text string) {
		t.Helper()
		for i, line := range strings.Split(text, "\n") {
			if m := internalWords.FindString(line); m != "" {
				t.Errorf("%s (line %d of the text) says %q, which is not a word of the library (docs/terminology.md): %s", where, i+1, m, strings.TrimSpace(line))
			}
		}
	}
	doc := func(where string, g *ast.CommentGroup) {
		t.Helper()
		if g != nil {
			check(where, g.Text())
		}
	}
	exportedRecv := func(d *ast.FuncDecl) bool {
		if d.Recv == nil || len(d.Recv.List) == 0 {
			return true
		}
		e := d.Recv.List[0].Type
		for {
			switch x := e.(type) {
			case *ast.StarExpr:
				e = x.X
				continue
			case *ast.IndexExpr:
				e = x.X
				continue
			case *ast.IndexListExpr:
				e = x.X
				continue
			}
			break
		}
		id, ok := e.(*ast.Ident)
		return ok && id.IsExported()
	}

	for name, f := range files {
		doc(name+": package documentation", f.Doc)
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() && exportedRecv(d) {
					doc(name+": "+d.Name.Name, d.Doc)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						if s.Doc != nil {
							doc(name+": "+s.Name.Name, s.Doc)
						} else {
							doc(name+": "+s.Name.Name, d.Doc)
						}
						switch typ := s.Type.(type) {
						case *ast.StructType:
							for _, fl := range typ.Fields.List {
								for _, n := range fl.Names {
									if n.IsExported() {
										doc(name+": "+s.Name.Name+"."+n.Name, fl.Doc)
									}
								}
							}
						case *ast.InterfaceType:
							for _, fl := range typ.Methods.List {
								for _, n := range fl.Names {
									if n.IsExported() {
										doc(name+": "+s.Name.Name+"."+n.Name, fl.Doc)
									}
								}
							}
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								if s.Doc != nil {
									doc(name+": "+n.Name, s.Doc)
								} else {
									doc(name+": "+n.Name, d.Doc)
								}
							}
						}
					}
				}
			}
		}
		// The texts of the compile errors, and of anything else that is a string in the package, are read by the person who has to fix the declaration.
		ast.Inspect(f, func(n ast.Node) bool {
			if bl, ok := n.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				if s, err := strconv.Unquote(bl.Value); err == nil {
					check(fset.Position(bl.Pos()).String()+": a string", s)
				}
			}
			return true
		})
	}

	// The README says what the library is and how to use it up to "Developing webui", which is for the people who work on it; the skill is all for the people who use it.
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	usage, _, found := strings.Cut(string(readme), "\n## Developing webui")
	if !found {
		t.Fatal(`README.md has no "Developing webui" section to end the part that users read at`)
	}
	check("README.md", usage)
	skill, err := os.ReadFile("../../skills/webui/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	check("skills/webui/SKILL.md", string(skill))
}
