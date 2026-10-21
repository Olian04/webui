package rules

import (
	"bufio"
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
const (
	msgFileUnreadable = "Could not read this file."
	msgFileRequired   = "Choose a file."
)

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

// OnlyImages is whether every file that the field takes is an image, so that the file that is stored is one.
func OnlyImages(spec *ir.FileSpec) bool {
	accepted := kinds(spec)
	return len(accepted) > 0 && !slices.ContainsFunc(accepted, func(k ir.FileKind) bool { return !isImage(k) })
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

// UploadName is the name of a file as it is kept: the base name only, since a browser may send a path, with the control
// characters taken out and at most 255 characters, and never empty or a dot.
func UploadName(raw string) string {
	raw = strings.ReplaceAll(raw, `\`, "/")
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		raw = raw[i+1:]
	}
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
	if raw == "" || raw == "." || raw == ".." {
		return "file"
	}
	return raw
}

// CheckFile checks an upload against what the field takes: that it is no larger than the field allows, and that it holds one of the
// kinds that it accepts. The kind is what the file holds (its first bytes, and for JSON, CSV and text the whole of it), and not
// what it is called or what the browser said, which a visitor chooses. It returns the content type that the file was found to
// have, the kind that it was found to be (FileAny for a file that is none of the kinds that have a name, which only a field that takes any
// file lets in), and the words for a file that is refused.
func CheckFile(spec *ir.FileSpec, up *ir.Upload) (contentType string, kind ir.FileKind, message string) {
	if up.Size > spec.Max {
		return "", "", FileTooLarge(spec.Max)
	}
	r, err := up.Open()
	if err != nil {
		return "", "", msgFileUnreadable
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	_ = r.Close()
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", "", msgFileUnreadable
	}
	sniffed := http.DetectContentType(head[:n])
	accepted := kinds(spec)
	if len(accepted) == 0 {
		// Any file: it is told apart only by what its first bytes say, which is not read through, as a kind of text would be.
		for _, k := range []ir.FileKind{ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP, ir.FilePDF} {
			if _, ok := holds(k, sniffed, up); ok {
				return sniffed, k, ""
			}
		}
		return sniffed, ir.FileAny, ""
	}
	// The most specific kind first, whatever order the field lists them in: a JSON file that a field takes as JSON or text is JSON.
	matching := slices.Clone(accepted)
	slices.SortStableFunc(matching, func(a, b ir.FileKind) int { return a.Order() - b.Order() })
	for _, k := range matching {
		if ct, ok := holds(k, sniffed, up); ok {
			return ct, k, ""
		}
	}
	return "", "", FileWrongKind(spec)
}

// holds is whether a file that was sniffed as a type is of the kind, and the content type that it then has.
func holds(k ir.FileKind, sniffed string, up *ir.Upload) (string, bool) {
	base, _, _ := strings.Cut(sniffed, ";")
	switch k {
	case ir.FilePNG:
		return base, base == "image/png"
	case ir.FileJPEG:
		return base, base == "image/jpeg"
	case ir.FileGIF:
		return base, base == "image/gif"
	case ir.FileWebP:
		return base, base == "image/webp"
	case ir.FilePDF:
		return base, base == "application/pdf"
	case ir.FileJSON:
		return "application/json", base == "text/plain" && readsAs(up, validText) && readsAs(up, validJSON)
	case ir.FileCSV:
		return "text/csv", base == "text/plain" && readsAs(up, validText) && readsAs(up, validCSV)
	case ir.FileText:
		return sniffed, base == "text/plain" && readsAs(up, validText)
	case ir.FileImage, ir.FileAny:
	}
	return "", false
}

// readsAs is whether the upload, read from its start, is what check says it is.
func readsAs(up *ir.Upload, check func(io.Reader) bool) bool {
	r, err := up.Open()
	if err != nil {
		return false
	}
	defer func() { _ = r.Close() }() // a read-only handle: there is nothing to lose if closing fails
	return check(r)
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
