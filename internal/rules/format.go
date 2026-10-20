package rules

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

// The words of a value that is not the format of its input.
const (
	msgEmail = "Enter an email address."
	msgURL   = "Enter a web address that starts with http:// or https://."
	msgTel   = "Enter a phone number."
	msgColor = "Enter a color as #rrggbb."
)

// emailAddress is what the HTML standard says a valid email address is, which is what a browser checks an email input
// by, so that the server and the browser agree. It is looser than the RFCs on purpose (it allows "a@b"), and stricter
// than nothing.
var emailAddress = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// telNumber is a permissive phone number: digits with the punctuation that numbers are written with, an extension's "x",
// and at least one digit. A browser checks none, so only the server does, and loosely.
var telNumber = regexp.MustCompile(`^[0-9 +().x/-]*[0-9][0-9 +().x/-]*$`)

// colorValue is what a color input holds: a "#" and six hex digits.
var colorValue = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// formatMessage is what to say of a value that is not of format f.
func formatMessage(f ir.Format) string {
	switch f {
	case ir.FormatEmail:
		return msgEmail
	case ir.FormatURL:
		return msgURL
	case ir.FormatTel:
		return msgTel
	case ir.FormatColor:
		return msgColor
	}
	return ""
}

// checkFormat is whether text is of the format its input is, or "" for a text of any format. An email is held to what the
// browser holds it to. A web address is held to more: a browser accepts any address with a scheme, javascript: among them,
// and what is stored is going to be linked, so it is an http or https address with a host.
func checkFormat(f ir.Format, s string) string {
	s = strings.TrimSpace(s)
	ok := true
	switch f {
	case ir.FormatEmail:
		ok = emailAddress.MatchString(s)
	case ir.FormatURL:
		u, err := url.Parse(s)
		ok = err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && !strings.ContainsAny(s, " \t\r\n")
	case ir.FormatTel:
		ok = telNumber.MatchString(s)
	case ir.FormatColor:
		ok = colorValue.MatchString(s)
	}
	if ok {
		return ""
	}
	return formatMessage(f)
}
