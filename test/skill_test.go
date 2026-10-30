package webui_test

import (
	"encoding/json"
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

const (
	skillPath        = "../skills/webui/SKILL.md"
	upgradeSkillPath = "../skills/webui-upgrade/SKILL.md"
)

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

	return skillAt(t, skillPath)
}

func skillAt(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Clean(path))
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

	api := declared(t)
	for _, text := range []string{skill(t), skillAt(t, upgradeSkillPath)} {
		// Identifiers written as webui.Name, if the skill ever uses any.
		for _, m := range regexp.MustCompile(`\bwebui\.([A-Z][A-Za-z0-9]*)`).FindAllStringSubmatch(text, -1) {
			if !api.top[m[1]] {
				t.Errorf("a skill names webui.%s, which the package does not declare", m[1])
			}
		}

		// The names it hands to go doc: a type, or a field or method of one.
		cmd := regexp.MustCompile(`go doc (?:-all )?github.com/Olian04/webui/pkg/webui(?: ([A-Z][A-Za-z0-9]*)(?:\.([A-Z][A-Za-z0-9]*))?)?`)
		for _, m := range cmd.FindAllStringSubmatch(text, -1) {
			typ, name := m[1], m[2]
			if typ != "" && !api.top[typ] {
				t.Errorf("a skill says to read go doc for %s, which the package does not declare", typ)
			}
			if name != "" && !api.members[typ][name] {
				t.Errorf("a skill says to read go doc for %s.%s, which the package does not declare", typ, name)
			}
		}
	}
}

// The upgrade skill says how to move between versions and what changed in none of them. It is found by its description, reads the migration guides
// from the place the project keeps them, and does not list the API, which would be wrong for the versions it is not about.
func TestTheUpgradeSkillReadsTheMigrationGuidesAndListsNoAPI(t *testing.T) {
	t.Parallel()

	text := skillAt(t, upgradeSkillPath)
	assert.True(t, strings.HasPrefix(text, "---\nname: webui-upgrade\ndescription:"))
	assert.Contains(t, text, "github.com/Olian04/webui")

	// Where the guides are, and how to get them for any version: from the module cache, which holds the source of the version.
	assert.Contains(t, text, "docs/migrations/<version>.md")
	assert.Contains(t, text, "go mod download github.com/Olian04/webui@<version>")
	assert.Contains(t, text, "go list -m -f '{{.Dir}}' github.com/Olian04/webui@<version>")
	assert.Contains(t, text, "go list -m -versions github.com/Olian04/webui")

	// The fallbacks for a release with no guide, and the two directions.
	assert.Contains(t, text, "go doc -all ./pkg/webui")
	assert.Contains(t, text, "https://github.com/Olian04/webui/releases")
	assert.Contains(t, text, "A downgrade")

	// The convention it relies on is the one the repository's own guidance states, so that the two cannot drift: a guide per stable release, in this folder.
	guidance, err := os.ReadFile("../docs/AGENTS.md")
	assert.NoError(t, err)
	assert.Contains(t, string(guidance), "docs/migrations/vX.Y.Z.md")
}

// The skill is packaged for Claude Code and Cursor by manifests at the repository root,
// which is the plugin. They must be valid, name the plugin as the skill is named, and
// list it in the marketplace, or the install commands in the README stop working.
func TestThePluginManifestsPackageTheSkill(t *testing.T) {
	t.Parallel()

	read := func(path string) map[string]any {
		b, err := os.ReadFile(filepath.Clean(path))
		assert.NoError(t, err)
		var out map[string]any
		assert.NoError(t, json.Unmarshal(b, &out))
		return out
	}

	// Cursor takes the repository root as the plugin.
	cursor := read("../.cursor-plugin/plugin.json")
	assert.Equal(t, cursor["name"], "webui")
	assert.Equal(t, cursor["license"], "MIT")

	// Claude Code installs only the folder of the skill, not the Go source around it:
	// the entry names that folder as its source, and a folder with a SKILL.md at its
	// root is a plugin of one skill.
	market := read("../.claude-plugin/marketplace.json")
	assert.Equal(t, market["name"], "olian04")
	plugins, _ := market["plugins"].([]any)
	assert.Equal(t, len(plugins), 2)
	for i, want := range []struct{ name, path string }{{"webui", "skills/webui"}, {"webui-upgrade", "skills/webui-upgrade"}} {
		entry, _ := plugins[i].(map[string]any)
		assert.Equal(t, entry["name"], any(want.name))
		assert.Equal(t, entry["license"], "MIT")
		source, _ := entry["source"].(map[string]any)
		assert.Equal(t, source["source"], "git-subdir")
		assert.Equal(t, source["url"], "Olian04/webui")
		assert.Equal(t, source["path"], any(want.path))
		// No version: a skill is the same for every version of the library, so an
		// install follows the default branch.
		_, pinned := entry["version"]
		assert.False(t, pinned)
	}

	// The README tells people to install what the marketplace calls it.
	readme, err := os.ReadFile("../README.md")
	assert.NoError(t, err)
	assert.Contains(t, string(readme), "/plugin marketplace add Olian04/webui")
	assert.Contains(t, string(readme), "/plugin install webui@olian04")
	assert.Contains(t, string(readme), "/plugin install webui-upgrade@olian04")
	assert.Contains(t, string(readme), "npx skills add Olian04/webui")

	for _, path := range []string{"../skills/webui/SKILL.md", "../skills/webui-upgrade/SKILL.md"} {
		_, err = os.Stat(path)
		assert.NoError(t, err)
	}
}
