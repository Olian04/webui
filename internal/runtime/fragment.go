package runtime

import (
	"net/http"

	"github.com/Olian04/webui/internal/render"
)

// wantsFragment is whether a request is a POST that script made, which names the leaf that it is for, as a request for that leaf's HTML does (see LeafHeader). It is
// answered with the HTML of what happened, a fragment that script puts in the page, and not with a flash and a redirect, which a page that is navigated to shows: the
// server answers script with HTML, never with data that script has to read (see docs/AGENTS.md).
func wantsFragment(r *http.Request) bool {
	return r.Method == http.MethodPost && r.Header.Get(LeafHeader) != ""
}

// writeResult answers a script-made POST with the result as a list item: the tone and the text of what the toast would have said, and, when the outcome goes somewhere
// else, a link to it, which is the next action that the response offers.
func (p *Program) writeResult(w http.ResponseWriter, r *http.Request, status int, tone render.ToastTone, text, next string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := render.UploadResult(tone, text, next).Render(r.Context(), w); err != nil {
		p.log.Error("webui: a result could not be written", "path", r.URL.Path, "err", err)
	}
}
