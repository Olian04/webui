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

func (p *Program) setFlash(w http.ResponseWriter, r *http.Request, text string, isErr bool) {
	if text == "" {
		return
	}
	if len(text) > flashMax {
		text = strings.ToValidUTF8(text[:flashMax], "")
	}
	kind := "t"
	if isErr {
		kind = "e"
	}
	//nolint:gosec // G124: Secure follows the request's scheme; the cookie holds one short-lived toast.
	http.SetCookie(w, &http.Cookie{
		Name: flashCookie, Value: base64.RawURLEncoding.EncodeToString([]byte(kind + text)),
		Path: p.flashPath(), MaxAge: 60, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
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
	//nolint:gosec // G124: clearing the cookie; Secure follows the request's scheme.
	http.SetCookie(w, &http.Cookie{
		Name: flashCookie, Path: p.flashPath(), MaxAge: -1, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || len(raw) < 2 || !utf8.Valid(raw) {
		return nil
	}
	return []render.Toast{{Title: string(raw[1:]), Error: raw[0] == 'e'}}
}
