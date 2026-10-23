package webui

import (
	"fmt"
	"io"

	"github.com/Olian04/webui/internal/ir"
)

// FileUpload is a leaf that is a panel for sending one file, of any size, to the app, which reads it as it arrives. It is for a file that is too
// large to be held in memory, a firmware image or a backup on its way to an object store; a file that can be held is a [File] in a [Form], which
// is part of a submission and is checked with the rest of it.
//
//	webui.FileUpload{
//		Title: "Upload firmware", Accept: []webui.FileKind{webui.FileAny}, MaxSize: 2 << 30,
//		Receive: webui.Action[webui.FileHandle]{
//			Label: "Upload",
//			Guard: canEdit,
//			Run: func(ctx context.Context, f webui.FileHandle) (webui.Outcome, error) {
//				_, err := bucket.PutObject(ctx, "firmware/"+f.Name, f.Reader) // streamed: nothing is held, in memory or on disk
//				if err != nil {
//					return webui.Outcome{}, err
//				}
//				return webui.Success("Uploaded " + f.Name), nil
//			},
//		},
//	}
//
// A FileUpload has no model and loads nothing, so it is not generic: what Receive needs of the page it is on, it reads with [ArgsOf]. Receive is an
// [Action] whose subject is the [FileHandle], the file that is arriving, and its Run is the upload: it reads the file from
// the handle's Reader, wherever it is to go, and says how it ended with an [Outcome], as any action does. The file is not held by the library, in
// memory or on disk: Run reads it as it comes, and a Run that returns before the end of it is not an error, since what is left is thrown away.
//
// A FileUpload is a native form of its own, with a file chooser and a button, so it works without script, and with script the chooser is the same
// frame that a file can be dropped on that a [File] has, with the size and kind said before the request is sent. It sends one file, in one request,
// and has no other fields: what the app wants to know of it besides the file is a second step, a [Form] on the page that it leads to, or the page's arguments.
//
// The file is checked as it arrives: its first 512 bytes tell its kind, which must be one of Accept's, and a file that is not is refused without
// Run being called; and the Reader is held to MaxSize, so that a file that is larger is refused, with "Must be at most …", and not stored whole by a Run
// that did not ask. Run reads to the end of the Reader, which is how it knows that the file is whole. Only the kinds that the first bytes tell
// can be taken (the images, [FilePDF] and [FileAny]), since the file is read once, by Run, and not again by the library to be checked; JSON, CSV and text
// are told by all of the file, and are a [File]'s.
//
// Receive's Guard is asked twice. Drawing the panel, it is given a FileHandle of kind [FileAny] with no Reader: [Disabled] draws the chooser and the
// button disabled, with its message, and any other error leaves the panel out, as for a button. When the file arrives it is given the file, after
// the first 512 bytes have been read (so that it can see the kind) and before anything more, so a visitor that it refuses has nothing more read from
// them, and a browser that was sending a large file may see the connection closed for it. A page's own Guard runs first, and refuses the page.
type FileUpload struct {
	// Title is the panel's heading.
	Title string

	// Desc is a description, shown in a popover from an information icon beside the title.
	Desc string

	// Help is a line or two that tells the visitor what the file is for, where the label and the rules cannot: it is in the popover
	// beside the label that lists the rules, above them. It is text, not markup.
	Help string

	// Accept lists the kinds of file that the panel takes. Left empty it takes any file. Only the kinds that the first bytes tell are allowed: the
	// images, [FilePDF] and [FileAny].
	Accept []FileKind

	// MaxSize is the most bytes that the file may have. It has no default, since how large is the app's to say and there is no size that is right for a
	// panel that is made for a large file: a MaxSize of 0, or less, is a compile error.
	MaxSize int64

	// Receive is the upload: the action whose Run is handed the file, and whose Label is the button, "Upload" unless it says. Its Guard decides
	// whether the visitor may, and how the panel is drawn (see above); Confirm asks the visitor before the file is sent.
	Receive Action[FileHandle]
}

// FileHandle is the file that is arriving, and the subject of a [FileUpload]'s Receive.
type FileHandle struct {
	// Name is what the visitor's file was called, as a base name: never a path, with the control characters taken out and at most 255
	// characters. It is the visitor's, so it is not to be used as a name on disk.
	Name string

	// Path is the folder that the file was sent in when the visitor chose a folder to send ("photos/2024" for "photos/2024/a.jpg"), made safe: both kinds of slash separate
	// it, empty elements and "." and ".." are dropped (a ".." is not followed, so "a/../b" is "a/b"), control characters are taken out, an element has at most 255
	// characters and there are at most 32. It never has a slash at either end, and is "" for a file that was not sent in a folder. It is the visitor's, and is for the app
	// to use as the folder of the object it makes, never to open on a disk as it is.
	Path string

	// RawName is the file's name exactly as the browser sent it, with its path and anything that Name has taken out of it. It is the visitor's and not safe, and is not to be
	// used as a name: it is for telling the visitor that their file's name was changed, by comparing it with Name and Path.
	RawName string

	// ContentType is what the first 512 bytes of the file hold, as a media type, and not what the browser said it was.
	ContentType string

	// DeclaredType is the media type that the browser said the file has (from its extension, usually), without its parameters, or "" when it said none. It is the visitor's, and
	// is what to prefer for a name that the visitor chose, for a stored object's type, where the first 512 bytes say "text/plain" of a JSON file: it is not what the file holds.
	DeclaredType string

	// Kind is the kind that the first bytes say the file is: [FilePNG], [FileJPEG], [FileGIF], [FileWebP] or [FilePDF], and [FileAny] for any
	// other file, which only a panel that takes any file lets in.
	Kind FileKind

	// Reader reads the file from its first byte to its last, and fails past the panel's MaxSize. It is nil when the panel is being drawn, to ask
	// the Guard whether it is for the visitor, and must not be read by a Guard.
	Reader io.Reader
}

// ASSERT: FileUpload implements PageBody
var _ PageBody = FileUpload{}

// headKind is whether a kind is told by the first bytes of a file alone, which is all that a panel that reads the file once can take.
func headKind(k FileKind) bool {
	switch k {
	case FilePNG, FileJPEG, FileGIF, FileWebP, FileImage, FilePDF, FileAny:
		return true
	case FileJSON, FileCSV, FileText:
	}
	return false
}

func (f FileUpload) validateBody(v *bodyValidator) {
	validateAction(v, "FileUpload.Receive", f.Receive, false)
	if f.MaxSize <= 0 {
		v.add(fmt.Sprintf("FileUpload has MaxSize %d", f.MaxSize), "Set MaxSize to how many bytes the file may have: there is no default for a panel that is made for a large file.")
	}
	for i, k := range f.Accept {
		switch {
		case int(k) > int(FileAny):
			v.add(fmt.Sprintf("FileUpload.Accept[%d] is not a FileKind", i), "Use one of the File… constants.")
		case !headKind(k):
			v.add(fmt.Sprintf("FileUpload.Accept[%d] is a kind that is told by all of the file", i),
				"A FileUpload reads the file once, and only the images, FilePDF and FileAny are told by its first bytes: a JSON, CSV or text file is a File in a Form.")
		}
	}
}

func (f FileUpload) lowerBody(at ir.Addr) ir.Node {
	spec := &ir.FileSpec{Max: f.MaxSize}
	for _, k := range f.Accept {
		spec.Kinds = append(spec.Kinds, k.ir())
	}
	a := f.Receive
	if a.Label == "" {
		a.Label = "Upload"
	}
	if a.Role == RoleSecondary {
		a.Role = RolePrimary // the one primary action of the panel
	}
	return &ir.FileUpload{
		At: at, Title: f.Title, Desc: f.Desc, Help: f.Help, Spec: spec,
		Receive: lowerAction(a),
		Subject: func(in ir.Incoming, r io.Reader) any {
			return FileHandle{
				Name: in.Name, Path: in.Path, RawName: in.RawName, ContentType: in.ContentType, DeclaredType: in.DeclaredType,
				Kind: fileKindOf(in.Kind), Reader: r,
			}
		},
	}
}
