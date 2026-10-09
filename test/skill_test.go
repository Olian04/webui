package webui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// The skill in skills/webui teaches an agent to build with the library, and is meant
// to stay true as the library changes. It therefore names almost nothing of the API
// and sends the reader to the documentation for the version in use. These tests
// keep that promise: whatever it does name must exist, and the way it says to read the
// documentation must be one that works.

const skillPath = "../skills/webui/SKILL.md"

// exported is what the package declares: top-level names, and for each type the names
// of its fields and methods.
type exported struct {
	top     map[string]bool
	members map[string]map[string]bool
}

func declared(t *testing.T) exported {
	t.Helper()

	files, err := filepath.Glob("../pkg/webui/*.go")
	assert.NoError(t, err)
	out := exported{top: map[string]bool{}, members: map[string]map[string]bool{}}
	member := func(typ, name string) {
		if out.members[typ] == nil {
			out.members[typ] = map[string]bool{}
		}
		out.members[typ][name] = true
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		assert.NoError(t, err)
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					out.top[d.Name.Name] = true
					continue
				}
				member(receiverName(d.Recv.List[0].Type), d.Name.Name)
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						out.top[s.Name.Name] = true
						if st, ok := s.Type.(*ast.StructType); ok {
							for _, field := range st.Fields.List {
								for _, n := range field.Names {
									member(s.Name.Name, n.Name)
								}
							}
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							out.top[n.Name] = true
						}
					}
				}
			}
		}
	}
	return out
}

// receiverName is the type a method is declared on, without its type arguments.
func receiverName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	case *ast.IndexListExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}

func skill(t *testing.T) string {
	t.Helper()

	b, err := os.ReadFile(skillPath)
	assert.NoError(t, err)
	return string(b)
}

func TestTheSkillIsASkillThatPointsAtTheDocumentationForTheVersionInUse(t *testing.T) {
	t.Parallel()

	text := skill(t)
	assert.True(t, strings.HasPrefix(text, "---\nname: webui\ndescription:"))
	// It is found by the description, so the description says when to use it.
	assert.Contains(t, text, "github.com/Olian04/webui")

	// The documentation is how the skill stays true: the pinned version, offline, and
	// the published one with examples.
	assert.Contains(t, text, "go doc -all github.com/Olian04/webui/pkg/webui")
	assert.Contains(t, text, "https://pkg.go.dev/github.com/Olian04/webui@<version>/pkg/webui")
	assert.Contains(t, text, "the documentation is right")
}

func TestEveryNameTheSkillGivesTheLibraryExists(t *testing.T) {
	t.Parallel()

	text := skill(t)
	api := declared(t)

	// Identifiers written as webui.Name, if the skill ever uses any.
	for _, m := range regexp.MustCompile(`\bwebui\.([A-Z][A-Za-z0-9]*)`).FindAllStringSubmatch(text, -1) {
		if !api.top[m[1]] {
			t.Errorf("the skill names webui.%s, which the package does not declare", m[1])
		}
	}

	// The names it hands to go doc: a type, or a field or method of one.
	cmd := regexp.MustCompile(`go doc (?:-all )?github.com/Olian04/webui/pkg/webui(?: ([A-Z][A-Za-z0-9]*)(?:\.([A-Z][A-Za-z0-9]*))?)?`)
	for _, m := range cmd.FindAllStringSubmatch(text, -1) {
		typ, name := m[1], m[2]
		if typ != "" && !api.top[typ] {
			t.Errorf("the skill says to read go doc for %s, which the package does not declare", typ)
		}
		if name != "" && !api.members[typ][name] {
			t.Errorf("the skill says to read go doc for %s.%s, which the package does not declare", typ, name)
		}
	}
}
