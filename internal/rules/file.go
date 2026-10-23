package rules

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Olian04/webui/internal/ir"
)

// The words of a file that is not what the field takes.
const msgFileRequired = "Choose a file."

// maxUploadName is the most characters of a file's name that are kept.
const maxUploadName = 255

// kinds are the kinds a field accepts with FileImage taken apart, and nothing for any file, which is what the zero value means.
func kinds(spec *ir.FileSpec) []ir.FileKind {
	var out []ir.FileKind
	add := func(ks ...ir.FileKind) {
		for _, k := range ks {
			if !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
	}
	for _, k := range spec.Kinds {
		switch k {
		case ir.FileAny:
			return nil
		case ir.FileImage:
			add(ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP)
		default:
			add(k)
		}
	}
	return out
}

// kindNames are what the kinds are called to a person.
var kindNames = map[ir.FileKind]string{
	ir.FilePNG: "PNG", ir.FileJPEG: "JPEG", ir.FileGIF: "GIF", ir.FileWebP: "WebP",
	ir.FilePDF: "PDF", ir.FileJSON: "JSON", ir.FileCSV: "CSV", ir.FileText: "text",
}

// Describe is what a field accepts, in words that a message and a rule can use: "PNG or JPEG", "" for any file.
func Describe(spec *ir.FileSpec) string {
	var names []string
	for _, k := range kinds(spec) {
		names = append(names, kindNames[k])
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// acceptTokens are the media type and the extensions of each kind, as an accept attribute has them.
var acceptTokens = map[ir.FileKind][]string{
	ir.FilePNG: {"image/png", ".png"}, ir.FileJPEG: {"image/jpeg", ".jpg", ".jpeg"}, ir.FileGIF: {"image/gif", ".gif"}, ir.FileWebP: {"image/webp", ".webp"},
	ir.FilePDF: {"application/pdf", ".pdf"}, ir.FileJSON: {"application/json", ".json"}, ir.FileCSV: {"text/csv", ".csv"}, ir.FileText: {"text/plain", ".txt"},
}

// FileAccepts are the tokens of the field's accept attribute: the media types and the extensions of the kinds it takes, none for any
// file. They are a hint to the browser's file chooser, and to script, which tells a visitor early that a file is not one of them. They are
// only a hint: the server decides by what the file holds.
func FileAccepts(spec *ir.FileSpec) []string {
	var out []string
	for _, k := range kinds(spec) {
		out = append(out, acceptTokens[k]...)
	}
	return out
}

// FileAccept is the accept attribute of the field's input, "" for any file.
func FileAccept(spec *ir.FileSpec) string { return strings.Join(FileAccepts(spec), ",") }

// AcceptsImages is whether the field takes a file that a browser can show as a picture: any file, or one of the four images.
func AcceptsImages(spec *ir.FileSpec) bool {
	accepted := kinds(spec)
	return len(accepted) == 0 || slices.ContainsFunc(accepted, isImage)
}

func isImage(k ir.FileKind) bool {
	return k == ir.FilePNG || k == ir.FileJPEG || k == ir.FileGIF || k == ir.FileWebP
}

// FileTooLarge is what to say of a file that is larger than the field takes.
func FileTooLarge(limit int64) string { return "Must be at most " + Size(limit) + "." }

// FileWrongKind is what to say of a file that is not one of the kinds that the field takes, "" for a field that takes any.
func FileWrongKind(spec *ir.FileSpec) string {
	if d := Describe(spec); d != "" {
		return "Choose a " + d + " file."
	}
	return ""
}

// Size is a number of bytes as a person reads it: "5 MiB", "1.5 KiB", "900 bytes".
func Size(n int64) string {
	if n == 1 {
		return "1 byte"
	}
	units := []string{"bytes", "KiB", "MiB", "GiB"}
	x, i := float64(n), 0
	for x >= 1024 && i < len(units)-1 {
		x /= 1024
		i++
	}
	return strconv.FormatFloat(math.Round(x*10)/10, 'f', -1, 64) + " " + units[i]
}

// maxUploadPath is the most elements of the folder of an uploaded file that are kept.
const maxUploadPath = 32

// cleanElement is one element of a name or of a folder as it is kept: with the control characters taken out, trimmed, and at most 255 characters.
func cleanElement(raw string) string {
	raw = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, raw)
	raw = strings.TrimSpace(raw)
	if runes := []rune(raw); len(runes) > maxUploadName {
		raw = string(runes[:maxUploadName])
	}
	return raw
}

// UploadName is the name of a file as it is kept: the base name only, since a browser may send a path, with the control
// characters taken out and at most 255 characters, and never empty or a dot.
func UploadName(raw string) string {
	raw = strings.ReplaceAll(raw, `\`, "/")
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		raw = raw[i+1:]
	}
	raw = cleanElement(raw)
	if raw == "" || raw == "." || raw == ".." {
		return "file"
	}
	return raw
}

// UploadPath is the folder that a file was sent in, from the name that the browser gave it, which is a path when the visitor chose a folder
// ("photos/2024/a.jpg" is "photos/2024"): made safe, so that it never leaves the folder that the app puts it in. Both kinds of slash separate it, empty elements and the
// elements "." and ".." are dropped (a ".." is not followed: "a/../b" is "a/b"), control characters are taken out, an element is at most 255 characters, and at most
// 32 are kept. It has no slash at either end, and is "" for a file that was not sent in a folder.
func UploadPath(raw string) string {
	raw = strings.ReplaceAll(raw, `\`, "/")
	i := strings.LastIndex(raw, "/")
	if i < 0 {
		return ""
	}
	var elements []string
	for _, el := range strings.Split(raw[:i], "/") {
		if el = cleanElement(el); el == "" || el == "." || el == ".." {
			continue
		}
		if elements = append(elements, el); len(elements) == maxUploadPath {
			break
		}
	}
	return strings.Join(elements, "/")
}

// HeadSize is how many bytes of a file are read to tell what it is, which is what http.DetectContentType looks at.
const HeadSize = 512

// FileHead tells what a file may be from its first bytes, which is all that it takes for an image or a PDF, and refuses a file that
// cannot be any of the kinds that the field accepts: so that a file of the wrong kind is refused before the rest of it is read. It returns the content type
// that the bytes have, the kind that they say the file is (FileAny for a file that is none of the kinds that are told by their first bytes,
// which is JSON, CSV or text, told only by reading all of it, or a file that nothing asked to be a kind), and the words for a file that is
// refused.
func FileHead(spec *ir.FileSpec, head []byte) (sniffed string, kind ir.FileKind, message string) {
	sniffed = http.DetectContentType(head)
	base, _, _ := strings.Cut(sniffed, ";")
	accepted := kinds(spec)
	for _, k := range []ir.FileKind{ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP, ir.FilePDF} {
		if binaryHolds(k, base) {
			if len(accepted) == 0 || slices.Contains(accepted, k) {
				return sniffed, k, ""
			}
			return sniffed, "", FileWrongKind(spec) // an image or a PDF is no text, and no other image
		}
	}
	if len(accepted) == 0 {
		return sniffed, ir.FileAny, ""
	}
	for _, k := range accepted {
		if (k == ir.FileJSON || k == ir.FileCSV || k == ir.FileText) && base == "text/plain" {
			return sniffed, ir.FileAny, "" // told by all of it
		}
	}
	return sniffed, "", FileWrongKind(spec)
}

// CheckFile checks the whole of an upload against what the field takes: that it is no larger than the field allows, and that it holds one of the
// kinds that it accepts. The kind is what the file holds (its first bytes, and for JSON, CSV and text the whole of it), and not
// what it is called or what the browser said, which a visitor chooses. It returns the content type that the file was found to
// have, the kind that it was found to be (FileAny for a file that is none of the kinds that have a name, which only a field that takes any
// file lets in), and the words for a file that is refused.
func CheckFile(spec *ir.FileSpec, data []byte) (contentType string, kind ir.FileKind, message string) {
	if int64(len(data)) > spec.Max {
		return "", "", FileTooLarge(spec.Max)
	}
	sniffed, kind, message := FileHead(spec, data[:min(len(data), HeadSize)])
	if message != "" {
		return "", "", message
	}
	accepted := kinds(spec)
	if len(accepted) == 0 || kind != ir.FileAny {
		return sniffed, kind, "" // told by its first bytes, which is not read through, as a kind of text would be
	}
	// The most specific kind first, whatever order the field lists them in: a JSON file that a field takes as JSON or text is JSON.
	matching := slices.Clone(accepted)
	slices.SortStableFunc(matching, func(a, b ir.FileKind) int { return a.Order() - b.Order() })
	for _, k := range matching {
		if ct, ok := holds(k, sniffed, data); ok {
			return ct, k, ""
		}
	}
	return "", "", FileWrongKind(spec)
}

// binaryHolds is whether a file of the content type is of an image or a PDF kind.
func binaryHolds(k ir.FileKind, base string) bool {
	switch k {
	case ir.FilePNG:
		return base == "image/png"
	case ir.FileJPEG:
		return base == "image/jpeg"
	case ir.FileGIF:
		return base == "image/gif"
	case ir.FileWebP:
		return base == "image/webp"
	case ir.FilePDF:
		return base == "application/pdf"
	case ir.FileJSON, ir.FileCSV, ir.FileText, ir.FileImage, ir.FileAny:
	}
	return false
}

// holds is whether a file that was sniffed as a type is of a kind that is told by reading all of it, and the content type that it then has.
func holds(k ir.FileKind, sniffed string, data []byte) (string, bool) {
	base, _, _ := strings.Cut(sniffed, ";")
	switch k {
	case ir.FileJSON:
		return "application/json", base == "text/plain" && validText(bytes.NewReader(data)) && validJSON(bytes.NewReader(data))
	case ir.FileCSV:
		return "text/csv", base == "text/plain" && validText(bytes.NewReader(data)) && validCSV(bytes.NewReader(data))
	case ir.FileText:
		return sniffed, base == "text/plain" && validText(bytes.NewReader(data))
	case ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP, ir.FilePDF:
		return base, binaryHolds(k, base)
	case ir.FileImage, ir.FileAny:
	}
	return "", false
}

// validText is text that a person could read: UTF-8, with no NUL, and not empty.
func validText(r io.Reader) bool {
	br := bufio.NewReader(r)
	seen := false
	for {
		c, size, err := br.ReadRune()
		if errors.Is(err, io.EOF) {
			return seen
		}
		if err != nil || c == 0 || (c == utf8.RuneError && size == 1) {
			return false
		}
		seen = true
	}
}

// validJSON is exactly one JSON value, read as a stream so that a large file is not held in memory.
func validJSON(r io.Reader) bool {
	dec := json.NewDecoder(r)
	depth, values := 0, 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return depth == 0 && values == 1
		}
		if err != nil {
			return false
		}
		if d, ok := tok.(json.Delim); ok {
			if d == '[' || d == '{' {
				depth++
				continue
			}
			depth--
		}
		if depth == 0 {
			values++
		}
	}
}

// validCSV is a file that reads as comma-separated records, each of any number of fields, and has at least one.
func validCSV(r io.Reader) bool {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = true
	records := 0
	for {
		_, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return records > 0
		}
		if err != nil {
			return false
		}
		records++
	}
}

// FileRequired is what to say of a file that is needed and was not chosen.
func FileRequired() string { return msgFileRequired }

// FileLines are what a file field says of itself in the popover beside its label: what it takes, and how large.
func FileLines(spec *ir.FileSpec) []string {
	var out []string
	if d := Describe(spec); d != "" {
		out = append(out, "A "+d+" file")
	}
	return append(out, fmt.Sprintf("Up to %s", Size(spec.Max)))
}
