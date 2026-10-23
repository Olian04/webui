package ir

import "io"

// FileUpload is a leaf that is a panel for sending one file, of any size, to the app, which reads it as it arrives. Its model is the options that the
// visitor chooses beside the file, if there are any; what it does is its Receive.
type FileUpload struct {
	At Addr

	Title, Desc string
	Help        string

	// Spec is what the panel takes: its Kinds and its Max. Only the kinds that the first bytes tell are allowed, since the file is read once, by the app.
	Spec *FileSpec

	// Multiple says that script may take several files, and folders, in one go, and sends them one at a time. A post is always one file.
	Multiple bool

	// Receive is the action that does the upload: its Run is handed the file as its subject, which Subject makes, and its Label is the button.
	Receive *Action

	// Options is what the visitor chooses beside the file, as a form that has no submit of its own (its Submit is a placeholder, which is what makes its
	// fields inputs): Fields may be none. Load, which gives the defaults, and Bind are always set.
	Options *Form

	// Subject makes the subject of Receive for a file: what is known of it, the options as they were bound, and the reader that has all of it. The panel is drawn
	// with the zero Incoming (of kind FileAny), the options as loaded and no reader, to ask the Guard whether it is for this visitor.
	Subject func(in Incoming, options any, r io.Reader) any
}

// Incoming is what is known of a file when it begins to arrive: its first bytes have been read, and nothing more.
type Incoming struct {
	Name         string // the base name, made safe
	Path         string // the folder that the file was sent in, made safe, "" for none
	RawName      string // the name exactly as the browser sent it, a path and all
	ContentType  string // what the first bytes hold
	DeclaredType string // the media type that the browser said, "" when it said none
	Kind         FileKind
}

// Addr reports the node's position in the body tree.
func (f *FileUpload) Addr() Addr { return f.At }
