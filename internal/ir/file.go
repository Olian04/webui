package ir

import (
	"context"
	"io"
)

// FileKind is a kind of file that a field accepts, as the library tells it by what the file holds and not by what it is called.
type FileKind string

// The kinds. FileImage is the four images together, and FileAny is any file.
const (
	FilePNG   FileKind = "png"
	FileJPEG  FileKind = "jpeg"
	FileGIF   FileKind = "gif"
	FileWebP  FileKind = "webp"
	FileImage FileKind = "image"
	FilePDF   FileKind = "pdf"
	FileJSON  FileKind = "json"
	FileCSV   FileKind = "csv"
	FileText  FileKind = "text"
	FileAny   FileKind = "any"
)

// Order is where the kind comes among the kinds that a file is matched against: the more exactly a kind says what a file is, the
// sooner it is tried, so that a file which is several kinds is the most specific of those that a field takes, whatever order the
// field lists them in. The images and the PDF are told apart by their first bytes and exclude one another; JSON, a grammar that a
// CSV or a text is not, is before CSV, which is before text, which any file is; and FileAny is last. FileImage is four of the
// kinds before it, and has the place of the first of them.
func (k FileKind) Order() int {
	switch k {
	case FilePNG, FileImage:
		return 0
	case FileJPEG:
		return 1
	case FileGIF:
		return 2
	case FileWebP:
		return 3
	case FilePDF:
		return 4
	case FileJSON:
		return 10
	case FileCSV:
		return 11
	case FileText:
		return 20
	case FileAny:
		return 30
	}
	return 40 // a kind that is not known is tried last
}

// FileSpec is what makes a Field a file input: what it accepts, how large, what is stored, and how a new file is stored.
type FileSpec struct {
	// Kinds are the kinds accepted. Empty, and FileAny, mean any file.
	Kinds []FileKind

	// Max is the most bytes a file may have.
	Max int64

	// Removable says a visitor may remove the file that is stored, without choosing another.
	Removable bool

	// Stored is the file that the model has, or nil for none.
	Stored func(model any) *StoredFile

	// Store hands a file to the model: a new upload, or nil for a file that is removed. It is nil for a field that is only read.
	Store func(model any, up *Upload)

	// Open is where a new file is written, as it arrives, with the model as it was loaded. It is set whenever Store is.
	Open func(ctx context.Context, model any, in Incoming) (FileWriter, error)
}

// FileWriter is where the bytes of a file go, and what is done with them when the submission is decided.
type FileWriter interface {
	io.WriteCloser

	// Submit makes what was written permanent: the submission was accepted.
	Submit() error

	// Discard removes what was written: the submission was not accepted.
	Discard() error
}

// Incoming is what is known of a file when it begins to arrive.
type Incoming struct {
	Name        string
	ContentType string
	Kind        FileKind
}

// StoredFile is a file that the app holds: what the page says of it.
type StoredFile struct {
	Name string
	Size int64
	URL  string // where the app serves it, "" for no link

	// Thumb is where the app serves a small picture of it, "" for none. It is a path on this site, which is all that a page may show a
	// picture from, and is only ever shown at a few dozen pixels.
	Thumb string
}

// Upload is a file that a visitor sent, and was written to where the app keeps it.
type Upload struct {
	Name        string // the base name that the visitor's browser gave it, made safe
	Size        int64
	ContentType string   // what the file holds, as the library sniffed it, and not what the browser said
	Kind        FileKind // which of the kinds that have a name it was found to be; FileAny for none of them
}

// Files are what a submission carried besides its values.
type Files struct {
	// Uploads are the file that each file field was given, by the field's name. A field that was given none is not in it.
	Uploads map[string]*Upload

	// Refused are the words for each file that was refused as it arrived, by the field's name: it was too large, or not a kind that the
	// field takes, or could not be written.
	Refused map[string]string

	// Removed are the file fields whose stored file the visitor asked to remove, by name.
	Removed map[string]bool
}
