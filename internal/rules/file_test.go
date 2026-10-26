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
		for k := range strings.SplitSeq(kinds, ",") {
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

func TestAFileIsToldByItsFirstBytesAndByAllOfItForTheKindsOfText(t *testing.T) {
	t.Parallel()

	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 600)...)
	for _, c := range []struct {
		name     string
		kinds    []ir.FileKind
		data     []byte
		wantType string
		wantKind ir.FileKind
		refused  bool
	}{
		{"a png", []ir.FileKind{ir.FilePNG}, png, "image/png", ir.FilePNG, false},
		{"a png where an image is taken", []ir.FileKind{ir.FileImage}, png, "image/png", ir.FilePNG, false},
		{"a png where a pdf is taken", []ir.FileKind{ir.FilePDF}, png, "", "", true},
		{"a png where json is taken", []ir.FileKind{ir.FileJSON}, png, "", "", true},
		{"json", []ir.FileKind{ir.FileJSON}, []byte(`{"a": [1, 2]}`), "application/json", ir.FileJSON, false},
		{"json that ends early", []ir.FileKind{ir.FileJSON}, []byte(`{"a": `), "", "", true},
		{"json past the head", []ir.FileKind{ir.FileJSON}, []byte(`[` + strings.Repeat(`1,`, 1000) + `1]`), "application/json", ir.FileJSON, false},
		{"json that goes wrong past the head", []ir.FileKind{ir.FileJSON}, []byte(`[` + strings.Repeat(`1,`, 1000) + `1,]`), "", "", true},
		{"csv", []ir.FileKind{ir.FileCSV}, []byte("a,b\n1,2\n"), "text/csv", ir.FileCSV, false},
		{"text that has a NUL past the head", []ir.FileKind{ir.FileText}, []byte(strings.Repeat("words ", 200) + "\x00"), "", "", true},
		{"json, csv and text at once, which is json", []ir.FileKind{ir.FileText, ir.FileCSV, ir.FileJSON}, []byte(`{"a": 1}`), "application/json", ir.FileJSON, false},
		{"the same, which is only csv", []ir.FileKind{ir.FileText, ir.FileCSV, ir.FileJSON}, []byte("a,b\n"), "text/csv", ir.FileCSV, false},
		{"the same, which is only text", []ir.FileKind{ir.FileText, ir.FileCSV, ir.FileJSON}, []byte("a,\"b\n"), "text/plain; charset=utf-8", ir.FileText, false},
		{"any file, a pdf", nil, []byte("%PDF-1.4\n%%EOF\n"), "application/pdf", ir.FilePDF, false},
		{"any file, which is none that is named", nil, []byte(`{"a": 1}`), "text/plain; charset=utf-8", ir.FileAny, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			ct, kind, message := CheckFile(&ir.FileSpec{Kinds: c.kinds, Max: 1 << 20}, c.data)
			assert.Equal(t, message != "", c.refused)
			assert.Equal(t, ct, c.wantType)
			assert.Equal(t, kind, c.wantKind)
		})
	}
	_, _, message := CheckFile(&ir.FileSpec{Max: 10}, []byte("0123456789a"))
	assert.Equal(t, message, "Must be at most 10 bytes.")
}

// What the first bytes tell is known before the rest has been read: the kind of an image or a PDF, and a refusal of a file that no kind takes.
func TestWhatTheFirstBytesTellIsKnownAtTheStart(t *testing.T) {
	t.Parallel()

	spec := &ir.FileSpec{Kinds: []ir.FileKind{ir.FileImage, ir.FileJSON}}
	sniffed, kind, message := FileHead(spec, []byte("\xff\xd8\xff\xe0\x00\x10JFIF"))
	assert.Equal(t, message, "")
	assert.Equal(t, kind, ir.FileJPEG)
	assert.Equal(t, sniffed, "image/jpeg")

	_, kind, message = FileHead(spec, []byte(`{"a": 1}`))
	assert.Equal(t, message, "")
	assert.Equal(t, kind, ir.FileAny) // told only by all of it

	_, _, message = FileHead(&ir.FileSpec{Kinds: []ir.FileKind{ir.FilePNG, ir.FileJPEG}}, []byte("%PDF-1.4"))
	assert.Equal(t, message, "Choose a PNG or JPEG file.")
	_, _, message = FileHead(&ir.FileSpec{Kinds: []ir.FileKind{ir.FilePNG}}, []byte("plain words"))
	assert.Equal(t, message, "Choose a PNG file.")
}

// The folder of a file is what the browser sent it in, made safe: whatever it says, it never leaves the folder that the app puts it in.
func TestTheFolderOfAFileIsMadeSafe(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		"photos/2024/a.jpg":             "photos/2024",
		`photos\2024\a.jpg`:             "photos/2024",
		"a.jpg":                         "",
		"":                              "",
		"../../etc/passwd":              "etc",
		"a/../../b/c.txt":               "a/b",
		"./a/./b/c":                     "a/b",
		"/abs/path/f":                   "abs/path",
		`\\server\share\f.txt`:          "server/share",
		"a//b///c/f":                    "a/b/c",
		"dir/":                          "dir",
		"/":                             "",
		"a\x00b/c\x07d/f":               "ab/cd",
		"  spaced / name /f":            "spaced/name",
		"../..":                         "",
		strings.Repeat("é", 300) + "/f": strings.Repeat("é", 255),
		"bad\xffbyte/f":                 "badbyte",
	} {
		assert.Equal(t, UploadPath(raw), want)
	}
	// Deep folders are cut, at 32 elements.
	deep := strings.Repeat("d/", 40) + "f"
	assert.Equal(t, strings.Count(UploadPath(deep), "/"), 31)
	// What is kept never has a way out of the folder in it.
	for _, raw := range []string{"../x/f", "a/../../x/f", `..\..\x\f`, "a/%2e%2e/f"} {
		for el := range strings.SplitSeq(UploadPath(raw), "/") {
			assert.True(t, el != ".." && el != "." && !strings.ContainsAny(el, `\`))
		}
	}
}
