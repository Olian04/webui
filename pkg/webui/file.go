package webui

import (
	"fmt"
	"io"

	"github.com/Olian04/webui/internal/ir"
)

// FileKind is a kind of file that a [File] accepts. A file is of a kind by what it holds, which the library reads (the first bytes
// of an image or a PDF, and the whole of JSON, CSV and text), and not by its name or by the type that the browser says it has,
// which the visitor chooses.
type FileKind uint8

// The kinds. A [File] that lists none accepts any file.
const (
	// FilePNG, FileJPEG, FileGIF and FileWebP are the images of those formats.
	FilePNG FileKind = iota
	FileJPEG
	FileGIF
	FileWebP

	// FileImage is any of the four.
	FileImage

	// FilePDF is a PDF document.
	FilePDF

	// FileJSON is exactly one JSON value, as UTF-8 text.
	FileJSON

	// FileCSV is comma-separated records, as UTF-8 text, with at least one.
	FileCSV

	// FileText is UTF-8 text with no NUL, which is not empty. HTML and XML, which a browser would run, are not text.
	FileText

	// FileAny is any file. It is what a [File] with no Accept takes, and nothing is known of what it holds beyond what was
	// sniffed. There is no kind for SVG, which is XML that a browser runs as a page and so is a way to inject script: an
	// app that takes it says FileAny, and serves what it stores so that no browser runs it (see [File]).
	FileAny
)

// fileKinds are the kinds as the IR holds them, by their number.
var fileKinds = [...]ir.FileKind{ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP, ir.FileImage, ir.FilePDF, ir.FileJSON, ir.FileCSV, ir.FileText, ir.FileAny}

// ir is the kind as the IR holds it.
func (k FileKind) ir() ir.FileKind { return fileKinds[k] }

// fileKindOf is the kind that the IR names. One it does not know is FileAny.
func fileKindOf(k ir.FileKind) FileKind {
	for i, name := range fileKinds {
		if name == k {
			return FileKind(i)
		}
	}
	return FileAny
}

// maxFileSize is the most that a [File] may be allowed to take, which is a bound on what a mistake can ask of the server.
const maxFileSize = 256 << 20

// defaultFileSize is what a [File] with no MaxSize takes.
const defaultFileSize = 5 << 20

// FileInfo is a file that the app holds, as a form shows it: its name, how large it is and where the app serves it.
type FileInfo struct {
	// Name is what the file is called, which a form shows.
	Name string

	// Size is how many bytes it has, 0 for not knowing.
	Size int64

	// URL is where the app serves the file, the address that the name links to, or "" for no link. It is held to the rules of a
	// [URL]: a path on this site, or an http, https or mailto address; anything else is shown as plain text.
	URL string
}

// Upload is a file that a visitor sent. It is held on the server, in memory or in a file of its own, until the request is
// answered, and then it is gone: Open is valid until the action's Run returns, so a Run that is to keep the file copies it,
// and a Store that only notes the Upload for Run to use has not kept it.
type Upload struct {
	// Name is what the visitor's file was called, as a base name: never a path, with the control characters taken out and at
	// most 255 characters. It is the visitor's, so it is not to be used as a name on disk.
	Name string

	// Size is how many bytes it has.
	Size int64

	// ContentType is what the file was found to hold (see [FileKind]), and not what the browser said it was. It is a media
	// type, as a Content-Type header has it, which is what to serve the file as.
	ContentType string

	// Kind is which of the kinds the file was found to be, to tell what arrived from a field that takes several: [FilePNG],
	// [FileJPEG], [FileGIF], [FileWebP], [FilePDF], [FileJSON], [FileCSV] or [FileText], and never [FileImage], which is
	// four of them. It is [FileAny] for a file that is none of those, which only a field that takes any file lets in. A field that
	// takes any file finds the kind of an image or a PDF from its first bytes, and of nothing else, since the text kinds are
	// read through to be told, which is not done for a file that nothing asked to be one. A file can be more than one of the kinds
	// that a field takes (JSON is also text, and a number alone is JSON and CSV), and the Kind is the most specific of them,
	// whatever order Accept lists them in: the images and the PDF, which exclude one another, then JSON, then CSV, then text.
	Kind FileKind

	// Open reads the file from its start, as often as it is called. The caller closes it.
	Open func() (io.ReadCloser, error)
}

// File projects a file of M that a visitor can send, and the one that the app holds. It is an input of a form: a native file
// chooser, with the file that is stored shown above it. In a table, or where it is only read, it is the stored file's name,
// which is a link when Load gives it a URL.
//
// The library keeps no files. Store is handed an [Upload], which it copies where the app keeps them, and Load says what is
// kept; where the app serves it from is the app's own address. A file that the app serves should be sent with
// Content-Disposition: attachment, with X-Content-Type-Options: nosniff and with the content type that the Upload has, and
// never from the page's own origin as the page it is inline in: what a visitor sent is theirs, and an HTML or SVG file that a
// browser opens is a script that runs as the app.
//
// A submission that chooses no file leaves the stored one alone and does not call Store. One that chooses a file calls Store
// with it, after every field has been checked, and only if none of them is refused, so a form that is refused stores nothing; and
// a browser cannot refill a file chooser, so the visitor chooses the file again, which the field says. With Removable, a
// checkbox beside the stored file calls Store with nil.
//
// The file is checked by what it holds, and its size, before Store sees it. A page that has a File reads the body of a
// submission as multipart, up to the sum of the files' MaxSize and the usual limit, after the page's Guard has run, so a visitor
// that the page refuses has nothing read from them. An action's own Guard runs after the form has been bound, so the file
// has been received when it refuses. A program that takes uploads sets ReadTimeout and WriteTimeout on its server, which a long
// upload would otherwise only be bounded by the header timeout of.
//
// It is an input of a [Form] and a column of a [Table], and nothing else: not of a [Toolbar], whose inputs are the page's
// arguments in its address, and an [Editor]'s content is a [String] by its type.
type File[M any] struct {
	// Label is the column header and the input's label. In a table it also names the column in the address and in a [Query].
	Label string

	// Load reads the file that the model holds, or nil for none.
	Load func(M) *FileInfo

	// Store is handed a file that the visitor sent, or nil for one that they asked to remove (see Removable). Without it the
	// field is read-only.
	Store func(*M, *Upload)

	// Accept lists the kinds of file that the field takes. Left empty it takes any file.
	Accept []FileKind

	// MaxSize is the most bytes that a file may have; 5 MiB when it is 0. A negative size, or one over 256 MiB, is a compile error.
	MaxSize int64

	// Required says the model must have a file: one that is stored and is not being removed, or a new one.
	Required bool

	// Removable puts a Remove checkbox beside the stored file, which calls Store with nil. It needs a Store.
	Removable bool

	// Help is a line or two that tells the visitor what the field is for, or what to put in it, where its label and
	// its rules cannot: it is in the popover beside the label that lists the rules, under a heading of its own and above
	// them, on hover and on keyboard focus, and is part of the icon's accessible name. It is shown where the field is, in a
	// form, read-only too. In a table it is not shown. It is text, not markup.
	Help string

	// NoSort leaves the sort link off the column's header, in a table: it cannot be sorted by,
	// and a sort by it in the address is ignored. It does nothing in a form.
	NoSort bool

	// NoFilter leaves the filter off the column's header, in a table: it cannot be filtered by,
	// and a filter on it in the address is ignored. It does nothing in a form.
	NoFilter bool
}

func (a File[M]) label() string { return a.Label }

func (a File[M]) meta() accessorMeta {
	return accessorMeta{NoSort: a.NoSort, NoFilter: a.NoFilter, Help: a.Help}
}

// validateFile checks what a File says of itself, and where it is.
func validateFile[M any](v *bodyValidator, at string, a File[M]) {
	switch {
	case a.Removable && a.Store == nil:
		v.add(at+" is Removable but has no Store", "A file is removed by calling Store with nil: set Store, or remove Removable.")
	case a.Required && a.Store == nil:
		v.add(at+" is Required but has no Store", "A file that is required is one that can be sent: set Store, or remove Required.")
	}
	if a.MaxSize < 0 || a.MaxSize > maxFileSize {
		v.add(fmt.Sprintf("%s has MaxSize %d", at, a.MaxSize), "Use a size from 1 byte to 256 MiB, or leave it 0 for 5 MiB.")
	}
	for i, k := range a.Accept {
		if int(k) > int(FileAny) {
			v.add(fmt.Sprintf("%s: Accept[%d] is not a FileKind", at, i), "Use one of the File… constants.")
		}
	}
}

// lowerFile is a File as the IR holds it.
func lowerFile[M any](name string, a File[M]) ir.Field {
	f := head(name, a.Label, ir.KindString)
	f.Display, f.Nullable = ir.DisplayLink, true
	stored := func(m any) *ir.StoredFile {
		info := a.Load(m.(M))
		if info == nil {
			return nil
		}
		out := &ir.StoredFile{Name: info.Name, Size: info.Size, URL: info.URL}
		if !validURL(out.URL) {
			out.URL = "" // a link that is not safe to follow is shown as a name
		}
		return out
	}
	f.Get = func(m any) string {
		if s := stored(m); s != nil {
			return s.Name
		}
		return ""
	}
	f.Link = func(m any) string {
		if s := stored(m); s != nil {
			return s.URL
		}
		return ""
	}
	spec := &ir.FileSpec{Max: a.MaxSize, Removable: a.Removable, Stored: stored}
	if spec.Max == 0 {
		spec.Max = defaultFileSize
	}
	for _, k := range a.Accept {
		spec.Kinds = append(spec.Kinds, k.ir())
	}
	f.Rules.Required = a.Required
	if a.Store != nil {
		spec.Store = func(m any, up *ir.Upload) {
			if up == nil {
				a.Store(m.(*M), nil)
				return
			}
			a.Store(m.(*M), &Upload{Name: up.Name, Size: up.Size, ContentType: up.ContentType, Kind: fileKindOf(up.Kind), Open: up.Open})
		}
	}
	f.File = spec
	return f
}
