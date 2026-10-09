package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
)

// The demo's authentication, small but real. A visitor with no session is sent to
// /logout, which asks who they want to be: a viewer, who may look, or an editor, who
// may change things. The choice is a signed cookie, so it cannot be forged, and the
// middleware puts the role in the request's context, where the app's Guards read it.
//
// webui itself does no authentication. It runs the Guard of a page or an action with
// the request's context, so whatever the surrounding handler put there is what a Guard
// can ask about. That is the whole contract.

// Role is what a session may do.
type Role string

// The roles.
const (
	Viewer Role = "viewer"
	Editor Role = "editor"
)

type roleKey struct{}

// RoleOf is the role of the session a request is part of.
func RoleOf(ctx context.Context) Role {
	role, _ := ctx.Value(roleKey{}).(Role)
	return role
}

// canEdit is the Guard of every action and of the pages for editors. One function
// serves them all: it takes the subject and ignores it, and asks the session.
func canEdit[T any](ctx context.Context, _ T) error {
	if RoleOf(ctx) != Editor {
		return errors.New("requires the editor role")
	}
	return nil
}

const sessionCookie = "demo_session"

// secret signs the sessions. It is new each time the demo starts, so a session does
// not outlive the process: choosing again is the price of not storing anything.
var secret = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}()

func sign(role Role) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(role))
	return string(role) + "." + hex.EncodeToString(mac.Sum(nil))
}

// verify is the role a cookie value is signed for, and false for a value that is not
// one of ours.
func verify(value string) (Role, bool) {
	role, sig, ok := strings.Cut(value, ".")
	if !ok || (Role(role) != Viewer && Role(role) != Editor) {
		return "", false
	}
	want, _ := hex.DecodeString(strings.TrimPrefix(sign(Role(role)), role+"."))
	got, err := hex.DecodeString(sig)
	if err != nil || !hmac.Equal(got, want) {
		return "", false
	}
	return Role(role), true
}

// authenticated serves next to a visitor with a session, with their role in the
// context, and sends anyone else to choose one.
func authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err == nil {
			if role, ok := verify(cookie.Value); ok {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), roleKey{}, role)))
				return
			}
		}
		http.Redirect(w, r, "/logout", http.StatusFound)
	})
}

// logout ends the session and asks who to continue as.
func logout(w http.ResponseWriter, _ *http.Request) {
	//nolint:gosec // G124: clearing the cookie; Secure follows the request's scheme, as at login.
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(chooser))
}

// login starts a session as the role chosen, and goes to the app.
func login(w http.ResponseWriter, r *http.Request) {
	role := Role(r.PostFormValue("role"))
	if role != Viewer && role != Editor {
		http.Error(w, "choose a viewer or an editor", http.StatusBadRequest)
		return
	}
	//nolint:gosec // G124: Secure follows the request's scheme, so the demo works over plain http locally.
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: sign(role), Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

const chooser = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="dark light"><title>Collector — choose who to continue as</title>
<style>
body{margin:0;min-height:100vh;display:grid;place-items:center;font:14px/1.5 system-ui,sans-serif;background:#111217;color:#ccccdc}
main{width:min(420px,calc(100vw - 32px));background:#181b1f;border:1px solid rgba(204,204,220,.11);border-radius:2px;padding:20px}
h1{font-size:17px;font-weight:500;margin:0 0 4px}p{margin:0 0 16px;color:rgba(204,204,220,.65)}
form{margin:0 0 8px}
.note{margin:16px 0 0;padding:10px 12px;font-size:12.5px;border:1px solid rgba(61,113,217,.45);background:rgba(61,113,217,.12);border-radius:2px;color:rgba(204,204,220,.8)}.note b{font-weight:500}button{width:100%;text-align:left;font:inherit;color:inherit;background:#22252b;border:1px solid rgba(204,204,220,.22);border-radius:2px;padding:10px 12px;cursor:pointer}
button:hover{border-color:#3d71d9}button b{display:block;font-weight:500}button span{color:rgba(204,204,220,.65);font-size:12.5px}
@media (prefers-color-scheme:light){body{background:#f4f5f5;color:#24292e}main{background:#fff;border-color:rgba(36,41,46,.12)}p,button span,.note{color:#5c6269}.note{background:#eef3fc;border-color:rgba(61,113,217,.35)}button{background:#fff;border-color:rgba(36,41,46,.24)}}
</style></head><body><main>
<h1>Collector</h1><p>You are signed out. Choose who to continue as.</p>
<form method="post" action="/login"><input type="hidden" name="role" value="viewer"><button><b>Continue as a viewer</b><span>Can look at everything, and change nothing: guarded controls are disabled, with the reason.</span></button></form>
<form method="post" action="/login"><input type="hidden" name="role" value="editor"><button><b>Continue as an editor</b><span>Can change devices, alerts and settings, and read the audit log.</span></button></form>
<p class="note" role="note"><b>About this page.</b> It is not part of the library: it was made for the demo, to stand in for a sign-in page. The library does no authentication of its own.</p>
</main></body></html>`
