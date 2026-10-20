package main

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Olian04/webui/pkg/webui"
)

// FileArgs is the rest of the path: for /files/logs/2026/10 the Path is
// ["logs", "2026", "10"]. The last segment of a path may be written {path...}, and then
// takes every segment from there on, one element of a []string for each. Each is a
// breadcrumb that leads to the folder on the way, so a nested folder reads as the folders
// above it.
type FileArgs struct {
	Path []string `webui:"path"`
}

// FileEntry is a file or a folder in a folder, and the folder it is in.
type FileEntry struct {
	Parent []string
	Name   string
	Folder bool
	Size   int
}

// tree is the whole tree, a folder's path joined by "/" to what is in it. A name that
// ends in "/" is a folder, and has an entry of its own.
var tree = map[string][]string{
	"":             {"config/", "logs/", "snapshots/"},
	"config":       {"collector.yml", "sinks/"},
	"config/sinks": {"file.yml", "stdout.yml"},
	"logs":         {"2026/"},
	"logs/2026":    {"09/", "10/"},
	"logs/2026/09": {"collector.log"},
	"logs/2026/10": {"collector.log", "sink-file.log"},
	"snapshots":    {"2026-10-01.tar.gz"},
}

// entriesIn is what is in a folder, folders first.
func entriesIn(folder []string) []FileEntry {
	names := tree[strings.Join(folder, "/")]
	out := make([]FileEntry, 0, len(names))
	for _, name := range names {
		e := FileEntry{Parent: folder, Name: strings.TrimSuffix(name, "/"), Folder: strings.HasSuffix(name, "/")}
		if !e.Folder {
			e.Size = 1024 * (1 + len(name)*7%40)
		}
		out = append(out, e)
	}
	slices.SortStableFunc(out, func(a, b FileEntry) int {
		switch {
		case a.Folder && !b.Folder:
			return -1
		case !a.Folder && b.Folder:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// lookup is the entry at a path, if there is one.
func lookup(p []string) (FileEntry, bool) {
	if len(p) == 0 {
		return FileEntry{Name: "/", Folder: true}, true
	}
	for _, e := range entriesIn(p[:len(p)-1]) {
		if e.Name == p[len(p)-1] {
			return e, true
		}
	}
	return FileEntry{}, false
}

var (
	FileName = webui.String[FileEntry]{Label: "Name", Load: func(e FileEntry) string {
		if e.Folder {
			return e.Name + "/"
		}
		return e.Name
	}}
	FileKind = webui.String[FileEntry]{Label: "Kind", Load: func(e FileEntry) string {
		if e.Folder {
			return "folder"
		}
		return strings.TrimPrefix(path.Ext(e.Name), ".")
	}}
	FileSize = webui.Int[FileEntry]{Label: "Size (bytes)", Load: func(e FileEntry) int { return e.Size }}
)

// FilesAtPath names the page of any path, for the rows that lead to it. A constant is not a
// variable, so the page and the table inside it can refer to each other.
const FilesAtPath webui.PageID[FileArgs] = "/files/{path...}"

// A row leads to the page for its path: the folder's own, with the name added to the end.
// A file has a page too, which says what it is.
var fileRowClick = webui.Link[FileEntry, FileArgs]{
	Page: FilesAtPath,
	Args: func(_ context.Context, e FileEntry) FileArgs {
		return FileArgs{Path: append(slices.Clone(e.Parent), e.Name)}
	},
}

func fileTable(title string, folder func(context.Context) []string) webui.Table[FileEntry] {
	return webui.Table[FileEntry]{
		Title:    title,
		Rows:     func(ctx context.Context) ([]FileEntry, error) { return entriesIn(folder(ctx)), nil },
		Columns:  []webui.Accessor[FileEntry]{FileName, FileKind, FileSize},
		RowClick: fileRowClick,
	}
}

// FileNote is the Markdown beside the table, about the path. Its links are paths in the
// app, such as /files/logs, which are given the mount prefix.
type FileNote struct{ Text string }

var FileNoteText = webui.String[FileNote]{Label: "About this path", Load: func(n FileNote) string { return n.Text }}

func fileNote(path []string) FileNote {
	e, ok := lookup(path)
	where := "/" + strings.Join(path, "/")
	kind := "a file of " + fmt.Sprint(e.Size) + " bytes"
	if e.Folder {
		kind = fmt.Sprintf("a folder of %d entries", len(entriesIn(path)))
	}
	if !ok {
		return FileNote{Text: "There is nothing at `" + where + "`."}
	}
	text := "`" + where + "` is " + kind + ".\n\n"
	for i := range path {
		text += fmt.Sprintf("- [%s](/files/%s)\n", path[i], strings.Join(path[:i+1], "/"))
	}
	return FileNote{Text: text + "\nBack to [all files](/files)."}
}

func fileNotePanel(path func(context.Context) []string) webui.Markdown[FileNote] {
	return webui.Markdown[FileNote]{
		Title:   "About this path",
		Load:    func(ctx context.Context) (FileNote, error) { return fileNote(path(ctx)), nil },
		Content: FileNoteText,
	}
}

// FilesPath is the root of the files, a constant for the reason that AlertsPath is.
const FilesPath webui.PageID[webui.NoArgs] = "/files"

var FilesRoot = webui.Page[webui.NoArgs]{
	Path: FilesPath,
	Nav:  webui.Nav{Label: "Files", Icon: "folder-open", Section: "Operations"},
	Body: webui.Stack{
		fileTable("Files", func(context.Context) []string { return nil }),
	},
}

func pathOf(ctx context.Context) []string { return webui.ArgsOf[FileArgs](ctx).Path }

// FilesAt is any folder or file under /files, however deep. It declares {path...} once, and
// its breadcrumb is one crumb for each segment of the path.
var FilesAt = webui.Page[FileArgs]{
	Path: FilesAtPath,
	// No Nav: it lights Files, the page at "/files".
	Guard: func(ctx context.Context, a FileArgs) error {
		if _, ok := lookup(a.Path); !ok {
			return webui.NotFound(fmt.Sprintf("There is nothing at /%s.", strings.Join(a.Path, "/"))).
				WithLink("Back to the files", webui.Open(ctx, FilesPath, webui.NoArgs{}))
		}
		return nil
	},
	Body: webui.Split{fileTable("Folder", pathOf), fileNotePanel(pathOf)},
}
