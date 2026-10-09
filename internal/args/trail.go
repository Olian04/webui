package args

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/Olian04/webui/internal/ir"
)

// A feed is paged by a cursor, which says where the next page is and nothing of
// where this one is, or of the way back. The address carries both: "<id>.row" is the
// number of the page's first row, and "<id>.back" the pages before it, so a table can
// say which rows it shows and offer Previous with nothing kept on the server.

// RowKey is the address parameter of the number of a feed page's first row.
func RowKey(id string) string { return ViewKey(id, "row") }

// BackKey is the address parameter of the pages a feed has been through, one value
// for each.
func BackKey(id string) string { return ViewKey(id, "back") }

// MaxTrail bounds what the address remembers of the way back, in characters of the
// address itself: each page costs its escaped text and the parameter name that
// carries it ("&objects.back="), so many short pages cost as much as a few long ones.
// A cursor is whatever the source made it and may be long, so the oldest pages are
// forgotten first: Previous goes back as far as this holds, and First page is always
// there.
const MaxTrail = 3000

// EncodeMark writes a page as "<start>:<cursor>", and DecodeMark reads one back. The
// cursor may contain ":", so only the first one divides.
func EncodeMark(m ir.Mark) string { return strconv.Itoa(m.Start) + ":" + m.After }

func DecodeMark(s string) (ir.Mark, bool) {
	start, after, ok := strings.Cut(s, ":")
	n, err := strconv.Atoi(start)
	if !ok || err != nil || n < 0 || n > 1<<40 {
		return ir.Mark{}, false
	}
	return ir.Mark{Start: n, After: after}, true
}

// DecodeTrail reads the pages out of the values of the back parameter, which the
// parser joins with ListSep. What is not a page, or does not fit, is dropped, the
// oldest first.
func DecodeTrail(id, joined string) []ir.Mark {
	var out []ir.Mark
	for _, part := range strings.Split(joined, ListSep) {
		if m, ok := DecodeMark(part); ok {
			out = append(out, m)
		}
	}
	return fit(id, out)
}

// PushMark adds the page being left to the pages before it, and forgets the oldest
// while they are too long for the address.
func PushMark(id string, trail []ir.Mark, m ir.Mark) []ir.Mark {
	return fit(id, append(append([]ir.Mark(nil), trail...), m))
}

// EncodeTrail is the value of the back parameter for these pages.
func EncodeTrail(trail []ir.Mark) string {
	parts := make([]string, len(trail))
	for i, m := range trail {
		parts[i] = EncodeMark(m)
	}
	return strings.Join(parts, ListSep)
}

// cost is what a page adds to the address of a table: its escaped text, and the "&",
// the parameter's name and the "=" around it.
func cost(id string, m ir.Mark) int {
	return len(url.QueryEscape(EncodeMark(m))) + len(BackKey(id)) + 2
}

func fit(id string, trail []ir.Mark) []ir.Mark {
	size := 0
	for _, m := range trail {
		size += cost(id, m)
	}
	for len(trail) > 0 && size > MaxTrail {
		size -= cost(id, trail[0])
		trail = trail[1:]
	}
	return trail
}
