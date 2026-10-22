package runtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
)

// The words of a file that could not be written, and of one that was written and could not be made permanent.
const (
	msgFileNotSaved  = "Could not save this file."
	msgFileNotStored = "Saved, but a file could not be stored."
	copyChunk        = 32 << 10 // how much of a file is read at a time
)

// received is what a submission carried, read as it arrived: its values, and the files that were written, each to the writer that the
// app gave it. It is the one place that holds the writers, so that every one of them is settled, once, however the request ends.
type received struct {
	files   *ir.Files // the uploads and the refused files, nil when there are none
	model   any       // the form's model, as it was loaded when the first file began to arrive
	loaded  bool
	writers []*openFile
	settled bool
}

// openFile is a writer that the app gave, the token that it gave for the file, and whether the writer has been closed.
type openFile struct {
	spec   *ir.FileSpec
	w      io.WriteCloser
	token  any
	closed bool
}

func (f *openFile) close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	return f.w.Close() //nolint:wrapcheck // the app's own error, which is logged as it is
}

func (rec *received) add() *ir.Files {
	if rec.files == nil {
		rec.files = &ir.Files{Uploads: map[string]*ir.Upload{}, Refused: map[string]string{}, Removed: map[string]bool{}}
	}
	return rec.files
}

// seen is whether a file has already come for the field, which is the only one that is taken.
func (rec *received) seen(name string) bool {
	if rec.files == nil {
		return false
	}
	_, uploaded := rec.files.Uploads[name]
	_, refused := rec.files.Refused[name]
	return uploaded || refused
}

// filesFor are the files of the submission for the form's file fields: what arrived, and the stored files that the visitor asked to
// remove. It is nil when there are none, which is the usual.
func (rec *received) filesFor(r *http.Request, fields []ir.Field) *ir.Files {
	out := rec.files
	for f := range ir.Leaves(fields) {
		if f.File != nil && f.File.Store != nil && slices.Contains(r.PostForm["_rm"], f.Name) {
			if out == nil {
				out = rec.add()
			}
			out.Removed[f.Name] = true
		}
	}
	return out
}

// commit makes the files permanent, which is what a submission that was accepted does, with the model as the action saw it. Every file is
// committed, and the errors are the ones that there were. The context is not cancelled with the request.
func (rec *received) commit(ctx context.Context, log *slog.Logger, model any) error {
	if rec.settled {
		return nil
	}
	rec.settled = true
	ctx = context.WithoutCancel(ctx)
	var errs []error
	for _, f := range rec.writers {
		if err := f.close(); err != nil {
			errs = append(errs, err)
			continue
		}
		if f.spec.Commit != nil {
			if err := f.spec.Commit(ctx, model, f.token); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		log.Error("webui: an uploaded file could not be stored", "err", err)
		return err
	}
	return nil
}

// discard removes the files, which is what every end of a request but an accepted submission does, a panic included. It is a no-op
// once the files are settled. The context is not cancelled with the request, which is often why the files are discarded.
func (rec *received) discard(ctx context.Context, log *slog.Logger) {
	if rec == nil || rec.settled {
		return
	}
	rec.settled = true
	ctx = context.WithoutCancel(ctx)
	for _, f := range rec.writers {
		if err := f.close(); err != nil {
			log.Error("webui: an uploaded file could not be closed", "err", err)
		}
		if f.spec.Discard != nil {
			if err := f.spec.Discard(ctx, f.token); err != nil {
				log.Error("webui: an uploaded file could not be discarded", "err", err)
			}
		}
	}
}

// writableFile is the file field of the form that has the name, which a visitor may send a file to.
func writableFile(fields []ir.Field, name string) *ir.FileSpec {
	for f := range ir.Leaves(fields) {
		if f.Name == name && f.File != nil && f.File.Store != nil && f.File.Open != nil {
			return f.File
		}
	}
	return nil
}

// receive reads a multipart body in the order that it arrives, once, with nothing held but the values: each file is written, as it comes,
// to the writer that the app gives for it, and checked as it goes. The leaf that the form is of must come before its files, as the
// markup of the library has it, since the files are the form's. It sets r.PostForm to the values, so that what follows reads them as it does
// those of a body that was not multipart, and answers the request itself when it cannot go on.
func (p *Program) receive(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, leaves map[string]ir.Node, rec *received, valueLimit int64) bool {
	mr, err := r.MultipartReader()
	if err != nil {
		p.refuse(w, r, unreadable(err))
		return false
	}
	values := url.Values{}
	left := valueLimit
	for {
		part, err := mr.NextPart() // which reads what is left of the part before it, so that a part that is not taken is thrown away
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			p.refuse(w, r, unreadable(err))
			return false
		}
		name := part.FormName()
		if part.FileName() == "" {
			b, err := io.ReadAll(io.LimitReader(part, left+1))
			if err != nil {
				p.refuse(w, r, unreadable(err))
				return false
			}
			if int64(len(b)) > left {
				p.refuse(w, r, "That submission was too large.")
				return false
			}
			left -= int64(len(b))
			values.Add(name, string(b))
			continue
		}
		leaf, known := leaves[values.Get("_leaf")]
		if !known {
			p.refuse(w, r, notUnderstood)
			return false
		}
		form, _ := leaf.(*ir.Form)
		if form == nil || rec.seen(name) {
			continue
		}
		spec := writableFile(form.Fields, name)
		if spec == nil {
			continue
		}
		if !p.receiveFile(w, r, page, req, rec, form, spec, name, part) {
			return false
		}
	}
	r.PostForm = values
	if r.Form == nil {
		r.Form = url.Values{}
	}
	for k, v := range values {
		r.Form[k] = append(r.Form[k], v...)
	}
	return true
}

// receiveFile writes one file to the writer that the app gives for it, and checks it as it goes. A file that is refused is a message for
// its field, and the rest of it is thrown away; what stops the request is a body that cannot be read, or a form that cannot be loaded.
func (p *Program) receiveFile(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, rec *received, form *ir.Form, spec *ir.FileSpec, name string, part *multipart.Part) bool {
	head := make([]byte, rules.HeadSize)
	n, err := io.ReadFull(part, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		p.refuse(w, r, unreadable(err))
		return false
	}
	if n == 0 {
		return true // a chooser that was left alone sends an empty part, which is no file
	}
	head = head[:n]
	refused := func(message string) bool {
		rec.add().Refused[name] = message
		return true
	}
	check, message := rules.BeginFile(spec, head)
	if message != "" {
		return refused(message)
	}
	if !rec.loaded {
		ctx := r.Context()
		base, err := timed(p, page, formKind(form), func() (any, error) { return form.Load(ctx) })
		if err != nil {
			check.Abort()
			p.fail(w, r, page, req, err)
			return false
		}
		rec.model, rec.loaded = base, true
	}
	in := ir.Incoming{Name: rules.UploadName(part.FileName()), ContentType: check.Sniffed(), Kind: check.HeadKind()}
	dst, token, err := spec.Open(r.Context(), rec.model, in)
	if err != nil {
		check.Abort()
		p.log.Error("webui: a file could not be opened for writing", "page", page.PathTemplate, "err", err)
		return refused(msgFileNotSaved)
	}
	of := &openFile{spec: spec, w: dst, token: token}
	rec.writers = append(rec.writers, of)

	var size int64
	write := func(b []byte) string {
		if size+int64(len(b)) > spec.Max {
			return rules.FileTooLarge(spec.Max)
		}
		size += int64(len(b))
		_, _ = check.Writer().Write(b) // which never fails
		if _, err := of.w.Write(b); err != nil {
			p.log.Error("webui: a file could not be written", "page", page.PathTemplate, "err", err)
			return msgFileNotSaved
		}
		return ""
	}
	refusal := write(head)
	buf := make([]byte, copyChunk)
	for refusal == "" {
		n, err := part.Read(buf)
		if n > 0 {
			refusal = write(buf[:n])
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			check.Abort()
			_ = of.close() // the request is over, and the file is discarded with it
			p.refuse(w, r, unreadable(err))
			return false
		}
	}
	if refusal != "" {
		check.Abort()
		_ = of.close()
		return refused(refusal)
	}
	contentType, kind, message := check.Finish(spec)
	if err := of.close(); err != nil {
		p.log.Error("webui: a file could not be closed", "page", page.PathTemplate, "err", err)
		return refused(msgFileNotSaved)
	}
	if message != "" {
		return refused(message)
	}
	rec.add().Uploads[name] = &ir.Upload{Name: in.Name, Size: size, ContentType: contentType, Kind: kind}
	return true
}
