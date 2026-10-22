package webui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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

	// Thumbnail is where the app serves a small picture of the file, if it is an image: a path on this site, which is shown beside
	// the name, at 32 pixels, and, when the file has a URL too, opens it at its full size in a dialog when it is clicked. The library never shows the
	// file itself as a thumbnail, which would send the visitor the whole of an image to see a few pixels of it, so without a Thumbnail
	// there is no picture. [Thumbnail] makes one from an [Upload].
	Thumbnail string
}

// Upload is a file that a visitor sent, and that was written to the [FileWriter] that [File.Open] returned for it. It is what is known
// of the file once all of it has arrived: it holds no bytes, which are where the writer put them.
type Upload struct {
	// Name is what the visitor's file was called, as a base name: never a path, with the control characters taken out and at
	// most 255 characters. It is the visitor's, so it is not to be used as a name on disk.
	Name string

	// Size is how many bytes were written.
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
}

// Incoming is what is known of a file when it begins to arrive, which is when [File.Open] is called: before any of it has been written,
// and so before the whole of it can be checked.
type Incoming struct {
	// Name is what the visitor's file is called, as [Upload.Name] has it.
	Name string

	// ContentType is what the first 512 bytes of the file hold, as [Upload.ContentType] has it. For JSON, CSV and text, which are told by
	// reading all of the file, it is only what those bytes look like (a "text/plain" type); the [Upload] has the final one.
	ContentType string

	// Kind is the kind of an image or a PDF, which its first bytes tell, and [FileAny] for any other file, whose kind is known only
	// when all of it has arrived, in [Upload.Kind].
	Kind FileKind
}

// FileWriter is where the bytes of an uploaded file go, and what is done with them once the submission is decided. A [File] gives one
// for each file that arrives (see [File.Open]), and the library writes the file to it as it is received, with nothing kept
// of its own, in memory or on disk, so that a file may be as large as the app takes and may go straight to where the app keeps it, an object
// store or a disk of its own.
//
// The writer manages what it wrote, all the way. The library calls Close when all of the file has been written, or when it is not going to be,
// and then exactly one of Submit and Discard: Submit when the submission was accepted, to make the file permanent (to move the file
// from the temporary name that it was written under to its own, say), and Discard when it was not, because the file was refused, or a
// rule, a Guard or the action's Run refused or failed the submission, or the request ended before it was done, to remove it. Close
// is only the end of the writing, and Submit and Discard are only called after it. Their errors are logged; a failed Close refuses the file, and a failed Submit tells the
// visitor that the file could not be stored.
type FileWriter interface {
	io.WriteCloser

	// Submit makes what was written permanent. It is called once, after Close, when the form was accepted.
	Submit() error

	// Discard removes what was written. It is called once, after Close, when the form was not accepted. It may be called for a
	// file that was only partly written.
	Discard() error
}

// NewFileWriter makes a [FileWriter] of any [io.WriteCloser], which is what a file on disk, a pipe to an object store and a buffer with a
// Close are: Close closes w, and submit and discard, either of which may be nil for nothing to do, are what Submit and Discard do. A writer of a
// file on disk that is written under a temporary name passes a submit that renames it and a discard that removes it.
func NewFileWriter(w io.WriteCloser, submit, discard func() error) FileWriter {
	return fileWriter{WriteCloser: w, submit: submit, discard: discard}
}

type fileWriter struct {
	io.WriteCloser
	submit, discard func() error
}

func (f fileWriter) Submit() error {
	if f.submit == nil {
		return nil
	}
	return f.submit()
}

func (f fileWriter) Discard() error {
	if f.discard == nil {
		return nil
	}
	return f.discard()
}

// File projects a file of M that a visitor can send, and the one that the app holds. It is an input of a form: a native file
// chooser, with the file that is stored shown above it. In a table, or where it is only read, it is the stored file's name,
// which is a link when Load gives it a URL.
//
// The library keeps no files, not in memory and not on disk. Open is called as a file begins to arrive, and returns the
// [FileWriter] that the file is written to as it is received, which is how a file of any size goes to where the app keeps it, an object
// store or a disk, with no copy of it made first; the writer makes the file permanent when the submission is accepted and
// removes it when it is not (see [FileWriter]). Store is handed the [Upload] when every field has been checked, to note in the model what
// arrived, and Load says what is kept; where the app serves it from is the app's own address. A file that the app serves should be sent with
// Content-Disposition: attachment, with X-Content-Type-Options: nosniff and with the content type that the Upload has, and
// never from the page's own origin as the page it is inline in: what a visitor sent is theirs, and an HTML or SVG file that a
// browser opens is a script that runs as the app.
//
// A submission that chooses no file leaves the stored one alone and calls neither Open nor Store. One that chooses a file calls
// Open when it arrives, and Store with the Upload after every field has been checked, and only if none of them is refused, so
// a form that is refused stores nothing, and has the writer discard what it was given; a browser cannot refill a file
// chooser, so the visitor chooses the file again, which the field says. With Removable, a checkbox beside the stored file calls Store with nil.
//
// The file is checked by what it holds, and its size, as it arrives: a file that is not one of the kinds that the field takes, which its
// first bytes tell for an image or a PDF, is refused before Open is called, and one that is larger than MaxSize is
// refused when it reaches it, and nothing more is written. JSON, CSV and text are told by all of the file, so they are written and then
// refused if they are not what they were taken for. A page that has a File reads the body of a submission as multipart, in the order that
// it is sent in, up to the sum of the files' MaxSize and the usual limit, after the page's Guard has run, so a visitor
// that the page refuses has nothing read from them. Open is called with the model as it was loaded, as the first part of the file arrives,
// and so before the fields that the browser sends after the file have been read, and before the action's own Guard, which runs after
// the form has been bound and so after the file has been received: an Open that is to refuse a visitor does so itself, from the context. A
// browser that sends a file that is refused still sends all of it, so the rest of it is read and thrown away, up to the limit, and
// the form is drawn again with what was typed. A program that takes uploads sets ReadTimeout on its server, which
// ReadHeaderTimeout does not bound.
//
// It is an input of a [Form] and a column of a [Table], and nothing else: not of a [Toolbar], which shows values and
// has only a name to show of a file, and an [Editor]'s content is a [String] by its type.
type File[M any] struct {
	// Label is the column header and the input's label. In a table it also names the column in the address and in a [Query].
	Label string

	// Load reads the file that the model holds, or nil for none.
	Load func(M) *FileInfo

	// Store is handed a file that the visitor sent, after all of it has been written and every field has been checked, or nil for one that
	// they asked to remove (see Removable). It notes in the model what arrived, for Run; the bytes are in the [FileWriter] that Open
	// returned. Without it the field is read-only, and it needs an Open.
	Store func(*M, *Upload)

	// Open is called when a file begins to arrive, with the model as it was loaded, and returns where the file is written (see
	// [FileWriter]). An error refuses the file before any of it is written, and its cause is logged, not shown. It is called once for
	// each file that is chosen, and not for a chooser that is left alone. Without it, Store is a compile error.
	Open func(ctx context.Context, m M, in Incoming) (FileWriter, error)

	// Accept lists the kinds of file that the field takes. Left empty it takes any file.
	Accept []FileKind

	// MaxSize is the most bytes that a file may have; 5 MiB when it is 0. How large is the app's to say, and the library sets no ceiling of its own; a negative size is a compile error.
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
	case a.Store != nil && a.Open == nil:
		v.add(at+" has a Store but no Open", "A file is written as it arrives, to where Open says: set Open, or remove Store to make the field read-only.")
	case a.Open != nil && a.Store == nil:
		v.add(at+" has an Open but no Store", "Open is where a new file goes, and Store is what notes it in the model: set Store, or remove Open.")
	case a.Removable && a.Store == nil:
		v.add(at+" is Removable but has no Store", "A file is removed by calling Store with nil: set Store, or remove Removable.")
	case a.Required && a.Store == nil:
		v.add(at+" is Required but has no Store", "A file that is required is one that can be sent: set Store, or remove Required.")
	}
	if a.MaxSize < 0 {
		v.add(fmt.Sprintf("%s has MaxSize %d", at, a.MaxSize), "Use a size in bytes, or leave it 0 for 5 MiB.")
	}
	for i, k := range a.Accept {
		if int(k) > int(FileAny) {
			v.add(fmt.Sprintf("%s: Accept[%d] is not a FileKind", at, i), "Use one of the File… constants.")
		}
	}
}

// errNoFileWriter is what an Open that returned no writer and no error is taken as.
var errNoFileWriter = errors.New("webui: File.Open returned no FileWriter and no error")

// samePath is whether an address is a path on this site, which is the only place that a picture is shown from: the content security policy
// lets an image be this site's own, and a thumbnail is not one that an address elsewhere should be asked for.
func samePath(s string) bool {
	return strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") && !strings.HasPrefix(s, `/\`) && !strings.ContainsAny(s, "\n\r")
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
		if samePath(info.Thumbnail) {
			out.Thumb = info.Thumbnail
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
			a.Store(m.(*M), &Upload{Name: up.Name, Size: up.Size, ContentType: up.ContentType, Kind: fileKindOf(up.Kind)})
		}
		spec.Open = func(ctx context.Context, m any, in ir.Incoming) (ir.FileWriter, error) {
			w, err := a.Open(ctx, m.(M), Incoming{Name: in.Name, ContentType: in.ContentType, Kind: fileKindOf(in.Kind)})
			if err != nil {
				return nil, err //nolint:wrapcheck // the app's own error, which is logged as it is
			}
			if w == nil {
				return nil, errNoFileWriter
			}
			return w, nil
		}
	}
	f.File = spec
	return f
}
