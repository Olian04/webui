package runtime

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Olian04/webui/internal/render"
)

// The flash is how a toast survives the redirect after an accepted POST without any
// script and without putting anything in the URL: a short-lived cookie, read
// and cleared by the next full page. It only ever holds text the user's own
// action produced, and the text is escaped when it is rendered, so a forged
// value can show its forger a message and nothing else. A submission that was
// refused is not redirected (see returnToForm), so nothing the visitor typed is in
// the cookie or on the server.
const (
	flashCookie  = "webui_flash"
	flashMax     = 300 // bytes of message kept
	flashSeconds = 60  // how long the cookie lives
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

// pending is what the flash cookie carries to this page: its toasts.
type pending struct {
	toasts []render.Toast
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
	tone := render.ToastOK
	switch raw[0] {
	case 'w':
		tone = render.ToastWarning
	case 'e':
		tone = render.ToastError
	}
	return &pending{toasts: []render.Toast{{Title: string(raw[1:]), Tone: tone}}}
}
