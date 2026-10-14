package runtime

import (
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
const (
	flashCookie = "webui_flash"
	flashMax    = 300 // bytes of message kept
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
	http.SetCookie(w, p.flash(r, base64.RawURLEncoding.EncodeToString([]byte(kind+text)), 60))
}

// flash is the cookie that carries a toast to the next page, or with a negative maxAge clears it.
func (p *Program) flash(r *http.Request, value string, maxAge int) *http.Cookie {
	//nolint:gosec // G124: Secure follows the request's scheme; the cookie holds one short-lived toast.
	return &http.Cookie{
		Name: flashCookie, Value: value, Path: p.flashPath(), MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	}
}

// takeFlash returns the pending toast, if any, and clears it. A cookie that
// does not decode is cleared and ignored.
func (p *Program) takeFlash(w http.ResponseWriter, r *http.Request) []render.Toast {
	if r.Method != http.MethodGet {
		return nil
	}
	cookie, err := r.Cookie(flashCookie)
	if err != nil {
		return nil
	}
	http.SetCookie(w, p.flash(r, "", -1))
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || len(raw) < 2 || !utf8.Valid(raw) {
		return nil
	}
	tone := render.ToastOK
	switch raw[0] {
	case 'w':
		tone = render.ToastWarning
	case 'e':
		tone = render.ToastError
	}
	return []render.Toast{{Title: string(raw[1:]), Tone: tone}}
}
