package runtime

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Olian04/webui/internal/render"
)

// The flash is how a toast survives the redirect after a POST without any
// script and without putting anything in the URL: a short-lived cookie, read
// and cleared by the next full page. It only ever holds text the user's own
// action produced, and the text is escaped when it is rendered, so a forged
// value can show its forger a message and nothing else.
//
// It also carries a token for a rejected submission that the server keeps (see
// [submissions]), so that the form can be drawn again with what was typed after a
// redirect, and not as the answer to the POST. The cookie then holds nothing the
// visitor typed.
const (
	flashCookie  = "webui_flash"
	flashMax     = 300 // bytes of message kept
	flashSeconds = 60  // how long the cookie lives
	flashKept    = 's' // the kind of a cookie that names a kept submission, where the others are tones
)

func (p *Program) flashPath() string {
	if p.Prefix == "" {
		return "/"
	}
	return p.Prefix
}

func (p *Program) setFlash(w http.ResponseWriter, r *http.Request, text string, tone render.ToastTone) {
	if text == "" {
		return
	}
	if len(text) > flashMax {
		text = strings.ToValidUTF8(text[:flashMax], "")
	}
	kind := "o"
	switch tone {
	case render.ToastWarning:
		kind = "w"
	case render.ToastError:
		kind = "e"
	case render.ToastOK:
	}
	http.SetCookie(w, p.flash(r, base64.RawURLEncoding.EncodeToString([]byte(kind+text)), flashSeconds))
}

// flash is the cookie that carries a toast to the next page, or with a negative maxAge clears it.
func (p *Program) flash(r *http.Request, value string, maxAge int) *http.Cookie {
	//nolint:gosec // G124: Secure follows the request's scheme; the cookie holds one short-lived toast.
	return &http.Cookie{
		Name: flashCookie, Value: value, Path: p.flashPath(), MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	}
}

// setKept names a kept submission in the cookie, for the page the visitor is sent to.
func (p *Program) setKept(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, p.flash(r, base64.RawURLEncoding.EncodeToString([]byte(string(rune(flashKept))+token)), flashSeconds))
}

// pending is what the flash cookie carries to this page: toasts, and a submission that was kept for it.
type pending struct {
	toasts []render.Toast
	kept   *kept
}

type pendingKey struct{}

// takeFlash is what the cookie carries to this page, and clears it. It is read once for a response,
// whichever of its steps asks first, since the cookie is cleared and what it named is taken. A cookie that
// does not decode is cleared and ignored. Only a GET takes it, so that a HEAD does not.
func (p *Program) takeFlash(w http.ResponseWriter, r *http.Request) *pending {
	if got, ok := r.Context().Value(pendingKey{}).(*pending); ok {
		return got
	}
	got := &pending{}
	if r.Method == http.MethodGet {
		if cookie, err := r.Cookie(flashCookie); err == nil {
			http.SetCookie(w, p.flash(r, "", -1))
			got = p.decodeFlash(cookie.Value)
		}
	}
	*r = *r.WithContext(context.WithValue(r.Context(), pendingKey{}, got))
	return got
}

func (p *Program) decodeFlash(value string) *pending {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) < 2 || !utf8.Valid(raw) {
		return &pending{}
	}
	if raw[0] == flashKept {
		k := p.subs.take(string(raw[1:]))
		if k == nil {
			// It expired, was dropped to make room, or went to another instance: the page is drawn as it
			// was loaded, and says so.
			return &pending{toasts: []render.Toast{{Title: "Your changes could not be kept", Desc: "Please enter them again.", Tone: render.ToastWarning}}}
		}
		return &pending{toasts: []render.Toast{k.toast}, kept: k}
	}
	tone := render.ToastOK
	switch raw[0] {
	case 'w':
		tone = render.ToastWarning
	case 'e':
		tone = render.ToastError
	}
	return &pending{toasts: []render.Toast{{Title: string(raw[1:]), Tone: tone}}}
}

// restore puts a submission that was kept for this address back into the request, so that its form is drawn with what
// was typed and what was refused. A submission kept for another address is dropped with its toast: it is what some
// other page, or the same page with other arguments, was sent.
func (p *Program) restore(w http.ResponseWriter, r *http.Request, req *Request) {
	got := p.takeFlash(w, r)
	if got.kept == nil {
		return
	}
	if got.kept.address != req.Address {
		got.toasts, got.kept = nil, nil
		return
	}
	req.sub, req.subLeaf = got.kept.sub, got.kept.leaf
}
