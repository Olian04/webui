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

// HeadSize is how many bytes of a file are read to tell what it is, which is what http.DetectContentType looks at.
const HeadSize = 512

// FileCheck checks a file as it arrives: that it holds one of the kinds that the field accepts. The kind is what the file holds (its first
// bytes, and for JSON, CSV and text the whole of it), and not what it is called or what the browser said, which a visitor chooses.
// Begin it with the first bytes, write every byte of the file to Writer, and ask Finish what the file was found to be, or Abort it
// if the file is not going to be finished. Whichever it is, it must be ended: the checks of the text kinds run on goroutines of
// their own, which Finish and Abort wait for.
type FileCheck struct {
	sniffed   string
	headKind  ir.FileKind
	candidate []ir.FileKind // the kinds that the file may be, the most specific first
	sinks     []*textCheck
	w         io.Writer
	anyFile   bool // the field takes any file
}

// textCheck is one check of a text kind, which reads what is written to it as a stream.
type textCheck struct {
	which  int // which of the checks it is
	pw     *io.PipeWriter
	pr     *io.PipeReader
	result chan bool
	ok     bool
}

// the checks that the text kinds are made of.
const (
	whichText = iota
	whichJSON
	whichCSV
)

var textChecks = [...]func(io.Reader) bool{whichText: validText, whichJSON: validJSON, whichCSV: validCSV}

// BeginFile starts the check of a file whose first bytes are head. It returns the words for a file that cannot be any of the kinds
// that the field accepts, and then there is nothing to end.
func BeginFile(spec *ir.FileSpec, head []byte) (*FileCheck, string) {
	c := &FileCheck{sniffed: http.DetectContentType(head), headKind: ir.FileAny}
	base, _, _ := strings.Cut(c.sniffed, ";")
	accepted := kinds(spec)
	if len(accepted) == 0 {
		c.anyFile = true
		// Any file: it is told apart only by what its first bytes say, which is not read through, as a kind of text would be.
		for _, k := range []ir.FileKind{ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP, ir.FilePDF} {
			if binaryHolds(k, base) {
				c.headKind = k
			}
		}
		c.w = io.Discard
		return c, ""
	}
	// The most specific kind first, whatever order the field lists them in: a JSON file that a field takes as JSON or text is JSON.
	slices.SortStableFunc(accepted, func(a, b ir.FileKind) int { return a.Order() - b.Order() })
	needs := [len(textChecks)]bool{}
	for _, k := range accepted {
		switch k {
		case ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP, ir.FilePDF:
			if binaryHolds(k, base) {
				c.candidate, c.headKind = append(c.candidate, k), k
			}
		case ir.FileJSON, ir.FileCSV, ir.FileText:
			if base == "text/plain" {
				c.candidate = append(c.candidate, k)
				needs[whichText] = true
				needs[whichJSON] = needs[whichJSON] || k == ir.FileJSON
				needs[whichCSV] = needs[whichCSV] || k == ir.FileCSV
			}
		case ir.FileImage, ir.FileAny:
		}
	}
	if len(c.candidate) == 0 {
		return nil, FileWrongKind(spec)
	}
	var writers []io.Writer
	for i, check := range textChecks {
		if !needs[i] {
			continue
		}
		t := &textCheck{which: i, result: make(chan bool, 1)}
		t.pr, t.pw = io.Pipe()
		go func() {
			t.result <- check(t.pr)
			_ = t.pr.Close() // a check that is done reads no more, so that what is written to it is not waited on
		}()
		c.sinks = append(c.sinks, t)
		writers = append(writers, t)
	}
	if len(writers) == 0 {
		c.w = io.Discard
	} else {
		c.w = io.MultiWriter(writers...)
	}
	return c, ""
}

// Write feeds what the file holds to the checks. It never fails: a check that has what it needs ignores the rest.
func (t *textCheck) Write(b []byte) (int, error) {
	_, _ = t.pw.Write(b) // closed by a check that is done, which is not an error of the file
	return len(b), nil
}

// HeadKind is the kind of the file as its first bytes tell it, which is the kind of an image or a PDF, and FileAny for a file whose kind
// is known only when all of it has been read, or that is none of the kinds that have a name.
func (c *FileCheck) HeadKind() ir.FileKind { return c.headKind }

// Sniffed is the content type that the first bytes have.
func (c *FileCheck) Sniffed() string { return c.sniffed }

// Writer is where every byte of the file is written, the first bytes too.
func (c *FileCheck) Writer() io.Writer { return c.w }

// Finish ends the check of a file that has been written whole. It returns the content type that the file was found to have, the kind that it
// was found to be (FileAny for a file that is none of the kinds that have a name, which only a field that takes any file lets in), and
// the words for a file that is not one of the kinds that the field accepts.
func (c *FileCheck) Finish(spec *ir.FileSpec) (contentType string, kind ir.FileKind, message string) {
	for _, t := range c.sinks {
		_ = t.pw.Close()
	}
	for _, t := range c.sinks {
		t.ok = <-t.result
	}
	if c.anyFile {
		return c.sniffed, c.headKind, ""
	}
	base, _, _ := strings.Cut(c.sniffed, ";")
	valid := func(i int) bool {
		for _, t := range c.sinks {
			if t.which == i {
				return t.ok
			}
		}
		return false
	}
	for _, k := range c.candidate {
		switch k {
		case ir.FileJSON:
			if valid(whichText) && valid(whichJSON) {
				return "application/json", k, ""
			}
		case ir.FileCSV:
			if valid(whichText) && valid(whichCSV) {
				return "text/csv", k, ""
			}
		case ir.FileText:
			if valid(whichText) {
				return c.sniffed, k, ""
			}
		case ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP, ir.FilePDF:
			return base, k, ""
		case ir.FileImage, ir.FileAny:
		}
	}
	return "", "", FileWrongKind(spec)
}

// Abort ends the check of a file that is not going to be finished.
func (c *FileCheck) Abort() {
	for _, t := range c.sinks {
		_ = t.pw.CloseWithError(io.ErrClosedPipe)
	}
	for _, t := range c.sinks {
		<-t.result
	}
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
