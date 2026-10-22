package runtime

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
	"github.com/Olian04/webui/internal/rules"
)

// maxBody bounds a submission. An admin form is a few fields; a megabyte is
// generous, and an unbounded body is a memory problem for whoever reaches it.
const maxBody = 1 << 20

// maxEditorBody bounds a submission to a page that has an editor, whose text is the
// body and may be large. A Content.Rules.MaxLen is how an application bounds it
// further.
const maxEditorBody = 16 << 20

// leavesOf maps each leaf's id to its node, so a POST can name one and nothing
// else: the id comes from position, never from anything the user typed.
func leavesOf(root ir.Node) map[string]ir.Node {
	out := map[string]ir.Node{}
	ir.Walk(root, func(n ir.Node) {
		switch n.(type) {
		case *ir.Form, *ir.Table, *ir.Diff, *ir.Markdown, *ir.Toolbar, *ir.FileUpload:
			out[render.LeafID(n.Addr())] = n
		}
	})
	return out
}

// tabsOf maps the id of each Tabs to its node, so that a request can ask for one alone, the strip and the tab that is
// selected, which is how a tab is followed with script. The id is in the form of a leaf's, from position and nothing the
// user typed.
func tabsOf(root ir.Node) map[string]*ir.Tabs {
	out := map[string]*ir.Tabs{}
	ir.Walk(root, func(n ir.Node) {
		if t, ok := n.(*ir.Tabs); ok {
			out[render.LeafID(t.Addr())] = t
		}
	})
	return out
}

// actionable reports whether any leaf under root posts.
func actionable(leaves map[string]ir.Node) bool {
	for _, n := range leaves {
		switch n := n.(type) {
		case *ir.Form:
			if n.Submit != nil {
				return true
			}
		case *ir.Table:
			if len(n.Actions)+len(n.Bulk) > 0 || n.RowAction != nil {
				return true
			}
		case *ir.Toolbar:
			if slices.ContainsFunc(n.Items, func(it ir.ToolbarItem) bool { return it.Action != nil }) {
				return true
			}
		case *ir.FileUpload:
			return true
		}
	}
	return false
}

// postHandler is the write pipeline of one page. The order is the contract and
// it is the README's: parse → rules → Bind → Guard → Run. The page Guard runs
// first, as it does on every request, so a POST cannot reach an action on a
// page the viewer may not open.
func (p *Program) postHandler(page *ir.Page, leaves map[string]ir.Node) http.HandlerFunc {
	parser := argParser(page)
	budget := fileBudget(leaves)
	return func(w http.ResponseWriter, r *http.Request) {
		if err := p.cross.Check(r); err != nil {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		limit := int64(maxBody)
		if hasEditor(leaves) {
			limit = maxEditorBody
		}
		valueLimit := limit // what the values of a multipart body may come to, which is what a body that is not multipart may
		// A page with a file field takes a multipart body of as much as its files may be, besides the usual: which is the
		// limit of that kind of body, and of no other that is posted to it.
		multipart := budget > 0 && strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
		if multipart {
			limit = addCapped(budget, maxBody)
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		if err := r.ParseForm(); err != nil { // of a multipart body, only the address
			p.refuse(w, r, unreadable(err))
			return
		}

		req, ok := p.begin(w, r, page, parser)
		if !ok {
			return
		}
		// The page's Guard has let the visitor in, so it is only now that what they sent is read: a visitor that the page
		// refuses has nothing of theirs received.
		rec := &received{}
		if multipart && !p.receive(w, r, page, req, leaves, rec, valueLimit) {
			return
		}
		switch n := leaves[r.PostForm.Get("_leaf")].(type) {
		case *ir.Form:
			p.submit(w, r, page, req, n, rec)
		case *ir.Table:
			p.tableAct(w, r, page, req, n)
		case *ir.Toolbar:
			p.toolbarAct(w, r, page, req, n)
		case *ir.FileUpload:
			// A file is received as it arrives, while the body is read, and answers the request itself: this is a post that sent none.
			p.refuse(w, r, rules.FileRequired())
		default:
			p.refuse(w, r, notUnderstood)
		}
	}
}

// unreadable is what to say of a body that could not be read: that it was too large, or that it was not understood.
func unreadable(err error) string {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "too large") {
		return "That submission was too large."
	}
	return "That submission could not be read."
}

// fileBudget is the most that the files of a page may be, together, which is 0 for a page that has none.
func fileBudget(leaves map[string]ir.Node) int64 {
	var total int64
	for _, n := range leaves {
		switch n := n.(type) {
		case *ir.Form:
			for f := range ir.Leaves(n.Fields) {
				if f.File != nil && f.File.Store != nil {
					total = addCapped(total, f.File.Max)
				}
			}
		case *ir.FileUpload:
			total = addCapped(total, n.Spec.Max)
		}
	}
	return total
}

// addCapped is a + b, or the largest size there is when that would overflow: an app that says a file may be as large as it likes, for more than one
// file, has a limit that is as large as it likes, and not a negative one.
func addCapped(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

// notUnderstood is what a POST that names no leaf, or no action, that the page has is answered with.
const notUnderstood = "That request was not understood."

// submit binds and runs a form's Submit.
func (p *Program) submit(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Form, rec *received) {
	ctx := r.Context()
	if n.Submit == nil {
		p.refuse(w, r, notUnderstood)
		return
	}

	base, err := timed(p, page, formKind(n), func() (any, error) { return n.Load(ctx) })
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("form load: %w", err))
		return
	}
	values := map[string]string{}
	selects := selectNames(n.Fields)
	for k, v := range r.PostForm {
		switch {
		case strings.HasPrefix(k, "_") || len(v) == 0:
		case selects[k]:
			values[k] = strings.Join(v, ir.ListSep) // a select sends one value for each chosen option
		default:
			values[k] = v[0]
		}
	}

	resolved, err := resolve(p, page, n, func() (*ir.Form, error) { return n.Resolved(ctx) }) // what its selects' values are checked against
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("form options: %w", err))
		return
	}
	model, errs := n.Bind(resolved.Fields, base, values, rec.filesFor(r, resolved.Fields))
	if len(errs) > 0 {
		p.m.Action(page.PathTemplate, "invalid") // the rules refused it before anything ran
		p.returnToForm(w, r, page, req, n, model, values, errs, fieldsToast(len(errs)))
		return
	}
	outcome, ok := p.perform(w, r, page, req, n.Submit, model, "submit")
	if !ok {
		return
	}
	switch outcome.Kind {
	case ir.OutcomeFailure:
		// Not done, and not for one field: show the form again with what was typed,
		// as a rejection does, with the reason for the whole form in a toast.
		p.returnToForm(w, r, page, req, n, model, values, nil, render.Toast{Title: outcomeText(outcome), Tone: render.ToastError})
	case ir.OutcomeReject:
		// Understood and refused, and the user can fix it: the same event, from
		// their side, as a rule the browser caught.
		p.returnToForm(w, r, page, req, n, model, values, outcome.Fields, fieldsToast(len(outcome.Fields)))
	case ir.OutcomeSuccess, ir.OutcomeWarning:
		// An accepted action that said nowhere to go returns to where the form was opened from, or
		// stays where it is (Stay: the form again, and not where it was opened from). The toast rides
		// a flash cookie and the browser is sent with a 303, so a reload does not repeat the POST.
		target := cmp.Or(req.From, r.URL.RequestURI())
		if outcome.Return == ir.ReturnStay {
			target = r.URL.RequestURI()
		}
		p.leave(w, r, outcome, target)
	}
}

// perform runs an action on its subject: its Guard, which refuses with a 403, and its Run, which
// fails with an error page, each counted in the metrics. An outcome that was told where to go, or
// to go Back, leaves: Back for where the user came from, even for an outcome that would have shown
// the form again, in which case what was typed is not kept and the message is in the toast it
// arrives to. It returns the outcome when it is for the caller to act on, and false when the
// response has been written. what names the action in the error that is logged.
func (p *Program) perform(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, act *ir.Action, subject any, what string) (ir.Outcome, bool) {
	ctx := r.Context()
	if act.Guard != nil {
		if err := act.Guard(ctx, subject); err != nil {
			p.m.Action(page.PathTemplate, "denied")
			p.refuse(w, r, cmp.Or(err.Error(), "Not allowed."))
			return ir.Outcome{}, false
		}
	}
	outcome, err := act.Run(ctx, subject)
	if err != nil {
		p.m.Action(page.PathTemplate, "error")
		p.fail(w, r, page, req, fmt.Errorf("%s: %w", what, err))
		return ir.Outcome{}, false
	}
	p.m.Action(page.PathTemplate, resultOf(outcome))
	switch {
	case outcome.Redirect != "":
		p.leave(w, r, outcome, outcome.Redirect) // told where to go, which replaces whatever it would have done
	case outcome.Return == ir.ReturnBack:
		p.leave(w, r, outcome, p.returnTarget(r, req, page))
	default:
		return outcome, true
	}
	return ir.Outcome{}, false
}

// fieldsToast is the toast of a submission that the rules or the action refused for n fields.
func fieldsToast(n int) render.Toast {
	return render.Toast{Title: "Not saved", Desc: plural(n, "field needs", "fields need") + " attention.", Tone: render.ToastError}
}

// selectNames are the names of the form's controls that are selects, which are the ones that may be
// sent more than once.
func selectNames(fields []ir.Field) map[string]bool {
	out := map[string]bool{}
	for f := range ir.Leaves(fields) {
		if f.Display == ir.DisplaySelect {
			out[f.Name] = true
		}
	}
	return out
}

// returnTarget is where an outcome that says Back leaves for: where the form was opened
// from, else the breadcrumb parent, else where the user already is.
func (p *Program) returnTarget(r *http.Request, req *Request, page *ir.Page) string {
	path, _ := req.encode(page)
	return cmp.Or(p.returnHref(req, page, path), r.URL.RequestURI())
}

// returnToForm answers a refused submission with the page, its form drawn again with what the user typed and what
// was refused, and the toast, with the status 422 (Unprocessable Content): the request was understood and the content
// refused. It is not redirected, as an accepted action is. A redirect exists so that a reload does not do again what
// was done, and a refusal did nothing, so the browser's own question when the page is reloaded ("send the form
// again?") is the right one, and the answer holds nothing on the server and nothing in a cookie. errs is empty for a
// failure that is not about any field.
func (p *Program) returnToForm(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Form,
	model any, values map[string]string, errs []ir.FieldError, toast render.Toast) {
	ctx := r.Context()
	req.sub, req.subLeaf = &submission{model: model, values: values, errs: errs}, render.LeafID(n.At)
	path, query := p.address(page, req)
	req.shell = p.startShell(ctx, render.Here{Page: page.PathTemplate, Path: path, Query: query}, p.render.Active(page))
	req.panels = &panelSet{}
	body := p.panel(ctx, req, page, page.Body)
	if err := req.panels.wait(&req.code); err != nil {
		p.fail(w, r, page, req, err)
		return
	}
	p.writePage(w, r, http.StatusUnprocessableEntity, page, req, body, toast)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// leave sends the user to target with the outcome's message as a toast on the
// page they arrive at. It is how an accepted action ends, and how any outcome that
// was told where to go does.
func (p *Program) leave(w http.ResponseWriter, r *http.Request, outcome ir.Outcome, target string) {
	p.flashRedirect(w, r, target, outcomeText(outcome), outcomeTone(outcome))
}

// flashRedirect sends the user to target with a 303, so that a reload does not repeat the POST, and
// text as a toast on the page they arrive at, which rides a flash cookie.
func (p *Program) flashRedirect(w http.ResponseWriter, r *http.Request, target, text string, tone render.ToastTone) {
	p.setFlash(w, r, text, tone)
	//nolint:gosec // G710: target is this request's own path, an address Open resolved, or one local accepted.
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// outcomeText is what an outcome tells the user once it has left the form: its
// message, or for a rejection the messages of its fields, since there are no
// fields on the page it arrives at. A refusal always says something.
func outcomeText(o ir.Outcome) string {
	switch o.Kind {
	case ir.OutcomeReject:
		messages := make([]string, 0, len(o.Fields))
		for _, f := range o.Fields {
			messages = append(messages, f.Message)
		}
		if len(messages) == 0 {
			return "Not saved"
		}
		return strings.Join(messages, "; ")
	case ir.OutcomeFailure:
		if o.Message == "" {
			return "Not saved"
		}
	case ir.OutcomeSuccess, ir.OutcomeWarning:
	}
	return o.Message
}

func outcomeTone(o ir.Outcome) render.ToastTone {
	switch o.Kind {
	case ir.OutcomeWarning:
		return render.ToastWarning
	case ir.OutcomeFailure, ir.OutcomeReject:
		return render.ToastError
	case ir.OutcomeSuccess:
	}
	return render.ToastOK
}

// safeRedirect accepts only a path on this host. Open produces these, and an
// address that came in with the request, such as the one a form was opened from,
// must be one before it is followed.
func safeRedirect(target string) bool {
	return strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//") && !strings.HasPrefix(target, `/\`)
}

// tableAct runs a row action or a bulk action. Rows are named by Key and
// re-loaded, never trusted by position: the table may have changed since the
// user saw it, and acting on a different row than the one they clicked is the
// failure this avoids.
func (p *Program) tableAct(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Table) {
	ctx := r.Context()
	kind, index, key, ok := parseAct(r.PostForm.Get("_act"))
	var action *ir.Action
	switch {
	case ok && kind == "row" && index < len(n.Actions):
		action = n.Actions[index]
	case ok && kind == "click" && n.RowAction != nil:
		action, kind = n.RowAction, "row" // a click on a row is the action on that row
	case ok && kind == "bulk" && index < len(n.Bulk):
		action = n.Bulk[index]
	default:
		p.refuse(w, r, notUnderstood)
		return
	}

	resolved, err := resolve(p, page, n, func() (*ir.Table, error) { return n.Resolved(ctx) }) // a filter by a select is held to the options it has now
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("table options: %w", err))
		return
	}
	window, err := timed(p, page, "table", func() (ir.Window, error) { return n.Load(ctx, queryOf(resolved, req.Raw)) })
	rows := window.Rows
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("table load: %w", err))
		return
	}
	byKey := make(map[string]any, len(rows))
	for _, row := range rows {
		byKey[n.Key(row)] = row
	}

	var subject any
	if kind == "row" {
		row, found := byKey[key]
		if !found {
			p.refuse(w, r, "That row no longer exists.")
			return
		}
		subject = row
	} else {
		want := map[string]bool{}
		for _, k := range r.PostForm["_sel"] {
			want[k] = true
		}
		if len(want) == 0 {
			p.refuse(w, r, "Nothing is selected.")
			return
		}
		var picked []any
		for _, row := range rows { // in table order, whatever order they arrived in
			if want[n.Key(row)] {
				picked = append(picked, row)
			}
		}
		if len(picked) != len(want) {
			p.refuse(w, r, "Some selected rows no longer exist.")
			return
		}
		subject = picked
	}

	outcome, ok := p.perform(w, r, page, req, action, subject, fmt.Sprintf("action %q", action.Label))
	if !ok {
		return
	}
	switch outcome.Kind {
	case ir.OutcomeFailure, ir.OutcomeReject:
		// A table action has no fields to point at, so the messages are the reason.
		p.refuse(w, r, outcomeText(outcome))
	case ir.OutcomeSuccess, ir.OutcomeWarning:
		p.leave(w, r, outcome, r.URL.RequestURI())
	}
}

// refuse sends the user back with an error toast: nothing was changed.
func (p *Program) refuse(w http.ResponseWriter, r *http.Request, message string) {
	p.flashRedirect(w, r, r.URL.RequestURI(), message, render.ToastError)
}

// again sends the user to the address they are at, with a 303: a POST is never answered with a page, so that
// a reload sends nothing again, and the page that answers is the same one that a visit would.
func (p *Program) again(w http.ResponseWriter, r *http.Request) {
	//nolint:gosec // G710: the request's own path and query, which the router has matched to a page.
	http.Redirect(w, r, r.URL.RequestURI(), http.StatusSeeOther)
}

// parseAct reads an "_act" value: "bulk:2" or "row:0:<key>". The key may
// itself contain colons.
func parseAct(s string) (kind string, index int, key string, ok bool) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return "", 0, "", false
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 {
		return "", 0, "", false
	}
	switch {
	case parts[0] == "bulk" && len(parts) == 2:
		return "bulk", index, "", true
	case parts[0] == "row" && len(parts) == 3:
		return "row", index, parts[2], true
	case parts[0] == "click" && len(parts) == 3:
		return "click", index, parts[2], true
	}
	return "", 0, "", false
}

// hasEditor reports whether any leaf is a code editor.
func hasEditor(leaves map[string]ir.Node) bool {
	for _, n := range leaves {
		if f, ok := n.(*ir.Form); ok && f.Editor != nil {
			return true
		}
	}
	return false
}

// resultOf is what an outcome is called in the metrics.
func resultOf(o ir.Outcome) string {
	switch o.Kind {
	case ir.OutcomeFailure:
		return "failure"
	case ir.OutcomeReject:
		return "reject"
	case ir.OutcomeWarning:
		return "warning"
	case ir.OutcomeSuccess:
	}
	return "success"
}
