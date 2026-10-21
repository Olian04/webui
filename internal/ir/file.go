package ir

import "io"

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
}

// StoredFile is a file that the app holds: what the page says of it.
type StoredFile struct {
	Name string
	Size int64
	URL  string // where the app serves it, "" for no link
}

// Upload is a file that a visitor sent, which is held on the server, in memory or in a file of its own, until the request is answered.
type Upload struct {
	Name        string // the base name that the visitor's browser gave it, made safe
	Size        int64
	ContentType string // what the file holds, as the library sniffed it, and not what the browser said
	Open        func() (io.ReadCloser, error)
}

// Files are what a submission carried besides its values.
type Files struct {
	// Uploads are the file that each file field was given, by the field's name. A field that was given none is not in it.
	Uploads map[string]*Upload

	// Removed are the file fields whose stored file the visitor asked to remove, by name.
	Removed map[string]bool
}
