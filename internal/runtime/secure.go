package runtime

import (
	"net/http"
	"strings"
)

// csp allows what the shell uses and nothing else. Inline styles stay because
// the components size bars and skeletons with style attributes; scripts are
// ours alone, so an injected one would not run.
const csp = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
	"object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

// policy is the content security policy of a page that draws pictures from origins: the
// same as everyone else's, with them added to img-src. Every other page's is csp,
// byte for byte. An origin that could break out of the directive is left out, though
// the declaration was checked when the app was compiled.
func policy(imgOrigins []string) string {
	var allowed []string
	for _, o := range imgOrigins {
		if strings.HasPrefix(o, "https://") && !strings.ContainsAny(o, " ;,'\"\\\n\r\t") {
			allowed = append(allowed, o)
		}
	}
	if len(allowed) == 0 {
		return csp
	}
	return strings.Replace(csp, "img-src 'self' data:", "img-src 'self' data: "+strings.Join(allowed, " "), 1)
}

// secure sets the headers every response carries.
func secure(next http.Handler) http.Handler { return securedFor(csp, next) }

// securedFor is secure with the content security policy it is given.
func securedFor(policy string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", policy)
		next.ServeHTTP(w, r)
	})
}
