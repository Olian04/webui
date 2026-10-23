package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
	"github.com/Olian04/webui/internal/rules"
)

// fileUpload builds a file upload panel's view. Its options are loaded, to be shown as the defaults of its inputs, and its Guard is asked with a file that has
// not arrived and those options, which decides how the panel is drawn for this visitor; the same Guard, asked with the file, refuses the POST. A load or a source
// of options that fails fails the panel, which says so, and not the page.
func (p *Program) fileUpload(ctx context.Context, req *Request, page *ir.Page, n *ir.FileUpload) templ.Component {
	view := render.FileUploadView{Node: n, Page: page}
	view.Path, view.Query = req.encode(page)
	resolved, err := resolve(p, page, n.Options, func() (*ir.Form, error) { return n.Options.Resolved(ctx) })
	if err != nil {
		if ctx.Err() == nil {
			p.log.Error("webui: the options of a file upload could not be read", "page", page.PathTemplate, "leaf", render.LeafID(n.At), "err", err)
		}
		view.Failed = true
		return p.render.FileUpload(view)
	}
	model, ok, refusal := loaded(p, ctx, page, "upload", "file upload", n.At, func() (any, error) { return n.Options.Load(ctx) })
	if !ok {
		view.Failed, view.Refusal = true, refusal
		return p.render.FileUpload(view)
	}
	view.Form = render.FormView{Node: resolved, Page: page, Path: view.Path, Query: view.Query, Model: model}
	if n.Receive.Guard != nil {
		view.Gate = p.gateOf(page, "the Receive of a file upload", func() error {
			return n.Receive.Guard(ctx, n.Subject(ir.Incoming{Kind: ir.FileAny}, model, nil))
		})
		view.Form.SubmitRefused = view.Gate.Reason != "" // a panel that is disabled shows its options and takes none
	}
	return p.render.FileUpload(view)
}

// errFileTooLarge is what a Read of a file that is larger than the panel takes fails with. The library answers "Must be at most …" in its place,
// whatever the action made of it.
var errFileTooLarge = errors.New("webui: the file is larger than the panel takes")

// capped is the file as the action reads it: all of it from its first byte, and no more than max, past which a Read fails.
type capped struct {
	r        io.Reader
	left     int64
	exceeded bool
}

func (c *capped) Read(b []byte) (int, error) {
	if c.exceeded {
		return 0, errFileTooLarge
	}
	if int64(len(b)) > c.left+1 {
		b = b[:c.left+1] // one more than it may have, to know that there is more
	}
	n, err := c.r.Read(b)
	if int64(n) > c.left {
		n, c.left, c.exceeded = int(c.left), 0, true
		return n, errFileTooLarge
	}
	c.left -= int64(n)
	return n, err //nolint:wrapcheck // the body's own error, which the action sees as it is
}

// maxRawName is the most of the name that a browser sent that is kept as it was sent.
const maxRawName = 4096

// rawFileName is the name that the browser gave the file, as it sent it, with its path: a part's FileName is the base name only, which is
// what a folder that the visitor chose would lose.
func rawFileName(part *multipart.Part) string {
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		return ""
	}
	name := params["filename"]
	if len(name) > maxRawName {
		name = name[:maxRawName]
	}
	return name
}

// declaredType is the media type that the browser said the file has, without its parameters, or "" when it said none or something that is not one.
func declaredType(part *multipart.Part) string {
	mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return mediaType
}

// uploadAct hands a file to a FileUpload's action as it arrives. Its first bytes are read, to tell what it is, and then it is the action's
// subject: the Guard, which refuses a visitor before any more is read, and the Run, which reads the rest, and answers with an outcome as any
// action does. What the action leaves unread is thrown away, to the limit of the body, so that the browser is answered and not reset.
func (p *Program) uploadAct(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.FileUpload, part *multipart.Part, posted url.Values) {
	// What the visitor chose beside the file came before it, and is checked by the rules of its fields before any of the file is read: a choice that is
	// refused refuses the upload, as the messages of a table action's refusal, and the file is not read.
	ctx := r.Context()
	base, err := timed(p, page, "upload", func() (any, error) { return n.Options.Load(ctx) })
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("file upload options load: %w", err))
		return
	}
	resolved, err := resolve(p, page, n.Options, func() (*ir.Form, error) { return n.Options.Resolved(ctx) })
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("file upload options: %w", err))
		return
	}
	options, errs := n.Options.Bind(resolved.Fields, base, formValues(posted, resolved.Fields), nil)
	if len(errs) > 0 {
		p.m.Action(page.PathTemplate, "invalid")
		messages := make([]string, len(errs))
		for i, e := range errs {
			messages[i] = e.Label + ": " + e.Message
		}
		p.refuse(w, r, strings.Join(messages, "; "))
		return
	}
	head := make([]byte, rules.HeadSize)
	got, err := io.ReadFull(part, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		p.refuse(w, r, unreadable(err))
		return
	}
	if got == 0 {
		p.refuse(w, r, rules.FileRequired()) // a chooser that was left alone sends an empty part
		return
	}
	head = head[:got]
	sniffed, kind, wrong := rules.FileHead(n.Spec, head)
	if wrong != "" {
		kind = ir.FileAny
	}
	raw := rawFileName(part)
	in := ir.Incoming{
		Name: rules.UploadName(raw), Path: rules.UploadPath(raw), RawName: raw, ContentType: sniffed, DeclaredType: declaredType(part), Kind: kind,
	}
	file := &capped{r: io.MultiReader(bytes.NewReader(head), part), left: n.Spec.Max}

	action := *n.Receive
	run := action.Run
	action.Run = func(ctx context.Context, subject any) (ir.Outcome, error) {
		var outcome ir.Outcome
		var err error
		if wrong != "" {
			outcome = ir.Outcome{Kind: ir.OutcomeFailure, Message: wrong}
		} else {
			outcome, err = run(ctx, subject)
		}
		// What the action did not read is read here: to know whether the file is larger than the panel takes, and to throw the rest of it away.
		_, _ = io.Copy(io.Discard, file)
		_, _ = io.Copy(io.Discard, part)
		if file.exceeded {
			return ir.Outcome{Kind: ir.OutcomeFailure, Message: rules.FileTooLarge(n.Spec.Max)}, nil
		}
		return outcome, err
	}
	outcome, ok := p.perform(w, r, page, req, &action, n.Subject(in, options, file), "upload")
	if !ok {
		return
	}
	switch outcome.Kind {
	case ir.OutcomeFailure, ir.OutcomeReject:
		// An upload has no fields to point at, so the messages are the reason.
		p.refuse(w, r, outcomeText(outcome))
	case ir.OutcomeSuccess, ir.OutcomeWarning:
		p.leave(w, r, outcome, r.URL.RequestURI())
	}
}
