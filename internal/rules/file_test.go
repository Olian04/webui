package rules

import (
	"strings"
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/test/util/assert"
)

func TestUploadNameIsTheBaseNameWithNoControlCharactersAndAtMost255(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		`C:\Users\me\logo.png`:   "logo.png",
		"a\x00b\x07c.png":        "abc.png",
		"line\nbreak\r\n.png":    "linebreak.png",
		"tab\there.png":          "tabhere.png",
		"a/b/c.png":              "c.png",
		"../../etc/passwd":       "passwd",
		"..":                     "file",
		".":                      "file",
		"dir/":                   "file",
		"":                       "file",
		"\x00\x01":               "file",
		"  spaced.png ":          "spaced.png",
		strings.Repeat("é", 300): strings.Repeat("é", 255),
		"bad\xffbyte.png":        "badbyte.png",
	} {
		assert.Equal(t, UploadName(raw), want)
	}
}

func TestSizeIsReadByAPerson(t *testing.T) {
	t.Parallel()

	for n, want := range map[int64]string{
		0: "0 bytes", 1: "1 byte", 900: "900 bytes", 1024: "1 KiB", 1536: "1.5 KiB", 5 << 20: "5 MiB", 256 << 20: "256 MiB", 3 << 30: "3 GiB",
	} {
		assert.Equal(t, Size(n), want)
	}
}

func TestWhatAFieldTakesIsSaidInWords(t *testing.T) {
	t.Parallel()

	for kinds, want := range map[string]string{
		"": "", "png": "PNG", "png,jpeg": "PNG or JPEG", "png,pdf,csv": "PNG, PDF or CSV", "image": "PNG, JPEG, GIF or WebP", "any": "", "png,any": "",
		"png,image": "PNG, JPEG, GIF or WebP", "text": "text",
	} {
		var spec ir.FileSpec
		for _, k := range strings.Split(kinds, ",") {
			if k != "" {
				spec.Kinds = append(spec.Kinds, ir.FileKind(k))
			}
		}
		assert.Equal(t, Describe(&spec), want)
	}
	assert.Equal(t, FileAccept(&ir.FileSpec{Kinds: []ir.FileKind{ir.FilePNG, ir.FileCSV}}), "image/png,.png,text/csv,.csv")
	assert.Equal(t, FileAccept(&ir.FileSpec{}), "")
}

// The order that kinds are matched in puts the more specific first: what excludes the rest, then the grammars, then text, then any.
func TestKindsAreMatchedFromTheMostSpecificToTheLeast(t *testing.T) {
	t.Parallel()

	inOrder := []ir.FileKind{ir.FilePNG, ir.FileJPEG, ir.FileGIF, ir.FileWebP, ir.FilePDF, ir.FileJSON, ir.FileCSV, ir.FileText, ir.FileAny}
	for i := 1; i < len(inOrder); i++ {
		assert.True(t, inOrder[i-1].Order() < inOrder[i].Order())
	}
	assert.Equal(t, ir.FileImage.Order(), ir.FilePNG.Order()) // four of the first
	assert.True(t, ir.FileKind("unknown").Order() > ir.FileAny.Order())
}
