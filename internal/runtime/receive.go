package runtime

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/rules"
)

// received is what a multipart submission carried, read as it arrived: the files that were chosen, held in memory, or the words for each that was
// refused.
type received struct {
	files *ir.Files // the uploads and the refused files, nil when there are none
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

// writableFile is the file field of the form that has the name, which a visitor may send a file to.
func writableFile(fields []ir.Field, name string) *ir.FileSpec {
	for f := range ir.Leaves(fields) {
		if f.Name == name && f.File != nil && f.File.Store != nil {
			return f.File
		}
	}
	return nil
}

// uploadField is the name of the file input of a FileUpload.
const uploadField = "file"

// receive reads a multipart body in the order that it arrives, once, holding only the values and the files that the form takes: a file is read into
// memory, as far as the field lets it be, and not into the temporary file that net/http's ParseMultipartForm would write what is more than a
// megabyte to, so that nothing of a submission is written to disk. The leaf that the form is of must come before its files, as the markup of the
// library has it, since the files are the form's. It sets r.PostForm to the values, so that what follows reads them as it does those of a
// body that was not multipart, and answers the request itself when it cannot go on.
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
		if upload, ok := leaf.(*ir.FileUpload); ok {
			// A panel that is for one file, of any size: the file is not held, and is handed to the action as it arrives, which answers the request.
			if name == uploadField {
				p.uploadAct(w, r, page, req, upload, part, values)
				return false
			}
			continue
		}
		form, _ := leaf.(*ir.Form)
		if form == nil || rec.seen(name) {
			continue
		}
		spec := writableFile(form.Fields, name)
		if spec == nil {
			continue
		}
		if !p.receiveFile(w, r, rec, spec, name, part) {
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

// receiveFile reads one file into memory and checks it. A file that is refused is a message for its field, and the rest of it is thrown away
// (by reading the next part); what stops the request is a body that cannot be read.
func (p *Program) receiveFile(w http.ResponseWriter, r *http.Request, rec *received, spec *ir.FileSpec, name string, part *multipart.Part) bool {
	refused := func(message string) bool {
		rec.add().Refused[name] = message
		return true
	}
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
	if _, _, message := rules.FileHead(spec, head); message != "" {
		return refused(message) // before the rest of it is held
	}
	var data bytes.Buffer
	data.Write(head)
	rest, err := io.ReadAll(io.LimitReader(part, spec.Max-int64(n)+1))
	if err != nil {
		p.refuse(w, r, unreadable(err))
		return false
	}
	data.Write(rest)
	if int64(data.Len()) > spec.Max {
		return refused(rules.FileTooLarge(spec.Max))
	}
	contentType, kind, message := rules.CheckFile(spec, data.Bytes())
	if message != "" {
		return refused(message)
	}
	rec.add().Uploads[name] = &ir.Upload{
		Name: rules.UploadName(part.FileName()), Size: int64(data.Len()), ContentType: contentType, Kind: kind, Data: data.Bytes(),
	}
	return true
}
