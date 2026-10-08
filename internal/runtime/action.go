package runtime

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render"
)

// maxBody bounds a submission. An admin form is a few fields; a megabyte is
// generous, and an unbounded body is a memory problem for whoever reaches it.
const maxBody = 1 << 20

// leavesOf maps each leaf's id to its node, so a POST can name one and nothing
// else: the id comes from position, never from anything the user typed.
func leavesOf(root ir.Node) map[string]ir.Node {
	out := map[string]ir.Node{}
	var walk func(ir.Node)
	walk = func(n ir.Node) {
		switch n := n.(type) {
		case *ir.Stack:
			for _, c := range n.Children {
				walk(c)
			}
		case *ir.Split:
			for _, c := range n.Children {
				walk(c)
			}
		case *ir.Tabs:
			for _, t := range n.Tabs {
				walk(t.Body)
			}
		case *ir.Form:
			out[render.LeafID(n.At)] = n
		case *ir.Table:
			out[render.LeafID(n.At)] = n
		}
	}
	if root != nil {
		walk(root)
	}
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
			if len(n.Actions)+len(n.Bulk) > 0 {
				return true
			}
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
	return func(w http.ResponseWriter, r *http.Request) {
		if err := p.cross.Check(r); err != nil {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		if err := r.ParseForm(); err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "too large") {
				status = http.StatusRequestEntityTooLarge
			}
			http.Error(w, http.StatusText(status), status)
			return
		}

		req, ok := p.begin(w, r, page, parser)
		if !ok {
			return
		}
		switch n := leaves[r.PostForm.Get("_leaf")].(type) {
		case *ir.Form:
			p.submit(w, r, page, req, n)
		case *ir.Table:
			p.tableAct(w, r, page, req, n)
		default:
			p.state(w, r, http.StatusBadRequest, page, req.Raw, render.BadRequest())
		}
	}
}

// submit binds and runs a form's Submit.
func (p *Program) submit(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Form) {
	ctx := r.Context()
	if n.Submit == nil {
		p.state(w, r, http.StatusBadRequest, page, req.Raw, render.BadRequest())
		return
	}

	base, err := n.Load(ctx)
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("form load: %w", err))
		return
	}
	values := map[string]string{}
	for k, v := range r.PostForm {
		if !strings.HasPrefix(k, "_") && len(v) > 0 {
			values[k] = v[0]
		}
	}

	model, errs := n.Bind(base, values)
	if len(errs) > 0 {
		p.reject(w, r, page, req, n, model, values, errs, "")
		return
	}
	if n.Submit.Guard != nil {
		if err := n.Submit.Guard(ctx, model); err != nil {
			p.state(w, r, http.StatusForbidden, page, req.Raw, render.Forbidden(err.Error()))
			return
		}
	}
	effect, err := n.Submit.Run(ctx, model)
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("submit: %w", err))
		return
	}
	if len(effect.Fields) > 0 {
		// Understood and refused, and the user can fix it: the same event, from
		// their side, as a rule the browser caught.
		p.reject(w, r, page, req, n, model, values, effect.Fields, effect.Toast)
		return
	}
	p.finish(w, r, effect, req.From)
}

// reject re-renders the page with the rejected form carrying what the user
// typed. It is not a redirect: a redirect would lose the input.
func (p *Program) reject(w http.ResponseWriter, r *http.Request, page *ir.Page, req *Request, n *ir.Form,
	model any, values map[string]string, errs []ir.FieldError, toast string) {
	req.sub, req.subLeaf = &submission{model: model, values: values, errs: errs}, render.LeafID(n.At)

	body, err := p.body(r.Context(), req, page, page.Body)
	if err != nil {
		p.fail(w, r, page, req, err)
		return
	}
	toasts := []render.Toast{{Title: "Not saved", Desc: plural(len(errs), "field needs", "fields need") + " attention.", Error: true}}
	if toast != "" {
		toasts = append(toasts, render.Toast{Title: toast})
	}
	p.writePage(w, r, http.StatusUnprocessableEntity, page, req, body, toasts...)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// finish ends a successful action: the toast rides a flash cookie, and the
// browser is sent back with a 303 so a reload does not repeat the POST.
func (p *Program) finish(w http.ResponseWriter, r *http.Request, effect ir.Effect, from string) {
	target := r.URL.RequestURI()
	if from != "" {
		target = from // a saved form returns to where it was opened from
	}
	if effect.Redirect != "" {
		if safeRedirect(effect.Redirect) {
			target = effect.Redirect
		} else {
			p.log.Error("webui: Effect.Redirect refused: not an address on this host", "target", effect.Redirect)
		}
	}
	p.setFlash(w, r, effect.Toast, false)
	//nolint:gosec // G710: target is this request's own path, or passed safeRedirect above.
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// safeRedirect accepts only a path on this host. Open produces these; a
// hand-built Target must not turn an action into an open redirect.
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
	case ok && kind == "bulk" && index < len(n.Bulk):
		action = n.Bulk[index]
	default:
		p.state(w, r, http.StatusBadRequest, page, req.Raw, render.BadRequest())
		return
	}

	rows, _, err := n.Load(ctx, queryOf(n, req.Raw))
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

	if action.Guard != nil {
		if err := action.Guard(ctx, subject); err != nil {
			p.state(w, r, http.StatusForbidden, page, req.Raw, render.Forbidden(err.Error()))
			return
		}
	}
	effect, err := action.Run(ctx, subject)
	if err != nil {
		p.fail(w, r, page, req, fmt.Errorf("action %q: %w", action.Label, err))
		return
	}
	p.finish(w, r, effect, "")
}

// refuse sends the user back with an error toast: nothing was changed.
func (p *Program) refuse(w http.ResponseWriter, r *http.Request, message string) {
	p.setFlash(w, r, message, true)
	//nolint:gosec // G710: the target is this request's own path.
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
	}
	return "", 0, "", false
}
